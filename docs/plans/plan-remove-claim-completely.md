# Remove the Claim Feature Completely (schema, data, UI, MCP)

Status: APPROVED by James 2026-09-09 — full removal, no legacy preservation anywhere. One behavior for every record and every part of the system.
Repo: jamespakele/ai-r3-v2 (app: r3-intake, Go + PocketBase + htmx)

## Context (read first)

The R3 intake app had a "claim" workflow: a case manager could claim an intake record, which set `status='claimed'` + `assigned_to=<user>` and restricted access to the owner. Users rejected it ("Nix the claiming restrictiveness"). The behavior layer is ALREADY REMOVED and verified on master (access open to all signed-in users, Claim button/route gone, Assigned column gone, auto-claim gone, attendance roster/site pinning gone).

What remains is EVERYTHING that still makes some records or parts of the system behave differently: the `claimed` status value, the `assigned_to` field, legacy data holding those values, the Claimed filter option, the claimed badge CSS, the claimed branch in the status filter, the claimed clause in public-resume, and the claim vocabulary in the MCP API. James's directive: "No legacy claimed anything, remove it entirely — remove the field, do not preserve what's there."

## Keep (do not touch)

- `casemanagerName` intake field + `ensureCaseManager` + the form section — Kika confirmed it helps.
- `created_by` — record provenance, not claim ownership.
- Status values `unassigned` and `completed` only.
- Soft-delete conventions, event/site scoping, notes + audit trail.

## Story 1 — Migration `017_remove_claim.go` (schema + data)

File: `r3-intake/pocketbase/migrations/017_remove_claim.go` (Go migration; register alongside the other Go migrations — check `migrations.go`/`014_*` for the registration pattern). Follow the existing patterns: enum edit like `003_add_intake_role.go`, field removal like `015_attendance_remove_site.go`. Idempotent guards on every step (migration chain may run on DBs in different states).

Up — this order matters (PocketBase rejects saves holding an enum value that no longer exists):
1. Rewrite data: every `intake` record with `status='claimed'` → `status='unassigned'` (iterate `FindRecordsByFilter` `status='claimed'`, Set + Save).
2. Edit the `intake.status` select values → `["unassigned","completed"]` (drop `claimed`).
3. Remove the `assigned_to` field from the `intake` collection. Its data goes with the field — do not preserve or copy it.

Down: re-add `claimed` to the status enum values and re-add the `assigned_to` relation field (dropped data is not restorable — attribution lives in `created_by` and the notes audit trail).

Note: fresh databases run 001 (which still declares the field/value) then 017 (which removes them) — that ordering is correct and matches how 015 removed `attendance.site`.

## Story 2 — App code: finish the removal

- `internal/server/handlers.go` `handlePublicIntake` (~line 393): delete the `|| rec.GetString("status") == "claimed"` clause. The rule becomes exactly one thing: public resume only when `created_by` is empty. Update the comment (no legacy reasoning, no mention of claimed).
- `internal/server/admin.go` `handleList` status filter (~line 97): accept only `unassigned` or `completed`.
- `internal/assets/public/index.html` (~line 592): remove the Claimed `<option>` from the status dropdown.
- `internal/assets/public/app.css` (~line 201): delete the `.status-claimed` rule.
- Interim check (non-test runtime files only): `grep -rniE 'claimed|assigned_to' r3-intake/internal/server --include='*.go' --exclude='*_test.go' r3-intake/internal/mcp r3-intake/internal/assets r3-intake/cmd` → ZERO hits. Test files still reference these values until Story 4 rewrites them — the FULL zero-hit gate is the final check at the end of Story 4. References in `pocketbase/migrations/` (001 history + 017 removal code) and `docs/plans/` are history/documentation, excluded from all gates.

## Story 3 — MCP cleanup (`internal/mcp/mcp.go`)

- Remove `claimed` from both status enums (search tool ~line 52, stats tool ~line 85).
- Remove the `assigned_to` filter parameter (~line 54) and its filter-building branch (~line 295).
- Remove `AssignedName`/`AssignedTo` from `intakeSummary` and the code populating them (~lines 239, 248, 320-326, 408-415).
- Remove the `Claimed` counter from `ByStatus` (~lines 425, 467) and its switch case (~line 467).
- Update descriptions that reference assignment ("assigned_to resolution context" ~line 119).

## Story 4 — Tests (update, do not delete)

