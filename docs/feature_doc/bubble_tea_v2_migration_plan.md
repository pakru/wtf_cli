# Implementation Plan: Bubble Tea v2 Migration & Architecture Improvements

## Overview

Migrate wtf_cli from Bubble Tea v1.3.10 to v2.0.0-rc.2 and implement architectural improvements based on the Crush comparison analysis.

---

## Phase 1: Bubble Tea v2 Migration

**Estimated Effort: 2-3 days**

### 1.1 Update Dependencies

#### [MODIFY] [go.mod](file:///home/dev/project/wtf_cli/wtf_cli/go.mod)

Update import paths from `github.com/charmbracelet/*` to `charm.land/*`:

```diff
-	github.com/charmbracelet/bubbles v0.21.0
-	github.com/charmbracelet/bubbletea v1.3.10
-	github.com/charmbracelet/lipgloss v1.1.0
+	charm.land/bubbles/v2 v2.0.0-rc.1
+	charm.land/bubbletea/v2 v2.0.0-rc.2
+	charm.land/lipgloss/v2 v2.0.0-beta.3
```

---

### 1.2 Update Import Paths (All 15 Files)

Files requiring import updates:
- `pkg/ui/model.go`
- `pkg/ui/input.go`
- `pkg/ui/viewport.go`
- `pkg/ui/sidebar.go`
- `pkg/ui/palette.go`
- `pkg/ui/result_panel.go`
- `pkg/ui/settings_panel.go`
- `pkg/ui/model_picker.go`
- `pkg/ui/option_picker.go`
- `pkg/ui/fullscreen_panel.go`
- `pkg/ui/statusbar.go`
- And 4 more files

**Change Pattern:**
```diff
-import tea "github.com/charmbracelet/bubbletea"
+import tea "charm.land/bubbletea/v2"

-import "github.com/charmbracelet/lipgloss"
+import "charm.land/lipgloss/v2"
```

---

### 1.3 Update View() Signatures (9 Files)

The v2 `View()` method returns `tea.View` instead of `string`.

| File | Current | v2 Required |
|------|---------|-------------|
| `model.go:562` | `func (m Model) View() string` | `func (m Model) View() tea.View` |
| `palette.go:142` | `func (p *CommandPalette) View() string` | Return string via `NewView()` |
| `sidebar.go:148` | `func (s *Sidebar) View() string` | Return string via `NewView()` |
| `viewport.go:70` | `func (v *PTYViewport) View() string` | Keep as `string` (helper) |
| `settings_panel.go:375` | `func (sp *SettingsPanel) View() string` | Return string via `NewView()` |
| `result_panel.go:121` | `func (rp *ResultPanel) View() string` | Return string via `NewView()` |
| `model_picker.go:198` | `func (p *ModelPickerPanel) View() string` | Return string via `NewView()` |
| `option_picker.go:152` | `func (p *OptionPickerPanel) View() string` | Return string via `NewView()` |
| `fullscreen_panel.go:44` | `func (p *FullScreenPanel) View() string` | Return string via `NewView()` |

**Strategy**: Only the main `Model.View()` needs to return `tea.View`. Sub-components can stay as `string` returns and be composed.

#### [MODIFY] [model.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/model.go)

```diff
-func (m Model) View() string {
+func (m Model) View() tea.View {
+	var v tea.View
 	if !m.ready {
-		return "Initializing..."
+		v.SetContent("Initializing...")
+		return v
 	}
 
 	// Full-screen mode handling
 	if m.fullScreenMode && m.fullScreenPanel != nil {
-		return m.fullScreenPanel.View()
+		v.AltScreen = true
+		v.SetContent(m.fullScreenPanel.View())
+		return v
 	}
 	
 	// ... rest of view logic ...
 	
-	return overlayView
+	v.SetContent(overlayView)
+	return v
 }
```

---

### 1.4 Update KeyMsg → KeyPressMsg (50+ Locations)

`tea.KeyMsg` is replaced by `tea.KeyPressMsg` in v2.

**Files with KeyMsg usage:**
| File | Occurrences |
|------|-------------|
| `input.go` | ~15 |
| `input_test.go` | ~25 |
| `model.go` | 1 |
| `palette.go` | 1 |
| `sidebar.go` | 2 |
| `settings_panel.go` | 1 |
| `settings_panel_test.go` | ~20 |
| `model_picker.go` | 1 |
| `model_picker_test.go` | ~8 |
| `option_picker.go` | 1 |
| `option_picker_test.go` | 3 |
| `result_panel.go` | 1 |

**Change Pattern:**
```diff
-case tea.KeyMsg:
+case tea.KeyPressMsg:
```

---

### 1.5 Update bubbles/viewport Import

#### [MODIFY] [viewport.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/viewport.go)

```diff
-import "github.com/charmbracelet/bubbles/viewport"
+import "charm.land/bubbles/v2/viewport"
```

---

## Phase 2: Extract Style System

**Estimated Effort: 1-2 hours**

### 2.1 Create Centralized Theme

#### [NEW] [theme.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/styles/theme.go)

