# RESULT — CSV Attendance Export Handler (Story 3.1)

## What was built
Implemented the CSV attendance export endpoint (FR14, UX-DR5) via omp-plan-execute.

- **Route:** `GET /attendance/export` → `handleExportCSV`, registered in `server.go` with `requireRole("admin", ...)` (admin-only per PRD §10; overrides §09 generic "auth").
- **Handler** (`attendance.go`): parses `?event={id}&site={id}&from=YYYY-MM-DD&to=YYYY-MM-DD` with the same defaults as the matrix (last 14 days, resolved site, no event filter), 30-day cap, inverted-range swap. Sets `Content-Type: text/csv` and `Content-Disposition: attachment; filename="attendance_export_YYYY-MM-DD.csv"` (HST today). Streams via `encoding/csv` with Flush/Error checks.
- **`loadExportRows`:** queries raw `attendance` records (NOT `loadMatrixRows`, which drops recorded_by/check_in_time/note), filters by date/site/event with `mcpmod.EscapeFilter`, resolves names via `nameFor`.
- **Pure builders (testable):** `exportCSVRecords` (header + data rows + summary row), `exportStatus` (title-case), `summaryCSVRow` (total check-ins, unique participants, avg rate).
- **Columns:** Participant, Site, Event, Date, Status, Recorded By, Check-in Time, Note (PRD §05 Screen 2).

## Verification
- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass (incl. new `TestExportCSVRecords` covering header, status title-casing, empty relations, comma-in-note, summary math)

## Artifacts
- Plan: `docs/plans/omp-plan-csv-export.md`
- Changed: `r3-intake/internal/server/attendance.go`, `server.go`, `attendance_test.go`

## Handoff
Sibling test card `t_7c6efa05` (unit/integration tests) is released by this completion. The pure `exportCSVRecords`/`summaryCSVRow` functions are the intended test surface.
