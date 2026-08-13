# Epic 6: Clicking a participant in the Attendance list should navigate to their record — IDENTIFICATION RESULT

**Status:** IDENTIFICATION COMPLETE — this card (t_ee889325) identifies the attendance list component and the participant record route. The implementation child card (t_c0b8b3b4) consumes this report to make rows clickable.

## 1. The attendance list component

The "Attendance list" is the **attendance matrix** screen, rendered by the `matrix-content` template block.

- **Template:** `r3-intake/internal/assets/public/index.html`, `{{define "matrix-content"}}` (lines ~850–937)
- **Handler:** `r3-intake/internal/server/attendance.go`, `handleMatrix` (line 76)
- **Route:** `GET /attendance` (auth-only) — registered in `r3-intake/internal/server/server.go` line 136
- **Row data struct:** `MatrixRow` in `attendance.go` (line 37) — carries `IntakeID` (the participant's intake record id) and `Name`.

### The participant row markup (the clickable target)

```html
{{range .Rows}}
<tr class="{{if .IsDropout}}row-dropout {{end}}{{if .NoLocation}}row-no-location{{end}}">
  <td class="matrix-name-col">
    <span class="matrix-name">{{.Name}}</span>
    {{if .IsDropout}}<span class="status-badge">STOPPED</span>{{end}}
    {{if .NoLocation}}<span class="matrix-no-location-note">no location</span>{{end}}
  </td>
  {{range .Cells}}<td>{{template "matrix-cell" .}}</td>{{end}}
  <td class="matrix-total-col"><span class="matrix-total-badge">{{.PresentCount}}/{{.TotalDays}}</span></td>
</tr>
{{end}}
```

- The row is a plain `<tr>` — **no link, no click handler** today.
- The participant's record id is available in the template as `{{.IntakeID}}` on each `MatrixRow`.
- CSS hooks already exist: `.matrix-name` (line 316 of `app.css`), `.matrix-name-col` (line 288), `.matrix-table tr.row-dropout` / `.row-no-location` (lines 299–301).

## 2. The participant record route

The participant's full record (intake form) is served at:

- **Route:** `GET /intake/{id}` — registered in `server.go` line 100 (`mux.HandleFunc("/intake/", s.handleIntakeCmd)`)
- **Handler:** `handleIntakeCmd` in `r3-intake/internal/server/handlers.go` (line 403). For `GET /intake/{id}` it loads the record via `findIntake(id)` and renders the full form (`stateFromRecord`).
- **Auth:** The intake view route is NOT wrapped in `requireAuth` — it renders for any session (the form itself is the public intake form; the attendance matrix is auth-only). The matrix is auth-only, so navigation from it is fine.
- **Existing precedent:** The walk-in flow already navigates to `/intake/{id}` after creating a participant (`handlers.go` lines 385/389/399), and Epic 4's per-participant attendance history uses `/intake/{id}/attendance`.

## 3. Implementation guidance for the child card (t_c0b8b3b4)

To make participant rows clickable, the implementer should:

1. **Wrap the participant name** in the `matrix-content` template with a link to `/intake/{{.IntakeID}}`:
   ```html
   <td class="matrix-name-col">
     <a class="matrix-name-link" href="/intake/{{.IntakeID}}">{{.Name}}</a>
     ...
   </td>
   ```
2. **Add a hover cue** in `app.css` (e.g. `.matrix-name-link:hover { text-decoration: underline; color: #b5502e; }` — accent color per design system).
3. **Consider making the whole row clickable** (JS click handler on the `<tr>` navigating to `/intake/{{.IntakeID}}`), while keeping the name link as the accessible fallback. Ensure the row click does NOT interfere with the per-cell attendance toggle forms (`matrix-cell` forms post to `/attendance/toggle`).
4. **Do not** break the `matrix-cell` toggle forms or the walk-in panel.

## Verification criteria

- `go build ./... && go vet ./... && go test ./...` pass.
- Clicking a participant name/row in the attendance matrix navigates to `GET /intake/{id}` showing that participant's full record.
- Hovering a participant name shows a clear clickability cue.
- Attendance dot toggles still work (row click must not swallow the cell form clicks).
