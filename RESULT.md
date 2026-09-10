# RESULT — migration 017_remove_claim.go

## What was built
Created `r3-intake/pocketbase/migrations/017_remove_claim.go` (Story 1 of the
"Remove the Claim Feature Completely" epic) and registered it in
`r3-intake/pocketbase/migrations/migrations.go`.

### Up migration (`upRemoveClaim`), idempotently guarded
1. Data rewrite: every `intake` record with `status='claimed'` -> `status='unassigned'`
   (FindRecordsByFilter `status='claimed'`, Set + Save), before dropping the enum value.
2. Drop `claimed` from `intake.status` select values -> `["unassigned", "completed"]`.
3. Remove the `assigned_to` field from the `intake` collection.

Ordering follows the epic spec: the claimed-value save happens before the enum edit
because PocketBase rejects saves holding an enum value that no longer exists.

### Down migration (`downRemoveClaim`), idempotently guarded
1. Re-adds `claimed` to `intake.status` values.
2. Re-adds `assigned_to` as an optional single-select relation to `users`.

## Files changed
- `r3-intake/pocketbase/migrations/017_remove_claim.go` (new)
- `r3-intake/pocketbase/migrations/migrations.go` (one registration line, filename `017_remove_claim.go`)

## Verification
- `gofmt -l pocketbase/migrations/017_remove_claim.go` -> empty (gofmt-clean)
- `go build ./...` -> exit 0
- `go vet ./...` -> exit 0 (run on ./pocketbase/... and full tree)
- `go test ./pocketbase/migrations/...` -> ok (0.097s)

## Out-of-scope (owned by sibling cards, as decomposed)
The `internal/server` package has one expected failure after this migration:
`TestPublicResumeLegacyClaimed` (claim_removal_integration_test.go) tries to save
`status='claimed'` and now fails with "Invalid value claimed" — the enum value was
correctly removed. Rewriting that fixture (and the other claimed/assigned_to test
references) is the job of sibling card `t_7a3e5403` (Story 4 — Update test suite),
which depends on this card. App-code/MCP removal are Story 2/3 (`t_916ad5b6`,
`t_081e1391`). The epic's full-suite green gate runs after all children merge.
