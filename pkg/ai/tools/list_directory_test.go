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
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func mustExecLD(t *testing.T, tool *ListDirectory, args any) Result {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	res, err := tool.Execute(context.Background(), raw, ExecGrant{})
	if err != nil {
		t.Fatalf("execute returned hard error: %v", err)
	}
	return res
}

func findLineContaining(lines []string, substr string) int {
	for i, l := range lines {
		if strings.Contains(l, substr) {
			return i
		}
	}
	return -1
}

// --- Definition / constructor -----------------------------------------

func TestListDirectory_Definition(t *testing.T) {
	tool := NewListDirectory(t.TempDir(), 100, 8192, false)
	def := tool.Definition()
	if def.Name != "list_directory" {
		t.Fatalf("name = %q, want list_directory", def.Name)
	}
	if def.Description == "" {
		t.Fatal("description is empty")
	}
	var v map[string]any
	if err := json.Unmarshal(def.JSONSchema, &v); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
}

func TestListDirectory_NewListDirectoryNormalizesCaps(t *testing.T) {
	tool := NewListDirectory("/tmp", -10, -10, false)
	if tool.MaxEntries < listDirectoryMinMaxEntries {
		t.Errorf("MaxEntries = %d, want >= %d", tool.MaxEntries, listDirectoryMinMaxEntries)
	}
	if tool.MaxBytes < listDirectoryMinMaxBytes {
		t.Errorf("MaxBytes = %d, want >= %d", tool.MaxBytes, listDirectoryMinMaxBytes)
	}
}

// --- Basic listing -------------------------------------------------------

func TestListDirectory_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "b.txt"), "hello")
	writeFile(t, filepath.Join(dir, "a.txt"), "world!") // 6 bytes
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	tool := NewListDirectory(dir, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "."})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}

	lines := strings.Split(strings.TrimRight(res.Content, "\n"), "\n")
	if !strings.HasPrefix(lines[0], ". (3 entries)") {
		t.Fatalf("unexpected header: %q", lines[0])
	}

	aLine := findLineContaining(lines, "a.txt")
	bLine := findLineContaining(lines, "b.txt")
	subLine := findLineContaining(lines, "sub/")
	if aLine < 0 || bLine < 0 || subLine < 0 {
		t.Fatalf("missing expected entries in:\n%s", res.Content)
	}
	if !(aLine < bLine && bLine < subLine) {
		t.Fatalf("expected alphabetical order a.txt < b.txt < sub/, got:\n%s", res.Content)
	}

	if !strings.HasPrefix(lines[aLine], "-rw") {
		t.Fatalf("expected regular-file mode prefix on a.txt row, got %q", lines[aLine])
	}
	if fields := strings.Fields(lines[aLine]); len(fields) < 3 || fields[2] != "6" {
		t.Fatalf("expected size field 6 on a.txt row, got %q", lines[aLine])
	}

	if !strings.HasPrefix(lines[subLine], "d") {
		t.Fatalf("expected directory mode prefix on sub row, got %q", lines[subLine])
	}
	if fields := strings.Fields(lines[subLine]); len(fields) < 3 || fields[2] != "-" {
		t.Fatalf("expected '-' size placeholder on sub row, got %q", lines[subLine])
	}
}

func TestListDirectory_DefaultPathListsCwd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "only.txt"), "x")

	tool := NewListDirectory(dir, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "only.txt") {
		t.Fatalf("expected default path to list cwd, got:\n%s", res.Content)
	}
}

// Regression: a real directory named with leading/trailing whitespace must
// not be corrupted by a TrimSpace on the model-supplied path. Only the
// literal empty string should default to ".".
func TestListDirectory_PathWithSurroundingWhitespaceIsNotTrimmed(t *testing.T) {
	dir := t.TempDir()
	weird := " spaced "
	if err := os.Mkdir(filepath.Join(dir, weird), 0o755); err != nil {
		t.Skipf("filesystem rejects whitespace-padded names: %v", err)
	}
	writeFile(t, filepath.Join(dir, weird, "inside.txt"), "x")

	tool := NewListDirectory(dir, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: weird})
	if res.IsError {
		t.Fatalf("a real whitespace-padded directory name should be usable as-is, got error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "inside.txt") {
		t.Fatalf("expected to list the whitespace-named directory's contents, got:\n%s", res.Content)
	}
}

func TestListDirectory_UnconfiguredCwd(t *testing.T) {
	tool := NewListDirectory("", 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{})
	if !res.IsError {
		t.Fatalf("expected error for empty cwd, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "working directory") {
		t.Fatalf("expected cwd message, got: %s", res.Content)
	}
}

// --- Hidden entries --------------------------------------------------------

func TestListDirectory_HiddenExcludedByDefault(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "visible.txt"), "x")
	writeFile(t, filepath.Join(dir, ".hidden"), "x")

	tool := NewListDirectory(dir, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "."})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if strings.Contains(res.Content, ".hidden") {
		t.Fatalf("hidden entry should be excluded by default:\n%s", res.Content)
	}
	if !strings.Contains(res.Content, "visible.txt") {
		t.Fatalf("expected visible entry present:\n%s", res.Content)
	}
}

func TestListDirectory_HiddenIncludedWithFlag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".hidden"), "x")

	tool := NewListDirectory(dir, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: ".", IncludeHidden: true})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if !strings.Contains(res.Content, ".hidden") {
		t.Fatalf("expected hidden entry with include_hidden=true:\n%s", res.Content)
	}
}

