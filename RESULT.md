# Epic 15: Event Manage page — participant roster section

**Status:** COMPLETE — child story branches merged into `epic/15-event-manage-page-participant-roster-sec`.

## Stories implemented

- **t_b478182c** — Cross-site participant search for Event Manage enrollment.
  Removes the site restriction from `handleEnrollSearch` in
  `r3-intake/internal/server/admin.go`. Search now returns matching intake
  records from all sites; the `enroll-search-results` template already renders
  each hit's `SiteName`.

- **t_d61f8ff7** — Fix `loadEnrolledRoster` NULL `deleted` filter.
  Changes the roster query from `event='<id>' && deleted=false` to
  `event='<id>' && (deleted = false || deleted = null)` so legacy enrollments
  with a NULL `deleted` value appear in the roster.

- **t_d6f46781** — Add event enrollment flow integration tests.
  Adds `r3-intake/internal/server/event_enrollment_flow_test.go` with 8 tests
  covering end-to-end enroll, idempotency, unenroll soft-delete, search,
  already-enrolled marking, roster stats rendering, auth boundaries, and the
  no-JS fallback. Tests use the existing `newTestServer` harness and
  `seedRosterData` fixtures.

## Files changed

- `r3-intake/internal/server/admin.go` — `handleEnrollSearch` (cross-site) and
  `loadEnrolledRoster` (NULL-deleted filter).
- `r3-intake/internal/server/event_enrollment_flow_test.go` — new integration
  test file.
- `docs/plans/omp-plan-cross-site-enroll-search.md` — working plan.
- `docs/plans/omp-plan-fix-loadenrolledroster.md` — working plan.
- `docs/plans/omp-plan-roster-enrollment-flow-tests.md` — working plan.
- `WORKING_PLAN_fix-loadEnrolledRoster.md` — working plan.
- `WORKING_PLAN_roster_enrollment_flow_tests.md` — working plan.
- `.hermes/plans/2026-08-13_handleEnrollSearch-cross-site.md` — working plan.

## Merge resolution notes

- `RESULT.md` was replaced by each child story branch. The final version
  synthesizes an Epic 15 summary covering all three stories.
- `r3-intake/internal/server/admin.go` changes are in independent functions,
  so they merge cleanly.
- `event_enrollment_flow_test.go` was written against the site-restricted
  search behavior. After merging the cross-site story, `TestEnrollSearch` was
  updated to assert that a participant from a different site (Carol in site2)
  is returned.

## Verification

- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass
- `go test ./internal/server/ -run 'TestEnroll|TestUnenroll|TestRosterRenderingWithStats' -v` — 8/8 PASS
- Conflict-marker sweep — none
