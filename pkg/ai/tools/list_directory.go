package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"wtf_cli/pkg/ai"
)

const (
	listDirectoryName        = "list_directory"
	listDirectoryDescription = "List the contents of a directory inside the user's current working directory, like `ls -l`. " +
		"Returns one entry per line: permissions, owner:group, size in bytes (`-` for directories), modification time, and name. " +
		"Directories end with `/`; symlinks show their target and are not followed. " +
		"Use this to discover files before reading them with read_file. The path must be inside the working directory."

	listDirectoryMinMaxEntries    = 1
	listDirectoryDefaultScanCap   = 10000
	listDirectoryDefaultReadBatch = 512

	// listDirectoryFooterReserve reserves space for up to three footer lines
	// with realistic (if generous) entry counts and error text. It must stay
	// well under listDirectoryMinMaxBytes, or the reserve alone would consume
	// the entire minimum budget and no row could ever render at the floor.
	listDirectoryFooterReserve = 256
	listDirectoryMinMaxBytes   = 512

	listDirectoryTimeLayout = "2006-01-02 15:04"
)

var listDirectorySchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "Directory to list. Relative paths are resolved against the current working directory; absolute paths must lie inside it. Defaults to \".\" (the working directory)."
    },
    "include_hidden": {
      "type": "boolean",
      "description": "Include entries whose name starts with a dot. Defaults to false."
    }
  }
}`)

// ListDirectoryArgs is the JSON shape the model must produce.
type ListDirectoryArgs struct {
	Path          string `json:"path"`
	IncludeHidden bool   `json:"include_hidden,omitempty"`
}

// ListDirectory is the list_directory tool.
//
// Cwd follows the same convention as ReadFile.Cwd: the shell's cwd
// snapshotted at agent-loop start. Containment is enforced with os.Root
// (descriptor-relative traversal), which — unlike read_file's
// resolveContainedPath — is immune to a check-then-use symlink race.
//
// scanCap and readBatch bound enumeration of pathologically large
// directories. They default to package constants in NewListDirectory but are
// unexported so same-package tests can lower them.
type ListDirectory struct {
	Cwd        string
	MaxEntries int
	MaxBytes   int

	scanCap   int
	readBatch int
}

// NewListDirectory builds a list_directory tool with caps, normalizing
// zero/negative values to the package minimums.
func NewListDirectory(cwd string, maxEntries, maxBytes int) *ListDirectory {
	if maxEntries < listDirectoryMinMaxEntries {
		maxEntries = listDirectoryMinMaxEntries
	}
	if maxBytes < listDirectoryMinMaxBytes {
		maxBytes = listDirectoryMinMaxBytes
	}
	return &ListDirectory{
		Cwd:        cwd,
		MaxEntries: maxEntries,
		MaxBytes:   maxBytes,
		scanCap:    listDirectoryDefaultScanCap,
		readBatch:  listDirectoryDefaultReadBatch,
	}
}

func (t *ListDirectory) Name() string { return listDirectoryName }

func (t *ListDirectory) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        listDirectoryName,
		Description: listDirectoryDescription,
		JSONSchema:  listDirectorySchema,
	}
}

// Execute decodes args, enforces path safety via os.Root, and returns an
// ls -l-style listing of one directory level.
//
// All recoverable failures (decode error, missing directory, path rejected,
// etc.) return Result{IsError: true} so the model sees a useful message and
// can retry. Only context cancellation propagates as a Go error.
func (t *ListDirectory) Execute(ctx context.Context, raw json.RawMessage) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	var args ListDirectoryArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return t.errResult("invalid arguments: %v", err), nil
		}
	}

	// Only the literal empty string (an omitted arg) defaults to ".". A
	// TrimSpace here would silently rewrite a real, if unusual, path like
	// " spaced " to "spaced" and look up the wrong directory.
	displayPath := args.Path
	if displayPath == "" {
		displayPath = "."
	}

	if strings.TrimSpace(t.Cwd) == "" {
		return t.errResult("list_directory is not configured: working directory is unknown"), nil
	}

	cwdAbs, err := filepath.Abs(t.Cwd)
	if err != nil {
		return t.errResult("resolve working directory: %v", err), nil
	}

	rel, err := normalizeToRootRelative(cwdAbs, displayPath)
	if err != nil {
		return t.errResult("path rejected: %v", err), nil
	}

	root, err := os.OpenRoot(cwdAbs)
	if err != nil {
		return t.errResult("open working directory: %v", err), nil
	}
	defer root.Close()

	// OpenRoot (rather than Open+Stat) anchors a stable handle to the exact
	// directory instance being listed. Every subsequent operation —
	// enumeration and per-entry Lstat/Readlink — resolves against this same
	// handle instead of re-walking the path string from the outer root. Doing
	// the metadata lookups by path from root instead of from this handle
	// would reintroduce the TOCTOU gap os.Root is meant to close (e.g. if rel
	// is renamed or repointed between enumeration and metadata lookup).
	// OpenRoot also doubles as the is-this-a-directory check.
	dirRoot, err := root.OpenRoot(rel)
	if err != nil {
		return t.errResult("%s", classifyDirOpenError(displayPath, err)), nil
	}
	defer dirRoot.Close()

	f, err := dirRoot.Open(".")
	if err != nil {
		return t.errResult("open directory: %s: %s", boundedDisplay(displayPath), boundedDisplay(err.Error())), nil
	}
	defer f.Close()

	names, scanned, scanCapped, collectErr := collectEntries(ctx, f, t.scanCap, t.readBatch, args.IncludeHidden)
	if collectErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Result{}, ctxErr
		}
		if len(names) == 0 {
			return t.errResult("read directory: %v", collectErr), nil
		}
	}
	sort.Strings(names)

	content, err := renderListing(ctx, dirRoot, displayPath, names, scanned, scanCapped, collectErr, t.MaxEntries, t.MaxBytes)
	if err != nil {
		return Result{}, err
	}
	return Result{Content: content}, nil
}

// errResult builds a recoverable error Result the same way the package-level
// errResult does, but additionally enforces this tool's own MaxBytes
// invariant. Unlike the success path (which routes through renderListing's
// budget), error messages can embed unbounded user-supplied or OS-provided
// text — a long path, a deep symlink-loop message — so every exit out of
// Execute must be capped here, not just the success path.
func (t *ListDirectory) errResult(format string, args ...any) Result {
	res := errResult(format, args...)
	res.Content = clipToBytes(res.Content, t.MaxBytes)
	return res
}

// normalizeToRootRelative converts a model-supplied path into a path relative
// to cwdAbs, rejecting anything that lexically escapes it. This only improves
// error messages; os.Root (the actual enforcement boundary) rejects escapes
// regardless. Absolute paths are compared with filepath.Rel, never a string
// prefix — a prefix check would wrongly accept siblings like
// "/tmp/project-other" under cwd "/tmp/project".
func normalizeToRootRelative(cwdAbs, path string) (string, error) {
	if filepath.IsAbs(path) {
		rel, err := filepath.Rel(cwdAbs, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", errors.New("path outside working directory")
		}
		return rel, nil
	}
	rel := filepath.Clean(path)
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("path outside working directory")
	}
	return rel, nil
}

// classifyDirOpenError maps a root.OpenRoot error to a model-facing message.
// os.Root does not export its internal "path escapes root" sentinel, so
// unrecognized errors get an honest generic message that includes the
// underlying error text rather than a guessed classification — reporting an
// unrelated ENOTDIR or EMFILE as a containment violation would actively
// mislead the model about why the call failed.
//
// Classification inspects only the innermost wrapped error, never the fully
// formatted err.Error() string: that outer text interpolates the (attacker-
// or model-controlled) path itself, so a directory literally named e.g.
// "not a directory-x" could otherwise spoof a substring match against an
// unrelated failure (e.g. ENAMETOOLONG) into the wrong classification.
func classifyDirOpenError(displayPath string, err error) string {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return fmt.Sprintf("directory not found: %s", boundedDisplay(displayPath))
	case errors.Is(err, os.ErrPermission):
		return fmt.Sprintf("permission denied: %s", boundedDisplay(displayPath))
	case errors.Is(err, syscall.ENOTDIR) || isNotADirectorySentinel(err):
		// errors.Is(err, syscall.ENOTDIR) covers an intermediate path
		// component being a file (e.g. "file.txt/sub"); isNotADirectorySentinel
		// covers the final component itself being a file (os.Root's own,
		// non-errno "not a directory" sentinel).
		return fmt.Sprintf("path is not a directory: %s (use read_file for files)", boundedDisplay(displayPath))
	default:
		return fmt.Sprintf("cannot open directory: %s: %s", boundedDisplay(displayPath), boundedDisplay(err.Error()))
	}
}

// isNotADirectorySentinel reports whether err wraps os.Root's own "not a
// directory" error (returned by newRoot when a path resolves to a non-
// directory). That error is a fresh errors.New("not a directory") on every
// occurrence — not an exported sentinel — so it can't be matched with
// errors.Is. Comparing only pathErr.Err.Error() (never the full formatted
// message, which interpolates the path) keeps this safe against a path
// whose name happens to contain the same phrase.
func isNotADirectorySentinel(err error) bool {
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr.Err == nil {
		return false
	}
	return pathErr.Err.Error() == "not a directory"
}

// listDirectoryMaxDisplayBytes bounds how much of an escaped path or error
// string is ever echoed into a header or error message. Without this, a
// pathologically long or heavily-escaped path could by itself consume the
// entire MaxBytes budget, leaving the final safety clip to slice through the
// middle of it — a valid-but-garbled fragment with no footer or explanation.
const listDirectoryMaxDisplayBytes = 200

// boundedDisplay escapes s for safe display (see escapeName) and caps the
// result so no single dynamic field can dominate the byte budget.
func boundedDisplay(s string) string {
	esc := escapeName(s)
	if len(esc) <= listDirectoryMaxDisplayBytes {
		return esc
	}
	return clipToBytes(esc, listDirectoryMaxDisplayBytes) + "…[truncated]"
}

// dirReader is the subset of *os.File used by collectEntries. Extracted so
// tests can inject a fake that returns partial batches plus errors
// deterministically, without racing the real filesystem.
type dirReader interface {
	ReadDir(n int) ([]os.DirEntry, error)
}

// collectEntries batches ReadDir calls, filtering hidden entries unless
// includeHidden, and stops at scanCap raw entries so a pathologically large
// directory cannot consume unbounded time or memory. ctx is checked before
// each batch and before the final boundary probe.
//
// scanCapped is true only when there really are more entries beyond scanCap
// (verified with a one-entry probe read) — a directory with exactly scanCap
// entries is not misreported as capped.
//
// A non-EOF read error stops enumeration but preserves whatever was already
// collected; the caller decides whether a partial listing is still useful.
func collectEntries(ctx context.Context, dr dirReader, scanCap, readBatch int, includeHidden bool) (names []string, scanned int, scanCapped bool, err error) {
	if readBatch < 1 {
		readBatch = 1
	}

	for scanned < scanCap {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return names, scanned, false, ctxErr
		}

		batch := readBatch
		if remaining := scanCap - scanned; batch > remaining {
			batch = remaining
		}

		entries, readErr := dr.ReadDir(batch)
		for _, e := range entries {
			scanned++
			if name := e.Name(); includeHidden || !strings.HasPrefix(name, ".") {
				names = append(names, name)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return names, scanned, false, nil
			}
			return names, scanned, false, readErr
		}
		if len(entries) == 0 {
			return names, scanned, false, nil
		}
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return names, scanned, false, ctxErr
	}
	// Probe read: distinguishes "exactly scanCap entries" from "more exist"
	// so a directory that happens to have precisely scanCap entries isn't
	// misreported as capped. A real (non-EOF) error alongside a found probe
	// entry is still propagated — scanCapped and err are independent facts,
	// not mutually exclusive outcomes.
	probe, probeErr := dr.ReadDir(1)
	scanCapped = len(probe) > 0
	if probeErr != nil && !errors.Is(probeErr, io.EOF) {
		return names, scanned, scanCapped, probeErr
	}
	return names, scanned, scanCapped, nil
}

// renderListing assembles the header, per-entry rows, and any truncation
// footers into the final tool result, enforcing len(result) <= maxBytes
// unconditionally via a final safety clip (the byte budget reserved for
// footers during row selection makes that clip a no-op in practice).
func renderListing(
	ctx context.Context,
	dirRoot *os.Root,
	displayPath string,
	names []string,
	scanned int,
	scanCapped bool,
	collectErr error,
	maxEntries, maxBytes int,
) (string, error) {
	total := len(names)
	// A truly empty directory only gets the terse "(empty)" body when we
	// actually scanned the whole thing. If the scan cap was hit (e.g. every
	// entry within it was a dotfile) or a read error cut enumeration short,
	// claiming "(empty)" would be false — there may be more, unseen entries —
	// so fall through to the normal header+footer path instead.
	if total == 0 && !scanCapped && collectErr == nil {
		return clipToBytes(boundedDisplay(displayPath)+"\n(empty)", maxBytes), nil
	}

	header := buildHeader(displayPath, total)

	budget := maxBytes - len(header) - listDirectoryFooterReserve
	if budget < 0 {
		budget = 0
	}

	entryCap := total
	if entryCap > maxEntries {
		entryCap = maxEntries
	}

	ownerCache := make(map[[2]uint32]string)
	var rows []string
	usedBytes := 0
	rendered := 0
	for i := 0; i < entryCap; i++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		name := names[i]
		info, statErr := dirRoot.Lstat(name)

		var owner, linkTarget string
		var linkErr error
		if statErr == nil {
			owner = ownerGroup(info, ownerCache)
			if info.Mode()&os.ModeSymlink != 0 {
				linkTarget, linkErr = dirRoot.Readlink(name)
			}
		}

		row := renderEntry(name, info, statErr, owner, linkTarget, linkErr) + "\n"
		if usedBytes+len(row) > budget {
			break
		}
		rows = append(rows, row)
		usedBytes += len(row)
		rendered++
	}

	entryTruncated := rendered < total

	var sb strings.Builder
	sb.WriteString(header)
	for _, r := range rows {
		sb.WriteString(r)
	}
	sb.WriteString(buildFooter(rendered, total, scanned, entryTruncated, scanCapped, collectErr))

	result := sb.String()
	if len(result) > maxBytes {
		result = clipToBytes(result, maxBytes)
	}
	return result, nil
}

func buildHeader(displayPath string, total int) string {
	unit := "entries"
	if total == 1 {
		unit = "entry"
	}
	return fmt.Sprintf("%s (%d %s)\n", boundedDisplay(displayPath), total, unit)
}

func buildFooter(rendered, total, scanned int, entryTruncated, scanCapped bool, collectErr error) string {
	var lines []string
	if entryTruncated {
		lines = append(lines, fmt.Sprintf("[truncated: showing %d of %d entries]", rendered, total))
	}
	if scanCapped {
		lines = append(lines, fmt.Sprintf("[directory has more than %d entries; listing is based on the first %d scanned]", scanned, scanned))
	}
	if collectErr != nil {
		lines = append(lines, fmt.Sprintf("[read error after %d entries: %s]", scanned, escapeName(collectErr.Error())))
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n" + strings.Join(lines, "\n")
}

// renderEntry formats one directory-entry row from pre-resolved metadata. It
// performs no filesystem access itself so the fallback branches (stat and
// readlink failures) are unit-testable with injected values instead of
// racing the real filesystem.
func renderEntry(name string, info os.FileInfo, statErr error, owner, linkTarget string, linkErr error) string {
	if statErr != nil {
		return fmt.Sprintf("??????????  %-13s  %9s  %16s  %s", "?:?", "?", "?", escapeName(name))
	}

	mode := formatMode(info.Mode())
	size := "-"
	if info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		size = strconv.FormatInt(info.Size(), 10)
	}
	modTime := info.ModTime().Format(listDirectoryTimeLayout)

	display := escapeName(name)
	switch {
	case info.IsDir():
		display += "/"
	case info.Mode()&os.ModeSymlink != 0 && linkErr == nil:
		display += " -> " + escapeName(linkTarget)
	}

	return fmt.Sprintf("%s  %-13s  %9s  %s  %s", mode, owner, size, modTime, display)
}

// formatMode renders the conventional 10-character Unix permission string
// (e.g. "drwxr-xr-x", "-rwsr-xr-x"). fs.FileMode.String() is NOT usable here:
// it renders symlinks as "L…" and setuid/setgid/sticky as prefix letters
// instead of the s/S/t/T execute-column substitution ls uses.
func formatMode(mode os.FileMode) string {
	var b [10]byte
	b[0] = typeChar(mode)
	perm := mode.Perm()
	b[1] = permChar(perm&0o400 != 0, 'r')
	b[2] = permChar(perm&0o200 != 0, 'w')
	b[3] = execChar(perm&0o100 != 0, mode&os.ModeSetuid != 0, 's', 'S')
	b[4] = permChar(perm&0o040 != 0, 'r')
	b[5] = permChar(perm&0o020 != 0, 'w')
	b[6] = execChar(perm&0o010 != 0, mode&os.ModeSetgid != 0, 's', 'S')
	b[7] = permChar(perm&0o004 != 0, 'r')
	b[8] = permChar(perm&0o002 != 0, 'w')
	b[9] = execChar(perm&0o001 != 0, mode&os.ModeSticky != 0, 't', 'T')
	return string(b[:])
}

func typeChar(mode os.FileMode) byte {
	switch {
	case mode&os.ModeSymlink != 0:
		return 'l'
	case mode.IsDir():
		return 'd'
	case mode&os.ModeNamedPipe != 0:
		return 'p'
	case mode&os.ModeSocket != 0:
		return 's'
	case mode&os.ModeDevice != 0:
		if mode&os.ModeCharDevice != 0 {
			return 'c'
		}
		return 'b'
	default:
		return '-'
	}
}

func permChar(set bool, ch byte) byte {
	if set {
		return ch
	}
	return '-'
}

// execChar picks the execute-column character, substituting the special-bit
// letter (setuid/setgid/sticky) per ls convention: lowercase when the
// execute bit is also set, uppercase when it isn't.
func execChar(exec, special bool, onChar, offChar byte) byte {
	switch {
	case exec && special:
		return onChar
	case !exec && special:
		return offChar
	case exec:
		return 'x'
	default:
		return '-'
	}
}

// ownerGroup resolves a file's uid/gid to "user:group", falling back to
// numeric IDs when NSS lookup fails and to "?:?" when the platform doesn't
// expose *syscall.Stat_t. cache avoids repeated NSS lookups within one
// listing — directories commonly share an owner across many entries.
func ownerGroup(info os.FileInfo, cache map[[2]uint32]string) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "?:?"
	}

	key := [2]uint32{stat.Uid, stat.Gid}
	if v, ok := cache[key]; ok {
		return v
	}

	uidStr := strconv.FormatUint(uint64(stat.Uid), 10)
	name := uidStr
	if u, err := user.LookupId(uidStr); err == nil && u.Username != "" {
		name = u.Username
	}

	gidStr := strconv.FormatUint(uint64(stat.Gid), 10)
	group := gidStr
	if g, err := user.LookupGroupId(gidStr); err == nil && g.Name != "" {
		group = g.Name
	}

	v := name + ":" + group
	cache[key] = v
	return v
}

// escapeName Go-quotes names or symlink targets that contain control
// characters or invalid UTF-8, so a hostile filename cannot break the
// one-entry-per-line contract or inject terminal control sequences into
// model context and logs. Ordinary names pass through unchanged.
func escapeName(name string) string {
	if isSafeDisplayString(name) {
		return name
	}
	return strconv.Quote(name)
}

func isSafeDisplayString(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// clipToBytes truncates s to at most max bytes on a valid UTF-8 boundary.
func clipToBytes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	b := s[:max]
	for len(b) > 0 && !utf8.ValidString(b) {
		b = b[:len(b)-1]
	}
	return b
}