- `records_list_integration_test.go`: `intakeA` seeds `status='claimed'` → seed `unassigned`/`completed` instead; the "union composes with status filter" subtest queries `status=claimed` → rewrite against `completed` (or unassigned).
- `records_list_attendance_join_integration_test.go`: same — claimed fixtures and the `status=claimed` query → unassigned/completed.
- `attendance_roster_integration_test.go`: the `iAssignedCM` fixture sets `assigned_to` — the field will not exist after Story 1; remove the set (keep the Dana record itself), rename struct fields if needed.
- `claim_removal_integration_test.go`: `TestPublicResumeLegacyClaimed` asserts a legacy-claimed leg — impossible once the value is gone. Rewrite as `TestPublicResumeRule` pinning the simplified rule: anon-created unassigned → 200 (public resume), staff-created (`created_by` set) → 303 login.
- `intake_save_validation_test.go`: the seed comment still reads "so the created_by/assigned_to relations validate on save" — update it to reference `created_by` only.
- New migration test in `pocketbase/migrations/` (pattern: `014_users_deleted_test.go`, which calls the migration's up/down funcs directly): the harness boots PocketBase with the FULL chain applied, so seed legacy data via a down→seed→up round-trip: (1) after `RunAllMigrations()`, call `down017(app)` — this restores the `claimed` enum value and the `assigned_to` field; (2) insert legacy-shaped intake rows (PB saves now work — the enum accepts `claimed` again); (3) call `up017(app)` — the data-rewrite step inside up() must run unconditionally (not skipped by an "already applied" guard on the field check alone) so it rewrites the seeded rows; (4) assert: no record has `status='claimed'`, the intake collection has no `assigned_to` field, status select values are exactly `[unassigned completed]`; (5) a second `up017(app)` call must be a no-op (idempotent). This doubles as the down/up round-trip test.
- `claim_removal_integration_test.go`: `TestNewRecordNotAutoClaimed` reads `GetString("assigned_to")` — after the field is removed that read returns "", and the final grep gate flags the reference. Rewrite the assertion to check `status` + `created_by` only.
- **Final gate (end of this story):** a source-wide search for the removed status value and the removed assignment field name must return ZERO hits across the whole application source tree, test files included (the schema-migrations folder and the planning docs are excluded — they retain the historical schema definition and the removal code). Then the full project gate must be green: from the application directory, run the embedded-template build, the Go vet pass, and the complete Go test suite (every package, not a subset); all checks must pass.

## Story 5 — Real-data verification + deploy (parent, after children)

1. Review server on `:8190` with a COPY of the real test data (`r3-intake/pocketbase/pb_data`; md5 the source `data.db` before and after — it must not change). Verify on the copy:
   - migration applied: `sqlite3 data.db 'SELECT DISTINCT status FROM intake'` → only `unassigned`/`completed`; `PRAGMA table_info(intake)` has no `assigned_to`.
   - list page status dropdown shows Unassigned/Completed only.
   - cross-user access: login `cm@r3.local`, open a record created by another user, save a section → 204 + persisted.
   - public resume: anon-created unassigned record → 200; staff-created → 303.
2. Deploy via the `vps-deploy-go` skill (ships binary + migrations together; snapshots data pre-restart). Verify live: service active, health check on r3.aipono.org, `017` in `_migrations`, `SELECT DISTINCT status FROM intake` has no `claimed`, one cross-user access check.

## Known consequence (accepted — state it in the close-out, do not relitigate)

Records that were anonymous-created and later claimed (created_by empty, status was claimed — e.g. 2 rows in the test dataset) become `unassigned` with `created_by` empty after the rewrite, so they are publicly resumable via their unguessable record ID — exactly like every other anonymous-created intake. That is the single consistent rule James asked for.

## Conventions

All timestamps HST (`hst` var, `formatTime`). PocketBase v0.39 API (no `app.dao()`, no `core.NewBaseCollection`). Templates are embedded at build time — rebuild the binary before any server restart. Tests are updated, not deleted. Keep the complete Go test suite green (run every package, not a subset).

## Note to the decomposer (story-card generation)

When generating story cards from this plan, describe every command in plain English (for example: run the full Go test suite across every package; search the source tree for the removed field name). Never write shell syntax or glob tokens into generated card titles or bodies — especially the Go test-everything idiom spelled dot-slash plus three dots — because the kanban engine's graph builder treats such tokens as unresolvable repository references and rejects the decomposition. Card text must be prose and bare file names only.
