# Multi-Provider LLM Support

Add support for multiple LLM providers, allowing users to leverage their existing subscriptions (GitHub Copilot, OpenAI, Anthropic) instead of requiring a separate OpenRouter API key.

## Background

Investigation of [anomalyco/opencode](https://github.com/anomalyco/opencode) revealed their approach:
- **Modular provider architecture** with a unified interface
- **OAuth Device Flow** for services like GitHub Copilot and ChatGPT Plus
- **API key storage** in `~/.local/share/opencode/auth.json`
- **Vercel AI SDK** for unified provider communication

Our existing `pkg/ai/provider.go` interface is well-suited for extension.

---

## Proposed Changes

### Auth Module

#### [NEW] `pkg/ai/auth/manager.go`
Token storage and retrieval for authenticated providers. Stores credentials in `~/.wtf_cli/auth.json`.

```go
type AuthManager struct {
    configPath string
}

type StoredCredentials struct {
    Provider     string    `json:"provider"`
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token,omitempty"`
    ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

func (m *AuthManager) Save(creds StoredCredentials) error
func (m *AuthManager) Load(provider string) (*StoredCredentials, error)
func (m *AuthManager) Delete(provider string) error
```

#### [NEW] `pkg/ai/auth/device_flow.go`
OAuth 2.0 Device Authorization Flow implementation (used by GitHub Copilot).

```go
type DeviceFlowConfig struct {
    ClientID        string
    DeviceCodeURL   string
    TokenURL        string
    Scopes          []string
}

func StartDeviceFlow(cfg DeviceFlowConfig) (*DeviceCodeResponse, error)
func PollForToken(cfg DeviceFlowConfig, deviceCode string, interval int) (*TokenResponse, error)
```

#### [NEW] `pkg/ai/auth/pkce_flow.go`
OAuth 2.0 Authorization Code with PKCE flow (used by OpenAI ChatGPT Plus/Pro).

```go
type PKCEFlowConfig struct {
    ClientID     string
    AuthURL      string
    TokenURL     string
    RedirectPort int      // Local callback server port
    Scopes       []string
}

func StartPKCEFlow(cfg PKCEFlowConfig) (*TokenResponse, error)
func generateCodeVerifier() string
func generateCodeChallenge(verifier string) string
```

---

### Supported Providers

| Provider | Auth Method | Subscription Required |
|----------|-------------|----------------------|
| **OpenRouter** | API Key | Pay-per-use |
| **OpenAI** | API Key or OAuth (PKCE) | ChatGPT Plus/Pro for OAuth |
| **GitHub Copilot** | OAuth Device Flow | Copilot Pro/Pro+/Business/Enterprise |
| **Anthropic** | API Key | Pay-per-use |
| **Google AI (Gemini)** | API Key | Free tier available |
| **Groq** | API Key | Free tier available |
| **Ollama** | None (local) | None (self-hosted) |
| **Custom/OpenAI-compatible** | API Key + Base URL | Varies |

> **Extensibility**: The provider registry pattern allows easy addition of new providers. Any OpenAI-compatible API can be added by specifying a custom base URL.

> [!TIP]
> **GitHub Copilot has a FREE tier!** Users with just a GitHub account (no subscription) get 2,000 code completions + 50 chat requests/month. Verified students, teachers, and OSS maintainers get full Copilot Pro for free.

> [!NOTE]
> **GitHub Copilot SDK** (Technical Preview, Jan 2026): Official Go SDK available via `go get github.com/github/copilot-sdk/go`. This provides a cleaner integration path than OAuth device flow.

> [!WARNING]
> **OpenAI OAuth uses unofficial endpoints**: The PKCE flow uses OpenAI's internal Codex CLI client credentials. This approach may break if OpenAI changes their auth system and is not officially supported.

---

### Provider Implementations

#### [NEW] `pkg/ai/providers/openai.go`
Direct OpenAI API provider supporting both API key and OAuth (ChatGPT Plus/Pro).

**OAuth Configuration (PKCE Flow):**
```go
var openaiOAuthConfig = &oauth2.Config{
    ClientID: "app_EMoamEEZ73f0CkXaXp7hrann", // OpenAI Codex CLI client
    Endpoint: oauth2.Endpoint{
        AuthURL:  "https://auth.openai.com/oauth/authorize",
        TokenURL: "https://auth.openai.com/oauth/token",
    },
    Scopes: []string{"openid", "profile", "email"},
}
```

**Authentication Flow (via Settings Panel):**
1. User opens Settings → selects "OpenAI" as provider → chooses "ChatGPT Plus/Pro"
2. Clicks "Connect" → App opens browser to OpenAI login page (with PKCE params)
3. User logs in with their OpenAI account
4. Browser redirects to local callback server (`http://localhost:PORT/callback`)
5. App exchanges auth code for access/refresh tokens
6. Token stored securely, provider ready to use

#### [NEW] `pkg/ai/providers/copilot.go`
GitHub Copilot provider with two integration options:

**Option A: Copilot SDK (Recommended)**
```go
import "github.com/github/copilot-sdk/go"

// Uses official SDK - handles auth, context, tool orchestration
client, _ := copilot.NewClient()
session, _ := client.NewSession(ctx)
response, _ := session.SendMessage(ctx, "Hello")
```

**Option B: OAuth Device Flow (Fallback)**
1. User opens Settings → selects "GitHub Copilot" as provider
2. Clicks "Connect" button → App requests device code from GitHub
3. UI displays: "Visit github.com/login/device and enter code: XXXX-XXXX"
4. App polls until user authorizes
5. Exchange code for GitHub token
6. Use token to access Copilot's OpenAI-compatible API

#### [NEW] `pkg/ai/providers/anthropic.go`
Direct Anthropic Claude API provider.

#### [MODIFY] `pkg/ai/openrouter.go`
Move to `pkg/ai/providers/` for consistency.

---

### Provider Registry

#### [NEW] `pkg/ai/registry.go`
Central factory for provider instantiation.

```go
type ProviderFactory func(cfg ProviderConfig, auth *auth.AuthManager) (Provider, error)

var registry = map[string]ProviderFactory{
    "openrouter": NewOpenRouterProvider,
    "openai":     NewOpenAIProvider,
    "copilot":    NewCopilotProvider,
    "anthropic":  NewAnthropicProvider,
}

func GetProvider(name string, cfg ProviderConfig, auth *auth.AuthManager) (Provider, error)
func ListProviders() []string
```

---

### Configuration Updates

#### [MODIFY] `pkg/config/config.go`

Extend config to support multiple providers:

```json
{
  "llm_provider": "copilot",
  "providers": {
    "openrouter": {
      "api_key": "",
      "model": "google/gemini-2.0-flash-exp:free"
    },
    "openai": {
      "auth_method": "oauth",
      "model": "gpt-4o"
    },
    "copilot": {
      "model": "gpt-4o"
    },
    "anthropic": {
      "api_key": "",
      "model": "claude-3-5-sonnet-20241022"
    }
  }
}
```

---

### Settings Panel Integration

#### [MODIFY] `pkg/ui/settings.go`
Extend existing settings panel with provider configuration.

**Settings Panel → LLM Provider Section:**
- Provider selection dropdown (OpenRouter, OpenAI, Copilot, Anthropic, etc.)
- API key input field (for API key providers)
- OAuth connect button (for OAuth providers → triggers device flow)
- Model selection dropdown (populated based on selected provider)
- Connection status indicator

**Flow:**
1. User opens `/settings` → navigates to "LLM Provider" section
2. User selects provider from dropdown
3. Based on provider:
   - **API Key providers**: Text input for key → validate on save
   - **OAuth providers**: "Connect" button → device flow → display code → poll for auth
4. Credentials stored via AuthManager
5. Model dropdown populates with available models for that provider

---

## Implementation Phases

### Phase 1: Auth Infrastructure
- [ ] Create `pkg/ai/auth/` module with AuthManager
- [ ] Implement secure credential storage (`~/.wtf_cli/auth.json`)
- [ ] Add OAuth Device Flow implementation

### Phase 2: Provider Registry
- [ ] Create provider registry with factory pattern
- [ ] Refactor OpenRouter provider into `pkg/ai/providers/`
- [ ] Update config validation for multi-provider support

### Phase 3: New Providers
- [ ] Implement OpenAI direct provider
- [ ] Implement GitHub Copilot provider with OAuth
- [ ] Implement Anthropic provider

### Phase 4: Settings Panel Integration
- [ ] Add "LLM Provider" section to settings panel
- [ ] Implement provider selection dropdown
- [ ] Add API key input with validation
- [ ] Add OAuth connect flow UI within settings
- [ ] Add model selection dropdown (per-provider)

---

## Verification Plan

### Unit Tests
- Auth manager store/load/delete operations
- Device flow request/polling logic
- Provider factory registration and instantiation
- Each provider's chat completion methods (mocked)

### Integration Tests
- Full device flow with mock OAuth server
- Provider switching via config changes
- Credential persistence across restarts

### Manual Testing
- Connect to each provider type
- Verify streaming responses work
- Test token refresh for OAuth providers
- Verify error handling for invalid/expired tokens

---

## Security Considerations

1. **Token Storage**: Credentials stored with 0600 permissions
2. **No Logging**: Tokens never written to log files
3. **Secure Transmission**: All API calls over HTTPS
4. **Token Refresh**: Automatic refresh before expiration
5. **Revocation**: Settings panel option to disconnect/remove credentials

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `golang.org/x/oauth2` | OAuth 2.0 implementation |
| `github.com/openai/openai-go` | Already used for OpenRouter |

---

## Estimated Effort

| Phase | Time Estimate |
|-------|---------------|
| Phase 1: Auth Infrastructure | 4-6 hours |
| Phase 2: Provider Registry | 2-3 hours |
| Phase 3: New Providers | 6-8 hours |
| Phase 4: UI Integration | 4-6 hours |
| **Total** | **16-23 hours** |
