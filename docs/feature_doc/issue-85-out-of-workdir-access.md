# Out-of-Workdir Tool Access with User Permission (Issue #85)

**Issue:** [pakru/wtf_cli#85](https://github.com/pakru/wtf_cli/issues/85) — *Add support for tools to work out of work dir with user permission*

> read_file and list_dir tools are currently not able to read files out of work dir path due to security concerns. We need to add support to if tools want to use out of user dir path - ask user for permission to do so

*Revision 2 — incorporates cross-review feedback (Codex gpt-5.5, 2026-07-04): (1) authorization binds to **object identity** (dev/inode verified on the opened handle), not path-string comparison — string matching alone left a symlink-swap race between re-resolution and open; the in-workdir `read_file` open migrates to `os.Root` in the same change because its existing race has the same shape; (2) path grants are keyed **per tool**, not shared — a `list_directory` grant for `$HOME` must not let `read_file` read `~/.ssh/id_rsa`; (3) `"deny"` mode is now explicitly "today's semantics verbatim" (ordinary approval popup, containment rejection at execute) — the previous draft self-contradictorily promised "no prompting"; (4) the approval popup gets control-character escaping and tail-preserving path display — the current panel prefix-truncates unescaped model-controlled strings; (5) `AllowEscapes` is enforced at the tool boundary and `PathGrants` validates its own invariants; (6) expanded test matrix (identity-swap race, symlink-under-grant, cross-tool isolation, dynamic definitions, loop-level audit logs); dropped the `AppendToolInstructions` edit, dropped `PathGrants.Reset`, and scoped the behavior-change claims around tilde expansion honestly.*

*Revision 3 — cross-review round 2 (Codex gpt-5.5, 2026-07-04) found the revision-2 identity **capture** still racy: `EvalSymlinks` followed by a separate `stat(resolved)` lets a parent-symlink swap capture the identity of an object other than the one the popup displays. Fixed: (1) identity is captured by a **component-wise no-follow `openat` walk** of the canonical path (`captureIdentity` helper), so the captured `(dev, ino)` provably belongs to the literal path shown to the user — while at *execution* a plain non-blocking `O_NOFOLLOW` open plus fstat-equality against the approved identity closes the traversal-route race, because identity equality defeats any redirection *other than* deletion-plus-inode-reuse of the specific approved object (a separate, narrower, accepted residual — see the note below); (2) all opens (both in-workdir and escape branches) are **non-blocking** with the regular-file check on the opened handle, restoring the FIFO/device protection the pre-open `Stat` used to provide; (3) `Target.Valid=false` is produced **only** for final-component `ENOENT` — every other capture failure (permissions, `ELOOP` from a swapped component, unsupported `Stat_t`) fails classification closed; plus deterministic parent-swap tests against the capture helper itself, not just fabricated-ID rejection.*

*Revision 4 — cross-review round 3 (Codex gpt-5.5, 2026-07-04) confirmed the execution-side open+fstat design and left three capture-side items, all fixed: (1) classification must never **open** the target before the user has consented — opening character/block devices can have driver side effects — so `captureIdentity` now stats the final component with `unix.Fstatat(parentFD, name, AT_SYMLINK_NOFOLLOW)` relative to the securely walked parent, explicitly rejecting a symlink mode, instead of opening it; only intermediate directories are ever opened during the walk; (2) `captureIdentity("/")` (no final component) is defined — stat the root path directly — with a `list_directory("/")` test; all `openat` flags include `O_CLOEXEC` and each superseded intermediate descriptor is closed promptly in the loop, not via accumulated defers; (3) dropped the inaccurate go.mod "promotion" step — `golang.org/x/sys` is already a direct dependency.*

*Post-implementation review (Codex gpt-5.5, 2026-07-04, reviewing the actual diff rather than the plan) found six issues in the real code, all fixed: (1) `PathGrants.IsAllowed`'s root special-case (`dir == "/"`) was not mirrored in `UIApprover.recordEscapeGrant`'s containment check, so a persistent grant for a file directly under `/` silently failed to save — fixed by extracting one shared `dirContainsPath` helper both call, so this class of drift can't recur; (2) `Validate()` accepted a whitespace-padded `out_of_workdir_access` value (comparing a trimmed copy) but downstream consumers (`buildToolRegistry`) compared the untrimmed field, so `" ask "` silently behaved as deny — fixed by normalizing (trimming) the value once during defaulting, so every consumer can use plain equality; (3) the escape popup's "Allow for session" scope note embedded the grant directory via ordinary head-truncation of the whole composed sentence, undoing the tail-preserving hardening applied everywhere else — fixed by truncating the directory independently within its own budget before composing the sentence; (4) the `"deny"`-policy tool descriptions never mentioned `~/` expansion, though the design says it applies under both policies — added; (5) the agent loop's audit log omitted `resolved_path`/`grant_dir` on a *denial* (only the allow path built them) — fixed by building the log fields once, before branching on allow/deny; (6) the UI-side `tool_approval_show`/`tool_approval_user_decision` logs (`pkg/ui/stream.go`), planned as supplementary to the loop's authoritative log, were never actually given the escape fields — added. Also fixed: two tests hard-coded a `"../../../"` traversal count assuming a specific `t.TempDir()` nesting depth, which holds on Linux but not on macOS (a supported target) — replaced with a depth-independent traversal; added a `list_directory("/")` execute-level round-trip and a char-device Execute-level rejection test.*

*Residual, accepted limitation raised in the same review, round 2: `FileID` equality is `(dev, ino)` only, which is not a *cross-time* identity if the approved path's object is deleted and a new one is allocated the same (recycled) inode before `Execute` re-verifies it. An initial "this is equivalent to overwriting the file in place, which is already out of scope" defense **does not hold in general** — directory-write and file-write are separate Unix permissions, so an attacker with write access only to a non-sticky parent directory can `unlink`+recreate a file they have no permission to modify directly (e.g. one owned by another user, or read-only), and — if the recycled inode number happens to match — substitute fully attacker-controlled content that passes identity verification. The equivalent risk exists for an empty, otherwise-unwritable directory replaced through a writable parent. This is **not** exploitable to switch device (`Dev` is checked) or substitute a directory for a file (`read_file` separately rejects non-regular targets), and closing it completely requires holding a live, securely-opened file descriptor across the entire UI approval round-trip — already rejected below ("Alternatives considered and rejected") for its goroutine/channel fd-lifetime and leak-on-cancel complexity, for what is a narrow, filesystem-implementation-dependent race requiring precise timing within the human-reaction-time approval window. Accepted and documented, not fixed: a parent-directory writer who does not otherwise have permission to modify a specific file or empty directory under that parent can, with successful inode-reuse timing, have their replacement content pass this feature's identity check.*

## Overview

Both agent tools (`read_file`, `list_directory`) hard-reject any path that resolves outside the shell's working directory: `read_file` via `resolveContainedPath` ([pkg/ai/tools/read_file.go](../../pkg/ai/tools/read_file.go)), `list_directory` via `os.Root` ([pkg/ai/tools/list_directory.go](../../pkg/ai/tools/list_directory.go)). That is the right default, but it makes the agent useless for common diagnostics: reading `/var/log/syslog`, `~/.bashrc`, `/etc/hosts`, or a config referenced by an error message.

**Goal:** when a tool call targets a path outside the working directory, ask the user for explicit permission instead of rejecting outright. Denied or unprompted escapes behave as today.

**Non-goal:** weakening the default. In-workdir approval flow, headless behavior (no UI approver), and the `"deny"` config policy all keep today's *authorization* semantics: nothing outside the workdir is ever readable without an explicit user decision. (Two deliberate, documented behavior deltas ride along even under `"deny"`: leading-`~` paths resolve to the home directory before containment is applied — changing the soft-error text from "file not found" to "path outside working directory" — and `read_file`'s in-workdir open migrates to `os.Root`, which closes its pre-existing symlink race and, like `list_directory` today, rejects in-tree symlinks with absolute targets.)

