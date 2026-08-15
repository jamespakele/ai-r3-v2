# RESULT: Change participant list Site column to Event column

## What was built
Changed the admin participant list table in `r3-intake/internal/assets/public/index.html`:
- Header cell (line 587): `<th>Site</th>` -> `<th>Event</th>`
- Row cell (line 594): `<td>{{.SiteName}}</td>` -> `<td>{{.EventName}}</td>`

## Dependency handling
The `IntakeRow.EventName` field + population live in sibling worktree
`t_27c37c36` (uncommitted). Copied the sibling's changed Go source files
(admin.go, handlers.go, attendance.go, person_attendance.go, mcp.go,
migrations.go, 016_intake_site_to_event.go, and 4 integration test files) into
this worktree so the template reference resolves. One documented deviation:
attendance page line 1421 `{{.SiteName}}` -> `{{.EventName}}`, required because
the copied sibling `PersonAttendanceView` removed `SiteName`.

## Verification
- `go build ./...` PASS
- `go vet ./...` PASS
- `go test ./...` PASS (server 16.6s, migrations 0.22s)
- Grep confirms `<th>Event</th>` (1 match, line 587) and `{{.EventName}}` (line 594)
- Collateral Site refs intact: `{{$ev.SiteName}}` (events table, line 787),
  `{{.SiteName}}` (line 1171), event-form site dropdown untouched

## Artifacts
- Plan: `docs/plans/omp-plan-participant-list-site-to-event.md`
