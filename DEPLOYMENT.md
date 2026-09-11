Deployment record (supersedes any earlier "deploy pending" note in this file)

Production deployment completed 2026-09-11 via vps-deploy-go from master bd50170
(amd64). Verified on https://r3.aipono.org:

- Migrations 017_remove_claim.go and 018_remove_intake_status.go recorded in
  _migrations and present on disk; intake schema has no status or assigned_to
  columns; counts preserved (60 intake, 108 attendance, 2 events); journal clean.
- Snapshots: pre-deploy /root/r3-predeploy-20260911-145604 (integrity ok,
  restore drill passed) and post-deploy /root/r3-postdeploy-20260911-150417
  (integrity ok); deploy-script backup data-20260911-150228 md5-verified
  identical to the pre-deploy live database.
- Public checks green: admin login + list, case-manager login + attendance +
  cross-user access, anonymous auth guard, anonymous-created records (4)
  publicly resumable per the approved created_by rule.
- Served UI verified: no Status column/badges/filter/Complete button; finish
  form, Save flow, and event status badges preserved.
- Deploy environment notes: tailnet SSH policy requires root (deploy user is
  rejected on hostinger-vps); the fish login shell required the bash -s
  heredoc transport and rsync-to-tar fallback now committed to the
  vps-deploy-go skill (ai-plugins f4bc41b, c062f17).

## Rollback (if needed)

Snapshot artifacts — two independent nets, on-box and off-box:

- Pre-deploy database snapshot (pre-017/018 state): `/root/r3-predeploy-20260911-145604/`
  on the VPS (data.db md5 `9f815526514d5eb19d441e62791f8c26`, auxiliary.db, storage/,
  and the running binary as `r3-intake-binary` sha256 `6d80a955a33d7cd272ec43bab08c9db264bf3c7e7662b37c868f38f0c8a02d8a`).
  Integrity ok; restore drill passed. Off-box copies in
  `/home/pakele/r3-deploy/snapshots/` (pre-deploy-20260911-145604-data.db and
  pre-deploy-r3-intake-binary, hashes verified after transfer).
- Deploy-script backup (byte-identical to the pre-deploy live database):
  `/srv/go-apps/r3-intake/backups/data-20260911-150228/` (data.db md5
  `836e7a575280795796c1990f4d5e5365` — matches the captured pre-deploy live md5).
- Post-deploy snapshot (post-017/018 state): `/root/r3-postdeploy-20260911-150417/`
  (data.db md5 `8e265887aa3e954f9b42a440588e37e9`, integrity ok); off-box copy
  `/home/pakele/r3-deploy/snapshots/post-deploy-20260911-150417-data.db`.

Full rollback to the pre-deploy state (code and data together):

1. `ssh root@hostinger-vps "sudo systemctl stop r3-intake"`
2. Restore the data directory from the deploy backup:
   `sudo rm -rf /srv/go-apps/r3-intake/data && sudo cp -a /srv/go-apps/r3-intake/backups/data-20260911-150228 /srv/go-apps/r3-intake/data && sudo chown -R apps:apps /srv/go-apps/r3-intake/data`
   (or upload the off-box pre-deploy snapshot instead).
3. Restore the pre-deploy binary:
   `sudo cp /root/r3-predeploy-20260911-145604/r3-intake-binary /srv/go-apps/r3-intake/r3-intake && sudo chmod 755 /srv/go-apps/r3-intake/r3-intake && sudo chown apps:apps /srv/go-apps/r3-intake/r3-intake`
4. `sudo systemctl start r3-intake`, then verify: service active, `GET /login` → 200,
   and the restored data.db md5 equals `836e7a575280795796c1990f4d5e5365`.

Important: restore data AND binary together. Restoring only the data under the new
binary would re-run migrations 017/018 (the restored database's `_migrations` lacks
them) and re-apply the removals.