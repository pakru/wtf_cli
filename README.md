# WTF CLI - Terminal Wrapper with AI Assistance

A transparent PTY-based terminal wrapper that captures all terminal I/O for AI-powered assistance. Built with Go and the Bubble Tea TUI framework.

## 🚀 Getting Started

### Prerequisites

- Go 1.21+
- Linux or macOS

### Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/wtf_cli.git
cd wtf_cli

# Build
make build

# Run
./wtf_cli
```

### Usage

```bash
# Start the wrapper
./wtf_cli

# Use your terminal normally
ls -la
cd ~/projects

# Press / at an empty prompt to open command palette
# Select a command with arrow keys and Enter
```

## ✨ Features

### Implemented ✅

- **Transparent PTY Wrapper**: Seamless terminal proxy that feels native
- **Full Terminal Support**: Works with vim, htop, nano, and all terminal apps
- **Signal Handling**: Proper SIGWINCH (resize), SIGINT, SIGTERM support
- **Circular Buffer**: Captures last 2000 lines of terminal output
- **Modern TUI**: Built with Charm's Bubble Tea and Lipgloss
- **Status Bar**: Shows current directory and helpful hints
- **Command Palette**: Press `/` to access AI commands
- **Welcome Banner**: Helpful shortcut reference on startup

### Commands (Available)

| Command | Description |
|---------|-------------|
| `/wtf` | Analyze last output and suggest fixes |
| `/explain` | Explain what the last command did |
| `/fix` | Suggest fix for last error |
| `/history` | Show command history |
| `/help` | Show help |

### Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+D` | Exit terminal |
| `Ctrl+C` | Cancel current command |
| `Ctrl+Z` | Suspend process |
| `/` | Open command palette (at empty prompt) |
| `Esc` | Close palette/panel |


### Tech Stack

- **Go** - Core language
- **[creack/pty](https://github.com/creack/pty)** - Pseudo-terminal management
- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** - TUI framework
- **[Lipgloss](https://github.com/charmbracelet/lipgloss)** - Styling
- **OpenRouter API** - LLM integration (coming soon)


## 🧪 Development

### Available Make Targets

```bash
# Run all checks, build, and test (default)
make all

# Build the binary
make build

# Run all tests
make test

# Format all Go code
make fmt

# Run static analysis
make vet

# Run formatting and vetting
make lint

# Full pre-commit validation (fmt, vet, build, test)
make check

# Clean build artifacts
make clean

# Build and run the application
make run

# Show help with all available targets
make help

# Run directly without make
go run cmd/wtf_cli/main.go
```

### Code Quality

Before committing, run:
```bash
make check
```

This will automatically:
1. Format your code with `go fmt`
2. Run `go vet` for static analysis
3. Build the project
4. Run all tests

## 🔧 Troubleshooting

### Go Version Mismatch Error

If you see an error like:
```
compile: version "go1.25.0" does not match go tool version "go1.25.5"
```

This happens when Go is updated but cached packages are from an older version. Fix it with:

```bash
# Clear all caches and reinstall standard library
rm -rf ~/.cache/go-build
go clean -cache -modcache -i -r
go install std

# Rebuild the project
make clean
make build
```

## 📝 Configuration

Configuration file: `~/.wtf_cli/config.json`

```json
{
  "llm_provider": "openrouter",
  "openrouter": {
    "api_key": "<your_api_key>",
    "model": "google/gemini-2.0-flash-exp:free",
    "temperature": 0.7,
    "max_tokens": 2000
  },
  "buffer_size": 2000,
  "context_window": 1000,
  "status_bar": {
    "position": "bottom"
  }
}
```

## 🤝 Contributing

Contributions are welcome! Please read the contribution guidelines first.

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.
