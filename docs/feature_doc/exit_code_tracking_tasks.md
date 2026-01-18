# Exit Code Tracking - Implementation Tasks

## Goal
Track last command exit codes inside the TUI without requiring users to change their shell configuration.

## Approach Summary
Inject a lightweight prompt hook into the spawned shell (bash/zsh) that emits an invisible sentinel containing `$?`. Parse the sentinel from PTY output and update `LastExitCode` and command history.

## Task 1: Sentinel design
**Description:** define a safe, hidden marker format that won’t clutter output.

**Tasks:**
- Use an OSC sequence (e.g., `ESC ] 9 ; __WTF_EXIT:<code>__ BEL`) to keep it invisible in the terminal.
- Define a regex/parser to extract the exit code from the PTY stream.

**Definition of Done:**
- Sentinel format and parser rules documented in code.

## Task 2: Shell hook injection
**Description:** inject prompt hooks for bash/zsh at shell spawn time.

**Tasks:**
- Detect shell type from `$SHELL` or the spawned command.
- For **bash**: prepend to `PROMPT_COMMAND` to echo the sentinel.
- For **zsh**: use `precmd` or `precmd_functions` to echo the sentinel.
- Preserve existing hooks (chain rather than overwrite).

**Definition of Done:**
- Bash and zsh hooks emit the sentinel on every prompt without altering user prompt appearance.

## Task 3: PTY parsing and state updates
**Description:** parse sentinel output and update last exit code.

**Tasks:**
- Extend the PTY buffer parser to scan for sentinel sequences.
- Update `commands.Context.LastExitCode` and `capture.SessionContext` with the latest code.
- Ensure sentinel output does not pollute the visible terminal buffer.

**Definition of Done:**
- Exit codes update reliably after each command without visible artifacts.

## Task 4: Command association (optional but recommended)
**Description:** link exit codes to command history.

**Tasks:**
- Capture user-entered commands (from input handler) into session history.
- Attach exit codes when the sentinel arrives.

**Definition of Done:**
- `/history` entries include exit codes for recent commands.

## Task 5: Fallbacks and safety
**Description:** keep behavior stable on unsupported shells.

**Tasks:**
- If shell isn’t bash/zsh, skip injection gracefully.
- Default `LastExitCode` to 0 or “unknown” when unavailable.
- Guard against malformed sentinel output.

**Definition of Done:**
- No crashes or visual artifacts on unsupported shells.

## Task 6: Tests
**Description:** validate parsing and hook generation.

**Tasks:**
- Unit test sentinel parsing (including partial reads).
- Unit test hook composition to ensure existing hooks are preserved.
- Manual test: run failing command and confirm `/wtf` sees non-zero exit code.

**Definition of Done:**
- Tests pass and manual flow works as expected.
