# CLI Command Capture Architecture Design

## Overview

This document outlines various approaches for capturing shell command information (command text, exit codes, output) for CLI diagnostic tools like WTF CLI. We analyze different architectural solutions, their trade-offs, and implementation details.

## Table of Contents

1. [Problem Statement](#problem-statement)
2. [Architecture Solutions](#architecture-solutions)
3. [Shell Integration (Recommended)](#shell-integration-recommended)
4. [Fallback Methods](#fallback-methods)
5. [Alternative Approaches](#alternative-approaches)
6. [Comparison Matrix](#comparison-matrix)
7. [Implementation Details](#implementation-details)
8. [Best Practices](#best-practices)

## Problem Statement

### Core Challenge
CLI diagnostic tools need to capture information about the **last executed command** to provide intelligent troubleshooting suggestions. This requires:

1. **Command Text**: Exact command that was executed
2. **Exit Code**: Success/failure status (0 = success, non-zero = failure)
3. **Command Output**: stdout/stderr for context analysis
4. **Execution Context**: Working directory, timing, environment
5. **Real-time Access**: Information available immediately after command execution

### Technical Constraints
- **Process Isolation**: CLI tools run as separate processes from the shell
- **History Delays**: Shell history files are not updated immediately
- **Subprocess Limitations**: `exec.Command()` creates new shell sessions without access to parent shell state
- **Cross-Shell Compatibility**: Different shells (bash, zsh, fish) have different mechanisms

## Architecture Solutions

### 1. Shell Integration (Recommended)

**Concept**: Hook directly into the shell's command execution lifecycle using built-in shell mechanisms.

#### Integration Points

```bash
# Command Lifecycle Hooks
User Input → DEBUG Trap → Command Execution → PROMPT_COMMAND → Next Prompt
     ↓           ↓              ↓                    ↓
   "ls -la"  Capture Cmd    Execute & Fail      Capture Exit Code
```

#### Technical Implementation

**A. Bash Hook Mechanisms**

```bash
# 1. DEBUG Trap - Fires BEFORE command execution
trap 'wtf_pre_command' DEBUG

wtf_pre_command() {
    WTF_COMMAND_START=$(date +%s.%N)     # High-precision timestamp
    WTF_LAST_COMMAND="$BASH_COMMAND"    # Exact command text
}

# 2. PROMPT_COMMAND - Fires AFTER command execution
PROMPT_COMMAND="wtf_prompt_command; $PROMPT_COMMAND"

wtf_prompt_command() {
    local exit_code=$?                   # Actual exit code
    wtf_capture_command "$WTF_LAST_COMMAND" "$WTF_COMMAND_START"
}
```

**B. Data Storage Format**

```json
{
    "command": "ls /nonexistent",
    "exit_code": 2,
    "start_time": "1703123456.789123456",
    "end_time": "1703123456.891234567",
    "duration": 0.102111111,
    "pwd": "/home/user/project",
    "timestamp": "2023-12-20T15:30:56-08:00"
}
```

**C. Integration Workflow**

```
Shell Startup:
├── ~/.bashrc loaded
├── source ~/.wtf/integration.sh
├── Set up DEBUG trap
├── Modify PROMPT_COMMAND
└── Ready for command capture

Command Execution:
├── User types: "git push origin main"
├── DEBUG trap: Capture command + start time
├── Command executes (may fail)
├── PROMPT_COMMAND: Capture exit code + save JSON
└── Data available in ~/.wtf/last_command.json

WTF Analysis:
├── User types: "wtf"
├── Read ~/.wtf/last_command.json
├── Parse command metadata
├── Send to LLM with full context
└── Display intelligent suggestions
```

#### Advantages
- ✅ **Real-time**: No delays, works immediately
- ✅ **Accurate**: Actual exit codes from `$?`
- ✅ **Complete**: Access to `$BASH_COMMAND`, timing, context
- ✅ **Reliable**: Same process context as user commands
- ✅ **Extensible**: Can capture output, environment variables

#### Disadvantages
- ❌ **Setup Required**: User must install shell integration
- ❌ **Shell-Specific**: Different implementations for bash/zsh/fish
- ❌ **Potential Conflicts**: May interfere with other shell customizations

### 2. Fallback Methods

When shell integration is not available, several fallback approaches can be used:

#### A. History File Reading

```go
func getCommandFromHistoryFile() (string, error) {
    homeDir, _ := os.UserHomeDir()
    historyFile := filepath.Join(homeDir, ".bash_history")
    
    // Read last line from history file
    content, err := os.ReadFile(historyFile)
    if err != nil {
        return "", err
    }
    
    lines := strings.Split(string(content), "\n")
    for i := len(lines) - 1; i >= 0; i-- {
        if strings.TrimSpace(lines[i]) != "" {
            return strings.TrimSpace(lines[i]), nil
        }
    }
    return "", fmt.Errorf("no commands found")
}
```

**Limitations:**
- History file only updated on shell exit or `history -a`
- No access to exit codes or output
- May show stale commands

#### B. FC Command Attempts

```go
func getCommandWithFC() (string, error) {
    commands := []string{
        "fc -ln -1",                    // Last command, no line numbers
        "bash -i -c 'fc -ln -1'",      // Interactive shell
        "bash -c 'set -o history; fc -ln -1'", // Enable history
    }
    
    for _, cmdStr := range commands {
        if result := tryCommand(cmdStr); result != "" {
            return result, nil
        }
    }
    return "", fmt.Errorf("fc command failed")
}
```

**Limitations:**
- Creates new shell sessions without access to parent history
- Inconsistent behavior across different bash configurations
- No exit code information

#### C. Environment Variable Overrides

```go
// For testing and development
if envCmd := os.Getenv("WTF_LAST_COMMAND"); envCmd != "" {
    cmd.Command = envCmd
    if envExitCode := os.Getenv("WTF_LAST_EXIT_CODE"); envExitCode != "" {
        cmd.ExitCode, _ = strconv.Atoi(envExitCode)
    }
    return cmd, nil
}
```

**Use Cases:**
- Development and testing
- CI/CD environments
- Scripted scenarios

### 3. Alternative Approaches

#### A. Process Monitoring

**Concept**: Monitor system processes to detect command execution.

```go
// Hypothetical implementation
func monitorProcesses() {
    // Use /proc filesystem or system calls
    // Monitor parent shell PID for child processes
    // Capture command line arguments
}
```

**Challenges:**
- Complex implementation
- Platform-specific code
- Security/permission issues
- High resource usage

#### B. Shell Wrapper Scripts

**Concept**: Replace common commands with wrapper scripts that log execution.

```bash
# /usr/local/bin/ls (wrapper)
#!/bin/bash
echo "ls $@" > ~/.wtf/last_command.txt
echo $? > ~/.wtf/last_exit_code.txt
exec /bin/ls "$@"
```

**Challenges:**
- Must wrap every possible command
- PATH manipulation required
- Breaks some command behaviors
- Maintenance nightmare

#### C. Terminal Emulator Integration

**Concept**: Integrate with terminal emulators to capture command I/O.

**Examples:**
- iTerm2 shell integration
- Windows Terminal command palette
- Custom terminal emulator modifications

**Challenges:**
- Terminal-specific implementations
- Limited cross-platform support
- Requires terminal emulator support

#### D. Kernel-Level Tracing

**Concept**: Use kernel tracing mechanisms (eBPF, ptrace, etc.).

```c
// eBPF program to trace execve system calls
int trace_execve(struct pt_regs *ctx) {
    // Capture command execution at kernel level
    char comm[TASK_COMM_LEN];
    bpf_get_current_comm(&comm, sizeof(comm));
    // Log to ring buffer
}
```

**Challenges:**
- Requires root privileges
- Complex implementation
- Platform-specific
- Overkill for most use cases

## Comparison Matrix

| Approach | Real-time | Exit Codes | Output | Setup | Reliability | Cross-Platform |
|----------|-----------|------------|--------|-------|-------------|----------------|
| **Shell Integration** | ✅ Immediate | ✅ Accurate | ✅ Optional | ⚠️ Required | ✅ High | ⚠️ Shell-specific |
| **History File** | ❌ Delayed | ❌ None | ❌ None | ✅ None | ⚠️ Medium | ✅ Good |
| **FC Commands** | ❌ Limited | ❌ None | ❌ None | ✅ None | ❌ Low | ⚠️ Bash-only |
| **Env Variables** | ✅ Immediate | ✅ Available | ✅ Available | ✅ None | ✅ High | ✅ Excellent |
| **Process Monitor** | ✅ Real-time | ⚠️ Limited | ⚠️ Limited | ❌ Complex | ⚠️ Medium | ❌ Platform-specific |
| **Kernel Tracing** | ✅ Real-time | ✅ Available | ✅ Available | ❌ Root required | ✅ High | ❌ Platform-specific |

## Implementation Details

### Shell Integration Setup

#### 1. Integration Script Structure

```bash
#!/bin/bash
# ~/.wtf/integration.sh

# Configuration
WTF_DATA_DIR="$HOME/.wtf"
mkdir -p "$WTF_DATA_DIR"

# Core capture function
wtf_capture_command() {
    local exit_code=$?
    local command="$1"
    local start_time="$2"
    local end_time=$(date +%s.%N)
    
    # Skip wtf commands to avoid recursion
    [[ -z "$command" || "$command" =~ ^wtf.* ]] && return
    
    # Create JSON with command metadata
    cat > "$WTF_DATA_DIR/last_command.json" << EOF
{
    "command": "$command",
    "exit_code": $exit_code,
    "start_time": "$start_time",
    "end_time": "$end_time",
    "duration": $(echo "$end_time - $start_time" | bc -l 2>/dev/null || echo "0"),
    "pwd": "$PWD",
    "timestamp": "$(date -Iseconds)"
}
EOF
}

# Pre-command hook
wtf_pre_command() {
    WTF_COMMAND_START=$(date +%s.%N)
    WTF_LAST_COMMAND="$BASH_COMMAND"
}

# Post-command hook
wtf_post_command() {
    wtf_capture_command "$WTF_LAST_COMMAND" "$WTF_COMMAND_START"
}

# Set up hooks
trap 'wtf_pre_command' DEBUG

# PROMPT_COMMAND integration
if [[ "$PROMPT_COMMAND" ]]; then
    PROMPT_COMMAND="wtf_post_command; $PROMPT_COMMAND"
else
    PROMPT_COMMAND="wtf_post_command"
fi
```

#### 2. Go Integration Code

```go
// CommandInfo represents captured command data
type CommandInfo struct {
    Command   string    `json:"command"`
    Output    string    `json:"output"`
    ExitCode  int       `json:"exit_code"`
    Duration  float64   `json:"duration,omitempty"`
    PWD       string    `json:"pwd,omitempty"`
    Timestamp time.Time `json:"timestamp,omitempty"`
}

// GetLastCommand with shell integration priority
func GetLastCommand() (CommandInfo, error) {
    // 1. Environment variables (testing/override)
    if cmd := getFromEnvironment(); cmd.Command != "" {
        return cmd, nil
    }
    
    // 2. Shell integration (preferred)
    if cmd, err := getFromShellIntegration(); err == nil {
        return cmd, nil
    }
    
    // 3. Fallback methods
    return getFromFallbackMethods()
}

func getFromShellIntegration() (CommandInfo, error) {
    homeDir, _ := os.UserHomeDir()
    dataFile := filepath.Join(homeDir, ".wtf", "last_command.json")
    
    data, err := os.ReadFile(dataFile)
    if err != nil {
        return CommandInfo{}, fmt.Errorf("shell integration not active: %w", err)
    }
    
    var cmd CommandInfo
    if err := json.Unmarshal(data, &cmd); err != nil {
        return CommandInfo{}, fmt.Errorf("invalid shell integration data: %w", err)
    }
    
    return cmd, nil
}
```

#### 3. Installation Automation

```bash
#!/bin/bash
# install_integration.sh

WTF_DIR="$HOME/.wtf"
INTEGRATION_SCRIPT="$WTF_DIR/integration.sh"
BASHRC="$HOME/.bashrc"

echo "🔧 Installing WTF CLI Shell Integration..."

# Create directory
mkdir -p "$WTF_DIR"

# Copy integration script
cp "shell/integration.sh" "$INTEGRATION_SCRIPT"
chmod +x "$INTEGRATION_SCRIPT"

# Add to bashrc if not already present
if ! grep -q "wtf/integration.sh" "$BASHRC" 2>/dev/null; then
    cat >> "$BASHRC" << 'EOF'

# WTF CLI Integration
if [[ -f "$HOME/.wtf/integration.sh" ]]; then
    source "$HOME/.wtf/integration.sh"
fi
EOF
    echo "✅ Integration added to $BASHRC"
fi

echo "🎉 Installation complete! Run: source ~/.bashrc"
```

### Output Capture Implementation

#### Optional Output Capture

```bash
# Enable output capture for next command
wtf_capture_output() {
    echo "Output capture enabled for next command"
    exec 3>&1 4>&2  # Save original file descriptors
    exec 1> >(tee "$WTF_DATA_DIR/command_output.tmp")
    exec 2> >(tee "$WTF_DATA_DIR/command_output.tmp" >&2)
}

# Disable output capture
wtf_stop_capture() {
    exec 1>&3 2>&4  # Restore original file descriptors
    exec 3>&- 4>&-  # Close saved file descriptors
}

# Usage:
# $ wtf_capture_output
# $ some_failing_command
# $ wtf  # Will have access to command output
```

#### Automatic Output Capture (Advanced)

```bash
# Automatic output capture with size limits
wtf_auto_capture() {
    local max_size=1048576  # 1MB limit
    
    # Create named pipes for output capture
    mkfifo "$WTF_DATA_DIR/stdout_pipe" "$WTF_DATA_DIR/stderr_pipe"
    
    # Background processes to capture and limit output
    (head -c $max_size < "$WTF_DATA_DIR/stdout_pipe" > "$WTF_DATA_DIR/last_stdout.txt") &
    (head -c $max_size < "$WTF_DATA_DIR/stderr_pipe" > "$WTF_DATA_DIR/last_stderr.txt") &
    
    # Redirect command output
    exec 1> >(tee "$WTF_DATA_DIR/stdout_pipe")
    exec 2> >(tee "$WTF_DATA_DIR/stderr_pipe")
}
```

## Best Practices

### 1. Shell Integration Design

**Minimize Performance Impact:**
```bash
# Fast JSON generation without external tools
wtf_capture_command() {
    # Use printf instead of cat for better performance
    printf '{"command":"%s","exit_code":%d,"timestamp":"%s"}\n' \
        "$command" "$exit_code" "$(date -Iseconds)" \
        > "$WTF_DATA_DIR/last_command.json"
}
```

**Error Handling:**
```bash
# Prevent integration from breaking shell
wtf_capture_command() {
    {
        # All capture logic here
    } 2>/dev/null || true  # Suppress errors
}
```

**Recursion Prevention:**
```bash
# Skip wtf commands and shell builtins
[[ "$command" =~ ^(wtf|cd|exit|source|\.|history).*$ ]] && return
```

### 2. Go Implementation Best Practices

**Graceful Degradation:**
```go
func GetLastCommand() (CommandInfo, error) {
    methods := []func() (CommandInfo, error){
        getFromEnvironment,
        getFromShellIntegration,
        getFromHistoryFile,
        getFromFCCommand,
    }
    
    for _, method := range methods {
        if cmd, err := method(); err == nil && cmd.Command != "" {
            return cmd, nil
        }
    }
    
    return CommandInfo{Command: "[Unable to retrieve command]"}, nil
}
```

**Configuration Management:**
```go
type Config struct {
    ShellIntegration bool   `json:"shell_integration"`
    FallbackMethods  []string `json:"fallback_methods"`
    OutputCapture    bool   `json:"output_capture"`
}
```

### 3. Cross-Shell Compatibility

**Zsh Integration:**
```zsh
# ~/.wtf/integration.zsh
preexec() {
    WTF_COMMAND_START=$(date +%s.%N)
    WTF_LAST_COMMAND="$1"
}

precmd() {
    local exit_code=$?
    wtf_capture_command "$WTF_LAST_COMMAND" "$WTF_COMMAND_START"
}
```

**Fish Integration:**
```fish
# ~/.wtf/integration.fish
function wtf_preexec --on-event fish_preexec
    set -g WTF_COMMAND_START (date +%s.%N)
    set -g WTF_LAST_COMMAND $argv
end

function wtf_postexec --on-event fish_postexec
    wtf_capture_command $WTF_LAST_COMMAND $WTF_COMMAND_START
end
```

### 4. Security Considerations

**Data Sanitization:**
```bash
# Escape special characters in JSON
wtf_escape_json() {
    local input="$1"
    # Escape quotes, backslashes, newlines
    printf '%s' "$input" | sed 's/\\/\\\\/g; s/"/\\"/g; s/$/\\n/g' | tr -d '\n'
}
```

**File Permissions:**
```bash
# Secure file creation
umask 077  # Only user can read/write
touch "$WTF_DATA_DIR/last_command.json"
chmod 600 "$WTF_DATA_DIR/last_command.json"
```

**Command Filtering:**
```bash
# Don't capture sensitive commands
case "$command" in
    *password*|*secret*|*token*|sudo*)
        return  # Skip sensitive commands
        ;;
esac
```

## Industry Examples

### Tools Using Shell Integration

1. **Atuin** - Advanced shell history
   - Uses similar hook mechanisms
   - Stores in SQLite database
   - Provides search and sync features

2. **McFly** - Neural network shell history
   - Bash/Zsh integration via hooks
   - Machine learning for command suggestions
   - Real-time command scoring

3. **Starship** - Cross-shell prompt
   - Uses shell-specific hooks
   - Real-time git status, directory info
   - Minimal performance impact

4. **Zoxide** - Smart directory jumping
   - Tracks directory usage via hooks
   - Provides intelligent `cd` replacement
   - Cross-shell compatibility

### Lessons Learned

1. **Shell Integration is Standard**: All major shell enhancement tools use hook-based integration
2. **Performance Matters**: Minimize hook execution time
3. **Graceful Degradation**: Always provide fallback methods
4. **User Experience**: Make installation as simple as possible
5. **Cross-Shell Support**: Different shells require different approaches

## Conclusion

**Shell Integration** is the recommended approach for CLI command capture because it provides:

- **Accuracy**: Real exit codes and command text
- **Timeliness**: Immediate data availability
- **Completeness**: Full execution context
- **Reliability**: Same-process capture

While it requires initial setup, the benefits far outweigh the complexity. The fallback methods ensure the tool works even without integration, but with reduced accuracy.

The architecture should prioritize shell integration while maintaining robust fallback mechanisms for maximum compatibility and user experience.
