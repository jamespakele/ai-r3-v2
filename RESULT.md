# RESULT — Inject csrf_token hidden field into plain POST forms via JS

Task: t_33247512 (child story of epic t_614f6db0)

## What was built
Added a tiny self-contained non-HTMX JS snippet to all 10 full-page template defines in r3-intake/internal/assets/public/index.html (page, login, list, admin, notes, note-history, matrix, event-manage, event-report, person-attendance).

The snippet reads the double-submit r3_csrf cookie and injects a hidden input name="csrf_token" into every plain form method="post" that: has method="post" (skips GET forms); has NO hx-post attribute (HTMX forms are already covered by the X-CSRF-Token header); and does NOT already contain a csrf_token input (idempotent; protects the login form server-rendered hidden field from duplication).

It runs on DOMContentLoaded (with a readyState guard) and also listens to htmx:afterProcessNode so forms injected by HTMX swaps (walk-in results, person-attendance-day) get the field too.

## Files changed
- r3-intake/internal/assets/public/index.html (only file; 321 insertions across the 10 defines)

## Verification
- go build ./... : PASS
- go vet ./... : PASS
- go test ./... : PASS (r3-intake/internal/server ok)
- No Go template tokens introduced inside any snippet; embedded template still parses.
- 10 </head> and 10 htmx:afterProcessNode occurrences: one snippet per full-page define.

## Notes / deferred verification
- This worktree is forked from committed master and does NOT contain the parent uncommitted CSRF middleware (cookie r3_csrf, csrf_token field, X-CSRF-Token header), so end-to-end 403 to success checks must run on the merged epic branch where the middleware exists. The JS here is the exact field-injection half the middleware form-field fallback consumes (cookie r3_csrf to input name=csrf_token).
- Regression test for this JS path is the SIBLING card t_6607a21e (out of scope here).
