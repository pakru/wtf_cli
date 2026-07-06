<h1 align="center">💻 WTF CLI - AI Assisted Terminal</h1>

A transparent PTY-based terminal wrapper with AI-powered assistance. Get instant explanations for errors, suggestions for fixes, and interactive chat with an AI that sees your terminal context.

[![Build Status](https://github.com/pakru/wtf_cli/actions/workflows/go.yml/badge.svg)](https://github.com/pakru/wtf_cli/actions/workflows/go.yml)
[![Go Version](https://img.shields.io/badge/go-1.26%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![Latest Release](https://img.shields.io/github/v/release/pakru/wtf_cli?include_prereleases&label=release)](https://github.com/pakru/wtf_cli/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS-lightgrey?logo=linux&logoColor=white)](https://github.com/pakru/wtf_cli/releases)

![WTF CLI Interface](docs/images/wtf_cli_interface.png)

## 🚀 Getting Started

### Requirements

- Go 1.26+
- Linux or macOS
- Supported LLM providers:
  - OpenAI
  - Anthropic
  - OpenRouter
  - Google AI
  - Copilot

### Installation

#### Quick Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/pakru/wtf_cli/main/install.sh | bash
```

This will automatically detect your platform, download the latest release, verify checksums, and install to `~/.local/bin`.

#### Option 1: Download Pre-built Binary

Download the latest release for your platform from the [Releases page](https://github.com/pakru/wtf_cli/releases):

```bash
# Linux/macOS example
wget https://github.com/pakru/wtf_cli/releases/latest/download/wtf_cli_<version>_<os>_<arch>.tar.gz
tar -xzf wtf_cli_<version>_<os>_<arch>.tar.gz
chmod +x wtf_cli
./wtf_cli --version

# Optionally install to PATH
mkdir -p ~/.local/bin
mv wtf_cli ~/.local/bin/
```

#### Option 2: Build from Source

```bash
# Clone the repository
git clone https://github.com/pakru/wtf_cli.git
cd wtf_cli

# Build
make build

# Run
./wtf_cli
```

### Usage

```bash
# Check version
./wtf_cli --version

# Start the wrapper
./wtf_cli

# Use your terminal normally
```

## ✨ Features

When `/chat` or `/explain` needs to inspect a file, it can only read inside your current directory by default. If it needs a file elsewhere (e.g. `/etc/hosts` or `~/.bashrc`), it asks first — approve once, or for the rest of the session for that specific folder. Set `agent.tools.out_of_workdir_access` to `"deny"` in `~/.wtf_cli/config.json` to disable this and keep the AI strictly confined to the current directory (see [AGENTS.md](AGENTS.md#configuration) for the full config reference).

### Commands (Available)

| Command | Description |
|---------|-------------|
| `/chat` | Toggle AI chat sidebar |
| `/explain` | Analyze last output and suggest fixes |
| `/history` | Show command history |
| `/settings` | Open settings panel |
| `/help` | Show help |

### Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+D` | Exit terminal (press twice) |
| `Ctrl+C` | Cancel current command |
| `Ctrl+Z` | Suspend process |
| `Ctrl+R` | Search command history |
| `Ctrl+T` | Toggle AI chat sidebar |
| `Shift+Tab` | Switch focus between terminal and chat sidebar |
| `/` | Open command palette (at empty prompt) |
| `Esc` | Close palette/panel/sidebar |
| `←`/`→` | Move cursor in command line |
| `Home`/`End` | Jump to start/end of command line |

### Tech Stack

- **Go** - Core language
- **[creack/pty](https://github.com/creack/pty)** - Pseudo-terminal management
- **[Bubble Tea v2](https://github.com/charmbracelet/bubbletea)** - TUI framework
- **[Lipgloss v2](https://github.com/charmbracelet/lipgloss)** - Styling
- **[vito/midterm](https://github.com/vito/midterm)** - Full-screen app terminal emulation (vim, htop, etc.)
- **[go-git](https://github.com/go-git/go-git)** - Git integration (branch display in status bar)
- **AI Providers** - OpenRouter, OpenAI, Anthropic, Google Gemini, GitHub Copilot


## 🤝 Contributing

Contributions are welcome! Please read the contribution guidelines first.

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.
