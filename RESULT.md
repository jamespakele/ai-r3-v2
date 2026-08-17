# RESULT: Remove unused EventName resolution from IntakeRow

## What was built
Removed the dead `IntakeRow.EventName` field and its population in `handleList`
(admin.go), and removed the now-orphaned Event column from the Records participant
list table (index.html header + cell + empty-state colspan 8→7 / 7→6). Updated the
README note that referenced the removed column.

## Files changed
- `r3-intake/internal/server/admin.go` — removed `EventName` field from `IntakeRow`
  struct and the `row.EventName = s.nameFor("events", ...)` population block.
- `r3-intake/internal/assets/public/index.html` — removed `<th>Event</th>` header,
  `<td>{{.EventName}}</td>` cell, and adjusted empty-state colspan.
- `r3-intake/README.md` — corrected the now-false "Event column still shows..." note.

## Logical consequences honored
- `nameFor` helper: KEPT (used by attendance.go, person_attendance.go, tests).
- Event filter dropdown: KEPT (single way to scope list to an event).
- `AdminView.EventName` (create-event form, tab activation): KEPT.
- Report/Attendance view `.EventName` usages (index.html 1317, 1388): KEPT.

## Verification
- `go build ./...` PASS
- `go vet ./...` PASS
- `go test ./internal/server/ -run 'TestList' -count=1` PASS (renders modified template)
- `go test ./... -count=1` — only failure is `TestExportCSVEventFilter/ev1_only`,
  confirmed PRE-EXISTING on clean HEAD (reproduced in a temp worktree at 7b3c152);
  exercises the CSV export path (`AttendanceExportRow.EventName`), untouched here.

## Plan
`docs/plans/omp-plan-remove-unused-eventname.md`
