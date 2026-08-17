# R3 Intake Form — server-backed multi-user app

A self-contained PocketBase + Go + htmx + Alpine.js implementation of the R3
(Restore, Reconnect, Revive) intake form used by case managers in Waiʻanae to
build a Personal Profile and Service Plan for participants experiencing
homelessness. The form's fields, sections, option labels, required-field rules,
and visual design are ported from the reference single-page HTML at
`docs/htmx-intake-form/project/R3 Intake Form.dc.html`.

## Stack

- **PocketBase** (embedded as a Go library) — data, auth, file storage. Runs
  internally on `127.0.0.1:8091`, NOT exposed on the public port.
- **Go** — single binary, HTTP server on `:8090`, serves embedded
  `internal/assets/public/`, proxies `/api/*` and `/_/` (PocketBase admin UI)
  ONLY when the `--admin` flag (or `R3_ADMIN=1`) is set.
- **htmx 2.x** — section submits, partial swaps, "saved as you go" indicator.
  htmx owns the persisted state.
- **Alpine.js 3.x** — display-only ephemeral view state (conditional sub-field
  reveals, household row add/remove). Alpine never holds the source of truth.
- **Vanilla CSS** in `public/app.css` — semantic class names, the reference's
  exact palette/typography/radii/print rules. No Tailwind, no CSS framework.

## Folder layout

```
r3-intake/
  cmd/r3-intake/main.go              # entry point, flag parsing, starts PB + Go
  internal/config/config.go          # Config struct incl. Encryption block
  internal/crypto/crypto.go          # Cipher interface + PlainCipher no-op
  internal/server/server.go          # HTTP mux, reverse proxy, asset serving
  internal/server/handlers.go        # form render + section save fragments
  internal/server/admin.go           # /admin dashboard, sites, claim flow
  internal/server/auth.go            # session cookie login/logout for Go UI
  internal/assets/public/index.html  # Go template: the five-section form
  internal/assets/public/app.css     # vanilla CSS ported from the reference
  internal/assets/public/htmx.min.js  # vendored (see Makefile / Run)
  internal/assets/public/alpine.min.js # vendored (see Makefile / Run)
  pocketbase/migrations/001_init.js  # PB schema: intake + sites + users.role
  pocketbase/migrations/002_encryption.go  # re-encrypts sensitive rows when enabled
  pocketbase/pb_data/                # created on first run
  Makefile
  README.md
```

## Robustness (non-negotiable)

The form works with JavaScript disabled. Every conditional sub-field renders
**expanded** server-side; Alpine's `x-show` hides them client-side when the
trigger is "No"/unchecked. With JS off, `x-show` is inert so sub-fields stay
visible and the user fills what applies. htmx degrades to a standard form POST
that returns a full page redirect to the saved form. No `<noscript>`, no
separate no-JS page. `x-cloak` is intentionally NOT used so the no-JS path stays
visible.

## Roles & auth

PocketBase `users` collection has a `role` field (`admin | case_manager`).

- **Admin** — sees all intakes, manages sites, manages users. Custom pages at `/admin`.
- **Case manager** — sees only intakes they created or claimed; can create new;
  can claim an unassigned public intake.
- **Participant self-fill** — anonymous public submit at `/public/intake` (also
  `/`), no login. The record is created with `assigned_to = null` and
  `status = "unassigned"`. A case manager claims it from the admin list, which
  sets `assigned_to = <user id>` and `status = "claimed"`.

All PocketBase data access is performed by the Go server **in-process** via
`pb.Dao()` (PocketBase is embedded as a Go library, not a separate process).
The public-facing PB collections are locked to admin/superuser via API rules;
Go is the policy layer, so the browser never talks to PocketBase directly.
End-user browser sessions are a lightweight signed Go cookie holding the user
id/email/name/role — no PB token is issued to the browser.

## Encryption seam (stub, not active)

`internal/config` carries an `Encryption` block and `internal/crypto` defines a
`Cipher` interface plus a no-op `PlainCipher`. All repository save/load paths
call `cipher.Encrypt/Decrypt` on `SensitiveFields` (ssn, dob, signature data
URLs) — they do not know whether encryption is on. `Encryption.Enabled` defaults
to `false` (plaintext at rest). Switching it on later is a config flip plus
migration `002_encryption.go` (currently a no-op with the re-encrypt TODO).
~60 lines, zero runtime cost now.

## Sensitive data

SSN, DOB, and case manager name are top-level text columns on the `intake`
record, marked sensitive in config. SSN is masked to last-4 in admin list
views. SSN/DOB are never logged. Legacy signature columns
(`participantSigDataUrl`, `casemanagerSigDataUrl`) remain in the schema but
are no longer written by the UI.

## Run

Vendor htmx + Alpine once:

```sh
cd r3-intake
make vendor          # or, manually:
curl -fsSL -o internal/assets/public/htmx.min.js https://unpkg.com/htmx.org@2.0.3/dist/htmx.min.js
curl -fsSL -o internal/assets/public/alpine.min.js https://unpkg.com/alpinejs@3.14.1/dist/cdn.min.js
```

