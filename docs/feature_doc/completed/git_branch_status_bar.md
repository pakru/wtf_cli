# Show Current Git Branch in Status Bar

**GitHub Issue**: [#6](https://github.com/pakru/wtf_cli/issues/6)

Display the current git branch with `⎇` (U+2387) symbol in the status bar:

```
[wtf_cli] ~/projects/myrepo ⎇ main       [llm]: model-1 | Press / for commands
```

## Proposed Changes

### New Dependency

```bash
go get github.com/go-git/go-git/v5@v5.16.5
```

---

### Git Branch Resolver

#### [NEW] [git.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/components/statusbar/git.go)

`ResolveGitBranch(dir string) string` — uses `go-git` with `PlainOpenWithOptions` and `DetectDotGit: true` for nested dirs/worktrees. Returns:
- Branch name if on a branch (e.g. `main`, `feature/foo`)
- Short SHA (7 chars) if detached HEAD
- `""` if not in a git repo or on error

---

### Status Bar View

#### [MODIFY] [statusbar_view.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/components/statusbar/statusbar_view.go)

- Add `gitBranch string` field and `SetGitBranch(branch string)` method
- Update `Render()` — branch appended **after** `truncatePath()`:
  ```
  branchSuffix := " ⎇ " + branch
  branchWidth := ansi.StringWidth(branchSuffix)
  truncated := truncatePath(leftText, bodyWidth - branchWidth)
  leftContent = prefix + " " + truncated + branchSuffix
  ```
- **Narrow-width**: branch suffix is **all-or-nothing** — if `bodyWidth` can't fit both the truncated path and the full branch suffix, the entire branch suffix is omitted (never partially cut, so `⎇` text is never mangled). All sizing uses `ansi.StringWidth`

> [!NOTE]
> Only `StatusBarView` is modified — `StatusBar` (ANSI version) is not used at runtime.

---

### Model Wiring (Async, Change-Gated)

#### [MODIFY] [model.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/model.go)

- Add fields: `gitBranch string`, `lastResolvedDir string`
- New message: `gitBranchMsg struct{ dir, branch string }` — includes `dir` to guard against stale results
- New command: `resolveGitBranchCmd(dir string) tea.Cmd` — a standard `tea.Cmd` (already async in Bubble Tea, no extra goroutine needed)
- **Change-gated, dispatch-time guard**: in `directoryUpdateMsg`, only dispatch `resolveGitBranchCmd` when `m.currentDir != m.lastResolvedDir`. Set `m.lastResolvedDir = m.currentDir` **at dispatch time** (not on arrival) to prevent redundant resolves while one is in flight
- **Stale guard**: in `gitBranchMsg` handler, only apply when `msg.dir == m.currentDir`
- **Initial resolve**: dispatch `resolveGitBranchCmd` in `Init()` for immediate branch display on startup
- In `renderCanvas()`: call `m.statusBar.SetGitBranch(m.gitBranch)`

---

### Tests

#### [NEW] [git_test.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/components/statusbar/git_test.go)

All tests use **`go-git` programmatic init** for hermetic CI:
- Repo with branch → correct name
- Detached HEAD → short SHA returned
- Non-git dir → `""`
- Nested subdir → correct branch via `DetectDotGit`

#### [MODIFY] [statusbar_view_test.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/components/statusbar/statusbar_view_test.go)

- `⎇ branch` appears after path
- Branch hidden when message active
- Branch fully dropped (not partially cut) on narrow widths

#### [MODIFY] [model_test.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/model_test.go)

- `directoryUpdateMsg` dispatches resolve only on dir change
- `gitBranchMsg` applied when `msg.dir == m.currentDir`
- Stale `gitBranchMsg` (dir mismatch) ignored

## Verification

```bash
cd /home/dev/project/wtf_cli/wtf_cli && make check
```

### Manual
1. Git repo → `⎇ main` visible
2. Non-git dir → branch disappears
3. `git checkout -b feature/test` → updates within ~1s, slash renders correctly
4. Narrow terminal → branch drops entirely, never partially cut
