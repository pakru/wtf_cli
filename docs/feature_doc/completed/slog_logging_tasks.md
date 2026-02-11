# Slog Logging Integration - Implementation Tasks

## Goal
Introduce structured logging for app workflow visibility without breaking the TUI.

## Principles
- Write logs to file (not stdout/stderr) to avoid corrupting the TUI.
- Use structured JSON via `log/slog`.
- Redact secrets (API keys, tokens, headers) before writing.
- Keep logging low overhead; use levels to control verbosity.

## Task 1: Logging package scaffold
**Description:** add a small logging package to initialize and manage `slog`.

**Tasks:**
- Add `pkg/logging` package with `Init(cfg config.Config) (*slog.Logger, error)`.
- Create log file path under `~/.wtf_cli/logs/wtf_cli.log`.
- Ensure log directory exists.
- Configure `slog.NewJSONHandler` with level from `cfg.LogLevel`.
- Set logger as default (`slog.SetDefault`).

**Definition of Done:**
- Logger initializes at startup without TUI output.
- Logs are written to the file in JSON format.

## Task 2: Add rotation and size limits
**Description:** prevent log growth using rotation.

**Tasks:**
- Add `github.com/natefinch/lumberjack` (or equivalent) as a log writer.
- Configure size/age/backup limits (e.g., 5 MB, 5 files, 14 days).
- Make rotation settings configurable (optional; can be constants at first).

**Definition of Done:**
- Log files rotate automatically.
- No unbounded log growth.

## Task 3: Config additions
**Description:** expose logging configuration in config file and settings UI.

**Tasks:**
- Extend `config.Config` with optional logging settings:
  - `log_file` (string, default `~/.wtf_cli/logs/wtf_cli.log`)
  - `log_format` (string, default `json`)
  - `log_level` (already present)
- Update defaults and validation (supported levels only).
- Update settings UI to surface log level and log file path.

**Definition of Done:**
- Config persists log settings.
- Validation rejects unknown log levels.

## Task 4: Add logging to core workflow
**Description:** instrument the critical path for visibility.

**Tasks:**
- Startup: log config load result and version/build info.
- Command dispatch: log command name, cwd, duration, success/failure.
- AI requests: log model, request start/stop, error type, and token counts if available.
- UI events: palette open/close, settings panel open/close, model picker refresh.
- PTY events: resize, read errors, shell exit.

**Definition of Done:**
- Logs capture the full workflow for a typical session.
- No secrets or full prompts in logs by default.

## Task 5: Redaction helpers
**Description:** prevent sensitive data from being logged.

**Tasks:**
- Add a helper to redact values (API keys, tokens, auth headers).
- Apply redaction in config logging, AI headers, and request metadata.

**Definition of Done:**
- Secrets never appear in logs.

## Task 6: Tests and validation
**Description:** add minimal tests around logging init.

**Tasks:**
- Unit test for log file creation and write (use temp dir).
- Unit test for log level parsing (invalid level returns error).
- Manual validation: run app and verify log output while issuing commands.

**Definition of Done:**
- Tests pass.
- Logs are readable and do not affect TUI.
