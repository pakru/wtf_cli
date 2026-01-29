package ai

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	Role    string // "user" | "assistant" | "system"
	Content string
}
