# Working Plan — Update Admin Events Go Tests for Removal of the Event Manage Screen & Enrollment Flow

## Objective

Update the Go tests under `r3-intake/internal/server/` so the integrated tree compiles and passes after Epic 17 removes the **Event Manage screen and enrollment-management flow** from the server, without touching production source or templates (owned by sibling cards).

The integrated tree must have **no** references to any removed symbol:

| Category | Removed symbols |
|---|---|
| Routes/handlers | `GET /admin/events/{id}/manage` (`handleAdminEventManage`), `POST /admin/events/{id}/enroll` (`handleEventEnroll`), `POST /admin/events/{id}/unenroll` (`handleEventUnenroll`), `GET /admin/events/{id}/enroll-search` (`handleEnrollSearch`), `POST /admin/events/{id}/status` (`handleEventStatus`) |
| Types | `EnrolledRow`, `EnrollSearchResult`, `EnrollSearchView` |
| Functions | `validEventTransition`, `renderEventManage`, `loadEnrolledRoster`, `loadEnrollmentStats`, `enrollmentRate`, `daysInRange`, `respondRoster` |
| `AdminView` fields | `EventSiteName`, `EventStatusError`, `EventEnrolled`, `EnrolledCount`, `Enrolled` |
| Template defines | `event-manage`, `event-roster`, `enroll-search-results` |

**KEPT** (tests may continue to reference): `event-report` template, `EventStatus` field, `EventRow.Enrolled` (admin list column), `loadEnrolledCount`, `eventEnrollmentCollection`, and the update/delete/create/report routes.

## Constraints

- **Test-only change.** Do NOT modify `server.go`, `admin.go`, `index.html`, `app.css`, or any production Go/template source. Sibling cards own those edits and land before/with this one.
- End state must satisfy: `cd r3-intake && go build ./... && go vet ./... && go test ./...` — all pass.
- No test file may reference any removed symbol (a reference to a removed type/function/field fails to compile).
- Keep imports minimal and valid — remove any that become unused (Go treats unused imports as compile errors).
- Preserve all KEPT test coverage (event-report render, create/update/delete routes, auth boundaries). Strip only coverage for removed functionality.
- **Scope note (important):** the task names two files, but `admin_events_update_delete_test.go` also exercises the removed `/manage` route. It must be edited too, or the integrated tree will not compile/pass. Included in this plan.

## File Structure

Files touched (all under `r3-intake/internal/server/`):

- `admin_events_test.go` (460 lines) — **edit**: strip removed-render/stats/transition tests + one assertion; drop now-unused `time` import.
- `event_enrollment_flow_test.go` (348 lines) — **delete**: entire file exercises removed routes/types; no kept coverage.
- `admin_events_update_delete_test.go` — **edit**: remove `doEventManage` helper + `TestAdminEventManageDeletedNotFound`.

No new files. No production files.

## Implementation Notes

### 1. `event_enrollment_flow_test.go` — DELETE the entire file

Every test and helper targets the removed routes/types:

- Tests: `TestEnrollFlowEndToEnd`, `TestEnrollIdempotent`, `TestUnenrollSoftDeletes`, `TestEnrollSearch`, `TestEnrollSearchMarksAlreadyEnrolled`, `TestRosterRenderingWithStats`, `TestEnrollAuthBoundary`, `TestEnrollNoJSFallback`.
- Helpers/vars: `enrollDateRe`, `saveActiveEnrollment`, `doEnrollPost`, `doEnrollSearch`, `countEnrollments`, `findEnrollment`.

The file references removed handlers (`handleEventEnroll/Unenroll/Search`), the removed `event-manage`/`event-roster`/`enroll-search-results` fragments, removed helpers `daysInRange`/`enrollmentRate`, and removed `AdminView` roster fields. No kept behavior remains — the enroll/unenroll/search/auth/soft-delete semantics no longer exist in the product. Delete the file outright; all its imports (`fmt`, `net/http`, `net/http/httptest`, `net/url`, `regexp`, `strings`, `testing`, `time`, `github.com/pocketbase/pocketbase`, `github.com/pocketbase/pocketbase/core`) disappear with it.

