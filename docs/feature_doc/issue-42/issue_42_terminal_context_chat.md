# Terminal Context Injection for Chat (Issue #42)

When a user opens the chat sidebar (without `/explain`), the LLM already receives terminal context — but the system prompt and context framing are copied from `/explain`, which tells the LLM to "diagnose issues." This makes the chat feel like a diagnostic tool rather than a general assistant that is *aware* of the terminal.

This plan separates the chat prompts from the `/explain` prompts so the LLM treats terminal output as **background context** rather than something to diagnose.

## Proposed Changes

### AI Context Layer

#### [MODIFY] [context.go](file:///home/pavel/STORAGE/Projects/my_projects/wtf_cli/pkg/ai/context.go)

Add two unexported helper functions and one exported function (unexported helpers consistent with existing `wtfSystemPrompt` / `buildUserPrompt` naming):

- **`chatSystemPrompt() string`** — A chat-oriented system prompt that tells the LLM:
  - "You are a terminal assistant. The user may ask about anything — if terminal context is provided, use it to inform your answers, but do not proactively diagnose unless asked."
  - Retains platform info via `GetPlatformInfo().PromptText()`
  - Keeps the `<cmd>` tag instruction for runnable commands
  - Omits the `/explain`-specific lines ("focus on that command", "diagnose issues", etc.)

- **`buildChatUserPrompt(meta TerminalMetadata, ctx TerminalContext) string`** — A lighter context injection prompt:
  - Frames terminal output as reference material: "Below is the user's recent terminal activity for context."
  - Includes the same metadata fields (cwd, last_command, last_exit_code, output_lines)
  - Does **not** say "Please check … and explain what's going on"

- **`BuildChatContext(lines [][]byte, meta TerminalMetadata) TerminalContext`** (exported) — Mirrors `BuildTerminalContext` but uses `chatSystemPrompt()` and `buildChatUserPrompt()`.

> [!NOTE]
> The existing `BuildTerminalContext`, `wtfSystemPrompt`, and `buildUserPrompt` are left unchanged — `/explain` continues to work exactly as before.

---

### Chat Handler

#### [MODIFY] [chat_handler.go](file:///home/pavel/STORAGE/Projects/my_projects/wtf_cli/pkg/commands/chat_handler.go)

Update `buildChatMessages()` (line ~138):

```diff
-termCtx := ai.BuildTerminalContext(lines, meta)
+termCtx := ai.BuildChatContext(lines, meta)
```

One-line change. The rest of the function remains the same — it already constructs `system` + `developer` + history messages correctly.

---

### Tests

#### [MODIFY] [context_test.go](file:///home/pavel/STORAGE/Projects/my_projects/wtf_cli/pkg/ai/context_test.go)

**New chat context tests:**
- `TestBuildChatContext_SystemPrompt` — Verify the chat system prompt does **not** contain "diagnose" or "explain what's going on" but **does** contain `<cmd>` instruction
- `TestBuildChatContext_UserPrompt` — Verify the chat user prompt contains metadata (last_exit_code, cwd, etc.) but does **not** contain "explain what's going on"
- `TestBuildChatContext_OutputIncluded` — Verify terminal output is included in context

**Regression tests for `/explain` prompt stability:**
- `TestBuildTerminalContext_SystemPromptContainsDiagnose` — Verify the `/explain` system prompt still contains "diagnose issues"
- `TestBuildTerminalContext_UserPromptContainsExplainWording` — Verify the `/explain` user prompt still contains "explain what's going on"

#### [MODIFY] [chat_handler_test.go](file:///home/pavel/STORAGE/Projects/my_projects/wtf_cli/pkg/commands/chat_handler_test.go)

Update `TestChatHandler_buildChatMessages_IncludesCommandTagInstruction` — should still pass (chat system prompt retains `<cmd>` instruction)

**New integration-style test proving the swap:**
- `TestChatHandler_buildChatMessages_UsesNonDiagnosticPrompt` — Build chat messages with terminal output, then assert the system prompt does **not** contain "diagnose issues" and the developer message does **not** contain "explain what's going on". This ensures the one-line swap to `BuildChatContext` actually took effect.

## Verification Plan

### Automated Tests

Run targeted package tests, then full project validation:

```bash
cd /home/pavel/STORAGE/Projects/my_projects/wtf_cli
go test ./pkg/ai/ -v -count=1
go test ./pkg/commands/ -v -count=1
make check
```

### Manual Verification

1. Build and run: `make run`
2. Open the chat sidebar with **Ctrl+T** (no `/explain`)
3. Type a general question like "What does the ls command do?" and confirm the response is a helpful general answer, not a diagnostic report about the terminal output
4. Run a failing command (e.g. `ls /nonexistent`), then ask in chat "What happened?" — confirm the LLM references the terminal error correctly
5. Use `/explain` and confirm it still produces diagnostic-style output as before
