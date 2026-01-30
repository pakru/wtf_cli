# Host Platform Info Enhancement

Add host platform information to the system prompt so the AI assistant knows what OS environment the user is working in.

## Goal

Enhance `wtfSystemPrompt()` to include:
- **Linux**: Distribution name, kernel version, architecture
- **macOS**: macOS version, architecture
- **Windows** (future): Windows version

## Proposed Changes

### pkg/ai

#### [NEW] [platform.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ai/platform.go)

New file with platform detection logic:
- `GetPlatformInfo() PlatformInfo` - returns cached platform info
- Uses `runtime.GOOS` and `runtime.GOARCH` as baseline

**Linux distro detection:**
1. Read `/etc/os-release` (primary)
2. Fallback to `/usr/lib/os-release` if `/etc/os-release` missing
3. Parse using shell-style syntax: `KEY=value` or `KEY="quoted value"`
4. Handle double-quoted values with backslash escapes (`\"`, `\\`, `\$`, etc.)
5. Use `PRETTY_NAME` field for display

**Linux kernel version:**
- Read `/proc/sys/kernel/osrelease` (fast, no process spawn)
- Fallback to `uname -r` only if proc read fails

**macOS version:**
- Run `sw_vers -productVersion` for version
- Cache result since it doesn't change during runtime

```go
type PlatformInfo struct {
    OS      string // "linux", "darwin", "windows"
    Arch    string // "amd64", "arm64"
    Distro  string // Linux only: "Ubuntu 22.04.3 LTS" (from PRETTY_NAME)
    Kernel  string // Linux only: "6.5.0-44-generic"
    Version string // macOS only: "14.2.1"
}

func (p PlatformInfo) PromptText() string {
    // Returns formatted string for system prompt
    // Never returns empty; falls back to "The user is on <OS> (<arch>)."
}
```

**Caching:**
- Global `var platformCache *PlatformInfo` with `sync.Once`
- Expose `ResetPlatformCache()` for testing (unexported or test-only)

---

#### [MODIFY] [context.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ai/context.go)

Update `wtfSystemPrompt()` to include platform info:

```diff
 func wtfSystemPrompt() string {
+    platform := GetPlatformInfo()
     return strings.Join([]string{
         "You are a terminal assistant.",
+        platform.PromptText(), // Always non-empty
         "Use the provided terminal output..."
     }, " ")
 }
```

---

### pkg/ai

#### [NEW] [platform_test.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ai/platform_test.go)

Tests using **mock-based injection** (primary strategy for CI consistency):

**Pure unit tests:**
- `TestPlatformInfo_PromptText` - verify formatted output for various inputs
- `TestPlatformInfo_PromptText_Fallback` - verify non-empty fallback when fields missing
- `TestParseOsRelease` - verify parsing: unquoted, quoted, escapes
- `TestParseOsRelease_MissingPrettyName` - verify fallback to NAME + VERSION

**Mock-based integration tests:**
- Inject `FileReader` interface to provide fake os-release content
- Call `ResetPlatformCache()` before each test to clear cached state
- `TestGetPlatformInfo_Linux` - mock `/etc/os-release` and `/proc/sys/kernel/osrelease`
- `TestGetPlatformInfo_Fallback` - mock missing `/etc/os-release`, uses `/usr/lib/os-release`

## Verification Plan

### Automated Tests
```bash
go test -v ./pkg/ai/... -run TestPlatformInfo
go test -v ./pkg/ai/... -run TestParseOsRelease
```

### Manual Verification
1. **Enable trace logging** to see system prompt content
2. Run `wtf_cli` and trigger `/explain`
3. Check logs for system prompt content
4. Verify platform info appears correctly in AI context

## Example Output

**Linux:**
```
The user is on Ubuntu 22.04.3 LTS (Linux 6.5.0-44-generic, amd64).
```

**macOS:**
```
The user is on macOS 14.2.1 (arm64).
```

**Unknown/minimal:**
```
The user is on linux (amd64).
```

