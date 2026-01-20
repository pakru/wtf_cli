# WTF_CLI vs Crush: Architecture & Stability Analysis

## Executive Summary

After analyzing both projects, the key difference comes down to **maturity and ecosystem leverage**. Crush is built by Charmbracelet (creators of the underlying TUI libraries) with access to experimental/internal packages, while `wtf_cli` implements similar functionality from scratch using public APIs.

---

## Architecture Comparison

### Package Structure

| Aspect | wtf_cli | Crush |
|--------|---------|-------|
| **Internal Packages** | 9 packages | 31+ packages |
| **UI Architecture** | Monolithic `model.go` (928 lines) | Modular with `components/`, `styles/`, `page/`, `util/` subdirectories |
| **ANSI Handling** | Custom `altscreen.go` | Dedicated `ansiext/` package |
| **Terminal Recording** | N/A | `charm.land/x/vcr` |
| **Styling** | Inline lipgloss | Centralized `styles/` package |

### Key Dependencies

| wtf_cli | Crush |
|---------|-------|
| `charmbracelet/bubbles v0.21.0` | `charm.land/bubbles/v2` (RC version) |
| `charmbracelet/bubbletea v1.3.10` | `charm.land/bubbletea/v2` (RC version) |
| `charmbracelet/lipgloss v1.1.0` | `charm.land/lipgloss/v2` (beta) |
| `vito/midterm` for terminal emulation | `charm.land/fantasy` for advanced rendering |
| Manual ANSI handling | `charm.land/glamour/v2` for markdown |
| N/A | `charm.land/x/vcr` for terminal recording |

---

## Why Crush Feels More Robust

### 1. **Bubble Tea v2 (Unreleased Features)**
Crush uses `charm.land/bubbletea/v2`, an RC (release candidate) version with improvements over the public v1:
- Better input handling
- Improved terminal state management
- Enhanced resize coordination

### 2. **Centralized Style System**
Crush has a dedicated `internal/tui/styles/` package, eliminating style inconsistencies:
```
internal/tui/
├── components/   # Reusable UI components
├── styles/       # Centralized color/style definitions
├── highlight/    # Syntax highlighting
├── page/         # Page-level layouts
└── util/         # Shared utilities
```

### 3. **VCR Module for Terminal State**
Crush uses `charm.land/x/vcr` which provides:
- Proper terminal state recording/replay
- Clean screen management
- Consistent frame rendering

### 4. **Fantasy Rendering Engine**
Crush uses `charm.land/fantasy v0.6.1`, an internal library for:
- Advanced cell-based rendering
- Proper Unicode handling
- Flicker-free updates

### 5. **Dedicated Subsystems**
Crush separates concerns into dedicated packages:
- `internal/ansiext/` - ANSI escape handling
- `internal/format/` - Output formatting
- `internal/shell/` - Shell integration
- `internal/event/` - Event system
- `internal/csync/` - Concurrency primitives
- `internal/pubsub/` - Message passing

---

## WTF_CLI Current Issues

### ❌ Monolithic Model
Your `model.go` at 928 lines handles:
- PTY management
- Window resizing
- Full-screen mode
- Sidebar
- Multiple overlay panels
- Stream handling

**Impact**: Hard to test, hard to modify, state bugs leak across components.

### ❌ Manual ANSI Parsing
Your `altscreen.go` manually detects alternate screen sequences:
```go
altScreenEnterSeqs = [][]byte{
    []byte("\x1b[?1049h"),
    []byte("\x1b[?1047h"),
    // ...
}
```
**Impact**: Edge cases with split sequences, timing issues.

### ❌ No Centralized Styling
Styles defined inline in component files:
```go
var viewportStyle = lipgloss.NewStyle().
    BorderStyle(lipgloss.RoundedBorder())
```
**Impact**: Inconsistent colors, hard to theme.

### ❌ Input Handler Complexity
Your `input.go` manually tracks:
- Line start position
- Palette mode
- Full-screen mode
- Cursor key application mode
- Keypad mode

**Impact**: State synchronization bugs.

---

## Recommended Improvements for WTF_CLI

### Priority 1: Adopt Bubble Tea v2 (When Public)
Monitor `charm.land/bubbletea/v2` for public release. It will solve many low-level issues.

### Priority 2: Extract Subsystems
Refactor into dedicated packages:
```
pkg/ui/
├── components/
│   ├── sidebar/
│   ├── palette/
│   ├── settings/
│   └── viewport/
├── styles/
│   └── theme.go
├── layout/
│   └── split.go
└── terminal/
    ├── altscreen.go
    └── modes.go
```

### Priority 3: Implement a Style System
Create `pkg/ui/styles/theme.go`:
```go
package styles

import "github.com/charmbracelet/lipgloss"

var (
    Primary   = lipgloss.Color("#7C3AED")
    Secondary = lipgloss.Color("#06B6D4")
    Muted     = lipgloss.Color("#6B7280")
    
    Border = lipgloss.NewStyle().
        BorderStyle(lipgloss.RoundedBorder()).
        BorderForeground(Primary)
        
    Panel = Border.Copy().
        Padding(1, 2)
)
```

### Priority 4: Use Cellbuf for Rendering
You already import `charmbracelet/x/cellbuf` but don't fully utilize it. Cell-based rendering prevents:
- Flicker during updates
- Partial line rendering issues
- Unicode width miscalculations

### Priority 5: Debounce and Batch Updates
Your resize handler is good but apply similar patterns to:
- PTY output batching
- Viewport updates
- Stream content updates

### Priority 6: Add Golden Tests
Crush uses `charmbracelet/x/exp/golden` for snapshot testing UI output. Adopt similar:
```go
func TestViewportRender(t *testing.T) {
    vp := NewPTYViewport()
    vp.SetSize(80, 24)
    vp.AppendOutput([]byte("Hello, World!\n"))
    golden.RequireEqual(t, []byte(vp.View()))
}
```

---

## Quick Wins (Low Effort, High Impact)

1. **Extract styles to a single file** - 1-2 hours
2. **Add debouncing to PTY output** - 30 minutes
3. **Use `lipgloss.JoinVertical` consistently** - 1 hour
4. **Add logging for state transitions** - Already done ✓
5. **Batch resize events** - Already done ✓

---

## Summary Table

| Feature | wtf_cli Current | Recommended | Crush Reference |
|---------|-----------------|-------------|-----------------|
| Architecture | Monolithic | Modular components | `internal/tui/components/` |
| Styling | Inline | Centralized theme | `internal/tui/styles/` |
| ANSI parsing | Manual | Use `x/ansi` more | `internal/ansiext/` |
| Terminal rendering | String-based | Cell-based | `charm.land/fantasy` |
| Testing | Basic unit tests | Golden tests | `x/exp/golden` |
| Event system | Direct calls | Pub/sub | `internal/pubsub/` |

---

## Next Steps

1. **Short-term**: Extract styles and create a theme package
2. **Medium-term**: Refactor `model.go` into component packages
3. **Long-term**: Monitor Bubble Tea v2 release and migrate

The good news: your structural foundation with Bubble Tea is correct. You're using the right patterns (Model-Update-View, message passing). The improvements are evolutionary, not revolutionary.
