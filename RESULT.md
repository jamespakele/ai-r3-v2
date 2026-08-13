<<<<<<< HEAD
# RESULT - Walk-in Check-in & Dropout Visual Highlighting (Story 1.4)

## What was built
- **r3-intake/internal/server/attendance.go** - added two handlers:
  - handleWalkinSearch (GET /attendance/walkin-search): HTMX name search (name ~ "q", min 2 chars, max 10 results), scoped to the resolved site for case managers, returns walkin-results fragment carrying filter context (site_id/from/to/event_id).
  - handleWalkin (POST /attendance/walkin): accepts either an existing intake_id or a name to create a minimal intake (name + site + created_by + status=unassigned), then idempotently upserts a walk_in attendance record for today (HST) at the resolved site (recorded_by + check_in_time), 303-redirects back to the matrix preserving filters. No duplicate attendance record on re-submit.
  - Added walkinResult struct.
- **r3-intake/internal/server/server.go** - registered /attendance/walkin-search and /attendance/walkin (both requireAuth).
- **r3-intake/internal/assets/public/index.html** - added "Add walk-in" panel (toggle button + search input with hx-get/hx-include + create-name-only form) to the matrix define, and a new walkin-results define.
- **r3-intake/internal/assets/public/app.css** - dropout row bg #fdf2f1 -> #fbeeec, scoped .matrix-table .status-badge (10px / #8f3a2e / #fbeeec / 4px radius), and walk-in panel/result/create styles.

## Verification
- go build ./... -> BUILD_OK
- go vet ./... -> VET_OK
- go test ./... -> all packages pass
- Template parse + render verified with throwaway test (removed after): matrix renders "Add walk-in" button + dot-walk_in; walkin-results renders name + intake_id.

## Notes
- Working plan artifact: docs/plans/omp-plan-walkin-checkin-dropout-highlighting.md.
- Dropout rule (AC 8) preserved: only participants with >=1 present record whose last present is >14 days before view end are flagged (row.LastPresent != "" && row.LastPresent < thresholdStr); zero-attendance participants are NOT flagged.
- Deviations from literal plan (reflected in plan file): handleWalkin/handleWalkinSearch use resolveSite(u, form site_id) instead of resolveSite(u, "") so an admin with a selected site resolves correctly; search input named name with hx-include forwarding filter context; search reads event_id query param.
- The worktree has no cmd/r3-intake main package (pre-existing partial-checkout state), so a live-server smoke test isn't possible here; manual UI checks remain for an environment with the entry point present.
=======
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
>>>>>>> wt/t_85043a61
