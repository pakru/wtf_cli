# Add Google Gemini as New LLM Provider

**Issue**: [#7 - Add google gemini as new llm provider](https://github.com/pakru/wtf_cli/issues/7)

Add native Google Gemini support so users can use their personal Google account's API key as an LLM source, without routing through OpenRouter.

---

## Reference Documentation

| Resource | URL |
|---|---|
| Gemini API Overview | https://ai.google.dev/gemini-api/docs |
| Get API Key (Google AI Studio) | https://aistudio.google.com/app/apikey |
| API Key Usage & Auth | https://ai.google.dev/gemini-api/docs/api-key |
| Available Models | https://ai.google.dev/gemini-api/docs/models |
| Pricing & Free Tier | https://ai.google.dev/gemini-api/docs/pricing |
| Go Gen AI SDK (pkg.go.dev) | https://pkg.go.dev/google.golang.org/genai |
| Go Gen AI SDK (GitHub) | https://github.com/googleapis/go-genai |
| REST API Reference | https://ai.google.dev/gemini-api/docs/text-generation |
| Thinking / Reasoning Config | https://ai.google.dev/gemini-api/docs/thinking |

---

## Approach: Native Google Gen AI SDK

Use the official **`google.golang.org/genai`** Go SDK (v1.46.0, Apache 2.0) to call the native Gemini API directly.

**Why native SDK over OpenAI-compatible endpoint:**
- Access to thinking/reasoning config (important for Gemini 2.5 Pro and 3.x models)
- Access to safety settings
- Better error handling for Gemini-specific errors (safety blocks, context overflow)
- Future-proof: new Gemini features land in the native API first
- First-party Google SDK with active maintenance

**Implementation pattern:** Follows the `anthropic.go` approach -- a standalone provider with its own SDK client, translating between wtf_cli's `ai.Message`/`ai.ChatRequest` types and Gemini's `genai.Content`/`genai.Part` types.

---

## Authentication

Users get a **free API key** from [Google AI Studio](https://aistudio.google.com/app/apikey).

- First-time users: Google AI Studio auto-creates a default GCP project and API key after accepting ToS
- No billing account required for free tier
- API key is passed via the **`x-goog-api-key`** HTTP header (the Go SDK handles this automatically)
- The Go SDK reads from `GOOGLE_API_KEY` or `GEMINI_API_KEY` env var by default, but wtf_cli uses config-only approach

> **Important**: A Google One AI Premium subscription (Gemini Advanced in browser) does NOT grant API access. API access is separate, through the API key.

---

## Available Models

Source: https://ai.google.dev/gemini-api/docs/models

| Model ID | Context Window | Max Output | Status | Best For |
|---|---|---|---|---|
| `gemini-3-pro-preview` | 1,048,576 | 65,536 | Preview | Most capable, complex reasoning |
| `gemini-3-flash-preview` | 1,048,576 | 65,536 | Preview | Fast general purpose (latest gen) |
| `gemini-2.5-pro` | 1,048,576 | 65,536 | Stable | Complex reasoning, coding |
| `gemini-2.5-flash` | 1,048,576 | 65,536 | Stable | Best price-performance (recommended default) |
| `gemini-2.5-flash-lite` | 1,048,576 | 65,536 | Stable | Cost efficiency, high volume |

### Pricing (Paid Tier, per 1M tokens)

Source: https://ai.google.dev/gemini-api/docs/pricing

| Model | Input | Output |
|---|---|---|
| Gemini 3 Flash Preview | $0.50 | $3.00 |
| Gemini 2.5 Flash | $0.30 | $2.50 |
| Gemini 2.5 Flash-Lite | $0.10 | $0.40 |
| Gemini 2.5 Pro | $1.25-$2.50 | $10.00-$15.00 |

Free tier is available with rate limits (exact RPM/TPM varies by model, see pricing page).

---

## Proposed Changes

### [NEW] `pkg/ai/providers/gemini.go`

New provider implementation using the official Go Gen AI SDK. Follows the `anthropic.go` pattern.

```go
import "google.golang.org/genai"

func init() {
    ai.RegisterProvider(ai.ProviderInfo{
        Type:        ai.ProviderGemini,
        Name:        "Google Gemini",
        Description: "Direct Google Gemini API access with free tier",
        AuthMethod:  "api_key",
        RequiresKey: true,
    }, NewGeminiProvider)
}

type GeminiProvider struct {
    client             *genai.Client
    defaultModel       string
    defaultTemperature float64
    defaultMaxTokens   int
}

func NewGeminiProvider(cfg ai.ProviderConfig) (ai.Provider, error) {
    geminiCfg := cfg.Config.Providers.Gemini
    // Validate config...

    client, err := genai.NewClient(ctx, &genai.ClientConfig{
        APIKey:  geminiCfg.APIKey,
        Backend: genai.BackendGeminiAPI,
    })
    // ...
}
```

**Key translation logic:**
- `ai.Message{Role: "system", Content: "..."}` -> `genai.GenerateContentConfig.SystemInstruction`
- `ai.Message{Role: "developer", Content: "..."}` -> Merge into `SystemInstruction` (appended after system content)
- `ai.Message{Role: "user", Content: "..."}` -> `genai.Content{Role: "user", Parts: []*genai.Part{{Text: "..."}}}`
- `ai.Message{Role: "assistant", Content: "..."}` -> `genai.Content{Role: "model", Parts: []*genai.Part{{Text: "..."}}}`
- Streaming: `client.Models.GenerateContentStream()` returns an iterator

> **Developer role handling**: `chat_handler.go:147` emits `{Role: "developer", Content: termCtx.UserPrompt}` for TTY context. Gemini has no "developer" role. The provider must collect all `system` and `developer` messages and concatenate them into a single `SystemInstruction`. The concatenation order follows message order (system first, then developer). If multiple system/developer messages appear, join with `"\n\n"`.

**Streaming implementation:**
```go
// Go Gen AI SDK streaming pattern:
stream := client.Models.GenerateContentStream(ctx, model, contents, config)
for resp, err := range stream {
    if err != nil {
        // handle error
    }
    text := resp.Text()
    // yield text delta
}
```

The `geminiStream` adapter will wrap this iterator to implement our `ai.ChatStream` interface (`Next()`, `Content()`, `Err()`, `Close()`).

### [MODIFY] `pkg/ai/registry.go`

Add `ProviderGemini` constant and include in `SupportedProviders()`.

```go
const (
    ProviderOpenRouter ProviderType = "openrouter"
    ProviderOpenAI     ProviderType = "openai"
    ProviderCopilot    ProviderType = "copilot"
    ProviderAnthropic  ProviderType = "anthropic"
    ProviderGemini     ProviderType = "gemini"      // NEW
)

func SupportedProviders() []ProviderType {
    return []ProviderType{
        ProviderOpenRouter,
        ProviderOpenAI,
        ProviderCopilot,
        ProviderAnthropic,
        ProviderGemini,  // NEW
    }
}
```

### [MODIFY] `pkg/config/config.go`

Add `GeminiConfig` struct and include in `ProvidersConfig`.

```go
type GeminiConfig struct {
    APIKey            string  `json:"api_key"`
    Model             string  `json:"model"`
    Temperature       float64 `json:"temperature"`
    MaxTokens         int     `json:"max_tokens"`
    APITimeoutSeconds int     `json:"api_timeout_seconds"`
}

type ProvidersConfig struct {
    OpenAI    OpenAIConfig    `json:"openai"`
    Copilot   CopilotConfig   `json:"copilot"`
    Anthropic AnthropicConfig `json:"anthropic"`
    Gemini    GeminiConfig    `json:"gemini"`     // NEW
}
```

> **Note**: No `api_url` field needed. The Go Gen AI SDK handles the endpoint internally via `genai.BackendGeminiAPI`.

**Default values:**
```json
{
  "api_key": "",
  "model": "gemini-2.5-flash",
  "temperature": 0.7,
  "max_tokens": 8192,
  "api_timeout_seconds": 60
}
```

**Where defaults are enforced (primary source + defensive fallback):**
- **`Default()` (config.go:79)**: Add Gemini defaults to `Providers.Gemini` in the returned `Config` struct. This is the primary source for default values.
- **`configPresence` (config.go:268)**: Add a `Providers` -> `Gemini` pointer struct mirroring `GeminiConfig` fields. This lets `applyDefaults()` detect which fields the user explicitly set vs omitted.
- **`applyDefaults()` (config.go:291)**: Add Gemini branch that fills missing fields from `Default()`, following the same pattern as OpenRouter (config.go:302-329).
- **Runtime fallbacks**: `getProviderSettings()` in `handlers.go` provides fallbacks for `model` (if empty string) and `timeout` (if <= 0). Temperature and max_tokens pass through from config as-is.
- **Provider factory fallback defaults**: Keep model/timeout fallback defaults in `NewGeminiProvider()` as defense-in-depth (matching `anthropic.go:70-78`). This catches edge cases where config defaults were not applied.

**Per-field defaulting rules (matching OpenRouter pattern in `applyDefaults`):**
- **model**: Default when absent or empty string. Provider factory also defaults as fallback.
- **temperature**: Default only when absent (nil in presence check). `0.0` is a valid user choice (deterministic output).
- **max_tokens**: Default when absent OR `<= 0`. Zero is not a valid user choice.
  - Behavior choice: silently normalize to default (8192), do not hard-reject at runtime.
- **api_timeout_seconds**: Default when absent OR `<= 0`.

Add `validateGemini()` method and wire it into `Validate()` switch. Also add `"gemini"` to `SupportedProviders()` in `config.go`.

### [MODIFY] `pkg/commands/handlers.go`

Add `"gemini"` case to `getProviderSettings()` (line 170). Without this, Gemini falls into the `default` branch and uses OpenRouter settings.

```go
func getProviderSettings(cfg config.Config) (model string, temperature float64, maxTokens int, timeout int) {
    switch cfg.LLMProvider {
    // ... existing cases ...
    case "gemini":                                       // NEW
        model = cfg.Providers.Gemini.Model
        if model == "" {
            model = "gemini-2.5-flash"
        }
        temperature = cfg.Providers.Gemini.Temperature
        maxTokens = cfg.Providers.Gemini.MaxTokens
        timeout = cfg.Providers.Gemini.APITimeoutSeconds
        if timeout <= 0 {
            timeout = 60
        }
    default:
        // OpenRouter fallback...
    }
    return
}
```

### [MODIFY] `pkg/ai/models.go`

Add `FetchGeminiModels()` function using the Go SDK's model listing, and a static fallback in `GetProviderModels()`.

```go
// FetchGeminiModels retrieves the model list via the Go Gen AI SDK.
func FetchGeminiModels(ctx context.Context, apiKey string) ([]ModelInfo, error) {
    client, err := genai.NewClient(ctx, &genai.ClientConfig{
        APIKey:  apiKey,
        Backend: genai.BackendGeminiAPI,
    })
    // List models, filter to gemini-* prefix
}

// Static fallback in GetProviderModels():
case "gemini":
    return []ModelInfo{
        {ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Description: "Best price-performance"},
        {ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Description: "Advanced reasoning and coding"},
        {ID: "gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash Lite", Description: "Lightweight, low latency"},
        {ID: "gemini-3-flash-preview", Name: "Gemini 3 Flash (Preview)", Description: "Latest generation flash"},
        {ID: "gemini-3-pro-preview", Name: "Gemini 3 Pro (Preview)", Description: "Most capable model"},
    }
```

### [MODIFY] `pkg/ui/components/settings/settings_panel.go`

The settings panel has **hardcoded per-provider switches** in multiple locations. All need a `"gemini"` case. This is NOT auto-discovered from the registry.

**1. `buildFields()` (line 77) -- add Gemini fields:**
```go
case "gemini":
    sp.fields = append(sp.fields,
        SettingField{Label: "API Key", Key: "gemini_api_key", Value: sp.config.Providers.Gemini.APIKey, Type: "string", Masked: true},
        SettingField{Label: "Model", Key: "gemini_model", Value: sp.getGeminiModel(), Type: "string"},
        SettingField{Label: "Temperature", Key: "gemini_temperature", Value: fmt.Sprintf("%.1f", sp.config.Providers.Gemini.Temperature), Type: "float"},
        SettingField{Label: "Max Tokens", Key: "gemini_max_tokens", Value: fmt.Sprintf("%d", sp.config.Providers.Gemini.MaxTokens), Type: "int"},
    )
```

**2. `applyField()` (line 520) -- add Gemini field writers:**
```go
// Gemini fields
case "gemini_api_key":
    sp.config.Providers.Gemini.APIKey = field.Value
    sp.refreshProviderStatusFields()
case "gemini_model":
    sp.config.Providers.Gemini.Model = field.Value
case "gemini_temperature":
    if v, err := strconv.ParseFloat(field.Value, 64); err == nil {
        sp.config.Providers.Gemini.Temperature = v
    }
case "gemini_max_tokens":
    if v, err := strconv.Atoi(field.Value); err == nil {
        sp.config.Providers.Gemini.MaxTokens = v
    }
```

**3. `getSelectedProviderStatus()` (line 186) -- add Gemini status:**
```go
case "gemini":
    return sp.getGeminiStatus()
```

**4. New helper methods:**
```go
func (sp *SettingsPanel) getGeminiModel() string {
    if sp.config.Providers.Gemini.Model != "" {
        return sp.config.Providers.Gemini.Model
    }
    return "gemini-2.5-flash"
}

func (sp *SettingsPanel) getGeminiStatus() string {
    if strings.TrimSpace(sp.config.Providers.Gemini.APIKey) != "" {
        return "✅ Ready"
    }
    return "❌ Missing API key"
}

func (sp *SettingsPanel) SetGeminiModelValue(modelID string) {
    // Update the gemini_model field value and config
}
```

**5. Model picker trigger (enter-key handler, ~line 327) -- add Gemini model picker:**

Must pass `APIKey` in `OpenModelPickerMsg` so that `model.go` can call `fetchGeminiModelsCmd()` for dynamic model fetching.

```go
if field.Key == "gemini_model" {
    options := ai.GetProviderModels("gemini")
    apiKey := sp.config.Providers.Gemini.APIKey
    return func() tea.Msg {
        return picker.OpenModelPickerMsg{
            Options:  options,
            Current:  sp.config.Providers.Gemini.Model,
            FieldKey: "gemini_model",
            APIKey:   apiKey,  // Required for dynamic model fetching
        }
    }
}
```

### [MODIFY] `pkg/ui/model.go`

Three switch statements need a `"gemini_model"` case.

**1. Model picker open handler (line 747) -- trigger dynamic model fetch:**
```go
case "gemini_model":
    cmd = fetchGeminiModelsCmd(msg.APIKey)
```

**2. Model picker select handler (line 777) -- write selected model back:**
```go
case "gemini_model":
    m.settingsPanel.SetGeminiModelValue(msg.ModelID)
```

**3. `getModelForProvider()` (line 1263) -- display model name in status bar:**
```go
case "gemini":
    if cfg.Providers.Gemini.Model != "" {
        return cfg.Providers.Gemini.Model
    }
    return "gemini-2.5-flash"
```

**4. New command function (follows `fetchOpenAIModelsCmd` pattern at model.go:1311):**
```go
func fetchGeminiModelsCmd(apiKey string) tea.Cmd {
    if apiKey == "" {
        return nil
    }

    return func() tea.Msg {
        slog.Info("gemini_models_fetch_start")
        ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
        defer cancel()

        models, err := ai.FetchGeminiModels(ctx, apiKey)
        return providerModelsRefreshMsg{Models: models, FieldKey: "gemini_model", Err: err}
    }
}
```

### [MODIFY] `cmd/wtf_cli/main.go`

Duplicate `getModelForProvider()` function at line 73 also needs a `"gemini"` case:

```go
case "gemini":
    if cfg.Providers.Gemini.Model != "" {
        return cfg.Providers.Gemini.Model
    }
    return "gemini-2.5-flash"
```

---

## Implementation Phases

### Phase 1: Config & Registry
**Files:** `pkg/config/config.go`, `pkg/ai/registry.go`

- [ ] Add `GeminiConfig` struct to `config.go`
- [ ] Add `Gemini` field to `ProvidersConfig`
- [ ] Add Gemini defaults in `Default()` (config.go:79): model=`gemini-2.5-flash`, temperature=0.7, max_tokens=8192, api_timeout_seconds=60
- [ ] Add Gemini pointer struct to `configPresence` (config.go:268)
- [ ] Add Gemini branch to `applyDefaults()` (config.go:291), following OpenRouter pattern
- [ ] Add `validateGemini()` validation method (require api_key)
- [ ] Wire validation into `Validate()` switch statement
- [ ] Add `"gemini"` to `SupportedProviders()` in `config.go`
- [ ] Add `ProviderGemini` constant to `registry.go`
- [ ] Add `ProviderGemini` to `SupportedProviders()` in `registry.go`

### Phase 2: Provider Implementation
**Files:** `pkg/ai/providers/gemini.go`

- [ ] Add `google.golang.org/genai` dependency (`go get google.golang.org/genai`)
- [ ] Create `pkg/ai/providers/gemini.go`
- [ ] Implement `init()` with `RegisterProvider` call
- [ ] Implement `GeminiProvider` struct with `*genai.Client`
- [ ] Implement `NewGeminiProvider` factory (validate config, create genai client with API key)
- [ ] Implement message translation: `ai.Message` -> `genai.Content`
  - `system` -> collected into `SystemInstruction`
  - `developer` -> collected into `SystemInstruction` (appended after system, joined with `"\n\n"`)
  - `user` -> `genai.Content{Role: "user", ...}`
  - `assistant` -> `genai.Content{Role: "model", ...}`
- [ ] Implement `CreateChatCompletion` using `client.Models.GenerateContent()`
- [ ] Implement `CreateChatCompletionStream` using `client.Models.GenerateContentStream()`
- [ ] Implement `geminiStream` struct adapting the SDK iterator to `ai.ChatStream` interface
- [ ] Add compile-time interface check: `var _ ai.Provider = (*GeminiProvider)(nil)`

### Phase 3: Runtime Wiring
**Files:** `pkg/commands/handlers.go`

- [ ] Add `"gemini"` case to `getProviderSettings()` (line 170)
- [ ] Set model fallback to `"gemini-2.5-flash"`, timeout fallback to 60

### Phase 4: Model Discovery
**Files:** `pkg/ai/models.go`

- [ ] Add static fallback model list in `GetProviderModels("gemini")`
- [ ] Implement `FetchGeminiModels()` using Go Gen AI SDK
- [ ] Filter model list to `gemini-*` prefixed models only

### Phase 5: Settings Panel & UI Wiring
**Files:** `pkg/ui/components/settings/settings_panel.go`, `pkg/ui/model.go`, `cmd/wtf_cli/main.go`

- [ ] Add `"gemini"` case to `buildFields()` with api_key, model, temperature, max_tokens fields
- [ ] Add Gemini field cases to `applyField()` (gemini_api_key, gemini_model, gemini_temperature, gemini_max_tokens)
- [ ] Add `"gemini"` case to `getSelectedProviderStatus()`
- [ ] Add helper methods: `getGeminiModel()`, `getGeminiStatus()`, `SetGeminiModelValue()`
- [ ] Add `"gemini_model"` model picker trigger in enter-key handler, passing `APIKey` in `OpenModelPickerMsg`
- [ ] Add `"gemini_model"` case in `model.go` model picker open handler (line 747) -> `fetchGeminiModelsCmd()`
- [ ] Add `"gemini_model"` case in `model.go` model picker select handler (line 777) -> `SetGeminiModelValue()`
- [ ] Add `"gemini"` case in `model.go` `getModelForProvider()` (line 1263)
- [ ] Add `fetchGeminiModelsCmd()` function to `model.go`
- [ ] Add `"gemini"` case in `main.go` `getModelForProvider()` (line 73)

### Phase 6: Tests
**Files:** `pkg/ai/providers/gemini_test.go` (NEW), `pkg/config/config_test.go`, `pkg/ai/registry_test.go`, `pkg/ai/models_test.go`, `pkg/ui/components/settings/settings_panel_test.go`, `pkg/ui/model_test.go`

**Provider tests (`gemini_test.go`):**
- [ ] `GeminiConfig` validation (valid config, missing key)
- [ ] `GeminiProvider` creation (valid config, empty key rejected, empty model defaults to `gemini-2.5-flash` in factory)
- [ ] Message translation: system -> SystemInstruction, user -> "user" role, assistant -> "model" role
- [ ] Message translation: developer -> merged into SystemInstruction
- [ ] Message translation: mixed system + developer + user messages
- [ ] Streaming: correctly iterates over chunks and handles errors
- [ ] Provider registration: verify init() registers correctly

**Config tests (`config_test.go`):**
- [ ] Add `validateGemini()` test cases (valid, missing key)

**Registry tests (`registry_test.go`):**
- [ ] Update `TestSupportedProviders`: expected count 4 -> 5
- [ ] Update `TestValidateProviderType`: add `"gemini"` to valid entries
- [ ] Update `TestProviderTypeConstants`: add `ProviderGemini` assertion

**Model tests (`models_test.go`):**
- [ ] Add `"gemini"` case to `TestGetProviderModels` table
- [ ] Test `FetchGeminiModels` with mock response

**Settings panel tests (`settings_panel_test.go`):**
- [ ] Test `buildFields()` produces correct fields when provider is `"gemini"`
- [ ] Test `applyField()` correctly writes Gemini config values
- [ ] Test `getGeminiStatus()` returns correct status strings
- [ ] Test model picker trigger sends `OpenModelPickerMsg` with `APIKey` and `FieldKey: "gemini_model"`

**Model UI tests (`model_test.go`):**
- [ ] Test `"gemini_model"` case in model picker open handler triggers `fetchGeminiModelsCmd`
- [ ] Test `"gemini_model"` case in model picker select handler calls `SetGeminiModelValue`
- [ ] Test `getModelForProvider()` returns correct model for `"gemini"` provider

### Phase 7: Documentation
- [ ] Update README or docs with Gemini setup instructions
- [ ] Verify `go build` succeeds
- [ ] Verify all tests pass

---

## Verification Plan

### Unit Tests
- Provider factory rejects empty API key; empty model falls back to default
- Message translation correctly maps roles (system -> SystemInstruction, developer -> SystemInstruction appended, assistant -> "model" role)
- Streaming correctly iterates over chunks and handles errors
- Config validation catches invalid Gemini config
- Model fetching parses response and filters correctly
- Registry count updated from 4 to 5
- Settings panel renders Gemini fields and applies values correctly
- Model picker triggers dynamic fetch with API key

### Integration Tests (Mock)
- Full chat completion round-trip
- Streaming response with multiple chunks
- Error handling: invalid API key, rate limit, server error
- Timeout handling

### Manual Testing
1. Get API key from https://aistudio.google.com/app/apikey
2. Set `"llm_provider": "gemini"` in `~/.wtf_cli/config.json` with the API key
3. Run `wtf_cli`, execute a failing command
4. Use `/explain` -- verify streaming response works
5. Use `/chat` -- verify multi-turn conversation works
6. Open `/settings` -- verify Gemini appears in provider picker
7. In `/settings` -- verify Gemini fields appear (api_key, model, temperature, max_tokens)
8. In `/settings` -- click model field, verify model picker opens with Gemini models
9. In `/settings` -- verify status shows "Ready" when API key is set
10. Test with different models (`gemini-2.5-flash`, `gemini-2.5-pro`)

---

## Example Config

```json
{
  "llm_provider": "gemini",
  "providers": {
    "gemini": {
      "api_key": "YOUR_GOOGLE_AI_API_KEY",
      "model": "gemini-2.5-flash",
      "temperature": 0.7,
      "max_tokens": 8192,
      "api_timeout_seconds": 60
    }
  }
}
```

---

## Dependencies

| Package | Version | Purpose |
|---|---|---|
| `google.golang.org/genai` | v1.46.0+ | Official Google Gen AI Go SDK |

Install: `go get google.golang.org/genai`

---

## Security Considerations

1. **API key storage**: Follows existing pattern -- stored in `~/.wtf_cli/config.json` with 0600 permissions
2. **No logging**: API key must not be written to log files (follow existing `slog.Debug` patterns that log `"has_key", apiKey != ""` not the actual key)
3. **HTTPS only**: The Go Gen AI SDK communicates over HTTPS by default

---

## File Summary

| File | Action | Description |
|---|---|---|
| `pkg/config/config.go` | MODIFY | Add `GeminiConfig`, defaults, validation, supported providers |
| `pkg/ai/registry.go` | MODIFY | Add `ProviderGemini` constant, update `SupportedProviders()` |
| `pkg/ai/providers/gemini.go` | NEW | Provider implementation using `google.golang.org/genai` SDK |
| `pkg/ai/models.go` | MODIFY | Add `FetchGeminiModels()` and static model fallback |
| `pkg/commands/handlers.go` | MODIFY | Add `"gemini"` case to `getProviderSettings()` |
| `pkg/ui/components/settings/settings_panel.go` | MODIFY | Add Gemini fields, apply logic, status, model picker trigger |
| `pkg/ui/model.go` | MODIFY | Add Gemini model picker open/select handlers, `getModelForProvider()`, `fetchGeminiModelsCmd()` |
| `cmd/wtf_cli/main.go` | MODIFY | Add `"gemini"` case to `getModelForProvider()` |
| `pkg/ai/providers/gemini_test.go` | NEW | Unit tests for provider, message translation, streaming |
| `pkg/ai/registry_test.go` | MODIFY | Update provider count 4->5, add Gemini entries |
| `pkg/ai/models_test.go` | MODIFY | Add Gemini model tests |
| `pkg/config/config_test.go` | MODIFY | Add Gemini config validation tests |
| `pkg/ui/components/settings/settings_panel_test.go` | MODIFY | Add Gemini fields/apply/status/picker tests |
