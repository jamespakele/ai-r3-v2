# RESULT — Epic 29: Records event filter shows only attendees + Event column removal + CSV export fix

## Goal

The Records screen event filter must show only intakes that actually attended
the selected event (attendance is the source of truth), the participant list
must no longer show a misleading per-row Event column, and the CSV export must
auto-scope to the selected event's dates. All five child stories merged into
this epic branch with no feature dropped.

## What was built

### 1. Strict attendance-only Records event filter (t_85d4bc7b)
`handleList` in `r3-intake/internal/server/admin.go` now returns ONLY intakes
with an attendance record for the selected event whose `date` falls within the
event's `start_date` -> `end_date` range. Home-event matching
(`intake.event = selected event`) was removed entirely; no home-event fallback.
The filter is built as OR-joined `(id='<id1>' || id='<id2>' || ...)` clauses
(PocketBase v0.39 has no `in` operator) and composes with the
`?status=`/`?q=` filters via ` && `. If the event record can't be loaded or no
in-range attendance records exist, the filter degrades to `id=''` (no matches).

### 2. Event column removed from participant list (t_16d5e06d / t_4eecdb6e)
- `IntakeRow.EventName` field and its population in `handleList` removed.
- "Event" column removed from the Records participant list table
  (`<th>Event</th>` header + `<td>{{.EventName}}</td>` cell); empty-state
  colspan updated 8→7 (admin) / 7→6 (non-admin).
- Remaining `EventName` references belong to other view models (AdminView,
  EventRow, report/attendance views) and are correctly preserved.
- `nameFor` helper kept (still used by attendance.go, person_attendance.go,
  admin.go).

### 3. CSV export auto-scopes to event dates (t_b0e9ba0f)
`handleExportCSV` in `r3-intake/internal/server/attendance.go` now sets
`explicitRange := errFrom == nil && errTo == nil` after parsing `from`/`to`,
and when the caller supplies an event filter but no explicit valid range,
auto-scopes `from`/`to` to the selected event's `start_date` -> `end_date`
(full span, no 30-day cap), mirroring `parseMatrixFilters`. This fixes
`TestExportCSVEventFilter/ev1_only`, which previously failed because the
default 14-day window excluded the seeded ev1 attendance dates.

### 4. Data model and end-to-end verification (t_daf94842)
- `intake.event` = home/primary event; `attendance` = per-event attendance
  (unique index `idx_attendance_event_intake_date`); `events` carries
  `start_date`/`end_date` for date-range scoping. Attendance is the source of
  truth for who attended an event.
- Applied the strict filter + date-range scoping to real test data; counts
  match the card's expectations:

| Event | Expected | Verified (in-range attendance) |
|---|---|---|
| R3 - Sprng 2027 | 4 people | Cancel Screenshot Gamma, James Pakele, John, John Doe ✓ |
| R3 - Fall 2026 | 3 people | James Pakele, John, John Doe ✓ |
| R3 - Fall 2026 Waianae | 4 people | James Pakele, John, John Doe, John Smith ✓ |

## Files changed

- `r3-intake/internal/server/admin.go` — strict attendance-only event filter
  with date-range scoping; removed `IntakeRow.EventName` + population.
- `r3-intake/internal/server/attendance.go` — `handleExportCSV` auto-scopes
  to event dates when no explicit range is given.
- `r3-intake/internal/assets/public/index.html` — removed Event column
  (header + cell), empty-state colspan 8→7 / 7→6.
- `r3-intake/internal/server/records_list_integration_test.go` — updated to
  strict attendance-only expectations.
- `r3-intake/internal/server/records_list_attendance_join_integration_test.go`
  — updated to strict expectations; added `TestListEventFilterConstrainsByDateRange`.
- `r3-intake/README.md` — Records event filter documented as strict
  attendance-only; Event column removal noted.
- `docs/plans/omp-plan-remove-event-column.md`,
  `docs/plans/omp-plan-remove-unused-eventname.md`,
  `docs/plans/omp-plan-strict-attendance-filter.md`,
  `docs/plans/omp-plan-fix-export-event-filter.md` — child plan artifacts.

## Verification

- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./internal/server/ -run 'TestListEventFilter|TestListNoEventFilter|TestListEventFilterConstrainsByDateRange' -count=1` — PASS (8 tests)
- `go test ./internal/server/ -run TestExportCSVEventFilter -count=1` — PASS
  (both `ev1_only` and `ev2_only`; no longer a pre-existing failure)
- `go test ./...` — PASS
- `grep -rE '^[<>=]{7}' r3-intake/ RESULT.md docs/plans/` — no output
  (no conflict markers)
- `git diff --check` — clean

## Conclusion

All five child stories merged into the epic branch with every feature set
intact: strict attendance-only Records filter with date-range scoping, Event
column / `IntakeRow.EventName` removal, CSV export event-date auto-scoping,
and data-model verification. Build, vet, and full test suite pass.