## Current approval architecture (what we build on)

- Every tool call round-trips through `Approver.Approve` before `Execute` ([pkg/commands/agent_loop.go](../../pkg/commands/agent_loop.go)). The UI implementation (`UIApprover`, [pkg/commands/ui_approver.go](../../pkg/commands/ui_approver.go)) emits a `WtfStreamEvent{ToolApproval:...}`, the Bubble Tea model shows the `toolapproval.Panel` popup (Allow once / Allow for session / Deny), and the decision comes back on a buffered `Reply` channel.
- "Allow for session" is stored in `SessionApprovals`, **keyed by tool name only**. This is the crux: once the user session-allows `read_file`, subsequent calls skip the popup entirely. If we simply relaxed containment inside the tool, a session-allowed tool could read anywhere with **no prompt at all**. Out-of-workdir access therefore needs its own consent dimension, checked independently of the per-tool session grant.

## Design

### Chosen shape: scope classification at approval time, object-identity enforcement at execution time

One popup per call, ever. The existing approval popup becomes *scope-aware*:

1. **Classification (advisory for control flow, load-bearing for what the user sees):** before requesting approval, the agent loop asks the tool — via a new optional interface — whether the call targets a path outside the working directory. The tool canonicalizes the path (`EvalSymlinks`) and captures the target's file identity `(dev, ino)` **through a component-wise no-follow `openat` walk of the canonical path**, so the captured identity provably belongs to the literal string shown in the popup (a plain `stat` after canonicalization would race: a parent swapped to a symlink between the two steps could bind the approval to `/etc/shadow` while the popup displays `/tmp/a/shadow`). The popup shows the *resolved* path — today the user approving `../../../etc/shadow` sees only the raw argument; this fixes that latent gap too.
2. **Approval (policy):** `UIApprover` applies a two-store policy — the existing per-tool-name `SessionApprovals` auto-allows **in-workdir calls only**; a new `PathGrants` store, keyed by **(tool, directory)**, auto-allows out-of-workdir calls whose resolved path falls under a directory the user granted *to that tool* earlier this session. Anything else prompts.
3. **Enforcement (authoritative, in the tool):** `Execute` re-resolves the path at use time, requires the fresh resolution to equal the approved path (fast-fail with a clear message), **opens the target non-blocking, and verifies via `fstat` on the opened handle that the object's `(dev, ino)` equals the identity the approval covered**; all reads/listings go through that verified handle, never a reopened path. At execution a full no-follow walk is unnecessary: identity equality with the approved object defeats symlink/rename-based redirection — any such redirection an attacker arranges either lands on the approved object (harmless: it is approved) or produces a mismatch and a soft error, *"path changed during approval; call the tool again"* (which re-runs classification and re-prompts). It does **not** defeat delete-plus-inode-reuse of the specific approved object by an attacker who already holds write access to its parent directory — see the accepted-limitation note above.

