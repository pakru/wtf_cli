# Fix Password Leak in Normal Terminal Mode

## Problem

When `sudo` prompts for a password in normal terminal mode, password keystrokes are captured in `lineBuffer` and submitted as `last_command` to the LLM. The existing echo-off detection only protects the fullscreen mode path.

## Detection Heuristic

| State | ECHO | ICANON | Detected? |
|-------|------|--------|-----------|
| **sudo password** | OFF | ON (canonical) | ✓ yes |
| **bash/readline** | OFF | OFF (raw) | ✗ no (safe) |

`IsSecretInputMode` checks `ECHO==0 && ICANON!=0`. Covers canonical-mode password prompts (`sudo`, `read -s`, `ssh` passphrase). Won't catch programs using raw mode with echo off.

## Design: Early-Exit in Model.Update

Secret mode is an **early-exit** in `Model.Update`, placed right after `fullScreenMode` — before any overlay can intercept.

### KeyPressMsg flow

```
fullScreenMode? → early return              (line 340)
secretMode?     → send to PTY, return       NEW (after line 350)
overlays / sidebar / inputHandler            (lines 352-424)
```

### PasteMsg flow

```
fullScreenMode? → paste to PTY, return      (line 257)
secretMode?     → paste to PTY, return      NEW (after line 266)
overlays / inputHandler.HandlePaste          (lines 268-336)
```

## Proposed Changes

### [MODIFY] [echostate_linux.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/pty/echostate_linux.go)

Add `IsSecretInputMode()` — ECHO off + ICANON on. Uses `unix.TCGETS`.

### [MODIFY] [echostate_darwin.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/pty/echostate_darwin.go)

Same, using `unix.TIOCGETA`.

### [MODIFY] [echostate_other.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/pty/echostate_other.go)

Stub returning `false`.

---

### [MODIFY] [input.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/input/input.go)

Add `secretMode` field + `SetSecretMode()`/`IsSecretMode()`.

`HandleKey`: early-exit after fullscreen check — `sendKeyToPTY(msg)`, return `(true, nil)`.

`HandlePaste`: early-exit — send to PTY with bracketed paste if enabled, skip `lineBuffer`.

---

### [MODIFY] [model.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/model.go)

Add injectable detector field for testability:

```go
type Model struct {
    ...
    // secretDetector checks PTY for secret input mode. Injectable for testing.
    secretDetector func(*os.File) bool
}
```

Default in `NewModel`:

```go
secretDetector: pty.IsSecretInputMode,
```

**KeyPressMsg** — insert after `fullScreenMode` block (after line 350):

```go
if m.ptyFile != nil && m.secretDetector(m.ptyFile) {
    m.inputHandler.SetSecretMode(true)
    handled, cmd := m.inputHandler.HandleKey(msg)
    if handled {
        return m, cmd
    }
    return m, nil
}
m.inputHandler.SetSecretMode(false)
```

**PasteMsg** — insert after `fullScreenMode` block (after line 266):

```go
if m.ptyFile != nil && m.secretDetector(m.ptyFile) {
    m.inputHandler.SetSecretMode(true)
    m.inputHandler.HandlePaste(msg.Content)
    return m, nil
}
m.inputHandler.SetSecretMode(false)
```

---

### Tests

#### [MODIFY] [echostate_test.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/pty/echostate_test.go)

1. Nil/invalid/closed FD tests for `IsSecretInputMode`.
2. Real-PTY termios toggle test: ECHO off + ICANON on → `true`, other combos → `false`.

#### [MODIFY] [input_test.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/input/input_test.go)

1. Secret mode `/` → no palette, PTY receives `/`.
2. Secret mode Ctrl+R → no history picker.
3. Secret mode Enter → no `CommandSubmittedMsg`.
4. Secret mode paste → `lineBuffer` empty.

#### [MODIFY] [model_test.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/model_test.go)

Using injectable `secretDetector` set to `func(*os.File) bool { return true }`:

1. **Routing precedence**: palette open + secret detector returns true → key goes to PTY, not palette.
2. **Normal regression**: secret detector returns false → `CommandSubmittedMsg` still captured normally.

## Verification

```bash
cd /home/dev/project/wtf_cli/wtf_cli && go test ./...
```

> [!IMPORTANT]
> Manual test with `sudo` required for final validation.
