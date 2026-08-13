# RESULT: Restructure event form layout

## What was built
Restructured the "Create Event" form on the admin screen from a 4-column grid
(`form-grid-4`) to a 2-column grid (`form-grid-2`) to use screen real estate
better. All three requirements met:

1. **Start and End dates side by side** — both date inputs now occupy one
   column each on the same row (previously each took a full line).
2. **Create Event button below the description box** — moved to its own
   full-width row beneath the textarea (previously on the same row).
3. **Description box bigger** — now spans the full 2-column width (same width
   as the event name textbox) and bumped from `rows="3"` to `rows="5"`.

## Files changed
- `r3-intake/internal/assets/public/index.html` — event form markup: class
  `form-grid-4` → `form-grid-2`; name + location get `grid-full` (own rows);
  dates unspanned (side by side); textarea `rows=5`; button `grid-full`.
- `r3-intake/internal/assets/public/app.css` — replaced `.form-grid-4` block
  with `.form-grid-2` (`grid-template-columns: 1fr 1fr`), preserved
  `.form-error`, kept the 620px single-column responsive fallback.

## Backend contract preserved
- `method="post"`, `action="/admin/events"`, and all field `name` attributes
  (`name`, `site`, `start_date`, `end_date`, `description`) unchanged.
- No Go code touched. No new dependencies.

## Verification
- `go build ./...` → exit 0
- `go vet ./...` → clean
- `go test ./...` → `ok r3-intake/internal/server` (3.571s), all packages pass
- `grep form-grid-4` → no remaining references
- Template render test `TestAdminEventsRender` PASS (asserts action + Create Event)

## Notes
- `make build` fails in this worktree because the Makefile targets
  `./cmd/r3-intake` which does not exist here (pre-existing condition, not
  caused by this change). `go build ./...` confirms all packages compile.
- Live-server responsive/functional checks can't run in this isolated worktree;
  covered by the template-render test + direct source inspection.

## Commit
`7cac38f` Restructure event form layout: 2-col grid, dates side by side, button below description, larger description box