> No orphan risk: `saveAttendance`/`seedRosterData` are defined in `attendance*_test.go` and used there too; Go errors on unused imports, not unused functions. Verify the package still builds after the delete.

### 2. `admin_events_test.go` — edit

Within `TestAdminEventsRender`:

- **KEEP** the header/admin-list render, but **remove** the single assertion `` `href="/admin/events/ev1/manage"` `` from the `for _, want := range []string{...}` block (the manage link no longer exists in the template). Keep the other assertions in that block (status badges, `/attendance?event=ev1`, `/admin/events` form action, `Create Event`).
- **KEEP** the Report-link assertions (kept `event-report` route / `Report` button).
- **KEEP** the validation-error re-render block (`viewErr`/`ebuf`).
- **REMOVE** the three event-manage render blocks: the `mview`/`mbuf` roster block, the `doneView`/`dbuf` read-only block, and the `errView`/`xbuf` transition-error block. These reference removed template `event-manage` and removed `AdminView` fields (`EventSiteName`, `EventStatusError`, `EventEnrolled`, `EnrolledCount`, `Enrolled`) and removed type `EnrolledRow`.
- **KEEP** the `event-report` block (`rview`/`rbuf`).

Remove the following test functions entirely:

- `TestEnrollSearchResultsRender` — renders removed `enroll-search-results` template via removed `EnrollSearchView`/`EnrollSearchResult`.
- `TestEnrollmentStatsCompute` — tests removed `daysInRange`/`enrollmentRate`.
- `TestEventStatusTransition` — tests removed `validEventTransition`.

Keep (unchanged): `findEventByName`, `doEventCreate`, and `TestAdminEventCreateRouting`, `TestAdminEventCreateValidation`, `TestAdminEventCreateAuthBoundary`, `TestAdminEventCreateNonAdminRejected`, `TestAdminEventCreateGetNoCreate`.

**Imports cleanup:** remove `"time"` from the import block — its only uses (`time.Parse`) live in `TestEnrollmentStatsCompute`, which is deleted. All other imports (`bytes`, `html/template`, `net/http`, `net/http/httptest`, `net/url`, `strings`, `testing`, `github.com/pocketbase/pocketbase/core`, `r3-intake/internal/assets`) remain in use by kept functions and must be kept.

### 3. `admin_events_update_delete_test.go` — edit

- **REMOVE** the `doEventManage` helper (the only reference to removed `GET /admin/events/{id}/manage`).
- **REMOVE** `TestAdminEventManageDeletedNotFound` (the only consumer of `doEventManage`), which asserts the removed manage page returns 200/404.
- Keep all update/delete tests and their helpers. `httptest`, `net/url`, `core`, and all other imports remain in use by the kept tests — no import changes here.

### Cross-cutting check

After the three edits, `grep` the entire `internal/server/` test package and confirm **zero** references to: `EnrolledRow`, `EnrollSearchResult`, `EnrollSearchView`, `validEventTransition`, `renderEventManage`, `loadEnrolledRoster`, `loadEnrollmentStats`, `enrollmentRate`, `daysInRange`, `respondRoster`, `EventSiteName`, `EventStatusError`, `EventEnrolled`, `EnrolledCount`, `event-manage`, `event-roster`, `enroll-search-results`, and the `/manage`, `/enroll`, `/unenroll`, `/enroll-search`, `/status` path segments (in tests). The kept references — `EventStatus`, `EventRow`, `.Enrolled` (list column), `loadEnrolledCount`, `eventEnrollmentCollection`, `event-report`, `/report`, `/admin/events` create/update/delete — must remain intact.

## Verification Criteria

In `r3-intake/`, with the sibling cards merged (removed routes/types/templates absent from `server.go`, `admin.go`, `index.html`):

1. `go build ./...` — exit 0, no compile errors.
2. `go vet ./...` — exit 0, no vet findings.
3. `go test ./...` — all tests pass, including kept admin event create/update/delete/auth and event-report tests.
4. `grep` scan confirms no test references any removed symbol (cross-cutting check above) — this guarantees the integrated tree compiles.
5. No production source or template file was modified (`git status` shows only the three test files changed).
