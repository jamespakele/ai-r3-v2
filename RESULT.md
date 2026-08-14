# RESULT — Update admin events tests for removed Manage screen

Epic 17 removed the Event Manage screen and the enrollment-management flow from
the server (sibling t_1c442540) and the template (sibling t_a7679d78). This card
updated the Go tests so the integrated tree compiles and passes with no reference
to any removed symbol.

## Changes (test-only — no production source or template touched)

- `r3-intake/internal/server/event_enrollment_flow_test.go` — **deleted**. Every
  test/helper exercised removed routes (`/enroll`, `/unenroll`, `/enroll-search`,
  `/manage`) and removed types (`EnrolledRow`, `EnrollSearchResult`,
  `EnrollSearchView`), removed funcs (`daysInRange`, `enrollmentRate`), and the
  removed roster fragments. No kept coverage remained.
- `r3-intake/internal/server/admin_events_test.go` — removed the manage-link
  assertion, the three `event-manage` render blocks, and
  `TestEnrollSearchResultsRender` / `TestEnrollmentStatsCompute` /
  `TestEventStatusTransition`. Dropped the now-unused `time` import. **Kept**
  event-report render, admin list render, validation-error path, and all
  create-route/auth tests.
- `r3-intake/internal/server/admin_events_update_delete_test.go` — removed the
  `doEventManage` helper and `TestAdminEventManageDeletedNotFound` (only uses of
  `GET /admin/events/{id}/manage`). Kept all update/delete tests.

## Verification

- `go build ./...` — exit 0
- `go vet ./...` — exit 0
- `go test ./...` — `ok r3-intake/internal/server`, `ok r3-intake/pocketbase/migrations`
- grep over `internal/server/*_test.go` for all removed symbols — zero code
  references (only an unrelated `Status/status` comment in
  `person_attendance_test.go`)

## Notes

Tests pass against the current master-forked worktree (production symbols still
present); the grep scan guarantees they stay clean once sibling production
removals land at the parent's merge. Plan artifact:
`docs/plans/omp-plan-update-admin-events-tests.md`.
