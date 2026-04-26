# Feature Implementation Plan: System Clock in Status Bar

**Feature Goal:** Implement a persistent, real-time system clock display in the application status bar, providing users with the current date and time within `wtf_cli`.

**Target File:** The primary logic changes will occur in `pkg/ui/model.go`, interacting directly with the `statusbar` component located in `pkg/ui/components/statusbar/`.

## 🗺️ Task Progress Checklist
- [ ] Analyze existing code base and status bar component structure. (Completed)
- [ ] Draft the comprehensive implementation plan (Markdown format). (In progress - This step)
- [ ] Write the plan to docs/feature_doc/implementation_plan.md. (To be executed by this tool use)

---

## 💡 Architectural Overview

The system clock feature requires integrating a periodic time source into the Bubble Tea (ELM) update loop, ensuring that the status bar view is updated every second without blocking user interaction or consuming excessive resources.

**Key Components Affected:**
1.  `pkg/ui/model.go`: The main Model struct and its `Init()` / `Update()` methods will be responsible for scheduling and handling the periodic time tick message.
2.  `pkg/ui/components/statusbar/status_bar_view.go` (and related files): This component must accept the current date/time string and use it to render the status bar segment dedicated to the clock.

## ⚙️ Detailed Implementation Steps

### Phase 1: Scheduling the Time Tick (`pkg/ui/model.go`)

1.  **Define Message:** Introduce a new message type (e.g., `TimeTickMsg`) that signals the passage of time.
2.  **Schedule Ticker:** In the `Model.Init()` function, add a periodic tick using `tea.Tick(time.Second, func(_ time.Time) tea.Msg { return TimeTickMsg{} })`. This ensures the clock updates reliably every second, similar to how directory updates are currently handled (`tickDirectory`).
3.  **Update Logic:** Modify the `Model.Update()` method to handle `TimeTickMsg`. When received:
    *   Calculate the current time (using Go's standard library `time` package).
    *   Call a new method on the `StatusBarView` component (e.g., `SetClockTime(t time.Time)`) passing the formatted time. This ensures the status bar view updates its internal state and triggers a re-render.

### Phase 2: Implementing Time Formatting & Update (`pkg/ui/components/statusbar/*`)

1.  **Refactor Status Bar State:** Determine where the clock logic should reside. If the `StatusBarView` component already manages complex status information (e.g., Git branch, prompt), it needs an internal state field for the formatted time string (e.g., `ClockTime string`).
2.  **Implement Clock Setter:** Create a dedicated public method on the status bar view (or its underlying model/view) to accept and store the formatted time (`SetClockTime(t time.Time)`).
3.  **Formatting:** Use Go's formatting utilities (`time.Format`) to ensure consistency (e.g., "Mon Jan 02 15:04:05 PDT").

### Phase 3: Integration and Cleanup

1.  **Layout Adjustment:** Review the main layout logic in `Model.Update()` to ensure that when the clock time is set, it does not conflict with other status bar elements (like exit messages or prompt context).
2.  **Testing:** Update unit tests for `pkg/ui/components/statusbar/*` to verify correct rendering of the clock component under various conditions.

## 🧪 Testing Strategy

- **Unit Tests:** Write specific tests for `time.Format()` usage and the state changes within `StatusBarView`.
- **Integration Tests:** Verify that receiving a `TimeTickMsg` in `Model.Update()` correctly updates the status bar view and triggers a re-render of the entire UI without causing excessive CPU usage (i.e., ensure the tick is efficient).

## 🚧 Potential Challenges & Mitigation

*   **Challenge:** Clock updating might interfere with other periodic tasks (e.g., directory checks, network polling).
    *   **Mitigation:** Ensure `TimeTickMsg` handling is isolated and low-overhead, utilizing Bubble Tea's inherent concurrency model for state updates.
*   **Challenge:** Timezone management could be complex across different operating systems/terminal environments.
    *   **Mitigation:** Use the system time provided by Go's standard library which typically respects environment variables (like `TZ`) or local machine settings, documenting this behavior clearly in user guides.