# `wtf` - CLI Assistant

`wtf` is a command-line utility for sell designed to help you understand and act upon the output of your last executed command. By leveraging Large Language Models (LLMs), `wtf` analyzes your last executed commandand exit status to provide suggestions, troubleshooting tips, and relevant follow-up actions.

## Getting Started

### Prerequisites

- **Linux** (any modern distribution)
- **Bash** (version 4.x or higher recommended)
- **GoLang** (version 1.22 or higher recommended)
- **OpenRouter.ai API key** - Get one at [openrouter.ai](https://openrouter.ai)

### Installation

#### Option 1: Automated Installation (Recommended)
```bash
# Clone the repository
git clone <repository-url>
cd wtf_cli

# Automated installation (recommended)
./scripts/install.sh
```

This will:
- Build and install the WTF CLI binary to `~/.local/bin/wtf`
- Set up shell integration for automatic command capture
- Create default configuration file
- Add necessary entries to your `~/.bashrc`

#### Option 2: Manual Build from Source
```bash
# Clone the repository
git clone <repository-url>
cd wtf_cli

# Build the binary
make build

# Install to your PATH
sudo cp wtf /usr/local/bin/
```
### Uninstallation

To completely remove WTF CLI:

```bash
# Using the installation script
./scripts/install.sh uninstall
```

This will:
- Remove the WTF CLI binary
- Remove the `~/.wtf` directory (with confirmation)
- Remove shell integration from `~/.bashrc`
- Restore your original bash configuration

## Configuration

On first run, `wtf` will create a configuration file at `~/.wtf/config.json`:

```json
{
  "llm_provider": "openrouter",
  "openrouter": {
    "api_key": "<your_openrouter_api_key_here>",
    "model": "google/gemma-3-27b",
    "temperature": 0.7,
    "max_tokens": 1000
  },
  "dry_run": false,
  "log_level": "info"
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
### Environment Variables

You can override configuration settings using environment variables:

```bash
# API Configuration
export WTF_API_KEY="your_api_key_here"        # Override API key
export WTF_MODEL="openai/gpt-4o-mini"         # Override model

# Debug and Development
export WTF_DEBUG=true                         # Enable debug mode
export WTF_DRY_RUN=true                       # Enable dry-run (no API calls)
export WTF_LOG_LEVEL=debug                    # Set log level (debug, info, warn, error)

# Shell Integration (for advanced users)
export WTF_LAST_COMMAND="ls /nonexistent"     # Override last command
export WTF_LAST_EXIT_CODE="2"                # Override last exit code
export WTF_LAST_OUTPUT="error output here"    # Override command output
```

**Debug Mode Features:**
- Detailed logging of internal operations
- Human-readable log format
- Step-by-step workflow visibility

**Dry-Run Mode:**
- No actual API calls made
- Mock responses for testing
- Safe for development and testing
- Useful when API key is not available

**Shell Integration:**
- Real-time command capture with automatic shell hooks
- JSON-based command data storage in `~/.wtf/last_command.json`
- Use `WTF_LAST_*` variables to override for testing
- Automatic installation via `./scripts/install.sh`

## Usage

Simply run `wtf` after any command to get intelligent suggestions:

```bash
# Example 1: Command failed
$ ls /nonexistent/directory
ls: cannot access '/nonexistent/directory': No such file or directory
$ wtf
# wtf analyzes the error and suggests solutions

# Example 2: Development/Testing with dry-run
$ export WTF_DRY_RUN=true
$ wtf
# Shows mock response without API calls

# Example 3: Debug mode for troubleshooting
$ export WTF_DEBUG=true WTF_LOG_LEVEL=debug
$ wtf
# Shows detailed debug information

# Example 4: Testing with simulated commands
$ WTF_LAST_COMMAND="ls /nonexistent" WTF_LAST_EXIT_CODE="2" WTF_DRY_RUN=true wtf
# Simulates a failed command for testing

# Example 5: Testing successful commands
$ WTF_LAST_COMMAND="git status" WTF_LAST_EXIT_CODE="0" WTF_DRY_RUN=true wtf
# Simulates a successful command

# Example 6: Pipe usage (with shell integration)
$ ls /nonexistent | wtf
# wtf analyzes both the command 'ls /nonexistent' and its error output

# Example 7: Pipe usage with any command
$ git status | wtf
$ docker ps | wtf
$ curl https://api.example.com | wtf
# wtf can analyze output from any command when piped
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

# Generate coverage reports
make coverage-report      # Text format with function breakdown
make coverage-html        # Interactive HTML report

# Clean build artifacts
make clean
```

### Docker Testing

For isolated testing in a containerized environment:

```bash
# Build Docker test image
make docker-build

# Run interactive container using Docker directly
docker run --rm -it wtf-cli-test:latest

# Or use Docker Compose for convenience
docker-compose -f docker/docker-compose.yml build
docker-compose -f docker/docker-compose.yml run --rm test
```

The Docker environment includes:
- Pre-installed WTF CLI with shell integration
- Ubuntu 24.04 base with all required tools
- `WTF_DRY_RUN=true` environment variable for safe testing
- Non-root `tester` user with sudo access

### Available Make Targets

- `make build` - Build the wtf binary
- `make test` - Run tests
- `make test-coverage` - Run tests with coverage profile
- `make coverage-report` - Generate text coverage report
- `make coverage-html` - Generate HTML coverage report
- `make clean` - Clean build artifacts
- `make run` - Build and run the application
- `make install` - Install binary to GOPATH/bin
- `make install-full` - Full installation with shell integration
- `make uninstall` - Uninstall WTF CLI completely
- `make docker-build` - Build Docker test image (requires binary to be built first)
- `make fmt` - Format code
- `make vet` - Run go vet
- `make lint` - Run golangci-lint (if available)
- `make dev` - Development workflow (fmt, vet, test, build)
- `make ci` - CI workflow (tidy, fmt-check, vet, test, build)
- `make help` - Show all available targets

## Troubleshooting

### Common Issues

**"Configuration error: API key is required"**
- Make sure you've set your OpenRouter.ai API key in `~/.wtf/config.json`
- Or use environment variable: `export WTF_API_KEY=your_api_key`
- Or run in dry-run mode: `export WTF_DRY_RUN=true`

**"Failed to get last command"**
- Ensure bash history is enabled: `set +H` (if disabled)
- Check that `HISTFILE` is set and accessible

**"Network error"**
- Check your internet connection
- Verify your API key is valid
- Check if OpenRouter.ai is accessible

### Debug Mode

```bash
# Enable debug mode with environment variables
export WTF_DEBUG=true
export WTF_LOG_LEVEL=debug
wtf

# Run in dry-run mode (no API calls)
export WTF_DRY_RUN=true
wtf

# Check configuration
cat ~/.wtf/config.json

# Test with mock responses
WTF_DRY_RUN=true wtf
```

**Debug Output Includes:**
- Configuration loading details
- Shell command retrieval process
- System information gathering
- API call preparation (when not in dry-run)
- Structured logging with timestamps

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
