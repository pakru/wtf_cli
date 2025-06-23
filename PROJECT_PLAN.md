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
- [ ] LLM integration (core functionality pending)

## Implementation Tasks

### References
- [OpenRouter AI Quickstart Guide](https://openrouter.ai/docs/quickstart)

### 1. Configuration Management
- [x] Create a configuration structure
- [x] Implement loading from ~/.wtf/config.json
- [x] Add secure API key storage
- [x] Add configuration validation

### 2. Command History Access
- [x] Implement retrieval of the last command from bash history
- [x] Capture stdout and stderr of the last command
- [x] Retrieve the exit code of the last command

### 3. System Information
- [x] Detect and report the current OS
- [x] Gather relevant system information for context

### 4. LLM Integration (OpenRouter AI)
- [ ] Create the payload structure for OpenRouter.ai API
- [ ] Implement API call functionality following [OpenRouter Quickstart Guide](https://openrouter.ai/docs/quickstart)
- [ ] Handle response parsing
- [ ] Implement error handling for API calls

### 5. User Interface
- [ ] Display suggestions in a user-friendly format
- [ ] Add color-coding for better readability
- [ ] Implement error handling for various failure scenarios

### 6. Documentation
- [ ] Update README with installation instructions
- [ ] Add usage examples
- [ ] Document configuration options
- [ ] Add troubleshooting section

### 7. Testing
- [x] Unit tests for core functionality
- [ ] Integration tests for LLM API interaction
- [ ] End-to-end testing

### 8. Build System
- [x] Create Makefile with common development tasks
- [x] Add build, test, clean, and development workflow targets
- [x] Include code formatting and linting tasks
- [x] Add CI-friendly targets for automated builds

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