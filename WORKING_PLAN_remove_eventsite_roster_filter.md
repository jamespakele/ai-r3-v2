# Working Plan: Remove eventSite roster filter in loadMatrixRows

## Objective
Make the participant roster in `loadMatrixRows` always the full site/role-scoped list, independent of the selected event. Event scoping applies **only** to the attendance map (`attMap`), never to which participants are listed. Concretely: delete the `case eventSite != ""` branch so `intakeFilter` is determined solely by role logic (`case_manager` → `assigned_to`, else `1=1`). Preserve the AC #1 comment. Update the integration test that asserts the old event-scoped roster behavior.

## Constraints
- **Do not** use `eventSite` for `intakeFilter` (acceptance criterion).
- **Do not** change attendance-map scoping: `attMap` must still be scoped by the selected event via `resolveEventIDs(eventID)`.
- Preserve the AC #1 comment (the "Participants: ALWAYS the full site/role-scoped roster…" comment).
- Code must compile; `go build ./...`, `go vet ./...`, `go test ./...` must pass.
- PocketBase v0.39 API only: `s.pb.FindCollectionByNameOrId`, `s.pb.FindRecordsByFilter`, `s.pb.FindRecordById`. No `app.dao()`. Filter escaping via `mcpmod.EscapeFilter`.

## File Structure
- `r3-intake/internal/server/attendance.go` — modify `loadMatrixRows` (~line 288).
- `r3-intake/internal/server/attendance_roster_integration_test.go` — update `TestMatrixRosterEventScoped` (~line 172).

## Implementation Notes

### 1. `attendance.go` — remove the event-site roster branch
In `loadMatrixRows`, replace the roster-scoping block:

```go
// Roster scope: the selected event's site when an event is selected;
// otherwise the role-based default (case_manager: assigned intakes;
// admin: all intakes).
eventSite := ""
if eventID != "" {
    eventsCol, err := s.eventsCollection()
    if err != nil {
        return nil, err
    }
    if eventRec, err := s.pb.FindRecordById(eventsCol.Id, eventID); err == nil {
        eventSite = eventRec.GetString("site")
    }
}
var intakeFilter string
switch {
case eventSite != "":
    intakeFilter = fmt.Sprintf("site='%s'", mcpmod.EscapeFilter(eventSite))
case u.Role == "case_manager":
    intakeFilter = fmt.Sprintf("assigned_to='%s'", mcpmod.EscapeFilter(u.ID))
default:
    intakeFilter = "1=1"
}
```

with:

```go
// Participants: ALWAYS the full site/role-scoped roster, independent of the
// selected event (AC #1). The event only scopes the attendance map below; it
// must never change which participants are listed.
var intakeFilter string
switch {
case u.Role == "case_manager":
    intakeFilter = fmt.Sprintf("assigned_to='%s'", mcpmod.EscapeFilter(u.ID))
default:
    intakeFilter = "1=1"
}
```

**Recommendation on `eventSite`:** **Keep** the `eventSite` variable and its lookup block (the `eventsCollection`/`FindRecordById` fetch) for `cellSiteID` at ~line 333. Rationale:
- The card forbids `eventSite` for `intakeFilter` only; it does not forbid it for `cellSiteID`.
- `cellSiteID` drives the disabled location textbox (`Disabled: cellSiteID == "" || eventID == ""`) and the `NoLocation` grouping. With the event-site roster filter removed, the roster is the full site/role-scoped list, so `cellSiteID` falling back to the event's site preserves the "selected event's location" display for the disabled box.
- Removing it entirely would change the disabled-box location semantics and the `NoLocation` grouping for event-selected views — a larger behavioral change than the card asks for.

So: keep the `eventSite` lookup block and the `cellSiteID := eventSite; if cellSiteID == "" { cellSiteID = rec.GetString("site") }` logic unchanged. Only the `switch` loses its `case eventSite != ""` branch. The `eventSite` variable remains used (for `cellSiteID`), so no "declared but not used" compile error.

### 2. `attendance_roster_integration_test.go` — update `TestMatrixRosterEventScoped`
The test currently asserts the OLD behavior (with `ev1`/Kona selected, only Kona intakes render). After the change, with an event selected the roster must equal the full admin list — identical to the no-event case.

- Update the doc comment: the roster is now the full site/role-scoped list regardless of event; only the attendance map is event-scoped.
- Change `wantWithEvent` from `[]string{fx.iInSite1, fx.iInSite2, fx.iAssignedCM}` to the full list `[]string{fx.iInSite1, fx.iInSite2, fx.iOtherSite, fx.iAssignedCM}` (same as `wantNoEvent`).
- **Remove** the "out-of-site walk-in is never rendered" loop — `iOtherSite` (Carol, Hilo) is now a valid roster row even with `ev1` selected.
- **Preserve** the attendance-map scoping assertions unchanged:
  - `cellStatus(withEvent, fx.iInSite1, "2026-08-13") == "present"` (ev1 record in range).
  - `cellStatus(withEvent, fx.iInSite2, "2026-08-13") == ""` (its record is ev2, not ev1).
  - `cellStatus(noEvent, fx.iInSite1, "2026-08-13") == "present"`.
  - `cellStatus(noEvent, fx.iInSite2, "2026-08-13") == "present"` (ev2 record in range).

Note: `TestMatrixNoEventAdminAllIntakes`, `TestMatrixNoEventEmptyCells`, and `TestExportRowsEventScoping` are unaffected (they already exercise no-event / export paths and remain valid).

## Verification Criteria
1. `cd r3-intake && go build ./...` — compiles cleanly.
2. `cd r3-intake && go vet ./...` — no vet warnings.
3. `cd r3-intake && go test ./...` — all tests pass, including the updated `TestMatrixRosterEventScoped`.
4. Grep confirms `eventSite` is **not** referenced in the `intakeFilter` switch (only in the `cellSiteID` fallback).
5. Attendance map still scoped by selected event: `resolveEventIDs(eventID)` and the `attFilter` event-OR clause are unchanged.
