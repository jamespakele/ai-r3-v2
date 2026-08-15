# Working Plan: Update attendance matrix — default event, empty state, remove No Location

## Objective

Two coordinated changes to the R3 Intake attendance matrix:

**CHANGE A — Default to first event + empty state**
- Remove the `Select an event…` placeholder option from the matrix event selector dropdown.
- Default the selected event to the first active event on load whenever no event is present in the query string.
- When there are **zero** active events, render a "Create an Event to track attendance" message (linking to the admin Events screen at `GET /admin`) instead of an empty matrix.
- The user must still be able to switch events via the dropdown.

**CHANGE B — Remove "No Location" warnings**
- Remove the "No Location" group header and its note from the matrix template.
- Remove the `row-no-location` class on participant rows.
- Remove the Go-side `NoLocation` / `HasNoLocation` fields, the `hasNoLocation` computation, `row.NoLocation = cellSiteID == ""`, and the sort that groups no-location rows at the top.

## Constraints

- **Event defaulting must happen before `loadMatrixRows`** so the roster/attendance are scoped to the first event. `parseMatrixFilters` returns `eventID` before events are loaded, so defaulting cannot live there.
- **`handleStats` shares the same defaulting logic** (it calls `parseMatrixFilters` + `loadMatrixRows`). It must resolve the same effective `eventID` so stat cards always match the matrix. It does not need events for its output, but it must load them (or call the shared helper) to resolve the default.
- **`EventRequired` semantics change.** With defaulting, `eventID` is non-empty whenever events exist, so `EventRequired` (`eventID == ""`) is now true **only when there are no events**. The empty-events message takes precedence over the old "Select an event to record attendance." banner (which becomes unreachable and is replaced).
- **`Disabled` cell logic is unchanged.** `MatrixCell.Disabled = cellSiteID == "" || eventID == ""` stays as-is; only the *display* of the no-location warning is removed, not the toggle-disable behavior.
- **Scope of placeholder removal is the matrix selector only** (`matrix-content`, line ~1165). The other `Select an event…` placeholders in `person-attendance-calendar` (~1446), `person-attendance-day` (~1492, ~1528), and `event-fragment` (~1553) are separate templates and are **out of scope** for this task.
- **Final row order** after removing the no-location sort is pure name order — `intakeRecs` is already sorted by `name` (`FindRecordsByFilter(..., "name", ...)`), so removing the `sort.SliceStable` leaves rows in name order. This is the intended final order.
- Design system: Public Sans + Lora, accent `#b5502e`, card `#fffdfa`, page bg `#f7f1e6`. All timestamps HST (UTC-10). PocketBase v0.39 JS migration API (no `app.dao()`).

## File Structure

```
r3-intake/internal/server/attendance.go        — Go handler, view struct, row loader, sort
r3-intake/internal/assets/public/index.html    — matrix-content template (selector, empty state, group header, row class)
r3-intake/internal/assets/public/app.css       — row-no-location / matrix-group-header / matrix-no-location-note rules
r3-intake/internal/server/attendance_test.go   — TestMatrixContentRender, TestMatrixContentRenderEventRequired
```

## Implementation Notes

### 1. `internal/server/attendance.go`

**View struct (`MatrixViewData`)**
- Remove field `HasNoLocation bool` (and its doc comment).
- Add field `NoEvents bool` — true when `len(events) == 0`.
- Keep `EventRequired bool` (now true only when no events exist) — still used to gate the walk-in panel.

**Row struct (`MatrixRow`)**
- Remove field `NoLocation bool // participant has no assigned site`.

**New helper — effective event defaulting**
- Add `func (s *Server) effectiveEventID(eventID string, events []Event) string`:
  - If `eventID != ""`, return it unchanged.
  - Else if `len(events) > 0`, return `events[0].ID` (first active event, since `loadEvents` sorts by `start_date,name`).
  - Else return `""`.

**`handleMatrix`**
- Reorder: call `parseMatrixFilters`, then `loadEvents()` **first**, then `eventID = s.effectiveEventID(eventID, events)`, then `loadMatrixRows(u, dates, eventID, to)`.
- Remove the `hasNoLocation` loop over rows.
- In the `MatrixViewData` literal: remove `HasNoLocation: hasNoLocation`; add `NoEvents: len(events) == 0`; keep `EventRequired: eventID == ""`.

**`handleStats`**
- After `parseMatrixFilters`, load events and resolve `eventID = s.effectiveEventID(eventID, events)` **before** calling `loadMatrixRows`, so the stat cards scope to the same effective event as the matrix.

