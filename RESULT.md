# RESULT — Fix loadEnrolledRoster to return enrolled participants and stats

## What was built
Fixed `loadEnrolledRoster` in `r3-intake/internal/server/admin.go` so the Event
Manage page roster and the HTMX `respondRoster` fragment return enrolled
participants with correct attendance stats.

## Root cause
The roster query filtered `event='<id>' && deleted=false`. In PocketBase v0.39 a
`BoolField` with `Required:false` (migration 008) defaults to **NULL**, not
`false`, for records where the field was never explicitly written. `deleted=false`
excluded NULL rows, so legacy/pre-existing enrollments never appeared — the roster
rendered empty even when enrollments existed.

## Change
Single-line filter fix in `loadEnrolledRoster` (admin.go ~531):
`event='<id>' && (deleted = false || deleted = null)`
— matches the codebase's own established pattern in `notes.go` `loadNoteRows`
(line 363). Returns active + legacy-NULL enrollments, still excludes soft-deleted
ones (unenroll keeps attendance history).

## Scope honored
- Only `loadEnrolledRoster` touched. Enroll-search handler (sibling t_b478182c)
  and tests (sibling t_d6f46781) untouched.
- `loadEnrolledCount` (admin.go ~899) has the same NULL-exclusion bug and
  under-counts legacy enrollments — flagged as follow-up for the parent, not
  changed here.

## Verification
- `go build ./...` — clean
- `go vet ./...` — no warnings
- `go test ./...` — all packages pass (internal/server ok)

## Artifacts
- Plan: docs/plans/omp-plan-fix-loadenrolledroster.md
