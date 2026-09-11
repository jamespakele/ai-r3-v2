# RESULT — t_f902df13: Migration 018 (remove intake status) + 017 test rework

## What was built
- **`r3-intake/pocketbase/migrations/018_remove_intake_status.go`** (new)
  - `upRemoveIntakeStatus`: drops the `status` select field from `intake`
    entirely. Idempotent (no-op if field absent). No data rewrite — values go
    with the field.
  - `downRemoveIntakeStatus`: re-adds `status` as optional single-select with
    values exactly `["unassigned","completed"]` (post-017 shape). Idempotent.
- **`r3-intake/pocketbase/migrations/migrations.go`**: registered 018
  (`migrations.Register(upRemoveIntakeStatus, downRemoveIntakeStatus, "018_remove_intake_status.go")`).
- **`r3-intake/pocketbase/migrations/017_remove_claim_test.go`**: handles 018 in
  the chain — calls `downRemoveIntakeStatus` after RunAllMigrations to restore
  the field (down018 before down017), runs the existing 017 round-trip
  unchanged, then `upRemoveIntakeStatus` at the end restores the final
  post-018 state and asserts `status` field is nil.

Implemented via omp (omp-plan-execute, `--plan-yolo --advisor`).

## Verification
- `go build ./...` — BUILD_OK
- `go vet ./pocketbase/migrations/` — VET_OK
- `go test ./pocketbase/migrations/ -v`:
  `TestUsersDeletedMigration` PASS, `TestRemoveClaimMigration` PASS, `ok`.
- Full `go test ./...`: pocketbase/migrations PASS. Three `internal/server`
  failures remain (TestNewRecordUnassignedDefault, status-filter list tests) —
  these are Story 4 scope (status-based assertions that must change once the
  field is gone), owned by sibling card t_f3446de5, NOT this Story 1 card.

## Changed files
- r3-intake/pocketbase/migrations/018_remove_intake_status.go (new)
- r3-intake/pocketbase/migrations/migrations.go
- r3-intake/pocketbase/migrations/017_remove_claim_test.go