func TestListDirectory_OnlyDotfilesRendersEmptyWithoutFlag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".onlyhidden"), "x")

	tool := NewListDirectory(dir, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "."})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "(empty)") {
		t.Fatalf("expected (empty) marker when all entries are hidden, got:\n%s", res.Content)
	}
}

// --- Path containment ------------------------------------------------------

func TestListDirectory_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	tool := NewListDirectory(dir, 100, 65536, false)
	// A generous, depth-independent "../" count: t.TempDir() nests only a
	// couple of levels on Linux but considerably deeper on macOS (a
	// supported target), so a fixed small count is not portable.
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: strings.Repeat("../", 20) + "etc"})
	if !res.IsError {
		t.Fatalf("expected error for traversal, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "outside working directory") {
		t.Fatalf("error message should mention containment, got: %s", res.Content)
	}
}

func TestListDirectory_RejectsAbsoluteOutsideCwd(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()

	tool := NewListDirectory(cwd, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: outside})
	if !res.IsError {
		t.Fatalf("expected error for outside-cwd absolute path, got: %s", res.Content)
	}
}

// Regression: a naive string-prefix containment check would wrongly accept a
// sibling directory whose name happens to start with the cwd's name.
func TestListDirectory_RejectsAbsoluteSiblingPrefix(t *testing.T) {
	parent := t.TempDir()
	cwd := filepath.Join(parent, "project")
	sibling := filepath.Join(parent, "project-other")
	if err := os.Mkdir(cwd, 0o755); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatalf("mkdir sibling: %v", err)
	}

	tool := NewListDirectory(cwd, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: sibling})
	if !res.IsError {
		t.Fatalf("expected sibling directory with prefixed name to be rejected, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "outside working directory") {
		t.Fatalf("expected containment rejection message, got: %s", res.Content)
	}
}

// --- os.Root symlink semantics ---------------------------------------------

func TestListDirectory_AllowsRelativeSymlinkedDirInsideCwd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "real"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(cwd, "real", "inside.txt"), "x")
	if err := os.Symlink("real", filepath.Join(cwd, "alias")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	tool := NewListDirectory(cwd, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "alias"})
	if res.IsError {
		t.Fatalf("relative in-tree symlinked dir should be traversable, got error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "inside.txt") {
		t.Fatalf("expected to list target directory contents, got:\n%s", res.Content)
	}
}

// os.Root documents that symbolic links must not be absolute — even when the
// absolute target happens to point back inside the root, it is rejected.
// This is a known, accepted limitation (see design doc), not a bug.
func TestListDirectory_RejectsAbsoluteSymlinkedDirEvenPointingInsideCwd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	cwd := t.TempDir()
	realDir := filepath.Join(cwd, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(realDir, filepath.Join(cwd, "alias")); err != nil { // absolute target
		t.Fatalf("symlink: %v", err)
	}

	tool := NewListDirectory(cwd, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "alias"})
	if !res.IsError {
		t.Fatalf("expected absolute-target symlink traversal to be rejected by os.Root, got: %s", res.Content)
	}
}

func TestListDirectory_SymlinkEntryShowsTargetNotFollowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "target.txt"), "x")
	if err := os.Symlink("target.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	tool := NewListDirectory(dir, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "."})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	lines := strings.Split(res.Content, "\n")
	linkLine := findLineContaining(lines, "link ->")
	if linkLine < 0 {
		t.Fatalf("expected symlink entry row with target arrow, got:\n%s", res.Content)
	}
	if !strings.HasPrefix(lines[linkLine], "l") {
		t.Fatalf("expected symlink mode prefix, got %q", lines[linkLine])
	}
	if !strings.Contains(lines[linkLine], "target.txt") {
		t.Fatalf("expected target name in row, got %q", lines[linkLine])
	}
}

func TestListDirectory_BrokenSymlinkEntryStillRenders(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	dir := t.TempDir()
	if err := os.Symlink("does-not-exist.txt", filepath.Join(dir, "broken")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	tool := NewListDirectory(dir, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "."})
	if res.IsError {
		t.Fatalf("broken symlink should not fail the listing: %s", res.Content)
	}
	if !strings.Contains(res.Content, "broken -> does-not-exist.txt") {
		t.Fatalf("expected broken symlink row with its target text, got:\n%s", res.Content)
	}
}

// Listing a symlink entry only Lstat/Readlinks it; it is never dereferenced,
// so a target outside cwd (even absolute) is displayed as-is without error.
func TestListDirectory_SymlinkEntryWithAbsoluteTargetIsListedNotFollowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	dir := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "shh")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(dir, "escape")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	tool := NewListDirectory(dir, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "."})
	if res.IsError {
		t.Fatalf("listing a directory containing an absolute-target symlink entry should succeed: %s", res.Content)
	}
	if !strings.Contains(res.Content, "escape -> "+filepath.Join(outside, "secret.txt")) {
		t.Fatalf("expected raw absolute target displayed without following it, got:\n%s", res.Content)
	}
}

// --- Not-a-directory / missing / permission --------------------------------

func TestListDirectory_MissingDirectory(t *testing.T) {
	tool := NewListDirectory(t.TempDir(), 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "does_not_exist"})
	if !res.IsError {
		t.Fatalf("expected error for missing directory, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "not found") {
		t.Fatalf("expected 'not found' message, got: %s", res.Content)
	}
}

