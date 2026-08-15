# Working Plan: Migrate intake site field to event reference

## Objective

Rename the `intake.site` relation field (currently a relation to the `sites`
collection, used for roster scoping and required-field validation) to
`intake.event`, repurposing it as a relation to the `events` collection. This
is the schema-migration card in the epic "Replace intake Site/Location with
Event". It owns the PocketBase schema migration (016) plus the Go field
renames that read/write `intake.site` -> `intake.event`. The sibling UI cards
(form dropdown t_146484f2, participant-list Site column t_fb0d46ce, admin tab
reorder t_35775bf1) depend on the field name and view-model fields defined
here.

## Constraints

- Go server + embedded PocketBase v0.39, server-rendered Go templates, HTMX +
  Alpine.js. All timestamps HST. Design system Public Sans + Lora, accent
  #b5502e.
- RENAME `intake.site` -> `intake.event` as a relation to `events` (NOT
  repurpose in place). Sibling cards read the field name defined here.
- Migration numbering: next available is 016. File
  `016_intake_site_to_event.go`, registered in `migrations.go` via
  `migrations.Register(upX, downX, "016_intake_site_to_event.go")` (filename
  passed LAST per the v0.39 signature).
- The `events` collection id must be resolved at migration time via
  `app.FindCollectionByNameOrId("events")` -> `.Id` for the RelationField
  `CollectionId`.
- New `intake.event` field: `required: false` (matches current `site`
  optionality), `maxSelect: 1`, `cascadeDelete: false`.
- Do NOT touch the template dropdown/column UI - that is the siblings' job.
  But the Go view-model fields (FormState.SiteSel->EventSel,
  IntakeRow.SiteName->EventName) and REQUIRED_FIELDS rename ARE this card's
  job since siblings read them.
