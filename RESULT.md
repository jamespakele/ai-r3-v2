# RESULT — Calendar Matrix View Grid + HTMX Cell Toggle (Stories 1.2 + 1.3)

## What was built
- **`r3-intake/internal/server/attendance.go`** (replaced skeleton): full matrix implementation.
  - `MatrixViewData`, `MatrixRow`, `MatrixCell` structs.
  - `handleMatrix`: parses/validates `from`/`to` (defaults to last 14 days HST), caps range at 30 days, resolves site per role, builds date columns, loads rows + attendance map, renders `matrix` template.
  - `resolveSite`: admin picks any active site or "All locations" (`""`); case_manager pinned to site derived from most-common `assigned_to` intake (fallback first active site).
  - `loadMatrixRows`: participants filtered by role/site; attendance map `intakeID→date→status`; computes `PresentCount`, `LastPresent`, `IsDropout` (last present >13 days before view end).
  - `cycleStatus`: `""→present→absent→excused→walk_in→""`.
  - `handleToggle`: POST-only; validates fields/dates; 403 for case_manager on non-assigned intake; finds existing record (Go-enforced uniqueness on `(intake,date)` / `(event,intake,date)`), cycles, creates/updates/deletes; returns `matrix-cell` fragment on `HX-Request`, else 303 redirect preserving filters (no-JS fallback).
- **`r3-intake/internal/server/server.go`**: added `mux.HandleFunc("/attendance/toggle", s.requireAuth(s.handleToggle))`.
- **`r3-intake/internal/assets/public/index.html`**: replaced `matrix` placeholder with filter bar (admin site select / case_manager static label, from/to date inputs, Apply), sticky-name matrix table with MM/DD date headers, per-cell `matrix-cell` partial, `PresentCount/TotalDays` badge, dropout STOPPED badge, empty state; added `{{define "matrix-cell"}}`; bumped stylesheet to `?v=4`.
- **`r3-intake/internal/assets/public/app.css`**: matrix CSS — sticky name col, status dots (present #3f6b34, absent #eee, excused #8a6a1e, walk_in #2a4d8a, empty transparent), dropout highlight, total badge.

## Verification
- `go build ./...` → BUILD_OK
- `go vet ./...` → VET_OK
- `go test ./...` → all packages pass
- Template parse + render verified with throwaway test (removed after): `matrix` + `matrix-cell` defines register; sample view renders name, total badge `1/2`, `dot-present`, `dot-empty`, sticky name col, date headers `07/28`/`07/29`, "All locations".

## Notes
- Working plan artifact: `docs/plans/omp-plan-attendance-matrix-grid-toggle.md`.
- The worktree has no `cmd/r3-intake` main package (pre-existing partial-checkout state, also true of the parent foundation), so a live-server smoke test isn't possible here; manual UI checks remain for an environment with the entry point present.
- Scope discipline: walk-in check-in, summary stat cards, CSV export, per-person calendar, and events CRUD belong to downstream cards.
