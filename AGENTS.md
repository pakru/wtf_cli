# AGENTS.md - wtf_cli

## Project Overview
`wtf_cli` is an AI-assisted terminal wrapper written in Go. It acts as a transparent proxy around the user's shell, providing an enhanced TUI overlay with AI capabilities.

**Core Philosophy:**
- **Transparent Wrapper:** The user should feel like they are in their native shell.
- **Enhanced Overlay:** AI features (command palette, history search, explanations) appear on top of the shell when requested.
- **Performance:** Minimal latency, high throughput (batching/debounce implemented).

## Technical Stack
- **Language:** Go 1.25+
- **TUI Framework:** [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) (`charm.land/bubbletea/v2`)
- **Styling:** [Lipgloss v2](https://github.com/charmbracelet/lipgloss) (`charm.land/lipgloss/v2`)
- **PTY Management:** [creack/pty](https://github.com/creack/pty)
- **Terminal Emulation:** [vito/midterm](https://github.com/vito/midterm) - For full-screen app rendering
- **AI Integration:** [openai-go](https://github.com/openai/openai-go)
- **Build System:** Make
- **CI/CD:** GitHub Actions (`.github/workflows/`)

## Project Structure
```
wtf_cli/
├── cmd/wtf_cli/          # Main entry point
├── pkg/
│   ├── ai/               # AI/LLM integration (OpenAI client, prompts)
│   ├── buffer/           # Buffer management utilities
│   ├── capture/          # Session recording and bash history
│   ├── commands/         # Command parsing and execution
│   ├── config/           # Configuration management
│   ├── logging/          # Structured logging (slog-based)
│   ├── pty/              # Pseudo-terminal management and I/O wrapping
│   ├── ui/               # Core TUI logic
│   │   ├── model.go      # Main application state (ELM architecture)
│   │   ├── components/   # Reusable TUI components (viewport, settings, picker, history panel)
│   │   ├── input/        # Input handling and key interception
│   │   ├── render/       # Rendering utilities
│   │   ├── styles/       # Lipgloss style definitions
│   │   └── terminal/     # Terminal emulation for full-screen apps (midterm)
│   └── version/          # Version information (injected at build time)
└── docs/
    ├── feature_doc/      # Implementation plans and feature specs
    └── RELEASE.md        # Release process documentation
```

## Development Workflow

### Build & Run
Always use the `Makefile` targets:
- `make build`: Builds the binary with version info.
- `make run`: Builds and runs the application.
- `make check`: Runs fmt, vet, build, and tests (**Use this before submitting changes**).
- `make install`: Builds and installs binary to `~/.local/bin`.
- `make lint`: Runs formatting and static analysis.
- `make release-local`: Builds optimized release binary locally.
- `make version`: Shows current version information.
- `make clean`: Removes build artifacts.
- `make help`: Shows all available targets.

### Testing
- **Unit Tests:** `go test ./...` or `make test`
- **Convention:** Tests should be co-located with the code (e.g., `model_test.go` alongside `model.go`).
- **Golden Files:** UI components use golden file testing (`pkg/ui/testdata/`). If you modify UI rendering, existing golden tests may fail. Regenerate with `go test ./pkg/ui/... -update`.

### CI/CD
GitHub Actions are configured in `.github/workflows/`:
- `go.yml`: Runs on pull requests - builds, tests, and lints.
- `release.yml`: Handles release builds via GoReleaser.

## Key Architectural Patterns

### 1. The Bubble Tea Model (ELM Architecture)
- The app follows the Model-Update-View pattern.
- `Update()` handles messages (keyboard events, PTY data, timer ticks).
- `View()` renders the UI string.
- **Critical:** Heavy operations (I/O, network) must be commands (`tea.Cmd`) to avoid blocking the UI thread.

### 2. PTY Wrapper
- The app spawns a shell in a PTY.
- It proxies Stdin/Stdout between the real terminal and the PTY.
- **Filtering:** Special keys (e.g., `Ctrl+R`, `/`) are intercepted by the Input Handler (`pkg/ui/input`) and NOT sent to the PTY.

### 3. Full-Screen App Support
- Apps like `vim`, `nano`, `htop` enter "alternate screen buffer" mode.
- `pkg/ui/terminal/` uses `midterm` to emulate the terminal and render these apps.
- When in full-screen mode, shortcuts are disabled and all input is passed directly to the PTY.

### 4. Performance Optimizations (Critical)
- **Debouncing/Batching:** PTY output and stream updates are batched (see `pty_batch.go`) to prevent UI lag.
- **Throttling:** High-frequency events are throttled to maintain responsiveness.
- Respect these patterns when adding high-frequency event sources.

### 5. Input Handling
- `pkg/ui/input/` handles all keyboard input.
- Intercepts special keys before they reach the PTY.
- Must be careful when modifying to not break raw terminal passthrough.

## Agent Guidelines

### Code Style
1. Follow Go standards. Run `make fmt` if unsure.
2. Use `make check` before committing to ensure all tests pass.
3. Run `make vet` and `make lint` for static analysis.

### UI Changes
1. Utilize existing Lipgloss styles from `pkg/ui/styles/` to maintain consistency.
2. Update golden files if your changes intentionally modify UI output.
3. Keep components in `pkg/ui/components/` reusable and self-contained.

### Critical Areas (Extra Caution Required)
1. **`pkg/pty/`**: PTY management - be extremely careful not to break raw terminal passthrough.
2. **`pkg/ui/input/`**: Input handling - ensure shortcuts don't interfere with normal shell usage.
3. **`pkg/ui/model.go`**: Main state machine - changes here can have wide-reaching effects.

### Feature Documentation
- Check `docs/feature_doc/` for implementation details of features:
  - History Search (`bash_history_search_tasks.md`)
  - Debouncing (`debounce_batch_updates_tasks.md`, `debounce_walkthrough.md`)
  - Full-screen Apps (`fullscreen_apps_tasks.md`)
  - AI Integration (`phase_6_ai_integration_tasks.md`)
  - And more...

## Configuration
- Configuration is managed in `pkg/config/`.
- AI features require an OpenAI API key (check config for environment variable names).
