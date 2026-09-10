# RESULT — remove claim references from server handlers, admin filter, and UI assets

## What was built (Story 2 of "Remove the Claim Feature Completely" epic)

Removed every remaining claim reference from the app-code and UI layers so
behavior is uniform: no `claimed` status value, no filtered/rendered Claim
state, public resume purely on `created_by` empty.

### Changes (committed 4a6447a on wt/t_916ad5b6)
1. internal/server/handlers.go (handlePublicIntake): deleted the
   `|| rec.GetString("status") == "claimed"` clause; public resume is now
   allowed whenever `created_by` is empty. Comment rewritten with no legacy
   claim rationale.
2. internal/server/admin.go (handleList status filter): accepts only
   `unassigned` or `completed` (removed `claimed`).
3. internal/assets/public/index.html: removed the Claimed `<option>` from the
   admin status dropdown.
4. internal/assets/public/app.css: deleted the .status-claimed rule.

Test reconciliation (kept the suite green for THIS change):
- records_list_integration_test.go + records_list_attendance_join_integration_test.go:
  fixture status claimed -> completed; composition queries ?status=claimed -> ?status=completed
  (filter semantics preserved).
- claim_removal_integration_test.go: dropped the "Claimed option must remain" assertion
  (option is gone); TestPublicResumeLegacyClaimed legacy-claimed leg now expects 200
  (publicly resumable when created_by empty).

## Verification (run independently in this worktree)
- grep "claimed" in the 4 changed production files -> ZERO hits
- go build ./... -> exit 0
- go vet ./... -> exit 0 (0 diagnostics)
- go test ./... -> ok (internal/server 114 PASS / 0 FAIL, migrations ok)
- Gated tests PASS: TestListHasNoClaimUI, TestPublicResumeLegacyClaimed,
  TestListEventFilterComposesWithStatusAndSearch, TestListEventFilterJoinsAttendance

Note: `make verify` cannot complete because its `build` target references
./cmd/r3-intake but no cmd/ dir exists on this (or the default) branch --
pre-existing, unrelated to this change. Gate substance run directly.

## Out-of-scope (owned by sibling cards, as decomposed)
- Migration 017_remove_claim.go: sibling t_ab332d9f (Story 1).
- MCP claim/assigned_to vocabulary: sibling t_081e1391 (Story 3).
  mcp.go is intentionally untouched here.
- Full test-suite claim/assigned_to sweep + migration test + rename of
  TestPublicResumeLegacyClaimed -> TestPublicResumeRule: sibling t_7a3e5403 (Story 4).
- OVERLAP to reconcile at parent merge: t_7a3e5403 also targets
  records_list_integration_test.go / records_list_attendance_join_integration_test.go /
  claim_removal_integration_test.go (differs: my branch still names
  TestPublicResumeLegacyClaimed and seeds a claimed row in one no status-filtered
  fixture; Story 4 intends to rewrite those). Recommend the parent merge resolve
  toward Story 4 conclusions once both branches land.
 
