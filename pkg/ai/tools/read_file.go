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
	"syscall"
	"unicode/utf8"

	"wtf_cli/pkg/ai"
)

const (
	readFileName = "read_file"

	readFileDescriptionInWorkdirOnly = "Read a slice of a UTF-8 text file inside the user's current working directory. " +
		"Use this when the terminal output references a file whose contents you need to inspect to answer the user. " +
		"Returns the requested line range; large files are clipped at the configured maximum. " +
		"The path must be inside the working directory; symlinks pointing outside are rejected. " +
		"`~/` (the user's home directory) is expanded before this check, so it is only usable when it resolves inside the working directory."

	readFileDescriptionWithEscapes = "Read a slice of a UTF-8 text file. " +
		"Use this when the terminal output references a file whose contents you need to inspect to answer the user. " +
		"Returns the requested line range; large files are clipped at the configured maximum. " +
		"Paths outside the user's working directory require the user's explicit permission — the harness will ask them; " +
		"call the tool normally with the path you need. `~/` refers to the user's home directory."

	readFileDefaultLineSpan = 200
	readFileMinMaxLines     = 1
	readFileMinMaxBytes     = 256
)

var readFileSchemaInWorkdirOnly = json.RawMessage(`{
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

var readFileSchemaWithEscapes = json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "Path to read. Relative paths are resolved against the current working directory. Absolute paths and \"~/\" (home directory) paths outside the working directory will prompt the user for permission."
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
//
// AllowEscapes mirrors the out_of_workdir_access config policy ("ask" ⇒
// true). When set, ClassifyCall offers out-of-workdir calls for user
// approval instead of leaving them silently unreachable. Execute enforces
// AllowEscapes independently of any grant it is handed — defense in depth,
// so a grant built under a stale/mismatched policy can never unlock access.
type ReadFile struct {
	Cwd          string
	MaxLines     int
	MaxBytes     int
	AllowEscapes bool
}

// NewReadFile builds a read_file tool with caps, normalizing zero/negative
// values to the package minimums.
func NewReadFile(cwd string, maxLines, maxBytes int, allowEscapes bool) *ReadFile {
	if maxLines < readFileMinMaxLines {
		maxLines = readFileMinMaxLines
	}
	if maxBytes < readFileMinMaxBytes {
		maxBytes = readFileMinMaxBytes
	}
	return &ReadFile{Cwd: cwd, MaxLines: maxLines, MaxBytes: maxBytes, AllowEscapes: allowEscapes}
}

func (t *ReadFile) Name() string { return readFileName }

func (t *ReadFile) Definition() ai.ToolDefinition {
	desc := readFileDescriptionInWorkdirOnly
	schema := readFileSchemaInWorkdirOnly
	if t.AllowEscapes {
		desc = readFileDescriptionWithEscapes
		schema = readFileSchemaWithEscapes
	}
	return ai.ToolDefinition{
		Name:        readFileName,
		Description: desc,
		JSONSchema:  schema,
	}
}

// ClassifyCall implements EscapeClassifier. It returns nil (no escape
// offered — ordinary in-workdir approval/containment applies) when escapes
// are disabled, the working directory is unconfigured, args can't be
// decoded, the path can't be resolved, the resolved path is inside the
// working directory, or identity capture fails.
func (t *ReadFile) ClassifyCall(raw json.RawMessage) *EscapeRequest {
	if !t.AllowEscapes || strings.TrimSpace(t.Cwd) == "" {
		return nil
	}
	var args ReadFileArgs
	if err := json.Unmarshal(raw, &args); err != nil || strings.TrimSpace(args.Path) == "" {
		return nil
	}
	resolved, inside, err := classifyPath(t.Cwd, args.Path)
	if err != nil || inside {
		return nil
	}
	id, err := captureIdentity(resolved)
	if err != nil {
		return nil
	}
	return &EscapeRequest{
		RequestedPath: args.Path,
		ResolvedPath:  resolved,
		GrantDir:      filepath.Dir(resolved),
		Target:        id,
	}
}

// Execute decodes args, enforces path safety, and returns a line slice.
//
// All recoverable failures (decode error, missing file, path rejected, etc.)
// return Result{IsError: true} so the model sees a useful message and can
// retry. Only context cancellation propagates as a Go error.
func (t *ReadFile) Execute(ctx context.Context, raw json.RawMessage, grant ExecGrant) (Result, error) {
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

	start, end, err := normalizeRange(args.StartLine, args.EndLine)
	if err != nil {
		return errResult("invalid line range: %v", err), nil
	}

	f, err := t.openTarget(args.Path, grant)
	if err != nil {
		return errResult("%v", err), nil
	}
	defer f.Close()

	content, lastReturned, hasMore, truncated, err := readLineRange(f, start, end, t.MaxLines, t.MaxBytes)
	if err != nil {
		return errResult("%v", err), nil
	}

	if lastReturned < start {
		return errResult("start_line %d is past end of file", start), nil
	}

	header := fmt.Sprintf("%s (lines %d-%d)\n", args.Path, start, lastReturned)
	body := content
	if truncated {
		body += fmt.Sprintf("\n[truncated at line %d; use start_line=%d to read more]", lastReturned, lastReturned+1)
	} else if hasMore {
		body += fmt.Sprintf("\n[file continues beyond line %d; use start_line=%d to read more]", lastReturned, lastReturned+1)
	}
	return Result{Content: header + body}, nil
}

// openTarget resolves userPath and opens it as a verified regular file,
// enforcing containment for in-workdir paths (via os.Root) and grant +
// identity verification for out-of-workdir paths. Every open in this
// function uses O_NONBLOCK, and the regular-file check happens on the
// opened handle — never a pre-open Stat — so a writer-less FIFO or a device
// node returns promptly instead of blocking, in both branches.
func (t *ReadFile) openTarget(userPath string, grant ExecGrant) (*os.File, error) {
	cwdAbs, err := filepath.Abs(t.Cwd)
	if err != nil {
		return nil, fmt.Errorf("resolve cwd: %w", err)
	}

	resolved, inside, err := classifyPath(t.Cwd, userPath)
	if err != nil {
		return nil, fmt.Errorf("path rejected: %v", err)
	}

	if inside {
		return t.openInWorkdir(cwdAbs, userPath)
	}
	return t.openEscape(resolved, grant)
}

// openInWorkdir opens userPath through os.Root anchored at cwdAbs.
// Descriptor-relative traversal (openat + O_NOFOLLOW per component)
// enforces containment race-free; symlinks are followed as long as they
// stay inside the root (a known, accepted os.Root limitation: an in-tree
// symlink with an *absolute* target is rejected even when the target is
// itself inside cwd).
func (t *ReadFile) openInWorkdir(cwdAbs, userPath string) (*os.File, error) {
	expanded, err := expandTilde(userPath)
	if err != nil {
		return nil, fmt.Errorf("path rejected: %v", err)
	}
	rel, err := normalizeToRootRelative(cwdAbs, expanded)
	if err != nil {
		return nil, fmt.Errorf("path rejected: %v", err)
	}

	root, err := os.OpenRoot(cwdAbs)
	if err != nil {
		return nil, fmt.Errorf("open working directory: %v", err)
	}
	defer root.Close()

	f, err := root.OpenFile(rel, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, classifyOpenError(userPath, err)
	}
	return checkRegularFile(f, userPath)
}

// openEscape opens resolved outside the working directory. It requires a
// non-empty grant whose ApprovedPath matches the fresh resolution, then
// opens the target itself (never following a final symlink) and verifies
// the *opened handle's* identity against grant.Target before returning it —
// path-string equality alone is not sufficient; the object actually opened
// must be the one the user approved.
func (t *ReadFile) openEscape(resolved string, grant ExecGrant) (*os.File, error) {
	if !t.AllowEscapes || grant.ApprovedPath == "" {
		return nil, fmt.Errorf("path outside working directory (not approved): %s", resolved)
	}
	if resolved != grant.ApprovedPath {
		return nil, fmt.Errorf("path changed during approval; call the tool again")
	}

	f, err := os.OpenFile(resolved, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		if !grant.Target.Valid && errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("file not found: %s", resolved)
		}
		return nil, fmt.Errorf("path changed during approval; call the tool again")
	}

	if !grant.Target.Valid {
		f.Close()
		return nil, fmt.Errorf("path changed during approval; call the tool again")
	}

	f, err = checkRegularFile(f, resolved)
	if err != nil {
		return nil, err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("path changed during approval; call the tool again")
	}
	id, ok := fileID(info)
	if !ok || !id.equal(grant.Target) {
		f.Close()
		return nil, fmt.Errorf("path changed during approval; call the tool again")
	}
	return f, nil
}

// checkRegularFile Stats an already-open handle and rejects non-regular
// files, closing f on rejection.
func checkRegularFile(f *os.File, displayPath string) (*os.File, error) {
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat: %w", err)
	}
	if info.IsDir() {
		f.Close()
		return nil, fmt.Errorf("path is a directory, not a file: %s", displayPath)
	}
	if !info.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("not a regular file: %s", displayPath)
	}
	return f, nil
}

// classifyOpenError maps an os.Root open error to a model-facing message.
// ENOTDIR (traversal through a non-directory component) is deliberately not
// reported as a containment violation — it has nothing to do with cwd
// boundaries.
func classifyOpenError(displayPath string, err error) error {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("file not found: %s", displayPath)
	case errors.Is(err, os.ErrPermission):
		return fmt.Errorf("permission denied: %s", displayPath)
	case errors.Is(err, syscall.ENOTDIR):
		return fmt.Errorf("path not found: %s", displayPath)
	default:
		return fmt.Errorf("cannot open path: %s: %v", displayPath, err)
	}
}

func errResult(format string, args ...any) Result {
	return Result{Content: fmt.Sprintf(format, args...), IsError: true}
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

// readLineRange reads lines [start, end] (1-indexed, inclusive) from an
// already-open, already-type-checked regular file f, clipped at maxLines /
// maxBytes. It stops scanning as soon as the slice is complete — it never
// reads past the last returned line to count total lines. The caller owns
// f's lifecycle (open and close); readLineRange neither opens nor closes it.
//
// Returned values:
//
//	content      — joined lines, possibly byte-truncated on the first line
//	lastReturned — last line index actually included in content
//	hasMore      — true if the file has content after lastReturned
//	truncated    — true if maxLines or maxBytes clipped the result short of end
func readLineRange(f *os.File, start, end, maxLines, maxBytes int) (string, int, bool, bool, error) {
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var sb strings.Builder
	lineNum := 0
	included := 0
	lastReturned := 0
	bytesUsed := 0
	truncated := false
	hasMore := false

	for scanner.Scan() {
		lineNum++

		if lineNum < start {
			continue
		}

		// Past the requested range — file has content beyond what was asked for.
		if lineNum > end {
			hasMore = true
			break
		}

		// Line cap reached — truncated within the requested range.
		if included >= maxLines {
			truncated = true
			hasMore = true
			break
		}

		line := scanner.Text()
		if !utf8.ValidString(line) {
			line = strings.ToValidUTF8(line, "�")
		}

		// Byte cap. Compute bytes needed: the line itself plus the newline
		// separator that precedes it (except for the very first included line).
		needed := len(line)
		if included > 0 {
			needed++ // newline separator
		}

		if bytesUsed+needed > maxBytes {
			truncated = true
			if included == 0 {
				// First line exceeds the byte cap. Include a truncated version
				// rather than returning nothing; trim to a valid UTF-8 boundary.
				avail := maxBytes
				for avail > 0 && !utf8.Valid([]byte(line[:avail])) {
					avail--
				}
				sb.WriteString(line[:avail])
				sb.WriteString("…[line truncated at byte cap]")
				included++
				lastReturned = lineNum
			}
			hasMore = true
			break
		}

		if included > 0 {
			sb.WriteByte('\n')
			bytesUsed++
		}
		sb.WriteString(line)
		bytesUsed += len(line)
		included++
		lastReturned = lineNum
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return "", 0, false, false, fmt.Errorf("file has lines longer than 1MB; cannot be read")
		}
		return "", 0, false, false, fmt.Errorf("read: %w", err)
	}

	if lastReturned == 0 {
		lastReturned = max(start-1, 0)
	}

	return sb.String(), lastReturned, hasMore, truncated, nil
}
