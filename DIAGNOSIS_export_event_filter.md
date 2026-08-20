# Diagnosis: CSV export event filter returns empty rows (TestExportCSVEventFilter)

## Task
t_fbfbb042 — Diagnose CSV export event filter empty rows

## Status
ROOT CAUSE IDENTIFIED. Fix already applied in this worktree (commit 82ef3f8)
and verified green. This document records the diagnosis and the applied fix.

## Root cause

`TestExportCSVEventFilter/ev1_only` calls `doExport(srv, admin, "?event="+fx.ev1)`
with NO explicit `from`/`to` query params. In `handleExportCSV`
(`r3-intake/internal/server/attendance.go`), when the caller omits a valid
date range, the handler fell back to a default 14-day window
`[today-13, today]` = `[2026-08-04, 2026-08-17]` (when run on 2026-08-17).

The seeded ev1 attendance records are dated **2026-08-01** and **2026-08-02** —
both OUTSIDE that default window. `loadExportRows` builds
`date>='from' && date<='to'` from those defaults, so ALL ev1 rows were excluded
by the date filter BEFORE the event filter was even considered. Result: zero
rows for ev1.

`ev2_only` passed only by coincidence: ev2's records (08-02, 08-05, 08-10)
happened to fall inside the default window on 2026-08-17.

## Affected lines

- `r3-intake/internal/server/attendance.go` — `handleExportCSV` (~line 766):
  the default-range fallback block (`if errFrom != nil || errTo != nil`) set
  `from`/`to` to the 14-day window with no awareness of the selected event.
- `r3-intake/internal/server/attendance.go` — `loadExportRows` (~line 840):
  correctly builds `date>='from' && date<='to'` + event OR-clause, but it can
  only filter on the `from`/`to` it is handed. It was NOT the bug — it was
  given a wrong (default) range.

## Fix applied (commit 82ef3f8)

Auto-scope the export date range to the selected event's `start_date` ->
`end_date` when the caller supplies an event filter but no explicit valid
`from`/`to`. Mirrors the existing `parseMatrixFilters` behavior.

1. Added `explicitRange := errFrom == nil && errTo == nil` right after the two
   `time.Parse` calls, before the default fallback block.
2. Added an auto-scope block after `requireEventID` and before
   `s.loadExportRows`: when `!explicitRange`, read the event record via
   `s.eventsCollection()` + `s.pb.FindRecordById`, and if `start_date`/`end_date`
   parse and `start <= end`, override `from`/`to` with the event span (no 30-day
   cap).

`loadExportRows`, `parseMatrixFilters`, templates, and all tests unchanged.

## Verification (current worktree state)

- `go test ./internal/server/ -run TestExportCSVEventFilter -v` — PASS
  (ev1_only, ev2_only)
- `go test ./internal/server/ -run 'TestExportCSV' -v` — all 13 tests PASS
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (server + migrations, no regressions)
