# Working Plan: Collapse Create Event form behind toggle button

## Objective
On the R3 Intake Admin > Events tab, the Create Event form (name, location,
start/end dates, description) is currently ALWAYS visible. Collapse it so it is
hidden by default and only appears when the user clicks a "Create Event" button.
Clicking the button again (now labeled "Cancel") hides the form. The form
renders below the button.

## Constraints
- Front-end only. No Go handler, view-model, or route changes. The POST target
  `/admin/events` and the `AdminView` fields (`.EventName`, `.EventSite`,
  `.EventStart`, `.EventEnd`, `.EventDescription`, `.EventError`) are untouched.
- Reuse the existing toggle pattern already in the file: `window.toggleEditRow`
  (index.html ~L880) toggles an element's `style.display` and swaps a button
  label between two strings. Mirror that pattern for the create form.
- The button label toggles between "Create Event" (form hidden) and "Cancel"
  (form shown).
- The form appears BELOW the button.
- This card is ONLY Change 2 (collapse the form). The CSS cache-buster `?v=`
  version bump is owned by a SIBLING card and is explicitly OUT OF SCOPE here.
- No generic `.hidden` class exists. `.tab-panel[hidden] { display: none; }`
  is the only `[hidden]` rule and is scoped to tab panels. The create form is
  NOT a tab panel, so do NOT rely on the `hidden` attribute for it — use the
  same inline `style="display:none"` + `style.display` toggle that
  `toggleEditRow` uses. (Optionally a `.hidden` utility could be added to
  app.css, but that is a CSS change and unnecessary; inline style matches the
  established pattern.)
- `.btn-auto { justify-self: start; width: auto; }` (app.css L75) already
  exists and is the correct class for the toggle button so it does not stretch
  across the grid.

## File Structure
- `r3-intake/internal/assets/public/index.html` — the ONLY file changed.
  - `{{define "admin"}}` block, Events tab panel (`panel-events`), the region
    around L734-745.
  - The inline `<script>` block near L880 where `window.toggleEditRow` is
    defined — add the new toggle function alongside it.
- No changes to `app.css`, `admin.go`, `admin_events_test.go`, or any Go file.

## Implementation Notes
1. **Add a toggle button above the form.** Insert a
   `<button type="button" class="btn btn-primary btn-auto"
   onclick="toggleCreateForm('create-event-form', this)">Create Event</button>`
   immediately before the `{{if .EventError}}` block (or directly above the
   form). It is a `type="button"` so it does NOT submit the form. It sits
   OUTSIDE the `<form>` element (see Logical Consequences).
2. **Give the form an id and hide it by default.** Add `id="create-event-form"`
   and `style="display:none"` to the existing
   `<form method="post" action="/admin/events" class="form-grid-2">` element.
   The form's inner fields, submit button, and `{{.EventName}}`/`{{.EventSite}}`/
   `{{.EventStart}}`/`{{.EventEnd}}`/`{{.EventDescription}}` bindings are
   UNCHANGED.
3. **Add the toggle function** next to `window.toggleEditRow`:
   `window.toggleCreateForm = function (formId, btn) { var f =
   document.getElementById(formId); if (!f) return; var showing = f.style.display
   === 'block'; f.style.display = showing ? 'none' : 'block';
   btn.textContent = showing ? 'Create Event' : 'Cancel'; };`
   - The form is a block-level grid element, so toggle between `'none'` and
     `'block'` (NOT `'table-row'` like the edit rows). `form-grid-2` sets
     `display: grid`, but the inline `style.display` assignment overrides it;
     `'block'` is the safe, simple value and matches the visual result.
   - The button label swap mirrors `toggleEditRow`'s `btn.textContent` swap.
4. **EventError / pre-filled values behavior.** The `{{if .EventError}}` error
   div and the form's pre-filled `value="{{.EventName}}"` etc. remain in the
   DOM. When the server re-renders after a failed POST (validation error), the
   form is hidden by default again, but the error message and pre-filled values
   are still present. See Logical Consequences for the recommended UX handling.
5. **Do NOT touch the CSS cache-buster** `?v=` on the stylesheet link — that is
   the sibling card's responsibility.

## Logical Consequences
Trace every downstream site:

