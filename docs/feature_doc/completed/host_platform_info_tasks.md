# Host Platform Info - Implementation Tasks

## Phase 1: Create Core Platform Detection

- [x] Create `pkg/ai/platform.go`
  - [x] Define `PlatformInfo` struct (OS, Arch, Distro, Kernel, Version)
  - [x] Implement `ParseOsRelease(content string) map[string]string`
  - [x] Implement `readOsRelease()` with `/etc/os-release` → `/usr/lib/os-release` fallback
  - [x] Implement `readKernelVersion()` using `/proc/sys/kernel/osrelease` with `uname -r` fallback
  - [x] Implement `readMacOSVersion()` using `sw_vers -productVersion`
  - [x] Implement `GetPlatformInfo() PlatformInfo` with caching (`sync.Once`)
  - [x] Implement `PromptText()` method with non-empty fallback
  - [x] Add `ResetPlatformCache()` for test use

## Phase 2: Integrate with System Prompt

- [x] Modify `pkg/ai/context.go`
  - [x] Update `wtfSystemPrompt()` to call `GetPlatformInfo().PromptText()`
  - [x] Ensure platform info is included in the system prompt string

## Phase 3: Write Tests

- [x] Create `pkg/ai/platform_test.go`
  - [x] `TestParseOsRelease` - unquoted values
  - [x] `TestParseOsRelease_Quoted` - quoted values with escapes
  - [x] `TestParseOsRelease_MissingPrettyName` - fallback to NAME + VERSION
  - [x] `TestPlatformInfo_PromptText_Linux` - formatted Linux output
  - [x] `TestPlatformInfo_PromptText_MacOS` - formatted macOS output
  - [x] `TestPlatformInfo_PromptText_Fallback` - minimal/unknown fallback
  - [x] `TestGetPlatformInfo_Basic` - verify OS/Arch populated

## Phase 4: Verification

- [ ] Run all tests: `go test -v ./pkg/ai/...`
- [ ] Build: `go build ./...`
- [ ] Manual test: run `wtf_cli`, trigger `/explain`, check logs for platform info
