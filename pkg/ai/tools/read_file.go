package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"wtf_cli/pkg/ai"
)

const (
	readFileName        = "read_file"
	readFileDescription = "Read a slice of a UTF-8 text file inside the user's current working directory. " +
		"Use this when the terminal output references a file whose contents you need to inspect to answer the user. " +
		"Returns the requested line range; large files are clipped at the configured maximum. " +
		"The path must be inside the working directory; symlinks pointing outside are rejected."

	readFileDefaultLineSpan = 200
	readFileMinMaxLines     = 1
	readFileMinMaxBytes     = 256
)

var readFileSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "Path to read. Relative paths are resolved against the current working directory; absolute paths must lie inside it."
    },
    "start_line": {
      "type": "integer",
      "minimum": 1,
      "description": "1-indexed first line to return. Defaults to 1."
    },
    "end_line": {
      "type": "integer",
      "minimum": 1,
      "description": "1-indexed last line to return (inclusive). Defaults to start_line + 199."
    }
  },
  "required": ["path"]
}`)

// ReadFileArgs is the JSON shape the model must produce.
type ReadFileArgs struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

// ReadFile is the read_file tool.
//
// Cwd should be the *shell's* current working directory at agent-loop start
// (snapshotted), not the wtf_cli process's cwd. The two can drift if the user
// `cd`s in the shell.
//
// MaxLines and MaxBytes hard-cap the returned slice; the model sees a footer
// noting any truncation and can request a different range.
type ReadFile struct {
	Cwd      string
	MaxLines int
	MaxBytes int
}

// NewReadFile builds a read_file tool with caps, normalizing zero/negative
// values to the package minimums.
func NewReadFile(cwd string, maxLines, maxBytes int) *ReadFile {
	if maxLines < readFileMinMaxLines {
		maxLines = readFileMinMaxLines
	}
	if maxBytes < readFileMinMaxBytes {
		maxBytes = readFileMinMaxBytes
	}
	return &ReadFile{Cwd: cwd, MaxLines: maxLines, MaxBytes: maxBytes}
}

func (t *ReadFile) Name() string { return readFileName }

func (t *ReadFile) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        readFileName,
		Description: readFileDescription,
		JSONSchema:  readFileSchema,
	}
}

// Execute decodes args, enforces path safety, and returns a line slice.
//
// All recoverable failures (decode error, missing file, path rejected, etc.)
// return Result{IsError: true} so the model sees a useful message and can
// retry. Only context cancellation propagates as a Go error.
func (t *ReadFile) Execute(ctx context.Context, raw json.RawMessage) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	var args ReadFileArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return errResult("invalid arguments: %v", err), nil
		}
	}

	if strings.TrimSpace(args.Path) == "" {
		return errResult("argument \"path\" is required"), nil
	}

	if strings.TrimSpace(t.Cwd) == "" {
		return errResult("read_file is not configured: working directory is unknown"), nil
	}

	resolvedPath, err := resolveContainedPath(t.Cwd, args.Path)
	if err != nil {
		return errResult("path rejected: %v", err), nil
	}

	start, end, err := normalizeRange(args.StartLine, args.EndLine)
	if err != nil {
		return errResult("invalid line range: %v", err), nil
	}

	content, totalLines, lastReturned, truncated, err := readLineRange(resolvedPath, start, end, t.MaxLines, t.MaxBytes)
	if err != nil {
		return errResult("%v", err), nil
	}

	if start > totalLines && totalLines > 0 {
		return errResult("start_line %d is past end of file (file has %d lines)", start, totalLines), nil
	}

	header := fmt.Sprintf("%s (lines %d-%d of %d)\n", args.Path, start, lastReturned, totalLines)
	body := content
	if truncated {
		body += fmt.Sprintf("\n[truncated: returned lines %d-%d of %d; request a narrower range to see more]", start, lastReturned, totalLines)
	}
	return Result{Content: header + body}, nil
}

func errResult(format string, args ...any) Result {
	return Result{Content: fmt.Sprintf(format, args...), IsError: true}
}

// resolveContainedPath turns a user-supplied path into an absolute, symlink-
// resolved path under cwd, or returns an error if the resolved path escapes
// cwd. The file does not need to exist; existence is checked when we open it.
func resolveContainedPath(cwd, userPath string) (string, error) {
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve cwd: %w", err)
	}
	cwdReal, err := filepath.EvalSymlinks(cwdAbs)
	if err != nil {
		// If cwd itself can't be resolved, fall back to the absolute form
		// rather than blocking everything. This matters when the working
		// directory was just created.
		cwdReal = cwdAbs
	}

	candidate := userPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(cwdAbs, candidate)
	}
	candidate = filepath.Clean(candidate)

	resolved, err := evalSymlinksAllowingMissing(candidate)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(cwdReal, resolved)
	if err != nil {
		return "", fmt.Errorf("path outside working directory")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path outside working directory")
	}

	return resolved, nil
}

// evalSymlinksAllowingMissing resolves symlinks but allows the final path
// component to not exist yet — useful for messages like "file not found"
// instead of "path rejected" when the user names a missing file.
func evalSymlinksAllowingMissing(path string) (string, error) {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	dir, base := filepath.Split(path)
	dir = filepath.Clean(dir)
	if dir == "" {
		dir = "."
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(realDir, base), nil
}

// normalizeRange validates and defaults the user-supplied range. It does NOT
// enforce the maxLines cap — readLineRange does that, so it can mark the
// result truncated when the cap kicks in.
func normalizeRange(start, end int) (int, int, error) {
	if start < 0 || end < 0 {
		return 0, 0, fmt.Errorf("line numbers must be non-negative")
	}
	if start == 0 {
		start = 1
	}
	if end == 0 {
		end = start + readFileDefaultLineSpan - 1
	}
	if end < start {
		return 0, 0, fmt.Errorf("end_line (%d) is less than start_line (%d)", end, start)
	}
	return start, end, nil
}

// readLineRange reads lines [start, end] (1-indexed, inclusive) from path,
// clipped at maxLines / maxBytes. It also reports the file's total line count.
//
// Returned values:
//
//	content       — joined lines, possibly truncated
//	totalLines    — total lines in the file
//	lastReturned  — last line index actually included in content
//	truncated     — true if maxLines or maxBytes clipped the result
func readLineRange(path string, start, end, maxLines, maxBytes int) (string, int, int, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", 0, 0, false, fmt.Errorf("file not found: %s", path)
		}
		if errors.Is(err, os.ErrPermission) {
			return "", 0, 0, false, fmt.Errorf("permission denied: %s", path)
		}
		return "", 0, 0, false, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err == nil && info.IsDir() {
		return "", 0, 0, false, fmt.Errorf("path is a directory, not a file: %s", path)
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var sb strings.Builder
	totalLines := 0
	included := 0
	lastReturned := 0
	bytesUsed := 0
	truncated := false

	for scanner.Scan() {
		totalLines++
		if totalLines < start {
			continue
		}
		if totalLines > end {
			// Keep counting to report totalLines, but stop appending.
			continue
		}

		line := scanner.Text()
		// If line is not valid UTF-8, replace invalid bytes so the model
		// always sees readable text.
		if !utf8.ValidString(line) {
			line = strings.ToValidUTF8(line, "�")
		}

		// Each line in output is followed by '\n' except possibly the last.
		needed := len(line) + 1
		if included > 0 && bytesUsed+needed > maxBytes {
			truncated = true
			// Keep counting totalLines below.
			end = totalLines - 1
			continue
		}
		if included >= maxLines {
			truncated = true
			end = totalLines - 1
			continue
		}

		if included > 0 {
			sb.WriteByte('\n')
			bytesUsed++
		}
		sb.WriteString(line)
		bytesUsed += len(line)
		included++
		lastReturned = totalLines
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return "", 0, 0, false, fmt.Errorf("file has lines longer than 1MB; cannot be read")
		}
		return "", 0, 0, false, fmt.Errorf("read: %w", err)
	}

	if lastReturned == 0 {
		// We returned nothing. Could be: empty file, start past EOF, or all
		// lines clipped. Caller decides how to phrase it.
		lastReturned = start - 1
		if lastReturned < 0 {
			lastReturned = 0
		}
	}

	return sb.String(), totalLines, lastReturned, truncated, nil
}
