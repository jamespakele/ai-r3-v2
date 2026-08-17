# Working Plan: Remove the Event column from the participant list table

## Objective
Remove the "Event" column from the Records participant list table in `r3-intake/internal/assets/public/index.html`. A participant may attend more than one event, so a single per-record "Event" cell is misleading. The event filter dropdown remains the single way to see who attended which events. Also remove the now-dead `EventName` field from the `IntakeRow` struct and its population in `admin.go` to keep the codebase clean.

## Constraints
- **Do NOT modify** the event filter dropdown (lines 588–589: `<select name="event" class="field-input">` … `</select>`). It is the intended single filter mechanism.
- Do not change any other column's header, cell, or CSS class.
- No `colgroup`/`<col>` exists in this table (grep confirmed), so no column-width cleanup is needed.
- The `colspan` on the empty-state row is **mandatory** to update (it counts columns).
- Do not write code — this is a plan only.

## File Structure
- `r3-intake/internal/assets/public/index.html` — the table markup (header, row cell, empty-state colspan).
- `r3-intake/internal/server/admin.go` — `IntakeRow` struct (line ~20) and its `EventName` population (lines 162–164).
- No test files require changes (see Logical Consequences).

## Implementation Notes

### 1. `index.html` — table header (line 597)
Remove the `<th>Event</th>` element from the `<thead>` row. The header becomes:
```
<thead><tr>{{if .IsAdmin}}<th style="width:28px"><input type="checkbox" title="Select all" onclick="document.querySelectorAll('.bulk-check').forEach(c=>c.checked=this.checked)"></th>{{end}}<th>Participant</th><th>SSN</th><th>Status</th><th>Assigned</th><th>Created</th><th></th></tr></thead>
```

### 2. `index.html` — row cell (line 604)
Remove the entire line `<td>{{.EventName}}</td>`. This is the **only** place `IntakeRow.EventName` is rendered in any template.

### 3. `index.html` — empty-state colspan (line 617)
The empty-state row currently uses `colspan="{{if .IsAdmin}}8{{else}}7{{end}}"`. Removing one column makes this **7 for admin, 6 for non-admin**:
```
<tr><td colspan="{{if .IsAdmin}}7{{else}}6{{end}}" class="empty-state">...
```

### 4. `admin.go` — remove dead `EventName` field
- Remove `EventName string` from the `IntakeRow` struct (line 23).
- Remove the population block (lines 162–164):
  ```go
  row.EventName = s.nameFor("events", rec.GetString("event"))
  if row.EventName == "" {
      row.EventName = "—"
  }
  ```
  This also removes the now-unused `s.nameFor("events", ...)` call in this handler. Verify `nameFor` is still used elsewhere (it is — attendance.go, person_attendance.go, admin.go lines 496/581/668) so the helper itself stays.

## Logical Consequences
Trace every downstream site that references the removed concept (the Event column / `IntakeRow.EventName`):

| Site | Location | Decision |
|------|----------|----------|
| `<th>Event</th>` header | index.html:597 | **Remove** (the column header) |
| `<td>{{.EventName}}</td>` row cell | index.html:604 | **Remove** (the only render of `IntakeRow.EventName`) |
| Empty-state `colspan` | index.html:617 | **Change** 8→7 (admin), 7→6 (non-admin) — mandatory |
| Event filter dropdown | index.html:588–589 | **Keep** — explicitly out of scope |
| `IntakeRow.EventName` struct field | admin.go:23 | **Remove** — dead after cell removal |
| `row.EventName = s.nameFor(...)` + fallback | admin.go:162–164 | **Remove** — dead population code |
| `s.nameFor("events", ...)` helper call | admin.go:162 | **Remove** the call; helper itself **Keep** (used by attendance.go, person_attendance.go, admin.go:496/581/668) |
| `AdminView.EventName` field | admin.go:41 | **Keep** — separate field for the event form/report, unrelated to the column |
| `EventName` in `admin_events_test.go` (lines 77, 95) | test | **Keep** — these set `AdminView.EventName` (event form/report), not `IntakeRow`; unaffected |
| `EventName` in attendance.go / person_attendance.go / their tests | — | **Keep** — separate attendance view models, unrelated to the admin list column |
| `colgroup` / `<col>` width refs | index.html | **None exist** — nothing to clean up |

No Go test references `IntakeRow.EventName` or renders the admin list `Rows` (grep of `*_test.go` for `IntakeRow`/`.Rows`/admin-list rendering returned no matches). Therefore **no test changes are required**.

## Verification Criteria
1. **Column gone:** The participant list table no longer renders an Event column — no `<th>Event</th>` and no `<td>{{.EventName}}</td>` remain in `index.html`.
2. **Renders correctly:** The table header and every row have the same number of cells (admin: 7 columns; non-admin: 6 columns), and the empty-state `colspan` matches (7/6).
3. **No dead markup:** `grep -n "EventName" r3-intake/internal/assets/public/index.html` returns only the unrelated event-form/report usages (lines 739, 871, 1317, 1388) — none in the admin list table.
4. **No dead Go code:** `grep -n "EventName" r3-intake/internal/server/admin.go` returns only `AdminView.EventName` (line 41) and the event-form/report assignments (496, 581, 668) — no `IntakeRow.EventName` and no `row.EventName` population.
5. **Build passes:** `go build ./...` in `r3-intake/` succeeds (no unused-field/struct errors).
6. **Tests pass:** `go test ./internal/server/...` passes — no test referenced the removed field, so no test edits were needed.
7. **Filter dropdown intact:** The `<select name="event">` dropdown (lines 588–589) is unchanged.
