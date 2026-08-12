---
stepsCompleted: ["step-01-validate-prerequisites", "step-02-design-epics", "step-03-create-stories", "step-04-final-validation"]
inputDocuments:
  - docs/attendance-prd.html
  - docs/attendance-mockups.html
  - r3-intake/README.md
  - docs/issues1.md
  - docs/issues2.md
---

# R3 Attendance Tracking - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for the R3 Attendance Tracking feature, decomposing the requirements from the PRD (Calendar Matrix + Event Sessions), UX mockups, and Architecture requirements into implementable stories.

## Requirements Inventory

### Functional Requirements

FR1: System shall store attendance records with fields: event (nullable relation), intake (required relation), site (required relation), recorded_by (relation), date (YYYY-MM-DD, required), status (select: present/absent/excused/walk_in, required), check_in_time (HH:MM, optional), note (text, max 500, optional)

FR2: System shall enforce unique constraint: one attendance record per (event, intake, date), or per (intake, site, date) when event is null — enforced in Go handler (PocketBase has no native unique constraints)

FR3: System shall provide a Calendar Matrix view (GET /attendance) displaying participants as rows and dates as columns with color-coded status dots

FR4: System shall allow cell toggle via HTMX: click cycles empty → present → absent → excused → walk_in → empty (delete record). Auto-saves with no explicit save button.

FR5: System shall default the matrix to last 14 days, user's assigned site (or first active site), no event filter

FR6: System shall support matrix filters: event (optional dropdown), site (select), date range (from/to date inputs), with Apply button

FR7: System shall make the participant name column sticky during horizontal scroll (CSS position:sticky; left:0)

FR8: System shall highlight dropout rows (last present date > 14 days ago) with red row background + "STOPPED" badge

FR9: System shall provide an "Add walk-in" button: search existing intake records by name or create minimal record (name only) → adds row with walk_in status

FR10: System shall support Events CRUD: admin can create events with name (req, max 200), site (req), start_date (req), end_date (req), description (opt, max 500), status (select: active/completed/cancelled, default active)

FR11: System shall provide Event Enrollment: admin can enroll existing intake participants into events via search by name; view enrolled roster with attendance stats (days attended, rate %, last present date)

FR12: System shall allow unenroll (soft delete preserving attendance history)

FR13: System shall allow admin to change event status: active → completed, or cancelled

FR14: System shall provide CSV export with columns: Participant, Site, Event, Date, Status, Recorded By, Check-in Time, Note; filterable by event, date range, or site; includes summary stats row (total check-ins, unique participants, avg rate)

FR15: System shall provide a Per-Person Calendar view (GET /intake/{id}/attendance) showing monthly calendar with present/absent/excused/walk_in days color-coded

FR16: System shall show per-person stats: total days in period, present count, rate %, current streak

FR17: System shall allow clicking a calendar day to see detail modal (status, event, recorded by, check-in time, note) and allow editing

FR18: System shall add an "Attendance" tab to the topbar (between Records and Admin, auth-only)

FR19: System shall add an "Events" accordion section to the admin settings page (alongside Sites and Users)

FR20: System shall add an "Attendance" tab to the intake detail view (alongside the form sections and Notes)

FR21: System shall auto-save attendance toggles via HTMX (hx-post to /attendance/toggle, returns updated cell HTML for swap)

FR22: System shall support no-JS fallback: matrix toggles work via full page reload (POST → 303 redirect)

FR23: System shall cascade delete: delete event → cascade delete enrollments + attendance; delete intake → cascade delete enrollment + attendance; delete site → restricted

FR24: System shall display summary stat cards on the matrix: total check-ins, active participants, stopped count, avg attendance rate

### NonFunctional Requirements

NFR1: Performance — Average check-in time < 3 seconds per participant; matrix loads in < 2 seconds for 100 participants × 14 days

NFR2: Performance — Default date range limited to 14 days; max 30 days queryable to prevent performance issues with large datasets

NFR3: Usability — Recording daily attendance for 10 participants takes < 30 seconds total

NFR4: Usability — Matrix works on 375px width (iPhone SE); tap targets ≥ 44px; horizontal scroll for date columns

NFR5: Compatibility — No-JS fallback: all attendance toggles work via full page reload (POST → 303 redirect)

