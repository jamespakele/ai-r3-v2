# Epic 11: CSRF 403 on plain HTML POST forms

**Status:** COMPLETE — child story branches merged into `epic/11-csrf-403-on-plainhtml-post-forms-adding`.

## Stories implemented

- **t_33247512** — Inject `csrf_token` hidden field into plain POST forms via JS.
  Adds a self-contained vanilla-JS snippet to all 10 full-page template defines in
  `r3-intake/internal/assets/public/index.html`. The snippet copies the
  `r3_csrf` cookie into a hidden `input[name="csrf_token"]` on every plain
  `method="post"` form that does not already carry one, while skipping
  `hx-post` forms (those continue to use the `X-CSRF-Token` header). It also
  listens for `htmx:afterProcessNode` so dynamically swapped fragments such as
  walk-in results and `person-attendance-day` get the field too.

- **t_6607a21e** — Add regression test for plain-form CSRF token injection.
  Adds `r3-intake/internal/server/csrf_plainform_test.go` with
  `TestPlainFormCSRFViaFormField` (cookie + form field only → 303) and
  `TestPlainFormCSRFMissingRejected` (cookie only → 403). Guards the middleware
  fallback used by the JS injection above.

## Files changed

- `r3-intake/internal/assets/public/index.html` — combined HTMX header helper
  and plain-form `csrf_token` injection in all 10 full-page defines.
- `r3-intake/internal/server/csrf_plainform_test.go` — new regression tests.
- `docs/plans/omp-plan-csrf-form-field-injection.md` — working plan for the JS
  injection story.
- `docs/plans/omp-plan-plainform-csrf-regression.md` — working plan for the
  regression-test story.

## Merge resolution notes

- `index.html` had a conflict in each full-page define because the child branch
  branched before the epic's HTMX `X-CSRF-Token` helper existed. Resolution kept
  the helper and added the plain-form injection logic to the same IIFE so both
  `hx-post` and plain `method="post"` forms are covered.
- Both children replaced `RESULT.md` with their own story-level summary; the file
  was synthesized into this Epic 11 document.

## Verification

- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass
- `go test -run 'TestPlainFormCSRF' -v ./internal/server/` — 2/2 PASS
- Conflict-marker sweep — none
