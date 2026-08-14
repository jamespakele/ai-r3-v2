# Working Plan: Remove the Event Manage route and enrollment-management handlers

## Objective
Remove the Event Manage screen and its enrollment-management handlers from the Go server. The `GET /admin/events/{id}/manage` route, the `handleAdminEventManage` / `renderEventManage` handlers, the four enrollment-management routes (`enroll`, `unenroll`, `enroll-search`, `status`), and all server-side helper code / view types / `AdminView` fields that exist only to serve that screen are deleted. Attendance remains available via the Matrix (`/attendance?event={id}`); event details via Edit (`/admin/events/{id}/update`).

## Constraints
- Go module lives in `r3-intake/` (this worktree: `/srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_1c442540/r3-intake`).
- `internal/server/server.go` `Mux()` registers routes; `internal/server/admin.go` holds the handlers, `AdminView`, `EventRow`, and the roster/search helpers.
- PocketBase v0.39 Go API via `s.pb` (`FindCollectionByNameOrId`, `FindRecordsByFilter`, `FindRecordById`, `Save`). No `app.dao()`.
- All timestamps HST (`var hst = time.FixedZone("HST", -10*60*60)`, admin.go line 17).
- Use `mcpmod.EscapeFilter` (`r3-intake/internal/mcp`) for user values in PB filters.
- **Server-code only.** Do NOT touch `index.html` templates (event-manage / event-roster / enroll-search-results defines, Manage link) and do NOT touch any `*_test.go` file — those are separate sibling cards.
- After omp implements, source is not to be hand-edited; this plan is the single source of truth for the change.

## File Structure
All changes are confined to two files:

| File | Change |
|------|--------|
| `r3-intake/internal/server/server.go` | Remove 5 route registrations in `Mux()` (lines ~125–131). Keep everything else, including `eventEnrollmentCollection()` (line 248). |
| `r3-intake/internal/server/admin.go` | Remove 3 view types, 5 `AdminView` fields, 11 handler/helper functions, and the now-unused `sort` import. Keep `EventStatus` field, `EventRow.Enrolled`, `loadEnrolledCount()`, `loadAllEvents()`, `eventsCollection()` (defined in `attendance.go` line 410). |

## Implementation Notes

### 1. `server.go` — remove routes (lines ~125–131)
In `Mux()`, delete these five registrations (and their surrounding comment lines):

- `mux.HandleFunc("/admin/events/", s.csrfMiddleware(s.requireRole("admin", s.handleAdminEventManage)))` (line 125, plus its comment `// Event enrollment-management placeholder — admin only` on line 124)
- `mux.HandleFunc("/admin/events/{id}/enroll", s.csrfMiddleware(s.requireRole("admin", s.handleEventEnroll)))` (line 127)
- `mux.HandleFunc("/admin/events/{id}/unenroll", s.csrfMiddleware(s.requireRole("admin", s.handleEventUnenroll)))` (line 128)
- `mux.HandleFunc("/admin/events/{id}/enroll-search", s.csrfMiddleware(s.requireRole("admin", s.handleEnrollSearch)))` (line 129)
- `mux.HandleFunc("POST /admin/events/{id}/status", s.csrfMiddleware(s.requireRole("admin", s.handleEventStatus)))` (line 131)

Note: the comment `// Story 2.2 enrollment routes` (line 126) and `// Story 2.3 lifecycle routes` (line 130) can be dropped with their routes, or left if they still introduce the kept update/delete lines. Keep these lines untouched (they are NOT removed):
- `POST /admin/events/{id}/update` → `handleAdminEventUpdate` (line 132)
- `POST /admin/events/{id}/delete` → `handleAdminEventDelete` (line 133)
- `POST /admin/events` → `handleAdminEventAdd` (line 137)
- `GET /admin/events/{id}/report` → `handleEventReport` (line 139)

### 2. `admin.go` — remove AdminView fields (lines 48, 50–53)
In the `AdminView` struct, delete these fields:
- `EventSiteName    string` (line 48)
- `EventStatusError string` (line 50)
- `EventEnrolled    int` (line 51)
- `EnrolledCount    int` (line 52)
- `Enrolled         []EnrolledRow` (line 53)

**KEEP** `EventStatus string` (line 49) — it is set by `handleEventReport` (line 1016) and consumed by the event-report template.

### 3. `admin.go` — remove view types (lines 73–96)
Delete:
- `EnrolledRow` struct (lines 73–82, including its `// EnrolledRow is one row...` comment)
- `EnrollSearchResult` struct (lines 84–90)
- `EnrollSearchView` struct (lines 92–96)

**KEEP** `EventRow` (lines 61–71) including its `Enrolled int` field (line 69) — set by `loadAllEvents()` via `loadEnrolledCount()` and rendered by the Events tab list (`{{$ev.Enrolled}}`).

### 4. `admin.go` — remove handlers/helpers
Delete each of these functions entirely (with leading doc comments):