NFR6: Security — All PocketBase collection rules null (Go is policy layer); browser never talks to PB directly

NFR7: Security — Session auth required for all attendance routes; admin-only for events CRUD, enrollment, and CSV export; case managers scoped to assigned site

NFR8: Data Quality — < 5% duplicate or erroneous records; unique constraint enforced in Go handler before save

NFR9: Design — Follows existing R3 design system: Public Sans + Lora fonts, #b5502e accent, #fffdfa cards, #f7f1e6 bg, 14px card radii, 8px input radii

NFR10: Timezone — All timestamps in Hawaii Standard Time (HST/UTC-10, no DST)

NFR11: Data — Walk-in participants can be tracked and later connected to existing intake records

### Additional Requirements

- Three new PocketBase collections: `events`, `event_enrollment`, `attendance` (migration `007_events_attendance.js`)
- New Go file: `internal/server/attendance.go` with handlers: handleMatrix, handleToggle, handlePersonAttendance, handleExportCSV
- New Go structs: MatrixViewData, MatrixRow, MatrixSummary, cycleStatus(), isDropout()
- New template definitions: `{{define "matrix"}}`, `{{define "matrix-cell"}}`, `{{define "attendance-person"}}`, `{{define "admin-events"}}`, `{{define "admin-event-manage"}}`
- Route registrations in `internal/server/server.go` Mux() method
- CSS additions to `internal/assets/public/app.css` for matrix grid, sticky column, status dots, calendar, dropout highlighting
- PocketBase v0.39 JS migration API (camelCase method names, no app.dao())
- Existing patterns to follow: HTMX hx-post for partials, Alpine.js for client state, server-rendered Go templates, session cookie auth
- Helper functions needed: buildDateRange(), getParticipantsForMatrix(), cycleStatus(), isDropout()
- 11 new HTTP routes (see PRD §09 Route Map)

### UX Design Requirements