func TestListDirectory_RejectsFileAsPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "file.txt"), "x")

	tool := NewListDirectory(dir, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "file.txt"})
	if !res.IsError {
		t.Fatalf("expected error for file path, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "not a directory") {
		t.Fatalf("expected 'not a directory' message, got: %s", res.Content)
	}
}

// Regression: traversing through a non-directory intermediate component
// (ENOTDIR) must not be misreported as a containment violation — it has
// nothing to do with cwd boundaries.
func TestListDirectory_PathThroughFileComponentReportsNotADirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "file"), "x")

	tool := NewListDirectory(dir, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "file/child"})
	if !res.IsError {
		t.Fatalf("expected error for path traversing through a file component, got: %s", res.Content)
	}
	if strings.Contains(res.Content, "outside working directory") {
		t.Fatalf("path-through-file should not be misreported as a containment violation, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "not a directory") {
		t.Fatalf("expected 'not a directory' message, got: %s", res.Content)
	}
}

func TestListDirectory_PermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits differ on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses permission checks")
	}
	dir := t.TempDir()
	sub := filepath.Join(dir, "locked")
	if err := os.Mkdir(sub, 0o000); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) }) // allow TempDir cleanup

	tool := NewListDirectory(dir, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "locked"})
	if !res.IsError {
		t.Fatalf("expected permission error, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "permission denied") {
		t.Fatalf("expected 'permission denied' message, got: %s", res.Content)
	}
}

// --- Empty directory ---------------------------------------------------

func TestListDirectory_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	tool := NewListDirectory(dir, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "."})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "(empty)") {
		t.Fatalf("expected (empty) marker, got:\n%s", res.Content)
	}
}

// --- Truncation / byte-cap contract -----------------------------------

func TestListDirectory_MaxEntriesCap(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("f%02d.txt", i)), "x")
	}

	tool := NewListDirectory(dir, 5, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "."})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "[truncated: showing 5 of 20 entries]") {
		t.Fatalf("expected truncation footer, got:\n%s", res.Content)
	}
	if got := strings.Count(res.Content, ".txt"); got != 5 {
		t.Fatalf("expected exactly 5 rendered rows, got %d in:\n%s", got, res.Content)
	}
	if len(res.Content) > tool.MaxBytes {
		t.Fatalf("result exceeds MaxBytes: %d > %d", len(res.Content), tool.MaxBytes)
	}
}

func TestListDirectory_MaxBytesCap(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 50; i++ {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("file-%03d.txt", i)), "x")
	}

	tool := NewListDirectory(dir, 1000, 400, false) // generous entry cap, tight byte cap
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "."})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "[truncated: showing") {
		t.Fatalf("expected truncation footer, got:\n%s", res.Content)
	}
	if len(res.Content) > tool.MaxBytes {
		t.Fatalf("result exceeds MaxBytes: %d > %d\ncontent:\n%s", len(res.Content), tool.MaxBytes, res.Content)
	}
}

// Degenerate case: escaping a long control-character-laden path inflates the
// header past MaxBytes on its own. The whole-result byte cap must still hold.
func TestListDirectory_ByteCapNeverExceededWithControlCharPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("control characters are not valid in windows filenames")
	}
	dir := t.TempDir()
	weirdName := strings.Repeat("\x01", 200)
	weirdDir := filepath.Join(dir, weirdName)
	if err := os.Mkdir(weirdDir, 0o755); err != nil {
		t.Skipf("filesystem rejects control characters in filenames: %v", err)
	}
	writeFile(t, filepath.Join(weirdDir, "f.txt"), "x")

	tool := &ListDirectory{
		Cwd:        dir,
		MaxEntries: 100,
		MaxBytes:   20,
		scanCap:    listDirectoryDefaultScanCap,
		readBatch:  listDirectoryDefaultReadBatch,
	}
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: weirdName})

	if len(res.Content) > tool.MaxBytes {
		t.Fatalf("result exceeds MaxBytes even in degenerate header case: %d > %d", len(res.Content), tool.MaxBytes)
	}
	// The pathological MaxBytes=20 budget proves only the length bound; also
	// require the clip to land on a valid UTF-8 boundary rather than
	// producing garbage — a bare length check would pass even for a broken
	// clipToBytes that always returned "".
	if !utf8.ValidString(res.Content) {
		t.Fatalf("clipped content is not valid UTF-8: %q", res.Content)
	}
}

// Same shape as the pathological case above, but at the real constructor
// minimum (256 bytes) with a moderately long path — proves clipping still
// leaves genuinely useful, non-empty output when the budget isn't absurd.
func TestListDirectory_ByteCapAtRealMinimumStillProducesUsefulOutput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "f.txt"), "x")

	tool := NewListDirectory(dir, 100, listDirectoryMinMaxBytes, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "."})

	if len(res.Content) > tool.MaxBytes {
		t.Fatalf("result exceeds MaxBytes: %d > %d", len(res.Content), tool.MaxBytes)
	}
	if res.Content == "" {
		t.Fatal("expected non-empty output at the real minimum byte budget")
	}
	if !strings.Contains(res.Content, "f.txt") {
		t.Fatalf("expected the one small entry to still fit at the minimum budget, got:\n%s", res.Content)
	}
}

