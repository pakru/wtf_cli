# WTF CLI Wrapper: Requirements & Implementation Plan

> [!NOTE]
> **From-Scratch Implementation**: This is a complete rewrite of `wtf_cli` with a new PTY-based architecture. We will build this from the ground up in a separate branch, not modify the existing codebase.

## 1. Core Vision
To create a transparent terminal wrapper (`wtf_cli`) that captures all input and output for AI analysis while providing a pixel-perfect, zero-latency experience identical to the native shell. This will be built as a **new implementation** from scratch, replacing the current shell-hook-based approach with a PTY wrapper architecture.

## 2. User Experience (UX) Goals
*   **Zero friction**: The user executes `wtf_cli` in their terminal to start the wrapper utility.
*   **Native feel**: Ctrl+C, Ctrl+Z, colors, cursor movements, and vim/nano must work exactly as normal.
*   **Subtle presence**: A minimal status bar indicates the utility is running without being intrusive.
*   **On-demand AI**:
    *   **Slash commands**: Typing `/` brings up a command palette with suggested actions.
    *   **Context-aware assistance**: AI analyzes recent terminal history to provide helpful suggestions.

## 3. Functional Requirements

### 3.1. The Wrapper (PTY Host)
*   **Binary name**: `wtf_cli`
*   **Language**: Go (Golang)
*   **Library**: `creack/pty` for cross-platform pseudo-terminal management
*   **Process Management**:
    *   Spawn user's default shell (`$SHELL`)
    *   Handle window resize events (SIGWINCH) and propagate to child shell
    *   Correctly handle raw mode vs. cooked mode (pass TTY state)
    *   Clean cleanup on exit (don't leave zombie shells)
    *   **Critical**: Capture and record exit codes from executed commands reliably

### 3.2. Data Capture (The "Recorder")
*   **Circular Buffer**: Maintain an in-memory ring buffer
    *   **Configurable size**: Allow users to set buffer size via config file
    *   **Default**: Last 2000 lines
*   **Stream Capture**: Record raw bytes (including ANSI codes) for full fidelity
*   **Exit Code Tracking**: Must reliably capture exit codes using shell integration hooks

### 3.3. The AI Interface
*   **Trigger**:
    *   **Slash command**: User types `/` which triggers a command palette overlay
    *   Commands available: `/wtf`, `/explain`, `/fix`, `/suggest`, etc.
*   **Context Payload**:
    *   **Last 1000 lines** of terminal output (configurable, but 1000 by default)
    *   Current command line (what the user is typing)
    *   **Exit code of last command** (captured via shell integration)
    *   Working directory
    *   Environment context
*   **UI Components**:
    *   **Status bar**: Always visible at top/bottom showing `wtf_cli` is active
    *   **Command palette**: Appears when `/` is typed, showing available commands
    *   **Overlay/Modal**: For AI responses (using TUI library like `bubbletea` or `tview`)

### 3.4. Settings Persistence & LLM Configuration
*   **Configuration file**: `~/.wtf_cli/config.json`
*   **Auto-save**: Settings are persisted immediately when changed
*   **Settings include**:
    *   Buffer size (default: 2000 lines)
    *   Context window size (default: 1000 lines)
    *   LLM provider configuration (see below)
    *   Status bar position (top/bottom/hidden)
    *   Status bar colors and theme
    *   UI preferences
*   **Validation**: Invalid settings should fall back to defaults with warnings
*   **Migration**: Support upgrading config from older versions

#### LLM Provider Configuration
*   **Provider**: OpenRouter (supports 100+ models from multiple providers)
*   **Configuration structure**:
    ```json
    {
      "llm_provider": "openrouter",
      "openrouter": {
        "api_key": "<your_openrouter_api_key>",
        "model": "google/gemini-2.0-flash-exp:free",
        "temperature": 0.7,
        "max_tokens": 2000,
        "api_timeout_seconds": 30
      },
      "buffer_size": 2000,
      "context_window": 1000,
      "status_bar": {
        "position": "bottom",
        "colors": "auto"
      },
      "log_level": "info"
    }
    ```
*   **Supported Models** (configurable via `model` field):
    *   `google/gemini-3.0-flash` (default, latest fast model)
    *   `anthropic/claude-3.5-sonnet`
    *   `openai/gpt-4o`
    *   `meta-llama/llama-3.1-70b-instruct`
    *   Any model available on OpenRouter
*   **Configuration Only**: All settings managed via config file (`~/.wtf_cli/config.json`), no environment variable overrides

## 4. Technical Architecture

### 4.1. Core Components
```
wtf_cli/
├── cmd/
│   └── wtf_cli/        # Main entry point for the wrapper
├── pkg/
│   ├── pty/            # PTY management and signal handling
│   ├── buffer/         # Circular buffer and history management
│   ├── capture/        # Exit code and command tracking
│   ├── ui/             # Bubbletea TUI code for status bar and overlay
│   ├── commands/       # Slash command handlers
│   ├── ai/             # AI/LLM integration
│   └── config/         # Configuration management and persistence
└── config/             # Configuration management
```

### 4.2. Shell Integration Strategy
> [!IMPORTANT]
> **No heuristics** - We will rely only on factual data from shell integration hooks

*   Keep existing `~/.wtf/last_command.json` mechanism
*   Enhance it to work seamlessly with the wrapper
*   Wrapper reads the JSON file to get accurate exit codes and command metadata

## 5. Implementation Tasks

### Phase 1: Basic PTY Wrapper (Foundation)
**Goal**: Create a transparent terminal proxy that feels native

#### Task 1.1: Set up Go project structure
**Description:**
  - **Create feature branch**: `git checkout -b feature/pty-wrapper` (work separately from main)
  - **Clean slate**: Start with fresh directory structure (remove old cmd/pkg if present)
  - Initialize Go module with `go mod init wtf_cli`
  - Create new directory structure as outlined in section 4.1
  - Add `creack/pty` dependency
  - Set up basic GitHub Actions or build scripts
  - **Note**: We can reference existing `api/`, `config/` packages for inspiration but will rewrite them to fit the new architecture

**Git Branching Strategy:**
  - All PTY wrapper development will happen in the `feature/pty-wrapper` branch
  - Keep `main` branch stable with current implementation
  - Merge to `main` only after Phase 7 completion and testing
  - Regular commits with descriptive messages for each task
  - **From-scratch approach**: Building entirely new codebase, not refactoring existing

**Definition of Done:**
  - ✅ New branch `feature/pty-wrapper` created from `main`
  - ✅ `go.mod` exists with correct module name
  - ✅ All directories from structure exist
  - ✅ `go build` completes successfully
  - ✅ `Makefile` with `build`, `test`, `clean` targets
  - ✅ README.md with project description
  - ✅ Initial commit: "feat: initialize PTY wrapper architecture"

#### Task 1.2: Implement basic PTY spawning
**Description:**
  - Spawn user's `$SHELL` in a PTY
  - Pass through stdin → PTY
  - Pass through PTY → stdout
  - Handle clean exit and cleanup

**Definition of Done:**
  - ✅ `pkg/pty/pty.go` implements `SpawnShell()` function
  - ✅ Shell spawns correctly with inherited environment
  - ✅ Input/output proxying works bidirectionally
  - ✅ Process cleans up on exit (no zombies)
  - ✅ **Unit tests**: Mock PTY spawn and verify I/O forwarding
  - ✅ Manual test: run `wtf_cli`, type `echo hello`, see output

#### Task 1.3: Handle terminal resize (SIGWINCH)
**Description:**
  - Listen for window resize signals
  - Propagate size changes to child PTY
  - Test with full-screen applications

**Definition of Done:**
  - ✅ SIGWINCH handler implemented in `pkg/pty/signals.go`
  - ✅ Window size propagated to child PTY correctly
  - ✅ **Unit tests**: Mock signal and verify size update
  - ✅ **Integration tests**: Resize terminal running `vim`, verify no corruption
  - ✅ Test passes with `vim`, `htop`, `less`

#### Task 1.4: Raw mode handling
**Description:**
  - Set parent terminal to raw mode for proper input handling
  - Restore terminal state on exit (even on crashes)
  - Add signal handlers (SIGINT, SIGTERM)

**Definition of Done:**
  - ✅ Raw mode enabled in `pkg/pty/terminal.go`
  - ✅ Deferred cleanup restores terminal state
  - ✅ Signal handlers (SIGINT, SIGTERM, SIGQUIT) implemented
  - ✅ **Unit tests**: Verify raw mode set/unset correctly
  - ✅ Manual test: Ctrl+C in vim works, terminal not broken after crash
  - ✅ No `reset` command needed after exit

**Phase Verification**: Run `vim`, `htop`, `nano`, colored `ls`. Everything must work perfectly.

---

### Phase 2: Output Capture & Buffering
**Goal**: Record all terminal output without affecting performance

#### Task 2.1: Implement circular buffer
**Description:**
  - Create configurable ring buffer structure
  - Default: 2000 lines
  - Efficiently handle ANSI escape sequences
  - Thread-safe for concurrent reads/writes

**Definition of Done:**
  - ✅ `pkg/buffer/circular.go` implements `CircularBuffer` type
  - ✅ Methods: `Write()`, `Read()`, `GetLastN()`, `Clear()`
  - ✅ Configurable size from config
  - ✅ **Unit tests**: Buffer wrap-around behavior
  - ✅ **Unit tests**: Thread safety (concurrent reads/writes)
  - ✅ **Unit tests**: ANSI sequence preservation
  - ✅ **Benchmark tests**: performance with 10k+ lines

#### Task 2.2: Tee output stream
**Description:**
  - Copy PTY output to buffer before writing to stdout
  - Preserve all ANSI codes and control characters
  - Zero added latency (use goroutines)

**Definition of Done:**
  - ✅ `pkg/pty/streamer.go` implements tee functionality
  - ✅ Buffer written concurrently without blocking stdout
  - ✅ All bytes preserved (no corruption)
  - ✅ **Unit tests**: Verify tee writes to both destinations
  - ✅ **Performance tests**: Latency < 1ms for typical output
  - ✅ Manual test: Run `cat large_file.log`, verify buffer captured correctly

#### Task 2.3: Settings persistence system
**Description:**
  - Create `~/.wtf_cli/config.json` structure with all settings
  - Implement load/save with JSON marshaling
  - Support default values and validation
  - Add buffer size configuration
  - Add context window size (default: 1000 lines)
  - Add complete LLM provider configuration
  - Auto-create config directory if missing
  - **Note**: Configuration file only, no environment variable overrides

**Config Structure:**
```go
type Config struct {
    LLMProvider    string           `json:"llm_provider"`
    OpenRouter     OpenRouterConfig `json:"openrouter"`
    BufferSize     int              `json:"buffer_size"`
    ContextWindow  int              `json:"context_window"`
    StatusBar      StatusBarConfig  `json:"status_bar"`
    LogLevel       string           `json:"log_level"`
}

type OpenRouterConfig struct {
    APIKey            string  `json:"api_key"`
    Model             string  `json:"model"`
    Temperature       float64 `json:"temperature"`
    MaxTokens         int     `json:"max_tokens"`
    APITimeoutSeconds int     `json:"api_timeout_seconds"`
}
```

**Definition of Done:**
  - ✅ `pkg/config/config.go` defines complete `Config` struct
  - ✅ Methods: `Load()`, `Save()`, `Validate()`, `SetDefaults()`
  - ✅ Config file created at `~/.wtf_cli/config.json`
  - ✅ Invalid values fall back to defaults with warning logged
  - ✅ **Unit tests**: Load valid config
  - ✅ **Unit tests**: Handle missing config (create defaults)
  - ✅ **Unit tests**: Handle corrupted JSON (fallback to defaults)
  - ✅ **Unit tests**: Validation rejects invalid values (negative buffer, bad temp range)
  - ✅ **Unit tests**: API key validation (required)
  - ✅ Config auto-saved on changes
  - ✅ Default model: `google/gemini-3.0-flash`

#### Task 2.4: Buffer export capability
**Description:**
  - Add debug command to dump buffer to file
  - Add API to retrieve buffer contents
  - Verify captured content matches visible output

**Definition of Done:**
  - ✅ `pkg/buffer/export.go` implements `ExportToFile()` method
  - ✅ Command: `wtf_cli --dump-buffer /path/to/file`
  - ✅ **Unit tests**: Export writes correct content
  - ✅ **Unit tests**: Handle write errors gracefully
  - ✅ Manual test: Run commands, dump buffer, verify content matches

**Phase Verification**: Run a long log output, verify buffer contains the latest N lines.

---

### Phase 3: Session Context Tracking
**Goal**: Track command history and working directory

> [!NOTE]
> **Simplified approach**: Since we're capturing all output via PTY, we don't need shell integration hooks or file watchers. Exit codes and command tracking will be inferred from PTY stream and prompt analysis.

#### Task 3.3: Session context tracker
**Description:**
  - Track working directory changes
  - Track command history with exit codes
  - Associate buffer sections with commands
  - Maintain timeline of session events

**Definition of Done:**
  - ✅ `pkg/capture/session.go` implements `SessionContext` type
  - ✅ Methods: `AddCommand()`, `GetHistory()`, `GetCurrentDir()`
  - ✅ Each command linked to buffer range
  - ✅ **Unit tests**: Add multiple commands, verify history
  - ✅ **Unit tests**: Directory change tracking
  - ✅ **Unit tests**: Buffer association correctly maintained
  - ✅ Memory usage bounded (e.g., max 1000 commands in history)

**Phase Verification**: Run failing commands (`ls /nonexistent`), verify exit code is captured.

---

### Phase 4: Bubble Tea TUI Integration
**Goal**: Integrate Bubble Tea for modern, professional TUI with status bar and overlays

> [!IMPORTANT]
> **Architectural Change**: Moving from simple ANSI escape sequences to full Bubble Tea TUI framework for proper terminal control and no interference with PTY output.

#### Task 4.1: Add Bubble Tea framework
**Description:**
  - Add `github.com/charmbracelet/bubbletea` dependency
  - Add `github.com/charmbracelet/lipgloss` for styling
  - Research PTY + Bubble Tea integration patterns
  - Create basic Bubble Tea model structure

**Bubble Tea + PTY Integration:**
  - Bubble Tea will manage the UI layer
  - PTY output needs to be displayed within Bubble Tea's viewport
  - Input goes through Bubble Tea → PTY
  - PTY output rendered in Bubble Tea viewport

**Definition of Done:**
  - ✅ Bubble Tea and Lipgloss dependencies added
  - ✅ `pkg/ui/model.go` implements Bubble Tea Model interface
  - ✅ Basic Init(), Update(), View() methods
  - ✅ **Unit tests**: Model initialization
  - ✅ **Research doc**: PTY integration patterns documented

#### Task 4.2: PTY viewport integration
**Description:**
  - Create viewport component for PTY output
  - Integrate PTY output stream into Bubble Tea's event loop
  - Handle PTY messages as Bubble Tea messages
  - Ensure real-time output (no buffering delays)

**Technical Approach:**
  ```go
  type ptyMsg struct {
      output []byte
  }
  
  // PTY output listener sends messages to Bubble Tea
  go func() {
      buf := make([]byte, 1024)
      for {
          n, _ := pty.Read(buf)
          program.Send(ptyMsg{output: buf[:n]})
      }
  }()
  ```

**Definition of Done:**
  - ✅ `pkg/ui/viewport.go` implements PTY viewport
  - ✅ PTY output messages sent to Bubble Tea program
  - ✅ Output displayed in real-time without lag
  - ✅ Terminal scrolling works correctly
  - ✅ **Unit tests**: PTY message handling
  - ✅ **Integration tests**: PTY output rendering
  - ✅ Manual test: Run commands, see immediate output

#### Task 4.3: Status bar with Bubble Tea
**Description:**
  - Create status bar component using Lipgloss
  - Fixed bottom position
  - Display: `[wtf_cli] /current/directory | Press / for commands`
  - Update on directory changes
  - Beautiful styling with gradients/colors

**Styling with Lipgloss:**
  ```go
  statusBarStyle := lipgloss.NewStyle().
      Foreground(lipgloss.Color("15")).
      Background(lipgloss.Color("62")).
      Padding(0, 1)
  ```

**Definition of Done:**
  - ✅ `pkg/ui/statusbar_view.go` implements status bar view
  - ✅ Status bar always visible at bottom
  - ✅ Directory updates reflected immediately
  - ✅ Styled with Lipgloss (gradient, colors)
  - ✅ **Unit tests**: Status bar rendering
  - ✅ **Unit tests**: Directory update logic
  - ✅ Manual test: Status bar persistent, looks professional

#### Task 4.4: Input handling with Bubble Tea
**Description:**
  - Route keyboard input through Bubble Tea
  - Send normal keypresses to PTY
  - Intercept `/` for command palette
  - Handle Ctrl+C, Ctrl+D gracefully

**Input Flow:**
  - User types → Bubble Tea Update()
  - Normal chars → send to PTY
  - `/` at prompt → trigger command palette
  - Ctrl+D → exit wtf_cli

**Definition of Done:**
  - ✅ `pkg/ui/input.go` implements input handler
  - ✅ Normal typing works (sent to PTY)
  - ✅ `/` intercepted for commands
  - ✅ Ctrl+C/Ctrl+D handled properly
  - ✅ **Unit tests**: Input routing logic
  - ✅ **Integration tests**: Type characters, verify PTY receives them
  - ✅ Manual test: Type in shell, everything works normally

#### Task 4.5: Layout and rendering
**Description:**
  - Combine viewport + status bar in layout
  - Handle terminal resize
  - Ensure no flicker or corruption
  - Clean shutdown and terminal restoration

**Layout Structure:**
  ```
  ┌──────────────────────────────┐
  │                              │
  │   PTY Output Viewport        │
  │   (scrollable)               │
  │                              │
  ├──────────────────────────────┤
  │ [wtf_cli] /dir | Press /     │
  └──────────────────────────────┘
  ```

**Definition of Done:**
  - ✅ `pkg/ui/layout.go` combines components
  - ✅ Viewport takes all space except status bar
  - ✅ Terminal resize handled correctly
  - ✅ No flicker during updates
  - ✅ **Unit tests**: Layout calculations
  - ✅ **Integration tests**: Resize terminal, verify layout adapts
  - ✅ Manual test: Resize terminal, run vim/htop, verify no issues

**Phase Verification**: 
- Status bar visible and persistent
- PTY output works perfectly
- Can run complex apps (vim, htop, ssh)
- Beautiful, professional appearance

---

### Phase 5: Slash Command System
**Goal**: Implement `/` trigger and command palette

#### Task 5.1: Input interception
**Description:**
  - Watch for `/` character at beginning of line
  - Pause PTY input forwarding
  - Enter command palette mode
  - Handle edge cases (e.g., `/` in the middle of typing)

**Definition of Done:**
  - ✅ `pkg/ui/interceptor.go` implements input monitoring
  - ✅ Detect `/` at line start only
  - ✅ PTY input paused when palette active
  - ✅ **Unit tests**: `/` at start triggers palette
  - ✅ **Unit tests**: `/` elsewhere ignored
  - ✅ **Unit tests**: Esc cancels palette
  - ✅ Manual test: Type `/`, palette appears

#### Task 5.2: Command palette UI
**Description:**
  - Show list of available commands
  - `/wtf` - Analyze last output and suggest fixes
  - `/explain` - Explain what the last command did
  - `/fix` - Suggest fix for last error
  - `/history` - Show command history
  - Arrow keys to navigate, Enter to select, Esc to cancel

**Definition of Done:**
  - ✅ `pkg/ui/palette.go` implements command palette
  - ✅ All commands registered in palette
  - ✅ Keyboard navigation works (↑↓, Enter, Esc)
  - ✅ Visual feedback for selected item
  - ✅ **Unit tests**: Command registration
  - ✅ **Unit tests**: Navigation logic
  - ✅ **Unit tests**: Selection and cancellation
  - ✅ Manual test: Navigate palette, select command

#### Task 5.3: Command dispatcher
**Description:**
  - Route selected command to appropriate handler
  - Pass buffer context to handler
  - Display results in overlay
  - Handle async command execution

**Definition of Done:**
  - ✅ `pkg/commands/dispatcher.go` implements routing
  - ✅ Each command has handler function signature
  - ✅ Context (buffer, exit code, etc.) passed correctly
  - ✅ **Unit tests**: Command routing
  - ✅ **Unit tests**: Context preparation
  - ✅ **Unit tests**: Error handling for failed commands
  - ✅ Manual test: Execute each command type

**Phase Verification**: Type `/`, see command palette, select command, see results.

---

### Phase 5.5: Settings Panel
**Goal**: Add in-app settings configuration via `/settings` command

#### Task 5.5.1: Add `/settings` command to palette
**Description:**
  - Add `/settings` command to the command palette
  - Create handler that opens settings panel

**Definition of Done:**
  - ✅ `/settings` appears in command palette
  - ✅ Selecting it opens settings panel
  - ✅ **Unit tests**: Command registration

#### Task 5.5.2: Create Settings Panel UI
**Description:**
  - Create full-width settings panel component
  - Display all config options with current values
  - Support navigation with arrow keys
  - Support inline editing of values
  - Display field types (string/int/float/bool)

**Settings to display:**
  - API Key (string, masked)
  - Model (string)
  - Temperature (float, 0.0-2.0)
  - Max Tokens (int)
  - API Timeout (int, seconds)
  - Buffer Size (int)
  - Context Window (int)
  - Dry Run (bool)

**Definition of Done:**
  - ✅ `pkg/ui/settings_panel.go` implements settings panel
  - ✅ All config fields displayed with labels
  - ✅ Arrow key navigation (↑↓)
  - ✅ Enter to edit selected field
  - ✅ Esc to close (with save prompt if changed)
  - ✅ **Unit tests**: Panel rendering
  - ✅ **Unit tests**: Navigation logic
  - ✅ **Unit tests**: Value editing

#### Task 5.5.3: Config save/load integration
**Description:**
  - Load current config when panel opens
  - Validate edited values
  - Save to `~/.wtf_cli/config.json` on confirm
  - Show error messages for invalid values

**Definition of Done:**
  - ✅ Config loaded from file on panel open
  - ✅ Changes saved when user confirms
  - ✅ Validation errors shown inline
  - ✅ **Unit tests**: Save/load cycle
  - ✅ Manual test: Edit settings, verify saved to file

**Phase Verification**: Open `/settings`, modify API key, save, verify file updated.

---

### Phase 6: AI Integration
**Goal**: Connect buffer context to LLM for assistance with multi-provider support

#### Task 6.1: Multi-provider AI client setup
**Description:**
  - **Create new** `pkg/ai/` package
  - Support multiple LLM providers (selectable in config):
    - **OpenRouter** (default) - Access to 400+ models via unified API
    - **OpenAI** (direct) - Direct OpenAI API access
    - **Anthropic** (direct) - Direct Claude API access
    - **Custom** - User-provided API URL (OpenAI-compatible)
  - Implement provider interface for easy extension
  - Add timeout and error handling per provider
  - API key from config file

**Provider Configuration:**
```json
{
  "llm_provider": "openrouter",  // "openrouter" | "openai" | "anthropic" | "custom"
  "openrouter": {
    "api_key": "<key>",
    "api_url": "https://openrouter.ai/api/v1",
    "model": "google/gemini-2.0-flash-exp:free"
  },
  "openai": {
    "api_key": "<key>",
    "api_url": "https://api.openai.com/v1",
    "model": "gpt-4o"
  },
  "anthropic": {
    "api_key": "<key>",
    "api_url": "https://api.anthropic.com/v1",
    "model": "claude-3-5-sonnet-20241022"
  },
  "custom": {
    "api_key": "<key>",
    "api_url": "http://localhost:11434/v1",
    "model": "llama3"
  }
}
```

**Definition of Done:**
  - ✅ `pkg/ai/provider.go` defines Provider interface
  - ✅ `pkg/ai/openrouter.go` implements OpenRouter provider
  - ✅ `pkg/ai/openai.go` implements OpenAI provider
  - ✅ `pkg/ai/anthropic.go` implements Anthropic provider
  - ✅ `pkg/ai/custom.go` implements custom URL provider
  - ✅ Provider selection from config `llm_provider` field
  - ✅ Each provider has its own API URL and authentication
  - ✅ **Unit tests**: Provider initialization
  - ✅ **Unit tests**: Provider selection logic
  - ✅ **Integration tests**: Mock API responses per provider

#### Task 6.1.5: Remove `/models` command
**Description:**
  - Remove `/models` command registration and handler
  - Drop `/models` from the command palette and help text
  - Remove model list formatting helpers tied to `/models`

**Definition of Done:**
  - ✅ `/models` is no longer registered or displayed
  - ✅ No `/models` references remain in UI/help

#### Task 6.2: Context preparation
**Description:**
  - Extract last 1000 lines from buffer
  - Strip excessive ANSI codes (keep basic colors)
  - Include exit code, command, working directory
  - Build appropriate system prompt
  - Token counting and truncation

**Definition of Done:**
  - ✅ `pkg/ai/context.go` implements context builder
  - ✅ ANSI stripping preserves readability
  - ✅ Context includes all required metadata
  - ✅ **Unit tests**: Line extraction
  - ✅ **Unit tests**: ANSI code filtering
  - ✅ **Unit tests**: Context size limits respected
  - ✅ **Unit tests**: Prompt generation

#### Task 6.3: Implement `/wtf` command
**Description:**
  - Send context to LLM
  - Request analysis and suggestions
  - Stream response if possible
  - Show loading indicator

**Definition of Done:**
  - ✅ `pkg/commands/wtf.go` implements `/wtf` handler
  - ✅ Loading spinner while waiting for response
  - ✅ Response streaming (if API supports)
  - ✅ Error messages for API failures
  - ✅ **Unit tests**: Request building
  - ✅ **Integration tests**: Mock LLM responses
  - ✅ Manual test: Run failing command, execute `/wtf`

#### Task 6.4: Response sidebar
**Description:**
  - Display AI response in a **sidebar** (not modal - doesn't block terminal)
  - Sidebar appears on right side of terminal
  - Terminal splits: 60% main terminal | 40% AI sidebar
  - Support markdown rendering if possible
  - Allow scrolling for long responses
  - Copy suggestion to clipboard option
  - Close with Esc or q

**Definition of Done:**
  - ✅ `pkg/ui/sidebar.go` implements sidebar viewer
  - ✅ Sidebar appears on right side without blocking main terminal
  - ✅ User can continue typing commands while sidebar is visible
  - ✅ Markdown rendering (basic: bold, code blocks)
  - ✅ Scroll sidebar with ↑↓ or PgUp/PgDn
  - ✅ Clipboard copy with `y` key
  - ✅ Close with Esc or q
  - ✅ **Unit tests**: Markdown parsing
  - ✅ **Unit tests**: Scroll logic
  - ✅ **Unit tests**: Split layout rendering
  - ✅ Manual test: View long response, type commands while sidebar open, close with command

**Phase Verification**: Run a failing command, type `/wtf`, get helpful AI suggestion.

---

### Phase 7: Polish & Configuration
**Goal**: Make it production-ready

#### Task 7.1: Comprehensive configuration management
**Description:**
  - All settings in `~/.wtf_cli/config.json`
  - Runtime config updates (some settings)
  - Config migration for version upgrades
  - Settings: buffer size, context window, API key, model, UI preferences

**Definition of Done:**
  - ✅ Complete `Config` struct with all settings
  - ✅ Config version field for migrations
  - ✅ Migration logic for v1 → v2
  - ✅ **Unit tests**: Full config load/save cycle
  - ✅ **Unit tests**: Migration from old version
  - ✅ **Unit tests**: Partial config (missing fields use defaults)
  - ✅ Documentation: Configuration reference guide

#### Task 7.2: Error handling
**Description:**
  - Terminal state restoration on crash
  - Graceful degradation if API unavailable
  - User-friendly error messages
  - Log errors to `~/.wtf_cli/errors.log`
  - **Prevent nested instances**: Detect if already running inside wtf_cli

**Nested Instance Detection:**
  - Set environment variable `WTF_CLI_ACTIVE=1` on startup
  - Check for `WTF_CLI_ACTIVE` before spawning
  - Display friendly message: "Already running inside wtf_cli wrapper"
  - Exit gracefully with code 1

**Definition of Done:**
  - ✅ All error paths have user-friendly messages
  - ✅ API failures don't crash wrapper
  - ✅ PTY errors handled gracefully
  - ✅ Panic recovery in all goroutines
  - ✅ **Nested instance prevention**: Check environment variable
  - ✅ **Unit tests**: Error scenarios for each package
  - ✅ **Unit tests**: Nested instance detection
  - ✅ **Integration tests**: API timeout, network failure
  - ✅ Error log rotated (max 10MB)

#### Task 7.3: Performance optimization
**Description:**
  - Profile PTY forwarding latency
  - Optimize buffer operations
  - Ensure zero perceivable lag
  - Memory leak detection

**Definition of Done:**
  - ✅ Benchmark tests for critical paths
  - ✅ PTY latency < 1ms (p99)
  - ✅ Memory usage < 50MB under normal use
  - ✅ No memory leaks (run for 8 hours, memory stable)
  - ✅ **Benchmark tests**: Buffer operations
  - ✅ **Benchmark tests**: PTY I/O throughput
  - ✅ CPU profiling shows no hotspots

#### Task 7.4: Documentation
**Description:**
  - Installation guide
  - Configuration reference
  - Command reference
  - Troubleshooting guide
  - Architecture docs for contributors

**Definition of Done:**
  - ✅ README.md with quick start
  - ✅ `docs/installation.md` with OS-specific instructions
  - ✅ `docs/configuration.md` with all config options
  - ✅ `docs/commands.md` with slash command reference
  - ✅ `docs/troubleshooting.md` with common issues
  - ✅ `docs/architecture.md` with component diagrams
  - ✅ Code comments and godoc coverage > 80%

**Phase Verification**: Install on fresh system, configure, use for a day of normal work.

---

## 6. Success Criteria
- ✅ User can run `wtf_cli` and use their terminal exactly as before
- ✅ Status bar is visible but non-intrusive
- ✅ Exit codes are captured reliably (100% accuracy)
- ✅ `/` triggers command palette smoothly
- ✅ AI provides helpful suggestions based on last 1000 lines
- ✅ Zero perceivable latency during normal use
- ✅ Vim, htop, and other full-screen apps work perfectly
- ✅ Settings persist across sessions
- ✅ Unit test coverage > 80%
- ✅ All integration tests passing
