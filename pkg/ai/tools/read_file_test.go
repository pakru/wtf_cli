package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func mustExec(t *testing.T, tool *ReadFile, args any) Result {
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

func TestReadFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "hello.txt"), "line1\nline2\nline3\n")

	tool := NewReadFile(dir, 100, 8192, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "hello.txt"})

	if res.IsError {
		t.Fatalf("unexpected error result: %s", res.Content)
	}
	if !strings.Contains(res.Content, "line1") || !strings.Contains(res.Content, "line3") {
		t.Fatalf("content missing lines:\n%s", res.Content)
	}
	if !strings.Contains(res.Content, "hello.txt (lines 1-3)") {
		t.Fatalf("missing header in content:\n%s", res.Content)
	}
}

func TestReadFile_LineRange(t *testing.T) {
	dir := t.TempDir()
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	writeFile(t, filepath.Join(dir, "f.txt"), strings.Join(lines, "\n"))

	tool := NewReadFile(dir, 100, 8192, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "f.txt", StartLine: 5, EndLine: 7})

	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "line 5\nline 6\nline 7") {
		t.Fatalf("content missing requested range:\n%s", res.Content)
	}
	if strings.Contains(res.Content, "line 8") {
		t.Fatalf("content includes lines past end_line:\n%s", res.Content)
	}
}

func TestReadFile_DefaultEndLine(t *testing.T) {
	dir := t.TempDir()
	var lines []string
	for i := 1; i <= 250; i++ {
		lines = append(lines, fmt.Sprintf("L%d", i))
	}
	writeFile(t, filepath.Join(dir, "big.txt"), strings.Join(lines, "\n"))

	tool := NewReadFile(dir, 500, 1<<20, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "big.txt"})

	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "lines 1-200") {
		t.Fatalf("expected default span to be lines 1-200, got header:\n%s", res.Content)
	}
}

func TestReadFile_MaxLinesCap(t *testing.T) {
	dir := t.TempDir()
	var lines []string
	for i := 1; i <= 50; i++ {
		lines = append(lines, fmt.Sprintf("L%d", i))
	}
	writeFile(t, filepath.Join(dir, "f.txt"), strings.Join(lines, "\n"))

	tool := NewReadFile(dir, 10, 1<<20, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "f.txt", StartLine: 1, EndLine: 50})

	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "truncated") {
		t.Fatalf("expected truncation marker, got:\n%s", res.Content)
	}
	if strings.Contains(res.Content, "L11") {
		t.Fatalf("content includes lines past max_lines cap:\n%s", res.Content)
	}
}

func TestReadFile_MaxBytesCap(t *testing.T) {
	dir := t.TempDir()
	// 20 lines of 50 chars each → 1KB total. Cap at 200 bytes.
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, strings.Repeat("x", 50))
	}
	writeFile(t, filepath.Join(dir, "f.txt"), strings.Join(lines, "\n"))

	tool := NewReadFile(dir, 100, 200, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "f.txt", StartLine: 1, EndLine: 20})

	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "truncated") {
		t.Fatalf("expected truncation marker, got:\n%s", res.Content)
	}
}

func TestReadFile_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ok.txt"), "inside\n")

	tool := NewReadFile(dir, 100, 8192, false)
	// A generous, depth-independent "../" count: t.TempDir() nests only a
	// couple of levels on Linux but considerably deeper on macOS (a
	// supported target), so a fixed small count is not portable.
	res := mustExec(t, tool, ReadFileArgs{Path: strings.Repeat("../", 20) + "etc/passwd"})

	if !res.IsError {
		t.Fatalf("expected error for traversal, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "outside working directory") {
		t.Fatalf("error message should mention containment, got: %s", res.Content)
	}
}

func TestReadFile_RejectsAbsoluteOutsideCwd(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "shh\n")

	tool := NewReadFile(cwd, 100, 8192, false)
	res := mustExec(t, tool, ReadFileArgs{Path: filepath.Join(outside, "secret.txt")})

	if !res.IsError {
		t.Fatalf("expected error for outside-cwd absolute path, got: %s", res.Content)
	}
}