// Regression: recoverable errors bypassed renderListing's budget entirely —
// classifyDirOpenError can embed an unbounded escaped path or OS error
// string, so the MaxBytes invariant must hold on every Execute exit, not
// just the success path.
func TestListDirectory_ErrorResultsAlsoRespectMaxBytes(t *testing.T) {
	dir := t.TempDir()

	tool := &ListDirectory{
		Cwd:        dir,
		MaxEntries: 100,
		MaxBytes:   20,
		scanCap:    listDirectoryDefaultScanCap,
		readBatch:  listDirectoryDefaultReadBatch,
	}
	longMissingPath := strings.Repeat("missing-", 40)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: longMissingPath})

	if !res.IsError {
		t.Fatalf("expected an error for a missing directory, got: %s", res.Content)
	}
	if len(res.Content) > tool.MaxBytes {
		t.Fatalf("error result exceeds MaxBytes: %d > %d", len(res.Content), tool.MaxBytes)
	}
	if !utf8.ValidString(res.Content) {
		t.Fatalf("clipped error content is not valid UTF-8: %q", res.Content)
	}
}

// Regression: invalid JSON and an unconfigured Cwd are the earliest exits in
// Execute, before the caps in scope even matter much in practice — but they
// must still honor MaxBytes on every path, not just the ones that happen to
// run after cwdAbs is resolved.
func TestListDirectory_EarlyExitErrorsRespectMaxBytes(t *testing.T) {
	t.Run("bad json", func(t *testing.T) {
		tool := &ListDirectory{
			Cwd:        t.TempDir(),
			MaxEntries: 100,
			MaxBytes:   1,
			scanCap:    listDirectoryDefaultScanCap,
			readBatch:  listDirectoryDefaultReadBatch,
		}
		res, err := tool.Execute(context.Background(), json.RawMessage(`{not valid json at all, and this decode error text could run long`), ExecGrant{})
		if err != nil {
			t.Fatalf("expected soft error, got hard error: %v", err)
		}
		if !res.IsError {
			t.Fatal("expected an error result for invalid JSON")
		}
		if len(res.Content) > tool.MaxBytes {
			t.Fatalf("bad-JSON error result exceeds MaxBytes: %d > %d", len(res.Content), tool.MaxBytes)
		}
	})

	t.Run("unconfigured cwd", func(t *testing.T) {
		tool := &ListDirectory{
			Cwd:        "",
			MaxEntries: 100,
			MaxBytes:   1,
			scanCap:    listDirectoryDefaultScanCap,
			readBatch:  listDirectoryDefaultReadBatch,
		}
		res := mustExecLD(t, tool, ListDirectoryArgs{})
		if !res.IsError {
			t.Fatal("expected an error result for unconfigured cwd")
		}
		if len(res.Content) > tool.MaxBytes {
			t.Fatalf("unconfigured-cwd error result exceeds MaxBytes: %d > %d", len(res.Content), tool.MaxBytes)
		}
	})
}

// Regression: classifyDirOpenError used to substring-match the FULL formatted
// error message, which interpolates the (model-controlled) path. A directory
// name that itself contains the phrase "not a directory" could spoof an
// unrelated failure (here, an overlong path component -> ENAMETOOLONG) into
// the wrong classification.
func TestListDirectory_PathContainingNotADirectoryPhraseIsNotMisclassified(t *testing.T) {
	dir := t.TempDir()
	weird := "not a directory-" + strings.Repeat("x", 300) // single component > NAME_MAX

	tool := NewListDirectory(dir, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: weird})
	if !res.IsError {
		t.Fatalf("expected an error for an overlong path component, got: %s", res.Content)
	}
	if strings.Contains(res.Content, "path is not a directory") {
		t.Fatalf("a path whose own name contains 'not a directory' must not be misclassified as such: %s", res.Content)
	}
}

// Regression: a long-but-real path (not adversarially padded, just deeply
// nested) must not let its escaped header text consume the entire MaxBytes
// budget at the real constructor floor, leaving the final safety clip to
// slice through the middle of it. The header's closing "(N entries)" must
// survive intact.
func TestListDirectory_LongRealPathAtFloorStillProducesStructuredOutput(t *testing.T) {
	dir := t.TempDir()
	segments := make([]string, 40)
	for i := range segments {
		segments[i] = fmt.Sprintf("segment-%02d", i)
	}
	relPath := filepath.Join(segments...)
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatalf("mkdir all: %v", err)
	}
	writeFile(t, filepath.Join(full, "leaf.txt"), "x")
	if len(relPath) <= listDirectoryMaxDisplayBytes {
		t.Fatalf("test setup: relPath is %d bytes, want > %d to actually exercise bounding", len(relPath), listDirectoryMaxDisplayBytes)
	}

	tool := NewListDirectory(dir, 100, listDirectoryMinMaxBytes, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: relPath})

	if len(res.Content) > tool.MaxBytes {
		t.Fatalf("result exceeds MaxBytes: %d > %d", len(res.Content), tool.MaxBytes)
	}
	if !utf8.ValidString(res.Content) {
		t.Fatalf("content is not valid UTF-8: %q", res.Content)
	}
	if !strings.Contains(res.Content, "entry)") && !strings.Contains(res.Content, "entries)") {
		t.Fatalf("expected the header's entry-count suffix to survive intact, got: %q", res.Content)
	}
}

