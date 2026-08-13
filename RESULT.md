# Result: Tabbed layout for admin screen

## What Was Done
Converted the admin settings screen from three stacked accordion sections
(Sites, Users, Events) into a tabbed interface with three tabs. Only one tab's
content is visible at a time. All existing functionality preserved.

## Changes
- `r3-intake/internal/assets/public/index.html` (`{{define "admin"}}` block):
  - Stylesheet link cache-buster bumped `?v=4` -> `?v=5`.
  - Three `.accordion` wrappers replaced with a single `.tabs[data-tabs]`
    container: `.tab-list` with Sites/Users/Events tab buttons (ARIA
    `role=tablist`/`tab`, `aria-selected`, `aria-controls`, `tabindex`) and
    `.tab-panel` panels. Inner tables/forms copied verbatim.
  - `{{if .IsAdmin}}` gating preserved around Users/Events tabs + panels;
    Sites always rendered (non-admin sees only Sites).
  - Inline vanilla-JS script before `</body>`: click-delegated `activate(id)`
    toggling `aria-selected`/`tabindex`/`hidden`, plus defaulting to the
    Events tab on load when `EventError` or `EventName` is truthy (so a failed
    event POST shows the error + entered values).
- `r3-intake/internal/assets/public/app.css`:
  - Added `.tabs`, `.tab-list`, `.tab`, `.tab[aria-selected="true"]`,
    `.tab-panel`, `.tab-panel[hidden]` rules (accent `#b5502e` active
    underline, `#fffdfa` panel, 14px bottom radii). Accordion rules left
    intact (harmless).

## Scope
- Event form markup preserved verbatim (state/end dates, create button,
  description box) - that restructuring is the sibling card t_8a12cbf2's job.
- No Go handler changes required - `AdminView` already carried all fields.

## Verification
- `go build ./...` - clean
- `go vet ./...` - no issues
- `go test ./...` - `internal/server` passes (template-parsing tests confirm
  the admin block is valid Go template syntax)
- omp rendered the admin template through the in-process-PocketBase test
  harness asserting: `data-tabs` present, three tabs with correct
  `aria-selected`, Users/Events panels start `hidden`, Sites active by default;
  non-admin output contains Sites but no Users/Events tabs/panels; event-error
  view renders the script's `(true || true)` branch.

## Artifacts
- Plan: `docs/plans/omp-plan-tabbed-admin-layout.md`
