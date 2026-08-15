# Working Plan: Update attendance matrix UI to reflect event-scoped date range

## Objective

When an event is selected on the attendance matrix, the UI should clearly communicate that the visible date range is scoped to that event's dates. This card is the **UI-only** portion: it adds two view-model fields (`EventStartDate`, `EventEndDate`), populates them in `handleMatrix` from the resolved event, and renders a human-readable "Event dates: MM/DD – MM/DD" label in the matrix-content template. The actual auto-scoping of `DateFrom`/`DateTo` to the event's dates is handled by the sibling backend card **t_8863d450** (which modifies `parseMatrixFilters`) — this card must **not** touch `parseMatrixFilters`.

## Constraints

- **Language:** Go (server), Go `html/template` (server-rendered templates), HTMX + Alpine.js (client behavior), vanilla CSS.
- **Framework:** R3 Intake Go server with embedded PocketBase.
- **Dependencies:** None new. Reuses existing `Event` struct (`StartDate`/`EndDate` fields already present) and the existing `{{slice . 5 7}}/{{slice . 8 10}}` MM/DD formatting pattern already used in the matrix table header.
- **Scope boundary:** Do **not** modify `parseMatrixFilters`. This card only adds view fields + template label.
- **Date format:** All dates are `YYYY-MM-DD` strings. Timestamps in HST.
- **Design system:** Public Sans + Lora, accent `#b5502e`, card `#fffdfa`, page bg `#f7f1e6`, 14px card radii, 8px input radii. Use existing `muted` class for the label to match the existing "Location:" line.

## File Structure

| File | Action | Notes |
|------|--------|-------|
| `r3-intake/internal/server/attendance.go` | **Modify** | Add `EventStartDate`/`EventEndDate` to `MatrixViewData`; populate them in `handleMatrix`. |
| `r3-intake/internal/assets/public/index.html` | **Modify** | Add event date-range label to the `matrix-content` template. |

No new files. No changes to `parseMatrixFilters`, `loadEvents`, or the `Event` struct.

## Implementation Notes

### 1. `MatrixViewData` struct (attendance.go, ~lines 16–40)

Add two fields next to the existing `EventLocation` field:

```go
// EventStartDate / EventEndDate are the selected event's date range
// (YYYY-MM-DD). They are empty when no event is selected or the event
// has no dates. Used to render the "Event dates" label.
EventStartDate string
EventEndDate   string
```

### 2. `handleMatrix` (attendance.go, ~lines 82–135)

Extend the existing event-resolution loop (currently lines 100–108, which only captures `eventLocation`). When `ev.ID == eventID`, also capture `ev.StartDate` and `ev.EndDate`:

```go
eventLocation := ""
eventStartDate := ""
eventEndDate := ""
if eventID != "" {
    for _, ev := range events {
        if ev.ID == eventID {
            eventLocation = s.nameFor("sites", ev.SiteID)
            eventStartDate = ev.StartDate
            eventEndDate = ev.EndDate
            break
        }
    }
}
```

Then add both to the `MatrixViewData` literal:

```go
EventLocation: eventLocation,
EventStartDate: eventStartDate,
EventEndDate:   eventEndDate,
```

**Key design decision:** Reuse the existing loop rather than adding a second pass — keeps the change minimal and guarantees the captured dates always correspond to the *effective* event (the one `effectiveEventID` resolved to, which may differ from the raw query param).

### 3. Template label (index.html, `matrix-content`)

Render the label inside the filter bar area, right after the `</form>` (or adjacent to the existing "Location:" line), guarded so it only shows when an event is selected **and** both dates are present:

```html
{{if and .EventStartDate .EventEndDate}}
<p class="muted">Event dates: {{slice .EventStartDate 5 7}}/{{slice .EventStartDate 8 10}} – {{slice .EventEndDate 5 7}}/{{slice .EventEndDate 8 10}}</p>
{{end}}
```

**Key design decision:** Use the same `{{slice . 5 7}}/{{slice . 8 10}}` slicing pattern already used in the matrix table header for consistency, and the existing `muted` class to match the "Location:" line's styling. The `and` guard handles the edge case where an event exists but has no dates.

**Edge cases handled:**
- **No event selected** (`EventID == ""`): both fields empty → label hidden.
- **Event selected but missing dates** (empty `StartDate`/`EndDate`): `and` guard → label hidden.
- **HTMX partial swap:** The label lives inside `#matrix-and-stats` (the `matrix-content` template root), so it re-renders correctly on every `hx-get="/attendance"` swap — no extra wiring needed.
- **Non-conflict with sibling:** We only *read* `ev.StartDate`/`ev.EndDate`; we never touch `parseMatrixFilters`. The from/to inputs remain visible and will reflect the event's dates via the sibling's change.

## Verification Criteria

1. **Build & static checks** (from `r3-intake/` or repo root):
   ```bash
   go build ./... && go vet ./... && go test ./...
   ```
   All must pass with no errors.

2. **Manual / functional check:**
   - Load `/attendance` with **no event selected** → no "Event dates" label appears; from/to inputs show the default 14-day window.
   - Select an **event that has start/end dates** → a `muted` "Event dates: MM/DD – MM/DD" label appears, matching the event's `start_date`/`end_date`; from/to inputs reflect the event's dates (via sibling card).
   - Select an **event with missing dates** (if any) → label hidden, no template error.
   - Confirm the label updates on HTMX partial swap (changing the event select re-renders the label without a full page reload).

3. **Regression:** Confirm the matrix table, walk-in panel, and stat cards still render unchanged, and that `parseMatrixFilters` was **not** modified (diff shows only the struct fields, the `handleMatrix` loop/literal, and the template label).
