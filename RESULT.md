# Epic 5: Attendance dots not clickable for participants without a Location — RESULT

**Status:** COMPLETE — child story branches `wt/t_774d3f6b` (root-cause investigation) and `wt/t_9c4aa7e3` (fix + No Location UI group) merged into `epic/5-attendance-dots-not-clickable-for-partic`.

## Stories implemented

- **5.1 Root-cause investigation** — `wt/t_774d3f6b`
- **5.2 Attendance dot fix + "No Location" UI grouping** — `wt/t_9c4aa7e3`

## Root-cause investigation (from `wt/t_774d3f6b`)

The attendance **matrix** toggle path requires a non-empty `site_id`, and a participant with no Location assigned has an empty `site` field. The dot is rendered and IS clickable in the DOM, but the HTMX POST is rejected server-side with HTTP 400 "missing required fields", so nothing visibly happens — the user perceives the dot as "not clickable".

### The exact mechanism

1. **Template** — `r3-intake/internal/assets/public/index.html`, `matrix-cell` block renders a `<form hx-post="/attendance/toggle">` with a hidden `<input name="site_id" value="{{.SiteID}}">`.
2. **Cell SiteID source** — `r3-intake/internal/server/attendance.go`, `loadMatrixRows`:
   ```go
   cellSiteID := siteID          // resolved filter site ("" = All locations)
   if cellSiteID == "" {
       cellSiteID = rec.GetString("site")   // participant's own site
   }
   ```
   For a participant with no Location, `rec.GetString("site")` is `""`, so the hidden `site_id` is empty.
3. **Server-side rejection** — `handleToggle` in `attendance.go`:
   ```go
   if intakeID == "" || date == "" || siteID == "" {
       http.Error(w, "missing required fields", http.StatusBadRequest)
       return
   }
   ```
   Empty `site_id` → 400. The HTMX swap fails, the dot never changes, and the user sees no feedback. This is the bug.

### Why the day-detail edit path is NOT affected

`person_attendance.go` `handlePersonAttendanceDay` sets `rec.Set("site", intake.GetString("site"))` directly and does NOT validate that site is non-empty. So the per-person calendar day-detail edit works fine for no-Location participants. Only the matrix toggle path is broken.

### Schema confirms Location is optional

`r3-intake/pocketbase/migrations/001_init.js`: the intake `site` relation field is `required: false`. So "no Location" is a legitimate state.

## What was built / fix (from `wt/t_9c4aa7e3`)

Fixed the attendance matrix so participants WITHOUT a Location (intake.site empty) can have their attendance dots toggled, and added a "No Location" group at the top of the attendance list with clear visual separation and an indicator.

### Changes

- `r3-intake/internal/server/attendance.go`
  - `handleToggle`: required check now only `intakeID`/`date`. Effective site falls back to the intake's own site when `site_id` is empty (may still be empty), matching the day-detail path. Missing intake proceeds with empty site rather than erroring.
  - `MatrixRow.NoLocation` + `MatrixViewData.HasNoLocation`; `loadMatrixRows` sets `NoLocation = cellSiteID == ""`, includes empty-site attendance under a site filter (`site='' || site='<site>'`), and stable-sorts no-location rows first. `handleMatrix` computes `HasNoLocation`.
- `r3-intake/internal/assets/public/index.html`: CSS `?v=5`; "No Location" group header gated on `.HasNoLocation` (colspan `len .Dates | add1 | add1`); combined `class` for dropout+no-location; per-row "no location" note.
- `r3-intake/internal/assets/public/app.css`: `.row-no-location`, `.row-dropout.row-no-location` (dropout dominant), `.matrix-group-header`, `.matrix-no-location-note`.
- `r3-intake/pocketbase/migrations/009_attendance_site_optional.go` (new): flips `attendance.site` from required to optional (down reverses). Registered in `002_encryption.go` `Register`.
- `r3-intake/internal/server/attendance_test.go`: extended `TestMatrixContentRender` with a no-location row + assertions (incl. Bob-before-Alice order).
- `r3-intake/internal/server/attendance_toggle_integration_test.go` (new): `TestToggleNoLocation` (200, dot-present, saved record has empty site) and `TestToggleLocated` (record site matches intake).

## Verification

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — all pass
- Targeted: `TestMatrixContentRender`, `TestToggleNoLocation`, `TestToggleLocated` — PASS

## Artifacts

- Plan: `docs/plans/omp-plan-attendance-dot-no-location.md`