func TestReadFile_RejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	cwd := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "shh\n")

	link := filepath.Join(cwd, "escape")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	tool := NewReadFile(cwd, 100, 8192, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "escape"})

	if !res.IsError {
		t.Fatalf("expected symlink escape to be rejected, got: %s", res.Content)
	}
}

func TestReadFile_AllowsSymlinkInsideCwd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, "real.txt"), "hello\n")

	link := filepath.Join(cwd, "alias")
	// Relative target: os.Root follows in-tree symlinks as long as they stay
	// inside the root. See TestReadFile_RejectsAbsoluteSymlinkedFileEvenPointingInsideCwd
	// for the absolute-target case, which os.Root rejects even when the
	// target is itself inside cwd.
	if err := os.Symlink("real.txt", link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	tool := NewReadFile(cwd, 100, 8192, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "alias"})

	if res.IsError {
		t.Fatalf("symlink inside cwd should be allowed, got error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "hello") {
		t.Fatalf("expected file content, got: %s", res.Content)
	}
}

// os.Root documents that symbolic links must not be absolute — even when the
// absolute target happens to point back inside the root, it is rejected.
// This is a known, accepted limitation (see design doc), matching the same
// limitation list_directory already has for symlinked directories.
func TestReadFile_RejectsAbsoluteSymlinkedFileEvenPointingInsideCwd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, "real.txt"), "hello\n")

	link := filepath.Join(cwd, "alias")
	if err := os.Symlink(filepath.Join(cwd, "real.txt"), link); err != nil { // absolute target
		t.Fatalf("symlink: %v", err)
	}

	tool := NewReadFile(cwd, 100, 8192, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "alias"})

	if !res.IsError {
		t.Fatalf("expected absolute-target symlink traversal to be rejected by os.Root, got: %s", res.Content)
	}
}

func TestReadFile_MissingFile(t *testing.T) {
	tool := NewReadFile(t.TempDir(), 100, 8192, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "does_not_exist.txt"})

	if !res.IsError {
		t.Fatalf("expected error for missing file, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "not found") {
		t.Fatalf("expected 'not found' message, got: %s", res.Content)
	}
}

func TestReadFile_RejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	tool := NewReadFile(dir, 100, 8192, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "sub"})

	if !res.IsError {
		t.Fatalf("expected error for directory, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "directory") {
		t.Fatalf("expected directory message, got: %s", res.Content)
	}
}

func TestReadFile_BadJSON(t *testing.T) {
	tool := NewReadFile(t.TempDir(), 100, 8192, false)
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

func TestReadFile_EmptyPath(t *testing.T) {
	tool := NewReadFile(t.TempDir(), 100, 8192, false)
	res := mustExec(t, tool, ReadFileArgs{Path: ""})

	if !res.IsError {
		t.Fatalf("expected error for empty path, got: %s", res.Content)
	}
}

func TestReadFile_UnconfiguredCwd(t *testing.T) {
	tool := NewReadFile("", 100, 8192, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "anything"})

	if !res.IsError {
		t.Fatalf("expected error for empty cwd, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "working directory") {
		t.Fatalf("expected cwd message, got: %s", res.Content)
	}
}

func TestReadFile_StartPastEOF(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "f.txt"), "one\ntwo\n")

	tool := NewReadFile(dir, 100, 8192, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "f.txt", StartLine: 100})

	if !res.IsError {
		t.Fatalf("expected error for start past EOF, got: %s", res.Content)
	}
}

func TestReadFile_EndBeyondEOFIsClamped(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "f.txt"), "one\ntwo\nthree\n")

	tool := NewReadFile(dir, 100, 8192, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "f.txt", StartLine: 1, EndLine: 100})

	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "lines 1-3") {
		t.Fatalf("expected clamped header, got:\n%s", res.Content)
	}
}

func TestReadFile_NegativeRangeRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "f.txt"), "x\n")

	tool := NewReadFile(dir, 100, 8192, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "f.txt", StartLine: -1})

	if !res.IsError {
		t.Fatalf("expected error for negative start, got: %s", res.Content)
	}
}

func TestReadFile_EndBeforeStartRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "f.txt"), "a\nb\nc\n")

	tool := NewReadFile(dir, 100, 8192, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "f.txt", StartLine: 5, EndLine: 2})

	if !res.IsError {
		t.Fatalf("expected error for end<start, got: %s", res.Content)
	}
}

