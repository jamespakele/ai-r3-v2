# Working Plan: Fix handleEnrollSearch to allow cross-site participant search and enrollment

> **For Hermes:** Use subagent-driven-development to implement this plan task-by-task.

**Goal:** Remove the site restriction from the "Add participant" search so intake records from ALL sites can be found and enrolled into an event.

**Architecture:** A one-line behavioral change in a single Go handler. `handleEnrollSearch` currently appends `&& site='<eventSite>'` to its PocketBase filter when the event has a site. Removing that clause makes the search cross-site. The result fragment already renders each hit's site name (`SiteName`), and the enrollment handler (`handleEventEnroll`) does not restrict by site, so no other code changes are required.

**Tech Stack:** Go, embedded PocketBase (v0.39 Go API), server-rendered Go templates, HTMX + Alpine.js.

---

## Objective

On the Event Manage page (`/admin/events/{id}/manage`), the "Add participant" search (`handleEnrollSearch`) filters intake records by name **and** restricts results to intakes at the event's site (`site='<eventSite>'`). When a participant is at a different site — or the event has no site set — the search returns nothing, so they cannot be added.

Build: allow cross-site participant search and enrollment by removing the site restriction from the search filter. The search should return matching intake records from **all** sites, each labeled with its own site name, and those records should be enrollable into the event regardless of site.

## Constraints

- **Language:** Go (module `r3-intake`).
- **Framework:** Embedded PocketBase v0.39 Go API (`s.pb.FindRecordsByFilter`, `s.pb.FindRecordById`, `mcpmod.EscapeFilter`). No `app.dao()`.
- **Dependencies:** No new dependencies. Reuse existing `mcpmod "r3-intake/internal/mcp"` for filter escaping.
- **Scope:** Only `r3-intake/internal/server/admin.go` changes. **No** template changes (the `enroll-search-results` fragment already renders `SiteName`), **no** schema/migration changes, **no** new collections.
- **Behavioral invariants to preserve:** min 2 chars before searching, cap ~10 results, `Already` flag to disable re-add, event-existence validation, soft-delete (`deleted=false`) idempotency check on enrollment.
- **Design system:** Public Sans + Lora, accent `#b5502e`. No UI changes, so no CSS/token work.

## File Structure

- **Modify:** `r3-intake/internal/server/admin.go` — `handleEnrollSearch` (~lines 735–787). Remove the `eventSite` variable and the `if eventSite != ""` block that appends the site clause to the filter.
- **No other files change.** Templates, tests, migrations, and CSS are untouched.

## Implementation Notes

### The change (in `handleEnrollSearch`)

1. **Keep** the event-existence validation: `eventRec, err := s.pb.FindRecordById("events", id)` — this still guards against a bad/unknown event id and must stay.
2. **Remove** the line `eventSite := eventRec.GetString("site")` — the derived variable is no longer needed.
3. **Remove** the block:
   ```go
   if eventSite != "" {
       filter += fmt.Sprintf(" && site='%s'", mcpmod.EscapeFilter(eventSite))
   }
   ```
4. The filter becomes just:
   ```go
   filter := fmt.Sprintf(`name ~ "%s"`, mcpmod.EscapeFilter(q))
   ```
   This returns matching intake records from **all** sites.

### Why this is sufficient

- **Display:** Each result already carries `SiteName: siteMap[rec.GetString("site")]`, and the `enroll-search-results` template renders it as `<span class="enroll-result-meta">{{.SiteName}}</span>` (index.html ~L1410). Cross-site hits will show their own site name with no template change.
- **Enrollment:** `handleEventEnroll` does **not** filter by site — it only verifies the event and intake exist and creates the `event_enrollment` record. So removing the search filter is the only change needed to allow cross-site enrollment.
- **Idempotency:** The per-result `Already` check (`event='%s' && intake='%s' && deleted=false`) is unchanged, so a participant already enrolled at this event is still marked and cannot be re-added.

### Edge cases

- **Event with no site set:** Previously the search returned nothing (empty `eventSite` → no site clause, but the bug report notes empty-site events still failed to surface cross-site participants). After the fix, the search is purely name-based and works regardless of the event's site value.
- **Empty/too-short query:** `len(q) < 2` early-return is unchanged — no search fires for 0–1 char input.
- **No matches:** The handler renders an empty results fragment (existing behavior); no error path changes.
- **Filter escaping:** `mcpmod.EscapeFilter(q)` still guards the user-supplied name against filter injection. The removed site clause was also escaped, but it is gone entirely.
- **`eventRec` unused warning:** After removing `eventSite`, `eventRec` is still used only for the existence check (`err`). Go will not complain because `eventRec` is assigned via `:=` and `err` is checked; `eventRec` itself is unused but that is fine (it is a declared-and-used-in-`:=` variable — no compile error). If desired, replace with `_, err := s.pb.FindRecordById(...)` to be explicit, but this is optional.

## Verification Criteria

1. **Build:** From the `r3-intake/` directory, run:
   ```bash
   go build ./...
   ```
   Expected: exit 0, no errors.

2. **Vet:** Run:
   ```bash
   go vet ./...
   ```
   Expected: exit 0, no findings.

3. **Tests:** Run:
   ```bash
   go test ./...
   ```
   Expected: all pass, including `TestEnrollSearchResultsRender` in `admin_events_test.go` (renders the `enroll-search-results` fragment and asserts names, the `action="/admin/events/ev1/enroll"` target, `intake_id` value, "+ Enroll", and "Already enrolled"). This test does **not** exercise the site filter, so it remains green.

4. **Manual / behavioral check (optional, if a test server is available):** On the Event Manage page, search a participant whose intake `site` differs from the event's `site` (or an event with no site). Confirm the participant appears in the results with their own site name shown, and that clicking "+ Enroll" successfully creates the enrollment.

5. **Regression guard:** Confirm the search still returns nothing for a 0–1 char query and still marks already-enrolled participants as "Already enrolled".
