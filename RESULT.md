# RESULT — Epic 28: Records event filter joins attendance + tests + docs

## What was built

The Records screen event filter (`handleList` in `r3-intake/internal/server/admin.go`) now returns a union: an intake matches a selected event if its home event (`intake.event`) equals that event **OR** it has an attendance record for that event (`attendance.intake == intake.id`). All attendance statuses count.

## Files changed

- `r3-intake/internal/server/admin.go` — event filter block replaced with the attendance-join union.
- `r3-intake/internal/server/records_list_integration_test.go` (new) — 4 subtests covering cross-home-event attendance join, home-event-only match, union ∩ status filter, and zero-attendance fallback.
- `r3-intake/internal/server/records_list_attendance_join_integration_test.go` (new) — 6 tests covering deduplication, multiple attendees, search/status composition, cross-event distinctness, and unfiltered regression guard.
- `r3-intake/README.md` — added "Records event filter is a union" bullet under Notes / inferences.
- `WORKING_PLAN_join_attendance_event_filter.md`, `omp-plan-records-event-filter-join-attendance.md`, `WORKING_PLAN_records_filter_attendance.md`, `omp-plan-records-filter-attendance-tests.md` — plan artifacts from child branches.

## Implementation notes

- PocketBase v0.39 has no `in` operator, so the union is built as OR-joined `(event='<id>' || id='<id1>' || id='<id2>' || ...)` clauses, composing with `?status=`/`?q=` via the existing ` && ` join.
- Falls back to home-event-only matching if the attendance query fails or returns no records, avoiding an empty-screen regression.

## Verification

- `make vendor` — fetched missing `htmx.min.js`/`alpine.min.js` so `go:embed` resolves.
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (server package and migrations; no regressions)
- `grep -E '^[<>=]{7}' RESULT.md` — no output (no conflict markers)

## Note

Live-browser smoke test not run; the template-render and integration tests cover the new behavior. No `cmd/` changes.
