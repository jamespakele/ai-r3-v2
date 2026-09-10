# Plan: Remove the "Claim" Feature (keep the Case Manager field)

Status: ready to implement.
Source of user direction: R3 staff conversation (Kika, check-in person, providers), 2026-09-09 — "Nix the claiming restrictiveness," keep the case manager portion.

## 1. What was asked

- Remove claim-based access restriction: any signed-in user can open and work on any intake record regardless of who claimed it.
- Remove the Claim action and claim workflow.
- Stop auto-claiming newly created records.
- Keep the Case manager form field (`casemanagerName`) — Kika: "we can keep that it helps."
- Nothing else. No new restrictions, no new behavior beyond the removal.

## 2. Where claim lives in the code (audit)

Access gates (the restrictiveness):

| Location | Current behavior |
|---|---|
| `internal/server/handlers.go:130-138` — `canAccessIntake` | Central gate: admin OR creator (`created_by`) OR `assigned_to`; everyone else bounced to login. Used by section save (:434), intake view (:499), finish (:540), cancel (:547) |
| `internal/server/attendance.go:364-370` — `loadMatrixRows` roster | Case manager roster filtered to `assigned_to='%s'` |
| `internal/server/attendance.go:234-253` — handleStats | Renders via `loadMatrixRows` → inherits the same roster filter (no separate edit needed) |
| `internal/server/attendance.go:619-624` — toggle | Case manager 403 unless `assigned_to == u.ID` |
| `internal/server/attendance.go:275-338` — `resolveSite` | Case manager's default location derived from assigned intakes (reads `assigned_to` — claim machinery) |
| `internal/server/person_attendance.go:110-113` | Case manager 403 on per-person attendance unless assigned |
| `internal/server/admin.go:374` — adminComplete | Blocked unless admin / creator / assignee |
| `internal/server/handlers.go:402-406` — handlePublicIntake | Public resume blocked when `created_by != ""` OR `status == "claimed"` (second clause kept — legacy claimed rows must stay non-public) |

Claim workflow + UI:

| Location | Current behavior |
|---|---|
| `handlers.go:557-576` — getOrCreateIntake | Authed create auto-sets `assigned_to=user`, `status="claimed"` |
| `admin.go:236-237` + `admin.go:347-364` | `POST /admin/intake/{id}/claim` route + handler |
| `index.html:618` | Claim button in list rows |
| `index.html:605,613` + `admin.go:24,167-179` | "Assigned" column — exists only to display claim state |
| `admin.go:98-101` | `?status=claimed` filter branch (kept — legacy rows) |

Schema: `001_init.js:149-150` defines `status` enum (includes `claimed`) and `assigned_to`. Left untouched — legacy rows keep their values.

## 3. Changes

**handlers.go**
- `canAccessIntake` (:130-138): any authenticated user (`u != nil`). This is the fix — every view/save/finish/cancel path opens up.
- `handlePublicIntake` (:402-406): unchanged — both clauses stay. The `status == "claimed"` clause is kept for legacy compatibility: the old Claim button set `status=claimed` WITHOUT setting `created_by`, so anonymous-created-then-claimed records exist and must remain non-public. Public-resume behavior is unchanged by this work.
- `getOrCreateIntake` (:570-574): keep `created_by=user.ID`; delete the `assigned_to` + `status="claimed"` lines (record keeps `newIntakeRecord`'s unassigned default).

**admin.go**
- Delete `adminClaim` (:347-364) and its dispatch case (:236-237) → `POST /admin/intake/{id}/claim` 404s.
- `adminComplete` (:374): delete the owner check (claim gate) — any signed-in user completes.
- `handleList`: unchanged (status filter still accepts `claimed` so legacy rows stay findable).
- Remove the `AssignedName`/`AssignedToID` plumbing (:24,167-179) with the column.

**attendance.go**
- `loadMatrixRows` (:364-370): roster filter → `1=1` for all roles (fixes matrix and stats together).
- `resolveSite` (:275-338): case_manager takes the same path as admin — the branch reads `assigned_to`, which is claim machinery.
- Toggle (:618-629): delete the case_manager branch; keep the siteID-from-event fallback (:630-634) for everyone.

**person_attendance.go**
- Delete the 403 gate (:110-113).

**index.html**
- Delete the Claim button (:618).
- Delete the Assigned column + header (:605, :613) — display of the removed feature.
- Keep the Claimed status-filter option (:592) and status badge: legacy rows still carry `status='claimed'` and must remain visible and filterable.

**app.css** — unchanged (`.status-claimed` badge kept for legacy rows).

**Schema / MCP** — unchanged. No migration, no data changes, legacy values stay queryable.

## 4. Explicitly untouched

- `created_by` behavior everywhere.
- Public intake form and resume rules — unchanged. The `claimed` public-resume clause stays: the old Claim button set status without created_by, so legacy claimed rows must remain non-public.
- `casemanagerName` field + `ensureCaseManager` + the form section (Kika said keep it).
- Delete/bulk-delete (admin-only), admin settings (admin-only), notes behavior.
- All event/site/date scoping filters.
- `intake.status` enum, `assigned_to` field, every existing row.
- MCP status enums and counters.

## 5. Tests

Update the ones encoding claim behavior:
- `person_attendance_integration_test.go`: forbidden → 200.
- `attendance_roster_integration_test.go`: case manager roster assertion → all participants appear (the `iAssignedCM` fixture stays; it just no longer grants exclusivity).

New focused coverage:
1. Case manager opens + saves a section on an intake created by another user.
2. `POST /admin/intake/{id}/claim` → 404.
3. List page renders no Claim button, no Assigned column.
4. New authed-created record: `status` not `claimed`, no assignment, `created_by` set.
5. Public-resume rule pinned: anon-created unassigned → 200; legacy claimed → login; staff-created → login.

## 6. Verification

1. `make build && go vet ./... && go test ./...` — green (templates are embedded; rebuild mandatory).
2. Review server on `:8190` with a copy of the real test data: legacy claimed records fully accessible to any staff login; cross-user edit; attendance toggle on an unassigned participant; stats match matrix.
3. Deploy per `vps-deploy-go`; verify live on r3.aipono.org.