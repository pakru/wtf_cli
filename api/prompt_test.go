package api

import (
	"strings"
	"testing"
)

func TestSystemPrompt(t *testing.T) {
	prompt := SystemPrompt

	// Check that prompt contains key elements
	expectedElements := []string{
		"troubleshooting expert",
		"actionable solutions",
		"copy-pasteable commands",
		"concise but thorough",
		"next command to run",
	}

	for _, element := range expectedElements {
		if !strings.Contains(strings.ToLower(prompt), strings.ToLower(element)) {
			t.Errorf("System prompt should contain '%s', but doesn't. Prompt: %s", element, prompt)
		}
	}

	// Check prompt is not empty
	if len(prompt) == 0 {
		t.Error("System prompt should not be empty")
	}

	// Check prompt has reasonable length
	if len(prompt) < 100 {
		t.Error("System prompt seems too short")
	}
}

func TestBuildPrompt_BasicCommand(t *testing.T) {
	cmdInfo := CommandInfo{
		Command:    "git push origin main",
		ExitCode:   "1",
		Output:     "Permission denied (publickey)",
		WorkingDir: "/home/user/project",
		Duration:   "0.5s",
	}

	sysInfo := SystemInfo{
		OS:           "linux",
		Distribution: "Ubuntu",
		Kernel:       "5.4.0-generic",
		Shell:        "/bin/bash",
		User:         "testuser",
		Home:         "/home/testuser",
	}

	prompt := BuildPrompt(cmdInfo, sysInfo)

	// Check that all command info is included
	if !strings.Contains(prompt, cmdInfo.Command) {
		t.Errorf("Prompt should contain command '%s'", cmdInfo.Command)
	}
	if !strings.Contains(prompt, cmdInfo.ExitCode) {
		t.Errorf("Prompt should contain exit code '%s'", cmdInfo.ExitCode)
	}
	if !strings.Contains(prompt, cmdInfo.Output) {
		t.Errorf("Prompt should contain output '%s'", cmdInfo.Output)
	}
	if !strings.Contains(prompt, cmdInfo.WorkingDir) {
		t.Errorf("Prompt should contain working directory '%s'", cmdInfo.WorkingDir)
	}

	// Check that system info is included
	if !strings.Contains(prompt, sysInfo.OS) {
		t.Errorf("Prompt should contain OS '%s'", sysInfo.OS)
	}
	if !strings.Contains(prompt, sysInfo.Distribution) {
		t.Errorf("Prompt should contain distribution '%s'", sysInfo.Distribution)
	}
	if !strings.Contains(prompt, sysInfo.User) {
		t.Errorf("Prompt should contain user '%s'", sysInfo.User)
	}

	// Check for analysis request
	if !strings.Contains(prompt, "analyze what went wrong") {
		t.Error("Prompt should ask for analysis")
	}
}

func TestBuildPrompt_MinimalInfo(t *testing.T) {
	cmdInfo := CommandInfo{
		Command:  "ls",
		ExitCode: "0",
	}

	sysInfo := SystemInfo{
		OS: "linux",
	}

	prompt := BuildPrompt(cmdInfo, sysInfo)

	// Should still contain basic elements
	if !strings.Contains(prompt, "ls") {
		t.Error("Prompt should contain command 'ls'")
	}
	if !strings.Contains(prompt, "0") {
		t.Error("Prompt should contain exit code '0'")
	}
	if !strings.Contains(prompt, "linux") {
		t.Error("Prompt should contain OS 'linux'")
	}
}

func TestBuildPrompt_EmptyFields(t *testing.T) {
	cmdInfo := CommandInfo{
		Command:    "test command",
		ExitCode:   "1",
		Output:     "",
		WorkingDir: "",
		Duration:   "",
	}

	sysInfo := SystemInfo{
		OS:           "linux",
		Distribution: "",
		Kernel:       "",
		Shell:        "",
		User:         "",
		Home:         "",
	}

	prompt := BuildPrompt(cmdInfo, sysInfo)

	// Should handle empty fields gracefully
	if !strings.Contains(prompt, "test command") {
		t.Error("Prompt should contain command")
	}
	if !strings.Contains(prompt, "linux") {
		t.Error("Prompt should contain OS")
	}

	// Should not contain empty field labels
	lines := strings.Split(prompt, "\n")
	for _, line := range lines {
		if strings.Contains(line, ": \n") || strings.HasSuffix(line, ": ") {
			t.Errorf("Prompt should not contain empty field: '%s'", line)
		}
	}
}

