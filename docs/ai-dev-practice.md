# AI Development Practice — Solo Dev Conventions

This document locks in the standard practices for solo development with AI agents on the R3 Intake project. Update this file as the practice evolves — it's the single source of truth for how we work.

---

## 1. Documentation Hierarchy

| Artifact | Path | Created when | Purpose |
|---|---|---|---|
| **PRD** | `docs/feature-name-prd.html` | Feature kickoff | Problem, goals, scope, data model, handler sketches, route map, mockups |
| **Epics & Stories** | `docs/planning/epics.md` | After PRD approved | Breakdown into epics (PRs) and stories (PR checkboxes) |
| **GitHub Issues** | One issue per epic | When work on that epic begins | Work tracker for the AI agent |
| **Code** | `r3-intake/internal/...` | During implementation | The actual feature |

**Rule:** PRD is the source of truth for *what*. Epics/stories are the source of truth for *how to break it down*. Issues are the work queue, not a second source of truth.

If a design changes mid-development, edit the PRD first, then update the story in `epics.md`, then update the issue. One direction of flow.

---

## 2. GitHub Issue Strategy: One Per Epic, Not Per Story

**Standard:** Create **one GitHub issue per epic**, not one per story.

**Why:**
- Solo dev + AI agent = no team board to signal work in progress
- All stories in an epic usually share files (e.g. `attendance.go` + templates + CSS)
- Working them in one branch/PR is the norm, not the exception
- 12 issues for a 4-epic feature is noise — nobody else is watching
- GitHub task lists give progress bars (3 of 5 ✓) without 5 separate issues to close

**Issue body template:**

```markdown
## Epic {N}: {Title}

**Goal:** {one-sentence value statement}

**Stories:**
- [ ] {N}.1 {Story title} (`docs/planning/epics.md#story-{n}1`)
- [ ] {N}.2 {Story title}
- [ ] {N}.3 {Story title}
...

**FRs covered:** FR{x}-FR{y}
**UX-DRs covered:** UX-DR{x}-UX-DR{y}
**PRD reference:** `docs/feature-name-prd.html` §{section}

**Files likely to change:**
- `r3-intake/internal/server/{file}.go`
- `r3-intake/internal/assets/public/{template}.html`
- `r3-intake/internal/assets/public/app.css`
- `r3-intake/pocketbase/migrations/{file}.js`
```

**Labels:** `epic`, feature name (e.g. `attendance`), `priority:high|medium|low`

---

## 3. When to Create Issues

**Create issues in waves, not all upfront:**

1. **Epic 1 now** — the foundation epic, create immediately after PRD + stories are approved
2. **Epic 2 when Epic 1 is merged** — don't pre-create future epics; early epics may reveal design constraints
3. **Epic 3 and 4 likewise** — create as you approach them

**Why:** Pre-creating future epics leads to stale issues when early epics reveal design constraints. The PRD and `epics.md` are the planning docs — issues are the work queue.

---

## 4. AI Agent Working Style

When working with an AI agent (Hermes, Claude Code, Cursor, Codex, etc.) on a story:

**Open the issue, paste the story as context:**
```
Working on Epic 1 from docs/planning/epics.md.
Implementing Story 1.3: Cell Toggle with HTMX Auto-Save.
Here's the AC:
[paste AC block]
```

**The agent should:**
- Read the related sections of the PRD before writing code
- Make minimal, focused changes (don't refactor unrelated code)
- Update the issue checkboxes as stories complete: `- [x] Story 1.3`
- Report when ACs are met, not when "code is written"

**Don't:**
- Paste the entire PRD (too much context, agent gets lost)
- Ask the agent to "improve the codebase" (scope creep)
- Run multiple stories in one agent session (hard to review)

---

## 5. Branch & PR Strategy

**One branch per epic, one PR per epic.**

```bash
git checkout -b epic/1-attendance-matrix
# work through stories 1.1 → 1.5
git commit -m "Story 1.1: attendance migration + topbar tab"
git commit -m "Story 1.2: calendar matrix view + filters"
# ...
git push origin epic/1-attendance-matrix
# PR references issue: "Closes #1"
```

**PR description:** Copy the epic issue body, add a "Stories completed" section, link to the deployed dev environment for visual review.

Since AI can implement fast, it is fine for several epics to be in flight in parallel branches.

---

## 6. When to Skip This Process

Use a quick commit for:
- Bug fixes (single file, < 2 hours work)
- Copy/text changes
- One-line config tweaks

Skip the PRD/stories ceremony. Just do it, commit, move on.

---

## 7. Review Cadence

Solo dev + AI means review is the bottleneck. Set checkpoints:

- **After each epic merge:** Run the full app, walk through the user flow, fix anything janky before starting the next epic
- **After every 3rd epic:** Review the whole feature, update the PRD with learnings
- **Monthly:** Check `docs/issues*.md` for accumulated tech debt; fold high-impact items into a new PRD

---

*This document is a living standard. When you find a better way, update it and move on.*