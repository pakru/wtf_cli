# New Agent Tool: `list_directory` (Issue #81)

**Issue:** [pakru/wtf_cli#81](https://github.com/pakru/wtf_cli/issues/81) — *Add new tool - listDirectory*

> Create new tool for ai assistant to list content of requested directory.
> Tool should provide content of dir like `ls -l`, with list of files, dirs, file size (if applicable), owner:group data, chmod data.

*Revision 3 — incorporates two rounds of cross-review feedback (Codex, 2026-07-02): race-free `os.Root` containment, bounded enumeration + output byte cap, explicit `ls`-style mode formatter, filename escaping, soft-error display fix in the agent loop, generic tool-instruction prompt, independent per-tool config defaulting, expanded test matrix; round 2: `filepath.Rel`-based path normalization (sibling-prefix bug), documented `os.Root` absolute-symlink limitation, whole-result byte-cap contract, directory close/read-error handling, testable enumeration knobs.*

*Post-implementation review, round 1 (Codex, 2026-07-02) found and fixed six further issues in the actual code: (1) error-path results bypassed the `MaxBytes` invariant entirely — added a per-tool `errResult` wrapper; (2) `strings.TrimSpace` on the model-supplied path corrupted real whitespace-padded directory names — now only the literal empty string defaults to `.`; (3) `classifyDirOpenError`'s catch-all mislabeled unrelated errors (e.g. `ENOTDIR` from traversing through a file) as containment violations — narrowed to a `"not a directory"` case plus an honest generic fallback; (4) per-entry `Lstat`/`Readlink` re-resolved `rel` from the outer root by path string, reopening the exact TOCTOU gap `os.Root` was chosen to close — fixed by anchoring a sub-root via `root.OpenRoot(rel)` once and addressing entries by bare name against that stable handle (this also subsumes the separate `Stat`+`IsDir` check); (5) the scan-cap boundary probe silently discarded a real error when it also found a further entry, and an all-hidden scan-cap directory was misreported as `(empty)` — both now preserved; (6) `listDirectoryMinMaxBytes` (256) was implementation-tested to equal `listDirectoryFooterReserve` (256), so the documented minimum config value produced zero usable rows ever — raised to 512.*

*Post-implementation review, round 2 (Codex, 2026-07-02) found three more: (1) the round-1 `errResult` wrapper fixed the byte-cap invariant's *bound*, but a sufficiently long real (not adversarial) path could still consume the whole budget on its own, leaving the final safety clip to slice through the header/footer structure rather than the structure surviving intact — fixed with a `boundedDisplay` helper that caps and ellipsizes any path/error text embedded in a message *before* assembly, applied to the header, the empty-directory body, and every `classifyDirOpenError` branch; (2) round 1's `"not a directory"` classification substring-matched the *fully formatted* error message, which interpolates the model-controlled path — a directory literally named to contain that phrase could spoof an unrelated failure (e.g. `ENAMETOOLONG`) into the wrong diagnosis — fixed with `errors.Is(err, syscall.ENOTDIR)` plus a structural `isNotADirectorySentinel` check that inspects only the innermost wrapped error, never the path-bearing outer text; (3) two early `Execute` exits (bad JSON, unconfigured cwd) still called the raw package `errResult` instead of `t.errResult`, leaving the "all exits respect MaxBytes" invariant technically false — both switched over. All nine findings across both rounds have regression tests; full suite and `make check` pass.*

## Overview

The agentic loop (issue #58) currently ships a single tool, `read_file`. The model can read a file it already knows about, but it cannot *discover* files — e.g. when `/explain` sees `config not found` it has no way to check what actually exists in the directory. This feature adds a second tool, `list_directory`, that returns an `ls -l`-style listing of one directory level, restricted to the user's current working directory subtree.

**Goal:** ship `list_directory` end-to-end — tool implementation, config plumbing, registration — reusing the existing registry, approval popup, tool-call UI, and agent loop. Two small adjacent fixes ride along: the agent loop's soft-error display bug and the `read_file`-specific tool-instruction prompt.

## Design Decisions

- **Tool name:** `list_directory` (snake_case, consistent with `read_file`). The issue title says `listDirectory`; the codebase convention for tool names is snake_case.
- **Non-recursive:** lists one directory level. The model can call the tool again on a subdirectory (each call goes through the normal approval flow / session policy).
- **CWD-contained via `os.Root`:** the shell's cwd is snapshotted at agent-loop start (same value `read_file` receives). All filesystem operations go through `os.OpenRoot(cwd)` (Go 1.26; project requires 1.26+), which enforces containment via descriptor-relative traversal (`openat` + `O_NOFOLLOW` per component on Unix) and is immune to the check-then-use race inherent in the `EvalSymlinks`-then-operate pattern used by `read_file`'s `resolveContainedPath`. Any traversal that would escape the root fails, which we surface as "path outside working directory". Migrating `read_file` to `os.Root` is a recommended follow-up, out of scope here.
  - **Path normalization** (lexical, for clearer error messages only — `os.Root` remains the enforcement boundary): relative paths are `filepath.Clean`ed and rejected if the result is `..` or starts with `../`. Absolute paths are converted with `filepath.Rel(cwd, path)` and rejected on error or a `..`-leading result — **never** a string-prefix comparison, which would wrongly accept siblings like `/tmp/project-other` under cwd `/tmp/project` (regression test required).
  - **Known `os.Root` limitation, documented and tested:** symlinks with *relative* targets that stay inside the root resolve during traversal; symlinks with *absolute* targets are rejected by `os.Root` even when the target points back inside the root. Such paths fail with "path outside working directory". This is acceptable: the model can list the link's parent, see the target, and address it by its in-tree path.
- **Symlink entries are shown, not followed:** per-entry metadata comes from `root.Lstat`, targets from `root.Readlink`, displayed as `lrwxrwxrwx … name -> target`. Targets are displayed as stored (they may point outside cwd — that string is already readable to the user via `ls`); they are never dereferenced.
- **Hidden files:** excluded by default (matches `ls -l`), opt-in via `include_hidden` arg. Dotfiles are often exactly what the model needs (`.env`, `.gitignore`), so the schema description tells the model the flag exists.
- **Explicit mode formatter:** Go's `FileMode.String()` is *not* `ls`-compatible (symlinks render as `L…`, setuid/setgid/sticky as prefix letters `u`/`g`/`t` instead of `s`/`t` in the execute columns). Implement a `formatMode(fs.FileMode) string` helper producing the conventional 10-character Unix string: type char (`-dlbcps`), then `rwx` triplets with `s`/`S`/`t`/`T` substitution for setuid/setgid/sticky.
- **Deterministic, bounded output:**
  - Entries sorted alphabetically by name (byte order). `os.File.ReadDir(n)` returns directory order, so we sort after collection.
  - Enumeration is batched (`f.ReadDir(batch)` in a loop, `ctx.Err()` checked per batch *and* per rendered row — owner/group lookups happen during rendering and may block on NSS) and capped at a scan limit (default 10000 entries) so a pathologically large directory cannot consume unbounded time/memory. Hitting the scan cap is reported in the footer. Scan cap and batch size are unexported fields on the tool struct (defaulted in the constructor) so tests can lower them.
  - `ReadDir(n)` can return partial entries *plus* an error; a mid-enumeration read error does not discard what was collected — the listing is rendered from the partial set with an error footer `[read error after N entries: …]`. If nothing was collected, the error is returned as `Result{IsError: true}`.
  - **The byte cap covers the complete `Result.Content`** — header, rows, and footers included, `len(Content) <= MaxBytes` always. Rows are emitted until the next row plus reserved footer space would exceed the budget. `max_entries` (config, default 500) and `max_bytes` (config, default 65536 — same default as `read_file`) both apply; whichever bites first triggers the truncation footer. (This is a stricter contract than `read_file`, whose header sits outside its cap — deliberate, not a bug to replicate.)
  - Sizes in raw bytes (unambiguous for the model). Timestamps in the fixed layout `2006-01-02 15:04`, **local time** (matches what the user sees from `ls -l`; tests format expectations from the same `ModTime()` so they are timezone-independent).
- **Filename escaping:** POSIX filenames may contain newlines, tabs, and terminal control sequences, which would break the one-entry-per-line contract and could inject junk into model context and logs. Names and symlink targets containing bytes < 0x20, 0x7F, or invalid UTF-8 are rendered in Go-quoted form (`strconv.Quote`); ordinary names pass through unchanged. The same escaping applies to the model-supplied path echoed in the header and in error messages.
- **Owner/group is best-effort:** resolved via `syscall.Stat_t` (`info.Sys()`) + `os/user.LookupId` / `LookupGroupId` with a per-call cache (NSS lookups can be slow); falls back to numeric `uid:gid` when name lookup fails, and to `?:?` when `info.Sys()` is not a `*syscall.Stat_t`. The project targets Linux and macOS only; `syscall.Stat_t` exists on both, so no build tags are needed.
- **Per-entry stat errors don't fail the listing:** an entry whose `Lstat` fails (e.g. racing deletion) is rendered with `?` placeholder fields instead of aborting.
- **Recoverable errors** (bad args, missing dir, not a directory, permission denied, path rejected) return `Result{IsError: true}` so the model can retry; only context cancellation propagates as a Go error — same contract as `read_file`.

## Tool Specification

### JSON schema (model-facing)

```json
{
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
}
```

No required properties — a bare `{}` call lists the cwd.

### Description (model-facing)

> List the contents of a directory inside the user's current working directory, like `ls -l`. Returns one entry per line: permissions, owner:group, size in bytes (`-` for directories), modification time, and name. Directories end with `/`; symlinks show their target and are not followed. Use this to discover files before reading them with read_file. The path must be inside the working directory.

### Output format

```
. (4 entries)
drwxr-xr-x  pavel:pavel         -  2026-06-28 09:15  docs/
-rw-r--r--  pavel:pavel      1234  2026-07-01 18:02  go.mod
lrwxrwxrwx  pavel:pavel        11  2026-05-10 11:40  latest -> version.txt
-rwxr-xr-x  pavel:pavel   8388608  2026-07-01 18:03  wtf_cli
```

- Header mirrors `read_file`'s: the path as the model supplied it (bounded and escaped for display — see implementation note below), plus the total count of entries *discovered* (not the count rendered — deliberate, avoids a circular dependency between the header's byte length and how many rows fit in the remaining budget; the truncation footer separately reports the precise shown/total split whenever they differ).
- Size column: byte count for regular files and symlinks; `-` for directories and other non-regular entries.
- Columns aligned with padded widths; exact widths are an implementation detail, tests assert on fields not spacing.
- Footers, as applicable (combinable):
  - `[truncated: showing 500 of 1372 entries]` (entry or byte cap hit; when the byte cap bites first the shown count is whatever fit)
  - `[directory has more than 10000 entries; listing is based on the first 10000 scanned]` (scan cap hit — total is unknown, sorting covers only scanned entries)
  - `[read error after N entries: …]` (mid-enumeration `ReadDir` failure; partial listing still returned)
- The whole result — header, rows, footers — never exceeds `max_bytes`.
- Empty directory (or everything filtered as hidden): header plus `(empty)`.

## Implementation Plan

> **Note:** the step-by-step description below is the pre-implementation design; the post-implementation review (see the note above) changed several details — most notably, containment now anchors a sub-root via `root.OpenRoot(rel)` once and addresses entries by bare name against that handle (closing a TOCTOU gap and subsuming the separate `Stat`+`IsDir` check), and every `Execute` error exit routes through a per-tool `t.errResult` that also bounds embedded path/error text length. [pkg/ai/tools/list_directory.go](../../pkg/ai/tools/list_directory.go) is the source of truth; treat the steps below as historical context for *why* the shape is what it is, not a literal spec of the final code.

### 1. Tool implementation — new file `pkg/ai/tools/list_directory.go`

Follow the structure of [read_file.go](../../pkg/ai/tools/read_file.go):

- Constants: `listDirectoryName = "list_directory"`, description, schema (`json.RawMessage`), `listDirectoryMinMaxEntries = 1`, `listDirectoryMinMaxBytes = 512` (must stay well above the footer-reserve constant below, or the reserve alone would starve every row at the floor), `listDirectoryDefaultScanCap = 10000`, `listDirectoryDefaultReadBatch = 512`.
- `ListDirectoryArgs struct { Path string; IncludeHidden bool }` with json tags `path`, `include_hidden`.
- `ListDirectory struct { Cwd string; MaxEntries int; MaxBytes int; scanCap int; readBatch int }` + `NewListDirectory(cwd string, maxEntries, maxBytes int)` normalizing non-positive caps to the package minimums (same pattern as `NewReadFile`) and setting `scanCap`/`readBatch` to the defaults. The two unexported fields exist so tests (same package) can lower them.
- `Execute`:
  1. `ctx.Err()` check (hard error path).
  2. Decode args; empty/missing `path` defaults to `"."`.
  3. Guard unconfigured `Cwd` (same message pattern as `read_file`).
  4. Normalize the path to root-relative: `filepath.Clean` for relative input, `filepath.Rel(cwd, path)` for absolute input; reject `..`-leading results with "path outside working directory" (see Design Decisions — no string-prefix comparison).
  5. `root, err := os.OpenRoot(t.Cwd)`; `defer root.Close()`. Failure → recoverable error.
  6. `f, err := root.Open(rel)`; `defer f.Close()`. Map `ErrNotExist` → "directory not found", `ErrPermission` → "permission denied", escape errors → "path outside working directory". `f.Stat()` not a dir → "path is a file, not a directory: %s (use read_file for files)".
  7. Enumerate via helper `collectEntries(ctx, f, scanCap, readBatch, includeHidden)`: loop `f.ReadDir(readBatch)`, checking `ctx.Err()` (hard error) each batch, collecting names, stopping at `scanCap`. Hidden names are filtered during collection unless `include_hidden` (filtered entries do not count toward `max_entries` but do count toward the scan cap). A non-EOF `ReadDir` error stops enumeration but keeps the partial batch it returned; the error is carried to step 9's footer (or returned as `Result{IsError: true}` if nothing was collected).
  8. Sort collected names; render rows up to `MaxEntries` and within the whole-result byte budget (header + rows + reserved footer space `<= MaxBytes`), checking `ctx.Err()` per row. Per entry: `root.Lstat(filepath.Join(rel, name))`; on error render `?????????? ?:? ? ? name`. Otherwise `formatMode`, owner:group, size, mod time, escaped name with `/` suffix for dirs and ` -> target` (via `root.Readlink`, escaped; readlink failure falls back to plain name). Row assembly lives in a helper `renderEntry(name string, info os.FileInfo, linkTarget string, statErr error, …) string` so the fallback branches are unit-testable without filesystem races.
  9. Assemble header (escaped path) + rows + applicable footers (entry/byte truncation, scan cap, read error); return `Result{Content: …}` with `len(Content) <= MaxBytes`.
- Helpers in the same file: `formatMode(fs.FileMode) string`, `escapeName(string) string`, `ownerGroup(info os.FileInfo, cache map[[2]uint32]string) string`, `collectEntries(…)`, `renderEntry(…)`.
- Implementation notes (from cross-review sign-off): `collectEntries` takes a minimal `interface { ReadDir(int) ([]os.DirEntry, error) }` instead of `*os.File` so partial-batch-plus-error behavior is deterministically testable with a fake; degenerate budgets where header/footer text alone exceeds `MaxBytes` clip that text rather than overshoot — `len(Content) <= MaxBytes` holds unconditionally.

Update the package comment in [registry.go](../../pkg/ai/tools/registry.go) to mention both tools.

### 2. Agent-loop soft-error display fix — `pkg/commands/agent_loop.go`

Pre-existing bug, adjacent and load-bearing for this tool: `executeOneTool` never sets `finished.ErrorMessage` when a tool returns `Result{IsError: true}` ([agent_loop.go:319](../../pkg/commands/agent_loop.go)), so soft failures like "path rejected" render in the sidebar as `— 1 lines` instead of `— error: …`. Fix:

```go
finished.Result = result.Content
if result.IsError {
    finished.ErrorMessage = result.Content
}
```

(`formatToolCallSuffix` in [pkg/ui/stream.go](../../pkg/ui/stream.go) already prioritizes `ErrorMessage` over `Result`.) Add a regression test in `agent_loop_test.go` asserting `ToolCallFinished.ErrorMessage` is populated for a soft-erroring tool.

### 3. Tool-instruction prompt — `pkg/ai/context.go`

`AppendToolInstructions` ends with the `read_file`-specific sentence "Read narrow ranges (a few hundred lines max) per call." ([context.go:162](../../pkg/ai/context.go)). Generalize to: *"Keep each call narrow: a few hundred lines of a file, or one directory level, per call."* There is no test covering this function today — add one asserting the guidance mentions both bounded file reads and one-level directory listings (and that an empty tool list leaves the prompt unchanged).

### 4. Config plumbing — `pkg/config/config.go`

Mirror `ReadFileToolConfig`:

- Add `ListDirectoryToolConfig struct { Enabled bool; MaxEntries int; MaxBytes int }` (json: `enabled`, `max_entries`, `max_bytes`).
- Add `ListDirectory ListDirectoryToolConfig` field to `AgentTools` (json: `list_directory`).
- Add `defaultListDirectoryMaxEntries = 500`, `defaultListDirectoryMaxBytes = 65536`; default `{Enabled: true, MaxEntries: 500, MaxBytes: 65536}` in `Default()`.
- Extend the **two** anonymous presence-struct declarations (they must stay textually identical or the `applyAgentDefaults` parameter type won't match): the `Agent.Tools` block in `configPresence` (~line 374) and the parameter type of `applyAgentDefaults` (~line 504). If the duplication grows irritating, name the type — optional.
- **Restructure `applyAgentDefaults` so each tool defaults independently.** The current code early-returns when `presence.Tools == nil || presence.Tools.ReadFile == nil` ([config.go:520](../../pkg/config/config.go)) — extended naively, a config specifying only `list_directory` would leave `read_file` fields zeroed or vice versa. New shape: if `presence.Tools == nil`, set both tools from defaults and return; otherwise apply per-tool blocks (`ReadFile == nil` → whole-struct default; else per-field fallback for nil/non-positive) for each tool separately.

### 5. Registration — `pkg/commands/handlers.go`

In `buildToolRegistry` ([handlers.go:233](../../pkg/commands/handlers.go)) add:

```go
if cfg.Agent.Tools.ListDirectory.Enabled {
    registry.Register(tools.NewListDirectory(
        cwd,
        cfg.Agent.Tools.ListDirectory.MaxEntries,
        cfg.Agent.Tools.ListDirectory.MaxBytes,
    ))
}
```

Generalize the `read_file`-specific wording in the function's doc comment ("read_file enforces cwd containment…" → "tools enforce cwd containment…"). Everything downstream is tool-agnostic: the agent loop, `UIApprover` (keyed by tool name — `list_directory` gets its own "allow always this session" entry), and the sidebar display. Note the sidebar shows only the call line and outcome (`list_directory({"path":"docs"}) — 42 lines` / `— error: …`), never the listing body; only the model sees the content.

### 6. Tests

**New `pkg/ai/tools/list_directory_test.go`** (table-driven, `t.TempDir()`, mirroring `read_file_test.go`):

- Definition: correct name, description non-empty, schema parses as JSON.
- Constructor: non-positive caps normalized to minimums.
- Basic listing: files + subdir, correct fields (mode string, size for files, `-` and `/` suffix for dirs), sorted order.
- Default path (`{}` args) lists cwd; unconfigured `Cwd` → `IsError`.
- Hidden entries excluded by default; included with `include_hidden: true`; a directory containing only dotfiles renders `(empty)` without the flag.
- Path escapes rejected: `../outside`, absolute path outside cwd, path traversing a symlinked dir that points outside cwd → `IsError`. **Sibling-prefix regression:** with cwd `<tmp>/project`, absolute path `<tmp>/project-other/…` is rejected.
- `os.Root` symlink semantics: relative in-tree symlinked dir traverses successfully; symlink with an *absolute* target pointing back inside cwd is rejected (documented limitation).
- Symlink *entry* is listed with `lrwxrwxrwx` and `-> target`, not followed; broken symlink (missing target) still renders with its target text.
- `renderEntry` unit tests: stat-error fallback row (`?` fields), readlink-failure fallback (plain name, no ` -> `), normal file/dir/symlink rows — exercised directly with injected values, no filesystem race needed.
- `collectEntries` unit tests with lowered `scanCap`/`readBatch`: scan cap stops enumeration and flags the footer; context cancelled between batches returns a hard error; partial-batch-plus-error keeps collected entries.
- `formatMode` unit tests: regular, dir, symlink, setuid, setgid, sticky (including `S`/`T` no-execute variants), fifo.
- `escapeName` unit tests: plain name unchanged; newline/tab/ESC/invalid-UTF-8 quoted.
- Missing directory → "directory not found"; path is a file → "not a directory"; unreadable directory (chmod 000, skipped when running as root) → "permission denied".
- Empty directory → header + `(empty)`.
- Truncation: `max_entries` cap with footer counts; `max_bytes` cap with tiny byte budget; **byte-cap contract:** `len(Result.Content) <= MaxBytes` asserted on every truncation case, including a long control-character-laden path echoed in the header.
- Owner:group: entry created by the test shows either the current user/group names *or* the numeric `uid:gid` fallback (don't require NSS to resolve — CI containers may not).
- Invalid JSON args → `IsError`; cancelled context → Go error, not `Result`.

**New/extended tests elsewhere:**

- `pkg/config/config_test.go`: defaults include `list_directory` (enabled, 500, 65536); presence matrix — `agent` absent, `tools` absent, only `read_file` present (list_directory gets full defaults), only `list_directory` present (read_file gets full defaults), partial fields, explicit `"enabled": false`, non-positive caps replaced by defaults.
- `pkg/commands`: **new** direct tests for `buildToolRegistry` (it has no coverage today) — both tools registered by default, each independently disableable, cap values propagate.
- `pkg/commands/agent_loop_test.go`: soft-error `Result{IsError: true}` populates `ToolCallFinished.ErrorMessage` (regression for §2).

### 7. Documentation

- [AGENTS.md](../../AGENTS.md): update `pkg/ai/tools/` line to `read_file, list_directory, registry`; the config example currently has no `agent` block at all — add a complete one:

```json
"agent": {
  "max_iterations": 5,
  "tools": {
    "read_file":      { "enabled": true, "max_lines": 500, "max_bytes": 65536 },
    "list_directory": { "enabled": true, "max_entries": 500, "max_bytes": 65536 }
  }
}
```

- Move this doc to `docs/feature_doc/completed/` when merged.

## Verification

1. `make check` (fmt, vet, build, full test suite).
2. Manual: run `wtf_cli`, trigger `/chat`, ask *"what files are in this directory?"* — expect an approval popup for `list_directory`; after approval the sidebar shows `list_directory({"path":"."}) — N lines` and the model's answer describes the directory contents. Approve "always", ask about a subdirectory, expect no second popup.
3. Manual negative: ask the model to list `/etc` (outside cwd) — the sidebar shows `— error: path outside working directory…` (exercises the §2 fix) and the model explains it can only look inside the working directory.
4. Disable via config (`"list_directory": {"enabled": false}`), restart, confirm the tool is not advertised (model answers without tool calls).

## Out of Scope

- Recursive listing / glob matching (a future `find`-like tool if needed).
- Pagination via offset args (caps + footers are enough at this size).
- Windows support (project targets Linux/macOS; `syscall.Stat_t` is available on both).
- Migrating `read_file` to `os.Root` containment (recommended follow-up — its `resolveContainedPath` has the same TOCTOU shape this plan avoids).
- Any change to approval UX, providers, or the agent-loop control flow beyond the §2 one-liner.
