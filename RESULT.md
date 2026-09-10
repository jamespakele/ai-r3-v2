# Story 4 — Test sweep: complete claim-reference removal (t_7a3e5403)

## What shipped
Updated the R3 intake test suite to align with the claim-free schema/enums
(migration 017 + app-code/MCP siblings), and added a round-trip migration
test for 017. No non-test source logic was authored by this card.

### Test files updated (r3-intake/internal/server)
- attendance_roster_integration_test.go: removed `assigned_to` seed set on the
  Dana (iAssignedCM) intake fixture (field no longer exists).
- person_attendance_integration_test.go: removed `assigned_to` sets on i1/i2
  intake fixtures.
- intake_save_validation_test.go: seed comment now references created_by only.
- records_list_attendance_join_integration_test.go: the date-range test's
  intakeA seed status claimed -> unassigned.
- claim_removal_integration_test.go:
  - TestNewRecordNotAutoClaimed -> TestNewRecordUnassignedDefault (asserts
    status == unassigned + created_by; assigned_to read removed).
  - TestPublicResumeLegacyClaimed -> TestPublicResumeRule (anon-created
    unassigned -> 200 public resume; staff-created -> 303 login; legacy
    claimed leg removed since the enum value no longer exists).

### New migration test (r3-intake/pocketbase/migrations/017_remove_claim_test.go)
TestRemoveClaimMigration boots an in-process PocketBase with the FULL chain
applied (so 017 has run), then exercises a down -> seed -> up round-trip:
1. downRemoveClaim restores the claimed enum + assigned_to relation field.
2. Seeded legacy rows: site -> event -> real user -> 3 intakes with
   status=claimed (two carry assigned_to, one empty).
3. upRemoveClaim runs: rewrites every claimed row to unassigned, drops the
   enum value, removes assigned_to.
4. Asserts: zero rows with status=claimed; no intake.assigned_to field; status
   select values exactly [unassigned completed].
5. Idempotent re-up: a second upRemoveClaim is a no-op (error-free, state
   unchanged). Down round-trip restores claimed + assigned_to.

### Regression reconcile (migrations.go)
The migration sibling t_ab332d9f had ACCIDENTALLY dropped the
`migrations.Register(upAttendanceRemoveSite, downAttendanceRemoveSite,
"015_attendance_remove_site.go")` line while adding 017 (file kept, register
line deleted). That unmounted migration 015, so `attendance.site` was never
removed and TestAttendanceSchemaNoSiteField failed. Restored the single 015
registration line (epic convention: resolve overlaps toward Story 4 at parent
merge). It is population of the sibling's own migration list, not new logic.

## Verification (all from r3-intake, HOME=/home/pakele)
- gofmt -l on all changed files: EMPTY (clean)
- go vet ./...: exit 0
- go build ./...: exit 0
- go test ./...: r3-intake/internal/server ok (16.65s), pocketbase/migrations ok
- TestRemoveClaimMigration: PASS (verbose, -count=1)
- Zero-hit gate (app source, test files INCLUDED, migrations folder + docs
  EXCLUDED per epic): `grep -rniE 'claimed|assigned_to' --include='*.go'
  --exclude-dir=migrations .` -> ZERO hits (exit 1); assets dir -> ZERO hits.
- All remaining claimed/assigned_to references are confined to
  pocketbase/migrations/ (001 history + 017 removal code + 017 test) — the
  epic explicitly excludes that folder from the gate.

## Commit
b4ddb20 story 4: test sweep — drop claimed/assigned_to refs, add 017 round-trip
migration test, restore 015 reg
