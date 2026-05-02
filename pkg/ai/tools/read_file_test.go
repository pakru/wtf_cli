package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
	res, err := tool.Execute(context.Background(), raw)
	if err != nil {
		t.Fatalf("execute returned hard error: %v", err)
	}
	return res
}

func TestReadFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "hello.txt"), "line1\nline2\nline3\n")

	tool := NewReadFile(dir, 100, 8192)
	res := mustExec(t, tool, ReadFileArgs{Path: "hello.txt"})

	if res.IsError {
		t.Fatalf("unexpected error result: %s", res.Content)
	}
	if !strings.Contains(res.Content, "line1") || !strings.Contains(res.Content, "line3") {
		t.Fatalf("content missing lines:\n%s", res.Content)
	}
	if !strings.Contains(res.Content, "hello.txt (lines 1-3 of 3)") {
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

	tool := NewReadFile(dir, 100, 8192)
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

	tool := NewReadFile(dir, 500, 1<<20)
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

	tool := NewReadFile(dir, 10, 1<<20)
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

	tool := NewReadFile(dir, 100, 200)
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

	tool := NewReadFile(dir, 100, 8192)
	res := mustExec(t, tool, ReadFileArgs{Path: "../../../etc/passwd"})

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

	tool := NewReadFile(cwd, 100, 8192)
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

	tool := NewReadFile(cwd, 100, 8192)
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
	if err := os.Symlink(filepath.Join(cwd, "real.txt"), link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	tool := NewReadFile(cwd, 100, 8192)
	res := mustExec(t, tool, ReadFileArgs{Path: "alias"})

	if res.IsError {
		t.Fatalf("symlink inside cwd should be allowed, got error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "hello") {
		t.Fatalf("expected file content, got: %s", res.Content)
	}
}

func TestReadFile_MissingFile(t *testing.T) {
	tool := NewReadFile(t.TempDir(), 100, 8192)
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

	tool := NewReadFile(dir, 100, 8192)
	res := mustExec(t, tool, ReadFileArgs{Path: "sub"})

	if !res.IsError {
		t.Fatalf("expected error for directory, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "directory") {
		t.Fatalf("expected directory message, got: %s", res.Content)
	}
}

func TestReadFile_BadJSON(t *testing.T) {
	tool := NewReadFile(t.TempDir(), 100, 8192)
	res, err := tool.Execute(context.Background(), json.RawMessage(`{not json`))
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
	tool := NewReadFile(t.TempDir(), 100, 8192)
	res := mustExec(t, tool, ReadFileArgs{Path: ""})

	if !res.IsError {
		t.Fatalf("expected error for empty path, got: %s", res.Content)
	}
}

func TestReadFile_UnconfiguredCwd(t *testing.T) {
	tool := NewReadFile("", 100, 8192)
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

	tool := NewReadFile(dir, 100, 8192)
	res := mustExec(t, tool, ReadFileArgs{Path: "f.txt", StartLine: 100})

	if !res.IsError {
		t.Fatalf("expected error for start past EOF, got: %s", res.Content)
	}
}

func TestReadFile_EndBeyondEOFIsClamped(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "f.txt"), "one\ntwo\nthree\n")

	tool := NewReadFile(dir, 100, 8192)
	res := mustExec(t, tool, ReadFileArgs{Path: "f.txt", StartLine: 1, EndLine: 100})

	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "lines 1-3 of 3") {
		t.Fatalf("expected clamped header, got:\n%s", res.Content)
	}
}

func TestReadFile_NegativeRangeRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "f.txt"), "x\n")

	tool := NewReadFile(dir, 100, 8192)
	res := mustExec(t, tool, ReadFileArgs{Path: "f.txt", StartLine: -1})

	if !res.IsError {
		t.Fatalf("expected error for negative start, got: %s", res.Content)
	}
}

func TestReadFile_EndBeforeStartRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "f.txt"), "a\nb\nc\n")

	tool := NewReadFile(dir, 100, 8192)
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

	tool := NewReadFile(dir, 100, 8192)
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

	tool := NewReadFile(dir, 100, 8192)
	raw, _ := json.Marshal(ReadFileArgs{Path: "f.txt"})
	_, err := tool.Execute(ctx, raw)
	if err == nil {
		t.Fatal("expected hard error from canceled context")
	}
}

func TestReadFile_Definition(t *testing.T) {
	tool := NewReadFile(t.TempDir(), 100, 8192)
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
	tool := NewReadFile("/tmp", -10, -10)
	if tool.MaxLines < readFileMinMaxLines {
		t.Errorf("MaxLines = %d, want >= %d", tool.MaxLines, readFileMinMaxLines)
	}
	if tool.MaxBytes < readFileMinMaxBytes {
		t.Errorf("MaxBytes = %d, want >= %d", tool.MaxBytes, readFileMinMaxBytes)
	}
}