**`loadMatrixRows`**
- Remove `row.NoLocation = cellSiteID == ""`.
- Remove the trailing `sort.SliceStable(rows, func(i, j int) bool { return rows[i].NoLocation && !rows[j].NoLocation })` block and its comment. Rows then stay in the name order produced by the `intakeRecs` loop.
- Keep `cellSiteID` derivation and `Disabled: cellSiteID == "" || eventID == ""` unchanged.

### 2. `internal/assets/public/index.html` — `matrix-content` define (~line 1159)

**Event selector**
- Remove the placeholder option: `<option value="">Select an event…</option>` (line ~1165). The `{{range .Events}}` options remain; the first event is now pre-selected because `EventID` is defaulted server-side.

**Empty-events state**
- Replace the `{{if .EventRequired}}<p class="empty-state">Select an event to record attendance.</p>{{end}}` block (lines ~1179–1181) with a `NoEvents`-gated message, e.g.:
  `{{if .NoEvents}}<p class="empty-state">Create an Event to track attendance. <a href="/admin">Go to Events</a></p>{{end}}`
- Keep `{{if not .EventRequired}}` gating the walk-in panel (unchanged).

**No Location group header**
- Remove the entire `{{if .HasNoLocation}}<tr class="matrix-group-header">…</tr>{{end}}` block (lines ~1222–1229), including the `matrix-group-title` "No Location" span and the `matrix-no-location-note` span.

**Row class**
- Change `<tr class="{{if .IsDropout}}row-dropout {{end}}{{if .NoLocation}}row-no-location{{end}}">` (line ~1231) to `<tr class="{{if .IsDropout}}row-dropout {{end}}">`.

### 3. `internal/assets/public/app.css`

Remove the now-dead rules (keep the `row-dropout` rules):
- `.matrix-table tr.row-no-location td { background: #fbf6ee; }` (line 308)
- `.matrix-table tr.row-dropout.row-no-location td { background: #fbeeec; }` (line 309)
- `.matrix-table tbody tr.row-no-location:hover td { background: #f5ecd8; }` (line 312)
- `.matrix-table tbody tr.row-dropout.row-no-location:hover td { background: #f6ddd9; }` (line 313)
- `.matrix-group-header td { … }` (lines 314–320)
- `.matrix-group-title { margin-right: 8px; }` (line 321)
- `.matrix-no-location-note { … }` (lines 322–326)

### 4. `internal/server/attendance_test.go`

**`TestMatrixContentRender`**
- Remove `NoLocation: true` from the Bob `MatrixRow` literal (field is gone).
- Remove `HasNoLocation: true` from the `MatrixViewData` literal.
- From the `want` list, remove `"No Location"`, `"matrix-group-header"`, `"row-no-location"`, and `"Select an event…"`. Keep `"dot-disabled"` (Bob's cells still have `Disabled: true`).
- Remove the Bob-before-Alice ordering check (`strings.Index(out, "Bob") > strings.Index(out, "Alice")`). With the sort gone, rows render in the literal order given (Alice first here), so no ordering assertion is valid.

**`TestMatrixContentRenderEventRequired`**
- Update the view to reflect the new semantics: set `NoEvents: true` (and keep `EventID: ""`, `EventRequired: true`).
- From the `want` list, remove `"Select an event…"` and `"Select an event to record attendance."`; add `"Create an Event to track attendance"` and the `/admin` link.
- Keep the `forbid` list (`"Add walk-in"`, `"walkin-search"`) — the walk-in panel must still be hidden when there are no events.

**New test (recommended)**
- Add a test that renders `matrix-content` with `Events` containing one event and `EventID` defaulted to that event's ID (i.e. `EventRequired: false`, `NoEvents: false`), asserting the first event's name is the selected option and no `Select an event…` placeholder is present — covering the default-to-first-event behavior at the template level.

## Verification Criteria

1. `go build ./...` and `go test ./internal/server/...` pass in `r3-intake/`.
2. `TestMatrixContentRender` passes with no references to `No Location`, `matrix-group-header`, `row-no-location`, or `Select an event…`, and no ordering assertion.
3. `TestMatrixContentRenderEventRequired` passes asserting the new empty-events message + `/admin` link and the absence of the walk-in panel.
4. `grep` confirms no remaining references to `NoLocation`, `HasNoLocation`, `hasNoLocation`, `row-no-location`, `matrix-group-header`, `matrix-no-location-note`, or `matrix-group-title` in `attendance.go`, `index.html`, `app.css`, or `attendance_test.go`.
5. Manual/HTMX check: loading `/attendance` with no `event` query param selects the first active event in the dropdown and scopes the matrix to it; with zero active events it shows "Create an Event to track attendance" linking to `/admin`; switching events via the dropdown still works.
6. Stat cards (`/attendance/stats`) reflect the same effective event as the matrix after defaulting.
