# Root Cause: Attendance dots not clickable for participants without a Location

## Task
t_774d3f6b — Investigate root cause of unclickable attendance dots without Location.

## Root Cause (confirmed)

The attendance **matrix** toggle path requires a non-empty `site_id`, and a
participant with no Location assigned has an empty `site` field. The dot is
rendered and IS clickable in the DOM, but the HTMX POST is rejected server-side
with HTTP 400 "missing required fields", so nothing visibly happens — the user
perceives the dot as "not clickable".

### The exact mechanism

1. **Template** — `r3-intake/internal/assets/public/index.html`, `matrix-cell`
   block (lines 939-951) renders a `<form hx-post="/attendance/toggle">` with a
   hidden `<input name="site_id" value="{{.SiteID}}">`.

2. **Cell SiteID source** — `r3-intake/internal/server/attendance.go`,
   `loadMatrixRows` (around line 300):
   ```go
   cellSiteID := siteID          // resolved filter site ("" = All locations)
   if cellSiteID == "" {
       cellSiteID = rec.GetString("site")   // participant's own site
   }
   ```
   For a participant with no Location, `rec.GetString("site")` is `""`, so the
   hidden `site_id` is empty.

3. **Server-side rejection** — `handleToggle` (attendance.go, line ~460):
   ```go
   if intakeID == "" || date == "" || siteID == "" {
       http.Error(w, "missing required fields", http.StatusBadRequest)
       return
   }
   ```
   Empty `site_id` → 400. The HTMX swap fails, the dot never changes, and the
   user sees no feedback. This is the bug.

### Why the day-detail edit path is NOT affected
`person_attendance.go` `handlePersonAttendanceDay` (line ~295) sets
`rec.Set("site", intake.GetString("site"))` directly and does NOT validate that
site is non-empty. So the per-person calendar day-detail edit works fine for
no-Location participants. Only the matrix toggle path is broken.

### Schema confirms Location is optional
`r3-intake/pocketbase/migrations/001_init.js` line 104: the intake `site`
relation field is `required: false`. So "no Location" is a legitimate state.

## Why the user's observation matches
- Participant "John" with no Location: dots appear but clicking does nothing
  (400 rejected, no visual change).
- After adding a Location: `site_id` becomes non-empty, toggle succeeds, dots
  become interactive. Consistent with the report.

## Fix direction (for sibling t_9c4aa7e3)
The intended behavior is that attendance can be tracked regardless of Location
(the day-detail path already allows it). The matrix toggle should not hard-fail
on empty site. Options:
1. **Preferred:** In `handleToggle`, when `site_id` is empty, fall back to the
   intake's own `site` field (may still be empty) and allow the record to be
   created with an empty site — matching the day-detail path. Remove the
   `siteID == ""` from the required-fields check (or make it non-fatal).
2. **UX (per epic):** Group no-Location participants under a "No Location"
   header in the matrix with a clear message, so the limitation is obvious.

## Files involved
- `r3-intake/internal/server/attendance.go` — `loadMatrixRows` (cell SiteID),
  `handleToggle` (400 on empty site_id)
- `r3-intake/internal/assets/public/index.html` — `matrix-cell` template
- `r3-intake/internal/server/person_attendance.go` — day-detail path (works,
  reference for the fix)
- `r3-intake/pocketbase/migrations/001_init.js` — intake.site is optional
