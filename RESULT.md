# Story: Implement clickable participant rows with navigation and hover cue — RESULT

**Status:** COMPLETE — implemented in worktree `wt/t_c0b8b3b4` (child story card; parent epic merges).

## What was built

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

## Verification

- `go build ./...` — clean
- `go vet ./...` — no issues
- `go test ./...` — `internal/server` passes (3.6s); tests parse the full embedded template, confirming template syntax is valid
- Diff inspected: 2 files, 8 insertions / 1 deletion, matches plan exactly

## Artifacts

- Plan: `docs/plans/omp-plan-clickable-matrix-rows.md`
