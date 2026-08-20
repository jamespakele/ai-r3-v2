# Working Plan: Replace intake form autosave with validated Save button

> **For Hermes:** implement task-by-task (bite-sized), verify with `go build ./... && go vet ./... && go test ./...` (rebuild required because `go:embed`).

## Objective

Remove the autosave behavior from the intake form (all sections), add a visible explicit **Save** button per section, split the participant Name field into **First Name** / **Last Name**, and add client-side + server-side inline validation so a section-01 save is blocked (with inline errors) when either name is missing, and any server-side 400 validation errors from `/section/01` render inline. All changes are confined to the embedded template `r3-intake/internal/assets/public/index.html`; no Go server code is touched (the sibling card `t_67747531` owns `handlers.go`/`applySection` and joins `first_name + " " + last_name` into the single `name` column; the parent epic merges).

Acceptance criteria addressed:
1. **No autosave** — remove the `change delay:300ms` trigger on all section forms.
2. **Save button present** — the existing `section-save-btn` becomes visible (drop `noscript-only`) and remains `type="submit"`.
3. **Missing names → inline errors, no submit** — `htmx:beforeRequest` blocks the section-01 POST and shows inline errors.
4. **Names filled → save succeeds** — valid POST proceeds to `/section/01`.
5. **Server-side validation errors display inline** — `htmx:beforeSwap` parses the `{"errors":{...}}` JSON 400 and populates the inline error divs.

## Constraints

