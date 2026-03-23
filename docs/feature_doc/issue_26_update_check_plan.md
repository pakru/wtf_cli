# Issue #26 Implementation Plan: Startup New Version Notification

## Issue Summary
Add a non-blocking startup enhancement that checks whether a newer `wtf_cli` release is available and, when true, shows a notification under the welcome shortcuts section. The notice should include:

- current running version
- latest available version
- link to Releases page
- one-line upgrade/install command from README

The check must **not impact bootstrap/startup responsiveness**.

---

## Goals and Non-Goals

### Goals
1. Run release check asynchronously during startup (no UI thread blocking).
2. Show update notification only when a newer version exists.
3. Keep startup robust when offline / rate-limited / API errors occur.
4. Provide config + caching controls to reduce repeated network calls.
5. Add test coverage for compare logic, model integration, and welcome rendering.

### Non-Goals
- Auto-updating binaries in-app.
- Mandatory update prompts.
- Introducing any startup hard-failure path due to update check errors.

---

## Proposed Design

### 1) New package: `pkg/updatecheck`
Create a focused service package for release lookup and version comparison.

**Proposed API**

```go
type Result struct {
    CurrentVersion  string
    LatestVersion   string
    ReleaseURL      string
    UpgradeCommand  string
    UpdateAvailable bool
}

func CheckLatest(ctx context.Context, currentVersion string) (Result, error)
```

**Behavior**
- Query GitHub latest release endpoint:
  - `https://api.github.com/repos/pakru/wtf_cli/releases/latest`
- Parse `tag_name` and normalize versions (`v0.3.0` vs `0.3.0`).
- Compare semver-like values; if parse fails, return no-update + error.
- Skip check for local/dev builds (`version.Version == "dev"` or empty).
- Always include canonical links/command in result:
  - Releases: `https://github.com/pakru/wtf_cli/releases`
  - Upgrade: `curl -fsSL https://raw.githubusercontent.com/pakru/wtf_cli/main/install.sh | bash`

**Failure policy**
- Never panic.
- On check failures, only write a warning log and continue silently (no user-facing error state).

---

### 2) Bubble Tea integration in `pkg/ui/model.go`
Use the existing async command/message architecture.

**Additions**
- New message type:

```go
type updateCheckMsg struct {
    Result updatecheck.Result
    Err    error
}
```

- New command:

```go
func fetchUpdateCheckCmd() tea.Cmd
```

This command should:
- execute with short timeout (e.g., 2–5s)
- call `updatecheck.CheckLatest(...)`
- return `updateCheckMsg`

**Startup wiring**
- Add to `Model.Init()` via `tea.Batch(...)` alongside existing startup commands.
- Ensure async network call never blocks PTY/UI startup.

**State handling**
- Add model field(s) for update notice payload and whether applied.
- On `updateCheckMsg` with `UpdateAvailable=true`, inject/update welcome notice section.
- On no-update/error: no visible notification; log at debug/info.

---

### 3) Welcome component extension
Current `WelcomeMessage()` is static. Extend it to optionally include update content.

**Option A (preferred):**
- Keep current API for compatibility:
  - `WelcomeMessage() string`
- Add:
  - `WelcomeMessageWithUpdate(info *UpdateNotice) string`

Where `UpdateNotice` contains:
- current version
- latest version
- release URL
- upgrade command

**Rendering requirements**
- Place section **under shortcut hints**.
- Preserve box layout/width.
- Truncate long lines safely with existing width utilities.
- Keep style consistent using existing `pkg/ui/styles` tokens.

---

### 4) Config + cache controls
To avoid unnecessary startup calls and allow opt-out, extend config.

**New config block**

```json
"update_check": {
  "enabled": true,
  "interval_hours": 24
}
```

**Implementation changes (`pkg/config/config.go`)**
- Add `UpdateCheckConfig` struct.
- Add field to `Config`.
- Add defaults in `Default()`.
- Extend `configPresence` + `applyDefaults()`.
- Validate `interval_hours > 0` (or clamp to default).

**Cache strategy**
- Store a lightweight cache file in `~/.wtf_cli/` with:
  - last check timestamp
  - last known latest tag
- Skip network call when:
  - disabled in config
  - interval not elapsed
  - running dev build

---

### 5) Logging and observability
Add structured logs for diagnosability without noise.

Suggested events:
- `update_check_start`
- `update_check_skipped` (`reason`: disabled/dev/interval)
- `update_check_success` (`current`, `latest`, `update_available`)
- `update_check_error` (`error`)

---

## Test Plan

### Unit tests (`pkg/updatecheck`)
1. `tag_name` parsing + normalization with/without `v` prefix.
2. Version compare cases:
   - current < latest
   - current == latest
   - current > latest
   - invalid versions
3. HTTP failures, malformed payload, timeout handling.
4. Dev-build skip behavior.

### UI component tests (`pkg/ui/components/welcome`)
1. Update section appears when notice exists.
2. Update section absent when notice is nil.
3. Strings included:
   - current version
   - latest version
   - releases URL
   - upgrade curl command

### Model tests (`pkg/ui/model_test.go`)
1. `updateCheckMsg` success with update toggles notice state.
2. No-update and error paths do not break normal rendering.
3. Startup command batching still initializes PTY listener + directory ticks.

### Golden tests
- If welcome rendering changes visible output, regenerate/update corresponding golden files.

---

## Rollout Strategy

1. Implement updatecheck package and unit tests.
2. Integrate async command/message in model.
3. Extend welcome component and tests.
4. Add config + cache controls.
5. Run `make check` and update golden files if necessary.
6. Open PR with screenshots optional (not required unless additional visual TUI changes beyond welcome text formatting need explicit review).

---

## Risks and Mitigations

1. **Startup slowdown**
   - Mitigation: async command + short timeout + interval cache.
2. **GitHub API rate limits/network errors**
   - Mitigation: silent fallback, log only, no user disruption.
3. **Version parsing edge cases**
   - Mitigation: strict normalization tests + conservative fallback.
4. **Welcome layout overflow**
   - Mitigation: truncation/wrapping tests at fixed box width.

---

## Acceptance Criteria

1. On startup, when a newer release exists, the welcome section shows:
   - current version
   - latest version
   - releases URL
   - install/upgrade curl command
2. On no-update/error/offline, no crash and no startup delay regressions.
3. Check is configurable and can be disabled.
4. Repeated launches within interval do not hit network.
5. Test suite and golden outputs pass via project checks.