### Alternatives considered and rejected

- **In-tool permission gate** (tool's `Execute` blocks mid-call on a second popup when it hits an outside path): keeps the agent loop untouched, but produces *two* sequential popups for a not-yet-session-allowed tool (generic approval, then escape approval), requires plumbing the UI event channel into tool construction (today the registry is built in `prepareAgentRun` before the channel exists), and puts UI round-trip machinery inside `Execute`. Rejected for UX and layering.
- **Approval scope passed via `context.Context` value:** avoids touching the `Tool` interface, but hides a security-relevant parameter in an implicit side channel. Rejected; the grant is passed explicitly.
- **Session-wide "this tool may read anywhere" grant:** simplest store, but "Allow for session" would silently escalate to whole-filesystem read for the model. Rejected in favor of per-tool directory-subtree grants — the popup states exactly which tool gets which directory.
- **Path-string-only enforcement (exact-match on re-resolution, no identity check):** the first draft. Rejected in cross-review round 1: `EvalSymlinks`-then-`open` leaves a race window in which a component can be swapped after the check.
- **Identity capture via plain `stat` after canonicalization:** the second draft. Rejected in cross-review round 2: the capture itself races the same way, binding the approval to an object the popup never displayed. Capture must traverse the canonical path with per-component `O_NOFOLLOW`.
- **Holding the securely opened handle from classification through approval to execution:** strongest binding, but the handle would have to travel `EscapeRequest → ApprovalRequest → UI popup → ExecGrant` across goroutines, channels, and user-speed think time, with leak-free close obligations on every deny/cancel/error path. The capture-walk + execute-time identity check achieves the same guarantee without fd-lifetime plumbing.

### Approval decision matrix

| Call scope | Tool session-allowed? | Path grant for (this tool, dir) covers resolved path? | Result |
|---|---|---|---|
| in workdir | yes | — | auto-allow (unchanged) |
| in workdir | no | — | popup: Allow once / Allow **tool** for session / Deny (unchanged) |
| outside | (ignored) | yes | auto-allow |
| outside | (ignored) | no | popup variant: Allow once / Allow **directory for this tool** for session / Deny |

Notes:

- A per-tool session grant **never** auto-allows an outside call — that is the entire point of the issue.
- "Allow directory for session" grants only the directory subtree **to the requesting tool**. Directory listing and file-content reading are different capabilities: granting `list_directory` on `$HOME` must not authorize `read_file` on `~/.ssh/id_rsa`. The popup states the resulting scope explicitly, e.g. *"read_file may read any file under /etc for this session"*.
- A path grant does **not** additionally session-allow the tool by name, and a tool-name grant never implies any path grant. The stores stay orthogonal, each recording exactly what its popup label said.
- Grants are **session-only** (in-memory, like `SessionApprovals`). No persistence to config — a durable "always allow /var/log" is out of scope.

### Security model

- **Identity capture provably matches the displayed path — and never opens the target.** `captureIdentity(canonicalPath)` walks the canonical path's *intermediate directories* component-by-component with `openat(..., O_RDONLY|O_DIRECTORY|O_NOFOLLOW|O_CLOEXEC)`, closing each superseded descriptor promptly inside the loop (no accumulated defers), then captures the final component with `unix.Fstatat(parentFD, name, AT_SYMLINK_NOFOLLOW)` — a stat relative to the securely held parent, **not an open**: classification runs *before* the user has consented, and opening a character/block device can itself have driver side effects (current `read_file` deliberately never opens non-regular objects; that property is preserved). A symlink mode from `Fstatat` is rejected explicitly. The canonical path contains no symlinks at canonicalization time, so any symlink encountered during the walk (`ELOOP`/`ENOTDIR`) or at the final component means a concurrent swap — the walk fails and **classification fails closed** (no escape offered; `Execute`'s containment produces the ordinary rejection). `captureIdentity("/")` has no final component and is defined as a direct `stat` of the root path itself (there is no parent component that could have been swapped, so a plain path-based stat is as safe here as the walk is for everything below it). Final-component `ENOENT` is the *only* failure mapped to `Target.Valid=false` (the legitimate "prompt for a missing file" case); permission errors, unsupported `Stat_t`, and every other failure also fail closed. Built on `golang.org/x/sys/unix` (`Openat`/`Fstatat`/`Fstat`), already a direct dependency (v0.46.0); Linux and macOS both support the required flags.
- **Execution verifies the opened object, and only ever reads through the verified handle.**
  - *Files:* `os.OpenFile(resolved, O_RDONLY|O_NOFOLLOW|O_NONBLOCK)`, then `f.Stat()` on the handle: require a regular file **and** `(dev, ino)` equal to `grant.Target`; then stream from that handle (`readLineRange` is refactored to accept an open `*os.File`). No walk needed here — see design point 3. (Hard links can alias an approved identity under a different name, but creating one requires rights on the *target* under default `fs.protected_hardlinks`, and the read still happens with the user's own privileges — outside the threat model, noted for completeness.)
  - *Directories:* `os.OpenRoot(resolved)`, then `fstat` the anchored handle (`root.Open(".")` → `Stat`) and require identity equality with `grant.Target` before listing `"."`. `OpenRoot`'s initial path may follow symlinks, but a redirected anchor cannot *forge* the approved directory's `(dev, ino)` via a symlink: directories cannot be hard-linked, and bind mounts require root. (A parent-directory writer replacing the approved, empty directory itself via delete-plus-inode-reuse is the same accepted residual as the file case above, not a new gap.) The existing containment machinery (symlink entries, caps, escaping) then operates from the verified anchor, unchanged.
  - *Missing-at-approval targets* (`Target.Valid=false`): the execution open must fail `ENOENT` → "file not found". Anything that *appears* there post-approval was never approved → "path changed during approval".
- **Non-blocking opens; type check on the handle.** Current `read_file` deliberately `Stat`s before opening to avoid blocking on FIFOs/devices; moving the type check onto the opened handle must not lose that. Every open in both tools' file paths — the in-workdir `root.OpenFile(rel, O_RDONLY|O_NOFOLLOW|O_NONBLOCK, 0)` and the escape-branch `os.OpenFile` — passes `O_NONBLOCK`, so a writer-less FIFO opens immediately instead of hanging; `f.Stat()` then rejects anything non-regular with today's error wording. (Regular files ignore `O_NONBLOCK` for reads on both target platforms.) FIFO and device regression tests assert prompt return.
- **`read_file`'s in-workdir open migrates to `os.Root` in this change.** Its current `EvalSymlinks`-then-`open` pattern can *already* escape the workdir without consent, so it cannot stay a deferred follow-up once we are hardening this exact boundary. `root.OpenFile(rel, ...)` gives descriptor-relative, race-free containment. Known, documented `os.Root` delta (already accepted for `list_directory`): in-tree symlinks with absolute targets are rejected even when the target is inside the workdir.
- **The popup shows the resolved path, control-escaped and tail-preserving.** If `../logs` is a symlink to `/var/log`, the user approves `/var/log`, not the innocuous-looking relative path; requested and resolved paths are both shown when they differ. All model-controlled strings rendered by the panel (requested path, resolved path, grant dir) are escaped for control characters / invalid UTF-8 (same policy as `list_directory`'s `escapeName`) so a hostile path cannot inject terminal sequences or fake popup lines; long paths wrap or truncate *preserving the tail* (the distinguishing suffix), never prefix-only — `/safe/very/long/.../secret` must not display as `/safe/very/long/…`. This hardening applies to the existing in-workdir popup too, which currently renders raw model strings.
- **Per-tool directory grants, boundary-aware matching on resolved paths.** `PathGrants` is keyed `(tool, dir)`; membership is a path-separator-boundary prefix check — a grant for `/var/log` covers `/var/log` and `/var/log/nginx/error.log`, never `/var/log2` (root dir `"/"` handled explicitly). Both sides are fully resolved paths, and — critically — the *fresh, per-call* resolution is what gets matched, so a granted-dir path whose symlink now resolves elsewhere falls outside the grant and re-prompts.
- **Defense in depth at the tool boundary.** `Execute` honors an escape grant only when the tool itself was constructed with `AllowEscapes` (config `"ask"`); a grant handed to a `"deny"`-configured tool is ignored and containment applies. `PathGrants.Allow` rejects non-absolute or non-clean directories; `UIApprover` verifies `GrantDir` actually contains `ResolvedPath` before persisting (mismatch → allow-once only, log a warning). "Callers construct it correctly" is not a sufficient invariant for a security store.
- **Headless flows keep today's containment.** `ApprovalDecision` gains an explicit `AllowOutsideWorkdir` field; the loop only passes an escape grant to `Execute` when it is set. `AutoAllowApprover` (tests, headless fallback) does not set it, so without a UI there is no path by which an escape can be silently approved. Tests that want escapes build a custom approver.
- **Classification failures never grant anything.** If `ClassifyCall` returns nil (unparseable args, unresolvable path, capture-walk failure, escapes disabled), the loop treats the call as in-workdir-scoped: normal approval, and `Execute`'s unconditional containment produces the proper soft error. Classification is best-effort for control flow; `Execute` is the error authority.
- **No sensitive-path denylist** (`/etc/shadow`, `~/.ssh`, …): the user operating their own terminal is the authority; every escape requires their explicit consent (or an explicit prior per-tool directory grant). A denylist/redaction layer can be a follow-up if wanted.
- **Audit trail lives in the agent loop (authoritative), not only the UI.** The loop's existing `tool_approval_decision` slog record gains `outside_workdir`, `resolved_path`, and `grant_dir` fields — this covers *auto-allowed* path-grant decisions, which never touch the UI. The UI additionally logs popup shows/decisions as today.

### Config policy

New field in the existing `agent.tools` block (sibling of the per-tool objects):

```json
"agent": {
  "tools": {
    "out_of_workdir_access": "ask",
    "read_file":      { "enabled": true, "max_lines": 500, "max_bytes": 65536 },
    "list_directory": { "enabled": true, "max_entries": 500, "max_bytes": 65536 }
  }
}
```

- `"ask"` (default): outside paths trigger the permission flow above.
- `"deny"`: **today's flow, verbatim** — classification is disabled, so an outside call goes through the *ordinary* approval popup (or a tool-name session grant) exactly as now, and `Execute` rejects it with the containment soft error. No escape popup, no escape grant, no silent-rejection special case. (A "reject before even prompting" mode was considered and dropped: it would change today's UX for no security gain, since the rejection is already unconditional at execute time.)
- Any other value fails `Validate()` with a clear message. Presence-defaulting follows the existing `agentToolsPresence` pattern (absent field → `"ask"`).

Default is `"ask"` deliberately: the feature is inert until a model actually requests an outside path, and even then nothing is readable without explicit per-user consent. `"deny"` exists for locked-down setups and is documented in AGENTS.md and the README config reference (the settings panel exposes no agent settings today, so config-file-only is consistent; a settings-panel toggle is a separate enhancement).

### Tilde expansion (small adjacent fix, in scope)

With escapes possible, `~/.bashrc` becomes a natural model request. Today a leading `~` is treated as a literal relative component (resolves to `<cwd>/~/...` → "file not found" — misleading). Both tools' path resolution will expand a leading `~/` (and bare `~`) to `os.UserHomeDir()` before resolution; `~user` is not supported (rejected with a clear soft error). Under `"deny"` policy the expansion still applies, so `~/x` now correctly reports "path outside working directory" instead of "file not found" — a deliberate error-text change, called out in the Non-goal section above.

## Implementation plan

### 1. `pkg/ai/tools` — shared resolution, classification, grant-aware execution

**New file `pkg/ai/tools/scope.go`:**

```go
// FileID identifies a filesystem object. Valid is false when the target did
// not exist at classification time (final-component ENOENT — the only
// capture failure that does not fail classification closed).
type FileID struct {
    Dev, Ino uint64
    Valid    bool
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
// escape (in-workdir path, escapes disabled by config, or the args/path
// cannot be resolved or identity-captured — Execute remains the error
// authority).
type EscapeClassifier interface {
    ClassifyCall(args json.RawMessage) *EscapeRequest
}

// ExecGrant carries the per-call scope approved for this execution.
// The zero value grants nothing beyond workdir containment.
type ExecGrant struct {
    // ApprovedPath is the exact resolved path (file for read_file, directory
    // for list_directory) the approval covered; empty means workdir-only.
    ApprovedPath string
    // Target is the object identity the approval covered. Execute verifies
    // the opened handle against it with fstat; Valid=false means the target
    // must still not exist.
    Target FileID
}
```

Plus:

- `captureIdentity(canonicalPath string) (FileID, error)`: the component-wise no-follow walk described in the Security model — `unix.Openat` chain rooted at `/` with `O_RDONLY|O_DIRECTORY|O_NOFOLLOW|O_CLOEXEC` for intermediate directories (each superseded fd closed promptly in the loop), then `unix.Fstatat(parentFD, finalName, AT_SYMLINK_NOFOLLOW)` for the final component — the target itself is never opened (pre-consent device-open side effects), and a symlink mode is rejected explicitly. `captureIdentity("/")` stats the root path directly (no final component, so no parent that could have been swapped). Returns `FileID{Valid: false}` only for final-component `ENOENT`; every other error (including `ELOOP`/`ENOTDIR` from a mid-walk swap) is returned as an error and fails classification closed. Small enough to be one function; unit-testable deterministically (see §8).
- A `fileID(fi os.FileInfo) (FileID, bool)` helper (`syscall.Stat_t` cast, mirroring `ownerGroup`'s existing pattern) for the execution-side handle checks; a non-`Stat_t` `Sys()` fails closed.

**Interface change** in [registry.go](../../pkg/ai/tools/registry.go):

```go
Execute(ctx context.Context, args json.RawMessage, grant ExecGrant) (Result, error)
```

Explicit parameter, not a context value — the grant is security-relevant. All implementations and test fakes updated (two real tools; fakes in `agent_loop_test.go`, registry tests, chat-handler tests).

**Shared helpers** (in `scope.go`): factor path resolution used by both classification and execution — tilde expansion, absolutize against `Cwd`, `filepath.Clean`, symlink resolution via the existing `evalSymlinksAllowingMissing`, and an in/out-of-workdir verdict computed with `filepath.Rel` boundary rules (never string prefix — the sibling-prefix pitfall `/tmp/project` vs `/tmp/project-other` is already regression-tested for `list_directory`; reuse the same discipline). `normalizeToRootRelative` moves here from `list_directory.go` so `read_file` can reuse it for its `os.Root` migration.

**`ReadFile` changes:**

- New field `AllowEscapes bool` (set from config policy by the constructor — signature grows one parameter).
- `ClassifyCall`: decode args; resolve (with tilde expansion); if in-workdir or `!AllowEscapes` or resolution fails → nil. Otherwise capture identity via `captureIdentity(resolved)`; on error → nil (fail closed); on success → `EscapeRequest{RequestedPath: args.Path, ResolvedPath: resolved, GrantDir: filepath.Dir(resolved), Target: id}`.
- `Execute(ctx, args, grant)`:
  - **In-workdir (or zero grant):** containment via `os.Root` — `os.OpenRoot(cwd)`, `root.OpenFile(rel, O_RDONLY|O_NOFOLLOW|O_NONBLOCK, 0)`, then `f.Stat()` on the handle: reject non-regular with today's messages ("path is a directory, not a file", "not a regular file"), map open errors to today's "file not found"/"permission denied"/"path outside working directory". This replaces the pre-open `Stat` + `os.Open` sequence (the resolution helper remains for classification and error wording). Race-free; behavior delta documented above.
  - **Outside with grant:** require `t.AllowEscapes && grant.ApprovedPath != ""`; fresh-resolve, require equality with `grant.ApprovedPath`; `os.OpenFile(resolved, O_RDONLY|O_NOFOLLOW|O_NONBLOCK, 0)`; `f.Stat()` the handle → require regular file and identity == `grant.Target` (or `ENOENT` when `!grant.Target.Valid`). Soft-error messages distinguish "path outside working directory (not approved)" from "path changed during approval; call the tool again".
  - `readLineRange` is refactored to accept an open `*os.File` (both branches produce one) instead of reopening a path; the regular-file check moves onto the handle as above.
- Description and schema text updated (see §7).

**`ListDirectory` changes:**

- Same `AllowEscapes` field + constructor parameter + `ClassifyCall` (`GrantDir` = the resolved directory itself; `Target` = its identity via the same capture walk).
- `Execute(ctx, args, grant)`: in-workdir → unchanged (`os.OpenRoot(cwd)` + rel path). Outside with grant: same gate (`AllowEscapes` + path equality after fresh resolve), then `os.OpenRoot(resolved)`, `fstat` the anchored handle (`root.Open(".")` → `Stat`) and require identity == `grant.Target`, then list `"."` — the existing containment machinery (symlink entries, caps, escaping) carries over verbatim, anchored at the verified directory. Mismatch/empty grant → same soft errors as `read_file`.

### 2. `pkg/commands/agent_loop.go` — classification, grant plumbing

`executeOneTool` reordered and extended:

1. **Registry lookup moves before approval.** (Side benefit: a hallucinated tool name no longer prompts the user before soft-failing.)
2. If the tool implements `EscapeClassifier`, attach the result to the request: `ApprovalRequest` gains `Escape *tools.EscapeRequest` (nil for normal calls).
3. Approve as today.
4. Build the grant: `ExecGrant{ApprovedPath: esc.ResolvedPath, Target: esc.Target}` **iff** `esc != nil && decision.Allow && decision.AllowOutsideWorkdir`; zero grant otherwise.
5. `tool.Execute(ctx, tc.Arguments, grant)`.
6. The existing `tool_approval_decision` log record gains `outside_workdir`, `resolved_path`, `grant_dir` — the loop is the audit point that sees *every* decision, including store-auto-allowed ones that never reach the UI.

`ApprovalDecision` gains `AllowOutsideWorkdir bool`, and its `Persistent` doc comment is updated to state the contextual meaning (tool-name grant for in-workdir requests; per-tool directory grant for escape requests). `AutoAllowApprover` is untouched (never sets `AllowOutsideWorkdir`) — its doc comment gains a sentence stating that it does *not* approve workdir escapes.

### 3. `pkg/commands/ui_approver.go` — `PathGrants` store + policy matrix

**New `PathGrants`** (same file or sibling `path_grants.go`), mirroring `SessionApprovals`' concurrency contract, **keyed by (tool, directory)**:

```go
type PathGrants struct { mu sync.RWMutex; grants map[string][]string } // tool → resolved dirs
// Allow records dir for tool. It rejects (and logs) non-absolute or
// non-clean dirs rather than storing them.
func (g *PathGrants) Allow(tool, dir string)
// IsAllowed reports whether path falls under any dir granted to tool
// (boundary-aware subtree match).
func (g *PathGrants) IsAllowed(tool, path string) bool
```

`IsAllowed`: true iff some stored `dir` for that tool satisfies `path == dir || strings.HasPrefix(path, dir + "/")` (root-dir `"/"` handled explicitly). No `Reset` — `SessionApprovals.Reset` exists but nothing calls it; parity can be added when a session-reset action actually lands.

**`UIApprover` policy** (constructor gains the store: `NewUIApprover(out, policy, grants)`):

- `req.Escape == nil`: unchanged — tool-name session check, else popup; on `Persistent` reply → `policy.Allow(req.Name)`.
- `req.Escape != nil`: check `grants.IsAllowed(req.Name, esc.ResolvedPath)` → auto-allow with `AllowOutsideWorkdir: true` (skip popup). Else popup; on allow reply set `AllowOutsideWorkdir: true`, and on `Persistent` → validate `esc.GrantDir` is absolute, clean, and contains `esc.ResolvedPath`, then `grants.Allow(req.Name, esc.GrantDir)` (validation failure → treat as allow-once, log warning). Never `policy.Allow` for escape requests — the popup said "directory for this tool", so that is what is recorded.

### 4. `pkg/ui` — wiring and popup variant

- [model.go](../../pkg/ui/model.go): hold `pathGrants *commands.PathGrants` next to `sessionApprovals` (~line 154); pass it in the approver factory (~line 196).
- [components/toolapproval/panel.go](../../pkg/ui/components/toolapproval/panel.go):
  - **Escape variant** when `req.Escape != nil`: warning line (existing warn/error style from `pkg/ui/styles`) `⚠ Path is OUTSIDE your working directory`; metadata rows for requested path, resolved path (only when different), and an explicit scope sentence for option 2, e.g. *"read_file may read any file under /etc for this session"*. Button 2 label becomes `2. Allow dir for session` (in-workdir variant keeps `2. Allow for Session`).
  - **Display hardening (applies to both variants):** every model-controlled string the panel renders (paths from args, resolved path, grant dir) is control-escaped — factor `escapeName`/`isSafeDisplayString` out of `list_directory.go` into a small shared helper (e.g. `pkg/ai/tools` export or `pkg/ui/components/utils`) rather than duplicating. Long values are truncated **tail-preserving** (`…irst/part/keeps/the/end/secret`) or wrapped across lines, never prefix-only. The current `renderApprovalKV`/`renderContentPanel` prefix-truncation of raw strings is the bug being fixed.
  - `DecisionKind` values are unchanged — `DecisionAllowSession`'s meaning is contextual and interpreted by `UIApprover` against `req.Escape`; its doc comment ("any future call to the same tool") is updated to describe both meanings. [stream.go](../../pkg/ui/stream.go)'s `handleToolApprovalDecision` mapping to `{Allow, Persistent}` stays as-is.
- [stream.go](../../pkg/ui/stream.go): extend the `tool_approval_show` / decision slog records with `outside_workdir`, `resolved_path`, `grant_dir` when applicable (supplementary to the loop-level audit log).
- Golden files: regenerate (`go test ./pkg/ui/... -update`); add goldens for the escape popup, a control-character-laden path, and a long-path truncation case.

### 5. `pkg/config/config.go` — policy field

- `AgentTools` gains `OutOfWorkdirAccess string` (json `out_of_workdir_access`), constants `WorkdirAccessAsk = "ask"` / `WorkdirAccessDeny = "deny"`, default `"ask"` in `Default()`.
- `agentToolsPresence` gains `OutOfWorkdirAccess *string`; `applyAgentToolsDefaults` fills the default when absent, preserving the existing per-tool independence.
- `Validate()` rejects values other than `"ask"`/`"deny"`.

### 6. `pkg/commands/handlers.go` — registration

`buildToolRegistry` passes `cfg.Agent.Tools.OutOfWorkdirAccess == config.WorkdirAccessAsk` into both constructors. Doc comment updated: tools enforce cwd containment *by default; out-of-workdir paths require per-call user approval when configured*.

### 7. Model-facing text

Stale text actively teaches the model not to try outside paths, so the tool self-descriptions become policy-aware — `Definition()` picks text based on `AllowEscapes` (they are currently fixed strings; this makes them dynamic, covered by tests):

- `read_file` description, `"ask"`: "…Paths outside the user's working directory require the user's explicit permission — the harness will ask them; call the tool normally with the path you need. `~/` refers to the user's home directory." `"deny"`: today's inside-only wording (plus the `~/` sentence, since expansion applies regardless).
- `list_directory` description: same treatment.
- Schema `path` property descriptions: same split; under `"ask"` drop "absolute paths must lie inside it".
- [ai/context.go](../../pkg/ai/context.go) `AppendToolInstructions` is **left unchanged**: it already says "do not ask the user for permission, the harness will", and it has no access to config — a global sentence about outside paths would be misleading under `"deny"`. The per-tool descriptions carry the policy-specific guidance.

### 8. Tests

**`pkg/ai/tools` (`scope_test.go`, extensions to both tool tests):**

- Resolution helper: tilde expansion (`~`, `~/x`, `~user` rejected), relative/absolute in- and out-of-workdir verdicts, sibling-prefix boundary (`/tmp/project-other` under cwd `/tmp/project` is *outside*), symlink pointing out of workdir classified as outside with resolved target.
- **`captureIdentity` (deterministic race-boundary tests, no timing dependence):**
  - Happy path: identity matches an independent `os.Stat` of the same file.
  - **Parent-swap regression:** canonicalize a path, then replace a parent directory with a symlink to elsewhere, then call `captureIdentity` on the *stale canonical string* — must error (`ELOOP`/`ENOTDIR`), never return the redirected object's identity.
  - Final component is a symlink (swapped post-canonicalization) → error (`Fstatat` mode check).
  - Final-component `ENOENT` → `FileID{Valid: false}`, nil error; *intermediate* `ENOENT` → error (fail closed).
  - Permission-denied intermediate (chmod 000, skipped as root) → error, not `Valid=false`.
  - FIFO / char device (`/dev/null`) as final component → identity captured **without opening the target** (`Fstatat`), returns promptly (test timeout guard).
  - Root path: `captureIdentity("/")` returns the root directory's identity; `list_directory` classification and (grant-matched) execution for `"/"` round-trips.
- `ClassifyCall`: nil for in-workdir paths, nil when `AllowEscapes=false`, nil on bad JSON/unresolvable path, **nil when `captureIdentity` errors**; correct `RequestedPath`/`ResolvedPath`/`GrantDir`/`Target` for files (parent dir) and directories (self); `Target.Valid=false` for a missing file.
- `Execute` with zero grant: outside path → soft error (both tools; exact current behavior preserved).
- `Execute` with matching grant: `read_file` reads an outside file (line ranges, caps still enforced); `list_directory` lists an outside dir (entries, caps, escaping still enforced).
- `Execute` with path mismatch (grant path ≠ fresh resolution — swap *before* re-resolution): soft error "path changed during approval".
- `Execute` with identity mismatch (correct `ApprovedPath`, different `FileID` — models a swap in the resolve→open window at the unit level): soft error, no content. Same for `list_directory` (root-anchor identity ≠ grant). *(Complements — does not replace — the `captureIdentity` boundary tests above; together they cover capture-time and use-time races, and by construction reads only ever go through the fstat-verified handle.)*
- `Execute` where `AllowEscapes=false` but a non-empty grant is passed (defense-in-depth): containment error, grant ignored.
- Missing-at-approval semantics: `Target.Valid=false` grant + file still missing → "file not found"; file created post-approval → "path changed during approval".
- **FIFO/device promptness:** in-workdir and escape-branch `read_file` on a writer-less FIFO returns the "not a regular file" soft error promptly (test timeout guard); char device (`/dev/null`) rejected as non-regular.
- `read_file` in-workdir via `os.Root`: existing test matrix passes; in-tree symlink with absolute target now rejected (documented delta, regression-tested); in-tree path swapped to an outside symlink between calls cannot escape.
- Outside file that does not exist: classification still reports the escape (prompt-for-missing-file is accepted, documented behavior).
- In-workdir call with a *stale* non-empty grant: containment logic unchanged (grant ignored for in-workdir resolutions).

**`pkg/commands`:**

- `PathGrants`: subtree membership, boundary non-match (`/var/log` vs `/var/log2`), root-dir grant, **cross-tool isolation** (grant for `list_directory` never satisfies `read_file` and vice versa), rejection of relative/unclean dirs, concurrent access.
- `UIApprover` matrix: escape request not auto-allowed by tool-name session grant; auto-allowed by a covering *same-tool* path grant (with `AllowOutsideWorkdir` set); popup allow-once sets `AllowOutsideWorkdir` without persisting; persistent escape decision lands in `PathGrants` under the right tool (and *not* in `SessionApprovals`); persistent normal decision unchanged; `GrantDir`-doesn't-contain-`ResolvedPath` downgrade to allow-once.
- Agent loop: grant passed to `Execute` only when classification + `Allow` + `AllowOutsideWorkdir` all hold; `AutoAllowApprover` never yields a grant; unknown tool no longer round-trips through the approver (reorder regression); `ApprovalRequest.Escape` populated from the classifier; audit log fields present on escape decisions (including store-auto-allowed ones).
- `buildToolRegistry`: policy flag propagates to both tools; under `"deny"` both classifiers return nil.
- **Deny-mode end-to-end:** outside call under `"deny"` goes through the *ordinary* popup (or session auto-allow) and fails with the containment soft error — i.e. today's flow, asserted explicitly.

**`pkg/config`:** default `"ask"`; presence matrix (absent block/field → default, explicit `"deny"` kept, invalid value fails `Validate`).

**`pkg/ui`:** popup goldens for the escape variant, hostile-control-character path, long-path tail-preserving truncation, small-terminal width; decision mapping unchanged for both variants.

### 9. Documentation

- [AGENTS.md](../../AGENTS.md): add `out_of_workdir_access` to the config example (§Configuration) and a sentence to the tool-calling bullet in Key Architectural Patterns.
- README config reference: document the `"ask"`/`"deny"` values and what the escape popup means (user-facing docs for the new prompt).
- Move this doc to `docs/feature_doc/completed/` when merged.

## Verification

1. `make check` (fmt, vet, build, full test suite).
2. Manual, `"ask"` policy: in `/chat` ask *"what's in /etc/hosts?"* — expect the escape popup showing the resolved path with the OUTSIDE warning; Allow once → sidebar shows the call and the model quotes the file. Repeat → prompted again. Choose "Allow dir for session" → a follow-up *"and /etc/hostname?"* runs with no popup (same directory, same tool), while *"list /etc"* prompts (different tool) and *"read /var/log/syslog"* prompts (different directory).
3. Manual, escalation guard: session-allow `read_file` on an in-workdir call first, then ask for `/etc/hosts` — the escape popup **must still appear**.
4. Manual, symlink display: `ln -s /etc target` inside the workdir, ask to read `target/hosts` — popup shows resolved `/etc/hosts`, not the relative path.
5. Manual, tilde: *"read ~/.bashrc"* prompts with the expanded home path.
6. Manual, `"deny"` policy: set `"out_of_workdir_access": "deny"`, restart — asking for `/etc/hosts` shows the ordinary approval popup (today's flow) and after approval yields the containment soft error; the tool description advertises inside-only.
7. Manual, FIFO: `mkfifo p` in the workdir, ask to read `p` — prompt returns immediately with "not a regular file", no hang.

## Out of scope

- Persistent (cross-session) path grants in config.
- A sensitive-path denylist or content redaction.
- `~user` expansion; environment-variable expansion in paths.
- Write-capable tools (any future write tool must **not** reuse `PathGrants` read grants; the per-tool keying enforces this structurally).
- A settings-panel toggle for `out_of_workdir_access` (config-file + docs only; the panel exposes no agent settings today).
- Windows path semantics (project targets Linux/macOS).