// Regression: if every entry scanned before the scan cap hit was hidden, the
// directory must not be reported as "(empty)" — that would falsely claim
// there is nothing there when in fact enumeration simply gave up early.
func TestListDirectory_AllHiddenWithinScanCapDoesNotFalselyReportEmpty(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		writeFile(t, filepath.Join(dir, fmt.Sprintf(".hidden%d", i)), "x")
	}

	tool := &ListDirectory{
		Cwd:        dir,
		MaxEntries: 100,
		MaxBytes:   65536,
		scanCap:    3, // smaller than the 5 hidden entries that exist
		readBatch:  1,
	}
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "."})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if strings.Contains(res.Content, "(empty)") {
		t.Fatalf("must not claim (empty) when the scan cap was hit while every scanned entry was hidden: %s", res.Content)
	}
	if !strings.Contains(res.Content, "more than") {
		t.Fatalf("expected a scan-cap footer explaining why nothing is shown, got: %s", res.Content)
	}
}

// --- Owner:group -------------------------------------------------------

func TestListDirectory_OwnerGroupResolvesOrFallsBackToNumeric(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "f.txt"), "x")

	tool := NewListDirectory(dir, 100, 65536, false)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: "."})
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}

	lines := strings.Split(res.Content, "\n")
	line := findLineContaining(lines, "f.txt")
	if line < 0 {
		t.Fatalf("missing f.txt row in:\n%s", res.Content)
	}
	fields := strings.Fields(lines[line])
	if len(fields) < 2 {
		t.Fatalf("unexpected row format: %q", lines[line])
	}
	owner := fields[1]
	if !strings.Contains(owner, ":") {
		t.Fatalf("expected owner:group format, got %q", owner)
	}

	current, err := user.Current()
	if err != nil {
		t.Skip("cannot resolve current user in this environment")
	}
	// Accept either the resolved username or the raw numeric uid — CI
	// containers may not have NSS entries for the running uid.
	if !strings.HasPrefix(owner, current.Username+":") && !strings.HasPrefix(owner, current.Uid+":") {
		t.Fatalf("owner = %q, want prefix %q or %q", owner, current.Username+":", current.Uid+":")
	}
}

// --- Args / cancellation ------------------------------------------------

func TestListDirectory_BadJSON(t *testing.T) {
	tool := NewListDirectory(t.TempDir(), 100, 65536, false)
	res, err := tool.Execute(context.Background(), json.RawMessage(`{not json`), ExecGrant{})
	if err != nil {
		t.Fatalf("expected soft error, got hard error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error result, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "invalid arguments") {
		t.Fatalf("expected decode error message, got: %s", res.Content)
	}
}

func TestListDirectory_ContextCanceled(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tool := NewListDirectory(dir, 100, 65536, false)
	raw, _ := json.Marshal(ListDirectoryArgs{Path: "."})
	_, err := tool.Execute(ctx, raw, ExecGrant{})
	if err == nil {
		t.Fatal("expected hard error from canceled context")
	}
}

// --- formatMode --------------------------------------------------------

func TestFormatMode(t *testing.T) {
	tests := []struct {
		name string
		mode os.FileMode
		want string
	}{
		{"regular 0644", 0o644, "-rw-r--r--"},
		{"dir 0755", os.ModeDir | 0o755, "drwxr-xr-x"},
		{"symlink 0777", os.ModeSymlink | 0o777, "lrwxrwxrwx"},
		{"setuid with owner exec", os.ModeSetuid | 0o755, "-rwsr-xr-x"},
		{"setuid without owner exec", os.ModeSetuid | 0o644, "-rwSr--r--"},
		{"setgid with group exec", os.ModeSetgid | 0o755, "-rwxr-sr-x"},
		{"setgid without group exec", os.ModeSetgid | 0o645, "-rw-r-Sr-x"},
		{"sticky with other exec", os.ModeSticky | 0o777, "-rwxrwxrwt"},
		{"sticky without other exec", os.ModeSticky | 0o766, "-rwxrw-rwT"},
		{"named pipe", os.ModeNamedPipe | 0o644, "prw-r--r--"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatMode(tt.mode); got != tt.want {
				t.Errorf("formatMode(%v) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

// --- escapeName ----------------------------------------------------------

func TestEscapeName(t *testing.T) {
	if got := escapeName("normal-name.txt"); got != "normal-name.txt" {
		t.Errorf("escapeName(plain) = %q, want unchanged", got)
	}
	if got := escapeName("a\nb"); got != strconv.Quote("a\nb") {
		t.Errorf("escapeName(newline) = %q, want quoted", got)
	}
	if got := escapeName("a\tb"); got != strconv.Quote("a\tb") {
		t.Errorf("escapeName(tab) = %q, want quoted", got)
	}
	if esc := "a\x1bb"; escapeName(esc) != strconv.Quote(esc) {
		t.Errorf("escapeName(ESC) = %q, want quoted", escapeName(esc))
	}
	if invalid := "a\xC3b"; escapeName(invalid) != strconv.Quote(invalid) {
		t.Errorf("escapeName(invalid utf8) = %q, want quoted", escapeName(invalid))
	}
}

// --- renderEntry (no filesystem access — pure formatting) ------------------

func TestRenderEntry_StatErrorFallback(t *testing.T) {
	row := renderEntry("weird", nil, errors.New("boom"), "", "", nil)
	if !strings.Contains(row, "?") {
		t.Fatalf("expected placeholder fields, got %q", row)
	}
	if !strings.Contains(row, "weird") {
		t.Fatalf("expected name preserved despite stat error, got %q", row)
	}
}

func TestRenderEntry_ReadlinkFailureFallsBackToPlainName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "target"), "x")
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}

	row := renderEntry("link", info, nil, "u:g", "", errors.New("readlink failed"))
	if strings.Contains(row, "->") {
		t.Fatalf("expected no target arrow on readlink failure, got %q", row)
	}
	if !strings.Contains(row, "link") {
		t.Fatalf("expected entry name present, got %q", row)
	}
}

func TestRenderEntry_NormalRows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "file.txt"), "hello")
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("file.txt", link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	fileInfo, err := os.Lstat(filepath.Join(dir, "file.txt"))
	if err != nil {
		t.Fatalf("lstat file: %v", err)
	}
	dirInfo, err := os.Lstat(filepath.Join(dir, "sub"))
	if err != nil {
		t.Fatalf("lstat dir: %v", err)
	}
	linkInfo, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat link: %v", err)
	}

	fileRow := renderEntry("file.txt", fileInfo, nil, "pavel:pavel", "", nil)
	if !strings.HasPrefix(fileRow, "-rw") || !strings.Contains(fileRow, "file.txt") {
		t.Fatalf("unexpected file row: %q", fileRow)
	}

	dirRow := renderEntry("sub", dirInfo, nil, "pavel:pavel", "", nil)
	if !strings.HasPrefix(dirRow, "d") || !strings.HasSuffix(dirRow, "sub/") {
		t.Fatalf("unexpected dir row: %q", dirRow)
	}

	linkRow := renderEntry("link", linkInfo, nil, "pavel:pavel", "file.txt", nil)
	if !strings.HasPrefix(linkRow, "l") || !strings.Contains(linkRow, "link -> file.txt") {
		t.Fatalf("unexpected symlink row: %q", linkRow)
	}
}

