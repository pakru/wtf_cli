package auth

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StoredCredentials holds authentication credentials for a provider.
type StoredCredentials struct {
	Provider     string    `json:"provider"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

// IsExpired returns true if the credentials have expired.
func (c *StoredCredentials) IsExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(c.ExpiresAt)
}

// IsExpiringSoon returns true if credentials expire within the given duration.
func (c *StoredCredentials) IsExpiringSoon(within time.Duration) bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().Add(within).After(c.ExpiresAt)
}

// authStore is the on-disk format for storing credentials.
type authStore struct {
	Credentials map[string]StoredCredentials `json:"credentials"`
}

// AuthManager handles secure storage and retrieval of provider credentials.
type AuthManager struct {
	configPath string
	mu         sync.RWMutex
}

// NewAuthManager creates a new AuthManager with the given config path.
func NewAuthManager(configPath string) *AuthManager {
	return &AuthManager{
		configPath: configPath,
	}
}

// DefaultAuthPath returns the default path for auth.json.
func DefaultAuthPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".wtf_cli", "auth.json")
	}
	return filepath.Join(homeDir, ".wtf_cli", "auth.json")
}

// Save stores credentials for a provider.
func (m *AuthManager) Save(creds StoredCredentials) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	store, err := m.loadStore()
	if err != nil {
		store = &authStore{Credentials: make(map[string]StoredCredentials)}
	}

	if store.Credentials == nil {
		store.Credentials = make(map[string]StoredCredentials)
	}

	store.Credentials[creds.Provider] = creds

	slog.Debug("auth_save",
		"provider", creds.Provider,
		"path", m.configPath,
		"has_refresh", creds.RefreshToken != "",
		"expires_at_set", !creds.ExpiresAt.IsZero(),
	)
	return m.saveStore(store)
}

// Load retrieves credentials for a provider.
func (m *AuthManager) Load(provider string) (*StoredCredentials, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	store, err := m.loadStore()
	if err != nil {
		slog.Debug("auth_load_error", "provider", provider, "error", err)
		return nil, err
	}

	creds, ok := store.Credentials[provider]
	if !ok {
		slog.Debug("auth_load_missing", "provider", provider)
		return nil, fmt.Errorf("no credentials found for provider: %s", provider)
	}

	slog.Debug("auth_load",
		"provider", provider,
		"expires_at_set", !creds.ExpiresAt.IsZero(),
		"expired", creds.IsExpired(),
	)
	return &creds, nil
}

// Delete removes credentials for a provider.
func (m *AuthManager) Delete(provider string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	store, err := m.loadStore()
	if err != nil {
		slog.Debug("auth_delete_error", "provider", provider, "error", err)
		return err
	}

	delete(store.Credentials, provider)

	slog.Debug("auth_delete", "provider", provider, "path", m.configPath)
	return m.saveStore(store)
}

// HasCredentials checks if credentials exist for a provider.
func (m *AuthManager) HasCredentials(provider string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	store, err := m.loadStore()
	if err != nil {
		return false
	}

	_, ok := store.Credentials[provider]
	return ok
}

// ListProviders returns a list of providers with stored credentials.
func (m *AuthManager) ListProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	store, err := m.loadStore()
	if err != nil {
		slog.Debug("auth_list_error", "error", err)
		return nil
	}

	providers := make([]string, 0, len(store.Credentials))
	for p := range store.Credentials {
		providers = append(providers, p)
	}
	return providers
}

// loadStore reads the auth store from disk.
func (m *AuthManager) loadStore() (*authStore, error) {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &authStore{Credentials: make(map[string]StoredCredentials)}, nil
		}
		return nil, fmt.Errorf("failed to read auth file: %w", err)
	}

	var store authStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse auth file: %w", err)
	}

	if store.Credentials == nil {
		store.Credentials = make(map[string]StoredCredentials)
	}

	return &store, nil
}

// saveStore writes the auth store to disk with secure permissions.
func (m *AuthManager) saveStore(store *authStore) error {
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create auth directory: %w", err)
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal auth data: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write auth file: %w", err)
	}

	return nil
}
