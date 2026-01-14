package commands

import (
	"wtf_cli/pkg/buffer"
	"wtf_cli/pkg/capture"
)

// Context contains all the context needed for command execution
type Context struct {
	Buffer       *buffer.CircularBuffer
	Session      *capture.SessionContext
	CurrentDir   string
	LastExitCode int
}

// NewContext creates a new command context
func NewContext(buf *buffer.CircularBuffer, sess *capture.SessionContext, cwd string) *Context {
	return &Context{
		Buffer:       buf,
		Session:      sess,
		CurrentDir:   cwd,
		LastExitCode: 0,
	}
}

// GetLastNLines returns the last N lines from the buffer
func (c *Context) GetLastNLines(n int) [][]byte {
	if c.Buffer == nil {
		return nil
	}
	return c.Buffer.GetLastN(n)
}
