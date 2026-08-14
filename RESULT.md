# RESULT — Fix handleEnrollSearch to allow cross-site participant search and enrollment

## What was built
Removed the site restriction from the "Add participant" search on the Event Manage page.
`handleEnrollSearch` (r3-intake/internal/server/admin.go) previously appended
`&& site='<eventSite>'` to its PocketBase filter, so participants at a different site
(or an event with no site) could not be found or enrolled. The filter is now name-only
(`name ~ "<q>"`), returning matching intake records from ALL sites.

## Why this is sufficient
- The `enroll-search-results` fragment already renders each hit's `SiteName`
  (`<span class="enroll-result-meta">{{.SiteName}}</span>`), so cross-site results
  display their own site name with no template change.
- `handleEventEnroll` does not restrict by site — it only verifies the event and intake
  exist and creates the enrollment. So removing the search filter enables cross-site
  enrollment with no other code change.
- The per-result `Already` idempotency check (`event && intake && deleted=false`) is
  unchanged, so already-enrolled participants are still marked and cannot be re-added.

## Files changed
- `r3-intake/internal/server/admin.go` — removed `eventSite` var and the site-filter block;
  switched `eventRec` to blank identifier since it became unused.

## Verification
- `go build ./...` — exit 0
- `go vet ./...` — exit 0
- `go test ./...` — all pass (internal/server 6.897s)
- `TestEnrollSearchResultsRender` — PASS

## Artifacts
- Working plan: `docs/plans/omp-plan-cross-site-enroll-search.md`
