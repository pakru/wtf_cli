package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PKCEFlowConfig holds configuration for OAuth 2.0 Authorization Code with PKCE flow.
type PKCEFlowConfig struct {
	ClientID     string
	AuthURL      string
	TokenURL     string
	RedirectPort int
	Scopes       []string
}

// PKCEFlowResult contains the result of a PKCE flow.
type PKCEFlowResult struct {
	AuthURL      string
	CodeVerifier string
	State        string
	Server       *http.Server
	TokenChan    chan *TokenResponse
	ErrorChan    chan error
}

// StartPKCEFlow initiates the PKCE authorization flow.
// Returns the authorization URL that the user should open in their browser.
func StartPKCEFlow(ctx context.Context, cfg PKCEFlowConfig) (*PKCEFlowResult, error) {
	slog.Debug("pkce_start",
		"auth_url", cfg.AuthURL,
		"redirect_port", cfg.RedirectPort,
		"scopes", strings.Join(cfg.Scopes, " "),
	)
	codeVerifier, err := generateCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}

	codeChallenge := generateCodeChallenge(codeVerifier)

	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/callback", cfg.RedirectPort)

	params := url.Values{}
	params.Set("client_id", cfg.ClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURI)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)
	if len(cfg.Scopes) > 0 {
		params.Set("scope", strings.Join(cfg.Scopes, " "))
	}

	authURL := fmt.Sprintf("%s?%s", cfg.AuthURL, params.Encode())
	slog.Debug("pkce_auth_url_ready")

	tokenChan := make(chan *TokenResponse, 1)
	errorChan := make(chan error, 1)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.RedirectPort),
		Handler: mux,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		if errCode := query.Get("error"); errCode != "" {
			errDesc := query.Get("error_description")
			errorChan <- fmt.Errorf("authorization error: %s - %s", errCode, errDesc)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><h1>Authorization Failed</h1><p>%s</p><p>You can close this window.</p></body></html>", errDesc)
			return
		}

		receivedState := query.Get("state")
		if receivedState != state {
			errorChan <- fmt.Errorf("state mismatch: expected %s, got %s", state, receivedState)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body><h1>Authorization Failed</h1><p>State mismatch.</p><p>You can close this window.</p></body></html>")
			return
		}

		code := query.Get("code")
		if code == "" {
			errorChan <- fmt.Errorf("no authorization code received")
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body><h1>Authorization Failed</h1><p>No code received.</p><p>You can close this window.</p></body></html>")
			return
		}

		token, err := exchangeCodeForToken(ctx, cfg, code, codeVerifier, redirectURI)
		if err != nil {
			errorChan <- err
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><h1>Authorization Failed</h1><p>%s</p><p>You can close this window.</p></body></html>", err.Error())
			return
		}

		tokenChan <- token
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h1>Authorization Successful</h1><p>You can close this window and return to wtf_cli.</p></body></html>")
	})

	return &PKCEFlowResult{
		AuthURL:      authURL,
		CodeVerifier: codeVerifier,
		State:        state,
		Server:       server,
		TokenChan:    tokenChan,
		ErrorChan:    errorChan,
	}, nil
}

// StartCallbackServer starts the local callback server and waits for the authorization response.
func (r *PKCEFlowResult) StartCallbackServer(ctx context.Context, timeout time.Duration) (*TokenResponse, error) {
	listener, err := net.Listen("tcp", r.Server.Addr)
	if err != nil {
		slog.Debug("pkce_callback_listen_error", "error", err)
		return nil, fmt.Errorf("failed to start callback server: %w", err)
	}

	go func() {
		if err := r.Server.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Debug("pkce_callback_server_error", "error", err)
			r.ErrorChan <- fmt.Errorf("callback server error: %w", err)
		}
	}()

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		r.Server.Shutdown(shutdownCtx)
	}()

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case token := <-r.TokenChan:
		slog.Debug("pkce_callback_success")
		return token, nil
	case err := <-r.ErrorChan:
		slog.Debug("pkce_callback_error", "error", err)
		return nil, err
	case <-timeoutCtx.Done():
		slog.Debug("pkce_callback_timeout")
		return nil, fmt.Errorf("authorization timed out")
	}
}

func exchangeCodeForToken(ctx context.Context, cfg PKCEFlowConfig, code, codeVerifier, redirectURI string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", cfg.ClientID)
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp DeviceFlowError
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			slog.Debug("pkce_token_error", "status", resp.StatusCode, "error", errResp.Error)
			return nil, fmt.Errorf("token error: %s - %s", errResp.Error, errResp.ErrorDescription)
		}
		slog.Debug("pkce_token_error", "status", resp.StatusCode)
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokenResp, nil
}

func generateCodeVerifier() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func generateState() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// OpenAIPKCEFlowConfig returns the PKCE flow config for OpenAI ChatGPT Plus/Pro.
func OpenAIPKCEFlowConfig() PKCEFlowConfig {
	return PKCEFlowConfig{
		ClientID:     "app_EMoamEEZ73f0CkXaXp7hrann",
		AuthURL:      "https://auth.openai.com/oauth/authorize",
		TokenURL:     "https://auth.openai.com/oauth/token",
		RedirectPort: 8484,
		Scopes:       []string{"openid", "profile", "email"},
	}
}
