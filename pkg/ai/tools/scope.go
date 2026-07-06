package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// FileID identifies a filesystem object by device and inode. Valid is false
// when the target did not exist at capture time (final-component ENOENT —
// the only capture failure that does not fail closed; see captureIdentity).
type FileID struct {
	Dev, Ino uint64
	Valid    bool
}

// equal reports whether two FileIDs refer to the same existing object. Two
// invalid IDs are never equal — "the target is still missing" is checked by
// the caller via Valid, not by identity comparison.
func (f FileID) equal(other FileID) bool {
	return f.Valid && other.Valid && f.Dev == other.Dev && f.Ino == other.Ino
}

// EscapeRequest describes a tool call that targets a path outside the
// working directory and is eligible for user-approved access.
type EscapeRequest struct {
	RequestedPath string // path exactly as the model supplied it
	ResolvedPath  string // absolute, symlink-resolved target
	GrantDir      string // directory a session grant would cover
	Target        FileID // identity of ResolvedPath, captured no-follow
}

// EscapeClassifier is implemented by tools that support user-approved
// out-of-workdir access. ClassifyCall returns nil when the call needs no
// escape: the path is inside the working directory, escapes are disabled by
// config, or the args/path cannot be resolved or identity-captured. Execute
// remains the sole error authority — a nil classification never blocks a
// call, it just means the ordinary in-workdir approval/containment applies.
type EscapeClassifier interface {
	ClassifyCall(args json.RawMessage) *EscapeRequest
}

// ExecGrant carries the per-call scope approved for this execution. The zero
// value grants nothing beyond workdir containment.
type ExecGrant struct {
	// ApprovedPath is the exact resolved path (file for read_file, directory
	// for list_directory) the approval covered; empty means workdir-only.
	ApprovedPath string
	// Target is the object identity the approval covered. Execute verifies
	// the opened handle against it with fstat; Valid=false means the target
	// must still not exist.
	Target FileID
}

// expandTilde replaces a leading "~" or "~/..." with the user's home
// directory. "~user" forms are rejected — cross-user home lookup is out of
// scope. Paths without a leading "~" are returned unchanged.
func expandTilde(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		if strings.HasPrefix(path, "~") {
			return "", fmt.Errorf("~user paths are not supported")
		}
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

// classifyPath resolves userPath against cwd — expanding a leading "~",
// absolutizing, cleaning, and following symlinks (allowing a missing final
// component via evalSymlinksAllowingMissing) — and reports whether the
// result lies inside cwd.
//
// This is shared resolution logic for escape classification and for
// re-resolving a path fresh at execution time. It is NOT an enforcement
// boundary by itself: in-workdir enforcement goes through os.Root, and
// out-of-workdir enforcement goes through a grant-checked, identity-verified
// open. See the tools' Execute methods.
func classifyPath(cwd, userPath string) (resolved string, insideWorkdir bool, err error) {
	expanded, err := expandTilde(userPath)
	if err != nil {
		return "", false, err
	}

	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return "", false, fmt.Errorf("resolve cwd: %w", err)
	}

	candidate := expanded
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(cwdAbs, candidate)
	}
	candidate = filepath.Clean(candidate)

	resolved, err = evalSymlinksAllowingMissing(candidate)
	if err != nil {
		return "", false, err
	}

	cwdReal, err := filepath.EvalSymlinks(cwdAbs)
	if err != nil {
		// If cwd itself can't be resolved, fall back to the absolute form
		// rather than failing outright — mirrors resolveContainedPath's
		// existing behavior for a just-created working directory.
		cwdReal = cwdAbs
	}

	rel, err := filepath.Rel(cwdReal, resolved)
	if err != nil {
		return resolved, false, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return resolved, false, nil
	}
	return resolved, true, nil
}

// normalizeToRootRelative converts a model-supplied path into a path
// relative to cwdAbs, rejecting anything that lexically escapes it. This
// only improves error messages; os.Root (the actual enforcement boundary)
// rejects escapes regardless. Absolute paths are compared with filepath.Rel,
// never a string prefix — a prefix check would wrongly accept siblings like
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

// fileID extracts (dev, ino) from fi.Sys(). ok is false when the platform
// doesn't expose *syscall.Stat_t — defense in depth: callers treat !ok the
// same as a hard failure, never as a match.
func fileID(fi os.FileInfo) (FileID, bool) {
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return FileID{}, false
	}
	return FileID{Dev: uint64(stat.Dev), Ino: uint64(stat.Ino), Valid: true}, true
}

// captureIdentity returns the file identity of canonicalPath without ever
// opening the final component, so classification — which runs before the
// user has approved anything — cannot trigger driver-specific side effects
// by opening a character or block device the model merely named.
//
// canonicalPath must be absolute and (the caller is expected to have
// resolved it via classifyPath/EvalSymlinks) free of symlinks at the moment
// of canonicalization. The walk re-verifies that property: every
// intermediate directory is opened with O_NOFOLLOW, so a symlink introduced
// after canonicalization surfaces as ELOOP/ENOTDIR and fails the capture —
// exactly the "concurrent swap" case classification must refuse to bless.
// The final component's identity comes from Fstatat with
// AT_SYMLINK_NOFOLLOW relative to its already-verified parent directory
// descriptor — a stat, not an open — and a symlink there is rejected
// explicitly.
//
// Returns FileID{Valid: false} only when the final component does not exist
// (ENOENT); every other error (a missing/denied/non-directory intermediate,
// a swapped final component, an unreadable root) is returned as an error so
// the caller fails classification closed.
func captureIdentity(canonicalPath string) (FileID, error) {
	clean := filepath.Clean(canonicalPath)
	if !filepath.IsAbs(clean) {
		return FileID{}, fmt.Errorf("captureIdentity: path must be absolute: %s", canonicalPath)
	}

	if clean == string(filepath.Separator) {
		var st unix.Stat_t
		if err := unix.Stat(clean, &st); err != nil {
			return FileID{}, fmt.Errorf("stat %q: %w", clean, err)
		}
		return FileID{Dev: uint64(st.Dev), Ino: uint64(st.Ino), Valid: true}, nil
	}

	segments := strings.Split(strings.TrimPrefix(clean, string(filepath.Separator)), string(filepath.Separator))
	parents, final := segments[:len(segments)-1], segments[len(segments)-1]

	const dirFlags = unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC
	dirFd, err := unix.Open(string(filepath.Separator), dirFlags, 0)
	if err != nil {
		return FileID{}, fmt.Errorf("open %q: %w", string(filepath.Separator), err)
	}

	for _, seg := range parents {
		childFd, openErr := unix.Openat(dirFd, seg, dirFlags, 0)
		_ = unix.Close(dirFd) // done with the parent once the child attempt is made
		if openErr != nil {
			return FileID{}, fmt.Errorf("open %q: %w", seg, openErr)
		}
		dirFd = childFd
	}
	defer func() { _ = unix.Close(dirFd) }()

	var st unix.Stat_t
	if statErr := unix.Fstatat(dirFd, final, &st, unix.AT_SYMLINK_NOFOLLOW); statErr != nil {
		if errors.Is(statErr, unix.ENOENT) {
			return FileID{Valid: false}, nil
		}
		return FileID{}, fmt.Errorf("stat %q: %w", final, statErr)
	}
	if st.Mode&unix.S_IFMT == unix.S_IFLNK {
		return FileID{}, fmt.Errorf("refusing identity capture: %q is a symlink", final)
	}
	return FileID{Dev: uint64(st.Dev), Ino: uint64(st.Ino), Valid: true}, nil
}