Build and run (default — PB admin UI NOT exposed):

```sh
make build
./r3-intake serve                  # app on http://localhost:8090
```

Dev run (PB admin UI reachable at http://localhost:8090/_/):

```sh
./r3-intake serve --admin
# or: R3_ADMIN=1 ./r3-intake serve
```
### First run — one command is the whole setup

`make run` (or `./r3-intake serve`) builds, runs migrations, creates
`pocketbase/pb_data/`, and idempotently seeds:

- a PocketBase superuser for the `/_/` admin UI — `admin@r3.local` / `r3admin123`
  (override with `R3_PB_ADMIN_EMAIL` / `R3_PB_ADMIN_PASSWORD`)
- an app **admin** user — `admin@r3.local` / `admin123`
  (override `R3_SEED_ADMIN_EMAIL` / `R3_SEED_ADMIN_PASSWORD`)
- a demo **case manager** — `cm@r3.local` / `cm123456`
  (override `R3_SEED_CM_EMAIL` / `R3_SEED_CM_PASSWORD`)

No env vars, restarts, or manual curls required. Change the defaults before
any real deployment.

### Logging in

- **App admin (custom `/admin`):** `http://localhost:8090/login` →
  `admin@r3.local` / `admin123`. Sees all intakes, manages sites + users.
- **Case manager:** `http://localhost:8090/login` → `cm@r3.local` / `cm123456`.
  Lands on `/admin` scoped to intakes they created or claimed.
- **PB admin UI (dev only, `--admin`):** `http://localhost:8090/_/` →
  `admin@r3.local` / `r3admin123`. For schema/migrations inspection only.
- **Anonymous participant self-fill:** `http://localhost:8090/` (or
  `/public/intake`). No login. Submits create an `unassigned` intake.
- **Claiming an unassigned intake:** a signed-in case manager opens `/admin`,
  sees unassigned rows, and clicks **Claim** → POST `/admin/intake/:id/claim`
  sets `assigned_to = <their user id>` and `status = "claimed"`. The intake
  then appears in their list and is editable by them.

## Deploy to VPS (`vps-deploy-go` skill)

The app ships as a single statically linked Go binary with all templates,
CSS, and JS embedded. PocketBase JS migrations are loaded from the
`pocketbase/migrations/` directory at runtime, so that directory must be
copied alongside the binary in production. The `vps-deploy-go` skill handles the
full deploy:
cross-compile, upload over Tailscale SSH, install the systemd unit, render the
Traefik dynamic config, and restart the service.

### Required project files

```
deploy/vps/
  deploy.env      # non-secret metadata: app name, domain, port, build target
  .env            # production secrets (gitignored)
  .env.example    # template for .env
```

`deploy/vps/.env` has already been generated with real random secrets; do **not**
commit it.

### Required local config

Create `~/.config/vps-deploy-go/config` on your dev machine:

```sh
VPS_HOST=hostinger-vps
VPS_USER=deploy
VPS_APPS_DIR=/srv/go-apps
VPS_TRAEFIK_DIR=/opt/traefik/dynamic
VPS_APPS_USER=apps
```

`VPS_APPS_DIR=/srv/go-apps` is the parent folder for all Go apps on the VPS;
each app gets its own subfolder (`/srv/go-apps/r3-intake/`).

### Before first deploy

1. Edit `deploy/vps/deploy.env` and replace `VPS_APP_DOMAIN` with the real
   public domain (e.g. `r3.yourdomain.org`).
2. If you changed the domain, also update the email addresses in
   `deploy/vps/.env` (e.g. `admin@r3.yourdomain.org`).
3. Make sure Tailscale is installed and the VPS is on your tailnet.

### Deploy

Run the skill from the `r3-intake/` project root:

```sh
/skill:vps-deploy-go deploy
```

Or invoke the helper script directly:

```sh
.omp/skills/vps-deploy-go/scripts/deploy.sh
```

For an ARM VPS, pass `arm64`:

```sh
.omp/skills/vps-deploy-go/scripts/deploy.sh arm64
```

The skill will:

1. Check Tailscale connectivity and prompt for the Tailscale auth URL if the
   VPS is not visible on the tailnet.
2. Build a statically linked Go binary for `linux/amd64` (or `linux/arm64`).
3. Upload the binary and `.env` to `/srv/go-apps/r3-intake/`.
4. Create the shared `apps` runtime user if missing.
5. Install `/etc/systemd/system/r3-intake.service` and reload systemd.
6. Install `/opt/traefik/dynamic/r3-intake.yml` and signal Traefik to reload
   (works whether Traefik runs as a systemd service or a Docker container).
7. Restart the app and run a health check against `http://127.0.0.1:8090/`.

The app listens on `127.0.0.1:8090`; Traefik terminates TLS and reverse-proxies
from the public domain. The session cookie is `HttpOnly` + `SameSite=Lax` and
intentionally **not** `Secure`, because behind a TLS-terminating proxy the
internal hop is plain HTTP. Keeping the app bound to localhost and reachable only
through Traefik is what makes that safe.

### Post-deploy ops

Use the skill helpers:

```sh
/skill:vps-deploy-go status
/skill:vps-deploy-go logs
/skill:vps-deploy-go edit-env
```

## Notes / inferences

- **Signatures** are stored as text columns (PNG data URLs), not PB file
  storage — simpler, no storage backend config. Both the canvas-drawn and the
  typed-name fallback are kept.
- **Roles** use a `role` field on the built-in `users` collection rather than a
  separate collection — simpler, fewer joins, PB's `@request.auth.role` works in
  rules.
- **Unassigned → claimed flow** is the only status transition implemented
  (`unassigned` → `claimed` → `completed`). Case managers see only their
  created + claimed records; admins see all.
- **Records event filter is a union** (`handleList` in `internal/server/admin.go`):
  filtering the Records list by an event surfaces intakes whose home event
  (`intake.event`) equals the selected event **OR** that have an attendance record
  for that event (`attendance.intake == intake.id`). All attendance statuses count
  (present/absent/excused/walk_in). The union is built as OR-joined
  `(event='<id>' || id='<id1>' || id='<id2>' || ...)` clauses (PocketBase v0.39 has
  no `in` operator) and composes with the `?status=`/`?q=` filters via ` && `. An
  event with no attendance records falls back to home-event-only matching, so a
  freshly-created event never renders an empty screen. The list no longer shows
  a per-row Event column; the event filter dropdown is the way to scope the
  list to an event.
- **PocketBase is embedded as a Go library** (`github.com/pocketbase/pocketbase`)
  so the app is a single binary plus the `pocketbase/migrations/` directory.
  Go migrations (`002_encryption.go`, `011_encrypt_existing_data.go`, etc.) are
  compiled in; JS migrations are loaded from `pocketbase/migrations/` at runtime.
  PB listens on `127.0.0.1:8091`; Go proxies `/api/*` and `/_/*` to it only
  when `--admin` / `R3_ADMIN=1` is set. The `pocketbase/` folder holds
  `pb_data/` and `migrations/` — set via `R3_PB_ROOT_DIR`.
- **x-cloak omitted** to satisfy the JS-off robustness rule (elements must stay
  visible without Alpine); a brief pre-init flash is accepted over a hidden
  no-JS form.
- **Reference ambiguity:** the reference section 02 is titled "02 — Medical
  History / Health & safety"; section 03 is "03 — Homeless Verification /
  Documentation on file" and contains the documents/housing/income checklists
  (the brief's "Documents, Housing & Income"). We preserve the reference's own
  eyebrow + heading text verbatim. The HMIS checkbox + provider live in section
  03 per the reference (the brief lists them under 02); the reference markup is
  the source of truth and we follow it.
- **Autosave model:** each section is its own htmx form with a "Save section N"
  button; the response swaps that section in place and shows "Saved ✓". The
  first save on a brand-new form does a full navigate to `/intake/{id}` (via
  htmx `HX-Redirect`) so every section form + the finish form learn the new
  record id — subsequent saves swap in place. "Review & Finish" validates the
  13 required fields against the persisted record (sections own their saves, so
  finish never overwrites fields).
- **No-JS household rows:** Alpine `x-for` shows nothing without JS, so a
  server-rendered fallback with real `household_name_N` / `household_rel_N`
  inputs is always in the HTML (hidden via `x-show="false"` when Alpine is on).
  `householdFromForm` prefers the Alpine JSON hidden input; without JS it reads
  the indexed named inputs, so no household data is lost.
- **PocketBase version:** `go.mod` pins `github.com/pocketbase/pocketbase
  v0.39.9` (requires Go 1.25; `go mod tidy` auto-fetches the toolchain via
  `GOTOOLCHAIN=auto`). The jsvm plugin is registered so the `.js` migration
  runs (an embedded PB does not auto-run `.js` migrations without it). v0.39
  removed the `daos` package and the `app.dao()` JS API — all repo access uses
  the embedded `core.App` methods directly (`s.pb.Find*` / `s.pb.Save` /
  `s.pb.Delete`), and the JS migration uses `app.findCollectionByNameOrId` /
  `app.save` / `new Collection({fields:[{type:"text",...}]})`.
- **Build-verified:** the binary compiles clean (`go vet ./...` passes) and was
  smoke-tested end-to-end against a live `./r3-intake serve`: GET / returns
  200 with all six sections; admin + case-manager login (303 → /admin);
  anonymous public section POST creates an `unassigned` intake; htmx section
  POST returns `Hx-Redirect` on first save and an in-place fragment on
  subsequent saves; case-manager claim (303 → /intake/:id, status→claimed);
  `/_/` is 404 without `--admin` and 200 with it; no-JS section POST returns
  303 (standard form redirect). `make vendor` fetches htmx/Alpine first.