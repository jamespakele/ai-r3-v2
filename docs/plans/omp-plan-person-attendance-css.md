# Working Plan: Per-Person Monthly Calendar View + Attendance Stats UI (CSS Layer)

## Objective

Add the vanilla-CSS styling layer for the per-participant monthly attendance calendar and attendance-stats UI (Story FR17) in the R3 Intake Go/PocketBase app. The backend (implemented in the sibling worktree `t_aefcfde6`) already renders three templates — `person-attendance` (page shell), `person-attendance-calendar` (stats row + nav + 7-column month grid + legend), and `person-attendance-day` (day-detail modal fragment) — that consume a set of `person-att-*` classes. This task adds the CSS rules for those classes to `r3-intake/internal/assets/public/app.css` and bumps the stylesheet `?v=` cache-buster. **No Go code is written in this task.**

## Constraints

- **Vanilla CSS only.** No framework, no preprocessor, no build step. Append rules to the existing `app.css` (currently 348 lines).
- **Match the design tokens exactly** (already declared in the file header comment):
  - Palette: card `#fffdfa`, border `#e4d9c8`, accent `#b5502e`, ink `#2b2320`, muted `#6b5f52`, input border `#ddd2c4`, error `#b3261e`, page bg `#f7f1e6`, soft green `#3f6b34`, soft red `#8f3a2e`, soft yellow `#8a6a1e`, blue `#2a4d8a`.
  - Type: Public Sans body, Lora serif headings. 14px card radii, 8px input radii.
- **Status colors must match the existing matrix dots** (`.matrix-dot.dot-*`): present `#3f6b34`, absent `#eee` bg / `#d9cbb6` border, excused `#8a6a1e`, walk_in `#2a4d8a`.
- **Reuse existing classes** — do not redefine `.btn`, `.btn-ghost`, `.btn-primary`, `.btn-tiny`, `.btn-danger`, `.empty-state`, `.form-error`, `.field-group`, `.field-label`, `.field-input`, `.textarea`, `.rate-good`, `.rate-low`, `.page-bg`, `.topbar*`, `.brand*`, `.container-admin`, `.section-title`. These already exist and are verified present in `app.css`.
- **Bump the cache-buster** on the stylesheet link from `?v=3` to `?v=4` in the `person-attendance` template's `<head>` (the sibling template currently hardcodes `?v=6`; the merged result must use the bumped value — see Implementation Notes).
- **Verification gates:** `go build ./...`, `go vet ./...`, `go test ./...` must pass; template parse must succeed.

## File Structure

All paths relative to repo root `/srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_cca37bee`.

| File | Change |
|------|--------|
| `r3-intake/internal/assets/public/app.css` | **Modify (append).** Add a `/* Person attendance calendar */` section with the `person-att-*` rules below. No existing rules are changed. |
| `r3-intake/internal/assets/public/index.html` | **Modify.** Bump the `app.css?v=3` → `app.css?v=4` cache-buster on the stylesheet link in the `person-attendance` template `<head>` (line ~1145 in the sibling worktree; the merged file's `person-attendance` define). |

> Note: the sibling worktree's `person-attendance` template currently references `?v=6` (a value that predates this worktree's `?v=3` baseline). The parent epic task merges the two worktrees; this task's contract is: **the merged `person-attendance` template must reference the same cache-buster value that this worktree's `index.html` uses after the bump.** Coordinate the final value with the parent so the merged template and CSS stay in sync.

## Implementation Notes

### 1. Stats row — `.person-att-stats`, `.person-att-stat`
- `.person-att-stats`: `display:flex; flex-wrap:wrap; gap:10px; margin:0 0 16px; align-items:center;`
- `.person-att-stat`: pill badge — `font:600 13px 'Public Sans',sans-serif; padding:6px 12px; border-radius:999px; border:1px solid #e4d9c8; background:#fffdfa; color:#2b2320;`
- The rate stat already carries `.rate-good` / `.rate-low` (existing classes: `bg #eef3ea`/`color #3f6b34` and `bg #fbeeec`/`color #8f3a2e`). Because `.person-att-stat` sets a background, ensure the rate classes win by declaring them **after** `.person-att-stat` in the file, or scope the pill background so the rate tint shows. Simplest: give `.person-att-stat` a neutral `background:#fffdfa` and let `.rate-good`/`.rate-low` (already later in the cascade) override it — verify specificity/order so the green/red tint actually renders.

### 2. Nav — `.person-att-nav`, `.person-att-month`
- `.person-att-nav`: `display:flex; align-items:center; justify-content:space-between; gap:10px; margin:0 0 12px;`
- `.person-att-month`: `font:600 18px 'Lora',serif; color:#2b2320;` (Lora heading per design system). Prev/Next are existing `.btn.btn-ghost`.

### 3. Calendar table — `.person-att-calendar`, `.person-att-cell`, `.person-att-daynum`, `.person-att-status`
- `.person-att-calendar`: mirror the existing matrix table treatment — `border-collapse:collapse; width:100%; font:14px 'Public Sans',sans-serif; background:#fffdfa; border:1px solid #e4d9c8; border-radius:10px; overflow:hidden;` (radius needs `overflow:hidden` on the table or a wrapping element to clip the corners).
- `th`: `padding:8px 10px; border-bottom:1px solid #eee3d2; font:600 12px 'Public Sans',sans-serif; color:#6b5f52; text-align:center;`
- `.person-att-cell`: `padding:8px 10px; border-bottom:1px solid #eee3d2; text-align:center; vertical-align:top; min-height:64px; cursor:pointer;` (cells are clickable to open the day-detail modal).
- `.person-att-daynum`: `font:600 14px 'Public Sans',sans-serif; color:#2b2320;`
- `.person-att-status`: status label inside a cell — `display:inline-block; margin-top:4px; font:600 10px 'Public Sans',sans-serif; padding:2px 8px; border-radius:999px; text-transform:capitalize;`

