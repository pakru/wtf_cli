package providers

import (
	"encoding/json"
	"fmt"
	"strings"

	"wtf_cli/pkg/ai"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/ssestream"
	"github.com/openai/openai-go/v3/shared"
)

// toChatMessageParam converts our normalized ai.Message to the OpenAI SDK's
// per-role message union. Used by both the OpenAI and OpenRouter providers.
//
// Tool-call wiring:
//   - role="tool" → ToolMessage(content, ToolCallID).
//   - role="assistant" with ToolCalls → ChatCompletionAssistantMessageParam
//     carrying both the text content (if any) and the tool_calls array.
func toChatMessageParam(msg ai.Message) (openai.ChatCompletionMessageParamUnion, error) {
	role := strings.ToLower(strings.TrimSpace(msg.Role))
	switch role {
	case "system":
		return openai.SystemMessage(msg.Content), nil
	case "user":
		return openai.UserMessage(msg.Content), nil
	case "developer":
		return openai.DeveloperMessage(msg.Content), nil
	case "tool":
		if strings.TrimSpace(msg.ToolCallID) == "" {
			return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("tool message requires ToolCallID")
		}
		return openai.ToolMessage(msg.Content, msg.ToolCallID), nil
	case "assistant":
		if len(msg.ToolCalls) == 0 {
			return openai.AssistantMessage(msg.Content), nil
		}
		var asst openai.ChatCompletionAssistantMessageParam
		if msg.Content != "" {
			asst.Content.OfString = openai.String(msg.Content)
		}
		asst.ToolCalls = make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			fn := openai.ChatCompletionMessageFunctionToolCallParam{
				ID: tc.ID,
				Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
					Name:      tc.Name,
					Arguments: argsAsString(tc.Arguments),
				},
			}
			asst.ToolCalls = append(asst.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &fn,
			})
		}
		return openai.ChatCompletionMessageParamUnion{OfAssistant: &asst}, nil
	default:
		return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("unsupported role: %s", msg.Role)
	}
}

// toOpenAIToolUnionParams maps our ToolDefinition list to the OpenAI tool
// union shape. Returns nil for an empty input so callers can leave Tools unset.
func toOpenAIToolUnionParams(defs []ai.ToolDefinition) ([]openai.ChatCompletionToolUnionParam, error) {
	if len(defs) == 0 {
		return nil, nil
	}
	out := make([]openai.ChatCompletionToolUnionParam, 0, len(defs))
	for _, d := range defs {
		params, err := jsonRawToFunctionParameters(d.JSONSchema)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", d.Name, err)
		}
		fn := openai.ChatCompletionFunctionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        d.Name,
				Description: openai.String(d.Description),
				Parameters:  params,
			},
		}
		out = append(out, openai.ChatCompletionToolUnionParam{OfFunction: &fn})
	}
	return out, nil
}

// toOpenAIToolChoice maps our string ToolChoice into the SDK's union. Empty
// string means "auto" when tools are present (handled by caller as omitted).
func toOpenAIToolChoice(choice string) openai.ChatCompletionToolChoiceOptionUnionParam {
	switch strings.ToLower(strings.TrimSpace(choice)) {
	case "", "auto":
		return openai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: openai.String("auto")}
	case "none":
		return openai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: openai.String("none")}
	case "required":
		return openai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: openai.String("required")}
	default:
		return openai.ChatCompletionToolChoiceOptionUnionParam{
			OfFunctionToolChoice: &openai.ChatCompletionNamedToolChoiceParam{
				Function: openai.ChatCompletionNamedToolChoiceFunctionParam{Name: choice},
			},
		}
	}
}

// jsonRawToFunctionParameters decodes a tool's JSON-Schema raw bytes into the
// SDK's FunctionParameters (a map[string]any). Empty input is treated as no
// parameters.
func jsonRawToFunctionParameters(raw json.RawMessage) (shared.FunctionParameters, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("invalid JSON schema: %w", err)
	}
	return shared.FunctionParameters(m), nil
}

func argsAsString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	return string(raw)
}

// openaiCompatStream wraps an OpenAI SDK SSE stream (used by both OpenAI and
// OpenRouter) and accumulates streaming tool-call deltas into a finalized list
// exposed via ToolCalls() at end-of-stream.
//
// OpenAI emits tool calls as deltas: each chunk's
// chunk.Choices[0].Delta.ToolCalls contains entries with an Index field,
// optionally an ID, and partial Function.Name/Function.Arguments. We
// accumulate by Index so the assembled list is whole only after Next() returns
// false.
type openaiCompatStream struct {
	stream         *ssestream.Stream[openai.ChatCompletionChunk]
	toolCallsByIdx map[int64]*pendingToolCall
	toolCallOrder  []int64
	finishReason   string
	finalized      bool
	finalizedCalls []ai.ToolCall
}

type pendingToolCall struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

func newOpenAICompatStream(s *ssestream.Stream[openai.ChatCompletionChunk]) *openaiCompatStream {
	return &openaiCompatStream{
		stream:         s,
		toolCallsByIdx: make(map[int64]*pendingToolCall),
	}
}

func (s *openaiCompatStream) Next() bool {
	for s.stream.Next() {
		chunk := s.stream.Current()
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]

		if choice.FinishReason != "" {
			s.finishReason = choice.FinishReason
		}

		// Accumulate tool-call deltas (don't emit them as text).
		for _, d := range choice.Delta.ToolCalls {
			s.absorbToolCallDelta(d)
		}

		if choice.Delta.Content != "" {
			return true
		}
	}
	return false
}

func (s *openaiCompatStream) absorbToolCallDelta(d openai.ChatCompletionChunkChoiceDeltaToolCall) {
	idx := d.Index
	pending, ok := s.toolCallsByIdx[idx]
	if !ok {
		pending = &pendingToolCall{}
		s.toolCallsByIdx[idx] = pending
		s.toolCallOrder = append(s.toolCallOrder, idx)
	}
	if d.ID != "" {
		pending.ID = d.ID
	}
	if d.Function.Name != "" {
		pending.Name = d.Function.Name
	}
	if d.Function.Arguments != "" {
		pending.Arguments.WriteString(d.Function.Arguments)
	}
}

func (s *openaiCompatStream) Content() string {
	chunk := s.stream.Current()
	if len(chunk.Choices) == 0 {
		return ""
	}
	return chunk.Choices[0].Delta.Content
}

func (s *openaiCompatStream) Err() error {
	return s.stream.Err()
}

func (s *openaiCompatStream) Close() error {
	return s.stream.Close()
}

func (s *openaiCompatStream) ToolCalls() []ai.ToolCall {
	if !s.finalized {
		s.finalize()
	}
	return s.finalizedCalls
}

func (s *openaiCompatStream) StopReason() string {
	return s.finishReason
}

func (s *openaiCompatStream) finalize() {
	s.finalized = true
	if len(s.toolCallOrder) == 0 {
		return
	}
	out := make([]ai.ToolCall, 0, len(s.toolCallOrder))
	for _, idx := range s.toolCallOrder {
		pc := s.toolCallsByIdx[idx]
		args := strings.TrimSpace(pc.Arguments.String())
		if args == "" {
			args = "{}"
		}
		out = append(out, ai.ToolCall{
			ID:        pc.ID,
			Name:      pc.Name,
			Arguments: json.RawMessage(args),
		})
	}
	s.finalizedCalls = out
}

// Compile-time assertion that openaiCompatStream satisfies the ChatStream
// interface.
var _ ai.ChatStream = (*openaiCompatStream)(nil)
