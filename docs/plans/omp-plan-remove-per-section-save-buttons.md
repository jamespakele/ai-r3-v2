# Working Plan — Remove per-section Save buttons from the intake form

**Status:** Draft (planning-only; no source edits)
**Scope:** `r3-intake/internal/assets/public/index.html` + `r3-intake/internal/assets/public/app.css`
**Story:** Remove the 5 redundant `Save section 0N` submit buttons; make the single top-level Save the one path for saving all sections.

---

## 0. Corrected premise (READ FIRST)

The literal task says: *"Keep each form's hx-post and hx-trigger='submit' attributes intact so the single top-level Save button still triggers section saves."*

**This premise is FALSE as written.** The two top-level Save buttons (index.html L77 in the topbar and L107 in `#finish-form`) currently trigger **only section 01**:

```html
<button type="button" onclick="htmx.trigger(document.getElementById('sec-01'),'submit')" class="btn btn-primary">Save</button>
```

There is **no JS anywhere that submits `sec-02`..`sec-05`**. The five per-section buttons are today the **only** way sections 02–05 are persisted. If the plan removed the five buttons and did nothing else, sections 02–05 would become **impossible to save** — a silent data-loss regression worse than the redundancy the story targets.

Therefore the plan is not a pure deletion. It is a **refactor**: remove the five redundant buttons AND upgrade the top-level Save to submit all five sections, while handling the new-form duplicate-record hazard (documented below). The Go handlers, routes, and CSRF plumbing are unchanged.

---

## 1. Overview

- **Goal:** a single, reliable "Save" that persists every section, no per-section buttons.
- **Constraint:** all changes minimal and confined to `index.html` (template + inline JS) and `app.css` (one dead-CSS removal + cache-buster bump). No Go changes, no migration, no route changes.
- **Non-goals:** do not touch `handleSection`/`applySection`/`getOrCreateIntake`; do not change the `hx-post`/`hx-trigger="submit"` contract on any form; do not alter the finish/cancel flows.

---

## 2. Files to change

| File | Change |
|------|--------|
| `index.html` | Remove 5 `section-save-btn` buttons (L307/352/405/424/443). Upgrade both top-level Save buttons (L77, L107) to submit all 5 sections. Add a small inline JS "save-all" that sequences the section posts and patches the record `id` across all forms after the first save on a brand-new form. Fix the now-misleading help text (L88, L95–96). |
| `app.css` | Remove dead `.section-save-btn` / `.section-save-btn:hover` rules (L84–88). |
| `index.html` (all `app.css?v=` links) | Bump cache-buster `?v=8` → `?v=9` at every occurrence (9 total: L9, 454, 529, 648, 927, 1039, 1130, 1284, 1352). |

---

## 3. Detailed implementation

### 3.1 Remove the 5 per-section submit buttons
Delete exactly these five lines (one at the bottom of each section form, immediately before its `</form>`):
- L307 `sec-01`
- L352 `sec-02`
- L405 `sec-03`
- L424 `sec-04`
- L443 `sec-05`

```html
-  <button type="submit" class="section-save-btn" data-no-print>Save section 01</button>
+  (removed)
```
No other markup in the form bodies changes. The forms keep `hx-post="/section/0X" hx-target="#sec-0X" hx-swap="none" hx-trigger="submit"` and their hidden `id` inputs. There is no leftover `</button>`/`<button>` orphan — each removed line is self-contained.

### 3.2 Upgrade the two top-level Save buttons to submit all sections
Replace both `onclick` handlers (L77 and L107) with a call to a shared save-all function:

```html
<button type="button" onclick="R3F.saveAll()" class="btn btn-primary">Save</button>
```

### 3.3 Add the `R3F.saveAll` inline function
Append to the existing `window.R3F = {...}` script (index.html L13). It must:

1. Read the current record `id` from each section form's hidden `input[name=id]`.
2. Submit section 01 first (`htmx.trigger(sec01,'submit')`).
3. On the first save of a **brand-new** record, `handleSection` returns an `HX-Redirect` to `/intake/{id}` (or `/public/intake?id={id}` for non-authed). The `htmx:beforeSwap` handler must:
   - read `xhr.getResponseHeader('HX-Redirect')`,
   - extract the new `id` from that URL,
   - write it into **every** `input[name=id]` across all five forms **and** `#finish-form`,
   - `e.preventDefault()` so the page does not reload and discard the remaining in-flight section saves,
   - then continue submitting `sec-02`..`sec-05` sequentially.
