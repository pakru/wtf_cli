# `wtf` - CLI Assistant

`wtf` is a command-line utility for Linux bash users designed to help you understand and act upon the output of your last executed command. By leveraging Large Language Models (LLMs), `wtf` analyzes your command history, output, and exit status to provide intelligent suggestions, troubleshooting tips, and relevant follow-up actions.

For detailed functional and non-functional requirements, please see the [SOFTWARE_REQUIREMENTS.md](SOFTWARE_REQUIREMENTS.md) file.

## Features

- 🔍 **Smart Analysis** - Analyzes your last command, output, and exit status
- 🤖 **LLM-Powered** - Uses OpenRouter.ai with GPT-4o for intelligent suggestions
- 🐧 **Linux Native** - Built specifically for Linux bash environments
- ⚡ **Single Binary** - No dependencies, just download and run
- 🔒 **Secure** - API keys stored securely in `~/.wtf/config.json`
- 📊 **Context Aware** - Includes system information for better suggestions

## Getting Started

### Prerequisites

- **Linux** (any modern distribution)
- **Bash** (version 4.x or higher recommended)
- **OpenRouter.ai API key** - Get one at [openrouter.ai](https://openrouter.ai)

### Installation

#### Option 1: Build from Source
```bash
# Clone the repository
git clone <repository-url>
cd wtf_cli

# Build the binary
make build

# Install to your PATH
sudo cp wtf /usr/local/bin/
```

#### Option 2: Using Go Install
```bash
go install github.com/your-username/wtf_cli@latest
```

#### Option 3: Download Binary
```bash
# Download the latest release (when available)
curl -L https://github.com/your-username/wtf_cli/releases/latest/download/wtf -o wtf
chmod +x wtf
sudo mv wtf /usr/local/bin/
```

## Configuration

On first run, `wtf` will create a configuration file at `~/.wtf/config.json`:

```json
{
  "llm_provider": "openrouter",
  "openrouter": {
    "api_key": "your_openrouter_api_key_here",
    "model": "openai/gpt-4o"
  }
}
```

### Setting up your API Key

1. **Get an OpenRouter.ai API key:**
   - Visit [openrouter.ai](https://openrouter.ai)
   - Sign up and get your API key

2. **Configure wtf:**
   ```bash
   # Run wtf once to create the config file
   wtf
   
   # Edit the config file
   nano ~/.wtf/config.json
   
   # Replace "your_openrouter_api_key_here" with your actual API key
   ```

3. **Available Models:**
   - `openai/gpt-4o` (default, recommended)
   - `openai/gpt-4o-mini` (faster, cheaper)
   - `anthropic/claude-3.5-sonnet`
   - See [OpenRouter models](https://openrouter.ai/models) for more options

## Usage

Simply run `wtf` after any command to get intelligent suggestions:

```bash
# Example 1: Command failed
$ ls /nonexistent/directory
ls: cannot access '/nonexistent/directory': No such file or directory
$ wtf
# wtf analyzes the error and suggests solutions

# Example 2: Successful command
$ git status
On branch main
Your branch is up to date with 'origin/main'.
$ wtf
# wtf might suggest next git actions like commit, push, etc.

# Example 3: System command
$ df -h
Filesystem      Size  Used Avail Use% Mounted on
/dev/sda1        20G   18G  1.2G  94% /
$ wtf
# wtf analyzes disk usage and suggests cleanup actions
```

### What wtf Analyzes

- **Last Command** - The command you just ran
- **Exit Code** - Whether it succeeded or failed
- **System Context** - OS type, distribution, kernel version
- **Command Output** - Available stdout/stderr (when possible)

## Development

### Building and Testing

```bash
# Install dependencies
make tidy

# Development workflow (format, vet, test, build)
make dev

# Run tests
make test

# Run tests with coverage
make test-coverage

# Clean build artifacts
make clean
```

### Available Make Targets

- `make build` - Build the wtf binary
- `make test` - Run tests
- `make test-coverage` - Run tests with coverage report
- `make clean` - Clean build artifacts
- `make run` - Build and run the application
- `make install` - Install binary to GOPATH/bin
- `make fmt` - Format code
- `make vet` - Run go vet
- `make lint` - Run golangci-lint (if available)
- `make dev` - Development workflow (fmt, vet, test, build)
- `make ci` - CI workflow (tidy, fmt-check, vet, test, build)
- `make help` - Show all available targets

### Project Structure

```
wtf_cli/
├── main.go              # Entry point
├── config/              # Configuration management
│   ├── config.go
│   └── config_test.go
├── shell/               # Shell history access
│   ├── history.go
│   └── history_test.go
├── system/              # System information
│   ├── info.go
│   └── info_test.go
├── Makefile            # Build automation
├── go.mod              # Go modules
└── README.md           # This file
```

## Troubleshooting

### Common Issues

**"Configuration error: API key is required"**
- Make sure you've set your OpenRouter.ai API key in `~/.wtf/config.json`

**"Failed to get last command"**
- Ensure bash history is enabled: `set +H` (if disabled)
- Check that `HISTFILE` is set and accessible

**"Network error"**
- Check your internet connection
- Verify your API key is valid
- Check if OpenRouter.ai is accessible

### Debug Mode

```bash
# Run with verbose output (when implemented)
wtf --debug

# Check configuration
cat ~/.wtf/config.json
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests (`make test`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### Development Guidelines

- Follow Go conventions and use `gofmt`
- Write tests for new functionality
- Update documentation as needed
- Run `make dev` before committing

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [OpenRouter.ai](https://openrouter.ai) for LLM API access
- Go community for excellent tooling
- All contributors who help improve this tool
