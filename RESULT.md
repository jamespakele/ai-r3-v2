# RESULT — Story 1.1: Database Migration & Attendance Tab

## What was built
- **`r3-intake/pocketbase/migrations/007_events_attendance.js`** (new): creates `events`, `event_enrollment`, `attendance` collections per PRD §07. All rules null (Go is policy layer). Cascade deletes: event→enrollment+attendance, intake→enrollment+attendance, site/users restricted. `attendance.event` nullable. Down migration drops in reverse order (attendance → event_enrollment → events). Idempotent guarded pattern per 005_notes.js.
- **`r3-intake/internal/server/attendance.go`** (new): `AttendanceView{UserName,Role,IsAdmin}` + `handleMatrix` skeleton rendering the `matrix` template. No data loading (downstream cards).
- **`r3-intake/internal/server/server.go`** (modified): registered `mux.HandleFunc("/attendance", s.requireAuth(s.handleMatrix))`.
- **`r3-intake/internal/assets/public/index.html`** (modified): Attendance link in `list` topbar between Records and Admin; added `{{define "matrix"}}` standalone placeholder page.

## Verification
- `go build ./...` → BUILD_OK
- `go vet ./...` → VET_OK
- `go test ./...` → all packages pass
- Template parse check: `matrix` and `list` defines both registered (throwaway test, removed after run)

## Notes
- Unauthenticated `/attendance` redirects to `/login?next=/attendance` (requireAuth behavior), matching other auth-gated routes.
- Scope discipline: no matrix grid UI, filters, HTMX toggle, walk-in, or stats built here — those belong to downstream cards (t_1ad219e4 et al.).
- Working plan artifact: `docs/plans/omp-plan-attendance-foundation-1-1.md`