4. If the record already exists (all ids populated), simply fire `sec-01`..`sec-05` sequentially; no id-patching needed.

> **Rationale for sequencing:** the codebase's own design (see `handleSection`'s comment and commit `e553265`) warns that on a brand-new form every section form carries an empty `id`, so if the other sections posted before the first record existed they would each call `getOrCreateIntake` and **create duplicate records**. Sequencing + id-patching (step 3.3) prevents this. `hx-swap="none"` means nothing in the DOM is replaced, so we patch ids explicitly.

**Recommended `htmx:beforeSwap` guard** (mirror existing L19 pattern): only act when `e.detail.elt.id` is a section form and `xhr.status` indicates a first-save redirect.

### 3.4 Fix misleading help text
- L88 `topbar-saved`: `Use the Save button to save each section` → `Use Save to save your progress` (or similar that no longer implies per-section buttons).
- L95–96 `intro`: replace the sentence *"use the Save button at the bottom of each section to save your progress as you go"* with wording reflecting the single top-level Save (e.g. *"use the Save button to save your progress as you go"*).

### 3.5 Remove dead CSS and bump cache-buster
- `app.css` L84–88: delete `.section-save-btn { ... }` and `.section-save-btn:hover { ... }`.
- Bump every `app.css?v=8` → `?v=9` (9 links in `index.html`) so clients fetch the pruned stylesheet and the new markup.

---

## 4. Verification

1. `grep -n "Save section" index.html` → no matches.
2. `grep -n "section-save-btn" index.html app.css` → no matches.
3. `grep -n 'id="sec-0' index.html` → 5 forms; each still shows `hx-post` + `hx-trigger="submit"`.
4. `grep -n "R3F.saveAll" index.html` → present; both top-level Save buttons call it.
5. `grep -n "app.css?v=" index.html` → all `?v=9`.
6. `go build ./...` and `go test ./...` (server) pass — the handler/route surface is untouched, so `intake_save_validation_test.go` and friends remain green (they exercise `/section/{n}` posts directly and never assert on the removed button text).
7. **Manual / browser smoke test:**
   - New (empty-id) form: fill section 01 + section 02, click top Save → exactly ONE record created (verify via List/Records), sections 01 and 02 both persisted, page does NOT hard-reload mid-save.
   - Existing record: edit section 05 only, click Save → only the intended record updated, no duplicate.
   - No-JS path: sections are JS/htmx-only (the forms have no `action`), so this is unchanged from today; per-section native submit already did not work without JS. Flagged, not a regression.

---

## 5. Logical Consequences (second-order review)

| Site | Decision | Horizon | Type | Notes |
|------|----------|---------|------|-------|
| Top-level Save buttons L77, L107 | **Change** — switch to `R3F.saveAll()` | Immediate | Intended | Without this, sec-02..05 become unsavable (premise fix). |
| Inline JS `window.R3F` L13 | **Change** — add `saveAll` + id-patching `beforeSwap` | Immediate | Intended | Needed to fire 5 posts and prevent duplicates on new forms. |
| `htmx:beforeSwap` L19 block | **Change** — extend to patch `id` from `HX-Redirect` | Immediate | Intended | Reuses existing pattern; do not break its sec-01 error-handling branch. |
| 5 per-section buttons L307/352/405/424/443 | **Remove** | Immediate | Intended | The story's literal ask. |
| `sec-01`..`sec-05` `hx-post`/`hx-trigger` attrs | **Keep** | Immediate | Intended | AC2; unchanged. |
| `app.css` `.section-save-btn` (L84–88) | **Remove** | Immediate | Unintended-negative (dead CSS) | Only usage was the 5 buttons. |
| `app.css?v=8` links (9×) | **Change** → `?v=9` | Immediate | Unintended-negative | CSS won't reload otherwise; stale rule harmless but dead code. |
| Help text L88, L95–96 | **Change** | Immediate | Unintended-negative (misleading) | "…at the bottom of each section" lies once buttons are gone. |
| `handleSection`/`applySection`/`getOrCreateIntake` | **Keep** | Immediate | Intended | No Go changes; `/section/{n}` routes unchanged. |
| `#finish-form` hidden `id` | **Keep** (+ patched by save-all) | Immediate | Intended | Must carry the record id for finish; save-all keeps it in sync. |
| `.section-cancel` button (L133) | **Keep** | Immediate | Intended | Attached to `#cancel-form`, not a section save; unrelated to the 5 removed buttons. |
| Household Remove/Add buttons (L205/208) | **Keep** | Immediate | Intended | They submit `sec-01` for household persistence; unaffected. |
| `intake_save_validation_test.go` | **Keep** | Immediate | Intended | Tests `/section/{n}` handler posts directly; no assertion on button text. |
| docs/plans/omp-plan-replace-intake-autosave-with-save-button.md | **Keep (historical)** | Next quarter | Unintended-negative (stale narrative) | Past design doc describing the buttons; it is a historical plan record, not live docs. Optionally annotate with a "superseded" note. |
| docs/plans/omp-plan-intake-save-validation.md | **Keep (historical)** | Next quarter | Unintended-negative | Same — historical, no live dependency. |

