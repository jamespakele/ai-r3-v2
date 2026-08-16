# RESULT — Add tests for Records filter with attendance

## What was built
Added complementary integration tests for the Records screen event filter's
attendance-join behavior (an intake matches a selected event if its home
`event` field equals it OR it has an `attendance` record for that event).

The sibling card (t_6e8ed9af) implemented the filter join in `admin.go` and
wrote `records_list_integration_test.go` in its own worktree. This card:
1. Copied the sibling's `admin.go` `handleList` event-filter union block into
   this worktree (byte-identical to the sibling's diff, so the epic merge is clean).
2. Created `r3-intake/internal/server/records_list_attendance_join_integration_test.go`
   with uniquely-named helpers (`attJoinFixtures`, `seedAttJoinFixtures`,
   `doAttJoinList`, `attJoinRowNames`, `attJoinCount`) to avoid symbol collisions
   when the epic merges both test files into package `server`.

## Tests added (6, all pass)
- `TestListEventFilterDedupsHomeAndAttendance` — intake matching via BOTH home-event
  and attendance branches appears exactly once.
- `TestListEventFilterSurfacesMultipleAttendees` — union scales to N attendees.
- `TestListEventFilterComposesWithSearch` — union && free-text ?q= search.
- `TestListEventFilterComposesWithStatusAndSearch` — three-way && composition.
- `TestListEventFilterCrossEventDistinct` — intake surfaces via attendance even when
  its home event is a different event; non-attendee does not.
- `TestListNoEventFilterReturnsAll` — regression guard: unfiltered list still returns all.

## Verification
- `go build ./...` — pass
- `go vet ./internal/server/` — clean
- New tests: 6/6 pass
- Full `go test ./internal/server/` — pass (17.8s, no regressions)
- admin.go diff matches sibling exactly (git diff vs /tmp/admin_join.diff → identical)
- No top-level symbol collisions with sibling's records_list_integration_test.go

## Files
- `r3-intake/internal/server/admin.go` (modified — copied sibling's join block)
- `r3-intake/internal/server/records_list_attendance_join_integration_test.go` (new)
- `docs/plans/omp-plan-records-filter-attendance-tests.md` (plan artifact)
