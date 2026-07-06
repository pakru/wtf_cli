package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// --- captureIdentity ------------------------------------------------------

func TestCaptureIdentity_HappyPathMatchesStat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	writeFile(t, path, "hello")

	id, err := captureIdentity(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !id.Valid {
		t.Fatal("expected a valid identity")
	}

	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		t.Fatalf("stat: %v", err)
	}
	if id.Dev != uint64(st.Dev) || id.Ino != uint64(st.Ino) {
		t.Fatalf("captureIdentity = %+v, want dev=%d ino=%d", id, st.Dev, st.Ino)
	}
}

func TestCaptureIdentity_RootPath(t *testing.T) {
	id, err := captureIdentity("/")
	if err != nil {
		t.Fatalf("unexpected error for root path: %v", err)
	}
	if !id.Valid {
		t.Fatal("expected a valid identity for /")
	}

	var st syscall.Stat_t
	if err := syscall.Stat("/", &st); err != nil {
		t.Fatalf("stat /: %v", err)
	}
	if id.Dev != uint64(st.Dev) || id.Ino != uint64(st.Ino) {
		t.Fatalf("captureIdentity(/) = %+v, want dev=%d ino=%d", id, st.Dev, st.Ino)
	}
}

func TestCaptureIdentity_RelativePathRejected(t *testing.T) {
	if _, err := captureIdentity("relative/path"); err == nil {
		t.Fatal("expected error for a non-absolute path")
	}
}

// TestCaptureIdentity_ParentSwapRegression is the core race-boundary test:
// identity must be captured through a no-follow walk of the *literal*
// canonical path, so a parent directory swapped to a symlink after
// canonicalization must fail the capture rather than silently follow the
// swap and report the identity of whatever is on the other side.
func TestCaptureIdentity_ParentSwapRegression(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	base := t.TempDir()
	realParent := filepath.Join(base, "a")
	if err := os.MkdirAll(realParent, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	canonicalPath := filepath.Join(realParent, "b")
	writeFile(t, canonicalPath, "hello")

	// Simulate a concurrent swap: the parent directory is replaced with a
	// symlink pointing somewhere else entirely, after canonicalPath was
	// already computed.
	elsewhere := t.TempDir()
	if err := os.RemoveAll(realParent); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.Symlink(elsewhere, realParent); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if _, err := captureIdentity(canonicalPath); err == nil {
		t.Fatal("expected captureIdentity to fail when a parent component was swapped to a symlink after canonicalization")
	}
}

func TestCaptureIdentity_FinalComponentSymlinkRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "real.txt")
	writeFile(t, real, "x")
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if _, err := captureIdentity(link); err == nil {
		t.Fatal("expected error when the final component is itself a symlink")
	}
}

func TestCaptureIdentity_FinalComponentMissingReturnsInvalidFileID(t *testing.T) {
	dir := t.TempDir()
	id, err := captureIdentity(filepath.Join(dir, "does-not-exist"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.Valid {
		t.Fatal("expected Valid=false for a missing final component")
	}
}

func TestCaptureIdentity_IntermediateComponentMissingFailsClosed(t *testing.T) {
	dir := t.TempDir()
	_, err := captureIdentity(filepath.Join(dir, "missing-parent", "child"))
	if err == nil {
		t.Fatal("expected an error (fail closed), not Valid=false, when an intermediate directory is missing")
	}
}

func TestCaptureIdentity_PermissionDeniedIntermediateFailsClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits differ on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses permission checks")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	_, err := captureIdentity(filepath.Join(locked, "child"))
	if err == nil {
		t.Fatal("expected an error (fail closed), not Valid=false, for a permission-denied intermediate directory")
	}
}

// TestCaptureIdentity_FIFODoesNotOpenTargetAndReturnsPromptly verifies the
// final component is captured via a stat relative to its parent, never an
// open — opening a writer-less FIFO for read can block indefinitely, and
// classification runs before the user has approved anything.
func TestCaptureIdentity_FIFODoesNotOpenTargetAndReturnsPromptly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFOs are not supported on windows")
	}
	dir := t.TempDir()
	fifoPath := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifoPath, 0o644); err != nil {
		t.Skipf("mkfifo not supported in this environment: %v", err)
	}

	type outcome struct {
		id  FileID
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		id, err := captureIdentity(fifoPath)
		done <- outcome{id, err}
	}()

	select {
	case o := <-done:
		if o.err != nil {
			t.Fatalf("unexpected error: %v", o.err)
		}
		if !o.id.Valid {
			t.Fatal("expected a valid identity for an existing FIFO")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("captureIdentity blocked on a writer-less FIFO — it must never open the target")
	}
}

