package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuthManager_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	authPath := filepath.Join(tmpDir, "auth.json")
	manager := NewAuthManager(authPath)

	creds := StoredCredentials{
		Provider:     "test-provider",
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	if err := manager.Save(creds); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := manager.Load("test-provider")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Provider != creds.Provider {
		t.Errorf("Provider mismatch: got %s, want %s", loaded.Provider, creds.Provider)
	}
	if loaded.AccessToken != creds.AccessToken {
		t.Errorf("AccessToken mismatch: got %s, want %s", loaded.AccessToken, creds.AccessToken)
	}
	if loaded.RefreshToken != creds.RefreshToken {
		t.Errorf("RefreshToken mismatch: got %s, want %s", loaded.RefreshToken, creds.RefreshToken)
	}
}

func TestAuthManager_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	authPath := filepath.Join(tmpDir, "auth.json")
	manager := NewAuthManager(authPath)

	creds := StoredCredentials{
		Provider:    "test-provider",
		AccessToken: "test-token",
	}

	if err := manager.Save(creds); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := manager.Delete("test-provider"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := manager.Load("test-provider")
	if err == nil {
		t.Error("Expected error loading deleted credentials")
	}
}

func TestAuthManager_HasCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	authPath := filepath.Join(tmpDir, "auth.json")
	manager := NewAuthManager(authPath)

	if manager.HasCredentials("nonexistent") {
		t.Error("HasCredentials should return false for nonexistent provider")
	}

	creds := StoredCredentials{
		Provider:    "test-provider",
		AccessToken: "test-token",
	}

	if err := manager.Save(creds); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if !manager.HasCredentials("test-provider") {
		t.Error("HasCredentials should return true for existing provider")
	}
}

func TestAuthManager_ListProviders(t *testing.T) {
	tmpDir := t.TempDir()
	authPath := filepath.Join(tmpDir, "auth.json")
	manager := NewAuthManager(authPath)

	providers := manager.ListProviders()
	if len(providers) != 0 {
		t.Errorf("Expected empty list, got %v", providers)
	}

	manager.Save(StoredCredentials{Provider: "provider1", AccessToken: "token1"})
	manager.Save(StoredCredentials{Provider: "provider2", AccessToken: "token2"})

	providers = manager.ListProviders()
	if len(providers) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(providers))
	}
}

func TestStoredCredentials_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{"zero time", time.Time{}, false},
		{"future", time.Now().Add(time.Hour), false},
		{"past", time.Now().Add(-time.Hour), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := StoredCredentials{ExpiresAt: tt.expiresAt}
			if got := creds.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStoredCredentials_IsExpiringSoon(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		within    time.Duration
		want      bool
	}{
		{"zero time", time.Time{}, time.Hour, false},
		{"expires in 30min, check 1hr", time.Now().Add(30 * time.Minute), time.Hour, true},
		{"expires in 2hr, check 1hr", time.Now().Add(2 * time.Hour), time.Hour, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := StoredCredentials{ExpiresAt: tt.expiresAt}
			if got := creds.IsExpiringSoon(tt.within); got != tt.want {
				t.Errorf("IsExpiringSoon() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthManager_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	authPath := filepath.Join(tmpDir, "auth.json")
	manager := NewAuthManager(authPath)

	creds := StoredCredentials{
		Provider:    "test-provider",
		AccessToken: "test-token",
	}

	if err := manager.Save(creds); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	info, err := os.Stat(authPath)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("Expected permissions 0600, got %o", perm)
	}
}
