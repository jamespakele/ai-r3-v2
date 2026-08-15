# RESULT — Fix attendance matrix date handling and remove STOPPED heuristic

## What was built
All four changes implemented in one coherent pass by omp (plan: docs/plans/omp-plan-attendance-matrix-date-fix.md):

1. **Removed from/to date filter boxes + Apply button** from the matrix filter form. The event dropdown is now the single source of truth for the date range.
2. **Grid dates reflect the event's dates** — with the date inputs gone, `explicitRange` is always false and `parseMatrixFilters` auto-scopes to the effective event's start/end dates. Grid header, stat cards, and toggle/walk-in forms all use the event-scoped range.
3. **Year shown in date labels** — event dates label renders `Mar 1 – Apr 15, 2026` (via `formatEventStart`/`formatEventEnd`); grid headers render compact `3/1/26` (via `formatGridDate` + new `DatesLabel []string`).
4. **Removed the STOPPED dropout heuristic** — `IsDropout` field, the 13-day threshold block, `row.IsDropout` computation, `s.Stopped++` branch, `Stopped` summary field, `row-dropout` CSS rules, matrix-only `.status-badge` block, `.stat-red`, the STOPPED badge, and the Stopped stat card all removed. Matrix rows are uniform attendance ticks.

## Files changed
- r3-intake/internal/server/attendance.go
- r3-intake/internal/assets/public/index.html
- r3-intake/internal/assets/public/app.css
- r3-intake/internal/server/attendance_test.go
- r3-intake/internal/server/attendance_stats_integration_test.go
- r3-intake/internal/server/attendance_roster_integration_test.go

## Verification
- `go build ./... && go vet ./... && go test ./...` — all pass (server 16.9s).
- Grep for `IsDropout|row-dropout|.Stopped|thresholdStr|EventStartDate|EventEndDate|stat-red|STOPPED` — zero hits.
- Filter form shows only the event select (no date inputs, no Apply).
- Event dates label uses `EventStartLabel – EventEndLabel` (year on end).
- Grid header ranges `.DatesLabel` (M/D/YY).
- Walk-in search/create and toggle/walk-in forms keep hidden event-scoped `from`/`to`.
- Stat cards: Total check-ins, Active participants, Avg attendance rate only.

## Acceptance criteria
- [x] Selecting an event shows the grid with exactly that event's date span, years visible.
- [x] No date filter boxes or Apply button.
- [x] No STOPPED red highlighting or stat card.
- [x] Event is the single source of truth for the matrix date range.
- [x] Toggle/walk-in forms still carry the correct event-scoped date range.
