# wtf_cli Project Plan

## Project Overview
The `wtf` CLI utility is designed for Linux bash users to analyze the last executed command, its output, and exit status. It leverages Large Language Models (LLMs) via OpenRouter AI to provide intelligent suggestions, troubleshooting tips, and relevant follow-up actions.

## Current Status
- [x] Basic project structure set up (main.go, go.mod)
- [x] Requirements documented in SOFTWARE_REQUIREMENTS.md
- [x] README.md with project overview
- [x] Makefile with build automation tasks
- [x] Configuration management implemented
- [x] Shell history access implemented
- [x] System information gathering implemented
- [x] Debug mode and structured logging implemented
- [x] Environment variable configuration overrides
- [x] Dry-run mode for development/testing
- [ ] LLM integration (core functionality pending)

## Implementation Tasks

### References
- [OpenRouter AI Quickstart Guide](https://openrouter.ai/docs/quickstart)

### 1. Configuration Management
- [x] Create a configuration structure
- [x] Implement loading from ~/.wtf/config.json
- [x] Add secure API key storage
- [x] Add configuration validation
- [x] Add debug and dry-run configuration options
- [x] Implement environment variable overrides

### 2. Logging and Debug Infrastructure
- [x] Implement structured logging with slog
- [x] Add debug mode with detailed workflow logging
- [x] Create dry-run mode for safe testing
- [x] Support multiple log levels (debug, info, warn, error)
- [x] Environment variable configuration (WTF_DEBUG, WTF_DRY_RUN, etc.)

### 3. Command History Access
- [x] Implement retrieval of the last command from bash history
- [x] Capture stdout and stderr of the last command
- [x] Retrieve the exit code of the last command
- [x] Add environment variable support for shell integration
- [x] Implement command simulation for testing (WTF_LAST_*)
- [x] Add intelligent exit code inference from command patterns

### 4. System Information
- [x] Detect and report the current OS
- [x] Gather relevant system information for context

### 5. LLM Integration (OpenRouter AI)
- [ ] Create the payload structure for OpenRouter.ai API
- [ ] Implement API call functionality following [OpenRouter Quickstart Guide](https://openrouter.ai/docs/quickstart)
- [ ] Handle response parsing
- [ ] Implement error handling for API calls
- [x] Mock responses for dry-run mode

### 6. User Interface
- [ ] Display suggestions in a user-friendly format
- [ ] Add color-coding for better readability
- [ ] Implement error handling for various failure scenarios
- [x] Basic dry-run mode output implemented

### 7. Documentation
- [x] Update README with installation instructions
- [x] Add usage examples
- [x] Document configuration options
- [x] Add troubleshooting section
- [x] Document debug mode and environment variables
- [ ] Add API integration examples

### 8. Testing
- [x] Unit tests for core functionality
- [ ] Integration tests for LLM API interaction
- [ ] End-to-end testing
- [x] Debug mode for development testing

## Development Approach
1. Implement core functionality incrementally
2. Focus on robust error handling
3. Ensure secure handling of API keys
4. Maintain lightweight resource usage

## Future Enhancements (v2+)
- Support for multiple LLM providers (currently using OpenRouter AI)
- User-defined suggestion rules
- Support for other shells (zsh, fish)
- Learning from user feedback
- Optional anonymous usage data collection