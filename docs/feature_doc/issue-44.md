# Fix Terminal Escape Sequence Handling

## Root Causes

Three user-visible display bugs caused by **missing CSI handlers in `LineRenderer`** (the viewport rendering path):

| Bug | Missing Handler | Effect |
|-----|----------------|--------|
| npm spinner concatenation | `CSI G` (cursor horizontal absolute) | Spinner chars append instead of overwrite |
| pkcon progress bar tails | `\r` + shorter text | Old trailing characters persist |
| Gemini CLI UI duplication | `CSI A/B/J` + `CSI H` bug | Each redraw appends below instead of overwriting |

### Test Evidence

**Gemini CLI** (Ink-based inline TUI) sends `\x1b[3A` (cursor up 3) then redraws. Since `A` is ignored:
```
Frame 1: "> Type your message\n~ no sandbox\nshift+tab to accept edits"
Frame 2: "> Type your message\n~ no sandbox\n> dft+tab to accept edits\n~ no sandbox\nshift+tab to accept edits"
                                              ^^^ duplicated + corrupted
```

---

## Proposed Changes

### Required: Visible Rendering (`LineRenderer`)

#### [MODIFY] [line_renderer.go](file:///home/pavel/STORAGE/Projects/my_projects/wtf_cli/pkg/ui/terminal/line_renderer.go)

Add to `handleCSI()`:

| Sequence | Action |
|----------|--------|
| `CSI A` | `r.row -= n` (cursor up, clamp to 0) |
| `CSI B` | `r.row += n` (cursor down, ensureLine) |
| `CSI G` | `r.col = param-1` (cursor horizontal absolute, default 1) |
| `CSI K` | **Enhance**: support param 2 (clear entire line) |

**`CSI J` — Erase in Display** (multi-line semantics):

| Param | Meaning | Erase rules |
|-------|---------|-------------|
| 0 (default) | Cursor → end of display | 1. Truncate current row from `r.col` onward (`truncateFromCol(r.col)`) <br> 2. Delete all rows after `r.row` (`r.lines = r.lines[:r.row+1]`) <br> 3. Cursor position unchanged |
| 2 | Entire display | 1. Clear all rows (`r.lines = r.lines[:0]`) <br> 2. Reset `r.row = 0`, `r.col = 0` <br> 3. `ensureLine(0)` to create empty first row |

Fix existing `CSI H` no-params: set **both** `r.row = 0` and `r.col = 0` (currently only resets col).

> [!WARNING]
> **`pendingCR` was attempted and reverted.** Truncating the line on bare `\r` caused Ctrl+C to erase prompt lines (bash sends `\r^C\n`). The pkcon progress bar issue remains open — `pkcon` likely uses `\r\n` between updates (not bare `\r`), so a different approach is needed.

### Optional: Buffer/Capture Path (`Normalizer`)

> [!IMPORTANT]
> The `Normalizer` feeds the circular buffer used for LLM context via `appendNormalizedLines`. It is a separate data path from the viewport display. Changes here should be **validated independently** to avoid regressions in capture logic.

#### [MODIFY] [normalizer.go](file:///home/pavel/STORAGE/Projects/my_projects/wtf_cli/pkg/ui/terminal/normalizer.go)

- Add `CSI G` handler (set `n.col`) — aligns column tracking with real cursor position
- Fix `pendingCR` + ESC: also clear line content when `\r` precedes an escape

> [!NOTE]
> The Normalizer doesn't need `CSI A/B/J` because it works line-by-line (flushing on `\n`), not as a 2D grid. Only `CSI G` and the `pendingCR` edge case are relevant.

---

## Verification Plan

### Primary: Full Pre-submit Validation
```bash
make check
```
Runs `fmt` → `vet` → `build` → `go test ./...` (entire project). Catches integration regressions in viewport/model paths.

### Supplemental: Targeted Tests

**LineRenderer** (rendering path):
```bash
go test -v -run TestLineRenderer ./pkg/ui/terminal/...
```
- `CSI A/B` — cursor up/down repositioning
- `CSI G` — cursor horizontal absolute
- `CSI J` — erase display (params 0 and 2)
- `CSI H` no-params → row=0, col=0
- `pendingCR` — `\r` + shorter text truncation
- Ink render cycle — multi-frame overwrite simulation
- Regression: CRLF, ANSI colors, Ctrl+C pattern all still work

**Normalizer** (capture path — validate separately):
```bash
go test -v -run TestNormalizer ./pkg/ui/terminal/...
```
- `CSI G` sets column correctly
- `\r` + ESC clears line
- Regression: existing prompt/command extraction still works