- **Submit button inside the form.** The existing
  `<button type="submit" class="btn btn-primary btn-auto">Create Event</button>`
  (L744) stays INSIDE the form and keeps its label "Create Event". It is the
  form's submit action and is only reachable once the form is shown. Because the
  toggle button (outside the form) and the submit button (inside the form) would
  BOTH read "Create Event", the toggle button's label is the one that flips to
  "Cancel" when shown — the submit button's label is never changed. This is
  acceptable and matches the requirement ("button text toggles between Create
  Event and Cancel" refers to the toggle button). No change to the submit
  button.

- **EventError display.** The `{{if .EventError}}<div class="form-error">...`
  (L734) sits ABOVE the form. If the form is hidden by default, a validation
  error would render the error text but the form (and its pre-filled values)
  would be hidden — the user sees an error with no form. Two options:
  (a) Keep it simple: leave the error div as-is; the error text appears above
  the hidden form. (b) Better UX: also auto-show the form when `.EventError` is
  set, by rendering the toggle button with the "Cancel" label and the form
  visible when `{{if .EventError}}` is true. The existing script already
  auto-activates the Events tab when `.EventError` is set (L868-870), so
  auto-showing the form on error is consistent. RECOMMENDED: render the form
  visible (no `style="display:none"`) and the button labeled "Cancel" when
  `{{if .EventError}}` is true, so the error is actionable. This is a small
  template conditional, not a Go change.

- **Pre-filled values from .EventName/.EventSite/.EventStart/.EventEnd/
  .EventDescription.** These bindings are unchanged and remain in the DOM. When
  the form is hidden by default, the values are still rendered into the hidden
  inputs (harmless). When shown (via toggle or auto-show-on-error), the
  pre-filled values appear correctly. No change to the bindings.

- **.btn-auto class.** Already defined (app.css L75) and used by the submit
  button. Reuse it on the new toggle button so it is left-aligned and
  auto-width instead of stretching across the `form-grid-2` grid. No CSS change
  needed.

- **Toggle button: separate element outside the form, or reuse the submit
  button?** It MUST be a SEPARATE element OUTSIDE the `<form>`. Reasons:
  - The submit button is `type="submit"`; clicking it submits the form. A
    toggle button must be `type="button"` to avoid submitting. Reusing the
    submit button as the toggle would break form submission.
  - The submit button is INSIDE the form, so it is hidden whenever the form is
    hidden — there would be no visible button to click to show the form.
  - The requirement says the form appears BELOW the button, so the button must
    be a sibling rendered above the form, not a child of it.
  Therefore: add a new `type="button"` toggle button as a sibling ABOVE the
  form, and keep the submit button inside the form unchanged.

- **Existing tests.** `admin_events_test.go` `TestAdminEventsRender` asserts
  the output contains `action="/admin/events"` and `Create Event` (L55). Both
  strings still appear (the form action is unchanged; the toggle button adds a
  second "Create Event" label). The validation-error assertions
  (`value="Kept Name"`, `value="2026-08-01"`, the error message) still pass
  because the bindings are unchanged. If the auto-show-on-error conditional is
  added, the error-path render still contains all asserted strings. No test
  changes required, but the plan should note the toggle button adds a second
  "Create Event" occurrence.

## Verification Criteria
- [ ] `grep -n "create-event-form" r3-intake/internal/assets/public/index.html`
      shows the form has `id="create-event-form"` and `style="display:none"`.
- [ ] `grep -n "toggleCreateForm" r3-intake/internal/assets/public/index.html`
      shows the new function defined and referenced by the toggle button.
- [ ] The toggle button is `type="button"` and sits OUTSIDE the form, above it.
- [ ] The submit button remains `type="submit"` inside the form, label
      "Create Event", class `btn btn-primary btn-auto` unchanged.
- [ ] The form's field bindings (`.EventName`, `.EventSite`, `.EventStart`,
      `.EventEnd`, `.EventDescription`) and the `{{if .EventError}}` div are
      unchanged.
- [ ] `go test ./internal/server/ -run TestAdminEventsRender` passes (existing
      assertions on `action="/admin/events"` and `Create Event` still hold).
- [ ] Manual: Admin > Events tab loads with the form hidden and a "Create
      Event" button above it. Clicking shows the form below the button and
      relabels the button "Cancel". Clicking again hides the form and relabels
      it "Create Event". Submitting a valid form creates the event (303 to
      /admin?tab=events). Submitting an invalid form shows the error and
      (if auto-show-on-error implemented) the form with pre-filled values.
- [ ] The CSS cache-buster `?v=` is NOT bumped in this card (sibling owns it).
