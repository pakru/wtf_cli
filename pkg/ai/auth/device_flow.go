package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeviceFlowConfig holds configuration for OAuth 2.0 Device Authorization Flow.
type DeviceFlowConfig struct {
	ClientID      string
	DeviceCodeURL string
	TokenURL      string
	Scopes        []string
}

// DeviceCodeResponse is the response from the device authorization endpoint.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse is the response from the token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// DeviceFlowError represents an OAuth error response.
type DeviceFlowError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// StartDeviceFlow initiates the device authorization flow.
func StartDeviceFlow(ctx context.Context, cfg DeviceFlowConfig) (*DeviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", cfg.ClientID)
	if len(cfg.Scopes) > 0 {
		data.Set("scope", strings.Join(cfg.Scopes, " "))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.DeviceCodeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request device code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read device code response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp DeviceFlowError
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("device code error: %s - %s", errResp.Error, errResp.ErrorDescription)
		}
		return nil, fmt.Errorf("device code request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var dcResp DeviceCodeResponse
	if err := json.Unmarshal(body, &dcResp); err != nil {
		return nil, fmt.Errorf("failed to parse device code response: %w", err)
	}

	if dcResp.Interval == 0 {
		dcResp.Interval = 5
	}

	return &dcResp, nil
}

// PollForToken polls the token endpoint until authorization is complete or times out.
func PollForToken(ctx context.Context, cfg DeviceFlowConfig, deviceCode string, interval int) (*TokenResponse, error) {
	if interval < 1 {
		interval = 5
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			token, err := requestToken(ctx, cfg, deviceCode)
			if err == nil {
				return token, nil
			}

			if isPendingError(err) {
				continue
			}

			if isSlowDownError(err) {
				ticker.Reset(time.Duration(interval+5) * time.Second)
				continue
			}

			return nil, err
		}
	}
}

func requestToken(ctx context.Context, cfg DeviceFlowConfig, deviceCode string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", cfg.ClientID)
	data.Set("device_code", deviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

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
			return nil, &oauthError{code: errResp.Error, description: errResp.ErrorDescription}
		}
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokenResp, nil
}

type oauthError struct {
	code        string
	description string
}

func (e *oauthError) Error() string {
	if e.description != "" {
		return fmt.Sprintf("%s: %s", e.code, e.description)
	}
	return e.code
}

func isPendingError(err error) bool {
	if oauthErr, ok := err.(*oauthError); ok {
		return oauthErr.code == "authorization_pending"
	}
	return false
}

func isSlowDownError(err error) bool {
	if oauthErr, ok := err.(*oauthError); ok {
		return oauthErr.code == "slow_down"
	}
	return false
}

// GitHubCopilotDeviceFlowConfig returns the device flow config for GitHub Copilot.
func GitHubCopilotDeviceFlowConfig() DeviceFlowConfig {
	return DeviceFlowConfig{
		ClientID:      "Iv1.b507a08c87ecfe98",
		DeviceCodeURL: "https://github.com/login/device/code",
		TokenURL:      "https://github.com/login/oauth/access_token",
		Scopes:        []string{"read:user"},
	}
}
