# Working Plan: Replace union filter with strict attendance-based filter in admin.go

## Objective
Replace the current union-based event filter in the admin Records list (which shows intakes matching `intake.event = selected_event` OR having attendance for that event) with a strict attendance-only filter that returns ONLY intakes with attendance records for the selected event, where the attendance date falls within the event's date range.

## Constraints
- PocketBase v0.39 has no `IN` operator, requiring OR-joined `id='<id>'` clauses
- Server-rendered Go templates with HTMX, no client-side filtering
- Must maintain compatibility with existing status and text search filters
- Test infrastructure uses specific helpers (do not redefine)
- Event date range scoping must mirror the matrix's `parseMatrixFilters` behavior

## File Structure

### Files to Modify
1. **r3-intake/internal/server/admin.go** (lines ~104-136)
   - Remove the `event='<id>'` home-event matching branch
   - Add event date lookup to constrain attendance query
   - Handle edge cases (missing event, invalid dates)
   - Ensure empty attendance returns no results (no fallback)

2. **r3-intake/internal/server/records_list_attendance_join_integration_test.go**
   - Update `TestListEventFilterDedupsHomeAndAttendance`: ev1 filter expects [Charlie] only (not [Alice Charlie])
   - Add test for date-range constraint (attendance outside event dates should not surface)

3. **r3-intake/internal/server/records_list_integration_test.go**
   - Update `TestListEventFilterJoinsAttendance` subtests:
     - "event with attendance from other-home-event intake": expects [Alice] only (not [Alice Bob])
     - "event with no cross-event attendance": expects [] (not [Alice])
     - "event with no attendance falls back to home event": expects [] (not [Alice])
   - Add fixture/test for attendance date outside event range

4. **r3-intake/README.md** (lines ~251-258)
   - Update documentation to reflect strict attendance-only behavior
   - Note that attendance is the source of truth, dates must fall within range
   - Remove mention of home-event fallback

## Implementation Notes

### Key Changes to handleList (admin.go)
1. When `eventFilter` is set:
   - Query the `events` collection to get the specific event record
   - Extract `start_date` and `end_date` from the event
   - Modify attendance query to include date constraint: `event='<id>' && date>='<start>' && date<='<end>'`
   - Build filter with ONLY attendance-derived intake IDs: `(id='<id1>' || id='<id2>' || ...)`
   - If no attendance records match, use a filter that matches nothing (e.g., `id=''`)

2. Error handling:
   - If event record cannot be loaded: log and treat as no matches
   - If dates are empty/invalid: proceed without date constraint (defensive fallback)

### Test Modifications
1. Existing tests expecting home-event matches must be adjusted to expect empty results or attendance-only results
2. Add new test case with attendance record dated outside event range (e.g., 2026-09-01 for event ending 2026-08-31)
3. Use existing test helpers without redefining them

## Logical Consequences

### admin.go eventFilter block (lines ~104-136)
- **REMOVE**: `ors = append(ors, fmt.Sprintf("event='%s'", escapedEvent))` line
- **ADD**: Event lookup and date extraction before attendance query
- **MODIFY**: Attendance query filter to include date constraints
- **CHANGE**: Empty attendance case to return no results (not home-event fallback)

### records_list_attendance_join_integration_test.go
- **TestListEventFilterDedupsHomeAndAttendance**:
  - ev1 filter: CHANGE expected from [Alice Charlie] to [Charlie]
  - ev2 filter: KEEP expected [Alice Bob]
- **TestListEventFilterSurfacesMultipleAttendees**: KEEP as-is (both have ev2 attendance)
- **TestListEventFilterComposesWithSearch**: KEEP as-is (Alice has ev2 attendance)
- **TestListEventFilterComposesWithStatusAndSearch**: KEEP as-is (Alice has ev2 attendance)
- **TestListEventFilterCrossEventDistinct**: KEEP as-is (correct behavior)
- **TestListNoEventFilterReturnsAll**: KEEP as-is (no event filter)
- **ADD**: New test for date-range constraint validation

### records_list_integration_test.go
- **TestListEventFilterJoinsAttendance**:
  - "event with attendance from other-home-event intake": CHANGE expected from [Alice Bob] to [Alice]
  - "event with no cross-event attendance": CHANGE expected from [Alice] to []
  - "union composes with status filter": KEEP as-is (Alice has ev2 attendance)
  - "event with no attendance falls back to home event": CHANGE expected from [Alice] to []

### README.md (lines ~251-258)
- **REMOVE**: Description of union behavior and home-event fallback
- **ADD**: Description of strict attendance-only filtering with date-range constraint
- **KEEP**: Note about PocketBase v0.39 lacking IN operator

## Verification Criteria
1. **Functional Requirements**:
   - Filtering by "R3 - Sprng 2027" returns exactly 4 people
   - Filtering by "R3 - Fall 2026" returns exactly 3 people
   - Filtering by "R3 - Fall 2026 Waianae" returns exactly 4 people
   - No intakes appear based solely on home event matching

2. **Technical Requirements**:
   - Code compiles without errors
   - All existing tests pass after adjustments
   - New date-range constraint test passes
   - Attendance dates outside event range do not surface intakes
   - Empty attendance returns no results (no fallback)

3. **Edge Cases Handled**:
   - Event not found: treat as no matches
   - Invalid/empty dates: proceed without date constraint
   - No attendance records: return empty result set
   - Date parsing errors: defensive handling
