# Epic 11: CSRF 403 on plain HTML POST forms

**Status:** IN PROGRESS — merging child story branches.

## Stories implemented

- **t_33247512** — Inject `csrf_token` hidden field into plain POST forms via JS.
  Adds a self-contained vanilla-JS snippet to all 10 full-page template defines in
  `r3-intake/internal/assets/public/index.html`. The snippet copies the
  `r3_csrf` cookie into a hidden `input[name="csrf_token"]` on every plain
  `method="post"` form that does not already carry one, while skipping
  `hx-post` forms (those continue to use the `X-CSRF-Token` header). It also
  listens for `htmx:afterProcessNode` so dynamically swapped fragments such as
  walk-in results and `person-attendance-day` get the field too.

## Files changed

- `r3-intake/internal/assets/public/index.html` — combined HTMX header helper
  and plain-form `csrf_token` injection in all 10 full-page defines.
- `docs/plans/omp-plan-csrf-form-field-injection.md` — working plan for the JS
  injection story.

## Verification

- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass
- Conflict-marker sweep — none
