# RESULT — Epic 36: Remove the Intake Status Feature Completely

Branch: `epic/36-remove-intake-status-field-and-complete`
Merge HEAD: `7b8ae90` (three child worktrees merged; VERIFY_MERGE.md for full detail)
Epic issue: remove the intake "Status" concept entirely (schema + data + UI + MCP).

## What shipped
The intake `status` concept is gone entirely, verified across the whole merge:
- **Schema + data (Story 1):** `pocketbase/migrations/018_remove_intake_status.go` drops
  the `intake.status` select field. Idempotent `up` (no-op if already absent, no data
  rewrite — stored status values drop with the field); idempotent `down` re-adds it with
  `["unassigned","completed"]`. Registered in `migrations.go`. The 017 round-trip test was
  reworked (down018 -> 017 round-trip -> up018) so it composes in the full chain.
- **Server + UI (Story 2):** `adminComplete` handler, `POST /admin/intake/{id}/complete`
  route/dispatch, `StatusFilter` view field + `handleList` whitelist branch, `IntakeRow.Status`,
  FormState/blank/stateFromRecord `Status`, `newIntakeRecord` status set, and the List
  Status column / badge / Complete button / status `select`, plus `.status-unassigned` /
  `.status-completed` CSS — all removed. Empty-state colspan 6/5 -> 5/4.
- **MCP (Story 3):** `status` enum + `Status` struct fields + filter branches removed from
  list/search/summary tools; stats `ByStatus` / `statusCounts` / `CompletedThisMonth` removed
  (stats keep total + by-site); tool descriptions updated.
- **Tests (Story 4):** all stale intake-status assertions/fixtures removed while keeping
  event/attendance status coverage. `TestNewRecordUnassignedDefault` -> `TestNewRecordCreatedByOnly`
  (asserts `created_by` only); public-resume seeds no longer set intake status; status-filter
  legs dropped from records-list tests.

## Preserved (verified intact — do not conflate)
finish route (`POST /intake/{id}/finish`), `events.status` + `.event-status-*` CSS,
`attendance.status` (matrix / CSV / person-attendance), shared `.status-badge`,
`created_by` provenance + public-resume rule, `casemanagerName` + `ensureCaseManager`,
notes audit trail, soft-delete conventions, event/site scoping.

## Verification
| Gate | Result |
|------|--------|
| `go build ./...` (embedded-template build) | **PASS** rc=0 |
| `go vet ./...` | **PASS** rc=0 |
| `go test ./...` (every package) | **PASS** rc=0 — ok internal/server; ok pocketbase/migrations |
| Final token gate (app tree incl. tests) | **ZERO** intake-status traces (adminComplete/StatusFilter/status-unassigned/ByStatus/by_status/CompletedThisMonth/unassigned + intake complete route) |
| Merge hygiene | clean; single RESULT.md doc conflict resolved keep-ours; no markers |

## Known consequence (accepted — stated, not relitigated)
Nothing marks a record complete anymore; the List screen shows every record equally with
no status notion. The Complete button, route, field, and data are gone entirely. That is
the point of the removal and exactly what James asked for.

## Next steps (parent, gated on James's review approval — deferred here)
Real-data review-server verification on a copy of `pb_data` (migration applied; no status
column; list page clean; complete endpoint 404; cross-user + public-resume unchanged;
counts preserved) and the production deploy via `vps-deploy-go` run only after this epic
receives review approval. Pending approval — do not deploy yet.