func TestReadFile_NonUTF8Content(t *testing.T) {
	dir := t.TempDir()
	// Invalid UTF-8: 0xC3 alone (incomplete two-byte sequence).
	if err := os.WriteFile(filepath.Join(dir, "bin.txt"), []byte{'a', 0xC3, 'b', '\n'}, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tool := NewReadFile(dir, 100, 8192, false)
	res := mustExec(t, tool, ReadFileArgs{Path: "bin.txt"})

	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	// The replacement char should appear in place of the invalid byte.
	if !strings.ContainsRune(res.Content, '�') {
		t.Fatalf("expected replacement char for invalid UTF-8, got: %q", res.Content)
	}
}

func TestReadFile_ContextCanceled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "f.txt"), "x\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tool := NewReadFile(dir, 100, 8192, false)
	raw, _ := json.Marshal(ReadFileArgs{Path: "f.txt"})
	_, err := tool.Execute(ctx, raw, ExecGrant{})
	if err == nil {
		t.Fatal("expected hard error from canceled context")
	}
}

func TestReadFile_Definition(t *testing.T) {
	tool := NewReadFile(t.TempDir(), 100, 8192, false)
	def := tool.Definition()
	if def.Name != "read_file" {
		t.Fatalf("name = %q, want read_file", def.Name)
	}
	if def.Description == "" {
		t.Fatal("description is empty")
	}
	if len(def.JSONSchema) == 0 {
		t.Fatal("schema is empty")
	}
	// Schema should be valid JSON.
	var v map[string]any
	if err := json.Unmarshal(def.JSONSchema, &v); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
}

func TestReadFile_NewReadFileNormalizesCaps(t *testing.T) {
	tool := NewReadFile("/tmp", -10, -10, false)
	if tool.MaxLines < readFileMinMaxLines {
		t.Errorf("MaxLines = %d, want >= %d", tool.MaxLines, readFileMinMaxLines)
	}
	if tool.MaxBytes < readFileMinMaxBytes {
		t.Errorf("MaxBytes = %d, want >= %d", tool.MaxBytes, readFileMinMaxBytes)
	}
}

// --- Escapes: Definition() is policy-aware ---------------------------------

func TestReadFile_DefinitionDiffersByAllowEscapes(t *testing.T) {
	denyTool := NewReadFile(t.TempDir(), 100, 8192, false)
	askTool := NewReadFile(t.TempDir(), 100, 8192, true)

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

func TestReadFile_ClassifyCall_NilWhenEscapesDisabled(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	tool := NewReadFile(cwd, 100, 8192, false)
	raw, _ := json.Marshal(ReadFileArgs{Path: filepath.Join(outside, "x")})
	if req := tool.ClassifyCall(raw); req != nil {
		t.Fatalf("expected nil classification with AllowEscapes=false, got %+v", req)
	}
}

func TestReadFile_ClassifyCall_NilForInWorkdirPath(t *testing.T) {
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, "in.txt"), "x")
	tool := NewReadFile(cwd, 100, 8192, true)
	raw, _ := json.Marshal(ReadFileArgs{Path: "in.txt"})
	if req := tool.ClassifyCall(raw); req != nil {
		t.Fatalf("expected nil classification for an in-workdir path, got %+v", req)
	}
}

func TestReadFile_ClassifyCall_NilOnBadJSON(t *testing.T) {
	tool := NewReadFile(t.TempDir(), 100, 8192, true)
	if req := tool.ClassifyCall(json.RawMessage(`{not json`)); req != nil {
		t.Fatalf("expected nil classification on invalid JSON, got %+v", req)
	}
}

func TestReadFile_ClassifyCall_OutsidePathPopulatesRequest(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	writeFile(t, target, "shh")

	tool := NewReadFile(cwd, 100, 8192, true)
	raw, _ := json.Marshal(ReadFileArgs{Path: target})
	req := tool.ClassifyCall(raw)
	if req == nil {
		t.Fatal("expected a non-nil escape request for an outside path")
	}
	if req.RequestedPath != target {
		t.Errorf("RequestedPath = %q, want %q", req.RequestedPath, target)
	}
	if req.ResolvedPath != target {
		t.Errorf("ResolvedPath = %q, want %q", req.ResolvedPath, target)
	}
	if req.GrantDir != outside {
		t.Errorf("GrantDir = %q, want parent dir %q", req.GrantDir, outside)
	}
	if !req.Target.Valid {
		t.Error("expected Target.Valid=true for an existing file")
	}
}

