# Working Plan: Add regression test for plain-form CSRF token injection

## Objective — what to build

Add a Go regression test in the R3 Intake server package that proves the CSRF middleware's **plain-form fallback** works: a state-changing request carrying the CSRF token **only** as a form field named `csrf_token` — with **no** `X-CSRF-Token` header — passes the middleware and reaches the handler.

This guards the no-JS fallback path that the front-end JS snippet feeds (it copies the `r3_csrf` cookie into a hidden `input[name=csrf_token]` on every plain `method=post` form, skipping `hx-post` forms that use the header). The regression test protects plain HTML POSTs — Add Event, sites/users, notes, walk-in, intake finish/cancel — from future regressions that would break them with a `403`.

Two cases must be covered:
1. **Accepted**: `POST /logout` with the `r3_csrf` cookie + `csrf_token` form field, **no header** → `303 See Other`.
2. **Rejected**: `POST /logout` with the cookie but **no** form field and **no** header → `403`.

`/logout` is the cleanest route for this: it is wrapped in `csrfMiddleware` and its handler (`handleLogout`) only clears the session cookie and redirects — no auth dependency, no DB writes, no body parsing needed.

## Constraints — language, framework, dependencies

- **Language**: Go (server lives in `r3-intake/`).
- **Framework**: `net/http` `http.ServeMux`; PocketBase embedded for integration tests.
- **Existing helpers to reuse** (do not duplicate):
  - `testCsrfToken` const, `csrfCookieForTest()`, `addCSRFToRequest(req)` from `csrf_test.go`.
  - `newTestServer(t)` from `attendance_export_integration_test.go` (boots in-process PocketBase with migrations, returns `*Server`).
  - `srv.Mux().ServeHTTP(rec, req)` dispatch pattern from `attendance_toggle_integration_test.go`.
- **CSRF constants** (from `auth.go`): `csrfCookieName="r3_csrf"`, `csrfHeaderName="X-CSRF-Token"`, `csrfFormName="csrf_token"`.
- **No new external dependencies.**
- **Do not modify** production code (`auth.go`, `server.go`) — this is test-only.

## File Structure — files to create/modify

| File | Action |
|------|--------|
| `r3-intake/internal/server/csrf_plainform_test.go` | **Create** (new test file, package `server`) |
| `r3-intake/internal/server/auth.go` | No change (reference only) |
| `r3-intake/internal/server/csrf_test.go` | No change (reuse helpers) |

New test functions in the created file:
- `TestPlainFormCSRFViaFormField` — form-field-only token is accepted (303).
- `TestPlainFormCSRFMissingRejected` — neither header nor form field → 403.

## Implementation Notes — key design decisions, edge cases

**Route choice — `/logout`:**
- Registered as `s.csrfMiddleware(s.handleLogout)` in `server.go:118`.
- `handleLogout` clears the session cookie and issues `303 /login` — no auth, no DB, no body parse. It is the minimal, dependency-free state-changing route, so it cleanly isolates the middleware behavior under test.
- `POST /logout` is non-safe (not GET/HEAD/OPTIONS/TRACE), so the middleware runs the full token check.

**Test 1 — accepted path (`TestPlainFormCSRFViaFormField`):**
- Build `POST /logout` with a form-encoded body `csrf_token=***`.
- Set `Content-Type: application/x-www-form-urlencoded` so `r.PostFormValue`/`r.FormValue` populate from the body.
- Add the `r3_csrf` cookie via `csrfCookieForTest()`.
- **Deliberately do NOT set** `X-CSRF-Token` header.
- Dispatch through `srv.Mux().ServeHTTP(rec, req)`.
- Assert `rec.Code == http.StatusSeeOther` (303), proving the middleware accepted the form-field token and the handler ran.

**Test 2 — rejection path (`TestPlainFormCSRFMissingRejected`):**
- Same `POST /logout`, add only the `r3_csrf` cookie, **no** form field and **no** header.
- Assert `rec.Code == http.StatusForbidden` (403) and (optionally) that the body contains the middleware's error text `"invalid or missing csrf token"`.

**Design decision — cookie value and hmac comparison:**
- The middleware reads the token from the `r3_csrf` cookie, and compares via `hmac.Equal([]byte(token), []byte(got))`. Using the shared `testCsrfToken` for both cookie and form field guarantees an exact byte match, mirroring how the real cookie→hidden-input JS copy works.
- No need to set a session/admin cookie — `/logout` doesn't require one, keeping the test minimal.

**Edge cases / pitfalls:**
- **Content-Type must be set.** Without `application/x-www-form-urlencoded`, `r.PostFormValue` returns empty and the middleware falls back to `r.ParseForm()` + `r.FormValue` — which for a raw body also returns empty. Keep the header set to exercise the primary plain-form path (`r.PostFormValue`).
- **No body-parsing side effects**: since the assertion is only on the status code, the empty-body reject case is cheap and deterministic.
- **Header must NOT be present in test 1** — this is the entire point of the regression test; if the middleware regresses to require the header, test 1 fails loudly with a 403.
- **Order of assertions**: assert the positive case first (proves the fallback works); the negative case guards that a missing token is genuinely rejected (proves the test isn't vacuously passing).
- **`t.Helper()`** on any local helper function; follow existing test-file conventions (package `server`, `net/http` + `net/http/httptest` imports).

**Helper (optional, local to the new file):**
- A small `postLogout(srv, withForm bool)` helper reduces duplication between the two tests: builds the request, sets Content-Type, adds the cookie, conditionally appends the form field, dispatches, returns the recorder. Keep it file-local; do not widen `csrf_test.go` helpers.

## Verification Criteria — how to test correctness

Run from the `r3-intake` directory:

1. **`go build ./...`** — must succeed (no compile errors).
2. **`go vet ./...`** — must pass (no vet complaints).
3. **`go test ./...`** — full suite must pass, including the new tests:
   - `go test -run TestPlainFormCSRFViaFormField -v ./internal/server/` → PASS, status 303.
   - `go test -run TestPlainFormCSRFMissingRejected -v ./internal/server/` → PASS, status 403.
4. **Negative sanity check** (manual, optional): temporarily forcing the middleware to require the header should make `TestPlainFormCSRFViaFormField` fail with a 403 — confirming the test actually guards the fallback. Revert afterward.

**Pass criteria:** both new tests green, existing integration tests unaffected, and no production code changed.
