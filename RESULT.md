# RESULT — Add regression test for POST /admin/events route

## What was built

Added three regression tests to `r3-intake/internal/server/admin_events_test.go`
covering the auth boundary of the `POST /admin/events` route (the Create Event
form action), which is wrapped in `requireRole("admin", ...)`:

1. **`TestAdminEventCreateAuthBoundary`** — unauthenticated POST (no session
   cookie) → `303` redirect to `/login`, and **no** event record created.
2. **`TestAdminEventCreateNonAdminRejected`** — case_manager session POST →
   `303` redirect to `/login`, and **no** event record created.
3. **`TestAdminEventCreateGetNoCreate`** — plain `GET /admin/events` (no
   trailing slash) → `301` redirect to `/admin/events/` subtree, and **no**
   event created (guards against the original 404 regression path).

These complement the parent card's existing `TestAdminEventCreateRouting`
(valid create) and `TestAdminEventCreateValidation` (validation failure).

## Files changed

- `r3-intake/internal/server/admin_events_test.go` (+80 lines, appended)
- `docs/plans/omp-plan-admin-events-auth-regression.md` (new, working plan)

## Verification

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./internal/server/ -run 'TestAdminEventCreate' -v` — all 5 pass
  (2 existing + 3 new)
- `go test ./...` — full module green (server package ok, 4.354s)
- Negative check (performed by omp): removing the `requireRole` wrapper makes
  the unauthenticated/non-admin tests fail (event created / panic), confirming
  the tests genuinely guard the auth boundary. Production files (`server.go`,
  `admin.go`) unchanged.

## Commit

`f3f2e93` on branch `wt/t_a5d0cbfe`
