# RESULT — Epic 4: Participant Attendance History

## What was built
- Backend endpoints and handlers for per-participant monthly attendance and day-level edits.
  - **GET /intake/{id}/attendance** — full-page monthly calendar render (defaults to current HST month, `?month=YYYY-MM` nav), per-person stats (X of Y days (Z%), rate color, current streak), legend data.
  - **GET /intake/{id}/attendance/day?date=YYYY-MM-DD** — day-detail modal fragment (status, event name, recorded-by, check-in time, note) or empty state.
  - **POST /intake/{id}/attendance/day** — save/update attendance record (creates if none, updates existing; site derived from intake, recorded_by = current user).
  - **POST /intake/{id}/attendance/day/delete** — delete record for a date (idempotent).
- Vanilla-CSS styling layer for the per-participant monthly attendance calendar and attendance-stats UI.
  - Stats pills, nav/month, 7-column calendar table + cells, daynum/status labels, cell-state classes (is-other-month dim, is-today highlight, has-record), status colors scoped to .person-att-cell.status-* and .person-att-legend-item.status-* (present #3f6b34, absent #eee/#d9cbb6, excused #8a6a1e, walk_in #2a4d8a), legend swatches via ::before, day-detail inline card, day-actions, delete-form spacing, and a @media (max-width: 620px) responsive block.

## Files
- Created: `r3-intake/internal/server/person_attendance.go`, `person_attendance_test.go`, `person_attendance_integration_test.go`
- Modified: `r3-intake/internal/server/server.go` (3 routes), `r3-intake/internal/assets/public/index.html` (3 template stubs, stylesheet link bumped to `?v=7`), `r3-intake/internal/assets/public/app.css` (Person attendance calendar section)
- Plans: `docs/plans/omp-plan-person-attendance-backend.md`, `docs/plans/omp-plan-person-attendance-css.md`

## Verification
- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — full suite green (server package 3.22s)
- Unit tests: stats, streak, month grid, template render
- Integration tests: authz (admin/cm/403), month render, day GET, save create/update (no dup), delete, validation
- CSS brace balance verified; status colors match existing matrix dots; rules scoped to avoid leaking onto other .status-* elements.

## Handoff to sibling UI cards
View structs (`PersonAttendanceView`, `PersonDayCell`, `PersonStats`, `PersonDayDetailView`) and template names (`person-attendance`, `person-attendance-calendar`, `person-attendance-day`) are defined for t_cca37bee (calendar UI) and t_1030de8b (day-detail/edit UI) to consume. The merged person-attendance template references `?v=7` to match the new CSS.
