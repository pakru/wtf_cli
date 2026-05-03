# AGENTS.md - wtf_cli

## Project Overview
`wtf_cli` is an AI-assisted terminal wrapper written in Go. It acts as a transparent proxy around the user's shell, providing an enhanced TUI overlay with AI capabilities.

**Core Philosophy:**
- **Transparent Wrapper:** The user should feel like they are in their native shell.
- **Enhanced Overlay:** AI features (command palette, history search, explanations, agentic chat) appear on top of the shell when requested.
- **Performance:** Minimal latency, high throughput (batching/debounce implemented).

## Technical Stack
- **Language:** Go 1.25+
- **TUI Framework:** Bubble Tea v2 (`charm.land/bubbletea/v2`), Bubbles v2 (`charm.land/bubbles/v2`)
- **Styling:** Lipgloss v2 (`charm.land/lipgloss/v2`)
- **PTY Management:** `github.com/creack/pty`
- **Terminal Emulation:** `github.com/vito/midterm` (for full-screen app rendering)
- **AI Providers:** `github.com/openai/openai-go/v3`, `google.golang.org/genai`, `github.com/github/copilot-sdk/go` (OpenAI, Anthropic, Google Gemini, OpenRouter, GitHub Copilot)
- **Git Integration:** `github.com/go-git/go-git/v5`
- **Logging:** `log/slog` with `gopkg.in/natefinch/lumberjack.v2` for rotation
- **Build System:** Make
- **CI/CD:** GitHub Actions, GoReleaser

## Project Structure
```
wtf_cli/
├── cmd/wtf_cli/          # Main entry point
├── pkg/
│   ├── ai/               # AI/LLM integration
│   │   ├── auth/         # OAuth device flow + PKCE for provider auth
│   │   ├── providers/    # Provider implementations (openai, anthropic, google, openrouter, copilot)
│   │   ├── tools/        # Tool calling (read_file, registry)
│   │   ├── context.go    # Conversation context assembly
│   │   ├── platform.go   # Host platform detection
│   │   ├── registry.go   # Provider registry
│   │   └── ...
│   ├── buffer/           # Buffer management utilities
│   ├── capture/          # Session recording and shell history
│   ├── commands/         # Slash command parsing and execution
│   ├── config/           # Configuration management
│   ├── logging/          # Structured logging (slog-based)
│   ├── pty/              # Pseudo-terminal management and I/O wrapping
│   ├── ui/               # Core TUI logic
│   │   ├── model.go      # Main application state (ELM architecture)
│   │   ├── pty_batch.go  # PTY output batching
│   │   ├── components/   # Reusable TUI components
│   │   │   ├── fullscreen, historypicker, layout, palette, picker,
│   │   │   ├── result, selection, settings, sidebar, statusbar,
│   │   │   ├── toolapproval, viewport, welcome, utils, testutils
│   │   ├── input/        # Input handling and key interception
│   │   ├── render/       # Rendering utilities
│   │   ├── styles/       # Lipgloss style definitions
│   │   ├── terminal/     # Terminal emulation for full-screen apps (midterm)
│   │   └── testdata/     # Golden files for UI tests
│   ├── updatecheck/      # Self-update version checking
│   └── version/          # Version information (injected at build time)
└── docs/
    ├── feature_doc/
    │   ├── completed/    # Implemented feature plans/tasks
    │   ├── archive/      # Historical / superseded docs
    │   └── *.md          # Active feature plans (e.g., issue-58-agentic-loop.md)
    ├── images/
    └── RELEASE.md
```

## Development Workflow

### Build & Run
Always use the `Makefile` targets:
- `make build`: Builds the binary with version info injected via ldflags.
- `make run`: Builds and runs the application.
- `make check`: Runs fmt, vet, build, and tests (**Use this before submitting changes**).
- `make install`: Builds and installs binary to `~/.local/bin`.
- `make lint`: Runs `go fmt` and `go vet`.
- `make release-local`: Builds optimized release binary locally (`-s -w -trimpath`).
- `make version`: Shows current version information.
- `make clean`: Removes build artifacts.
- `make help`: Shows all available targets.