- **Files:** only `r3-intake/internal/assets/public/index.html` (and `app.css` only if styling is required — this plan needs no CSS change, so no cache-buster bump).
- **No Go changes:** do not touch `internal/server/*.go` (handlers.go, server.go), `assets.go`, FormState fields, or template funcs. Anything needing a Go change is a **cross-card dependency**, flagged below.
- **No-JS fallback must keep working:** server-rendered forms + native submit → full-page redirect. `first_name`/`last_name` are real named inputs, so a native POST carries them and the sibling backend reads them. No hidden single-`name` input is needed (adding one would create precedence ambiguity with the sibling's first/last→name join).
- **Do not reference non-existent template fields.** `FormState` has no `.FirstName`/`.LastName`. Referencing them in the template would make Go template execution fail. Values must be prefilled by JS from the existing `{{.Name}}` via a data attribute.
- Stack: Go (html/template) + embedded PocketBase, htmx + Alpine.js, vanilla CSS. htmx events used: `htmx:beforeRequest`, `htmx:beforeSwap`. CSRF header auto-injected via existing `htmx:configRequest`; no-JS native forms get the hidden `csrf_token` injected by the existing `scan()` helper (both are already in the template head).
- Do not restructure unrelated sections or refactor.

## File Structure

| Path | Action |
|------|--------|
| `r3-intake/internal/assets/public/index.html` | **Modify** — the only file changed by this card. |
| `r3-intake/internal/assets/public/app.css` | **No change** (verify only). |
| `r3-intake/README.md` | **Flag for follow-up (next sprint)** — line ~278 documents the autosave model; out of this card's allowed file set. |
| `r3-intake/internal/server/server.go` | **Flag as cross-card** — line ~111 comment `// Section autosave`; sibling/Go territory, do not edit here. |

## Implementation Notes

### 1. Remove autosave on all 5 section forms (AC1)

Each section form (section-01 line 114, section-02 line 293, section-03 line 338, section-04 line 391, section-05 line 410) currently has:
`hx-trigger="change delay:300ms, submit"`

Replace with:
`hx-trigger="submit"`

This kills the change-event autosave; the only trigger left is an explicit form submit (the Save button or Enter). Do this for ALL sections so no section silently autosaves.

### 2. Make the Save button visible in every section (AC2)

Each section ends with:
```html
<button type="submit" class="section-save-btn noscript-only" data-no-print>Save section 0N</button>
```
Remove the `noscript-only` class (keep `section-save-btn` + `type="submit"`). With JS + htmx the button triggers the `submit` hx-post; without JS it triggers the native submit → full-page redirect. One button serves both paths — no separate Alpine/hidden button needed. No CSS change (`.section-save-btn` already styled; `data-no-print` preserved).

### 3. Split the Name field into First / Last (section-01 only)

**Current (lines 133–138):** a `grid-full` `.field-group` with a single `name="name"` input bound to `{{.Name}}`.

**Replace with** two half-width `.field-group`s inside the existing `grid-2`:
```html
<div class="field-group {{if .Errors.first_name}}has-error{{end}}" id="fn-group">
  <label class="field-label">First Name <span class="field-required">*</span></label>
  <input type="text" name="first_name" value="" placeholder="First name"
         class="field-input {{if .Errors.first_name}}has-error{{end}}"
         id="fn-input" @input="R3F.clearFieldErr('fn-input')">
  <div class="field-error" id="fn-error">{{if .Errors.first_name}}First name is required.{{end}}</div>
</div>
<div class="field-group {{if .Errors.last_name}}has-error{{end}}" id="ln-group">
  <label class="field-label">Last Name <span class="field-required">*</span></label>
  <input type="text" name="last_name" value="" placeholder="Last name"
         class="field-input {{if .Errors.last_name}}has-error{{end}}"
         id="ln-input" @input="R3F.clearFieldErr('ln-input')"
         {{if .IsAuthed}}hx-get="/search/duplicates" hx-trigger="keyup changed delay:500ms" hx-target="#dup-results" hx-swap="innerHTML"
         hx-vals='js:({"name": (document.getElementById("fn-input").value.trim()+" "+document.getElementById("ln-input").value.trim()).trim()})'{{end}}>
  {{if .Errors.last_name}}<div class="field-error">Last name is required.</div>{{end}}
</div>
{{if .IsAuthed}}<div id="dup-results" class="dup-results grid-full"></div>{{end}}
```
Notes:
- **Inputs start empty.** `.FirstName`/`.LastName` do not exist on `FormState`; referencing them would break template execution. Value preservation on reload is handled by JS from a data attribute (see #4).
- **Duplicate search preserved** on the `last_name` input: `hx-vals` builds the combined `first + " " + last` and sends it as the backend's existing `?name=` param, so `handleDuplicateSearch` behavior is unchanged. `dup-results` moved out of a now-removed `grid-full` wrapper to a `grid-full` div directly in `grid-2`. (If `hx-vals` `js:` syntax proves unsupported in the bundled htmx version, fallback: drop `hx-vals` and search by last name only — acceptable degradation, see Logical Consequences.)

### 4. Prefill first/last from persisted `{{.Name}}` (value preservation)

On section-01's `<form>` (line 112), add a data attribute carrying the persisted full name:
```html
<form id="sec-01" ... data-fullname="{{.Name}}" x-data="...">
```
Add a `window.R3F` helper that splits the full name at the **first space** into first/last, only when both inputs are empty (so it never clobbers user typing):
```js
splitNameIntoFields:function(form){
  if(!form) return;
  var fn=form.querySelector('[name=first_name]'), ln=form.querySelector('[name=last_name]');
  if(!fn||!ln) return;
  if(fn.value.trim()||ln.value.trim()) return;
  var full=(form.getAttribute('data-fullname')||'').trim(); if(!full) return;
  var i=full.indexOf(' ');
  if(i<0){ fn.value=full; } else { fn.value=full.slice(0,i); ln.value=full.slice(i+1); }
}
```
Invoke on `DOMContentLoaded` (loop `#sec-01`) and in `htmx:afterProcessNode` (harmless after swaps). Heuristic documented: "John David Smith" → first "John", last "David Smith"; single-word names put the whole word in first name (empty last → correctly flagged by validation). This is the least-surprise split; a server-side first/last data model is a next-quarter improvement (flagged).

### 5. Client-side validation blocks the section-01 save (AC3)

Add to `window.R3F` (the existing inline `<head>` object at line 13) and register a `htmx:beforeRequest` listener in the `<head>` script (alongside the existing `htmx:configRequest`):

```js
setFieldErr:function(id,msg){
  var inp=document.getElementById(id), group=document.getElementById(id.replace('-input','-group')),
      err=document.getElementById(id.replace('-input','-error'));
  if(inp&&inp.classList)inp.classList.toggle('has-error',!!msg);
  if(group&&group.classList)group.classList.toggle('has-error',!!msg);
  if(err){ err.textContent=msg||''; }
},
clearFieldErr:function(id){ R3F.setFieldErr(id,''); },
validateSection01:function(evt){
  var f=document.getElementById('sec-01'); if(!f) return true;
  var cfg=evt.detail.requestConfig;
  if(!cfg || cfg.verb!=='post' || cfg.path.indexOf('/section/01')<0) return true;
  var fn=document.getElementById('fn-input'), ln=document.getElementById('ln-input');
  var fnErr=!fn.value.trim(), lnErr=!ln.value.trim();
  R3F.setFieldErr('fn-input', fnErr?'First name is required.':'');
  R3F.setFieldErr('ln-input', lnErr?'Last name is required.':'');
  if(fnErr||lnErr){ evt.preventDefault(); if(fnErr&&fn)fn.focus(); return false; }
  return true;
}
document.addEventListener('htmx:beforeRequest', function(e){ R3F.validateSection01(e); });
```
- `evt.preventDefault()` on `htmx:beforeRequest` cancels the htmx request (htmx checks `defaultPrevented`), so **the form does not submit** (AC3). htmx has already prevented the native submit, so there is no full-page fallback.
- Error classes reuse the existing design system: `.field-group.has-error`, `.field-input.has-error`, `.field-error` (accent error color #b3261e) — no new CSS.
- `@input` on each name input calls `clearFieldErr` so errors disappear as the user types.

### 6. Server-side 400 errors render inline (AC5)

`POST /section/01` returns HTTP 400 `Content-Type: application/json` with body `{"errors":{"first_name":"First name is required","last_name":"Last name is required"}}` when names are missing. Because the section form uses `hx-swap="none"`, htmx will not swap anything on error — so intercept `htmx:beforeSwap`:

```js
document.addEventListener('htmx:beforeSwap', function(e){
  var el=e.detail.elt; if(!el || el.id!=='sec-01') return;
  var xhr=e.detail.xhr; if(!xhr || xhr.status!==400) return;
  if((xhr.getResponseHeader('Content-Type')||'').indexOf('application/json')<0) return;
  e.preventDefault();
  var data={}; try{ data=JSON.parse(xhr.responseText); }catch(_){}
  var errs=(data&&data.errors)||{};
  R3F.setFieldErr('fn-input', errs.first_name||'');
  R3F.setFieldErr('ln-input', errs.last_name||'');
});
```
- `beforeSwap` fires for all section forms; the guard `el.id==='sec-01'` scopes it to the name-validated section. (Sections 02–05 have no JSON error path; they keep existing behavior.)
- `preventDefault()` stops htmx from treating the 400 as a swap/redirect (the HX-Redirect first-save path only fires on 200/202, so this is safe).
- Result: server messages populate the same inline error divs (AC5), and the `has-error` classes apply.

### 7. Update header/intro copy that claims autosave

- Line 73: `<div class="topbar-saved">Saves automatically</div>` → `<div class="topbar-saved">Use the Save button to save each section</div>`. Keep the element (CSS exists); text no longer lies.
- Lines 79–81 (`.intro`): replace the sentence `...everything saves automatically as you go.` with `...use the Save button at the bottom of each section to save your progress as you go.`

### 8. Rebuild + verify

Because templates are embedded via `go:embed`, rebuild after editing:
```bash
cd r3-intake && go build ./... && go vet ./... && go test ./...
```
No new CSS, so no `app.css?v=` bump is required.

## Verification Criteria

| AC | How to verify |
|----|---------------|
| 1. No autosave | In browser (JS on), type/change a field in section-01 and wait >300ms — no network request to `/section/01` fires. `grep` the template: no `change delay:` trigger remains on any section form. |
| 2. Save button present | `Save section 01`..`05` buttons are visible with JS enabled (no `noscript-only`). Also visible without JS. |
| 3. Missing names → inline errors, no submit | With JS on, clear first/last, click `Save section 01`. `.field-error` shows "First name is required." under both inputs, `.field-group.has-error`/`.field-input.has-error` apply, and no request reaches the server (Network tab). |
| 4. Names filled → save succeeds | Fill first+last (and any other required fields), click Save. POST `/section/01` returns 202/HX-Redirect; record persists (reload shows names prefilled from `data-fullname` split). |
| 5. Server-side errors inline | Force a server 400 (e.g. send first_name only via a curl/DevTools POST with valid CSRF) — the returned JSON messages render in the inline error divs. Also: with JS on but names empty, remove the client check temporarily OR post names that are only whitespace to trigger the server path. |
| Regression | No-JS: fill both name inputs + required fields, native submit → full-page redirect, values persist. Duplicate search (authed) still fires on last_name and returns combined-name matches. `go build/vet/test` all pass. |

## Logical Consequences

Second-order review (7 steps). Downstream sites, decisions, and horizons:

| Site | Decision | Horizon | Type | Rationale |
|------|----------|---------|------|-----------|
| `index.html` § section-01 form `hx-trigger` (line 114) | Change → `hx-trigger="submit"` | Immediate | Intended | Removes autosave (AC1). |
| `index.html` § section-02/03/04/05 `hx-trigger` (lines 293/338/391/410) | Change → `hx-trigger="submit"` | Immediate | Intended | Autosave is a form-wide behavior; leaving any section autosaving contradicts the "no autosave" goal. |
| `index.html` § all `section-save-btn` (5 buttons) | Change → drop `noscript-only` | Immediate | Intended | The existing submit button IS the htmx/no-JS Save button (AC2); no second button needed. |
| `index.html` § section-01 Name field (lines 133–138) | Change → two `first_name`/`last_name` inputs | Immediate | Intended | Backend contract requires these keys; join on server. |
| `index.html` § duplicate-search `hx-get` on name | Change → move to `last_name` + `hx-vals` combined name | Immediate | Intended | Preserves authed duplicate detection across the split. If `hx-vals js:` is unsupported → fallback: search by `last_name` only (documented, acceptable degradation, next-sprint if needed). |
| `index.html` § `{{.Name}}` references | Change → move to `data-fullname`; inputs start empty | Immediate | Intended | `.FirstName`/`.LastName` don't exist on `FormState`; referencing them would fail template execution. JS split preserves persisted values. |
| `index.html` § topbar `Saves automatically` (line 73) | Change → "Use the Save button..." | Immediate | Intended (fix misleading UI) | Would otherwise lie once autosave is gone. |
| `index.html` § intro "saves automatically as you go" (lines 79–81) | Change → "use the Save button..." | Immediate | Intended (fix misleading UI) | Same reason. |
| `index.html` § section-01 `.Errors.name` error divs | Remove (name key obsolete) | Immediate | Intended | Server now returns `first_name`/`last_name` keys; `name`-keyed render is dead. |
| `index.html` § household add/remove row buttons (`htmx.trigger(...,'submit')`) | Keep | Immediate | Unintended-negative → accepted | They now trigger an explicit save; with empty names this is blocked by validation until names are filled. Acceptable/correct (can't save a household without a participant name). Documented in code comment. |
| `app.css` | Keep (no change) | Immediate | Intended | Reuses existing `.field-error`/`.has-error`/`.section-save-btn`. No cache-buster bump. |
| No-JS + missing names → sibling 400 JSON | Keep (out of scope) | Next sprint | Unintended-negative | Sibling returns JSON even for non-HX requests, so a no-JS user who omits names sees raw JSON. Fix requires a Go change (render full page on non-HX 400) → **cross-card dependency** with t_67747531 / parent epic. |
| `README.md` § Autosave model (line ~278) | Flag → update | Next sprint | Intended (docs must match UI) | Out of this card's allowed file set; a docs follow-up must reflect Save-button + first/last model. |
| `server.go` § `// Section autosave` comment (line 111) | Flag → leave (sibling) | Next sprint | Intended | Go/sibling territory; parent epic can rename comment. Cosmetic. |
| `handlers.go` § `Progress()` / `validateRecord` / `validateState` (use `st.Name`, key `name`) | Keep | Immediate | Intended | Still function: sibling persists `name`; progress bar and finish validation read the single column. No break. |
| First/last data model (server stores single `name` only) | Flag → no change | Next quarter | Unintended-negative | Round-tripping first/last through a joined string and re-splitting is lossy for multi-space/compound names. A proper `first_name`/`last_name` schema (migration + FormState fields + template refs) is the durable fix; out of scope here. |
| **And-then-what chain** | — | — | — | (a) Autosave removed → user must explicitly Save → user navigates away without saving → loses unsaved edits → no dirty-state warning exists. **Add a next-sprint `beforeunload`/dirty-indicator card** (this card won't add it to keep scope tight, but the risk is real). (b) Save now persists whole section → clicking Save in section-01 also writes event/dob/contact/etc. (same as before, unchanged) → fine. (c) First save still HX-Redirects (unchanged handler) → after first save user is navigated to resume URL → subsequent Save swaps none → fine. |

### Pre-mortem flip

> If a user is frustrated six months from now, what went wrong?

Most likely: (1) They navigated away mid-form and silently lost typed-but-unsaved work because there is now no autosave and no unsaved-changes warning — **folded in**: add a "dirty state → `beforeunload`/unsaved-changes prompt" follow-up card (next sprint). (2) Persisted names round-trip incorrectly through the first-space split (e.g. "Mary Beth Jones" or compound last names) — **folded in**: documented split heuristic + next-quarter first/last schema card; the inline validation still lets the user correct. (3) No-JS users hitting the raw JSON 400 — **folded in**: cross-card dependency flagged to the sibling. (4) A stale `README` claiming autosave confuses developers — **folded in**: docs follow-up. None of these block shipping this card; each is tracked.

### Stakeholders

- **End users (participants / case managers):** get explicit save + clearer save UX; gain guard against missing names; at risk of losing unsaved edits (mitigated by follow-up).
- **Case managers (authed):** duplicate search preserved via combined-name `hx-vals`.
- **No-JS users:** form still functions end-to-end with first/last inputs; known gap (raw JSON on 400) delegated cross-card.
- **Sibling/backend card + parent epic:** frontend now matches the first/last contract; no Go conflicts introduced (template-only).
- **Maintainers:** template-only change keeps merge surface minimal; docs + schema debt tracked as follow-ups.
