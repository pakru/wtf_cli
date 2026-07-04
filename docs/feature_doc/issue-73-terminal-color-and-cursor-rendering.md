# Fix Terminal Rendering: Color Bleed & Progress-Bar Corruption (Issue #73)

**Status:** Implemented (pending PR)
**Issue:** [#73 — Fix CLI text coloring issues](https://github.com/pakru/wtf_cli/issues/73)
**Also fixes:** scattered/duplicated progress bars from `pkcon`-style tools (reported alongside #73, no separate issue)

## Problem

Both symptoms live in the normal-mode viewport renderer
[pkg/ui/terminal/line_renderer.go](../../pkg/ui/terminal/line_renderer.go) (`LineRenderer`) — a
hand-rolled partial terminal emulator whose output feeds
[pkg/ui/components/viewport/viewport.go](../../pkg/ui/components/viewport/viewport.go).
It is **display-only**: AI/WTF context capture uses the separate plain-text
`Normalizer` ([pkg/ui/terminal/normalizer.go](../../pkg/ui/terminal/normalizer.go)) and is not
affected by this plan.

### Symptom 1 — inconsistent coloring (issue #73)

`LineRenderer` stores SGR color sequences as **zero-width cells at fixed columns** inside the
line buffer (`handleCSI` case `'m'` → `insertZeroWidthAt`). Real terminals treat SGR as
*state*, not content. Consequences:

- **Erase drops resets.** `CSI K` (`truncateFromCol`) deletes every cell after the cursor —
  including a `\x1b[0m` reset stored there — while an opener earlier in the line survives.
  Verified repro: feeding `\x1b[33mYELLOW STATUS\x1b[0m` then `\rok\x1b[K\n` renders line 0 as
  `\x1b[33mok` — unbalanced. Since `Content()` joins lines verbatim, the stray opener colors
  **every following line** until some later output happens to reset. This mechanism is
  consistent with the whole-lines-wrongly-yellow/green `apt update` screenshot in #73 (the
  `\r` + rewrite + `CSI K` redraw pattern is what apt and shell prompts emit constantly);
  the mechanism is proven by repro, though no `apt` byte capture was taken, so the
  attribution is high-confidence rather than byte-proven.
- **Overwrite keeps stale openers.** After `\r`, new plain text overwrites visible cells but
  old zero-width SGR cells stay put: `\x1b[32m[=== ] 50%\x1b[0m` + `\rDONE......` renders
  `DONE......` in stale green.
- The same positional representation corrupts styling under ![alt text](image.png)`DCH`/`ICH`/`ECH` and line
  erasure — the defect is the representation, not one code path.

### Symptom 2 — scattered progress bars (`pkcon update`)

pkcon redraws its progress bar with **DECSC/DECRC**: `ESC 7` (save cursor) once after the
label, then `ESC 8` (restore cursor) + a fresh frame for every update. A real PTY byte
capture of `pkcon get-updates` (via `script(1)`) contains 3× `ESC 7` and 14× `ESC 8`.

`LineRenderer.Append` recognizes only `ESC [` (CSI) and `ESC ]` (OSC); every other
post-`ESC` byte is silently discarded, and `CSI s`/`CSI u` (ANSI save/restore) are also
absent from `handleCSI`. The cursor therefore never returns to the bar start and every
frame **appends after the previous one**, producing one enormous line of concatenated
frames (`[    ] (0%)  [    ] (0%)  [ ==   ] …`). Feeding the real capture through
`LineRenderer` reproduces the concatenated-frame corruption seen in the report.

### Why not just swap in a real emulator?

The repo already ships `vito/midterm` (used for fullscreen apps in
[pkg/ui/components/fullscreen/fullscreen_panel.go](../../pkg/ui/components/fullscreen/fullscreen_panel.go)),
but v0.2.4 is a **fixed-size screen** emulator: its scrollback callback is commented out
(`terminal.go` `scrollUpN`), it has no wide-character (CJK/emoji) cell support, and
narrowing on `Resize` clips rows. A drop-in swap would lose scrollback history — the one
thing normal mode needs. Unification on a real emulator remains a possible **Stage 2**
(separate doc/issue); this plan is the low-risk targeted fix.

## Design

Two changes inside `LineRenderer`, no public API changes — the viewport contract
(`Append`, `Content`, `CursorPosition`, `Reset`) stays identical.

### 1. SGR becomes state attached to visible cells

- Add a **pen** (current rendition) to `LineRenderer`, updated by `CSI … m`:

  ```go
  type cellStyle struct {
      fg    string // canonical SGR fragment: "31", "38;5;208", "38;2;r;g;b"; "" = default
      bg    string // "41", "48;5;n", "48;2;r;g;b"; "" = default
      attrs uint16 // bit flags: bold, dim, italic, underline, blink, reverse, conceal, strike
  }
  ```

  Cells reference the style via pointer: `lineCell{text, width, style *cellStyle}` with
  `nil` meaning default style. The renderer keeps one **interned** `*cellStyle` per pen
  change, so a run of cells written under the same pen shares a single allocation; unstyled
  cells pay only the 8-byte nil pointer. (A flat embedded struct would cost ~40 bytes on
  *every* cell — two string headers plus flags — which is why the pointer/interning form is
  specified.) **Interned `cellStyle` values are immutable**: `applySGR` computes the next
  pen as a *new* (or cache-shared) `*cellStyle` and must never mutate a style already
  referenced by existing cells — otherwise a later SGR would retroactively recolor
  previously written text. `insertZeroWidthAt` and the SGR-as-cell path are removed.

- **The pen persists across `\r` and `\n`** — exactly like a real terminal. Per-line
  balancing below is a *rendering* concern; the pen itself is only changed by SGR
  sequences (and `Reset()`).

- **SGR parsing** (`applySGR(params []int)`) supports: `0` (full reset — also the meaning
  of an empty parameter list, so `applySGR(nil)` resets), `1/2/3/4/5/7/8/9` (attribute
  set), partial resets `22` (bold **and** dim off), `23`, `24`, `25`, `27`, `28`, `29`,
  colors `30–37`, `90–97`, `39` (default fg), `40–47`, `100–107`, `49` (default bg), and
  extended forms `38;5;n`, `48;5;n`, `38;2;r;g;b`, `48;2;r;g;b`.
  - `21` is **explicitly ignored**: per ECMA-48/xterm it means *double underline*, not a
    partial reset; it is rare in normal-mode output and not worth an attribute bit.
  - **Malformed extended colors:** if a `38`/`48` introducer lacks the required following
    parameters (`38;5` with no index, `38;2` with fewer than three channels), the parser
    consumes the remaining parameters of the sequence and stops — it must never
    reinterpret color-argument leftovers as blink/reverse/etc.
  - Other unknown parameters are skipped without corrupting the rest of the sequence.

- **Parser fix for empty CSI parameters.** `pushCSIParam` currently drops empty params
  (`CSI ;5H` loses the implicit `0`). Precise algorithm (a new `csiSep`/separator-seen flag
  accompanies the existing `csiParam`/`csiHas`):
  - On `;`: **always** append the current value (default `0`) to `csiParams`, then mark
    that a separator was seen.
  - On the final byte: append the current value only if a digit **or** separator was seen
    since CSI start.
  - The separator-tracking flag is cleared on CSI start, on sequence completion, and in
    `Reset()`.

  Resulting parses: `CSI ;5H` → `[0,5]`; `CSI 5;H` → `[5,0]`; `CSI ;;H` → `[0,0,0]`;
  `CSI m` → `nil` (empty), which `applySGR` treats as reset. Well-formed `38;5;n` already
  parses correctly today; this fix is for omitted positional parameters and mixed empty
  forms.

- **Render-time emission with per-line balancing:** `lineBuffer.String()` walks cells,
  emits an SGR sequence only when a cell's style differs from the previous cell's
  (transition rule: emit `ESC[0m` then the full style of the new cell — conservative and
  always correct; a line's first styled cell establishes its style from scratch), and
  appends `ESC[0m` at end of line if the last cell is styled. **Every rendered line is
  self-balanced**, so bleed across lines becomes impossible by construction, including when
  the viewport is scrolled so an "opener line" is off-screen.

- **Erase/edit ops become style-safe for free:** `CSI K/J/P/X/@` manipulate visible cells
  only; there are no reset cells to lose. Cells blanked by `ECH`/`EL` get the default
  style. **Erasure does not touch the pen** — clearing cells and resetting the current
  rendition are independent in a real terminal. (Background-color-erase is a non-goal, see
  below.)

### 2. Cursor save/restore: DECSC/DECRC and CSI s/u

- In the `inEscape` branch of `Append`, handle `ESC 7` (DECSC) and `ESC 8` (DECRC).
- In `handleCSI`, handle `s` (SCOSC) and `u` (SCORC).
- Saved state as far as we model it: `{row, col, pen}` plus a `savedValid bool` — DECRC
  restores the rendition too, not only the position (per the VT100 spec; pkcon relies on
  position, themed tools may rely on rendition).
- **One shared save slot**: `ESC 7` and `CSI s` write the same slot (xterm-consistent);
  a repeated save overwrites the previous one.
- **Restore without a prior save is a no-op.** This is an *intentional deviation* from DEC
  semantics (a real VT restores home position/default rendition); since these sequences
  were previously ignored entirely, a no-op is strictly closer to correct than today and
  cannot move the cursor to scrollback row 0 (which for `LineRenderer` would mean the top
  of history, not the top of the screen). Documented in a code comment and pinned by test.
- `Reset()` clears the pen, the saved-cursor slot (`savedValid = false`), and all CSI/SGR
  parser bookkeeping, alongside the existing state.
- The escape state machine already persists across `Append` chunk boundaries (`inEscape`
  flag), so `ESC` split from `7` by PTY batching works; covered by tests below.

### Out of scope (explicit non-goals)

| Non-goal | Rationale |
|---|---|
| Line wrapping model / screen-relative `CUP`/`ED` | Needs a screen-height model; separate Stage 2 with a real emulator. Today `ESC[H` already addresses scrollback-absolute rows; unchanged. |
| `ESC[2J` semantics change | Real terminals clear the visible screen (`ED 3` clears history); `LineRenderer` has no height model. Current clear-all behavior kept; noted as known limitation. |
| Background-color-erase (BCE) | Erased cells get default style; deviation is cosmetic and rare in normal-mode output. |
| Scroll regions, `ESC D/M/E` | Not emitted by the affected tools in normal mode; alt-screen apps go through midterm. |
| Scrollback cap / memory bound | Pre-existing unbounded growth; separate issue. Note: the new representation adds 8 bytes (style pointer) to **every** cell, plus one shared `cellStyle` allocation per pen change — acknowledged in Risks. |
| Split-rune UTF-8 buffering | `Append` decodes UTF-8 only within the current chunk (`utf8.DecodeRune(data[i:])`), so a CJK/emoji rune split across PTY reads is already corrupted today. Pre-existing limitation, recorded here, not widened or fixed by this change. |
| midterm unification | Stage 2 candidate; blocked on scrollback support upstream (callback commented out in v0.2.4). |

## Changes (file by file)

| File | Change |
|---|---|
| [pkg/ui/terminal/line_renderer.go](../../pkg/ui/terminal/line_renderer.go) | Add `cellStyle` + interning, pen state, `applySGR`; store style pointer per visible cell; remove SGR zero-width-cell path; style-aware `lineBuffer.String()` with per-line balancing; `ESC 7/8` in escape branch; `CSI s/u` in `handleCSI`; `pushCSIParam` empty-param fix (with separator flag); extend `Reset()` to clear pen/saved-cursor/parser state. |
| [pkg/ui/terminal/line_renderer_test.go](../../pkg/ui/terminal/line_renderer_test.go) | Update tests that assert raw positional SGR pass-through (byte-level expectations change; visual result must not). Add new cases (below). |
| [pkg/ui/components/viewport/viewport_test.go](../../pkg/ui/components/viewport/viewport_test.go) | Integration tests: cursor overlay + selection composed over **styled** renderer output in their real order (`renderContent`), including copied-text extraction. |
| `pkg/ui/terminal/testdata/pkcon_capture.raw` (new) | Committed, sanitized `script(1)` capture of `pkcon get-updates` progress frames (trimmed to a few KB, no personal data). |
| Golden files under [pkg/ui/](../../pkg/ui/) (`model_golden_test.go` testdata) | Regenerate with `go test ./pkg/ui/... -update` **and manually inspect**: welcome banner ANSI is re-emitted in canonical form — bytes may differ, rendering must be identical. Back the inspection with an ANSI-stripped semantic assertion (below). |

Unchanged consumers (verified surface): viewport uses only
`Append`/`Content`/`CursorPosition`/`Reset`; cursor overlay needs only `(row, col)`;
selection ([pkg/ui/components/selection/selection.go](../../pkg/ui/components/selection/selection.go))
already walks ANSI-aware over "lines containing SGR" — line shape does not change
(no fixed-width padding is introduced).

## Verification Plan

Unit tests (`pkg/ui/terminal` unless noted):

1. **Bleed repros become regression tests:**
   - `\x1b[33mYELLOW STATUS\x1b[0m` + `\rok\x1b[K\n` + `plain line\n` → line 0 renders `ok`
     with **no** styling; line 1 unaffected. Every output line has balanced SGR.
   - `\x1b[32m[=== ] 50%\x1b[0m` + `\rDONE......` → `DONE......` unstyled.
   - Mid-line color change: `A\x1b[31mB\x1b[0mC` → transitions emitted at B and C, line ends
     balanced.
   - **Pen persists across newline:** `\x1b[31mred\nstill red\x1b[0m\n` → *both* lines render
     red, each line independently opening and closing its style.
2. **pkcon fixture test:** feed `testdata/pkcon_capture.raw`; assert each label line contains
   **exactly one** final bar frame (`strings.Count(line, "] (") == 1` style assertion), not
   concatenated frames.
3. **DECSC/DECRC & CSI s/u:** save → newline/other output → restore → overwrite lands at
   saved position **with saved pen**; repeated save overwrites; `ESC 7`/`CSI s` share the
   slot; `ESC 8`/`CSI u` without a prior save is a no-op (documented deviation).
4. **SGR state machine:** each supported partial reset, explicitly including `22` clearing
   **both** bold and dim; `21` ignored; 256-color and truecolor forms; attribute
   combinations; `ESC[m` as reset; malformed extended colors (`38;5`, `38;2;1;2`) consume
   the sequence without corrupting other attributes.
   - **Style-immutability regression:** write `\x1b[31mred`, switch pen `\x1b[34mblue`,
     switch back `\x1b[31mred2` — the first run must still render red (a mutated shared
     style would recolor it), and the two red runs may share one interned style.
5. **Empty CSI parameters (positional):** `CSI ;5H` → `[0,5]`, `CSI 5;H` → `[5,0]`,
   `CSI ;;H` → `[0,0,0]`, and `ESC[;31m`.
6. **Styled edit/erase ops:** `DCH`, `ICH`, `ECH`, `EL`, `ED` over styled cells — styles of
   surviving cells intact, blanked cells default, pen unchanged after erasure.
7. **`Reset()` clears everything new:** pen, saved cursor (`savedValid`), separator flag,
   CSI bookkeeping — plus all existing state.
8. **Chunk-split robustness:** feed inputs byte-by-byte through `Append` and assert output
   identical to single-chunk feeding — covering `ESC`|`7`, split CSI params, split SGR.
   Restricted to ASCII/escape-sequence inputs: intra-rune UTF-8 splits are a pre-existing
   limitation (see non-goals) and are excluded.
9. **Wide characters:** existing CJK/emoji tests keep passing (style attaches to the single
   wide cell).
10. **Viewport integration** (`pkg/ui/components/viewport`): styled content + cursor overlay
    + drag selection composed in their real order; `FinishSelection` copied text matches the
    plain text of the selected region.
11. **Welcome banner semantics:** `ansi.Strip(rendered) == ansi.Strip(welcome.WelcomeMessage())`
    (using `github.com/charmbracelet/x/ansi`), so golden regeneration cannot silently hide a
    banner regression.

Integration/manual:

- `make check` (fmt + vet) and `go test ./...` green; golden diffs reviewed, not
  blind-regenerated.
- Manual run: `pkcon get-updates` (in-place bar), `apt list --upgradable` / colored `ls`,
  colored shell prompt + `Ctrl+R`/completion redraws — no bleed, no scatter; vim/htop
  (fullscreen path) unaffected; sidebar toggle + resize while colored output streams.

## Risks

| Risk | Mitigation |
|---|---|
| Golden/byte-level test churn (canonicalized SGR output) | Inspect diffs; only byte-form may change, never visible rendering. ANSI-stripped semantic assertions (welcome banner, key unit cases) make regeneration unable to mask content regressions. |
| Welcome banner (lipgloss ANSI injected via `AppendOutput`) re-canonicalized | Golden inspection + the `ansi.Strip` equality test; banner is static so a one-time visual check suffices. |
| Cursor overlay emits `27m`-style closures that don't restore ambient reverse video | Pre-existing limitation, unchanged by this plan; per-line balancing reduces ambient-state surprises. Recorded, not fixed here. |
| Per-cell memory: +8 bytes/cell (style pointer) on all cells, shared `cellStyle` per pen change | Interning keeps styled runs to one allocation; add a micro-benchmark comparing before/after on a large plain + styled scrollback to document the real overhead. Scrollback cap remains a separate issue. |
| `Content()` rebuilds the whole scrollback string on every append (pre-existing); style emission adds work per styled cell | No behavior change in this PR; measured by the same benchmark; a rebuild/cap optimization belongs to the scrollback-cap issue. |

## Rollout

Single PR: branch `issues/issue-73-terminal-color-rendering`, tests-first for the SGR state
machine, then renderer swap, then fixture + golden review. No config, no migration.

---

*Review: plan independently cross-validated against the codebase by a second AI reviewer
(OpenAI Codex CLI); all requested edits are incorporated above (SGR 21 semantics, style
interning/immutability and memory accounting, empty-CSI-parameter algorithm, DECRC no-save
deviation, expanded test matrix, split-UTF-8 scoping).*