- `validEventTransition(from, to string) bool` (line 651) — used only by `handleEventStatus`.
- `renderEventManage(w, r, u, rec, statusErr)` (line 661) — uses `loadEnrolledRoster`, `loadEnrolledCount`, `siteNameMap`; sets the removed `AdminView` fields.
- `handleAdminEventManage(w, r)` (line 685) — the `/admin/events/` subtree handler; parses `/manage` off the path and calls `renderEventManage`.
- `loadEnrolledRoster(eventID, start, end)` (line 710) — returns `[]EnrolledRow`.
- `loadEnrollmentStats(intakeID, eventID, start, end)` (line 750) — used only by `loadEnrolledRoster`.
- `enrollmentRate(daysAttended, totalDays int) int` (line 788) — used only by `loadEnrollmentStats`.
- `daysInRange(start, end, cap string) int` (line 797) — used only by `loadEnrollmentStats`.
- `handleEventEnroll(w, r)` (line 819) — calls `respondRoster`.
- `handleEventUnenroll(w, r)` (line 865) — calls `respondRoster`.
- `respondRoster(w, r, eventID)` (line 895) — redirects to `/admin/events/{id}/manage` and renders the `event-roster` fragment; uses `loadEnrolledRoster` and the removed `Enrolled`/`EnrolledCount` fields.
- `handleEnrollSearch(w, r)` (line 919) — renders `enroll-search-results` with `EnrollSearchView`/`EnrollSearchResult`.
- `handleEventStatus(w, r)` (line 971) — the lifecycle route; redirects to `/manage` and re-renders via `renderEventManage` on error.

**KEEP** `handleEventReport` (line 998) and `handleAdminEventUpdate` (line 624) / `handleAdminEventDelete` (line 636) / `handleAdminEventAdd` (line 547) / `adminEventUpdate` (line 559) — still routed. `loadAllEvents` (line 1044) and `loadEnrolledCount` (line 1076) are KEPT — `loadAllEvents` feeds the Events tab "Enrolled" column.

### 5. `admin.go` — fix imports
The `sort` package is used **only** at line 742 inside `loadEnrolledRoster`. After its removal, delete `"sort"` from the import block (line 6) or the file will not compile.

Imports that REMAIN used and must be kept:
- `fmt` (lines 132, 138, 145, 1081, etc.)
- `net/http` (all handlers)
- `strings` (lines 130–142, 217–242, 269–490, 571–575, 690, 825, 871, 925, etc.)
- `time` (line 17 `hst`, lines 504–505, 589–590, 1124)
- `mcpmod "r3-intake/internal/mcp"` (lines 132, 138, 145, 1081)
- `github.com/pocketbase/pocketbase/core` (still used across handlers)

### 6. Things NOT removed (double-check)
- `eventEnrollmentCollection()` in `server.go` (line 248) — KEEP.
- `loadEnrolledCount()` in `admin.go` (line 1076) — KEEP (used by `loadAllEvents`).
- `eventsCollection()` in `attendance.go` (line 410) — KEEP.
- `EventStatus` AdminView field — KEEP.
- `EventRow.Enrolled` — KEEP.
- `siteNameMap()` / `userNameMap()` — KEEP (used by `loadAllEvents` and other admin list paths).
- `hst` var — KEEP (`time` still used elsewhere).

## Verification Criteria
Run from `r3-intake/`:

```
cd r3-intake && go build ./... && go vet ./...
```

- `go build ./...` must pass cleanly with **zero errors** (non-test packages). This proves the routes, handlers, helper code, view types, `AdminView` fields, and the `sort` import were fully and consistently removed.
- `go vet ./...` also type-checks test files in package dirs; it is expected to report errors referencing the removed handlers/types **until the sibling test-update card (t_4ac127a6) lands** — that is acceptable for this card and NOT a regression of this change.
- `go test ./...` is NOT a gating criterion for this card (test files will fail to compile until t_4ac127a6).

### Test files that WILL need updating in the sibling test card (t_4ac127a6)
List these so the test card knows exactly what references the removed code:
- `internal/server/admin_events_test.go` — references `AdminView` fields `EventSiteName` (line 96), `EnrolledCount`/`EventEnrolled`/`Enrolled`/`EnrolledRow` (lines 98–99), `EventStatus` (lines 135, 160), `EventStatusError` (line 161), and `EnrollSearchView`/`EnrollSearchResult` (lines 200, 202). It also renders the `enroll-search-results` fragment (`TestEnrollSearchResultsRender`, line 191) and asserts on the manage screen / status-transition flow.
- `internal/server/admin_events_update_delete_test.go` — may exercise `handleEventStatus` / status lifecycle and the manage-page redirect.
- `internal/server/event_enrollment_flow_test.go` — calls `srv.eventEnrollmentCollection()` (lines 75, 89) and exercises `enroll`/`unenroll`/`respondRoster`/roster rendering; note this test also uses `eventEnrollmentCollection()`, which is KEPT, so only the enroll/unenroll/roster assertions need removal.
