# RESULT — Story 2.2: Event Enrollment and Roster Management

Implemented via omp-plan-execute (MOA plan → omp --plan-yolo --advisor).

## What was built
- **Real enrollment screen** replacing the Story 2.1 placeholder at `/admin/events/{id}/manage`: event header (name, site, dates, status badge), tabs (Enrolled (N) active; History/Summary inert), "Add participant" HTMX name search, and an enrolled-roster table (Participant, Enrolled date, Days Attended N/M, Rate badge %, Last Present, Remove).
- **Backend handlers** (admin-only):
  - `POST /admin/events/{id}/enroll` — idempotent (no duplicate event+intake), sets enrolled_date (HST).
  - `POST /admin/events/{id}/unenroll` — soft delete (sets `deleted=true`), preserving attendance history (FR12).
  - `GET /admin/events/{id}/enroll-search` — site-scoped name search (min 2 chars, max 10), marks already-enrolled.
- **Attendance stats** per enrolled participant scoped to the event date range: days attended (present+walk_in), total elapsed-or-total days, rate %, last present. Division-by-zero guarded.
- **Matrix integration:** when an event is selected, `loadMatrixRows` now scopes rows to active enrollments ∪ walk-ins for that event (PRD: "Enrolled = expected — they appear in the matrix by default").
- **Migration `008_event_enrollment_deleted.go`** adds a `deleted` bool to `event_enrollment` (soft-delete seam); registered in `002_encryption.go`. `loadEnrolledCount` filters `deleted=false` so the admin events list count stays consistent.

## Files
- `r3-intake/internal/server/admin.go` (EnrolledRow/EnrollSearchResult/EnrollSearchView structs; handlers; roster/stats helpers)
- `r3-intake/internal/server/server.go` (routes + eventEnrollmentCollection)
- `r3-intake/internal/server/attendance.go` (matrix event scoping)
- `r3-intake/internal/assets/public/index.html` (event-manage screen + event-roster + enroll-search-results fragments; ?v=5)
- `r3-intake/internal/assets/public/app.css` (enroll tabs, search panel, roster, rate badges)
- `r3-intake/pocketbase/migrations/008_event_enrollment_deleted.go` + `002_encryption.go` (migration registration)
- `r3-intake/internal/migrations/pocketbase/migrations/008_event_enrollment_deleted.go` (mirror)
- `r3-intake/internal/server/admin_events_test.go` (TestAdminEventsRender extended, TestEnrollSearchResultsRender, TestEnrollmentStatsCompute)
- `docs/plans/omp-plan-event-enrollment.md` (plan artifact)

## Verification
- `go build ./...` — OK
- `go vet ./...` — OK
- `go test ./...` — all pass (incl. TestAdminEventsRender, TestEnrollSearchResultsRender, TestEnrollmentStatsCompute)
- Template parse + render verified for event-manage, event-roster, enroll-search-results.

## Notes
- No runnable server binary in this worktree (no cmd/main.go), so DB-backed handler paths are covered by unit tests only; no fake-PocketBase harness exists in the suite (infra gap, not a defect).
- This is a child story card; integration/merge to the epic branch is the parent's (t_9c800987) job.
