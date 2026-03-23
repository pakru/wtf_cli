package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLatestReleaseURL = "https://api.github.com/repos/pakru/wtf_cli/releases/latest"
	releasePageURL          = "https://github.com/pakru/wtf_cli/releases"
	upgradeCommand          = "curl -fsSL https://raw.githubusercontent.com/pakru/wtf_cli/main/install.sh | bash"
)

type Result struct {
	CurrentVersion  string
	LatestVersion   string
	ReleaseURL      string
	UpgradeCommand  string
	UpdateAvailable bool
}

type CheckOptions struct {
	LatestReleaseURL string
	CachePath        string
	Interval         time.Duration
	HTTPClient       *http.Client
	Now              func() time.Time
}

type latestReleaseResponse struct {
	TagName string `json:"tag_name"`
}

type cacheState struct {
	LastChecked   time.Time `json:"last_checked"`
	LatestVersion string    `json:"latest_version"`
}

func DefaultCachePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return filepath.Join(".wtf_cli", "update_check_cache.json")
	}
	return filepath.Join(homeDir, ".wtf_cli", "update_check_cache.json")
}

func CheckLatest(ctx context.Context, currentVersion string, opts CheckOptions) (Result, error) {
	result := Result{
		CurrentVersion: strings.TrimSpace(currentVersion),
		ReleaseURL:     releasePageURL,
		UpgradeCommand: upgradeCommand,
	}
	current, ok := normalizeVersion(result.CurrentVersion)
	if !ok || current == "dev" {
		return result, nil
	}

	cachePath := strings.TrimSpace(opts.CachePath)
	if cachePath == "" {
		cachePath = DefaultCachePath()
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}

	if cached, ok := readCache(cachePath); ok && nowFn().Sub(cached.LastChecked) < interval {
		result.LatestVersion = cached.LatestVersion
		latest, parsed := normalizeVersion(cached.LatestVersion)
		if parsed {
			result.UpdateAvailable = compareVersions(current, latest) < 0
		}
		return result, nil
	}

	latest, err := fetchLatestVersion(ctx, opts)
	if err != nil {
		return result, err
	}
	result.LatestVersion = latest

	normalizedLatest, ok := normalizeVersion(latest)
	if !ok {
		return result, fmt.Errorf("invalid latest release version: %q", latest)
	}
	result.UpdateAvailable = compareVersions(current, normalizedLatest) < 0

	_ = writeCache(cachePath, cacheState{LastChecked: nowFn(), LatestVersion: latest})

	return result, nil
}

func fetchLatestVersion(ctx context.Context, opts CheckOptions) (string, error) {
	url := strings.TrimSpace(opts.LatestReleaseURL)
	if url == "" {
		url = defaultLatestReleaseURL
	}

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("latest release status: %d", resp.StatusCode)
	}

	var payload latestReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode latest release payload: %w", err)
	}
	if strings.TrimSpace(payload.TagName) == "" {
		return "", errors.New("empty tag_name in latest release payload")
	}

	return strings.TrimSpace(payload.TagName), nil
}

func normalizeVersion(input string) (string, bool) {
	v := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), "v"))
	if v == "" {
		return "", false
	}
	if strings.EqualFold(v, "dev") {
		return "dev", true
	}

	base := strings.SplitN(v, "-", 2)[0]
	parts := strings.Split(base, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return "", false
	}
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return "", false
		}
	}
	return strings.Join(parts, "."), true
}

func compareVersions(current, latest string) int {
	curParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")
	for i := range 3 {
		curN, _ := strconv.Atoi(curParts[i])
		latestN, _ := strconv.Atoi(latestParts[i])
		if curN < latestN {
			return -1
		}
		if curN > latestN {
			return 1
		}
	}
	return 0
}

func readCache(path string) (cacheState, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cacheState{}, false
	}
	var cache cacheState
	if err := json.Unmarshal(data, &cache); err != nil {
		return cacheState{}, false
	}
	if cache.LastChecked.IsZero() || strings.TrimSpace(cache.LatestVersion) == "" {
		return cacheState{}, false
	}
	return cache, true
}

func writeCache(path string, cache cacheState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