func TestBuildPrompt_SpecialCharacters(t *testing.T) {
	cmdInfo := CommandInfo{
		Command:  "echo 'Hello \"World\"' | grep -E '^[A-Z]'",
		ExitCode: "1",
		Output:   "Error: Invalid regex pattern\nLine 2: Syntax error",
	}

	sysInfo := SystemInfo{
		OS:   "linux",
		User: "user@domain.com",
		Home: "/home/user with spaces",
	}

	prompt := BuildPrompt(cmdInfo, sysInfo)

	// Should handle special characters properly
	if !strings.Contains(prompt, cmdInfo.Command) {
		t.Error("Prompt should contain command with special characters")
	}
	if !strings.Contains(prompt, "user@domain.com") {
		t.Error("Prompt should contain user with @ symbol")
	}
}

func TestCreateChatRequest(t *testing.T) {
	cmdInfo := CommandInfo{
		Command:  "git status",
		ExitCode: "0",
		Output:   "On branch main",
	}

	sysInfo := SystemInfo{
		OS:   "linux",
		User: "testuser",
	}

	request := CreateChatRequest(cmdInfo, sysInfo)

	// Check basic request structure
	if len(request.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(request.Messages))
	}

	// Check system message
	systemMsg := request.Messages[0]
	if systemMsg.Role != "system" {
		t.Errorf("Expected first message role 'system', got '%s'", systemMsg.Role)
	}
	if len(systemMsg.Content) == 0 {
		t.Error("System message content should not be empty")
	}

	// Check user message
	userMsg := request.Messages[1]
	if userMsg.Role != "user" {
		t.Errorf("Expected second message role 'user', got '%s'", userMsg.Role)
	}
	if !strings.Contains(userMsg.Content, "git status") {
		t.Error("User message should contain the command")
	}

	// Note: CreateChatRequest returns a request with default values set
	// This is different from the original expectation in the test
}

func TestBuildPrompt_Structure(t *testing.T) {
	cmdInfo := CommandInfo{
		Command:    "npm install",
		ExitCode:   "1",
		Output:     "EACCES: permission denied",
		WorkingDir: "/home/user/project",
		Duration:   "2.3s",
	}

	sysInfo := SystemInfo{
		OS:           "linux",
		Distribution: "Ubuntu",
		Kernel:       "5.4.0",
		Shell:        "/bin/bash",
		User:         "developer",
		Home:         "/home/developer",
	}

	prompt := BuildPrompt(cmdInfo, sysInfo)

	// Check prompt structure
	sections := []string{
		"COMMAND DETAILS:",
		"Exit Code:",
		"SYSTEM ENVIRONMENT:",
		"OS:",
		"Distribution:",
		"Kernel:",
		"Shell:",
		"User:",
		"analyze what went wrong",
	}

	for _, section := range sections {
		if !strings.Contains(prompt, section) {
			t.Errorf("Prompt should contain section '%s'", section)
		}
	}

	// Check that sections appear in reasonable order
	cmdDetailsPos := strings.Index(prompt, "COMMAND DETAILS:")
	systemEnvPos := strings.Index(prompt, "SYSTEM ENVIRONMENT:")
	analyzePos := strings.Index(prompt, "analyze what went wrong")

	if cmdDetailsPos == -1 || systemEnvPos == -1 || analyzePos == -1 {
		t.Error("Could not find required sections in prompt")
	}

	if cmdDetailsPos > systemEnvPos {
		t.Error("Command Details section should come before System Environment")
	}
	if systemEnvPos > analyzePos {
		t.Error("System Environment section should come before analysis request")
	}
}

func TestBuildPrompt_LongOutput(t *testing.T) {
	longOutput := strings.Repeat("Error line\n", 100)
	
	cmdInfo := CommandInfo{
		Command:  "long-running-command",
		ExitCode: "1",
		Output:   longOutput,
	}

	sysInfo := SystemInfo{
		OS: "linux",
	}

	prompt := BuildPrompt(cmdInfo, sysInfo)

	// Should include the full output (no truncation in prompt building)
	if !strings.Contains(prompt, longOutput) {
		t.Error("Prompt should contain full command output")
	}

	// Should still be well-formed
	if !strings.Contains(prompt, "long-running-command") {
		t.Error("Prompt should contain command name")
	}
}

func TestBuildPrompt_UnicodeContent(t *testing.T) {
	cmdInfo := CommandInfo{
		Command:  "echo 'Hello 世界 🌍'",
		ExitCode: "0",
		Output:   "Hello 世界 🌍",
	}

	sysInfo := SystemInfo{
		OS:   "linux",
		User: "用户",
	}

	prompt := BuildPrompt(cmdInfo, sysInfo)

	// Should handle Unicode properly
	if !strings.Contains(prompt, "世界") {
		t.Error("Prompt should contain Unicode characters")
	}
	if !strings.Contains(prompt, "🌍") {
		t.Error("Prompt should contain emoji")
	}
	if !strings.Contains(prompt, "用户") {
		t.Error("Prompt should contain Unicode username")
	}
}
