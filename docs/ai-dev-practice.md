# AI Development Practice — Solo Dev Conventions

This document locks in the standard practices for solo development with AI agents on the R3 Intake project. Update this file as the practice evolves — it's the single source of truth for how we work.

---

## 1. Source of Truth

**GitHub Issues are the source of truth.** One place for bugs, features, epics, and tweaks.

| Artifact | Path | Role |
|---|---|---|
| **GitHub Issues** | `github.com/jamespakele/ai-r3-v2/issues` | **Source of truth** — epics, bugs, feature requests, tweaks all live here |
| **Epics & Stories** | `docs/planning/epics.md` | Planning artifact — written during BMAD workflow, then transcribed into issues |
| **PRD** | `docs/feature-name-prd.html` | Design reference — problem, goals, data model, mockups |
| **Code** | `r3-intake/internal/...` | The actual implementation |

**Rule:** When a design changes mid-development, update the GitHub Issue first, then update the PRD, then update `epics.md`. The issue is what you read when starting work. The PRD and `epics.md` are references, not trackers.

**Why issues win:**
- Comments, edits, and linked PRs keep history in one place
- Bugs reported by users go into the same system as planned epics
- Checkboxes render progress bars natively
- Closing PRs auto-close issues with `Closes #N`

---

## 2. GitHub Issue Strategy: One Per Epic

**Standard:** Create **one GitHub issue per epic**, not one per story.

**Stories are checkbox task lists inside the epic issue body.** GitHub renders a progress bar (e.g. `3 of 5 ✓`).

**Create all epic issues upfront** when the PRD and `epics.md` are approved. This gives you a complete roadmap visible in the Issues tab.

---

## 3. Refinement Before Coding (Mandatory)

Before starting work on ANY issue, do a refinement pass:

1. Open the issue
2. Review its stories against what you learned from previous epics
3. If the schema changed or the approach shifted, **edit the issue body** to match reality
4. Update `docs/planning/epics.md` if the change affects the spec
5. Only then start coding

This is the standard check that catches stale issues — it happens at implementation time, not creation time.

---

## 4. Branch & PR Strategy

- **One branch per epic, one PR per epic.** Work all of the epic's stories in that branch.
- Name branches `epic/N-short-slug` (e.g. `epic/1-attendance-matrix`).
- The PR body copies the epic's story checklist and links the epic issue with `Closes #N`.
- Since AI can implement fast, it is fine for several epics to be in flight in parallel branches.

---

## 5. AI Agent Working Style

When working with an AI agent on a story:

- **Paste the story's AC block as context**, not the entire PRD (too much context, agent gets lost)
- **One agent session per story** — easier to review, harder to scope-creep
- **Report when ACs are met**, not when "code is written"
- **Make minimal, focused changes** — don't refactor unrelated code
- **Update the issue checkboxes** as stories complete: `- [x] Story 1.3`

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

## 8. GitHub Projects (Optional, Not Currently Used)

**GitHub Projects** is a kanban board that links to Issues. It could give you a visual Todo/In Progress/Done board, but:

- **Projects don't store markdown files** — they track Issues
- **`epics.md` lives in the repo** (`docs/planning/epics.md`) — that's the planning artifact
- **Use Projects if you want:** visual sprint board, milestone tracking, iteration planning
- **Skip Projects if you want:** minimal overhead — Issues + labels are enough

**Current status:** Not using Projects. Issues + labels (`epic`, `attendance`, `priority:*`) give enough structure. Add a Project board later if visual sprint tracking becomes useful.

---

*This document is a living standard. When you find a better way, update it and move on.*