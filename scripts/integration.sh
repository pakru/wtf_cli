#!/bin/bash
# WTF CLI Shell Integration
# Add this to your ~/.bashrc or ~/.bash_profile

# Directory for storing command info
WTF_DATA_DIR="$HOME/.wtf"
mkdir -p "$WTF_DATA_DIR"

# Function to capture command before execution
wtf_pre_command() {
    # Skip wtf internal functions and direct wtf calls
    if [[ "$BASH_COMMAND" != wtf_* && "$BASH_COMMAND" != "wtf" && "$BASH_COMMAND" != "./wtf" && "$BASH_COMMAND" != */wtf ]]; then
        WTF_COMMAND_START=$(date +%s.%N)
        WTF_LAST_COMMAND="$BASH_COMMAND"
    fi
}

# Process captured commands
wtf_prompt_command() {
    local exit_code=$?
    
    # Only process if we have a captured command
    if [[ -n "$WTF_LAST_COMMAND" && "$WTF_LAST_COMMAND" != wtf_* ]]; then
        # Skip direct wtf calls
        if [[ "$WTF_LAST_COMMAND" == "wtf" || "$WTF_LAST_COMMAND" == "./wtf" || "$WTF_LAST_COMMAND" =~ ^.*/wtf$ ]]; then
            # Clear variables and return
            WTF_LAST_COMMAND=""
            WTF_COMMAND_START=""
            return
        fi
        
        # Calculate duration
        local end_time=$(date +%s.%N)
        local duration
        if command -v bc >/dev/null 2>&1; then
            duration=$(echo "$end_time - $WTF_COMMAND_START" | bc -l)
            [[ "$duration" =~ ^\. ]] && duration="0$duration"
        else
            duration="0.0"
        fi
        
        # Escape strings for JSON
        local escaped_command=$(echo "$WTF_LAST_COMMAND" | sed 's/\\/\\\\/g; s/"/\\"/g')
        local escaped_pipe_command=$(echo "$pipe_command" | sed 's/\\/\\\\/g; s/"/\\"/g')
        local escaped_pwd=$(echo "$PWD" | sed 's/\\/\\\\/g; s/"/\\"/g')
        
        # Create JSON file
        cat > "$WTF_DATA_DIR/last_command.json" << EOF
{
    "command": "$escaped_command",
    "exit_code": $exit_code,
    "start_time": "$WTF_COMMAND_START",
    "end_time": "$end_time",
    "duration": $duration,
    "pwd": "$escaped_pwd",
    "timestamp": "$(date -Iseconds)"
}
EOF
        
        # Clear variables
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