func TestCaptureIdentity_CharDeviceIdentityWithoutOpening(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("device files are not supported on windows")
	}
	if _, err := os.Stat("/dev/null"); err != nil {
		t.Skip("/dev/null not available")
	}
	id, err := captureIdentity("/dev/null")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !id.Valid {
		t.Fatal("expected a valid identity for /dev/null")
	}
}

// --- classifyPath ----------------------------------------------------------

func TestClassifyPath_InsideAndOutside(t *testing.T) {
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, "in.txt"), "x")
	outside := t.TempDir()

	tests := []struct {
		name       string
		path       string
		wantInside bool
	}{
		{"relative inside", "in.txt", true},
		{"dot", ".", true},
		// A generous, depth-independent "../" count: t.TempDir() nests only
		// a couple of levels on Linux but considerably deeper on macOS (a
		// supported target), so a fixed small count is not portable.
		{"relative traversal outside", strings.Repeat("../", 20) + "etc/hosts", false},
		{"absolute outside", filepath.Join(outside, "x"), false},
		{"absolute inside", filepath.Join(cwd, "in.txt"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, inside, err := classifyPath(cwd, tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if inside != tt.wantInside {
				t.Fatalf("inside = %v, want %v", inside, tt.wantInside)
			}
		})
	}
}

// Regression: a naive string-prefix containment check would wrongly accept a
// sibling directory whose name happens to start with the cwd's name.
func TestClassifyPath_RejectsAbsoluteSiblingPrefix(t *testing.T) {
	parent := t.TempDir()
	cwd := filepath.Join(parent, "project")
	sibling := filepath.Join(parent, "project-other")
	if err := os.Mkdir(cwd, 0o755); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatalf("mkdir sibling: %v", err)
	}

	_, inside, err := classifyPath(cwd, sibling)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inside {
		t.Fatal("sibling directory with a prefixed name must be classified as outside")
	}
}

func TestClassifyPath_SymlinkPointingOutsideIsOutside(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	cwd := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	writeFile(t, secret, "x")
	link := filepath.Join(cwd, "escape")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	resolved, inside, err := classifyPath(cwd, "escape")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inside {
		t.Fatal("symlink pointing outside cwd must classify as outside")
	}
	if resolved != secret {
		t.Fatalf("resolved = %q, want %q", resolved, secret)
	}
}

func TestClassifyPath_TildeExpandsBeforeClassification(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory available")
	}
	cwd := t.TempDir()
	resolved, inside, err := classifyPath(cwd, "~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	homeReal, herr := filepath.EvalSymlinks(home)
	if herr != nil {
		homeReal = home
	}
	if resolved != homeReal {
		t.Fatalf("resolved = %q, want home dir %q", resolved, homeReal)
	}
	if inside {
		t.Fatal("home directory should not classify as inside an unrelated temp cwd")
	}
}

// --- expandTilde -----------------------------------------------------------

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory available")
	}

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{"bare tilde", "~", home, false},
		{"tilde slash", "~/foo/bar", filepath.Join(home, "foo/bar"), false},
		{"tilde user rejected", "~someone/foo", "", true},
		{"no tilde relative", "relative/path", "relative/path", false},
		{"no tilde absolute", "/etc/hosts", "/etc/hosts", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandTilde(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- fileID / FileID.equal ---------------------------------------------

func TestFileID_Equal(t *testing.T) {
	a := FileID{Dev: 1, Ino: 2, Valid: true}
	b := FileID{Dev: 1, Ino: 2, Valid: true}
	c := FileID{Dev: 1, Ino: 3, Valid: true}
	invalid := FileID{}

	if !a.equal(b) {
		t.Fatal("identical valid FileIDs should be equal")
	}
	if a.equal(c) {
		t.Fatal("different inode should not be equal")
	}
	if invalid.equal(invalid) {
		t.Fatal("two invalid FileIDs must never compare equal — 'still missing' is checked via Valid, not identity")
	}
	if a.equal(invalid) || invalid.equal(a) {
		t.Fatal("a valid and an invalid FileID must never be equal")
	}
}

func TestFileID_FromFileInfo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	writeFile(t, path, "x")

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	id, ok := fileID(info)
	if !ok {
		t.Fatal("expected fileID to succeed on a regular file")
	}
	if !id.Valid {
		t.Fatal("expected Valid=true")
	}

	captured, err := captureIdentity(path)
	if err != nil {
		t.Fatalf("captureIdentity: %v", err)
	}
	if !id.equal(captured) {
		t.Fatalf("fileID(Stat) = %+v, captureIdentity = %+v; want equal", id, captured)
	}
}
