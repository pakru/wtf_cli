# WTF CLI - Terminal Wrapper with AI Assistance

A transparent PTY-based terminal wrapper that captures all terminal I/O for AI-powered assistance. Built with Go and the Bubble Tea TUI framework.

![WTF CLI Demo](docs/demo.gif)

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

## 📦 Project Status

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 1 | Basic PTY Wrapper | ✅ Complete |
| Phase 2 | Output Capture & Buffering | ✅ Complete |
| Phase 3 | Session Context Tracking | ✅ Complete |
| Phase 4 | Bubble Tea TUI Integration | ✅ Complete |
| Phase 5 | Slash Command System | ✅ Complete |
| Phase 6 | AI Integration (OpenRouter) | 🚧 Next |
| Phase 7 | Polish & Configuration | 📋 Planned |

## 🏗️ Architecture

```
wtf_cli/
├── cmd/wtf_cli/          # Main entry point
├── pkg/
│   ├── pty/              # PTY management, signals, streaming
│   ├── buffer/           # Circular buffer for output capture
│   ├── capture/          # Session context tracking
│   ├── ui/               # Bubble Tea TUI components
│   │   ├── model.go      # Main Bubble Tea model
│   │   ├── viewport.go   # PTY output viewport
│   │   ├── statusbar.go  # Status bar component
│   │   ├── palette.go    # Command palette
│   │   └── ...
│   ├── commands/         # Slash command handlers
│   └── config/           # Configuration management
└── docs/                 # Documentation
```

### Technologies

- **Go** - Core language
- **[creack/pty](https://github.com/creack/pty)** - Pseudo-terminal management
- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** - TUI framework
- **[Lipgloss](https://github.com/charmbracelet/lipgloss)** - Styling
- **OpenRouter API** - LLM integration (coming soon)

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

## 🧪 Development

```bash
# Build
make build

# Run all tests
make test

# Clean build artifacts
make clean

# Run directly
go run cmd/wtf_cli/main.go
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

---

**Built with ❤️ using Go and Charm**
