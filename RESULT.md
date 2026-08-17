# RESULT — Remove the Event column from the participant list table

## What was built
Removed the "Event" column from the Records participant list table and the now-dead
`IntakeRow.EventName` field/population.

## Changes
- `r3-intake/internal/assets/public/index.html`
  - Removed `<th>Event</th>` header (was line 597)
  - Removed `<td>{{.EventName}}</td>` row cell (was line 604)
  - Updated empty-state colspan 8→7 (admin) / 7→6 (non-admin) (line 616)
- `r3-intake/internal/server/admin.go`
  - Removed `EventName string` from `IntakeRow` struct (was line 23)
  - Removed `row.EventName = s.nameFor("events", ...)` + `"—"` fallback block (was lines 162-164)

## Logical consequences handled
- Event filter dropdown (`<select name="event">`, index.html:587) — KEPT (single filter mechanism)
- `nameFor` helper — KEPT (still used by attendance.go, person_attendance.go, admin.go)
- `AdminView.EventName` + event form/report/attendance `.EventName` usages — KEPT (unrelated views)
- No colgroup/`<col>` existed — nothing to clean up
- No test referenced `IntakeRow.EventName` — no test changes needed

## Verification
- `go build ./...` → PASS
- `go vet ./...` → PASS
- `go test ./internal/server/...` → only failure is `TestExportCSVEventFilter/ev1_only`,
  confirmed PRE-EXISTING on clean HEAD (git stash) and unrelated to this change (CSV export
  attendance path, tracked by sibling card t_b0e9ba0f)
- grep confirms no `<th>Event</th>` / `<td>{{.EventName}}</td>` remain in the admin list table
- Filter dropdown intact

## Artifacts
- Plan: `docs/plans/omp-plan-remove-event-column.md`