```go
package styles

import "charm.land/lipgloss/v2"

// Color Palette
var (
    Primary   = lipgloss.Color("#7C3AED") // Purple
    Secondary = lipgloss.Color("#06B6D4") // Cyan
    Success   = lipgloss.Color("#10B981") // Green
    Warning   = lipgloss.Color("#F59E0B") // Amber
    Error     = lipgloss.Color("#EF4444") // Red
    Muted     = lipgloss.Color("#6B7280") // Gray
    
    Background = lipgloss.Color("#1F2937")
    Surface    = lipgloss.Color("#374151")
    Text       = lipgloss.Color("#F9FAFB")
    TextMuted  = lipgloss.Color("#9CA3AF")
)

// Component Styles
var (
    Border = lipgloss.NewStyle().
        BorderStyle(lipgloss.RoundedBorder()).
        BorderForeground(Primary)
        
    Panel = Border.Copy().
        Padding(1, 2)
        
    StatusBar = lipgloss.NewStyle().
        Background(Surface).
        Foreground(Text)
        
    Overlay = lipgloss.NewStyle().
        Background(Background).
        BorderStyle(lipgloss.RoundedBorder()).
        BorderForeground(Secondary)
)
```

---

### 2.2 Migrate Inline Styles

Update files using inline lipgloss styles to use centralized theme:
- `palette.go` - uses inline border/padding styles
- `result_panel.go` - uses inline styles
- `settings_panel.go` - uses inline styles
- `sidebar.go` - uses inline styles
- `statusbar_view.go` - uses inline styles

---

## Phase 3: Component Refactoring

**Estimated Effort: 1 week (optional, can be deferred)**

### 3.1 Proposed Directory Structure

```
pkg/ui/
├── components/
│   ├── palette/
│   │   ├── palette.go
│   │   └── palette_test.go
│   ├── sidebar/
│   │   ├── sidebar.go
│   │   └── sidebar_test.go
│   ├── settings/
│   │   ├── panel.go
│   │   └── panel_test.go
│   └── viewport/
│       ├── viewport.go
│       └── viewport_test.go
├── styles/
│   └── theme.go
├── terminal/
│   ├── altscreen.go
│   └── modes.go
├── model.go (slimmed down)
└── input.go
```

> [!NOTE]
> This refactoring is optional and can be done incrementally after v2 migration is stable.

---

## Phase 4: Enhanced Testing

**Estimated Effort: 1-2 days**

### 4.1 Add Golden Tests

#### [NEW] [model_golden_test.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/model_golden_test.go)

Add snapshot tests for UI output stability:

```go
func TestModelViewGolden(t *testing.T) {
    m := NewTestModel()
    m.width = 80
    m.height = 24
    m.ready = true
    
    view := m.View()
    golden.RequireEqual(t, []byte(view.String()))
}
```

---

## Verification Plan

### Automated Tests

**Existing test suite (25 files):**
```bash
make test
# or
go test -v ./...
```

**After v2 migration, all 25 test files must pass:**
- `pkg/ui/input_test.go` - Tests KeyMsg handling
- `pkg/ui/model_test.go` - Tests Model lifecycle
- `pkg/ui/settings_panel_test.go` - Tests settings navigation
- ... (22 more)

### Manual Verification

After migration, verify these flows work correctly:

1. **Basic Launch**
   ```bash
   make build && ./wtf_cli
   ```
   - Expect: Terminal opens with welcome message

2. **Command Palette**
   - Press `/` at empty prompt
   - Expect: Palette overlay appears

3. **Full-Screen Mode (vim)**
   ```bash
   # Inside wtf_cli
   vim test.txt
   ```
   - Expect: vim opens in alternate screen, no artifacts

4. **Settings Panel**
   - Open palette → select `/settings`
   - Expect: Settings panel renders correctly

5. **Resize Handling**
   - Resize terminal window while running
   - Expect: No flicker, content reflows correctly

---

## Execution Order

| Step | Phase | Task | Est. Time |
|------|-------|------|-----------|
| 1 | 1.1 | Update go.mod dependencies | 15 min |
| 2 | 1.2 | Update import paths (sed script) | 30 min |
| 3 | 1.4 | Replace KeyMsg → KeyPressMsg | 1-2 hours |
| 4 | 1.3 | Update Model.View() signature | 2-3 hours |
| 5 | 1.5 | Update bubbles/viewport import | 15 min |
| 6 | - | Run tests, fix breakages | 2-4 hours |
| 7 | 2.1 | Create styles/theme.go | 30 min |
| 8 | 2.2 | Migrate inline styles | 1 hour |
| 9 | 4.1 | Add golden tests (optional) | 2-4 hours |

**Total Phase 1 + 2: ~2-3 days**

---

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| v2 is still RC | Pin to specific version, monitor releases |
| bubbles/v2 API changes | Review charm.land/bubbles/v2 docs |
| Test failures | Fix incrementally, use git branches |
| viewport behavior changes | Compare v1 vs v2 behavior |

---

## Decision Points for User

1. **Proceed with v2 RC?** - It's pre-release but used by Crush in production
2. **Phase 3 refactoring now or later?** - Can defer component restructuring
3. **Golden tests with what framework?** - Can use `charmbracelet/x/exp/golden` or simple file comparison
