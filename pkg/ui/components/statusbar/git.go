package statusbar

import (
	"strings"

	git "github.com/go-git/go-git/v5"
)

// ResolveGitBranch returns the branch name (or short SHA for detached HEAD)
// for the git repository containing dir. It returns "" when no repo is found.
func ResolveGitBranch(dir string) string {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" {
		return ""
	}

	repo, err := git.PlainOpenWithOptions(trimmed, &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return ""
	}

	head, err := repo.Head()
	if err != nil {
		return ""
	}

	if head.Name().IsBranch() {
		return head.Name().Short()
	}

	hash := head.Hash().String()
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
