# Epic 6: Clicking a participant in the Attendance list should navigate to their record — RESULT

**Status:** COMPLETE — child story branches `wt/t_ee889325` (identification) and `wt/t_c0b8b3b4` (implementation) merged into `epic/6-clicking-a-participant-in-the-attendance`.

## Stories implemented

- **6.1 Identification** — `wt/t_ee889325` — identified the attendance list component and the participant record route.
- **6.2 Implementation** — `wt/t_c0b8b3b4` — made participant rows clickable with navigation and hover cue.

## Identification (from `wt/t_ee889325`)

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

- The row was a plain `<tr>` — **no link, no click handler** before this epic.
- The participant's record id is available in the template as `{{.IntakeID}}` on each `MatrixRow`.
- CSS hooks already exist: `.matrix-name`, `.matrix-name-col`, `.matrix-table tr.row-dropout` / `.row-no-location`.

### The participant record route

The participant's full record (intake form) is served at:

- **Route:** `GET /intake/{id}` — registered in `server.go` (`mux.HandleFunc("/intake/", s.handleIntakeCmd)`)
- **Handler:** `handleIntakeCmd` in `r3-intake/internal/server/handlers.go`. For `GET /intake/{id}` it loads the record via `findIntake(id)` and renders the full form (`stateFromRecord`).
- **Auth:** The intake view route is NOT wrapped in `requireAuth` — it renders for any session (the form itself is the public intake form; the attendance matrix is auth-only). The matrix is auth-only, so navigation from it is fine.
- **Existing precedent:** The walk-in flow already navigates to `/intake/{id}` after creating a participant, and Epic 4's per-participant attendance history uses `/intake/{id}/attendance`.

## Implementation (from `wt/t_c0b8b3b4`)

Made each participant row in the attendance matrix (GET /attendance, `matrix-content` template) navigate to that participant's record page at `GET /intake/{id}` when clicked, with a clear hover cue — without breaking the per-cell attendance toggle forms.

### Changes

- `r3-intake/internal/assets/public/index.html` (matrix-content block, ~line 917)
  - Wrapped the participant name in a real link: `{{if .IntakeID}}<a href="/intake/{{.IntakeID}}" class="matrix-name-link"><span class="matrix-name">{{.Name}}</span></a>{{else}}<span class="matrix-name">{{.Name}}</span>{{end}}`.
  - `{{if .IntakeID}}` guard falls back to plain text if a row ever lacks an ID.
  - `.status-badge` (STOPPED) and `.matrix-no-location-note` remain outside the link.
- `r3-intake/internal/assets/public/app.css`
  - `.matrix-name-link` — inherits name color, no underline, `cursor: pointer`; `:hover` → accent `#b5502e` + underline; `:focus-visible` → accent outline (keyboard-accessible).
  - Row hover tint variants (respect existing row-state backgrounds):
    - normal rows `#f3ead9`, dropout `#f6ddd9`, no-location `#f5ecd8`, dropout+no-location `#f6ddd9`.
    - These apply to the sticky `.matrix-name-col` td too, so no horizontal seam on scroll.

### Design decisions

- **Name link is the primary affordance** (not a whole-row JS click handler). It is a genuine `<a href>` — works without JS, keyboard-focusable, server-rendered so it survives HTMX re-renders, and structurally cannot interfere with the per-cell `.matrix-dot` toggle forms. The plan's optional whole-row click handler was skipped (no `app.js` exists; the name-link satisfies the ACs).
- **No Go changes required** — `MatrixRow.IntakeID` and the `GET /intake/{id}` route already existed.

## Verification criteria

- `go build ./... && go vet ./... && go test ./...` pass.
- Clicking a participant name/row in the attendance matrix navigates to `GET /intake/{id}` showing that participant's full record.
- Hovering a participant name shows a clear clickability cue.
- Attendance dot toggles still work (row click must not swallow the cell form clicks).

## Verification (run after merge)

- `go build ./...` — clean
- `go vet ./...` — no issues
- `go test ./...` — `internal/server` passes; tests parse the full embedded template, confirming template syntax is valid

## Artifacts

- Plan: `docs/plans/omp-plan-clickable-matrix-rows.md`
- Plan (hermes): `.hermes/plans/2026-08-13_clickable-matrix-rows.md`
