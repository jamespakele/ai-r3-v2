# Working Plan: Scope attendance matrix by selected event

## Objective

Make `loadMatrixRows` always load the **full site/role-scoped participant roster** regardless of which event is selected, so the participant list renders identically whether or not an event is chosen (AC #1). The selected event must only scope the **attendance map** (which records are shown/edited), never the roster.

## Constraints

- **AC #1 (hard):** Roster must be byte-identical whether `eventID == ""` or `eventID == "ev1"`. Selecting a different event changes only which attendance cells are populated.
- PocketBase v0.39 Go API only (`s.pb.FindCollectionByNameOrId`, `s.pb.FindRecordsByFilter`, `s.pb.FindRecordById`, `s.pb.Save`, `s.pb.Delete`). No `app.dao()`.
- Filter escaping via `mcpmod.EscapeFilter(s)` (`mcpmod "r3-intake/internal/mcp"`).
- `attendance.event` is required (migration 010); uniqueness keyed on `(event, intake, date)`.
- `handleToggle`/`handleWalkin`/`handlePersonAttendanceDaySave` already gate on `requireEventID` (400) — no change needed there.
- Verification gates: `cd r3-intake && go build ./... && go vet ./... && go test ./...` must all pass.

## File Structure

| File | Role |
|------|------|
| `r3-intake/internal/server/attendance.go` | `loadMatrixRows` (line 237) — the only required code change |
| `r3-intake/internal/assets/public/index.html` | `matrix-cell` template (~line 983) — optional aria-label generalization |
| `r3-intake/internal/server/attendance_test.go` | Existing template render tests (no change required) |
| `r3-intake/internal/server/attendance_toggle_integration_test.go` | Existing toggle tests (no change required) |
| `r3-intake/internal/server/attendance_roster_integration_test.go` | **New** — roster-identity integration test |

## Implementation Notes

### 1. `loadMatrixRows` — remove the event-scoped roster branch (attendance.go ~lines 247–306)

Replace the entire `if eventID != "" { ... } else { ... }` participant-loading block with the full-roster query only. Delete the `event_enrollment` query, the walk-in union query, and the `ids` map. The `else` branch becomes unconditional:

```go
	// Participants: ALWAYS the full site/role-scoped roster, independent of
	// the selected event (AC #1). The event only scopes the attendance map
	// below; it must never change which participants are listed.
	var intakeFilter string
	switch {
	case u.Role == "case_manager":
		intakeFilter = fmt.Sprintf("assigned_to='%s'", mcpmod.EscapeFilter(u.ID))
	case siteID != "":
		intakeFilter = fmt.Sprintf("site='%s'", mcpmod.EscapeFilter(siteID))
	default:
		intakeFilter = "1=1"
	}
	intakeRecs, err := s.pb.FindRecordsByFilter(intakeCol.Id, intakeFilter, "name", 1000, 0)
	if err != nil {
		return nil, err
	}
```

**Do NOT touch** the attendance-map block below it (lines ~307–329). It already scopes correctly: `attFilter` adds `&& event='%s'` only when `eventID != ""`, and the `site` clause `(site='' || site='%s')` is applied regardless. This is the only place `eventID` should influence output.

### 2. Walk-in participants — recommendation: do NOT union into the roster

**Recommended approach: no union.** Keep the roster purely site/role-scoped. Rationale:

- **Walk-in-created intakes already appear naturally.** The name-only walk-in path (`handleWalkin`, attendance.go ~line 725) sets `rec.Set("site", siteID)` and `status="unassigned"`. Since the walk-in panel is site-scoped (`handleWalkinSearch` filters by resolved site), any intake created or selected through it is already within the site scope and therefore appears in the full roster on the next render.
- **Unioning would violate AC #1.** If we unioned walk-in intake IDs into the roster, the roster would differ between "no event" (no walk-ins for any event) and "event selected" (walk-ins for that event) — exactly the divergence AC #1 forbids.
- **Out-of-scope walk-ins are acceptable to omit.** A walk-in for an intake outside the current site/role scope (e.g. a `case_manager` walking in an intake not assigned to them) will still be **recorded** in the attendance map (it's in `attMap`), but simply won't render as a row. That is correct behavior for a site/role-scoped roster and keeps the roster deterministic.

### 3. Template — generalize the disabled-dot aria-label (index.html ~line 983)

Cells are now disabled for two reasons: no location **or** no event selected. The current label `"Attendance requires a location"` is misleading when the cause is a missing event. Generalize to a neutral label:

```html
{{if .Disabled}}<span class="matrix-dot dot-disabled" aria-label="Attendance unavailable"></span>
```

This is cosmetic/accessibility-only; the `Disabled` field is already set correctly in `loadMatrixRows` (`Disabled: cellSiteID == "" || eventID == ""`). No logic change.

### 4. New integration test — `attendance_roster_integration_test.go`

Add a test asserting the roster is identical with and without an event selected. It should:

- Seed an intake collection with a known set of intakes (some in-site, some out-of-site, one assigned to a case_manager).
- Seed an `event_enrollment` record for one intake and a `walk_in` attendance record for another intake **not** in the site scope.
- Call `s.loadMatrixRows(u, siteID, dates, "ev1", to)` and `s.loadMatrixRows(u, siteID, dates, "", to)`.
- Assert both return the **same ordered set of `IntakeID`s** (the full site-scoped roster), and that the event-scoped call does **not** include the out-of-scope walk-in intake as a row.
- Assert the attendance map differs: the event-scoped call populates cells only for records with `event='ev1'`, while the no-event call populates cells for all records in the date range.

Follow the existing integration-test pattern in `attendance_toggle_integration_test.go` (in-memory PocketBase setup, `s.pb.Save` seeding, `requireEventID`-style assertions).

## Verification Criteria

1. **Roster identity (AC #1):** `loadMatrixRows` returns the identical ordered `IntakeID` list for `eventID == ""` and `eventID == "ev1"` — proven by the new integration test.
2. **Event scoping preserved:** With an event selected, only that event's attendance records populate cells; with no event, all in-range records populate cells. The `attFilter` `&& event='%s'` clause is unchanged.
3. **Walk-in behavior:** A walk-in-created intake (site set) appears in the roster naturally; an out-of-scope walk-in intake is recorded but not rendered as a row.
4. **No regression:** `handleToggle`/`handleWalkin`/`handlePersonAttendanceDaySave` still 400 without an event (unchanged).
5. **Gates pass:** `cd r3-intake && go build ./... && go vet ./... && go test ./...` all green, including the new test and the existing `TestMatrixContentRender*` / toggle tests.
