# RESULT — Attendance dot fix + "No Location" UI grouping

## What was built

Fixed the attendance matrix so participants WITHOUT a Location (intake.site empty)
can have their attendance dots toggled, and added a "No Location" group at the top
of the attendance list with clear visual separation and an indicator.

### Root cause (fixed)
`handleToggle` in `attendance.go` rejected the HTMX POST with HTTP 400
"missing required fields" when `site_id` was empty. No-Location participants send
an empty `site_id`, so their dots rendered clickable but the server rejected the
toggle. The per-person day-detail path already worked (it does not validate site).

### Changes
- `r3-intake/internal/server/attendance.go`
  - `handleToggle`: required check now only `intakeID`/`date`. Effective site falls
    back to the intake's own site when `site_id` is empty (may still be empty),
    matching the day-detail path. Missing intake proceeds with empty site rather
    than erroring.
  - `MatrixRow.NoLocation` + `MatrixViewData.HasNoLocation`; `loadMatrixRows` sets
    `NoLocation = cellSiteID == ""`, includes empty-site attendance under a site
    filter (`site='' || site='<site>'`), and stable-sorts no-location rows first.
    `handleMatrix` computes `HasNoLocation`.
- `r3-intake/internal/assets/public/index.html`: CSS `?v=5`; "No Location" group
  header gated on `.HasNoLocation` (colspan `len .Dates | add1 | add1`); combined
  `class` for dropout+no-location; per-row "no location" note.
- `r3-intake/internal/assets/public/app.css`: `.row-no-location`,
  `.row-dropout.row-no-location` (dropout dominant), `.matrix-group-header`,
  `.matrix-no-location-note`.
- `r3-intake/pocketbase/migrations/009_attendance_site_optional.go` (new): flips
  `attendance.site` from required to optional (down reverses). Required for
  empty-site saves (PocketBase RelationField rejects empty when Required).
  Registered in `002_encryption.go` `Register`.
- `r3-intake/internal/server/attendance_test.go`: extended `TestMatrixContentRender`
  with a no-location row + assertions (incl. Bob-before-Alice order).
- `r3-intake/internal/server/attendance_toggle_integration_test.go` (new):
  `TestToggleNoLocation` (200, dot-present, saved record has empty site) and
  `TestToggleLocated` (record site matches intake).

## Verification
- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — all pass (server package ok, 3.5s)
- Targeted: `TestMatrixContentRender`, `TestToggleNoLocation`, `TestToggleLocated` — PASS

## Artifacts
- Plan: `docs/plans/omp-plan-attendance-dot-no-location.md`
