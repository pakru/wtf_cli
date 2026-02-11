# Use Cellbuf for Rendering - Implementation Tasks Plan

## Context / Current Status

- Bubble Tea v2 migration is complete and running (`charm.land/*` deps in `go.mod`).
- Rendering is still string-based: `Model.Render()` composes strings with `lipgloss.JoinHorizontal/JoinVertical` and centers overlays via `overlayCenter()` (`pkg/ui/model.go`).
- `cellbuf` is only used for per-line overlay blending in `overlayLine()` (`pkg/ui/model.go`).
- Width handling is mixed (len, `lipgloss.Width`, `runewidth.StringWidth`), which can mismatch terminal cell widths (`pkg/ui/components/statusbar/statusbar_view.go`, `pkg/ui/components/viewport/viewport.go`).
- Golden tests validate string output from `Render()` (`pkg/ui/model_golden_test.go`).

## Goals

- Move to cell-based rendering for the main UI to eliminate flicker and width drift.
- Preserve overlay layering behavior (settings > option picker > model picker > result > palette).
- Maintain current fullscreen alternate-screen behavior.
- Keep UI output stable (update golden tests, add renderer-specific tests).

## Non-Goals

- Redesigning UI visuals or changing layout structure.
- Large-scale package refactors beyond rendering and width calculations.

## Decision

**Chosen:** Option A — Lipgloss Canvas/Layer (cellbuf-backed, integrated with Bubble Tea v2).

**Rationale:** Closest to how Crush composes UI, minimizes custom rendering code, and keeps
component views string-based while still benefiting from cellbuf-backed rendering.

Option B (direct `cellbuf.Buffer`) is deferred; it adds more custom drawing and isn’t needed
for the current refactor.

## Plan

### Phase 0: Design & Scoping

1) Document the layer-based composition approach and overlay order.
2) Define a small set of layout helpers (centered rect, sidebar split, viewport height).
3) Identify special cases: cursor overlay, sidebar width, status bar padding/clipping.

#### Phase 0 Output (Decisions)

- Composition approach: build `[]*lipgloss.Layer` in `Model.Render()` and render with `lipgloss.NewCompositor(layers...).Render()`.
- Base layout:
  - viewport layer at (0, 0), size `viewportWidth x (height-1)`.
  - sidebar layer at (viewportWidth, 0) when visible, size `sidebarWidth x (height-1)`.
  - status bar layer at (0, height-1), size `width x 1`.
- Overlay order (topmost last): settings -> option picker -> model picker -> result -> palette.
- Bounds enforcement: set explicit `Width/Height` on each layer; clamp overlay rects to screen size.

#### Phase 0 Output (Helpers)

- `CenterRect(panelW, panelH, screenW, screenH) (x, y, w, h)`:
  - compute top-left with integer centering, clamp to (0..screenW, 0..screenH).
- `ClampRect(x, y, w, h, screenW, screenH) (x, y, w, h)`:
  - ensure rect stays within screen bounds and widths/heights are non-negative.
- `ViewportHeight(height int) int`:
  - return `max(height-1, 0)` to reserve the status bar line.
- `SidebarSplit(totalW int) (leftW, rightW int)`:
  - reuse existing `splitSidebarWidths` logic to keep behavior stable.

#### Phase 0 Output (Special Cases)

- Cursor overlay is handled in `viewport` via `CursorTracker`; no change needed.
- Status bar padding/truncation must use ANSI width (avoid `len`).
- Viewport padding: avoid double-padding once layers clip to bounds.
- Overlay panels rely on their `View()` output for size; use `lipgloss.Width/Height` consistently.

### Phase 1: Layer Composition Core (Option A)

1) Add a lightweight helper (optional) for layer sizing/positioning (e.g. `pkg/ui/render/layers.go`):
   - `CenterRect(panelW, panelH, screenW, screenH)` using ANSI width.
   - `ClampRect()` to keep overlays within screen bounds.
2) Standardize width calculations on `ansi.StringWidth` or `lipgloss.Width` consistently.
3) Decide how to enforce bounds: set explicit `Width/Height` on layers or pre-pad strings.

### Phase 2: Wire Model.Render() to Layers

1) Replace string joins in `Model.Render()` with layer composition:
   - Create a base layer for viewport at (0,0) with explicit width/height.
   - Add a sidebar layer at (viewportWidth, 0) when visible.
   - Add a status bar layer anchored at the last line across full width.
2) Overlay panels as additional layers using centered rectangle and z-order:
   - settings -> option picker -> model picker -> result -> palette
3) Remove `overlayCenter()` and `overlayLine()` once parity is confirmed.
4) Build the final view via `lipgloss.NewCompositor(layers...).Render()`.

### Phase 3: Component Alignment

1) Status bar: replace `len(content)` logic with ANSI width to avoid padding drift.
2) Viewport: review manual line padding; avoid double-padding now that layers clip.
3) Pickers/palette/sidebar: ensure width calculations use ANSI width and clamp to layer bounds.
4) Confirm overlay panels render at expected sizes with explicit layer width/height.

### Phase 4: Tests & Verification

1) Update golden tests to match the new renderer output:
   - `pkg/ui/model_golden_test.go`
2) Add composition unit tests for:
   - overlay centering with odd widths/heights
   - wide Unicode characters and ANSI styles
3) Run `go test ./...` and confirm no regressions.

## Files Likely Touched

- `pkg/ui/model.go`
- `pkg/ui/components/statusbar/statusbar_view.go`
- `pkg/ui/components/viewport/viewport.go`
- `pkg/ui/components/picker/model_picker.go`
- `pkg/ui/components/picker/option_picker.go`
- `pkg/ui/components/palette/palette.go`
- `pkg/ui/components/sidebar/sidebar.go`
- `pkg/ui/model_golden_test.go`
- Layer/layout helper (optional): `pkg/ui/render/*`

## Risks / Mitigations

- Risk: ANSI width handling differs from current padding and causes layout shifts.
  - Mitigation: centralize width functions and add Unicode width tests.
- Risk: Overlay order or clipping changes view compared to string-based rendering.
  - Mitigation: keep exact overlay order and update golden tests after review.
- Risk: Layer bounds not enforced causes composited view to exceed screen size.
  - Mitigation: set explicit layer width/height and clamp overlay rects.

## Verification Checklist

- `go test ./...`
- Manual run: open palette, settings, model picker, result panel, sidebar
- Manual run: resize terminal while streaming output
- Manual run: run a fullscreen app (vim/nano) and exit cleanly