// --- collectEntries (fake dirReader — no filesystem races) -----------------

type fakeDirEntry struct{ name string }

func (e fakeDirEntry) Name() string      { return e.name }
func (e fakeDirEntry) IsDir() bool       { return false }
func (e fakeDirEntry) Type() os.FileMode { return 0 }
func (e fakeDirEntry) Info() (os.FileInfo, error) {
	return nil, errors.New("fakeDirEntry: not implemented")
}

func namesEntries(names ...string) []os.DirEntry {
	entries := make([]os.DirEntry, len(names))
	for i, n := range names {
		entries[i] = fakeDirEntry{name: n}
	}
	return entries
}

// fakeDirReader replays pre-scripted batches (each optionally paired with an
// error) regardless of the requested batch size, so tests can drive
// collectEntries through exact edge-case sequences.
type fakeDirReader struct {
	batches [][]os.DirEntry
	errs    []error
	idx     int
}

func (f *fakeDirReader) ReadDir(n int) ([]os.DirEntry, error) {
	if f.idx >= len(f.batches) {
		return nil, io.EOF
	}
	b := f.batches[f.idx]
	var err error
	if f.idx < len(f.errs) {
		err = f.errs[f.idx]
	}
	f.idx++
	return b, err
}

type dirReaderFunc func(n int) ([]os.DirEntry, error)

func (f dirReaderFunc) ReadDir(n int) ([]os.DirEntry, error) { return f(n) }

