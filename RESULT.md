# RESULT — t_85043a61: Summary Stats Cards and Attendance Filters (Story 1.5)

## What was built
Implemented via omp-plan-execute (MOA plan -> omp --plan-yolo --advisor):

1. Four summary stat cards below the matrix (flex row, 2x2 wrap at 375px):
   - Total check-ins (green #3f6b34) = present + walk_in records in range
   - Active participants (accent #b5502e) = rows with >=1 present
   - Stopped (red #8f3a2e) = rows flagged as dropouts (>14 days since last present)
   - Avg attendance rate (yellow #8a6a1e) = totalPresent / (participants x days), integer-division, division-by-zero guarded
2. Program/event filter dropdown in the filter bar (All dates - no event filter + active events, site-scoped for case managers via loadEvents(siteID)).
3. HTMX dynamic updates: extracted matrix-content partial wrapped in div#matrix-and-stats; filter form uses hx-get=/attendance hx-target=#matrix-and-stats hx-swap=outerHTML hx-trigger=change,submit. handleMatrix branches on HX-Request to render the partial (HTMX) or full page (no-JS fallback).

## Files changed
- r3-intake/internal/server/attendance.go — MatrixSummary, WalkInCount on MatrixRow, computeSummary(), Event struct, loadEvents(), Events+Summary on MatrixViewData, HTMX branch in handleMatrix
- r3-intake/internal/assets/public/index.html — matrix-content + stat-cards partials, event dropdown, HTMX attrs
- r3-intake/internal/assets/public/app.css — .stat-cards/.stat-card/.stat-number/.stat-label + color classes
- r3-intake/internal/server/attendance_test.go — TestComputeSummary (4 cases) + TestMatrixContentRender

## Verification
- go build ./... — PASS
- go vet ./... — PASS
- go test ./... — PASS (TestComputeSummary + TestMatrixContentRender green)
- Template parse+render of matrix-content + stat-cards verified via TestMatrixContentRender

## Notes
- Stats computed from the same rows that render the grid -> cards always match the matrix.
- Toggle-driven stat refresh (after a cell toggle) is out of scope for this card (filter-driven updates only).
- Plan artifact: docs/plans/omp-plan-summary-stats-attendance-filters.md
