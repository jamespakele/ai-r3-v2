# RESULT — Epic 22: Live intake filtering + attendance matrix default event / No Location removal

This epic merged two child story worktrees into `epic/22-live-intakelist-filtering-remove-no-loca`:

1. `wt/t_9f9c1728` — Convert intake list filter to live HTMX.
2. `wt/t_ca71ac0b` — Update attendance matrix (default event, empty state, remove No Location).

## 1. Convert intake list filter to live HTMX

## What was built
Converted the intake records list (landing screen at `/`) from a button-submitted
GET form to live HTMX filtering. The list now re-renders in place as the user
interacts — no Filter button needed.

## Changes (2 files)
- `r3-intake/internal/assets/public/index.html`
  - Added `<script src="/static/htmx.min.js" defer></script>` to the `list` template head.
  - Extracted the form + result count + bulk bar + table into a new
    `{{define "list-content"}}` partial wrapped in `<div id="list-content">`.
  - Form now uses `hx-get="/" hx-target="#list-content" hx-swap="outerHTML"
    hx-trigger="change, keyup changed delay:300ms" hx-push-url="true"`.
  - Removed the Filter submit button; kept the Clear link (shown only when a filter is active).
- `r3-intake/internal/server/admin.go`
  - `handleList` now branches on `HX-Request: true` to render only the `list-content`
    partial; non-HTMX GETs render the full `list` page as before.

## Behavior
- Selecting an event applies the filter immediately.
- Selecting a status applies the filter immediately.
- Typing a name applies the filter as they type (300ms debounce; min-2-char rule intact).
- URL query string stays in sync (`hx-push-url`) so filters are shareable.
- Bulk-delete form/bar survive HTMX swaps (CSRF re-injected via `htmx:afterProcessNode`).

## Verification
- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — full suite passes (internal/server ok, pocketbase/migrations ok)
- omp ran an httptest against the real Mux(): plain GET `/` returns full page with no
  Filter button; `HX-Request: true` returns only the `list-content` partial; `?q=`,
  `?status=`, `?event=` filters work; Clear link present only when filtered;
  bulk-delete form survives; `/attendance` unaffected.

## Scope
Intake list only. Attendance matrix untouched (separate sibling task t_ca71ac0b).

## 2. Update attendance matrix (default event, empty state, remove No Location)

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