- Do NOT rename `events.site` (the EVENT's location) - that stays. Only
  `intake.site` becomes `intake.event`.
- Idempotency guards on both up and down migrations (check field presence
  before acting).
- Backfill is best-effort: map each intake's old site value to an active event
  at that site, or leave empty.

## File Structure

Create:
- `r3-intake/pocketbase/migrations/016_intake_site_to_event.go` - the schema
  migration (up: remove intake.site, add intake.event -> events, backfill;
  down: remove intake.event, re-add intake.site -> sites, backfill).

Modify:
- `r3-intake/pocketbase/migrations/migrations.go` - register 016.
- `r3-intake/internal/server/handlers.go` - REQUIRED_FIELDS "site"->"event";
  FormState.SiteSel->EventSel; blankState default-site -> default-event;
  stateFromRecord SiteSel: rec.GetString("site")->rec.GetString("event");
  applySection rec.Set("site",...)->rec.Set("event",...);
  applyToState st.SiteSel->st.EventSel; validateState check("site",...)->
  check("event",...); Progress() st.SiteSel->st.EventSel.
- `r3-intake/internal/server/admin.go` - IntakeRow.SiteName->EventName;
  participant-list Go resolution row.SiteName = siteMap[rec.GetString("site")]
  -> event name resolution (soft-delete aware); the ?site= participant-list
  filter (admin.go:105-110) -> ?event= filter on intake.event.
- `r3-intake/internal/server/attendance.go` - resolveSite (case_manager
  derives site from assigned intakes rec.GetString("site")->rec.GetString("event"));
  loadMatrixRows cellSiteID = rec.GetString("site") (NoLocation grouping);
  handleToggle siteID = rec.GetString("site") (intake home site derivation);
  handleWalkin rec.Set("site", siteID) on the INTAKE record -> rec.Set("event",
  eventID).
- `r3-intake/internal/server/person_attendance.go` - SiteName:
  s.nameFor("sites", intake.GetString("site")) -> event name resolution.

Modify (tests - seed intake records with event id instead of site id):
- `r3-intake/internal/server/attendance_roster_integration_test.go`
- `r3-intake/internal/server/attendance_export_integration_test.go`
- `r3-intake/internal/server/attendance_toggle_integration_test.go`
- `r3-intake/internal/server/person_attendance_integration_test.go`

Do NOT modify (owned by siblings / out of scope):
- `r3-intake/internal/assets/public/index.html` site-fragment + "R3 Site
  Location" field group (t_146484f2 dropdown).
- The participant-list Site column template (t_fb0d46ce).
- Admin tab ordering (t_35775bf1).
- `events.site` reads (attendance.go:298, 453, 963; admin.go:678, 685) - these
  are the EVENT's location and stay as-is.

## Implementation Notes

### 1. Migration 016_intake_site_to_event.go

Follow the established pattern in `015_attendance_remove_site.go` (uses
`app.FindCollectionByNameOrId`, `col.Fields.GetByName`, `col.Fields.RemoveByName`,
`col.Fields.Add(&core.RelationField{...})`, `app.Save(col)`).

Up migration:
1. `intakeCol, err := app.FindCollectionByNameOrId("intake")`.
2. Idempotency guard: if `intakeCol.Fields.GetByName("event") != nil`, no-op
   return nil (field already added).
3. Resolve `eventsCol, err := app.FindCollectionByNameOrId("events")`; if it
   fails, return the error (we need its Id for the RelationField).
4. Remove the old field: `intakeCol.Fields.RemoveByName("site")`.
5. Add the new field:
   `intakeCol.Fields.Add(&core.RelationField{Name: "event", CollectionId:
   eventsCol.Id, Required: false, MaxSelect: 1, CascadeDelete: false})`.
6. `app.Save(intakeCol)`.
7. Backfill (best-effort): load all intake records
   (`app.FindRecordsByFilter(intakeCol.Id, "", "", 100000, 0)`). For each rec,
   read the OLD site value BEFORE removing it - so capture the site value in
   step 4/5 loop, or read it from a pre-save snapshot. Recommended order:
   load all recs first, capture `oldSite := rec.GetString("site")` per rec,
   then remove field + add field + save schema, then for each rec pick an
   active event at that site and `rec.Set("event", eventID)` + `app.Save(rec)`.
   To pick an event at a site: query events with
   `site='<oldSite>' && status='active' && (deleted = false || deleted = null)`
   (use `mcpmod.EscapeFilter` for the site id), take the first by
   `start_date,name`; if none found, leave `event` empty (field is optional).
   Log a warning when no event is found so the operator can reconcile.

Down migration:
1. `intakeCol, err := app.FindCollectionByNameOrId("intake")`.
2. Idempotency guard: if `intakeCol.Fields.GetByName("site") != nil`, no-op.
3. Resolve `sitesCol` and `eventsCol`.
4. Load all intake recs, capture `oldEvent := rec.GetString("event")` per rec.
5. Remove `event`, add `site` as RelationField to sites (required false,
   maxSelect 1, cascadeDelete false), save schema.
6. Backfill: for each rec, resolve its event via
   `app.FindRecordById(eventsCol.Id, oldEvent)`; if found and the event has a
   site, `rec.Set("site", eventSite)` + save; else leave empty (best-effort).

### 2. Go field renames (handlers.go)

- `REQUIRED_FIELDS` (handlers.go:51): `"site"` -> `"event"`.
- `FormState.SiteSel` (handlers.go:62) -> `FormState.EventSel`. Sibling card
  t_146484f2 reads this for the dropdown.
- `Progress()` (handlers.go:150): `st.SiteSel` -> `st.EventSel`.
- `blankState` (handlers.go:228): the default-site loop sets `st.SiteSel =
  site.ID`. Change to default-event: load active events (reuse
  `s.loadEvents()` which returns active, non-deleted events) and set
  `st.EventSel = events[0].ID` (or the first event; there is no "default
  event" concept, so pick the first active event). Keep `Sites` in the view
  model for now (the sibling dropdown card owns the template; the Go view
  model may still need Sites for other sections).
- `stateFromRecord` (handlers.go:248): `SiteSel: rec.GetString("site")` ->
  `EventSel: rec.GetString("event")`.
- `applySection` (handlers.go:533): `rec.Set("site", r.FormValue("site"))` ->
  `rec.Set("event", r.FormValue("event"))`.
- `applyToState` (handlers.go:618): `st.SiteSel = r.FormValue("site")` ->
  `st.EventSel = r.FormValue("event")`.
- `validateState` (handlers.go:681): `check("site", st.SiteSel)` ->
  `check("event", st.EventSel)`.

### 3. Go field renames (admin.go - participant list)

- `IntakeRow.SiteName` (admin.go:23) -> `IntakeRow.EventName`. Sibling card
  t_fb0d46ce reads this for the column.
- Participant-list Go resolution (admin.go:136): `row.SiteName =
  siteMap[rec.GetString("site")]` -> resolve the event name. Use
  `s.nameFor("events", rec.GetString("event"))` (attendance.go:981) which
  returns "" for missing/soft-deleted events; keep the "—" fallback
  (admin.go:137-138). Note: `nameFor` does NOT filter soft-deleted events, so
  a soft-deleted event still shows its name - acceptable for the participant
  list (the event is referenced by the intake). If a stricter behavior is
  desired, resolve via `loadAllEvents()` map (deleted=false) and fall back to
  "—" for soft-deleted events.
- Participant-list filter (admin.go:105-110): the `?site=` query param filters
  `site='<id>'` on intake. Rename to `?event=` filtering `event='<id>'` on
  intake. Update `view.SiteFilter` -> `view.EventFilter` (AdminView field) and
  the template reference is owned by t_fb0d46ce. Keep the filter value as an
  event record ID.

### 4. Go field renames (attendance.go - intake.site reads)

These read the INTAKE's home site and must become intake.event:
- `resolveSite` (attendance.go:249): `sid := rec.GetString("site")` ->
  `rec.GetString("event")`. This derives a case_manager's site from assigned
  intakes. NOTE: this now derives from the intake's EVENT, not its site. The
  event's site is the location; if the intent is still "which site does this
  case manager serve", resolve the event's site via
  `s.nameFor("sites", eventRec.GetString("site"))` or keep the event id as the
  grouping key. Decide based on how `counts` is consumed (it feeds the
  case_manager's default site filter). If the matrix is now event-scoped, the
  event id is the correct key; if it must remain a site, resolve through the
  event. Document the choice in the code comment.
- `loadMatrixRows` (attendance.go:354): `cellSiteID = rec.GetString("site")`
  (NoLocation grouping fallback) -> `rec.GetString("event")`. This is the
  intake's home event used when the matrix has no event filter.
- `handleToggle` (attendance.go:549, 554): `siteID = rec.GetString("site")`
  (intake home site derivation) -> `rec.GetString("event")`.
- `handleWalkin` (attendance.go:769): `rec.Set("site", siteID)` on the INTAKE
  record -> `rec.Set("event", eventID)`. The walk-in creates a new intake; the
  event id (from the walk-in form) is set on the intake.

Do NOT change these (they read `events.site`, the EVENT's location):
- attendance.go:298 `eventSite = eventRec.GetString("site")`
- attendance.go:453 `SiteID: r.GetString("site")` (Event struct, event's site)
- attendance.go:963 `siteName = s.nameFor("sites", eventRec.GetString("site"))`
- admin.go:678, 685 (EventRow SiteID/SiteName, event's site)

### 5. Go field renames (person_attendance.go)

- person_attendance.go:183: `SiteName: s.nameFor("sites",
  intake.GetString("site"))` -> resolve the intake's event name:
  `s.nameFor("events", intake.GetString("event"))`. Rename the struct field
  `SiteName` -> `EventName` (person_attendance.go:26) if the template reads it
  (the person-attendance template is not owned by a sibling card in this epic,
  so this rename is safe here; verify the template reference).

### 6. Test updates

The tests seed intake records with `r.Set("site", siteID)`. Change to
`r.Set("event", eventID)` using an event id. All four test files already
create events (ev1/ev2/ev), so reference those ids:
- `attendance_roster_integration_test.go` seedRosterData: `r.Set("site", site)`
  -> `r.Set("event", ev1)` (or ev2 for the other-site intake). The fixtures
  already return ev1/ev2.
- `attendance_export_integration_test.go` seedExportData: `r.Set("site",
  site1)` -> `r.Set("event", ev1)`; `r.Set("site", site2)` -> `r.Set("event",
  ev2)`.
- `attendance_toggle_integration_test.go` seedToggleData: `r.Set("site",
  site)` -> `r.Set("event", ev)`.
- `person_attendance_integration_test.go`: `r.Set("site", site)` ->
  `r.Set("event", ev)`.

Do NOT touch the `attendance.site` assertions in these tests (e.g.
attendance_toggle_integration_test.go:147-148, person_attendance:326-327) -
those assert attendance.site is empty (already removed by 015) and are
unrelated to intake.site.

### 7. Soft-delete awareness

The `events` collection has a `deleted` bool (soft-delete, migration 014).
When resolving an event name for the participant list, prefer
`loadAllEvents()` (deleted=false) or `nameFor` (no filter). For the backfill
in migration 016, only pick ACTIVE, non-deleted events
(`status='active' && (deleted = false || deleted = null)`) so a soft-deleted
event is never assigned as an intake's home event. Remember the bool-NULL trap:
use `(deleted = false || deleted = null)` in filters, not bare `deleted=false`.

## Verification Criteria

1. Migration compiles and registers: `016_intake_site_to_event.go` exists in
   `r3-intake/pocketbase/migrations/` and `migrations.go` has the
   `migrations.Register(upIntakeSiteToEvent, downIntakeSiteToEvent,
   "016_intake_site_to_event.go")` line.

2. Build/vet/test pass. Run from the worktree root:
   - `cd r3-intake && go build ./...`
   - `cd r3-intake && go vet ./...`
   - `cd r3-intake && go test ./...`
   (Run these as SEPARATE commands, not chained with `&&`, to avoid the
   terminal backgrounding guard.)

3. No remaining references to `intake.site` in Go source. Grep for
   `GetString("site")` / `Set("site"` in handlers.go, admin.go, attendance.go,
   person_attendance.go and confirm every remaining hit is on the `events`
   collection (event's location) or the `sites` collection - NOT on intake.
   The intake reads must all be `GetString("event")` / `Set("event", ...)`.

4. The four integration test files seed intake records with `r.Set("event",
   <eventID>)` (not `r.Set("site", <siteID>)`), and the tests pass.

5. Migration idempotency: running the up migration twice is a no-op (guard on
   `event` field presence); running down then up restores state. Verify by
   inspecting the migration guards.

6. Backfill correctness: after up-migration, every intake that previously had
   a `site` value pointing at a site with an active event now has `event` set
   to that event; intakes with no matching active event have `event` empty
   (field optional). Down-migration restores `site` from each intake's event's
   site.

7. View-model contract for siblings: `FormState.EventSel` (not SiteSel) and
   `IntakeRow.EventName` (not SiteName) exist and are populated, so sibling
   cards t_146484f2 and t_fb0d46ce can read them. `REQUIRED_FIELDS` contains
   "event" (not "site").

8. `events.site` (the EVENT's location) is untouched: attendance.go:298, 453,
   963 and admin.go:678, 685 still read `GetString("site")` on the events
   collection.

9. The plan file itself is intact: all sections present (`grep -n '^## '`),
   no leftover `@AMP@` sentinels, and Go filter operators with `&&` survived
   (verify with `grep -n 'deleted=false'` or `grep -n 'EscapeFilter'`, NOT
   `grep -n '&&'` which the terminal guard mangles).
