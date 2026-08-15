# RESULT — Epic 21: Replace intake site/location with event reference

This epic merged four child story worktrees into `epic/21-replace-intake-sitelocation-with-event-a`.

## Child stories merged

### 1. wt/t_27c37c36 — Migrate intake site field to event reference

Renamed `intake.site` (→ `sites`) to `intake.event` (→ `events`) across schema and Go code.

- `pocketbase/migrations/016_intake_site_to_event.go`: up removes `intake.site`, adds `intake.event`, best-effort backfills each old site to the first active non-deleted event at that site; down reverses and backfills `site` from the event's site. Idempotency guards both ways.
- `pocketbase/migrations/migrations.go`: registered migration 016.
- `r3-intake/internal/server/handlers.go`: `REQUIRED_FIELDS` "site"→"event"; `FormState.SiteSel`→`EventSel`; `blankState` defaults to first active event; `stateFromRecord`, `applySection`, `applyToState`, `validateState` read/write `event`.
- `r3-intake/internal/server/admin.go`: `IntakeRow.SiteName`→`EventName`; participant-list resolution via `nameFor("events", …)`; `?site=` filter → `?event=` filter on `intake.event` (`EventFilter`).
- `r3-intake/internal/server/attendance.go`: `intake.site` reads → `intake.event` in `resolveSite`, `loadMatrixRows`, `handleToggle`, `handleWalkin`.
- `r3-intake/internal/server/person_attendance.go`: `SiteName`→`EventName` via `nameFor("events", …)`.
- `r3-intake/internal/mcp/mcp.go`: `intake.site` reads → `intake.event`; new `loadEventMap`/`resolveEvent` helpers; MCP contract field name stays `site` for backward compatibility.
- Tests: 4 integration files seed intakes with `r.Set("event", <eventID>)`; `person_attendance_test.go` fixture renamed.

### 2. wt/t_fb0d46ce — Change participant list Site column to Event column

- `r3-intake/internal/assets/public/index.html` admin participant-list table:
  - Header `<th>Site</th>` → `<th>Event</th>`.
  - Row cell `{{.SiteName}}` → `{{.EventName}}`.

### 3. wt/t_146484f2 — Replace Site/Location dropdown with Event dropdown on intake form

- `r3-intake/internal/assets/public/index.html` section 01:
  - Label "R3 Site Location" → "R3 Event".
  - `{{template "site-fragment" .}}` → `{{template "event-fragment" .}}`.
  - Error key `Errors.site` → `Errors.event`.
  - New template block `event-fragment` renders `<select name="event">` from `.Events` with `selected` on `.EventSel`.
- `r3-intake/internal/server/handlers.go`:
  - Added `Events []Event` to `FormState`.
  - `blankState` loads events via `must(s.loadEvents())`, assigns `st.Events`, defaults `EventSel` to the first active event.
  - `stateFromRecord` populates `Events: must(s.loadEvents())`.

### 4. wt/t_35775bf1 — Reorder admin tabs to put Events first

- `r3-intake/internal/assets/public/index.html` admin tabs:
  - Order for admins: Events (default active) → Users → Sites.
  - Non-admins still see only the Sites tab.
  - Panel order matches tab order.

## Verification

- `make vendor` — fetched htmx + alpine.
- `go build ./...` — PASS.
- `go vet ./...` — PASS.
- `go test ./...` — PASS except one pre-existing unrelated failure:
  - `TestExportCSVSiteFilter/site1_only` fails on current master (`row count = 3, want 4`) and is unrelated to this epic.

## Artifacts added

- `docs/plans/omp-plan-intake-site-to-event.md`
- `docs/plans/omp-plan-participant-list-site-to-event.md`
- `docs/plans/omp-plan-event-dropdown-on-intake.md`
- `docs/plans/omp-plan-reorder-admin-tabs-events-first.md`
- `WORKING_PLAN_reorder-admin-tabs-events-first.md`
- `.hermes/plans/20260815-participant-list-site-to-event.md`
- `pocketbase/migrations/016_intake_site_to_event.go`