func TestCollectEntries_ScanCapStopsEnumeration(t *testing.T) {
	dr := &fakeDirReader{
		batches: [][]os.DirEntry{
			namesEntries("a", "b"),
			namesEntries("c", "d"),
			namesEntries("e"), // beyond scanCap=4; only reachable via the probe read
		},
	}
	names, scanned, scanCapped, err := collectEntries(context.Background(), dr, 4, 2, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scanned != 4 {
		t.Fatalf("scanned = %d, want 4", scanned)
	}
	if !scanCapped {
		t.Fatal("expected scanCapped = true")
	}
	if len(names) != 4 {
		t.Fatalf("names = %v, want 4 entries", names)
	}
}

// A directory with exactly scanCap entries (no more) must not be reported as
// capped — the boundary probe distinguishes "exactly full" from "more".
func TestCollectEntries_ExactBoundaryNotFalselyScanCapped(t *testing.T) {
	dr := &fakeDirReader{
		batches: [][]os.DirEntry{
			namesEntries("a", "b"),
			namesEntries("c", "d"),
			{},
		},
		errs: []error{nil, nil, io.EOF},
	}
	names, scanned, scanCapped, err := collectEntries(context.Background(), dr, 4, 2, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scanCapped {
		t.Fatal("expected scanCapped = false when directory has exactly scanCap entries")
	}
	if scanned != 4 || len(names) != 4 {
		t.Fatalf("scanned=%d names=%v, want 4 and 4 entries", scanned, names)
	}
}

// scanCapped and a read error are independent facts, not mutually exclusive
// outcomes: the boundary probe finding a further entry must not silently
// discard a real error that arrived alongside it.
func TestCollectEntries_ProbeEntryPlusErrorPreservesBoth(t *testing.T) {
	wantErr := errors.New("boom during probe")
	dr := &fakeDirReader{
		batches: [][]os.DirEntry{
			namesEntries("a", "b"),
			namesEntries("c", "d"),
			namesEntries("e"), // the probe call: finds an entry AND errors
		},
		errs: []error{nil, nil, wantErr},
	}
	names, scanned, scanCapped, err := collectEntries(context.Background(), dr, 4, 2, false)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the probe's error to be preserved, got %v", err)
	}
	if !scanCapped {
		t.Fatal("expected scanCapped = true: the probe did find a further entry")
	}
	if scanned != 4 || len(names) != 4 {
		t.Fatalf("scanned=%d names=%v, want 4 and 4 entries", scanned, names)
	}
}

func TestCollectEntries_PartialBatchPlusErrorKeepsCollected(t *testing.T) {
	wantErr := errors.New("boom")
	dr := &fakeDirReader{
		batches: [][]os.DirEntry{namesEntries("a", "b")},
		errs:    []error{wantErr},
	}
	names, scanned, scanCapped, err := collectEntries(context.Background(), dr, 100, 10, false)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if scanCapped {
		t.Fatal("scanCapped should be false on a read error")
	}
	if scanned != 2 || len(names) != 2 {
		t.Fatalf("expected the partial batch to be preserved despite the error, got scanned=%d names=%v", scanned, names)
	}
}

func TestCollectEntries_ContextCancelledBetweenBatches(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	dr := dirReaderFunc(func(n int) ([]os.DirEntry, error) {
		calls++
		if calls == 1 {
			cancel() // cancel after the first batch, before the loop re-checks ctx
			return namesEntries("a"), nil
		}
		t.Fatalf("ReadDir called again after cancellation (call #%d)", calls)
		return nil, nil
	})

	names, _, _, err := collectEntries(ctx, dr, 100, 1, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if len(names) != 1 || names[0] != "a" {
		t.Fatalf("expected the first batch to still be collected before cancellation, got %v", names)
	}
}

// --- Escapes: Definition() is policy-aware ---------------------------------

func TestListDirectory_DefinitionDiffersByAllowEscapes(t *testing.T) {
	denyTool := NewListDirectory(t.TempDir(), 100, 65536, false)
	askTool := NewListDirectory(t.TempDir(), 100, 65536, true)

	if denyTool.Definition().Description == askTool.Definition().Description {
		t.Fatal("description should differ between deny and ask policies")
	}
	if strings.Contains(denyTool.Definition().Description, "harness will ask") {
		t.Fatal("deny-policy description should not advertise escape prompting")
	}
	if !strings.Contains(askTool.Definition().Description, "harness will ask") {
		t.Fatal("ask-policy description should advertise escape prompting")
	}
	if !strings.Contains(denyTool.Definition().Description, "~/") {
		t.Fatal("deny-policy description should still mention ~/ expansion, since it applies regardless of policy")
	}
}

// --- Escapes: ClassifyCall --------------------------------------------------

func TestListDirectory_ClassifyCall_NilWhenEscapesDisabled(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	tool := NewListDirectory(cwd, 100, 65536, false)
	raw, _ := json.Marshal(ListDirectoryArgs{Path: outside})
	if req := tool.ClassifyCall(raw); req != nil {
		t.Fatalf("expected nil classification with AllowEscapes=false, got %+v", req)
	}
}

func TestListDirectory_ClassifyCall_NilForInWorkdirPath(t *testing.T) {
	cwd := t.TempDir()
	tool := NewListDirectory(cwd, 100, 65536, true)
	raw, _ := json.Marshal(ListDirectoryArgs{Path: "."})
	if req := tool.ClassifyCall(raw); req != nil {
		t.Fatalf("expected nil classification for an in-workdir path, got %+v", req)
	}
}

func TestListDirectory_ClassifyCall_NilOnBadJSON(t *testing.T) {
	tool := NewListDirectory(t.TempDir(), 100, 65536, true)
	if req := tool.ClassifyCall(json.RawMessage(`{not json`)); req != nil {
		t.Fatalf("expected nil classification on invalid JSON, got %+v", req)
	}
}

func TestListDirectory_ClassifyCall_OutsidePathPopulatesRequest(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "f.txt"), "x")

	tool := NewListDirectory(cwd, 100, 65536, true)
	raw, _ := json.Marshal(ListDirectoryArgs{Path: outside})
	req := tool.ClassifyCall(raw)
	if req == nil {
		t.Fatal("expected a non-nil escape request for an outside directory")
	}
	if req.RequestedPath != outside {
		t.Errorf("RequestedPath = %q, want %q", req.RequestedPath, outside)
	}
	if req.ResolvedPath != outside {
		t.Errorf("ResolvedPath = %q, want %q", req.ResolvedPath, outside)
	}
	if req.GrantDir != outside {
		t.Errorf("GrantDir = %q, want the directory itself %q", req.GrantDir, outside)
	}
	if !req.Target.Valid {
		t.Error("expected Target.Valid=true for an existing directory")
	}
}

// --- Escapes: Execute grant enforcement -------------------------------------

func TestListDirectory_Execute_OutsidePathWithoutGrantIsRejected(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()

	tool := NewListDirectory(cwd, 100, 65536, true)
	res := mustExecLD(t, tool, ListDirectoryArgs{Path: outside})
	if !res.IsError {
		t.Fatalf("expected rejection without a grant, got: %s", res.Content)
	}
}

func TestListDirectory_Execute_MatchingGrantListsOutsideDir(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "only.txt"), "x")

	tool := NewListDirectory(cwd, 100, 65536, true)
	raw, _ := json.Marshal(ListDirectoryArgs{Path: outside})
	req := tool.ClassifyCall(raw)
	if req == nil {
		t.Fatal("expected classification to succeed")
	}

	grant := ExecGrant{ApprovedPath: req.ResolvedPath, Target: req.Target}
	res, err := tool.Execute(context.Background(), raw, grant)
	if err != nil {
		t.Fatalf("execute returned hard error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error with a valid grant: %s", res.Content)
	}
	if !strings.Contains(res.Content, "only.txt") {
		t.Fatalf("expected directory listing, got: %s", res.Content)
	}
}

