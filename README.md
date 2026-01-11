# WTF CLI - Terminal Wrapper with AI Assistance

A transparent PTY-based terminal wrapper that captures all terminal I/O for AI-powered assistance.

## Features (In Development)

- **Transparent PTY Wrapper**: Intercepts all terminal input/output without disrupting the user experience
- **AI-Powered Assistance**: Get contextual help via slash commands (`/wtf`, `/explain`, `/fix`)
- **Session Recording**: Circular buffer captures last N lines for context
- **Status Bar**: Minimal UI showing current directory and hints
- **Sidebar View**: Non-blocking AI responses in a terminal sidebar

## Project Status

🚧 **Phase 1**: Basic PTY wrapper (In Progress)

## Architecture

Built from scratch in Go using:
- `creack/pty` for pseudo-terminal management
- Custom TUI for status bar and sidebar
- OpenRouter API for LLM integration

## Development

```bash
# Build
make build

# Run
./wtf_cli

# Test
make test
```

## License

TBD