func TestReadFile_ClassifyCall_MissingOutsideFileStillClassifiesWithInvalidTarget(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "does-not-exist.txt")

	tool := NewReadFile(cwd, 100, 8192, true)
	raw, _ := json.Marshal(ReadFileArgs{Path: target})
	req := tool.ClassifyCall(raw)
	if req == nil {
		t.Fatal("expected an escape request even though the target does not exist yet")
	}
	if req.Target.Valid {
		t.Error("expected Target.Valid=false for a missing file")
	}
}

// --- Escapes: Execute grant enforcement -------------------------------------

func TestReadFile_Execute_OutsidePathWithoutGrantIsRejected(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	writeFile(t, target, "shh")

	tool := NewReadFile(cwd, 100, 8192, true)
	res := mustExec(t, tool, ReadFileArgs{Path: target})
	if !res.IsError {
		t.Fatalf("expected rejection without a grant, got: %s", res.Content)
	}
}

func TestReadFile_Execute_MatchingGrantReadsOutsideFile(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	writeFile(t, target, "top secret contents\n")

	tool := NewReadFile(cwd, 100, 8192, true)
	raw, _ := json.Marshal(ReadFileArgs{Path: target})
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
	if !strings.Contains(res.Content, "top secret contents") {
		t.Fatalf("expected file content, got: %s", res.Content)
	}
}

