# RESULT — CSV Attendance Export: Handler + Tests (Story 3.1)

## What was built
Implemented the CSV attendance export endpoint (FR14, UX-DR5) and added unit +
integration tests covering it end-to-end. Two child cards were merged:
`wt/t_54a1f19a` (implementation) and `wt/t_7c6efa05` (tests).

### Implementation — `internal/server/attendance.go`
- **Route:** `GET /attendance/export` → `handleExportCSV`, registered in `server.go` with `requireRole("admin", ...)` (admin-only per PRD §10; overrides §09 generic "auth").
- **Handler** (`attendance.go`): parses `?event={id}&site={id}&from=YYYY-MM-DD&to=YYYY-MM-DD` with the same defaults as the matrix (last 14 days, resolved site, no event filter), 30-day cap, inverted-range swap. Sets `Content-Type: text/csv` and `Content-Disposition: attachment; filename="attendance_export_YYYY-MM-DD.csv"` (HST today). Streams via `encoding/csv` with Flush/Error checks.
- **`loadExportRows`:** queries raw `attendance` records (NOT `loadMatrixRows`, which drops recorded_by/check_in_time/note), filters by date/site/event with `mcpmod.EscapeFilter`, resolves names via `nameFor`.
- **Pure builders (testable):** `exportCSVRecords` (header + data rows + summary row), `exportStatus` (title-case), `summaryCSVRow` (total check-ins, unique participants, avg rate).
- **Columns:** Participant, Site, Event, Date, Status, Recorded By, Check-in Time, Note (PRD §05 Screen 2).

### Tests
#### Pure unit tests — `internal/server/attendance_export_test.go`
- `TestExportStatus` — status → title-case mapping (present/absent/excused/walk_in), unknown/empty → "".
- `TestSummaryCSVRow` — check-in counting (present+walk_in), unique participants, avg-rate math, empty-set no-division-by-zero, trailing empty cells.
- `TestExportCSVRecords_Header` — exact header row equality (order-sensitive).
- `TestExportCSVRecords_FieldFormatting` — title-cased status, verbatim check_in_time/note/recorded_by pass-through, comma-in-note quoting.
- `TestExportCSVRecords_Empty` — empty input → header + empty summary row only.
- `TestExportCSVRecords` (in `attendance_test.go`) — header, status title-casing, empty relations, comma-in-note, summary math.

#### Integration tests — `internal/server/attendance_export_integration_test.go`
Boots a real in-process PocketBase (embedded JS migrations + Go stubs + pb.Bootstrap(), temp data dir) and drives the handler through `srv.Mux()` with signed session cookies:
- `TestExportCSVPermissions` — unauthenticated / case_manager / tampered-cookie → 303 redirect to /login; admin → 200 + CSV headers.
- `TestExportCSVDateRangeFilter` — ordered range, swapped range (from>to), 30-day cap, defaults.
- `TestExportCSVSiteFilter` — site1/site2/invalid-site (all locations)/no-site.
- `TestExportCSVEventFilter` — ev1/ev2/no-event.
- `TestExportCSVHeaderAndFormatting` — full-wire header, resolved names, summary row, Content-Type/Disposition.
- `TestExportCSVEmptyResultSet` — 200, header + empty summary row.
- `TestExportCSVNameResolution` — nameFor + loadExportRows against booted PB.

## Verification
- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — all pass (ok r3-intake/internal/server), including `TestExportCSVRecords` and the 12 new tests.

## Artifacts
- Plans: `docs/plans/omp-plan-csv-export.md`, `docs/plans/omp-plan-csv-export-tests.md`
- Changed: `r3-intake/internal/server/attendance.go`, `server.go`, `attendance_test.go`, `attendance_export_test.go`, `attendance_export_integration_test.go`

## Handoff
The pure `exportCSVRecords`/`summaryCSVRow`/`exportStatus` functions are the intended test surface. No production files were dropped during the merge; both child cards' feature sets are preserved.
