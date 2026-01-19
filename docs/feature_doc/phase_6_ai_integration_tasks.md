# Phase 6: AI Integration - Implementation Tasks

## Review Summary
- Phase 6 focuses on OpenRouter-only integration using an OpenAI-compatible client.
- Use `openai-go` with a Base URL override to avoid a bespoke OpenRouter HTTP client.
- Update status bar to show the active model from config.

## Go OpenAI Client Library (Research)
- `github.com/openai/openai-go/v3` (official)
  - Supports `option.WithBaseURL(...)` for OpenAI-compatible endpoints and `option.WithHeader(...)` for OpenRouter-specific headers.
  - Go requirement: 1.22+.

Recommendation: use `openai-go` with OpenRouter base URL + headers.

## Task Breakdown

### Task 6.1: OpenRouter config updates
**Goal:** update configuration to cover OpenRouter-only integration.

**Tasks:**
- Add/confirm OpenRouter config block in `pkg/config/config.go` (api_key, api_url, model, temperature, max_tokens, api_timeout_seconds).
- Add validation rules (required api_key, valid model string, timeout ranges, temperature bounds).
- Add migration logic from the Phase 2 config shape to the Phase 6 OpenRouter-only shape.
- Update settings UI to expose OpenRouter fields only (no provider selection).

**Definition of Done:**
- Config supports OpenRouter settings with defaults.
- Validation errors map to friendly UI errors.
- Migration preserves existing OpenRouter config.

### Task 6.2: OpenRouter client adapter (openai-go)
**Goal:** avoid a bespoke OpenRouter implementation by using an OpenAI-compatible library.

**Tasks:**
- Implement `OpenRouterProvider` in `pkg/ai/openrouter.go` using `openai-go`.
- Support `BaseURL` override for OpenRouter endpoints.
- Add optional OpenRouter headers: `HTTP-Referer` and `X-Title`.
- Support streaming and non-streaming responses from the same adapter.

**Definition of Done:**
- OpenRouter uses the adapter with config-defined Base URL and headers.
- Base URL and headers are configurable and tested.
- Streaming works end-to-end for `/wtf`.

### Task 6.3: Dynamic model list and cache
**Goal:** implement the `/models` command and cached model lists.

**Tasks:**
- Implement OpenRouter model list fetch (`GET /models`) and cache to `~/.wtf_cli/models_cache.json`.
- Add `/models` command and integrate with the settings panel.

**Definition of Done:**
- `/models` displays available models with pricing/context info when available.
- Cache is read on startup and refreshable on demand.

### Task 6.4: Context builder and prompt assembly
**Goal:** assemble a stable, readable context for LLM calls.

**Tasks:**
- Extract last 100 lines from the buffer.
- Normalize/strip ANSI sequences while preserving readable content.
- Add metadata: last command, exit code, working directory.
- Implement size-safe truncation if output exceeds configured limit.
- Use a system prompt that asks for help and suggestions based on terminal output.

**Definition of Done:**
- Context output is readable and stable for common terminal output.
- Size limits are respected with tests.

### Task 6.5: `/wtf` command handler
**Goal:** dispatch a request to the LLM and render results.

**Tasks:**
- Build request payload from the context builder and user-selected model.
- Implement streaming response handling with a loading indicator.
- Map errors to user-facing messages and logs.

**Definition of Done:**
- `/wtf` sends a request and shows a streamed response.
- Failures display actionable messages without breaking the UI.

### Task 6.6: Response sidebar UI
**Goal:** display AI output without blocking the terminal.

**Tasks:**
- Add right-side split layout and a scrollable sidebar component.
- Support basic markdown formatting (code blocks, bold).
- Add keybinds for close (`Esc`, `q`) and copy (`y`).

**Definition of Done:**
- Sidebar opens/closes without affecting PTY interaction.
- Long responses are scrollable and readable.

### Task 6.7: Status bar model indicator
**Goal:** show the currently active model in the status bar.

**Tasks:**
- Update status bar view model to include the active model from config.
- Refresh status bar when model changes (settings panel or config reload).
- Keep styling consistent with current status bar theme.
- Place the model after the current working directory in the status bar layout.

**Definition of Done:**
- Status bar shows model identifier (e.g., `google/gemini-3.0-flash`).
- Model updates reflect immediately after config changes.
- Format example: `[wtf_cli] /current/dir | [llm]: google/gemini-3.0-flash | Press /`.

### Task 6.8: Testing and verification
**Goal:** ensure provider selection, requests, and UI changes are stable.

**Tasks:**
- Unit tests for OpenRouter config validation.
- Mocked integration tests for OpenRouter requests and streaming.
- Tests for model list parsing and cache read/write.
- UI tests for sidebar layout and scroll behavior.
- UI tests for status bar model rendering.

**Definition of Done:**
- Test coverage matches existing project targets.
- Manual test confirms `/wtf` works for OpenRouter.
