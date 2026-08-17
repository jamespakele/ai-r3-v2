# RESULT — Fix event filter logic in loadExportRows (TestExportCSVEventFilter)

## What was built

Fixed `handleExportCSV` in `r3-intake/internal/server/attendance.go` so the CSV export
auto-scopes its date range to the selected event's `start_date` -> `end_date` when the
caller supplies an event filter but no explicit valid `from`/`to` dates. This makes
`TestExportCSVEventFilter` (specifically the `ev1_only` subtest) pass.

## Root cause

`TestExportCSVEventFilter/ev1_only` calls `doExport(srv, admin, "?event="+fx.ev1)` with no
explicit dates. `handleExportCSV` fell back to the default 14-day window `[today-13, today]`
= `[2026-08-04, 2026-08-17]` when run on 2026-08-17. The seeded ev1 attendance records
(2026-08-01, 2026-08-02) fall outside that window, so `loadExportRows`'s
`date>='from' && date<='to'` filter excluded all ev1 rows before the event filter was
considered. `ev2_only` passed only because ev2's records (08-02, 08-05, 08-10) happened to
fall inside the default window.

## Change (only handleExportCSV in attendance.go)

1. Added `explicitRange := errFrom == nil && errTo == nil` immediately after the two
   `time.Parse` calls, before the default fallback block.
2. Added an auto-scope block after `requireEventID` and before `s.loadExportRows`: when
   `!explicitRange`, reads the event record via `s.eventsCollection()` +
   `s.pb.FindRecordById`, and if `start_date`/`end_date` parse and `start <= end`, overrides
   `from`/`to` with the event span (no 30-day cap). Mirrors `parseMatrixFilters` behavior.

`loadExportRows`, `parseMatrixFilters`, templates, and all tests are unchanged.

## Verification

- `go test ./internal/server/ -run TestExportCSVEventFilter -v` — PASS (ev1_only, ev2_only)
- `go test ./internal/server/ -run 'TestExportCSV' -v` — all 13 tests PASS
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (server + migrations, no regressions)
- `grep -rE '^[<>=]{7}' attendance.go` — no conflict markers

## Files changed

- `r3-intake/internal/server/attendance.go` — handleExportCSV auto-scoping (18 insertions)
- `docs/plans/omp-plan-fix-export-event-filter.md` — working plan artifact