---

## 6. Consequence mapping ("and then what?")

- Remove a button → **1st order:** fewer redundant controls. **2nd order:** sections 02–05 lose their only save trigger → must upgrade top Save. **3rd order:** top Save now fires 5 posts → on a new form that would create duplicate records → must sequence + patch `id` (3.3).
- Upgrade top Save → **1st order:** single Save persists all. **2nd order:** new-form first-save returns `HX-Redirect` → must intercept to avoid reload mid-save. **3rd order:** id patch keeps `#finish-form` correct so finishing works.
- Remove `.section-save-btn` CSS → **1st order:** dead CSS gone. **2nd order:** cache-buster must bump or clients keep stale stylesheet. **3rd order:** if left stale, cosmetic-only; still do it for hygiene.

---

## 7. Intended vs. unintended consequences / stakeholders

**Intended:** simpler, less redundant UI; a single Save is the obvious action; fewer duplicate-looking controls for case managers and public self-service users.

**Positive to amplify:** one clear Save action reduces mis-clicks (users were clicking "Save section 01" then scrolling to re-save others); the pattern matches modern form UX. Amplify by keeping the label just "Save" and the topbar + bottom placements.

**Negative to mitigate:**
- Silent data loss for sec-02..05 if Save isn't upgraded (the headline risk — mitigated by 3.2/3.3).
- Duplicate records on first save (mitigated by 3.3 sequencing/id-patch; verify in step 6/7).
- Mid-save hard reload discarding work (mitigated by `preventDefault` on the redirect).
- Stale help text and dead CSS (mitigated by 3.4/3.5).

**Stakeholders:** case managers (primary), public intake self-service users, the project's own test suite (must stay green), future maintainers (dead CSS/copy removed).

---

## 8. Pre-mortem flip

> "Six months later a user is frustrated — what went wrong?"

The most likely complaint: **"I filled out sections 2–5, hit Save, and nothing was saved — I lost everything."** That is exactly what happens if the five buttons are deleted without upgrading the top-level Save (the literal request, as-is). The plan folds this in by making the top Save the real all-sections save (3.2/3.3). Second complaint: **"My form saved twice / created a duplicate."** — addressed by the new-form sequencing + id-patch (3.3) and verified in step 6/7. Third: **"The help text still told me to use a button at the bottom of each section that no longer exists."** — addressed by 3.4.

If the team decides the full save-all refactor (3.2/3.3) is too large for this card, the correct alternative is to **not ship the removal at all** (or to split into a separate card) rather than ship the literal deletion that breaks sec-02..05. Do not ship the naive version.

---

## 9. Out of scope / future

- **Next sprint:** optional UX — per-section inline "saved ✓" feedback after each section posts (uses existing `hx-swap`/`alert` styling).
- **Next quarter:** revisit the no-JS fallback for sections (forms currently have no `action`; only JS/htmx submits them). If accessibility hardening is wanted, add `action="/section/0X" method="post"` + a noscript submit path — separate card, Go handler already supports non-HX posts.
- **Next quarter:** annotate the two historical `docs/plans/omp-plan-*` files as superseded to avoid future confusion.