### 4. Cell state classes
- `.is-other-month`: dim the day number — `.person-att-daynum { color:#c9bda8; }` and reduce cell emphasis (e.g. `opacity:.55` on the cell, or just muted daynum). Keep cells clickable but visually de-emphasized.
- `.is-today`: highlight — give the cell a distinct background/border, e.g. `background:#fdf3e3;` (soft accent tint) and/or a `box-shadow: inset 0 0 0 1px #b5502e;` ring so "today" reads at a glance without clashing with status colors.
- `.has-record`: optional subtle affordance (e.g. `background:#fbf7ef;`) so recorded days stand out from empty days; keep it lighter than `.is-today`.
- Status color coding on the cell (match matrix dots exactly):
  - `.status-present` → label `background:#3f6b34; color:#fffdfa;`
  - `.status-absent` → label `background:#eee; color:#6b5f52; border:1px solid #d9cbb6;`
  - `.status-excused` → label `background:#8a6a1e; color:#fffdfa;`
  - `.status-walk_in` → label `background:#2a4d8a; color:#fffdfa;`
- **Edge case — class collision:** `.status-present`/`.status-absent`/etc. are generic names already used by the intake status badges (`.status-unassigned`, `.status-claimed`, `.status-completed`). The person-attendance statuses are a **different set** (`present/absent/excused/walk_in`) and do not collide with those names, but to be safe scope all status rules under `.person-att-cell.status-*` and `.person-att-legend-item.status-*` so they never leak onto other `.status-*` elements.

### 5. Legend — `.person-att-legend`, `.person-att-legend-item`
- `.person-att-legend`: `display:flex; flex-wrap:wrap; gap:8px; margin:14px 0 0;`
- `.person-att-legend-item`: `display:inline-flex; align-items:center; gap:6px; font:600 12px 'Public Sans',sans-serif; color:#6b5f52;` with a small color swatch. Since the template renders the label text directly inside the item (no separate swatch element), render the swatch via a `::before` pseudo-element: `width:10px; height:10px; border-radius:50%; background:<status color>;` — one rule per `.person-att-legend-item.status-*` using the same four status colors as the cells.

### 6. Day-detail modal — `.person-att-day-detail`, `.person-att-day-actions`
- `.person-att-day-detail`: card treatment — `background:#fffdfa; border:1px solid #e4d9c8; border-radius:14px; padding:18px 20px; margin:16px 0;` (14px card radius per design system). The fragment is swapped into the page via HTMX (`hx-target="#person-attendance-calendar"`), so it renders inline below the calendar rather than as a fixed overlay — style it as a prominent inline card, not a fixed-position modal.
- `.person-att-day-actions`: `display:flex; gap:10px; margin-top:16px; align-items:center;` (Save + Cancel buttons; the Delete form is a separate sibling form — give it `margin-top:12px` so it sits below the actions row).
- Reuse `.field-group`/`.field-label`/`.field-input`/`.textarea`/`.form-error`/`.empty-state`/`.btn*` as-is; no new field styles needed.

### 7. Responsive
- Add a `@media (max-width: 620px)` block (matching the existing breakpoint) that:
  - lets `.person-att-calendar` scroll horizontally (`overflow-x:auto` on a wrapper, or `display:block; overflow-x:auto` on the table) so 7 columns don't crush on narrow screens;
  - stacks `.person-att-nav` (or keeps it but reduces gaps) and lets `.person-att-stats` wrap naturally (already `flex-wrap:wrap`);
  - reduces `.person-att-cell` padding to `6px 6px` and `.person-att-status` font to `9px` on small screens.

### 8. Cache-buster
- Bump `app.css?v=3` → `app.css?v=4` in the `person-attendance` template's `<head>` link. This is the only cache-buster change for this feature; the other templates' links are untouched by this task.

## Verification Criteria

1. **CSS sanity:** `app.css` still parses (no unbalanced braces) — spot-check by appending the new section and running a quick brace-balance check or opening in a browser devtools.
2. **Build gates:** `go build ./...`, `go vet ./...`, `go test ./...` all pass in this worktree (CSS is embedded via `embed`, so a malformed file would still build, but the template parse must succeed).
3. **Template render test (in the sibling worktree, run by the parent after merge):** the existing `person_attendance_test.go` renders `person-attendance`, `person-attendance-calendar`, and `person-attendance-day` and asserts expected strings. Extend/confirm it asserts the presence of the CSS classes the templates emit: `person-att-stats`, `person-att-stat`, `person-att-nav`, `person-att-month`, `person-att-calendar`, `person-att-cell`, `person-att-daynum`, `person-att-status`, `person-att-legend`, `person-att-legend-item`, `person-att-day-detail`, `person-att-day-actions`, plus the state classes `is-other-month`, `is-today`, `has-record`, `status-present`, `status-absent`, `status-excused`, `status-walk_in`.
4. **Cache-buster check:** grep the merged `index.html` to confirm the `person-attendance` template's stylesheet link uses the bumped `?v=` value and that it matches the value in this worktree's `index.html`.
5. **Visual check (manual):** load a person's attendance page, confirm (a) stats pills render with correct rate tint, (b) the 7-column Sun–Sat grid lays out cleanly, (c) today is highlighted, other-month days are dimmed, (d) each status shows its correct color in both cells and legend, (e) clicking a cell opens the day-detail card with working Save/Cancel/Delete, (f) the layout degrades gracefully below 620px.
