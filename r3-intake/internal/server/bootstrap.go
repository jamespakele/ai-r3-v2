package server

import (
	"log"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// SeedAll idempotently ensures the PocketBase superuser (for /_/) and the two
// app users (admin + demo case manager) exist. It is meant to run from an
// OnServe hook: OnServe fires AFTER RunAllMigrations (so the `role` field that
// 001_init.js adds to `users` already exists) and BEFORE the serve command's
// default handler runs loadInstaller + ListenAndServe. That ordering is what
// closes the installer race — the superuser exists before needInstallerSuperuser
// is checked, so the "create your first superuser" banner never prints — while
// still seeding after migrations so app-user fields are not silently dropped.
func SeedAll(
	app core.App,
	superuserEmail, superuserPassword string,
	adminEmail, adminPassword, adminName string,
	cmEmail, cmPassword, cmName string,
) error {
	return retry(10, 500*time.Millisecond, func() error {
		if err := upsertSuperuser(app, superuserEmail, superuserPassword); err != nil {
			return err
		}
		if err := upsertAppUser(app, adminEmail, adminPassword, "admin", adminName); err != nil {
			return err
		}
		return upsertAppUser(app, cmEmail, cmPassword, "case_manager", cmName)
	})
}

// upsertSuperuser creates or finds the _superusers record for /_/ login.
func upsertSuperuser(app core.App, email, password string) error {
	col, err := app.FindCollectionByNameOrId("_superusers")
	if err != nil {
		log.Printf("seed superuser: collection lookup failed: %v", err)
		return err
	}
	rec, _ := app.FindFirstRecordByData(col.Id, "email", email)
	if rec != nil {
		return nil
	}
	rec = core.NewRecord(col)
	rec.SetEmail(email)
	rec.SetPassword(password)
	if err := app.Save(rec); err != nil {
		log.Printf("seed superuser: save failed: %v", err)
		return err
	}
	log.Printf("seed superuser: created %s", email)
	return nil
}

// upsertAppUser creates the app user if missing (does not reset an existing
// user's password — env overrides only seed new records).
func upsertAppUser(app core.App, email, password, role, name string) error {
	col, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	rec, _ := app.FindFirstRecordByData(col.Id, "email", email)
	if rec != nil {
		return nil
	}
	rec = core.NewRecord(col)
	rec.SetEmail(email)
	rec.Set("name", name)
	rec.Set("role", role)
	rec.SetPassword(password)
	if err := app.Save(rec); err != nil {
		log.Printf("seed app user %s: save failed: %v", email, err)
		return err
	}
	log.Printf("seed app user: created %s (%s)", email, role)
	return nil
}

// retry calls fn up to n times, sleeping d between attempts, returning the
// last error.
func retry(n int, d time.Duration, fn func() error) error {
	var err error
	for i := 0; i < n; i++ {
		if err = fn(); err == nil {
			return nil
		}
		time.Sleep(d)
	}
	return err
}