func TestReadFile_Execute_GrantWithMismatchedPathIsRejected(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	other := filepath.Join(outside, "other.txt")
	writeFile(t, target, "shh")
	writeFile(t, other, "not this one")

	tool := NewReadFile(cwd, 100, 8192, true)
	raw, _ := json.Marshal(ReadFileArgs{Path: target})

	// Grant approved a *different* resolved path than the one being requested.
	otherID, err := captureIdentity(other)
	if err != nil {
		t.Fatalf("captureIdentity: %v", err)
	}
	grant := ExecGrant{ApprovedPath: other, Target: otherID}
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

// TestReadFile_Execute_GrantWithMismatchedIdentityIsRejected models a swap in
// the resolve-then-open window: the grant's ApprovedPath matches the fresh
// resolution, but the identity is wrong (as if the object were replaced).
// Path-string equality alone must not be sufficient authorization.
func TestReadFile_Execute_GrantWithMismatchedIdentityIsRejected(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	writeFile(t, target, "shh")

	tool := NewReadFile(cwd, 100, 8192, true)
	raw, _ := json.Marshal(ReadFileArgs{Path: target})

	grant := ExecGrant{ApprovedPath: target, Target: FileID{Dev: 999999, Ino: 999999, Valid: true}}
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

func TestReadFile_Execute_MissingTargetApprovedStillMissingReportsFileNotFound(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "does-not-exist.txt")

	tool := NewReadFile(cwd, 100, 8192, true)
	raw, _ := json.Marshal(ReadFileArgs{Path: target})
	grant := ExecGrant{ApprovedPath: target, Target: FileID{Valid: false}}

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

func TestReadFile_Execute_MissingTargetApprovedButNowExistsRejectsAsChanged(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "appeared.txt")

	tool := NewReadFile(cwd, 100, 8192, true)
	raw, _ := json.Marshal(ReadFileArgs{Path: target})
	grant := ExecGrant{ApprovedPath: target, Target: FileID{Valid: false}}

	// The file appears *after* approval was granted for "does not exist".
	writeFile(t, target, "surprise")

	res, err := tool.Execute(context.Background(), raw, grant)
	if err != nil {
		t.Fatalf("execute returned hard error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected rejection when a file appears where none was approved")
	}
	if !strings.Contains(res.Content, "changed during approval") {
		t.Fatalf("expected 'changed during approval' message, got: %s", res.Content)
	}
}

// TestReadFile_Execute_AllowEscapesFalseIgnoresGrant is the defense-in-depth
// check: even if a caller (mis)constructs a valid-looking grant, a tool built
// with AllowEscapes=false must still enforce plain containment.
func TestReadFile_Execute_AllowEscapesFalseIgnoresGrant(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	writeFile(t, target, "shh")

	tool := NewReadFile(cwd, 100, 8192, false) // deny policy
	raw, _ := json.Marshal(ReadFileArgs{Path: target})

	id, err := captureIdentity(target)
	if err != nil {
		t.Fatalf("captureIdentity: %v", err)
	}
	grant := ExecGrant{ApprovedPath: target, Target: id}
	res, execErr := tool.Execute(context.Background(), raw, grant)
	if execErr != nil {
		t.Fatalf("execute returned hard error: %v", execErr)
	}
	if !res.IsError {
		t.Fatal("expected containment to apply regardless of a grant when AllowEscapes=false")
	}
}

// --- Escapes: FIFO / device promptness --------------------------------------

func TestReadFile_Execute_FIFORejectedPromptlyInWorkdir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFOs are not supported on windows")
	}
	dir := t.TempDir()
	fifoPath := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifoPath, 0o644); err != nil {
		t.Skipf("mkfifo not supported in this environment: %v", err)
	}

	tool := NewReadFile(dir, 100, 8192, false)
	done := make(chan Result, 1)
	go func() {
		done <- mustExec(t, tool, ReadFileArgs{Path: "fifo"})
	}()

	select {
	case res := <-done:
		if !res.IsError {
			t.Fatal("expected a FIFO to be rejected as non-regular")
		}
		if !strings.Contains(res.Content, "not a regular file") {
			t.Fatalf("expected 'not a regular file' message, got: %s", res.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reading a writer-less FIFO blocked instead of returning promptly")
	}
}

func TestReadFile_Execute_FIFORejectedPromptlyOnEscapeBranch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFOs are not supported on windows")
	}
	cwd := t.TempDir()
	outside := t.TempDir()
	fifoPath := filepath.Join(outside, "fifo")
	if err := syscall.Mkfifo(fifoPath, 0o644); err != nil {
		t.Skipf("mkfifo not supported in this environment: %v", err)
	}

	tool := NewReadFile(cwd, 100, 8192, true)
	raw, _ := json.Marshal(ReadFileArgs{Path: fifoPath})
	req := tool.ClassifyCall(raw)
	if req == nil {
		t.Fatal("expected classification to succeed for the FIFO")
	}
	grant := ExecGrant{ApprovedPath: req.ResolvedPath, Target: req.Target}

	done := make(chan Result, 1)
	go func() {
		res, _ := tool.Execute(context.Background(), raw, grant)
		done <- res
	}()

	select {
	case res := <-done:
		if !res.IsError {
			t.Fatal("expected a FIFO to be rejected as non-regular")
		}
		if !strings.Contains(res.Content, "not a regular file") {
			t.Fatalf("expected 'not a regular file' message, got: %s", res.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reading a writer-less FIFO on the escape branch blocked instead of returning promptly")
	}
}

// TestReadFile_Execute_CharDeviceRejectedAsNonRegular covers the escape
// branch's own regular-file check on a real character device, complementing
// the FIFO promptness tests above (a different non-regular type) and
// scope_test.go's classification-level device coverage (which never opens
// the target at all).
func TestReadFile_Execute_CharDeviceRejectedAsNonRegular(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("device files are not supported on windows")
	}
	if _, err := os.Stat("/dev/null"); err != nil {
		t.Skip("/dev/null not available")
	}

	tool := NewReadFile(t.TempDir(), 100, 8192, true)
	raw, _ := json.Marshal(ReadFileArgs{Path: "/dev/null"})
	req := tool.ClassifyCall(raw)
	if req == nil {
		t.Fatal("expected classification to succeed for /dev/null")
	}
	grant := ExecGrant{ApprovedPath: req.ResolvedPath, Target: req.Target}

	res, err := tool.Execute(context.Background(), raw, grant)
	if err != nil {
		t.Fatalf("execute returned hard error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected a character device to be rejected as non-regular")
	}
	if !strings.Contains(res.Content, "not a regular file") {
		t.Fatalf("expected 'not a regular file' message, got: %s", res.Content)
	}
}
