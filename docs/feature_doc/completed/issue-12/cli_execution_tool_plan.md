# Add CLI Execution Tool to WTF Analysis Chat (Issue #12)

Enable users to select and execute CLI commands suggested by the LLM directly from the chat sidebar. Commands are always visually highlighted. Up/down scrolling integrates command tracking — the command nearest to the viewport center is auto-selected. **Ctrl+Enter** pastes the selected command into the PTY (overriding existing input) and switches focus to the terminal.

---

## Design Decisions

- **Command marker**: `<cmd>...</cmd>` tags in LLM output (custom protocol, low false-positive)
- **No separate mode**: commands integrate into existing up/down scroll — no Tab toggle
- **Always highlighted**: commands in chat are styled with bright color + underline
- **Active command**: as user scrolls, the command closest to viewport center auto-selects
- **Ctrl+Enter**: pastes selected command to PTY, clears existing input (Ctrl+U), switches focus to terminal
- **No auto-execution**: user must press Enter in terminal to run (future enhancement candidate)

---

## Proposed Changes

### AI System Prompt — Global

#### [MODIFY] [context.go](file:///home/pavel/STORAGE/Projects/my_projects/wtf_cli/pkg/ai/context.go)

Add `<cmd>` instruction to the shared `wtfSystemPrompt()` so both `/chat` and `/explain` can return runnable commands with markers:

```go
...
"When suggesting CLI commands the user can run, wrap each command in <cmd>...</cmd> tags, e.g. <cmd>ls -la</cmd>. Only wrap safe, single-line shell commands. Do not wrap multi-line scripts, code snippets, or explanations.",
...
```

---

### Command Parser

#### [NEW] [cmdparse.go](file:///home/pavel/STORAGE/Projects/my_projects/wtf_cli/pkg/ui/components/sidebar/cmdparse.go)

- `CommandEntry` struct — `Command string`, `SourceIndex int` (index in raw content)
- `ExtractCommands(content string) []CommandEntry` — parses `<cmd>...</cmd>` tags
- `StripCommandMarkers(content string) string` — removes tags for display, preserving command text
- `SanitizeCommand(cmd string) (string, bool)` — trims whitespace, returns `("", false)` if empty or contains `\n`/`\r` (rejects multiline — stripping could silently change command meaning)

#### [NEW] [cmdparse_test.go](file:///home/pavel/STORAGE/Projects/my_projects/wtf_cli/pkg/ui/components/sidebar/cmdparse_test.go)

Tests: single/multiple commands, no commands, malformed tags, strip markers, sanitize (empty, multiline rejected, whitespace-only).

---

### Sidebar Integration

#### [MODIFY] [sidebar.go](file:///home/pavel/STORAGE/Projects/my_projects/wtf_cli/pkg/ui/components/sidebar/sidebar.go)

**New fields** on `Sidebar`:
- `cmdSelectedIdx int` — active command index (-1 = none)
- `cmdList []CommandEntry` — extracted from all assistant messages
- `cmdRenderedLines []int` — rendered line indices of commands (tracked during render, not post-hoc)

**Command-line tracking during render** (fixes brittle post-hoc matching):
Modify `renderMarkdown()` / `reflow()` to track command lines **during rendering**, before ANSI styling is applied. When `StripCommandMarkers()` processes content, record which source lines contained commands. Map these to rendered line indices as part of the reflow pipeline. Store in `cmdRenderedLines[]`.

**Lazy command extraction** (avoids per-tick parsing during streaming):
Do **not** run `ExtractCommands()` on every `reflow()` call — `reflow()` runs on every throttled stream tick. Instead, use a `cmdDirty bool` flag set when messages change (in `UpdateLastMessage`, `StartAssistantMessage`, etc.). `RefreshCommands()` checks the flag and only re-parses when dirty. This keeps long chats performant.

**Active command initialization**: Call `updateActiveCommand()` in three places: (1) `handleScroll()` after updating `scrollY`, (2) end of `reflow()` after lines are recalculated, (3) `SetSize()` after resize. This ensures Ctrl+Enter always has a valid active command without requiring the user to scroll first.

**Scroll integration**: `updateActiveCommand()` sets `cmdSelectedIdx` to the command whose rendered line is closest to the viewport center.

**Key handling**: Add `ctrl+enter` to `ShouldHandleKey()` and `Update()`. When pressed with active command (`cmdSelectedIdx >= 0`), emit `CommandExecuteMsg`.
`Ctrl+Enter` is scoped to chat-focused routing only: it is handled when the sidebar is visible and terminal is not focused (`terminalFocused == false`).

**Rendering**: Command lines get `CommandStyle` (bright + underline). The active command line gets `CommandActiveStyle` (distinct from `CommandStyle`). Both style names are consistent.

**Footer hint with layout math**: Add a conditional footer line when commands exist: `Ctrl+Enter Apply Command`. Update constants:
- `borderLines` in `renderChatView()`: conditionally +1 when footer is shown
- `maxScroll()`: adjust the viewport height subtraction accordingly (currently hard-coded to -5)

**New message**: `CommandExecuteMsg { Command string }`

---

### Model Integration

#### [MODIFY] [model.go](file:///home/pavel/STORAGE/Projects/my_projects/wtf_cli/pkg/ui/model.go)

Handle `CommandExecuteMsg` with runtime safety guards:
```go
case sidebar.CommandExecuteMsg:
    cmd, ok := sidebar.SanitizeCommand(msg.Command)
    if !ok {
        return m, nil
    }
    if m.inputHandler != nil {
        m.inputHandler.SendToPTY([]byte{21}) // Ctrl+U clears existing input
        m.inputHandler.SendToPTY([]byte(cmd))
        m.inputHandler.SetLineBuffer(cmd)
    }
    m.setTerminalFocused(true)
    return m, nil
```

No extra `RefreshCommands()` call needed on `msg.Done` — `RefreshView()` already handles it through `reflow()`.

---

### Styles

#### [MODIFY] [theme.go](file:///home/pavel/STORAGE/Projects/my_projects/wtf_cli/pkg/ui/styles/theme.go)

- `CommandStyle` — bright foreground + underline (all command lines)
- `CommandActiveStyle` — brighter/accent background (the selected command)

---

### Docs

#### [MODIFY] [handlers.go](file:///home/pavel/STORAGE/Projects/my_projects/wtf_cli/pkg/commands/handlers.go)

Update shortcut docs to include `Ctrl+Enter` for applying chat commands.

#### [MODIFY] [README.md](file:///home/pavel/STORAGE/Projects/my_projects/wtf_cli/README.md)

Add `Ctrl+Enter` to keyboard shortcuts section.

---

## Verification Plan

### Automated Tests
```bash
# Command parser tests
go test ./pkg/ui/components/sidebar/ -run TestExtractCommands -v
go test ./pkg/ui/components/sidebar/ -run TestStripCommandMarkers -v
go test ./pkg/ui/components/sidebar/ -run TestSanitizeCommand -v

# Sidebar integration tests
go test ./pkg/ui/components/sidebar/ -v

# Model routing test (CommandExecuteMsg handling)
go test ./pkg/ui/ -run TestCommandExecute -v

# Full suite
make check
```

### Manual Verification
1. `make run` → `/chat` → ask for CLI suggestions
2. Verify commands have bright + underline styling
3. Scroll — nearest command auto-highlights
4. **Ctrl+Enter** → command pasted, focus on terminal, existing text cleared
5. Verify footer hint appears when commands present
6. Verify `/explain` can also produce `<cmd>` tags for runnable command suggestions
