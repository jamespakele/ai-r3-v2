# Working Plan: Make Save and New buttons same size

## Objective
In the intake form's bottom action row (`.finish-actions`), the "Save" button uses
`.finish-actions .btn-primary` with `padding: 13px 26px; font-size: 15px`, while the
"New" button is a `.btn-ghost` that inherits the base `.btn` sizing
(`padding: 8px 14px; font: 600 13px`). This makes the two buttons different heights.
Update the CSS so "Save" and "New" render with identical height and padding, scoped
strictly to `.finish-actions` so no other ghost/primary buttons are affected.

## Constraints
- Vanilla CSS only, single file: `r3-intake/internal/assets/public/app.css`.
- No JS/Go/template-structure changes; this is purely a CSS fix.
- Design system: 8px input radii, Public Sans, accent `#b5502e`, card `#fffdfa`.
- Scope to `.finish-actions` only — do not touch base `.btn`, `.btn-ghost`,
  `.btn-primary`, `.btn-danger`, `.btn-tiny`.
- **No global `box-sizing` reset exists.** The only `box-sizing: border-box` rule is
  on `.field-input` (line 127). Buttons `.btn` are **content-box** by default.

## File Structure
- MODIFY: `r3-intake/internal/assets/public/app.css` (line 180 area, `.finish-actions` rules)
- MODIFY: `r3-intake/internal/assets/public/index.html` — bump the cache-buster on the
  `app.css?v=8` link tags to `?v=9` (9 occurrences) so browsers refetch the updated CSS.

## Implementation Notes
The root cause of the height mismatch is twofold: (1) different padding
(`13px 26px` @15px on primary vs `8px 14px` @13px on ghost), and (2) different borders
(primary has `border: none`, ghost has `1px solid #d9cbb6`). Because buttons are
**content-box**, equal padding does NOT guarantee equal height: the ghost's 1px border
adds 2px to its total height (46px vs 44px for the same padding). To make them truly
identical:

1. Extend the existing scoped rule so it covers BOTH buttons and forces equal sizing:
   ```css
   .finish-actions .btn-primary,
   .finish-actions .btn-ghost {
     box-sizing: border-box;
     padding: 13px 26px;
     font-size: 15px;
     line-height: 1;
   }
   ```
2. Keep `border: none` on primary and `1px` on ghost as they are — with
   `box-sizing: border-box` both buttons' total outer height becomes identical
   (padding + content are contained inside the border box). This is the critical
   decision: **without `border-box`, equal padding still leaves a 2px gap.**
3. Keep the large primary button as the baseline (it is the prominent CTA); raise the
   ghost to match. Do NOT shrink the primary down to the ghost's 8px/13px — that would
   shrink the primary Save button and change the intended visual emphasis.
4. Leave `line-height` default on the base `.btn`; set `line-height: 1` inside the
   scoped rule so the larger font-size does not inflate the ghost's line box and cause
   a height drift between the two.
5. `align-items: center` on `.finish-actions` (line 179) already centers both buttons,
   so once heights match they will align visually.

### Verification of acceptance criteria
- Equal height & padding for Save and New.
- No regression to ghost/primary buttons outside `.finish-actions` (header
  `.topbar-actions`, list/detail pages, admin pages all keep their base sizing —
  confirmed untouched since the change is scoped).

## Verification Criteria
- Confirm the only rule changes are the `.finish-actions .btn-primary` block in
  `app.css` (replaced by the combined `.btn-primary, .btn-ghost` selector) and the
  cache-buster bumps in `index.html`.
- Run `go build ./... && go vet ./... && go test ./...` in `r3-intake/` — must pass
  (no Go logic changed, but the build gate is required).
- Grep confirms no other `.finish-actions` button sizing rules remain that could
  override the new block.
- Visual check (or CSS reasoning): Save (borderless, border-box, 13px 26px, 15px) and
  New (1px border, border-box, 13px 26px, 15px) now have identical outer dimensions.

## Logical Consequences
1. **`.finish-actions` in `index.html` (line 105-108)** — DECISION: change (via the CSS
   rule). The Save and New buttons inside this form now match. No markup change needed;
   the existing `class="btn btn-primary"` / `class="btn btn-ghost"` selectors are
   sufficient. RATIONALE: scoping the sizing to `.finish-actions` fixes only this row.
   Horizon: immediate.

2. **Base `.btn` / `.btn-ghost` / `.btn-primary` (app.css 62-68)** — DECISION: keep
   unchanged. The fix is entirely additive/scoped under `.finish-actions`, so every
   other ghost/primary button (header topbar, List/Sign in/Clear, attendance, admin,
   print row on the top form lines 77-81) keeps its 8px/14px/13px sizing. RATIONALE:
   guarantees the "no regression to other ghost/primary buttons" acceptance criterion.
   Horizon: immediate.

3. **Cache-buster `app.css?v=8` in `index.html` (9 link tags)** — DECISION: change to
   `?v=9`. Without bumping the version, browsers serve the cached stylesheet and the
   button fix appears "not to work" for returning users — a classic stale-asset
   regression. RATIONALE: the existing convention bumps the version on every CSS change
   (current version is 8). Horizon: immediate.

4. **Header `.topbar-actions` Save/ghost buttons (index.html ~74-80)** — DECISION:
   keep for now; NOTE as a known sibling issue. The top form's Save (`.btn-primary`) and
   Print / Save PDF (`.btn-ghost`) row is NOT inside `.finish-actions`, so it retains the
   same size mismatch. This is out of scope for this story but is a real visual
   inconsistency. RATIONALE: flag it so a follow-up can standardize topbar buttons too.
   Horizon: next sprint.

5. **`docs/planning/epics.md` story checkbox** — DECISION: keep (update checkbox to
   complete). This is a story within Epic 31 (intake-form replace autosave with button).
   RATIONALE: the plan/implementation closes a story AC; the epic issue checkbox should
   reflect completion when the parent merges. Horizon: immediate (parent's close-out).
