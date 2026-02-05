# Fix Zsh History Sync on Mac

This bug occurs because `wtf_cli` only reads from `.bash_history` and doesn't support zsh, which is the default shell on macOS.

## Root Cause Analysis

| Issue | Current Behavior | Required Fix |
|-------|-----------------|--------------|
| Wrong history file | Hardcoded fallback to `~/.bash_history` | Detect shell and use `~/.zsh_history` for zsh |
| Zsh format not parsed | Only skips lines starting with `#` | Parse zsh extended format `: timestamp:0;command` |
| Function name | `ReadBashHistory` implies bash-only | Rename to `ReadShellHistory` |

## Proposed Changes

### Shell History Component

#### [MODIFY] [bash_history.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/capture/bash_history.go)

1. **Add shell detection logic**:
   - Check `$SHELL` environment variable
   - Check OS platform (`runtime.GOOS`) - macOS defaults to zsh, Linux to bash
   - Combine both signals for robust detection

2. **Update history file resolution**:
   ```go
   // New priority:
   // 1. $HISTFILE (explicit override)
   // 2. ~/.zsh_history (if shell is zsh OR macOS detected)
   // 3. ~/.bash_history (fallback)
   ```

3. **Add zsh extended history format parsing**:
   ```diff
   -// Skip bash timestamps (lines starting with #)
   -if strings.HasPrefix(line, "#") {
   +// Skip bash timestamps (lines starting with #) and parse zsh extended format
   +if strings.HasPrefix(line, "#") {
   +    continue
   +}
   +// Handle zsh extended history format: ": timestamp:0;command"
   +if strings.HasPrefix(line, ": ") {
   +    if idx := strings.Index(line, ";"); idx != -1 {
   +        line = line[idx+1:]
   +    } else {
   +        continue
   +    }
   +}
   ```

4. **Optional**: Rename `ReadBashHistory` → `ReadShellHistory` (with alias for backward compat)

---

#### [MODIFY] [bash_history_test.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/capture/bash_history_test.go)

Add new test case for zsh extended history format:

```go
func TestReadShellHistory_ZshExtendedFormat(t *testing.T) {
    // Test parsing of zsh extended history: ": timestamp:0;command"
}
```

## Verification Plan

### Automated Tests

Run the existing and new tests:

```bash
cd /home/dev/project/wtf_cli/wtf_cli && go test -v ./pkg/capture/... -run History
```

### Manual Verification

Since you're experiencing this on Mac with zsh:

1. Build and run `wtf_cli`
2. Press `Ctrl+R` to open history picker
3. Verify your recent zsh commands appear in the list