UX-DR1: Calendar Matrix grid with color-coded status dots: green (#3f6b34) present, gray (#eee + border) absent, yellow (#8a6a1e) excused, blue (#2a4d8a) walk_in

UX-DR2: Sticky participant name column during horizontal scroll (CSS position:sticky; left:0; background:var(--card))

UX-DR3: Dropout row highlighting: red background (#fbeeec) + "STOPPED" badge (10px font, #8f3a2e text, #fbeeec bg, 4px border-radius)

UX-DR4: Summary stat cards below matrix: 4 cards in flex row — total check-ins (green), active participants (accent), stopped count (red), avg rate (yellow)

UX-DR5: Filter bar: event dropdown (260px, optional), site select, from/to date inputs (120px each), Apply button (primary), Export CSV button (right-aligned)

UX-DR6: Events management table: columns Event Name, Location, Dates, Enrolled count, Status badge (active=green, completed=yellow), action buttons (Manage, Matrix, Report)

UX-DR7: Event creation form: 4-column grid (name, location, start date, end date) + description field + Create Event button; dashed accent border to distinguish from list

UX-DR8: Event enrollment screen: search input (260px) + Enroll button; roster table with Participant, Enrolled date, Days Attended (x/y), Rate % (badge), Last Present, Remove button (danger style)

UX-DR9: Per-person monthly calendar: 7-column grid, day cells with date number + status text ("✓ Present" / "Absent"), has-attendance days get green border + light green bg, today gets accent border

UX-DR10: Calendar legend: 4 color dots with labels (Present, Absent, Excused, Walk-in) in a horizontal row with light bg

UX-DR11: Mobile-optimized per-person calendar (420px max-width, narrow mockup style)

UX-DR12: Tab navigation: Records, Matrix (Attendance), Admin — active tab gets accent color + 3px bottom border

UX-DR13: Per-person calendar: prev/next month navigation arrows

### FR Coverage Map

| FR | Epic | Description |
|---|---|---|
| FR1 | Epic 1 | Attendance collection with all fields |
| FR2 | Epic 1 | Unique constraint enforcement in Go |
| FR3 | Epic 1 | Calendar Matrix view (participants × dates) |
| FR4 | Epic 1 | Cell toggle via HTMX (5-state cycle) |
| FR5 | Epic 1 | Default matrix to last 14 days, user's site |
| FR6 | Epic 1 | Matrix filters (event, site, date range) |
| FR7 | Epic 1 | Sticky participant name column |
| FR8 | Epic 1 | Dropout row highlighting |
| FR9 | Epic 1 | Walk-in check-in |
| FR10 | Epic 2 | Events CRUD (create, view, edit, complete) |
| FR11 | Epic 2 | Event enrollment with roster stats |
| FR12 | Epic 2 | Unenroll (soft delete) |
| FR13 | Epic 2 | Event status management |
| FR14 | Epic 3 | CSV export with filters + summary row |
| FR15 | Epic 4 | Per-person monthly calendar view |
| FR16 | Epic 4 | Per-person stats (total, present, rate, streak) |
| FR17 | Epic 4 | Day detail modal with edit |
| FR18 | Epic 1 | Attendance tab in topbar |
| FR19 | Epic 2 | Events accordion in admin |
| FR20 | Epic 4 | Attendance tab on intake detail |
| FR21 | Epic 1 | HTMX auto-save on toggle |
| FR22 | Epic 1 | No-JS fallback (POST → 303 redirect) |
| FR23 | Epic 1 | Cascade delete rules |
| FR24 | Epic 1 | Summary stat cards on matrix |

**All 24 FRs mapped.** No gaps.

## Epic List

### Epic 1: Daily Attendance Tracking (Calendar Matrix)

Coordinators can record daily check-ins and see attendance patterns over time using a spreadsheet-style grid. Works Day 1 without any events created.

- **FRs covered:** FR1, FR2, FR3, FR4, FR5, FR6, FR7, FR8, FR9, FR18, FR21, FR22, FR23, FR24
- **NFRs addressed:** NFR1, NFR2, NFR3, NFR4, NFR5, NFR6, NFR7, NFR8, NFR9, NFR10
- **UX-DRs addressed:** UX-DR1, UX-DR2, UX-DR3, UX-DR4, UX-DR5, UX-DR12
- **Implementation scope:** DB migration (007_events_attendance.js creates all 3 collections), new file `internal/server/attendance.go`, new templates `{{define "matrix"}}` + `{{define "matrix-cell"}}`, new routes, CSS for grid/dots/sticky column, Attendance tab in topbar.
- **Standalone value:** Yes — full daily check-in capability without events. Events filter is optional.

### Epic 2: Event Program Management

Admins can create multi-week outreach programs (events), enroll participants from existing intake records, and manage event lifecycle (active → completed → cancelled).

- **FRs covered:** FR10, FR11, FR12, FR13, FR19
- **NFRs addressed:** NFR6, NFR7
- **UX-DRs addressed:** UX-DR6, UX-DR7, UX-DR8
- **Implementation scope:** Extend `internal/server/admin.go` with event handlers, new templates `{{define "admin-events"}}` + `{{define "admin-event-manage"}}`, Events accordion in admin page, matrix gets optional event filter dropdown.
- **Standalone value:** Yes — events can be created and managed independently. Matrix works without events (Epic 1), but events add organizational layer for reporting.

### Epic 3: Attendance Reporting & Export

Admins can export attendance data as CSV for funder reports and board presentations, filtered by event, date range, or site.

- **FRs covered:** FR14
- **NFRs addressed:** NFR6, NFR7
- **UX-DRs addressed:** UX-DR5 (Export button)
- **Implementation scope:** Add handleExportCSV to `internal/server/attendance.go`, new route GET /attendance/export, CSV columns: Participant, Site, Event, Date, Status, Recorded By, Check-in Time, Note + summary stats row.
- **Standalone value:** Yes — reporting is distinct from daily operations. Can be used independently once any attendance data exists.

### Epic 4: Participant Attendance History

Case managers can view a single participant's attendance history as a monthly calendar, with stats and day-level detail, accessible from the intake record.

- **FRs covered:** FR15, FR16, FR17, FR20
- **NFRs addressed:** NFR1, NFR4, NFR6, NFR7, NFR9, NFR10
- **UX-DRs addressed:** UX-DR9, UX-DR10, UX-DR11, UX-DR13
- **Implementation scope:** Add handlePersonAttendance to `internal/server/attendance.go`, new template `{{define "attendance-person"}}`, calendar CSS, Attendance tab on intake detail view, day detail modal with edit.
- **Standalone value:** Yes — per-person view is distinct from matrix. Case managers can review individual engagement patterns independently.

---

## Epic 1: Daily Attendance Tracking (Calendar Matrix)

Coordinators can record daily check-ins and see attendance patterns over time using a spreadsheet-style grid. Works Day 1 without any events created.

### Story 1.1: Database Migration & Attendance Tab

As a developer,
I want to create the PocketBase collections for events, enrollment, and attendance, and add an Attendance tab to the topbar,
So that the foundation exists for all attendance tracking features.

**Acceptance Criteria:**

**Given** the R3 Intake server is started with PocketBase v0.39
**When** the server runs pending migrations on boot
**Then** three new collections are created: `events`, `event_enrollment`, and `attendance`
**And** the `attendance` collection has fields: event (nullable relation→events), intake (required relation→intake), site (required relation→sites), recorded_by (relation→users), date (text, required), status (select: present/absent/excused/walk_in, required), check_in_time (text, optional), note (text, max 500, optional), created (autodate), updated (autodate)
**And** the `events` collection has fields: site (relation→sites), name (text, req, max 200), start_date (text, req), end_date (text, req), description (text, opt, max 500), status (select: active/completed/cancelled), created_by (relation→users), created/updated (autodate)
**And** the `event_enrollment` collection has fields: event (relation→events, cascade), intake (relation→intake, cascade), enrolled_date (text, opt), created (autodate)
**And** all three collections have null list/view/create/update/delete rules (Go is policy layer)
**And** cascade delete is configured: event→enrollment+attendance, intake→enrollment+attendance, site→restricted

**Given** an authenticated user views any R3 Intake page
**When** the topbar renders
**Then** an "Attendance" tab appears between "Records" and "Admin"
**And** clicking it navigates to `GET /attendance`
**And** unauthenticated users are redirected to `/public/intake` (same as Records tab)

**Given** the migration `007_events_attendance.js` is placed at `r3-intake/pocketbase/migrations/`
**When** the migration runs
**Then** it references existing `intake`, `sites`, and `users` collections by name
**And** the down migration drops collections in reverse dependency order: attendance, event_enrollment, events

### Story 1.2: Calendar Matrix View with Filters

As a coordinator,
I want to see a spreadsheet-style grid showing participants as rows and dates as columns with color-coded status dots,
So that I can see attendance patterns at a glance and filter by site and date range.

**Acceptance Criteria:**

**Given** an authenticated user navigates to `/attendance`
**When** the page loads with no query parameters
**Then** the matrix displays the last 14 days as column headers (YYYY-MM-DD format)
**And** participant rows show all intake records at the user's assigned site (or first active site if admin)
**And** each cell shows a color-coded dot: green (#3f6b34) for present, gray (#eee + border) for absent, yellow (#8a6a1e) for excused, blue (#2a4d8a) for walk_in, or empty (no dot) for no record
**And** the participant name column is sticky during horizontal scroll (CSS position:sticky; left:0; background:var(--card))
**And** a "Total" column shows each participant's present count as "x/y" badge

**Given** the matrix filter bar is displayed
**When** the user selects a different site, enters a date range (from/to), and clicks "Apply"
**Then** the matrix reloads with the applied filters
**And** the URL updates to include query params: `?site={id}&from=YYYY-MM-DD&to=YYYY-MM-DD`
**And** the date range is limited to max 30 days (if user enters more, it's truncated to 30 days from the start date)

**Given** an authenticated user with role "case_manager"
**When** they view the matrix
**Then** they see only their assigned site's participants
**And** they cannot select other sites in the filter

**Given** an admin user views the matrix
**When** they open the site filter
**Then** they can select any active site or "All locations"
**And** "All locations" shows participants from all active sites

**Given** the matrix renders with more dates than fit in the viewport
**When** the user scrolls horizontally
**Then** the participant name column remains fixed on the left
**And** date columns scroll smoothly beneath

**Given** the matrix loads with 100 participants × 14 date columns
**When** the page renders
**Then** it loads in under 2 seconds

### Story 1.3: Cell Toggle with HTMX Auto-Save

As a coordinator,
I want to click a cell in the matrix to toggle attendance status and have it auto-save,
So that I can record check-ins in under 3 seconds per participant without clicking a save button.

**Acceptance Criteria:**

**Given** a matrix cell is empty (no attendance record)
**When** the user clicks the cell
**Then** the status cycles to "present" and a green dot appears
**And** an HTMX POST request is sent to `/attendance/toggle` with intake_id, date, site_id (and event_id if event filter is active)
**And** the server creates a new attendance record with status="present", recorded_by=current user, check_in_time=current HST time
**And** the response returns updated cell HTML for HTMX swap

**Given** a matrix cell shows "present" status
**When** the user clicks the cell
**Then** the status cycles to "absent" (yellow dot)
**And** the existing attendance record is updated (not duplicated)

**Given** the status cycle order is: empty → present → absent → excused → walk_in → empty
**When** the user clicks through all 5 states and clicks once more
**Then** the cell returns to empty
**And** the attendance record is deleted from the database

**Given** an attendance record already exists for (intake, date) — or (event, intake, date) if event is selected
**When** the user toggles the cell
**Then** the existing record is updated, never duplicated
**And** the Go handler checks for existing records before creating new ones (unique constraint enforcement)

**Given** JavaScript is disabled in the browser
**When** the user clicks a cell
**Then** a POST form submission occurs and the page reloads with a 303 redirect
**And** the attendance status is still toggled correctly
**And** the page renders at the same scroll position with the updated cell

**Given** the user toggles 10 participants to "present" for today's date
**When** all toggles complete
**Then** total elapsed time is under 30 seconds
**And** all 10 records are persisted with correct status, recorded_by, and check_in_time

### Story 1.4: Walk-in Check-in & Dropout Highlighting

As a coordinator,
I want to check in walk-in participants who aren't in the system yet, and see which enrolled participants have stopped attending,
So that I can capture everyone who shows up and follow up with dropouts.

**Acceptance Criteria:**

**Given** the matrix is displayed
**When** the user clicks "Add walk-in"
**Then** a search input appears where the user can type a name
**And** as the user types, existing intake records matching the name are shown as suggestions (HTMX search, same pattern as duplicate search in intake form)
**And** if an existing record is selected, that participant is added to the matrix with a walk_in status for today's date
**And** if no match is found, the user can create a minimal intake record (name only) which is added to the matrix with walk_in status

**Given** a walk-in participant is checked in
**When** the matrix renders
**Then** their row shows a blue (#2a4d8a) walk_in dot for today's date
**And** the walk-in participant appears in future matrix views for the same site

**Given** a participant's last "present" attendance date is more than 14 days ago
**When** the matrix renders their row
**Then** the row has a red background (#fbeeec)
**And** a "STOPPED" badge appears next to their name (10px font, #8f3a2e text, #fbeeec bg, 4px border-radius)

**Given** a participant has no attendance records at all
**When** the matrix renders their row
**Then** the row is not highlighted as a dropout (only participants with at least one present record who then stopped are flagged)

### Story 1.5: Summary Stats Cards

As a coordinator,
I want to see summary statistics below the matrix,
So that I can quickly understand overall attendance trends without counting cells.

**Acceptance Criteria:**

**Given** the matrix is displayed with data
**When** the user looks below the matrix grid
**Then** four stat cards appear in a flex row:
- "Total check-ins" (green #3f6b34 number) — count of all present + walk_in records in the current date range
- "Active participants" (accent #b5502e number) — count of participants with at least 1 present record in range
- "Stopped" (red #8f3a2e number) — count of participants flagged as dropouts (>14 days since last present)
- "Avg attendance rate" (yellow #8a6a1e number) — percentage: total present / (participants × days in range)

**Given** the matrix filter changes (different site, date range, or event)
**When** the matrix reloads
**Then** the stat cards recalculate based on the filtered data
**And** if no attendance records exist in the filter range, all cards show 0 (or 0% for rate)

**Given** the stat cards render on a 375px mobile viewport
**When** they display
**Then** they wrap to 2 columns × 2 rows (flex-wrap)
**And** each card maintains its colored number and muted label

---

## Epic 2: Event Program Management

Admins can create multi-week outreach programs (events), enroll participants from existing intake records, and manage event lifecycle (active → completed → cancelled).

### Story 2.1: Events List & Creation

As an admin,
I want to view all events in a list and create new outreach programs,
So that I can organize attendance by program cycle for reporting.

**Acceptance Criteria:**

**Given** an admin user navigates to `/admin` and the admin settings page renders
**When** the accordion sections display
**Then** a new "Events" accordion section appears alongside "Sites" and "Users"
**And** expanding it shows a table with columns: Event Name, Location, Dates, Enrolled count, Status badge, Actions

**Given** the events list is displayed
**When** an event has status "active"
**Then** a green badge ("Active") is shown
**And** when status is "completed", a yellow badge ("Completed") is shown
**And** when status is "cancelled", a red badge ("Cancelled") is shown

**Given** the admin clicks "+ New Event"
**When** the event creation form renders
**Then** it shows a 4-column grid: Event Name (text input), Location (select with active sites), Start Date (date input), End Date (date input)
**And** a Description field (optional, max 500 chars)
**And** a "Create Event" button (primary style)

**Given** the admin fills in all required fields (name, site, start_date, end_date)
**When** they click "Create Event"
**Then** a new event record is created with status="active", created_by=current user
**And** the page reloads showing the new event in the list
**And** if any required field is missing, the form shows a validation error

**Given** an event exists in the list
**When** the admin clicks "Manage"
**Then** they navigate to `/admin/events/{id}/manage` (enrollment screen)

**Given** an event exists in the list
**When** the admin clicks "Matrix"
**Then** they navigate to `/attendance?event={id}` with the event pre-selected in the filter

### Story 2.2: Event Enrollment & Roster

As an admin,
I want to enroll participants into an event and see their attendance stats,
So that I can track who is expected at each program cycle.

**Acceptance Criteria:**

**Given** the admin is on the event management screen (`/admin/events/{id}/manage`)
**When** the page loads
**Then** it shows the event name, location, date range, and status in the header
**And** an "Enrolled (N)" tab is active showing the roster table
**And** the roster table has columns: Participant, Enrolled date, Days Attended (x/y), Rate % (badge), Last Present date, Remove button (danger style)

**Given** the admin types a name in the "Add participant" search input
**When** matching intake records are found
**Then** suggestions appear (HTMX search, same pattern as intake form duplicate search)
**And** selecting a participant and clicking "+ Enroll" creates an event_enrollment record
**And** the page reloads with the new participant in the roster

**Given** a participant is enrolled in the event
**When** the roster renders
**Then** their "Days Attended" shows "X / Y" where X = count of attendance records with status present or walk_in for this event, and Y = number of days from event start_date to today (or end_date if past)
**And** their "Rate %" shows a colored badge: green if ≥50%, red if <50%
**And** their "Last Present" shows the date of their most recent present attendance (HST formatted)

**Given** a participant is enrolled in the event
**When** the admin clicks "Remove"
**Then** the enrollment record is soft-deleted (preserves attendance history)
**And** the participant is removed from the roster
**And** their attendance records remain in the database for historical reporting

**Given** the admin clicks "View Matrix"
**When** the matrix loads
**Then** it is filtered to this event (`?event={id}`)
**And** only enrolled participants appear as rows (plus any walk-ins for this event)

### Story 2.3: Event Status Management

As an admin,
I want to change an event's status from active to completed or cancelled,
So that I can manage the event lifecycle and generate final reports.

**Acceptance Criteria:**

**Given** an event has status "active"
**When** the admin clicks "Complete" on the event management screen
**Then** the event status changes to "completed"
**And** the status badge updates to yellow ("Completed")
**And** the event remains viewable in the events list and matrix

**Given** an event has status "active"
**When** the admin clicks "Cancel"
**Then** the event status changes to "cancelled"
**And** a confirmation prompt appears before the change ("Mark this event as cancelled?")

**Given** an event has status "completed" or "cancelled"
**When** the admin views the event management screen
**Then** the enrollment and status change actions are disabled (read-only)
**And** a "Report" button appears in the events list for completed events

**Given** an admin attempts to change event status
**When** they are not an admin (role ≠ "admin")
**Then** the action is forbidden (403 or redirect to /)

---

## Epic 3: Attendance Reporting & Export

Admins can export attendance data as CSV for funder reports and board presentations, filtered by event, date range, or site.

### Story 3.1: CSV Export

As an admin,
I want to export attendance data as a CSV file,
So that I can include attendance records in funder reports and board presentations.

**Acceptance Criteria:**

**Given** the matrix is displayed with filters applied (site, date range, event)
**When** the admin clicks "Export CSV"
**Then** a CSV file downloads with filename `attendance_{site}_{from}_{to}.csv`
**And** the CSV contains columns: Participant, Site, Event, Date, Status, Recorded By, Check-in Time, Note
**And** each row represents one attendance record matching the current filters
**And** the Status column uses human-readable values ("Present", "Absent", "Excused", "Walk-in")
**And** the Event column shows the event name or "(No event)" if null
**And** the Recorded By column shows the user's name

**Given** the CSV export includes data
**When** the CSV is opened
**Then** a summary stats row appears at the bottom: "Summary: {total_check_ins} check-ins, {unique_participants} unique participants, {avg_rate}% avg attendance rate"

**Given** a case_manager (non-admin) attempts to export
**When** they view the matrix
**Then** the "Export CSV" button is not visible
**And** a direct request to `/attendance/export` returns 403

**Given** no attendance records match the current filters
**When** the admin exports
**Then** the CSV contains only the header row and a summary row with zeros

---

## Epic 4: Participant Attendance History

Case managers can view a single participant's attendance history as a monthly calendar, with stats and day-level detail, accessible from the intake record.

### Story 4.1: Per-Person Calendar View

As a case manager,
I want to view a participant's attendance history as a monthly calendar,
So that I can discuss their engagement patterns during case management meetings.

**Acceptance Criteria:**

**Given** a case manager is viewing an intake record at `/intake/{id}`
**When** they click the new "Attendance" tab
**Then** they navigate to `/intake/{id}/attendance`
**And** a monthly calendar renders with 7 columns (days of the week)
**And** the current month is displayed by default
**And** each day cell shows the date number and status text ("✓ Present", "Absent", "Excused", "Walk-in") if an attendance record exists
**And** days with present/walk_in status get a green border (#cfe0c6) and light green background (#f6fbf4)
**And** today's date gets an accent border (#b5502e, 2px)
**And** days with no attendance record are plain (default card style)

**Given** the calendar is displayed
**When** the case manager clicks the prev/next month arrows
**Then** the calendar navigates to the previous/next month
**And** the URL updates to include `?month=YYYY-MM`

**Given** the calendar renders
**When** a day has attendance data
**Then** the status text color matches the matrix dots: green for present, gray for absent, yellow for excused, blue for walk-in
**And** a legend appears below the calendar showing all 4 color dots with labels (Present, Absent, Excused, Walk-in)

### Story 4.2: Per-Person Attendance Stats

As a case manager,
I want to see attendance statistics for a participant,
So that I can quickly assess their engagement level.

**Acceptance Criteria:**

**Given** the per-person calendar is displayed
**When** the stats row renders above the calendar
**Then** it shows: participant name, site name, "X of Y days (Z%)" where X=present count, Y=total attendance records, Z=rate percentage
**And** the rate percentage is color-coded: green if ≥50%, red if <50%

**Given** the participant has attendance records in the current month
**When** the stats calculate
**Then** "X" counts only present and walk_in statuses (not absent or excused)
**And** "Y" counts all attendance records in the visible month
**And** "Current streak" shows the number of consecutive days with present status ending today (or most recent present date)

**Given** the participant has no attendance records
**When** the calendar renders
**Then** the stats show "0 of 0 days (0%)" and "No attendance recorded yet"

### Story 4.3: Day Detail & Edit

As a case manager,
I want to click a calendar day to see attendance details and edit them,
So that I can correct mistakes or add notes after the fact.

**Acceptance Criteria:**

**Given** a calendar day has an attendance record
**When** the case manager clicks the day cell
**Then** a modal opens showing: status (with dropdown to change), event name (if linked), recorded by (user name), check-in time, note (editable textarea)
**And** a "Save" button and "Cancel" button are displayed

**Given** the case manager changes the status or edits the note in the modal
**When** they click "Save"
**Then** the attendance record is updated via POST
**And** the calendar refreshes to show the updated status
**And** the modal closes

**Given** the case manager clicks a day with no attendance record
**When** the modal opens
**Then** it shows "No attendance recorded" with an option to add a new record (status dropdown + note field)
**And** saving creates a new attendance record for that date

**Given** the case manager wants to delete an attendance record
**When** they open the day detail modal and click "Delete"
**Then** a confirmation prompt appears ("Delete this attendance record?")
**And** confirming deletes the record and refreshes the calendar
