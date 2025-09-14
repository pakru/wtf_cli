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
    
    # Skip if command is empty or is a wtf internal function/command
    [[ -z "$command" || "$command" =~ ^wtf_ || "$command" =~ ^wtf$ || "$command" =~ ^\.\/wtf || "$command" =~ ^.*\/wtf$ ]] && return
    
    # Calculate duration (fallback if bc is not available)
    local duration
    if command -v bc >/dev/null 2>&1; then
        duration=$(echo "$end_time - $start_time" | bc -l)
        # Ensure duration has leading zero for JSON validity
        [[ "$duration" =~ ^\. ]] && duration="0$duration"
    else
        duration="0.0"
    fi
    
    # Create command info file
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
    
    # Create output file with limitation message
    echo "[Command output not captured - shell integration limitations]" > "$WTF_DATA_DIR/last_output.txt"
}

# Function to capture command before execution
wtf_pre_command() {
    # Only capture if this is a user command (not internal shell functions)
    if [[ "$BASH_COMMAND" != wtf_* && "$BASH_COMMAND" != *wtf_* ]]; then
        WTF_COMMAND_START=$(date +%s.%N)
        WTF_LAST_COMMAND="$BASH_COMMAND"
    fi
}

# Use PROMPT_COMMAND for reliable capture
wtf_prompt_command() {
    local exit_code=$?
    # Only process if we have a captured command and it's not a wtf internal command
    if [[ -n "$WTF_LAST_COMMAND" && "$WTF_LAST_COMMAND" != wtf_* && "$WTF_LAST_COMMAND" != *wtf_* ]]; then
        wtf_capture_command "$WTF_LAST_COMMAND" "$WTF_COMMAND_START" "$exit_code"
        # Clear the variables to prevent duplicate captures
        WTF_LAST_COMMAND=""
        WTF_COMMAND_START=""
    fi
}

# Set up command capture
trap 'wtf_pre_command' DEBUG

# Add to PROMPT_COMMAND
if [[ "$PROMPT_COMMAND" ]]; then
    PROMPT_COMMAND="wtf_prompt_command; $PROMPT_COMMAND"
else
    PROMPT_COMMAND="wtf_prompt_command"
fi
