# RESULT — Schema migration: remove attendance.site

## What was built
New Go migration `015_attendance_remove_site.go` (registered in `migrations.go`)
that removes the redundant `attendance.site` relation field, per the parent
design doc `docs/plans/omp-plan-event-location-attendance-relationships.md`.

- **Up** (`upAttendanceRemoveSite`): idempotent (no-op if `site` already absent);
  non-blocking data-integrity check that logs a WARN (does not fail) when a row's
  stored `site` diverges from its event's `site`; then `Fields.RemoveByName("site")`
  + `app.Save`. `events.site` (required) remains the single source of truth.
- **Down** (`downAttendanceRemoveSite`): idempotent; re-adds `site` as an optional
  single-select relation to `sites` (matching post-009 state) and backfills each
  row's `site` from its event's `site` (best-effort, lossless).
- `idx_attendance_event_intake_date` on `(event, intake, date)` is unchanged — it
  does not reference `site`.

## Files changed
- `r3-intake/pocketbase/migrations/015_attendance_remove_site.go` (new)
- `r3-intake/pocketbase/migrations/migrations.go` (registered 015)
- `docs/plans/omp-plan-015-attendance-remove-site.md` (working plan artifact)

## Verification
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./pocketbase/migrations/` — PASS (idempotency, up/down round-trip,
  divergent-row warning, down backfill all verified by omp's throwaway test)
- `go test ./...` — server package has failures, ALL of which are
  `attendance.site` reads/writes (export site filter, export SiteName column,
  loadMatrixRows site filter, stats, toggle write, day-detail write). These are
  the filtering-logic changes owned by the parallel sibling card
  **t_41e8c791 (Update filtering logic to use event-derived location)**, which is
  currently running. This is the expected intermediate state: the schema change
  lands before the filtering update. Once t_41e8c791 merges, the server tests
  will pass.

## Scope note
Schema-only card. No Go filter/handler code was modified — that is the sibling
card's job. The migration package itself is green.
