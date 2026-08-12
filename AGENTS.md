# AGENTS.md

Operational conventions for AI-assisted development of this repository.
AI coding agents (Claude Code, Cursor, Codex, Hermes, etc.) read this file at session start.

## Project: R3 Intake — Attendance Tracking

- **Stack:** Go server + embedded PocketBase, server-rendered Go templates, HTMX + Alpine.js, vanilla CSS
- **Browser never talks to PocketBase directly** — Go is the policy layer (all PB rules `null`)
- **All timestamps in Hawaii Standard Time** (HST / UTC-10, no DST)
- **Design system:** Public Sans + Lora, accent `#b5502e`, card `#fffdfa`, page bg `#f7f1e6`, 14px card radii, 8px input radii

## AI Development Practice (Locked Standard)

These practices are locked in as the standard for AI-driven development in this project.
Do not deviate without explicit user approval. Full details in `docs/ai-dev-practice.md`.

### 1. GitHub Issues Are the Source of Truth

- **GitHub Issues are the source of truth** — one place for bugs, features, epics, and tweaks.
- `docs/planning/epics.md` is a planning artifact, not a living spec. Issues win because comments, linked PRs, and checkboxes keep history in one place.
- **Create ONE issue per EPIC, not per story.** Stories are checkbox task lists inside the epic issue body (GitHub renders a progress bar).
- **Create all epic issues upfront** when the PRD and `epics.md` are approved.
- **Before coding any issue**, do a refinement pass: open the issue, review against what you learned from previous epics, edit the issue body if the approach shifted, then start coding.
- **Close the epic issue** when its task list is fully checked (via `Closes #N` in the PR).

### 2. Branch & PR Strategy

- **One branch per epic, one PR per epic.** Work all of the epic's stories in that branch.
- Name branches `epic/N-short-slug` (e.g. `epic/1-attendance-matrix`).
- The PR body copies the epic's story checklist and links the epic issue with `Closes #N`.
- Since AI can implement fast, it is fine for several epics to be in flight in parallel branches.

### 3. Source of Truth

- `docs/planning/epics.md` is the single source of truth for epics/stories/acceptance criteria.
- GitHub issues are a lightweight mirror for tracking, not a second source of truth. If they diverge, `epics.md` wins.
- The PRD is `docs/attendance-prd.html`; UI mockups are `docs/attendance-mockups.html`.
- If a design changes mid-development, edit the PRD first, then update the story in `epics.md`, then update the issue. One direction of flow.

### 4. Creating Issues (when starting an epic)

Generate the epic issue from `docs/planning/epics.md`. Standard body template:

```markdown
## Epic {N}: {Title}

**Goal:** {one-sentence value statement}

**Stories:**
- [ ] {N}.1 {Story title}
- [ ] {N}.2 {Story title}
...

**FRs covered:** FR{x}-FR{y}
**UX-DRs covered:** UX-DR{x}-UX-DR{y}
**PRD reference:** `docs/attendance-prd.html` §{section}

**Files likely to change:**
- `r3-intake/internal/server/{file}.go`
- `r3-intake/internal/assets/public/{template}.html`
- `r3-intake/internal/assets/public/app.css`
```

Labels: `attendance` + `epic` (+ priority when relevant).

### 5. AI Agent Working Style

When working with an AI agent on a story:

- **Paste the story's AC block as context**, not the entire PRD (too much context, agent gets lost)
- **One agent session per story** — easier to review, harder to scope-creep
- **Report when ACs are met**, not when "code is written"
- **Make minimal, focused changes** — don't refactor unrelated code
- **Update the issue checkboxes** as stories complete: `- [x] Story 1.3`

### 6. When to Skip This Process

Use a quick commit for:
- Bug fixes (single file, < 2 hours work)
- Copy/text changes
- One-line config tweaks

Skip the PRD/stories ceremony. Just do it, commit, move on.

### 7. Review Cadence

- **After each epic merge:** Run the full app, walk through the user flow, fix anything janky
- **After every 3rd epic:** Review the whole feature, update the PRD with learnings
- **Monthly:** Check `docs/issues*.md` for accumulated tech debt

