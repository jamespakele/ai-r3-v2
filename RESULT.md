# RESULT — Epic 1: Daily Attendance Tracking (Calendar Matrix)

All five stories implemented and integrated on branch `epic/1-daily-attendance-tracking-calendar-matri`.

## Stories
- **1.1** Database migration `007_events_attendance.js` (events, event_enrollment, attendance; null rules, cascade deletes, nullable attendance.event) + Attendance tab + Go skeleton.
- **1.2** Calendar Matrix grid: sticky participant column, MM/DD date headers, Total present-count badge, filter bar (site/from/to, 30-day cap, 14-day HST default, role-based site scoping).
- **1.3** HTMX cell toggle: 5-state cycle (empty→present→absent→excused→walk_in→empty), Go-enforced uniqueness, recorded_by + HST check_in_time, matrix-cell fragment swap, no-JS 303 fallback.
- **1.4** Walk-in check-in (HTMX name search + select-existing or create-minimal-intake + idempotent walk_in upsert) + dropout highlighting (AC colors #fbeeec / #8f3a2e).
- **1.5** Four summary stat cards (Total check-ins, Active, Stopped, Avg rate) computed from the same rows as the grid, program/event filter dropdown (site-scoped), HTMX dynamic updates via #matrix-and-stats partial.

## Integration
Four parallel child worktrees (one per story) were merged into this epic branch. Conflicts in
`attendance.go`, `server.go`, `index.html`, `app.css` reconciled (disjoint-intent combined; superseded
placeholder dropped; walk-in + stats both preserved).

## Verification
- `go build ./...` — OK
- `go vet ./...` — OK
- `go test ./...` — all pass (incl. TestComputeSummary, TestMatrixContentRender)
- Template parse+render verified for matrix, matrix-content, matrix-cell, stat-cards, walkin-results

## Files
- `r3-intake/pocketbase/migrations/007_events_attendance.js` (new)
- `r3-intake/internal/server/attendance.go` (new)
- `r3-intake/internal/server/attendance_test.go` (new)
- `r3-intake/internal/server/server.go` (routes)
- `r3-intake/internal/assets/public/index.html` (templates)
- `r3-intake/internal/assets/public/app.css` (grid, dots, sticky column, stat cards, walk-in)
