# Remove the Intake Status Feature Completely (schema, data, UI, MCP)

Status: DIRECTIVE from James 2026-09-10 — remove the intake "Status" concept entirely. The Status column and Complete button on the List screen are unclear and serve no purpose. Remove ALL of it: the `intake.status` schema field, all stored status data, the status filter, the status badges, the Complete action and route, and every status trace in the MCP API. No legacy preservation anywhere. One behavior for every record.
Repo: jamespakele/ai-r3-v2 (app: r3-intake, Go + PocketBase + htmx)

## Context (read first)

The records List screen renders a Status column (Unassigned/Completed badges) and a per-row Complete button that posts to `/admin/intake/{id}/complete` and sets `status='completed'`. The list also has a status filter dropdown (Unassigned/Completed). The claim workflow was already removed completely by migration 017 — `intake.status` now holds only `unassigned`/`completed`. This epic removes the field itself and everything that reads or writes it. After this epic: a record is just a record. `created_by` provenance and the notes audit trail remain the only state notions. Nothing else in the app changes.

Line references below were audited on master at the merge commit 5e9d7fe and are approximate — re-grep before editing.

## Keep (do not touch)

- The finish form and route (`POST /intake/{id}/finish`, `handleIntakeCmd`, template block `finish-form`) — full-form final validation, completely unrelated to the List screen's Complete action. Do not conflate them.
- `events.status` and event badges (Events tab rows, event manage screen, event report gate; `.event-status-*` CSS rules).
- `attendance.status` (matrix dot cycle, CSV export) and person-attendance status handling.
- The shared `.status-badge` CSS class — event badges reuse it. Only the intake-specific `.status-unassigned` and `.status-completed` rules are removed.
- `casemanagerName` intake field + `ensureCaseManager` + the form section — Kika confirmed it helps.
- `created_by` provenance and the public-resume rule (public resume only when `created_by` is empty) — unchanged, no new behavior.
- Notes with audit trail, soft-delete conventions, event/site scoping.
- No new restrictions, validations, or security framing around this removal. Remove only what is named here.

## Story 1 — Migration `018_remove_intake_status.go` (schema + data)

File: `r3-intake/pocketbase/migrations/018_remove_intake_status.go` (Go migration; register alongside the others in `migrations.go` — same pattern as 015/016/017). Idempotent guard: if the intake collection has no `status` field, up is a no-op.

Up: remove the `status` select field from the `intake` collection. Stored status values go with the field — there is no rewrite step and nothing is preserved or copied. All other intake data is untouched.

Down: re-add `status` as a select field with values exactly `["unassigned","completed"]` (the post-017 shape). Dropped data is not restorable.

Note: fresh databases run 001 (declares the field with unassigned/claimed/completed values), then 017 (drops the claimed value), then 018 (removes the field) — ordering is correct and matches how 015 removed `attendance.site`.

Chain interaction that MUST be handled: migration 017's up/down edit the status select values, and its round-trip test (`017_remove_claim_test.go`) calls down then seeds legacy rows then up. Once 018 removes the field, the test harness's full chain includes 018, so the 017 round-trip can no longer run as-is. Rework the 017 test to first call down018 (restoring the status field with the post-017 values), then its existing down017 → seed → up017 sequence, then up018 to restore the final state — or an equivalent approach. Add absence guards to 017's up/down only if needed. Acceptance: both migration tests pass with 018 in the chain.

## Story 2 — App code + UI: finish the removal

- `internal/server/admin.go`: delete the `adminComplete` handler (~line 333-342) and its dispatch case in `handleAdminSub` (~line 224-225). The route disappears (POST to it falls through to the 404 path). Delete the `StatusFilter` view field (~line 46) and the status whitelist branch in `handleList` (~line 97-99).
- `internal/server/handlers.go`: delete `Status` from the FormState struct (~line 109), the `"unassigned"` default in blankState (~line 240), and the population from the record in stateFromRecord (~line 299). The finish flow and validateRecord have no status references — leave them alone.
- `internal/server/server.go`: delete the `rec.Set("status", "unassigned")` line in `newIntakeRecord` (~line 306).
- `internal/assets/public/index.html` list-content block: remove the entire status `<select>` dropdown (~line 589-593), drop `.StatusFilter` from the Clear-link condition (~line 598), remove the Status `<th>` (~line 604), the status badge `<td>` (~line 611), and the Complete button form (~line 616). Fix the empty-state colspan from 6/5 to 5/4 and drop `.StatusFilter` from its condition (~line 621).
- `internal/assets/public/app.css`: delete the `.status-unassigned` and `.status-completed` rules (~line 200-201). Keep `.status-badge` (shared with event badges) and all `.event-status-*` rules.
- Interim gate (runtime source only, tests still pending): search the server, mcp, assets, and cmd trees for the tokens adminComplete, StatusFilter, status-unassigned, status-completed, ByStatus, by_status, CompletedThisMonth, and any set/get of the status field on intake records — zero hits. Migration history and docs are excluded from all gates.

