# RESULT — Add regression test for plain-form CSRF token injection

Task: t_6607a21e (child story of epic t_614f6db0)

## What was built
Added `r3-intake/internal/server/csrf_plainform_test.go` (package `server`) with
two regression tests guarding the CSRF middleware's **plain-form fallback** path:

- `TestPlainFormCSRFViaFormField` — POSTs `/logout` carrying the `r3_csrf` cookie
  and the token ONLY as a `csrf_token` form field (no `X-CSRF-Token` header).
  Asserts `303 See Other` — proving a plain HTML form (Add Event, sites/users,
  notes, walk-in, intake finish/cancel) passes CSRF without HTMX.
- `TestPlainFormCSRFMissingRejected` — POSTs `/logout` with the cookie but neither
  header nor form field. Asserts `403` + middleware error body.

This is the regression guard for the JS snippet added in sibling story t_33247512
(which copies the `r3_csrf` cookie into a hidden `csrf_token` input on plain
method=post forms). If the middleware ever regresses to require the header, or the
form-field fallback breaks, these tests fail loudly.

## Artifacts
- `r3-intake/internal/server/csrf_plainform_test.go` (new)
- `docs/plans/omp-plan-plainform-csrf-regression.md` (MOA working plan)
- This `RESULT.md`

## Verification
From `r3-intake/`:
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (full suite green, incl. both new tests)
- `go test -run 'TestPlainFormCSRF' -v ./internal/server/` — 2/2 PASS
  (303 via form field, 403 when missing)

No production source changed (auth.go, server.go untouched) — test-only addition.