## Existing Code Patterns (follow these)

- **PocketBase v0.39 JS migration API:** `app.findCollectionByNameOrId`, `app.save`, `app.delete` — camelCase, no `app.dao()`
- **Auth:** Session cookie via `currentSession(r)` helper in `server.go`; check `u.Role == "admin"` for admin routes
- **HTMX:** Use `hx-post` + `hx-target` for partial swaps; return raw HTML fragments from handlers
- **Templates:** Add new `{{define "name"}}` blocks to `internal/assets/public/index.html`; reference via `s.tpl.ExecuteTemplate(w, "name", view)`
- **Routes:** Register in `internal/server/server.go` `Mux()` method; auth via `s.requireAuth()` or `s.requireRole("admin", handler)`
- **Helper funcs:** `formatTime(s)` for HST display; `hst` timezone variable already exists; `ssnLast4()`, `fmtPhone()`, `normalizeDob()` in `handlers.go`
- **Encryption:** SSN fields encrypted at rest via `s.cipher` (see `internal/crypto/crypto.go`)
- **Migrations:** JS migrations in `pocketbase/migrations/` (numeric prefix, auto-run on boot); Go migrations in `pocketbase/migrations/` (call from `migrations.go`)

## Important File Locations

| File | Purpose |
|---|---|
| `r3-intake/internal/server/server.go` | Route registration, auth helpers, template funcs |
| `r3-intake/internal/server/handlers.go` | Intake form handlers (sections 01-05), helpers |
| `r3-intake/internal/server/admin.go` | Admin dashboard, sites/users management |
| `r3-intake/internal/server/notes.go` | Per-participant notes with audit trail |
| `r3-intake/internal/assets/public/index.html` | All Go templates (page, admin, notes, etc.) |
| `r3-intake/internal/assets/public/app.css` | All CSS (vanilla, no framework) |
| `r3-intake/pocketbase/migrations/` | Schema migrations (JS + Go) |
| `docs/attendance-prd.html` | PRD for current feature |
| `docs/attendance-mockups.html` | 3 UI approach mockups |
| `docs/planning/epics.md` | Epics and stories (source of truth) |
| `docs/ai-dev-practice.md` | Solo dev conventions (full manual) |

## Current Feature: Attendance Tracking

Active feature being built. See `docs/planning/epics.md` for the epic/story breakdown.

Epic 1 (Daily Attendance Tracking) is the first to implement. It creates 3 new PB collections (`events`, `event_enrollment`, `attendance`) in migration `007_events_attendance.js`, adds a new `attendance.go` handler file, new templates, and CSS. See PRD §06-§09 for data model, migrations, handler sketches, and routes.

## Common Pitfalls

- **PocketBase v0.39 API:** No `app.dao()` — use `app.findCollectionByNameOrId` directly. No `core.NewBaseCollection` in v0.39 — use `new Collection({...})` in JS migrations.
- **Template execution:** The template file is a single embedded HTML file with multiple `{{define}}` blocks. New blocks go at the end, before the closing `{{end}}` of the last block.
- **HTMX with Go templates:** Return raw HTML strings (not JSON) from handlers. Use `s.tpl.ExecuteTemplate(w, "partial-name", data)` for partials.
- **Time:** Always use `time.Now().In(hst)` for "now" and `formatTime()` for display. The server may run in a different timezone.
- **Auth:** The `currentSession(r)` helper returns nil for unauthenticated requests. Always check before using `u.ID` or `u.Role`.
- **Unique constraints:** PocketBase has no native unique constraints. Enforce in Go handler by querying existing records before creating new ones.

## First-Session Checklist

When you first open this repo, do all of the following:

1. Read this file end to end.
2. Read `docs/ai-dev-practice.md` for the full working manual.
3. Read `README.md` at the project root to understand the build and run commands.
4. Confirm the dev server starts: follow the README's "Getting Started" section.
5. Locate the planning artifacts: `docs/planning/epics.md` is the work queue.
6. Locate the PRD: `docs/attendance-prd.html` is the source of truth for the current feature.
7. Do not start a new feature without first asking the user to confirm which epic to pick up next.