## Story 3 — MCP cleanup (`internal/mcp/mcp.go`)

- Remove the `"status"` enum property from the list_intakes (~line 52) and search_intakes (~line 84) tool schemas, the `Status` fields from their input structs (~line 236/244/341), and the status filter-building branches (~line 275-276, ~line 369-370).
- Remove `Status` from the intake summary struct and both population sites (~line 308, ~line 389).
- Stats tool: remove `ByStatus`/`statusCounts`/`CompletedThisMonth` (~line 399-446) and the status switch; the stats output keeps total and by-site only. Update the tool description (~line 94) to match.
- There are no MCP tests; this story is code-only.

## Story 4 — Tests (update, do not delete) + final gate

- `claim_removal_integration_test.go`: `TestNewRecordNotAutoClaimed` asserts the new record's status is unassigned — after the field is gone, rewrite it to assert `created_by` only (rename as needed, e.g. new-record provenance). The public-resume test seeds records with a status value via a helper — remove the status sets; the rule stays: anonymous-created (created_by empty) → 200, staff-created → 303.
- `records_list_integration_test.go` and `records_list_attendance_join_integration_test.go`: the status-filter subtests (queries like status=unassigned/completed) — remove the status leg or rewrite the composition subtest against the event/search filters only. Fixtures that seed intake status values: drop those sets.
- `intake_edit_event_default_integration_test.go`: two intake fixtures set status unassigned (~line 77, 84) — remove the sets.
- Verify no test posts to the complete route or references StatusFilter.
- **Final gate (end of this story):** search the whole application source tree INCLUDING test files for: the complete-route suffix on intake, adminComplete, StatusFilter, status-unassigned, status-completed, ByStatus, by_status, CompletedThisMonth, and any set/get of the status field on intake records. ZERO hits. Exclusions: the schema-migrations folder (001 history + 017/018 code and tests) and planning docs keep historical references; event-status and attendance-status code paths and the shared `.status-badge` class are unrelated features and must remain; HTTP status-code identifiers (http.Status*) are unrelated. Then the full project gate: from the application directory run the embedded-template build, the Go vet pass, and the complete Go test suite (every package, not a subset); all must pass.

## Story 5 — Real-data verification + deploy (parent, after children)

1. Review server on the alternate port (:8190/:8191) with a COPY of the real test data (`r3-intake/pocketbase/pb_data` copied to a persistent home-directory location; md5 the source `data.db` before and after — it must not change). Verify on the copy:
   - migration applied: `018_remove_intake_status.go` present in the migrations table; `PRAGMA table_info(intake)` has no `status` column.
   - list page: no Status column header, no badge cells, no status dropdown, no Complete button; colspan correct.
   - the complete endpoint returns 404 for a signed-in user with a valid CSRF token.
   - cross-user access unchanged: case manager opens a record created by another user and saves a section.
   - public resume unchanged: anonymous-created record → 200; staff-created → 303.
   - record counts preserved (intake/attendance/events rows unchanged except the dropped column).
2. Deploy via the `vps-deploy-go` skill (ships binary + migrations together; snapshots data pre-restart). Verify live: service active, health check on r3.aipono.org, `018` in the migrations table, no `status` column on intake, one cross-user access check. Execute the production deploy only after James's review approval of the epic.

## Known consequence (accepted — state it in the close-out, do not relitigate)

Nothing marks a record as complete anymore; the List screen shows every record equally with no status notion. The Complete button, the route, the field, and the data are gone entirely. That is the point of the removal and is exactly what James asked for.

## Note to the decomposer (story-card generation)

When generating story cards from this plan, describe every command in plain English (for example: run the full Go test suite across every package; search the source tree for the removed field name). Never write shell syntax or glob tokens into generated card titles or bodies — especially the Go test-everything idiom spelled dot-slash plus three dots — because the kanban engine's graph builder treats such tokens as unresolvable repository references and rejects the decomposition. Card text must be prose and bare file names only.