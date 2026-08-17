# Working Plan: Remove Unused EventName Resolution from IntakeRow

## Objective
Remove the dead `EventName` field from the `IntakeRow` struct and its population logic in `handleList`, along with removing the Event column from the participant list table in the HTML template. This field became unused when the Event column was removed from the Records participant list table.

## Constraints
- Language: Go 1.21+
- Framework: Server-rendered Go templates with embedded PocketBase
- Frontend: HTMX + Alpine.js, vanilla CSS
- All files are embedded in the binary
- Timestamps in HST

## File Structure
Files to modify:
1. `r3-intake/internal/server/admin.go` - Remove IntakeRow.EventName field and its population
2. `r3-intake/internal/assets/public/index.html` - Remove Event column from participant list table

## Implementation Notes

### Key Changes:
1. **admin.go**:
   - Remove `EventName string` field from `IntakeRow` struct (line 23)
   - Remove the EventName population block in `handleList` (lines 162-166)
   - Keep the `nameFor` helper function - it's used elsewhere
   - Keep `EventName` field in `AdminView` struct (line 41) - used for event creation form

2. **index.html**:
   - Remove `<th>Event</th>` from table header (line 597)
   - Remove `<td>{{.EventName}}</td>` from table row (line 604)
   - Update empty-state colspan from 8→7 for admin, 7→6 for non-admin (line 617)

### Edge Cases:
- The `nameFor` helper function is still actively used in multiple places (attendance views, person views) - must be kept
- The event filter dropdown functionality must remain intact - it's used for filtering the list
- Other view models have their own `EventName` fields that serve different purposes

## Logical Consequences

### Template References to Trace:
1. **Line 604: `<td>{{.EventName}}</td>`** → **REMOVE** (IntakeRow.EventName in participant list)
2. **Line 739: `value="{{.EventName}}"`** → **KEEP** (AdminView.EventName in event create form)
3. **Line 871: `{{if .EventName}}true{{else}}false{{end}}`** → **KEEP** (AdminView.EventName for tab activation)
4. **Line 1317: `<h2>Report — {{.EventName}}</h2>`** → **KEEP** (Different view model - Report view)
5. **Line 1388: `{{if .EventName}} ({{.EventName}}){{end}}`** → **KEEP** (Different view model - Attendance view)

### Other Components:
- **`nameFor` helper function** → **KEEP** (used in attendance.go, person_attendance.go, tests)
- **Event filter dropdown** → **KEEP** (still functional for filtering records)
- **AdminView.EventName field** → **KEEP** (used for event creation form state)
- **EventFilter field and logic** → **KEEP** (active filtering functionality)

## Verification Criteria

1. **Compilation**: The Go server should compile without errors after removing the IntakeRow.EventName field
2. **Template Rendering**: The participant list should render without the Event column
3. **Column Count**: The empty state colspan should correctly show 7 columns for admin users, 6 for non-admin
4. **No Regression**:
   - Event creation form should still work (uses AdminView.EventName)
   - Report view should still show event names
   - Attendance view should still show event names
   - Event filtering should continue to work
5. **Visual Check**: The participant list table should have columns: [checkbox if admin], Participant, SSN, Status, Assigned, Created, Actions
6. **Functional Test**: 
   - Load `/admin` page and verify no Event column in Records tab
   - Create a new event and verify it works
   - Filter by event and verify filtering still functions
