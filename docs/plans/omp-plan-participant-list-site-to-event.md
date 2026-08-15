# Working Plan: Change participant list Site column to Event column

## Objective
In the admin participant list table, replace the "Site" column header with an
"Event" column header, and render each row's cell from `{{.EventName}}` instead
of `{{.SiteName}}`. This is a template-only change; the Go view model has
already been migrated by a sibling task.

## Constraints
- **Template-only change.** Only edit the admin participant list table region in
  `r3-intake/internal/assets/public/index.html`. Do NOT touch any Go code.
- **Do NOT touch other Site references.** These are unrelated and must stay:
  - The events table's `{{$ev.SiteName}}` (line ~787).
  - The attendance page's `{{.SiteName}}` (line ~1421).
  - The event form's site dropdown.
- **Dependency on sibling migration.** The `IntakeRow.EventName` field and its
  population (`s.nameFor("events", rec.GetString("event"))` with `"—"` fallback)
  live in the sibling worktree `t_27c37c36` and are NOT yet merged into this
  branch. This card must be implemented/merged only after that migration lands,
  otherwise the template will reference a non-existent field and `go build` /
  template execution will fail. Do not re-implement the Go migration here.
- Preserve the exact surrounding markup (checkbox column, `{{if .IsAdmin}}`
  guard, `{{range .Rows}}` loop) — only the header text and the row cell change.

## File Structure
- Modify: `r3-intake/internal/assets/public/index.html`
  - Line 587 (header row): change `<th>Site</th>` to `<th>Event</th>`.
  - Line 594 (row cell): change `<td>{{.SiteName}}</td>` to
    `<td>{{.EventName}}</td>`.
- No other files change.

## Implementation Notes
1. **Header cell (line 587).** In the `<thead>` row, replace the literal text
   `Site` with `Event`:
   ```
   <th>Participant</th><th>SSN</th><th>Event</th><th>Status</th>...
   ```
   Keep the column position identical (between SSN and Status).
2. **Row cell (line 594).** Inside the `{{range .Rows}}` loop, replace
   `{{.SiteName}}` with `{{.EventName}}`:
   ```
   <td>{{.EventName}}</td>
   ```
   `EventName` is a string on `IntakeRow`; the sibling migration already
   populates it via `s.nameFor("events", rec.GetString("event"))` and falls back
   to `"—"` when empty, so no extra template guard is needed.
3. **Scope discipline.** After editing, grep the file to confirm the ONLY
   changed Site references are these two. `{{$ev.SiteName}}` (events table),
   `{{.SiteName}}` (attendance page), and the event-form site dropdown must
   remain untouched.
4. **No CSS/cache-buster change.** This is a text/field swap only; no stylesheet
   change, so no `?v=` bump is required.

## Verification Criteria
1. `cd r3-intake && go build ./...` — compiles cleanly (requires the sibling
   EventName migration to be present).
2. `cd r3-intake && go vet ./...` — no vet warnings.
3. `cd r3-intake && go test ./...` — all tests pass.
4. Grep confirms the two intended edits and no collateral changes:
   - `grep -n '<th>Event</th>' r3-intake/internal/assets/public/index.html`
     matches the participant list header.
   - `grep -n '{{.EventName}}' r3-intake/internal/assets/public/index.html`
     matches the participant list row cell.
   - `grep -n '{{$ev.SiteName}}' r3-intake/internal/assets/public/index.html`
     still present (events table, unchanged).
   - `grep -n '{{.SiteName}}' r3-intake/internal/assets/public/index.html`
     still present (attendance page, unchanged).
5. `git diff --stat` shows exactly one modified file
   (`r3-intake/internal/assets/public/index.html`) with a 2-line diff.

## ADDENDUM: sibling dependency merge (required before template edit)

The `IntakeRow.EventName` field and its population live in the sibling worktree
`/srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_27c37c36` as UNCOMMITTED
changes (working-tree modifications, not committed). They are NOT in this branch.

Before editing the template, copy the sibling's changed source files into this
worktree so the template's `{{.EventName}}` reference resolves and build/vet/test
pass. Copy these files from the sibling worktree (preserving paths):

- r3-intake/internal/server/admin.go
- r3-intake/internal/server/handlers.go
- r3-intake/internal/server/attendance.go
- r3-intake/internal/server/person_attendance.go
- r3-intake/internal/mcp/mcp.go
- r3-intake/pocketbase/migrations/migrations.go
- r3-intake/pocketbase/migrations/016_intake_site_to_event.go
- r3-intake/internal/server/attendance_export_integration_test.go
- r3-intake/internal/server/attendance_roster_integration_test.go
- r3-intake/internal/server/attendance_toggle_integration_test.go
- r3-intake/internal/server/person_attendance_integration_test.go
- r3-intake/internal/server/person_attendance_test.go

Do NOT copy RESULT.md or the sibling's docs/plans. Do NOT copy the sibling's
index.html (this card owns the participant-list column edit in index.html; the
sibling's index.html change is a different region and the parent will reconcile).

Then apply the two template edits (header `Site`→`Event`, cell `{{.SiteName}}`→`{{.EventName}}`)
and run build/vet/test.
