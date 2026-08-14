# Epic 15: Event Manage page — participant roster section

**Status:** IN PROGRESS — child story branches being merged.

## Stories implemented

- **t_b478182c** — Cross-site participant search for Event Manage enrollment.
  Removes the site restriction from `handleEnrollSearch` in
  `r3-intake/internal/server/admin.go`. Search now returns matching intake
  records from all sites, and the existing `enroll-search-results` template
  already shows each hit's `SiteName`.

- **t_d61f8ff7** — Fix `loadEnrolledRoster` NULL `deleted` filter.
  Changes the roster query from `event='<id>' && deleted=false` to
  `event='<id>' && (deleted = false || deleted = null)` so legacy enrollments
  with a NULL `deleted` value are rendered instead of hidden.

## Files changed

- `r3-intake/internal/server/admin.go` — `handleEnrollSearch` and
  `loadEnrolledRoster` filters.
- `docs/plans/omp-plan-cross-site-enroll-search.md` — working plan.
- `docs/plans/omp-plan-fix-loadenrolledroster.md` — working plan.
- `WORKING_PLAN_fix-loadEnrolledRoster.md` — working plan.
- `.hermes/plans/2026-08-13_handleEnrollSearch-cross-site.md` — working plan.

## Merge resolution notes

- `RESULT.md` conflicts across child stories are resolved by synthesizing this
  Epic 15 summary.
- The `admin.go` changes are independent (different functions), so they merge
  cleanly.

## Verification

- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass
