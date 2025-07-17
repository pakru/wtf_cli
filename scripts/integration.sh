#!/bin/bash
# WTF CLI Shell Integration
# Add this to your ~/.bashrc or ~/.bash_profile

# Directory for storing command info
WTF_DATA_DIR="$HOME/.wtf"
mkdir -p "$WTF_DATA_DIR"

# Function to capture command info
wtf_capture_command() {
    local command="$1"
    local start_time="$2"
    local exit_code="${3:-$?}"
    local end_time=$(date +%s.%N)
    
    # Skip if command is empty or starts with wtf
    [[ -z "$command" || "$command" =~ ^wtf.* ]] && return
    
    # Calculate duration (fallback if bc is not available)
    local duration
    if command -v bc >/dev/null 2>&1; then
        duration=$(echo "$end_time - $start_time" | bc -l)
        # Ensure duration has leading zero for JSON validity
        [[ "$duration" =~ ^\. ]] && duration="0$duration"
    else
        duration="0.0"
    fi
    
    # Create command info file (ensure directory exists)
    mkdir -p "$WTF_DATA_DIR" 2>/dev/null || true
    cat > "$WTF_DATA_DIR/last_command.json" << EOF
{
    "command": "$command",
    "exit_code": $exit_code,
    "start_time": "$start_time",
    "end_time": "$end_time",
    "duration": $duration,
    "pwd": "$PWD",
    "timestamp": "$(date -Iseconds)"
}
EOF
    
    # Capture output if available (from previous command)
    if [[ -f "$WTF_DATA_DIR/command_output.tmp" ]]; then
        mv "$WTF_DATA_DIR/command_output.tmp" "$WTF_DATA_DIR/last_output.txt"
    fi
}

# Function to capture command before execution
wtf_pre_command() {
    WTF_COMMAND_START=$(date +%s.%N)
    WTF_LAST_COMMAND="$BASH_COMMAND"
}

# Function to capture command after execution
wtf_post_command() {
    local exit_code=$?
    wtf_capture_command "$WTF_LAST_COMMAND" "$WTF_COMMAND_START" "$exit_code"
}

# Set up traps to capture commands
trap 'wtf_pre_command' DEBUG
trap 'wtf_post_command' ERR EXIT

# Alternative: Use PROMPT_COMMAND for more reliable capture
wtf_prompt_command() {
    local exit_code=$?
    if [[ -n "$WTF_LAST_COMMAND" ]]; then
        wtf_capture_command "$WTF_LAST_COMMAND" "$WTF_COMMAND_START" "$exit_code"
    fi
}

# Add to PROMPT_COMMAND
if [[ "$PROMPT_COMMAND" ]]; then
    PROMPT_COMMAND="wtf_prompt_command; $PROMPT_COMMAND"
else
    PROMPT_COMMAND="wtf_prompt_command"
fi

# Function to enable output capture for next command
wtf_capture_output() {
    echo "Output capture enabled for next command"
    exec 3>&1 4>&2
    exec 1> >(tee "$WTF_DATA_DIR/command_output.tmp")
    exec 2> >(tee "$WTF_DATA_DIR/command_output.tmp" >&2)
}

# Function to disable output capture
wtf_stop_capture() {
    exec 1>&3 2>&4
    exec 3>&- 4>&-
}
