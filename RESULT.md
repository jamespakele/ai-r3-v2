# RESULT — Card t_36fab24a: Remove intake status from server handlers, UI templates, and MCP

## What shipped (Story 2 app+UI + Story 3 MCP)
Intake "status" removed completely from the application runtime. Implemented via omp
(omp-plan-execute, --plan-yolo --advisor). Exactly 6 files changed (8 insertions, 79 deletions),
no tests, no migrations touched (sibling cards own those).

- `internal/server/admin.go` — deleted `adminComplete` handler, its `POST /admin/intake/{id}/complete`
  dispatch case in `handleAdminSub`, `AdminView.StatusFilter` view field, the status whitelist branch
  in `handleList`, and `IntakeRow.Status` (+ its population literal).
- `internal/server/handlers.go` — removed `FormState.Status`, `blankState` `Status: "unassigned"`,
  and `stateFromRecord` Status population.
- `internal/server/server.go` — removed `rec.Set("status", "unassigned")` in `newIntakeRecord`.
- `internal/assets/public/index.html` (list-content) — removed status `<select>` dropdown, dropped
  `.StatusFilter` from the Clear-link condition, removed Status `<th>`, status badge `<td>`, and the
  Complete button form; empty-state colspan 6/5 → 5/4 and its condition dropped StatusFilter.
- `internal/assets/public/app.css` — deleted `.status-unassigned` and `.status-completed` rules only;
  kept shared `.status-badge` and all `.event-status-*` rules.
- `internal/mcp/mcp.go` — removed `status` enum from list_intakes/search_intakes schemas, Status fields
  from input structs + intakeSummary, both status filter-building branches, stats
  `ByStatus`/`statusCounts`/`CompletedThisMonth` + status switch; updated stats tool description to
  "Aggregate counts of intake records by site."

## Kept intact (verified)
`POST /intake/{id}/finish` (handleIntakeCmd), `events.status` (admin.go events), `attendance.status`
(attendance.go / person_attendance.go), shared `.status-badge`, `.event-status-*`.

## Verification
- `go build ./...` — PASS (rc=0, CGO_ENABLED=0)
- `go vet ./internal/...` — PASS (rc=0)
- Interim token gate (runtime source, excludes tests/migrations/docs): zero hits for `adminComplete`,
  `StatusFilter`, `status-unassigned`, `status-completed` (intake rule; only `.event-status-completed`
  kept), `ByStatus`, `by_status`, `CompletedThisMonth`, and no intake-record `Set/Get("status")`. All
  remaining status refs are events/attendance/.event-status keep-set.
- MCP tree has zero intake status traces.

## Note on full test suite
Not run to green — test-file updates (Story 4, e.g. `TestNewRecordUnassignedDefault`, status-filter
list tests) are owned by sibling card t_f3446de5 which also runs the full final gate. No NEW
compilation breakage from these struct changes (test binaries compile cleanly).

## Commit
`1df1f7b` on branch `wt/t_36fab24a` — "feat(intake): remove intake status feature from server, UI, and MCP"
