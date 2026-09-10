# Epic close-out — Remove the Claim Feature Completely (schema, data, UI, MCP)

Repo: jamespakele/ai-r3-v2 · App: r3-intake (Go + PocketBase + htmx)
Branch: epic/remove-the-claim-feature-completely-sche-0910064449

## What shipped
The claim workflow is fully removed across every layer of the system — no legacy claimed anything remains. The behavioral restriction was already gone on master; this epic removed everything else that still made records behave differently.

- Migration 017 (story 1,: 017_remove_claim.go) rewrites every intake.status=claimed to unassigned, drops claimed from the status select enum (now exactly [unassigned completed]), and removes the assigned_to relation field. Idempotent up/down; the data rewrite runs unconditionally. Registered in migrations.go alongside 015.
- App code/UI (story 2,: handlePublicIntake public-resume rule is now exactly one thing: created_by empty → public resume. Admin status filter accepts only unassigned/completed. Claimed dropdown option and the .status-claimed CSS rule removed.
)
- MCP (story  ́3,: removed claimed from both status enums, removed assigned_to filter param/field/branch, removed AssignedName from intakeSummary and its population, removed Claimed counter + switch branch from ByStatus, updated descriptions.
- Tests (4,: all claimed/assigned_to references swept from the server test suite; added017 down→seed→up round-trip migration test (TestRemoveClaimMigration;; TestNewRecordNotAutoClaimed→TestNewRecordUnassignedDefaultand TestPublicResumeLegacyClaimed→TestPublicResumeRule.

## Merge + conflict resolution
Merged child worktree branches wt/t_ab332d9f, wt/t_916ad5b6, wt/t_081e1391, and wt/t_7a3e5403 into the epic branch (final merge commit 47a805d; intermediate c861c5f and 24d3522. Test-file conflicts resolved toward Story4 (final authority for tests;; migrations.go retains BOTH the 015_attendance_remove_site and 017_remove_claim registrations. No conflict markers remain.



## Verification (full gate, independently re-run by the parent)
All from the app module (r3-intake/, with GOPATH=/home/pakele/go because $HOME is unset in the worker shell:
- gofmt -l on all changed .go files: empty (clean;
- go build ./...: exit  ́0
- go vet ./...: exit  ́0
- go test ./...: internal/server ok (16.7s,, migrations ok (0.27s, — every package passes
- TestRemoveClaimMigration: PASS (round-trips  017 down→seed→up, asserts zero claimed, no assigned_to field, status enum exactly [unassigned completed], idempotent re-up.launch))
- Zero-hit gate: grepfor claimed|assigned_to across internal/server, internal/mcp, internal/assets, deploy/ — ZERO hits (exit 1,, incl tests, excl pocketbase/migrations/ + docs/ per epic. All remaining references live onlyin pocketbase/migrations/ (001 historical schema + 017 removal code + 017 test,, explicitly excluded from the gate.)

## Known consequence (accepted — not relitigated)
Records that were anonymous-created and later claimed ()created_by empty, status formerly claimed — e.g. rows in the production test dataset) become unassigned with created_by empty after the rewrite, so they are publicly resumable via their unguessable record ID — exactly like every other anonymous-created intake. That is the single consistent rule James asked for.



## Story 5 (deploy — PENDING review approval)
Local real-data verification against the production test dataset could not run from this worktree:the real source data lives on the production server(at r3.aipono.org;; the local r3-intake/pocketbase/pb_data does not exist (created on first run.. Deployment via the vps-deploy-go skill(ships binary + migrations together;snapshot data pre-restart)wil, after review approval, snapshot the live DB, apply 017, restart the service,and verify live:no claimed status, 017 in _migrations, cross-user access, public-resume rule. Story 5 step 2 is the intended post-approval production run.



## Parent close-out
Final source commit on epic:47a805dffe092d627db097b07441e7f5a2908b59. RESULT.md + VERIFY_MERGE.md committed as the epic close-out record. Card handed to the review gate — reviewer approval is the only path to done (per the omp-plan-execute skill;; no self-complete. Production deploy (Story 5 step 2) executes after approval.
