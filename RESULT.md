# RESULT — Verify data model and end-to-end behavior against test data

## Task
t_daf94842 — Verify the data model and end-to-end behavior of the Records
event filter (strict attendance-only) and the removed Event column against
the test data.

## What was verified

### 1. Data model relationships (schema)
- `intake.event` = the person's home/primary event (relation to `events`).
- `attendance` = the per-event attendance relationship, linking
  `intake_id` + `event_id` + `date` (unique index `idx_attendance_event_intake_date`).
- `events` carries `start_date`/`end_date` used for date-range scoping.
- Confirmed: attendance is the source of truth for who attended an event;
  `intake.event` is the home event and must NOT drive the Records filter.

### 2. Strict attendance-only filter (sibling t_85d4bc7b)
`handleList` event filter now returns ONLY intakes with an attendance record
for the selected event whose date falls within the event's
`start_date..end_date` range. Home-event matching removed entirely; no
home-event fallback. Verified in merged state: all 8 event-filter tests pass
(`TestListEventFilter*`, `TestListNoEventFilter*`,
`TestListEventFilterConstrainsByDateRange`).

### 3. Event column removed (sibling t_16d5e06d / t_4eecdb6e)
- `IntakeRow.EventName` field removed from struct + population in `handleList`.
- "Event" column removed from participant list table (header + cell);
  empty-state colspan 8→7 / 7→6.
- Remaining `EventName` references belong to other view models (AdminView,
  EventRow) and are correctly preserved.
- Verified: merged epic builds and vets clean.

### 4. End-to-end behavior against test data (`/tmp/r3-testdata`)
Applied the strict attendance-only filter + date-range scoping to the real
test data and confirmed the card's expected counts:

| Event | Expected | Verified (in-range attendance) |
|---|---|---|
| R3 - Sprng 2027 | 4 people | Cancel Screenshot Gamma, James Pakele, John, John Doe ✓ |
| R3 - Fall 2026 | 3 people | James Pakele, John, John Doe ✓ |
| R3 - Fall 2026 Waianae | 4 people | James Pakele, John, John Doe, John Smith ✓ |

All three match exactly. The Records filter and the matrix auto-scoping
(attendance date within event start_date..end_date) agree on who attended
each event.

## Verification commands
- `go build ./...` — PASS (merged epic)
- `go vet ./...` — PASS (merged epic)
- `go test ./internal/server/ -run 'TestListEventFilter|TestListNoEventFilter|TestListEventFilterConstrainsByDateRange'` — PASS (8 tests)
- `go test ./...` — PASS except pre-existing `TestExportCSVEventFilter/ev1_only`
  (time-dependent; confirmed failing at clean baseline HEAD, not a regression)

## Conclusion
Data model is correct, sibling changes merge cleanly (disjoint regions of
admin.go), and end-to-end behavior matches the card's expected counts for all
three events. Epic is coherent and ready for parent merge.
