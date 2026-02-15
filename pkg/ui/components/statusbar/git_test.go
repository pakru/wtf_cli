package statusbar

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func initTestRepo(t *testing.T) (string, *git.Repository, plumbing.Hash) {
	t.Helper()

	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit() failed: %v", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree() failed: %v", err)
	}

	filePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	if _, err := worktree.Add("README.md"); err != nil {
		t.Fatalf("Add() failed: %v", err)
	}

	hash, err := worktree.Commit("init", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}

	return dir, repo, hash
}

func TestResolveGitBranch_BranchRepo(t *testing.T) {
	dir, repo, _ := initTestRepo(t)

	head, err := repo.Head()
	if err != nil {
		t.Fatalf("Head() failed: %v", err)
	}

	got := ResolveGitBranch(dir)
	want := head.Name().Short()
	if got != want {
		t.Fatalf("ResolveGitBranch() = %q, want %q", got, want)
	}
}

func TestResolveGitBranch_DetachedHead(t *testing.T) {
	dir, repo, hash := initTestRepo(t)

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, hash)); err != nil {
		t.Fatalf("SetReference() failed: %v", err)
	}

	got := ResolveGitBranch(dir)
	want := hash.String()[:7]
	if got != want {
		t.Fatalf("ResolveGitBranch() = %q, want %q", got, want)
	}
}

func TestResolveGitBranch_NonRepo(t *testing.T) {
	if got := ResolveGitBranch(t.TempDir()); got != "" {
		t.Fatalf("ResolveGitBranch() = %q, want empty", got)
	}
}

func TestResolveGitBranch_NestedSubdir(t *testing.T) {
	dir, repo, _ := initTestRepo(t)

	head, err := repo.Head()
	if err != nil {
		t.Fatalf("Head() failed: %v", err)
	}

	nested := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	got := ResolveGitBranch(nested)
	want := head.Name().Short()
	if got != want {
		t.Fatalf("ResolveGitBranch() = %q, want %q", got, want)
	}
}
