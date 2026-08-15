# RESULT — Reorder admin tabs to put Events first

## What was built
Reordered the Admin screen's tab list so **Events** is the first tab (before Sites),
and made **Events** the default active tab so the Admin screen opens on the Events tab.

Single-file change: `r3-intake/internal/assets/public/index.html` (74 insertions, 73 deletions).

- Tab buttons: `{{if .IsAdmin}}` → Events (`aria-selected="true"`, no tabindex) → Users
  (`aria-selected="false"`, tabindex="-1") → `{{end}}` → Sites (`aria-selected="false"`, tabindex="-1").
- Panels: `{{if .IsAdmin}}` → `panel-events` (no `hidden`) → `panel-users` (`hidden`) → `{{end}}`
  → `panel-sites` (`hidden`).
- All IDs, `aria-controls`/`aria-labelledby`/`data-tab-target` relationships preserved.
- JS untouched — `activate()` is order-agnostic (derives state from `aria-controls`/`hidden` by ID),
  so tab switching, `EventError`/`EventName` auto-activate, and `?tab=` restore all still work.
- Admin guard intact: non-admin users see only the Sites tab.

## Verification
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./internal/server/ -run TestAdmin` — PASS (all admin tests, incl. TestAdminEventsRender)
- Full `go test ./...` — one failure, `TestExportCSVSiteFilter/site1_only` (row count 3, want 4),
  confirmed **pre-existing and unrelated** (fails identically on pristine HEAD; the export handler
  never executes the template).

## Artifacts
- `docs/plans/omp-plan-reorder-admin-tabs-events-first.md` — MOA working plan
- `WORKING_PLAN_reorder-admin-tabs-events-first.md` — plan copy at worktree root