### Testing
- **Unit Tests:** `make test` (runs `go test -v ./...`).
- **Convention:** Tests are co-located with the code (e.g., `model_test.go` alongside `model.go`).
- **Golden Files:** UI tests use `github.com/charmbracelet/x/exp/golden`. Regenerate with `go test ./pkg/ui/... -update`.

### CI/CD
GitHub Actions in `.github/workflows/`:
- `go.yml`: Runs on PRs - builds, tests, and lints.
- `release.yml`: Handles release builds via GoReleaser.

## Key Architectural Patterns

### 1. Bubble Tea Model (ELM Architecture)
- Model-Update-View pattern.
- `Update()` handles messages (keyboard events, PTY data, timer ticks, AI stream events).
- `View()` renders the UI string.
- **Critical:** Heavy operations (I/O, network, LLM calls) must be commands (`tea.Cmd`) to avoid blocking the UI thread.

### 2. PTY Wrapper
- The app spawns a shell in a PTY.
- It proxies Stdin/Stdout between the real terminal and the PTY.
- **Filtering:** Special keys (e.g., `Ctrl+R`, `/`) are intercepted by the input layer (`pkg/ui/input`) and NOT sent to the PTY.

### 3. Full-Screen App Support
- Apps like `vim`, `nano`, `htop` enter "alternate screen buffer" mode.
- `pkg/ui/terminal/` uses `midterm` to emulate the terminal and render these apps.
- When in full-screen mode, shortcuts are disabled and all input passes through to the PTY.

### 4. Performance Optimizations (Critical)
- **PTY Batching:** PTY output is batched in `pty_batch.go` to prevent UI lag.
- **Stream Throttling:** AI stream deltas are throttled (`stream_throttle_test.go`) to maintain responsiveness.
- Respect these patterns when adding high-frequency event sources.

### 5. Input Handling
- `pkg/ui/input/` handles all keyboard input.
- Intercepts special keys before they reach the PTY.
- Be careful when modifying — must not break raw terminal passthrough.

### 6. Multi-Provider AI
- `pkg/ai/registry.go` selects the active provider from config.
- Each provider in `pkg/ai/providers/` implements a common interface.
- OAuth-based providers (Copilot, Google) use `pkg/ai/auth/` for device flow / PKCE.
- Tool calling is handled via `pkg/ai/tools/` with an approval flow (`pkg/ui/components/toolapproval/`).

## Agent Guidelines

### Code Style
1. Follow standard Go conventions. Run `make fmt` if unsure.
2. Use `make check` before committing to ensure all tests pass.
3. Run `make vet` and `make lint` for static analysis.

### UI Changes
1. Reuse Lipgloss styles from `pkg/ui/styles/` to maintain consistency.
2. Update golden files (`-update`) if your changes intentionally modify UI output.
3. Keep components in `pkg/ui/components/` reusable and self-contained.

### Critical Areas (Extra Caution Required)
1. **`pkg/pty/`**: PTY management - do not break raw terminal passthrough.
2. **`pkg/ui/input/`**: Input handling - shortcuts must not interfere with normal shell usage.
3. **`pkg/ui/model.go`**: Main state machine - changes here have wide-reaching effects.
4. **`pkg/ai/providers/`**: Provider implementations - keep interface contracts consistent.

### Feature Documentation
- Active feature plans live in `docs/feature_doc/` (e.g., agentic loop, terminal context chat, sidebar resize).
- Completed work is moved to `docs/feature_doc/completed/`.
- Check `docs/feature_doc/completed/INDEX.md` for a quick reference of past implementations.

## Configuration
- Configuration is managed in `pkg/config/`.
- AI providers are configured per-provider; OAuth-based providers store credentials via `pkg/ai/auth/`.
