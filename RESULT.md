# t_7d99815c — Add deleted field to events collection via migration

**Status:** COMPLETE

## What was built
- New migration `r3-intake/pocketbase/migrations/014_events_deleted.go` adding a
  soft-delete `deleted` bool field (`core.BoolField{Name: "deleted", Required: false}`)
  to the `events` collection, following the exact `008_event_enrollment_deleted.go`
  pattern (idempotent up/down guards on `GetByName("deleted")`).
- Registered in `r3-intake/pocketbase/migrations/migrations.go`:
  `migrations.Register(upEventsDeleted, downEventsDeleted, "014_events_deleted.go")`.

## Scope
Schema migration only. No handlers, templates, or other fields touched. The
sibling handler card (t_dc1890be) consumes the `deleted` field.

## Verification
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (server suite 7.866s green)
- Idempotency guards present in both up and down funcs.

## Artifacts
- `docs/plans/omp-plan-events-deleted-field.md` — MOA working plan