func TestListDirectory_Execute_GrantWithMismatchedPathIsRejected(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	otherDir := t.TempDir()

	tool := NewListDirectory(cwd, 100, 65536, true)
	raw, _ := json.Marshal(ListDirectoryArgs{Path: outside})

	otherID, err := captureIdentity(otherDir)
	if err != nil {
		t.Fatalf("captureIdentity: %v", err)
	}
	grant := ExecGrant{ApprovedPath: otherDir, Target: otherID}
	res, execErr := tool.Execute(context.Background(), raw, grant)
	if execErr != nil {
		t.Fatalf("execute returned hard error: %v", execErr)
	}
	if !res.IsError {
		t.Fatalf("expected rejection when the grant's approved path does not match the request, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "changed during approval") {
		t.Fatalf("expected 'changed during approval' message, got: %s", res.Content)
	}
}

func TestListDirectory_Execute_GrantWithMismatchedIdentityIsRejected(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()

	tool := NewListDirectory(cwd, 100, 65536, true)
	raw, _ := json.Marshal(ListDirectoryArgs{Path: outside})

	grant := ExecGrant{ApprovedPath: outside, Target: FileID{Dev: 999999, Ino: 999999, Valid: true}}
	res, err := tool.Execute(context.Background(), raw, grant)
	if err != nil {
		t.Fatalf("execute returned hard error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected rejection for a fabricated/mismatched identity, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "changed during approval") {
		t.Fatalf("expected 'changed during approval' message, got: %s", res.Content)
	}
}

func TestListDirectory_Execute_MissingTargetApprovedStillMissingReportsNotFound(t *testing.T) {
	cwd := t.TempDir()
	outside := filepath.Join(t.TempDir(), "does-not-exist")

	tool := NewListDirectory(cwd, 100, 65536, true)
	raw, _ := json.Marshal(ListDirectoryArgs{Path: outside})
	grant := ExecGrant{ApprovedPath: outside, Target: FileID{Valid: false}}

	res, err := tool.Execute(context.Background(), raw, grant)
	if err != nil {
		t.Fatalf("execute returned hard error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error for a still-missing approved target")
	}
	if !strings.Contains(res.Content, "not found") {
		t.Fatalf("expected 'not found' message, got: %s", res.Content)
	}
}

// TestListDirectory_Execute_AllowEscapesFalseIgnoresGrant is the defense-in-
// depth check: even if a caller (mis)constructs a valid-looking grant, a
// tool built with AllowEscapes=false must still enforce plain containment.
func TestListDirectory_Execute_AllowEscapesFalseIgnoresGrant(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()

	tool := NewListDirectory(cwd, 100, 65536, false) // deny policy
	raw, _ := json.Marshal(ListDirectoryArgs{Path: outside})

	id, err := captureIdentity(outside)
	if err != nil {
		t.Fatalf("captureIdentity: %v", err)
	}
	grant := ExecGrant{ApprovedPath: outside, Target: id}
	res, execErr := tool.Execute(context.Background(), raw, grant)
	if execErr != nil {
		t.Fatalf("execute returned hard error: %v", execErr)
	}
	if !res.IsError {
		t.Fatal("expected containment to apply regardless of a grant when AllowEscapes=false")
	}
}

// Edge case: the model asks to list_directory an outside path that turns out
// to be a file, not a directory. Classification doesn't check file type (it
// just captures identity), so this must fail gracefully at Execute rather
// than crash or misreport "changed during approval".
func TestListDirectory_Execute_EscapeTargetIsFileReportsNotADirectory(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	filePath := filepath.Join(outside, "file.txt")
	writeFile(t, filePath, "x")

	tool := NewListDirectory(cwd, 100, 65536, true)
	raw, _ := json.Marshal(ListDirectoryArgs{Path: filePath})
	req := tool.ClassifyCall(raw)
	if req == nil {
		t.Fatal("expected classification to succeed even for a file target")
	}
	grant := ExecGrant{ApprovedPath: req.ResolvedPath, Target: req.Target}

	res, err := tool.Execute(context.Background(), raw, grant)
	if err != nil {
		t.Fatalf("execute returned hard error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected an error for a file target, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "not a directory") {
		t.Fatalf("expected 'not a directory' message, got: %s", res.Content)
	}
}

// TestListDirectory_Execute_RootPathRoundTrips exercises the actual host
// filesystem's "/" end to end: classification captures its identity via
// captureIdentity("/") (no final component), the resulting grant is
// accepted, and Execute lists it through os.OpenRoot("/"). Contents aren't
// asserted (environment-dependent) — only that the whole path succeeds.
func TestListDirectory_Execute_RootPathRoundTrips(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("root path semantics differ on windows")
	}
	tool := NewListDirectory(t.TempDir(), 100, 65536, true)
	raw, _ := json.Marshal(ListDirectoryArgs{Path: "/"})
	req := tool.ClassifyCall(raw)
	if req == nil {
		t.Fatal("expected classification to succeed for /")
	}
	if req.ResolvedPath != "/" {
		t.Fatalf("ResolvedPath = %q, want /", req.ResolvedPath)
	}

	grant := ExecGrant{ApprovedPath: req.ResolvedPath, Target: req.Target}
	res, err := tool.Execute(context.Background(), raw, grant)
	if err != nil {
		t.Fatalf("execute returned hard error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error listing /: %s", res.Content)
	}
}
