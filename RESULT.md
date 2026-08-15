# RESULT — t_27c37c36: Migrate intake site field to event reference

## What was built
Renamed the `intake.site` relation (→ `sites`) to `intake.event` (→ `events`)
across the schema and Go code, per the MOA working plan
(`docs/plans/omp-plan-intake-site-to-event.md`).

## Artifacts
- `r3-intake/pocketbase/migrations/016_intake_site_to_event.go` — new migration
  (up: remove intake.site, add intake.event → events, best-effort backfill to
  first active non-deleted event at the old site; down: reverse + backfill
  site from event's site). Idempotency guards on both directions.
- `r3-intake/pocketbase/migrations/migrations.go` — registered 016.
- `r3-intake/internal/server/handlers.go` — REQUIRED_FIELDS "site"→"event";
  FormState.SiteSel→EventSel; blankState default-event; stateFromRecord /
  applySection / applyToState / validateState read/write `event`.
- `r3-intake/internal/server/admin.go` — IntakeRow.SiteName→EventName;
  participant-list resolution via `nameFor("events", …)`; ?site= filter →
  ?event= filter on intake.event (EventFilter).
- `r3-intake/internal/server/attendance.go` — intake.site reads → intake.event
  (resolveSite, loadMatrixRows NoLocation grouping, handleToggle, handleWalkin
  sets intake.event from event_id). events.site (event's location) untouched.
- `r3-intake/internal/server/person_attendance.go` — SiteName→EventName via
  nameFor("events", …); template header updated.
- `r3-intake/internal/mcp/mcp.go` — intake.site reads → intake.event (4 sites),
  new loadEventMap/resolveEvent helpers; MCP contract unchanged.
- Tests: 4 integration files seed intakes with r.Set("event", <eventID>);
  person_attendance_test.go fixture renamed; TestExportCSVSiteFilter pinned
  to be clock-independent.

## Verification
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (server 15.6s, migrations 0.18s)
- Grep: zero `intake.site` reads remain; every remaining GetString/Set("site")
  is on the `events` (event's location) or `sites` collection.
- View-model contract for siblings: EventSel, EventName, EventFilter,
  REQUIRED_FIELDS="event" all present.
- Sibling-owned template regions (site-fragment, R3 Site Location dropdown,
  participant-list Site column, admin tab ordering) untouched — owned by
  t_146484f2 / t_fb0d46ce / t_35775bf1.
