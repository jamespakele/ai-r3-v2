# RESULT — t_f3446de5: Review + Story 4 test sweep + full final gate

## Scope
Peer review of the migration (018), server code, UI templates, and MCP cleanup for the
complete removal of the intake status feature, plus the Story 4 test-file updates and the
end-of-epic full gate.

## Peer review findings
- Migration `018_remove_intake_status.go` (Story 1, from sibling): correct and idempotent.
  up drops intake.status (no data rewrite); down re-adds it with post-017 values
  [unassigned completed]. Correctly registered in migrations.go. The 017 round-trip test
  was reworked to down018 -> 017 round-trip -> up018, composing cleanly in the chain.
- Server/UI/MCP removal (Story 2+3, from parent t_36fab24a): complete. adminComplete
  handler + route/dispatch gone, StatusFilter view field + handleList branch gone,
  FormState/blank/stateFromRecord Status gone, newIntakeRecord status set gone, List
  status column/badge/Complete button/dropdown gone (colspan 6/5->5/4), .status-unassigned/
  .status-completed CSS gone, and all MCP status schemas/filters/summary/ByStatus/
  CompletedThisMonth gone. No issues found.
- Preserved (verified intact): finish route, events.status + .event-status-* CSS,
  attendance.status (matrix/export/person-attendance), shared .status-badge class,
  created_by provenance + public-resume rule.

## Story 4 changes (this card)
Test files updated to remove every stale intake-status reference (event/attendance status
kept):
- claim_removal_integration_test.go: TestNewRecordUnassignedDefault ->
  TestNewRecordCreatedByOnly (asserts created_by only); TestPublicResumeRule mk() helper
  no longer sets intake.status (rule is created_by-based).
- records_list_integration_test.go: dropped intake status seed sets; removed the
  "union composes with status filter" subtest.
- records_list_attendance_join_integration_test.go: dropped intake status seed sets and
  comments; removed TestListEventFilterComposesWithStatusAndSearch (behaviorally
  identical to the surviving TestListEventFilterComposesWithSearch once the stale
  status=completed filter is gone).
- intake_edit_event_default_integration_test.go: dropped intake status seed sets.
- internal/mcp/mcp.go: gofmt alignment fix (leftover from status property removal).
- Also carried the uncommitted migration-018 files from sibling t_f902df13 into this
  branch so the epic merge does not lose them (sibling had left them uncommitted).

## Verification (independently re-run, not just omp self-report)
- go build ./...  : rc=0
- go vet ./...    : rc=0
- gofmt -l .      : clean (empty)
- go test ./...   :
    ok  r3-intake/internal/server 17s
    ok  r3-intake/pocketbase/migrations
- Final mechanical token gate (whole tree incl tests, excl migrations dir + docs):
    adminComplete / StatusFilter / status-unassigned / ByStatus / by_status /
    CompletedThisMonth  -> ZERO hits
    status-completed    -> only .event-status-completed (event feature, must stay)
    no intake complete-route references; no intake Set/Get status anywhere

## Commits on wt/t_f3446de5
- 051ac89 feat(intake): Story 4 test sweep — remove intake-status assertions, add migration 018
- 4992927 feat(intake): finish intake-status removal in attendance-join and event-default tests
- 6c80f14 chore(mcp): gofmt alignment of search_intakes schema keys

## Note for the epic parent merge
Sibling t_f902df13 (Story 1, migration 018) completed its card but LEFT ITS WORK
UNCOMMITTED — its branch wt/t_f902df13 is identical to master. This card carried the 018
migration files (018_remove_intake_status.go, migrations.go registration, 017 test rework)
into wt/t_f3446de5 and committed them. Merge wt/t_f3446de5 to capture the schema/data leg.
