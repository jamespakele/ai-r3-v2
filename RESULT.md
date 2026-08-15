# RESULT — Remove eventSite roster filter in loadMatrixRows

## What was built
Removed the event-site-based intake filter from `loadMatrixRows` in
`r3-intake/internal/server/attendance.go`. The roster is now always the full
site/role-scoped participant list, independent of the selected event. Event
scoping applies only to the attendance map.

## Changes
- `r3-intake/internal/server/attendance.go` — deleted the `case eventSite != ""`
  branch from the `intakeFilter` switch. `intakeFilter` is now role-only
  (`case_manager` → `assigned_to`, else `1=1`). Preserved the AC #1 comment
  verbatim. Kept the `eventSite` lookup block and `cellSiteID := eventSite`
  fallback (drives the disabled event-location box + NoLocation grouping).
  Attendance-map scoping via `resolveEventIDs(eventID)` unchanged.
- `r3-intake/internal/server/attendance_roster_integration_test.go` — updated
  `TestMatrixRosterEventScoped`: `wantWithEvent` is now the full 4-intake admin
  roster (identical to no-event); removed the out-of-site walk-in exclusion
  loop; preserved all attendance-map scoping assertions.

## Verification
- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — all pass (internal/server ok, migrations ok)
- `grep eventSite attendance.go` — only in lookup block + cellSiteID fallback;
  none in the intakeFilter switch

## Acceptance criteria
- [x] Code compiles
- [x] No eventSite variable used for intakeFilter
- [x] Attendance map still scoped by the selected event
