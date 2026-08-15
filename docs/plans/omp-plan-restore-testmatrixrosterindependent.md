# Working Plan

## Objective

Restore `TestMatrixRosterEventScoped` to its original intent as `TestMatrixRosterEventIndependent` in `r3-intake/internal/server/attendance_roster_integration_test.go`. The test must assert that the roster (participant list) is **identical** whether or not an event is selected, and that attendance dots differ only for the attendee whose record belongs to a different event. All assertions that expect event-site filtering must be removed.

## Constraints

- **Do NOT restore the old 5-arg signature.** The pre-epic-19 test used `srv.loadMatrixRows(admin, fx.site, dates, fx.ev1, "2026-08-13")`. The current code is `loadMatrixRows(u *sessionUser, dates []string, eventID, to string)` (4 args, no site param) — the restored test must compile against this.
- Keep the existing seed data and attendance setup unchanged:
  - `saveEnrollment(t, srv.pb, fx.ev1, fx.iInSite1)`
  - `saveAttendance` for `iInSite1@site/ev1 present`, `iInSite2@site/ev2 present`, `iOtherSite@site2/ev1 walk_in` on `2026-08-13`.
- Only change the roster assertions and remove the event-site-filtering assertion.
- The "out-of-site walk-in is never rendered as a row" check must **remain** (it asserts the walk-in is never a row, which is still valid).
- The sibling card `t_e0d89c58` removes the eventSite roster filter, so for an admin the full roster is all intakes: `[iInSite1, iInSite2, iOtherSite, iAssignedCM]`.

## File Structure

- **File modified:** `r3-intake/internal/server/attendance_roster_integration_test.go`
- **Function renamed:** `TestMatrixRosterEventScoped` → `TestMatrixRosterEventIndependent`
- **No new files.** All helper functions (`saveEnrollment`, `cellStatus`, `equalStrings`, `saveAttendance`, `seedRosterData`) already exist in the file and are reused as-is.

## Implementation Notes

1. **Rename the test function** and update its doc comment to describe roster independence rather than event-site scoping.

2. **Roster assertions — both calls must return the SAME full list.** Replace the two divergent expectations with a single shared expectation:
   ```go
   want := []string{fx.iInSite1, fx.iInSite2, fx.iOtherSite, fx.iAssignedCM} // Alice, Bob, Carol, Dana
   if got := idsOf(withEvent); !equalStrings(got, want) {
       t.Errorf("roster with event = %v, want %v", got, want)
   }
   if got := idsOf(noEvent); !equalStrings(got, want) {
       t.Errorf("roster without event = %v, want %v", got, want)
   }
   ```
   This removes the old `wantWithEvent := [iInSite1, iInSite2, iAssignedCM]` (Kona-only, event-site filtered) and the old `wantNoEvent` divergence.

3. **Remove the event-site-filtering assertion.** The old block that asserted `withEvent` excludes `iOtherSite` because "the event's site scopes the roster" must be deleted. Replace it with a **neutral** check that the out-of-site walk-in is never rendered as a row in **either** result (this preserves the walk-in-never-a-row guarantee without implying event-site filtering):
   ```go
   // The out-of-site walk-in is recorded but never rendered as a row.
   for _, rows := range [][]MatrixRow{withEvent, noEvent} {
       for _, r := range rows {
           if r.IntakeID == fx.iOtherSite {
               t.Errorf("out-of-site walk-in intake rendered as a row")
           }
       }
   }
   ```

4. **Attendance-dot assertions — keep unchanged.** These already encode the correct independence semantics and must be preserved verbatim:
   - `withEvent(ev1)`: `iInSite1` → `"present"`; `iInSite2` → `""` (its record is ev2, not ev1).
   - `noEvent`: `iInSite1` → `"present"`; `iInSite2` → `"present"` (all in-range records regardless of event).
   These demonstrate that dots differ only for the attendee whose record belongs to a different event.

5. **Keep the `idsOf` closure** (defined inline in the test) — it is used by the new roster assertions.

## Verification Criteria

- `TestMatrixRosterEventIndependent` compiles against the current 4-arg `loadMatrixRows(u *sessionUser, dates []string, eventID, to string)` signature.
- The test asserts the **same** participant list `[iInSite1, iInSite2, iOtherSite, iAssignedCM]` for both `withEvent` and `noEvent`.
- No assertion expects event-site filtering (no Kona-only expectation, no "event's site scopes the roster" comment).
- The out-of-site walk-in (`iOtherSite`) is still asserted to never render as a row.
- Attendance dots: `withEvent(ev1)` → `iInSite1=present`, `iInSite2=""`; `noEvent` → `iInSite1=present`, `iInSite2=present`.
- `go test ./internal/server/ -run TestMatrixRosterEventIndependent` passes (and the old `TestMatrixRosterEventScoped` name no longer exists).
