# RESULT — Modify Records event filter to join attendance

## What was built
The Records screen (`handleList` in `r3-intake/internal/server/admin.go`) event filter
now joins the `attendance` collection. Filtering by an event surfaces intakes that
**attended** that event (per attendance records), not just those whose `intake.event`
(home event) matches.

**Semantics (union):** an intake matches the event filter if its home event equals the
selected event **OR** it has an attendance record for that event
(`attendance.intake == intake.id`). All attendance statuses count. This satisfies the
verification scenario (people with attendance for R3 - Sprng 2027 appear even though
their intake.event is R3 - Fall 2026) while avoiding an empty-screen regression for
events with zero attendance records (home-event fallback).

## Files changed
- `r3-intake/internal/server/admin.go` — event filter block replaced with the
  attendance-join union. Queries `attendance` for `event='<id>'`, collects distinct
  non-empty `intake` IDs, builds `(event='<id>' || id='<id1>' || id='<id2>' || ...)`,
  falls back to `event='<id>'` on attendance query error / no records. Composes with
  `?status=`/`?q=` via the existing ` && ` join. `view.EventFilter` unchanged.
- `r3-intake/internal/server/records_list_integration_test.go` (new) —
  `TestListEventFilterJoinsAttendance`, 4 subtests covering: cross-home-event attendance
  join, home-event match, union ∩ status filter, and zero-attendance fallback.

## Deviation from plan
Plan proposed `id in ('...')`; PocketBase v0.39 filter syntax has no `in` operator, so
omp used OR-joined `id='<id>'` clauses — consistent with the codebase's own precedent
(`attendance.go` OR-joins `event='<id>'`). Union semantics unchanged.

## Verification
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (server 16.0s, migrations ok; export/roster/person-attendance
  tests unaffected)
- `TestListEventFilterJoinsAttendance` — 4/4 subtests PASS
- Template (`index.html`) untouched; select/Clear/empty-state/count driven by unchanged
  `EventFilter`/`Total`.

## Note
The real-data scenario (`?event=lu9ohnxysl9pccf` → 4 people) can't run in this
worktree (no real PB data dir), but the integration test reproduces that exact data
shape (home event ≠ attended event) and proves the union behavior.
