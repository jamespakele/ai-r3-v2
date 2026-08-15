# RESULT — t_ca71ac0b: Update attendance matrix (default event, empty state, remove No Location)

## What was built

Two coordinated changes to the attendance matrix, implemented by omp from the MOA working plan.

### Change A — Default to first event + empty state
- Removed the "Select an event…" placeholder option from the matrix event selector.
- Added `effectiveEventID(eventID, events)` helper: explicit event wins; otherwise defaults to the first active event (loadEvents sorts by start_date,name); returns "" when no events.
- `handleMatrix` now loads events first, resolves the effective eventID, then loads rows — so the roster/attendance scope to the default event.
- `handleStats` resolves the same effective eventID so stat cards always match the matrix.
- Added `NoEvents` view field; when zero active events, the matrix renders "Create an Event to track attendance. [Go to Events](/admin)" instead of an empty matrix.
- `EventRequired` is now true only when there are no events (still gates the walk-in panel).

### Change B — Remove "No Location" warnings
- Removed the "No Location" group header + note from the matrix template.
- Removed the `row-no-location` class from participant rows.
- Removed Go-side `NoLocation` / `HasNoLocation` fields, the `hasNoLocation` computation, `row.NoLocation = cellSiteID == ""`, and the no-location sort.
- Rows now render in pure name order (intakeRecs is already sorted by name).
- Removed 7 dead CSS rules (row-no-location, matrix-group-header, matrix-no-location-note, matrix-group-title).

## Files changed
- r3-intake/internal/server/attendance.go
- r3-intake/internal/assets/public/index.html
- r3-intake/internal/assets/public/app.css
- r3-intake/internal/server/attendance_test.go
- r3-intake/internal/server/attendance_matrix_default_integration_test.go (new)

## Verification
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (server 17.0s, migrations ok)
- New tests: TestMatrixContentRenderDefaultsToFirstEvent, TestMatrixDefaultsToFirstEvent, TestMatrixNoEventsEmptyState, TestStatsDefaultsToFirstEvent
- grep across the 5 files: zero matches for NoLocation/HasNoLocation/hasNoLocation/row-no-location/matrix-group-header/matrix-no-location-note/matrix-group-title
- Remaining "Select an event…" refs are in out-of-scope person-attendance templates and in test forbid lists (asserting absence) — correct.

## Notes
- No runnable binary in this worktree (no cmd/r3-intake main package), so browser-level check was not possible; the new integration tests exercise the identical HTTP path (handler, auth, template rendering) through the real in-process server.
