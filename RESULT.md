# RESULT — Document Records event filter union semantics

## What was done
Updated `r3-intake/README.md` (Notes / inferences section) to document the new
Records screen event-filter behavior implemented by the parent epic
(t_6e8ed9af): filtering the Records list by an event now surfaces intakes whose
home event (`intake.event`) equals the selected event **OR** that have an
attendance record for that event (`attendance.intake == intake.id`) — a union.

## Where it lives
- `r3-intake/README.md` — added a "Records event filter is a union" bullet under
  "## Notes / inferences".

## Why this location
The PRD (`docs/attendance-prd.html`) and `docs/planning/epics.md` have no
dedicated Records screen section — the only existing doc reference to the Records
filter was `r3-intake/README.md:250` ("created + claimed records; admins see
all"). The README's Notes/inferences section is the correct home for this
behavioral note.

## What the bullet documents
- Union semantics: home event match OR attendance record for the event.
- All attendance statuses count (present/absent/excused/walk_in).
- Implementation detail: OR-joined `(event='<id>' || id='<id1>' || ...)` clauses
  (PocketBase v0.39 has no `in` operator), composing with `?status=`/`?q=` via ` && `.
- Zero-attendance fallback to home-event-only matching (no empty-screen regression).
- Event column still shows each intake's home event for context.

## Verification
- Docs-only change; no code touched. `git diff` shows a single 10-line addition.
