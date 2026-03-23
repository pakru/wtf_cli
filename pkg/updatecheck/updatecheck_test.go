package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		in   string
		out  string
		good bool
	}{
		{in: "v1.2.3", out: "1.2.3", good: true},
		{in: "1.2", out: "1.2.0", good: true},
		{in: " v0.9.1 ", out: "0.9.1", good: true},
		{in: "v2.0.0-beta.1", out: "2.0.0", good: true},
		{in: "dev", out: "dev", good: true},
		{in: "", out: "", good: false},
		{in: "abc", out: "", good: false},
	}

	for _, tt := range tests {
		got, ok := normalizeVersion(tt.in)
		if ok != tt.good || got != tt.out {
			t.Fatalf("normalizeVersion(%q) = (%q,%v), expected (%q,%v)", tt.in, got, ok, tt.out, tt.good)
		}
	}
}

func TestCheckLatest_FetchesAndCaches(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name":"v1.4.0"}`))
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")
	now := time.Now()
	opts := CheckOptions{
		LatestReleaseURL: srv.URL,
		CachePath:        cachePath,
		Interval:         24 * time.Hour,
		Now: func() time.Time {
			return now
		},
	}

	result, err := CheckLatest(context.Background(), "v1.3.0", opts)
	if err != nil {
		t.Fatalf("CheckLatest() error = %v", err)
	}
	if !result.UpdateAvailable {
		t.Fatalf("expected update available")
	}
	if hits != 1 {
		t.Fatalf("expected 1 HTTP call, got %d", hits)
	}

	result2, err := CheckLatest(context.Background(), "v1.3.0", opts)
	if err != nil {
		t.Fatalf("CheckLatest() cached error = %v", err)
	}
	if !result2.UpdateAvailable {
		t.Fatalf("expected update available from cache")
	}
	if hits != 1 {
		t.Fatalf("expected cached run to skip HTTP call; hits=%d", hits)
	}
}

func TestCheckLatest_DevVersionSkips(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9"}`))
	}))
	defer srv.Close()

	_, err := CheckLatest(context.Background(), "dev", CheckOptions{LatestReleaseURL: srv.URL})
	if err != nil {
		t.Fatalf("expected nil error for dev version, got %v", err)
	}
	if called {
		t.Fatalf("expected no network call for dev version")
	}
}

func TestCheckLatest_InvalidPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name":""}`))
	}))
	defer srv.Close()

	_, err := CheckLatest(context.Background(), "1.0.0", CheckOptions{LatestReleaseURL: srv.URL, CachePath: filepath.Join(t.TempDir(), "cache.json")})
	if err == nil {
		t.Fatalf("expected error for invalid payload")
	}
}
