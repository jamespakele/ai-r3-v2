# RESULT — Add tests for roster rendering and enrollment flow

## What was built
New white-box `package server` integration test file
`r3-intake/internal/server/event_enrollment_flow_test.go` exercising the event
roster rendering and the enrollment / unenroll / search HTTP flow against a
real in-process PocketBase (via the existing `newTestServer` harness).

Local helpers added in the new file: `saveActiveEnrollment` (sets
`deleted=false` explicitly — the shared `saveEnrollment` does not, and the
roster filter excludes NULL-deleted rows), `doEnrollPost`, `doEnrollSearch`,
`countEnrollments`, `findEnrollment`.

## Tests added (8)
1. `TestEnrollFlowEndToEnd` — admin enroll POST → roster fragment (Alice,
   roster-table) + record created with `deleted=false`, `enrolled_date` YYYY-MM-DD.
2. `TestEnrollIdempotent` — double enroll → 1 record, roster lists participant once.
3. `TestUnenrollSoftDeletes` — record kept (soft-delete), `deleted=true`, empty state rendered.
4. `TestEnrollSearch` — same-site Al→Alice, Bo→Bob, Ca excludes Carol (site-restricted in this worktree), 1-char→empty, zz→no-match message.
5. `TestEnrollSearchMarksAlreadyEnrolled` — disabled button, no "+ Enroll".
6. `TestRosterRenderingWithStats` — 2 / {totalDays} via daysInRange, last-present 2026-08-14, rate badge class + value.
7. `TestEnrollAuthBoundary` — cm + anon → 303 /login; no record created.
8. `TestEnrollNoJSFallback` — 303 to manage screen, record still created.

## Verification
- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./internal/server/ -run 'TestEnroll|TestUnenroll|TestRosterRenderingWithStats' -v` — all 8 pass
- `go test ./...` — pass (server suite 8.2s)
- `git status --short` — only the new test file added; no production code or
  existing tests modified.

## Merge-forward compatibility
Tests seed enrollments with explicit `deleted=false` and assert only same-site
search results, so they pass in this worktree (unfixed `deleted=false` filter +
site-restricted search) AND remain valid after the parent merges the sibling
fixes (`(deleted = false || deleted = null)` roster filter, cross-site search).
