# Software Requirements for `wtf` CLI Utility

## 1. Introduction

The `wtf` (What The Failure/Follow-up) CLI utility is designed for Linux bash users. In future we will support macOS zsh, and Windows PowerShell. Its primary purpose is to analyze the last executed command, its output, and exit status to provide helpful suggestions for the user's next steps, troubleshoot errors, or offer relevant follow-up actions. The source of suggestions and follow-up actions is the LLM service.

## 2. Functional Requirements

### 2.1. Command History Access
*   **FR1.1:** The utility MUST be able to retrieve the last command executed by the user in the current bash session.

### 2.2. Command Output Access
*   **FR2.1:** The utility MUST be able to access the standard output (stdout) of the last executed command.
*   **FR2.2:** The utility MUST be able to access the standard error (stderr) of the last executed command.

### 2.3. Command Exit Status Access
*   **FR3.1:** The utility MUST be able to retrieve the exit code of the last executed command.

### 2.4. Analysis and Suggestion Engine
*   **FR4.1:** The utility MUST analyze the retrieved command, its output (stdout/stderr), and exit code to determine the context. It also should provide current OS, to understand more clearly the user environment.
*   **FR4.2:** Based on the analysis, the utility MUST leverage a Large Language Model (LLM) service to generate and display relevant suggestions to the user.
*   **FR4.4:** The utility MUST allow for configuration of the LLM service, including API endpoint and authentication details (e.g., API key).
*   **FR4.5:** The utility MUST securely store and manage LLM API keys.
*   **FR4.3:** Suggestions MAY include, but are not limited to:
    *   Corrections for common typos or syntax errors.
    *   Common follow-up commands for a successful operation.
    *   Troubleshooting steps or diagnostic commands if an error occurred (based on exit code and error messages).
    *   Links to relevant documentation, man pages, or online resources (e.g., Stack Overflow).
    *   Alternative commands or approaches.

### 2.5. User Interface
*   **FR5.1:** The utility MUST be invokable via a simple command (e.g., `wtf`) entered in the bash prompt.
*   **FR5.2:** The output (suggestions) MUST be displayed clearly in the terminal.
*   **FR5.3:** The utility SHOULD provide an option for users to get help on how to use `wtf` itself (e.g., `wtf --help`).

### 2.6. App Configuration
*   **FR6.1:** The utility MUST provide a mechanism for users to configure the API endpoint for the chosen LLM service.
*   **FR6.2:** The utility MUST provide a mechanism for users to configure their API key for the LLM service.
*   **FR6.3:** API key configuration SHOULD support environment variables or a configuration file.
*   **FR6.4:** App configuration, api keys, etc. should be stored in a .wtf directory in the user's home directory in json format in file config.json.
*   **FR6.5:** The utility MUST provide clear instructions on how to configure these settings.
*   **FR6.6:** For the primary supported LLM provider will be OpenRouter.ai as for now. Configuration instructions and defaults SHOULD be tailored for OpenRouter.ai initially.

## 3. Non-Functional Requirements

*   **NFR1.1 (Performance):** The utility SHOULD execute quickly. While LLM response times are a factor, the utility's own processing should be minimal. Overall suggestion time, including LLM interaction, should be acceptable to the user (e.g., within a few seconds for typical cases).
*   **NFR1.2 (Accuracy):** Suggestions provided SHOULD be highly relevant and accurate to the context of the last command.
*   **NFR1.3 (Ease of Use):** The utility SHOULD be easy to install and configure.
*   **NFR1.4 (Extensibility):** The suggestion logic SHOULD be designed in a way that allows for future expansion with new rules, patterns, or integrations.
*   **NFR1.5 (Compatibility):** The utility MUST be compatible with common Linux distributions (e.g., Ubuntu, Fedora, Debian) and standard bash versions (e.g., Bash 4.x+).
*   **NFR1.6 (Error Handling):** The utility MUST gracefully handle situations where it cannot retrieve necessary information (e.g., history not available, output capture failed) and provide informative error messages to the user.
*   **NFR1.7 (Resource Consumption):** The utility SHOULD be lightweight and not consume excessive system resources (CPU, memory).
*   **NFR1.8 (Security):** The utility MUST NOT introduce security vulnerabilities. It must handle command history, output data, and LLM API keys securely. API keys SHOULD NOT be hardcoded and SHOULD be stored in a secure manner (e.g., environment variables, a configuration file with appropriate permissions).
*   **NFR1.9 (Network Dependency):** The utility will depend on network connectivity to access the LLM service. It SHOULD handle network errors gracefully (e.g., timeouts, connection failures) and inform the user.
*   **NFR1.10 (Implementation Technology & Deliverable):** To ensure the utility can be distributed as a single, statically-linked executable binary with no external runtime dependencies (other than the OS itself), it WILL be implemented in the Go programming language.

## 4. Assumptions

*   **A1:** The user is operating within a bash shell environment.
*   **A2:** Bash history (`fc -ln -0`, `history 1`, or similar mechanisms) is enabled and accessible by the utility.
*   **A3:** The utility will primarily focus on common command-line tools and scenarios in its initial version.
*   **A4:** The utility will use an external LLM service for generating suggestions. The user is responsible for providing valid API credentials for this service.
*   **A5:** The user has network access to reach the configured LLM service.

## 5. Future Considerations (Out of Scope for V1)

*   **FC1:** Support for multiple LLM providers or local LLM models.
*   **FC2:** User-defined or community-contributed suggestion rules/plugins.
*   **FC3:** Support for other shells (e.g., zsh, fish).
*   **FC4:** Ability to learn from user feedback on suggestions.
*   **FC5:** Optional anonymous usage data collection to improve suggestions (with explicit user consent).
