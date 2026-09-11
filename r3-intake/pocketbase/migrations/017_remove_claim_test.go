package migrations

import (
	"path/filepath"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/jsvm"
)

// TestRemoveClaimMigration exercises the up/down round-trip of the
// claim-removal migration (017) against an in-process PocketBase whose
// migration chain is already fully applied by RunAllMigrations.
//
// The schema currently has NO "claimed" status enum value and NO
// intake.assigned_to field. The test downgrades to re-introduce them,
// seeds legacy-shaped rows, runs upRemoveClaim, and asserts the data
// rewrite, enum/field removal, idempotent re-up, and reverse-down round-trip.
func TestRemoveClaimMigration(t *testing.T) {
	migrationsDir, err := filepath.Abs(filepath.Join("..", "..", "pocketbase", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	pb := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir:  dataDir + "/pb_data",
		HideStartBanner: true,
	})
	jsvm.MustRegister(pb, jsvm.Config{MigrationsDir: migrationsDir})
	Register(pb)
	if err := pb.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	if err := pb.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pb.ResetBootstrapState() })

	app := pb // pocketbase.PocketBase implements core.App

	// Migration 018 (remove intake status) is now in the chain, so
	// RunAllMigrations has removed intake.status. Restore it first (down018
	// re-adds status with the exact post-017 values) so the 017 round-trip
	// below exercises against the pre-018 schema it was written for.
	if err := downRemoveIntakeStatus(app); err != nil {
		t.Fatalf("downRemoveIntakeStatus: %v", err)
	}

	// Local record-creation helpers mirroring the server integration-test
	// pattern (rec + save).
	rec := func(name string) *core.Record {
		col, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatalf("collection %s: %v", name, err)
		}
		return core.NewRecord(col)
	}
	save := func(label string, r *core.Record) string {
		t.Helper()
		if err := app.Save(r); err != nil {
			t.Fatalf("save %s: %v", label, err)
		}
		return r.Id
	}

	findIntakeCol := func() *core.Collection {
		col, err := app.FindCollectionByNameOrId("intake")
		if err != nil {
			t.Fatalf("find intake collection: %v", err)
		}
		return col
	}
	statusValues := func(col *core.Collection) []string {
		f := col.Fields.GetByName("status")
		if f == nil {
			t.Fatal("intake missing status field")
		}
		sf, ok := f.(*core.SelectField)
		if !ok {
			t.Fatalf("intake.status is %T, want *core.SelectField", f)
		}
		return sf.Values
	}
	equalStringSlices := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	// Baseline: 017 already applied — no claimed enum, no assigned_to field.
	col := findIntakeCol()
	if col.Fields.GetByName("assigned_to") != nil {
		t.Fatal("baseline: intake still has assigned_to field (migration 017 not applied)")
	}
	if got := statusValues(col); !equalStringSlices(got, []string{"unassigned", "completed"}) {
		t.Fatalf("baseline: status values = %v, want [unassigned completed]", got)
	}

	// A site + event the legacy intakes will reference (event.site is required).
	site := save("site", func() *core.Record {
		r := rec("sites")
		r.Set("name", "Test Site")
		r.Set("active", true)
		return r
	}())
	ev := save("event", func() *core.Record {
		r := rec("events")
		r.Set("site", site)
		r.Set("name", "Morning Program")
		r.Set("start_date", "2026-08-01")
		r.Set("end_date", "2026-08-31")
		r.Set("status", "active")
		return r
	}())
	// Real user record that assigned_to will (briefly) reference.
	user := save("user", func() *core.Record {
		r := rec("users")
		r.SetEmail("mig@example.com")
		r.SetPassword("mig-password")
		r.Set("name", "Mig User")
		r.Set("role", "admin")
		return r
	}())

	// Down restores the claimed enum and the assigned_to relation field.
	if err := downRemoveClaim(app); err != nil {
		t.Fatalf("downRemoveClaim: %v", err)
	}
	col = findIntakeCol()
	if col.Fields.GetByName("assigned_to") == nil {
		t.Fatal("down did not restore assigned_to field")
	}
	if got := statusValues(col); !contains(got, "claimed") {
		t.Fatalf("down did not restore claimed enum: %v", got)
	}

	// Seed legacy-shaped intakes: status='claimed', name + event set, and at
	// least two carry assigned_to (one row has it empty). These would be
	// invalid after up drops claimed/assigned_to, proving the rewrite must run
	// first.
	i1 := save("i1", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Alice")
		r.Set("event", ev)
		r.Set("status", "claimed")
		r.Set("assigned_to", user)
		return r
	}())
	i2 := save("i2", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Bob")
		r.Set("event", ev)
		r.Set("status", "claimed")
		r.Set("assigned_to", user)
		return r
	}())
	i3 := save("i3", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Carol")
		r.Set("event", ev)
		r.Set("status", "claimed")
		return r
	}())

	// Up rewrites every claimed -> unassigned, drops claimed, removes field.
	if err := upRemoveClaim(app); err != nil {
		t.Fatalf("upRemoveClaim: %v", err)
	}
	col = findIntakeCol()

	// No intake row may retain the dropped status value.
	claimedRecs, err := app.FindRecordsByFilter(col.Id, "status='claimed'", "", 100000, 0)
	if err != nil {
		t.Fatalf("FindRecordsByFilter claimed: %v", err)
	}
	if len(claimedRecs) != 0 {
		t.Fatalf("claimed rows = %d, want 0 after up rewrite", len(claimedRecs))
	}

	// Positive proof the rewrite ran: every seeded intake is now unassigned.
	allRecs, err := app.FindRecordsByFilter(col.Id, "1=1", "", 100000, 0)
	if err != nil {
		t.Fatalf("FindRecordsByFilter all: %v", err)
	}
	wantNames := map[string]struct{}{"Alice": {}, "Bob": {}, "Carol": {}}
	if len(allRecs) != 3 {
		t.Fatalf("intake records = %d, want 3", len(allRecs))
	}
	for _, r := range allRecs {
		if got := r.GetString("status"); got != "unassigned" {
			t.Errorf("intake %q status = %q, want %q", r.Id, got, "unassigned")
		}
		delete(wantNames, r.GetString("name"))
	}
	if len(wantNames) != 0 {
		t.Fatalf("missing reassigned intakes: %v", wantNames)
	}

	// assigned_to field must be gone and enum exactly [unassigned completed].
	if col.Fields.GetByName("assigned_to") != nil {
		t.Fatal("intake still has assigned_to field after up")
	}
	if got := statusValues(col); !equalStringSlices(got, []string{"unassigned", "completed"}) {
		t.Fatalf("status values after up = %v, want [unassigned completed]", got)
	}

	// Idempotency: a second up must be a no-op and error-free.
	if err := upRemoveClaim(app); err != nil {
		t.Fatalf("upRemoveClaim idempotent: %v", err)
	}
	col = findIntakeCol()
	if col.Fields.GetByName("assigned_to") != nil {
		t.Fatal("assigned_to re-appeared on idempotent up")
	}
	claimedRecs, err = app.FindRecordsByFilter(col.Id, "status='claimed'", "", 100000, 0)
	if err != nil {
		t.Fatalf("FindRecordsByFilter claimed (idempotent): %v", err)
	}
	if len(claimedRecs) != 0 {
		t.Fatalf("claimed rows = %d, want 0 after idempotent up", len(claimedRecs))
	}

	// Round-trip: down restores claimed + assigned_to after up has run.
	if err := downRemoveClaim(app); err != nil {
		t.Fatalf("downRemoveClaim (round-trip): %v", err)
	}
	col = findIntakeCol()
	if col.Fields.GetByName("assigned_to") == nil {
		t.Fatal("assigned_to not restored by round-trip down")
	}
	if got := statusValues(col); !contains(got, "claimed") {
		t.Fatalf("claimed enum not restored by round-trip down: %v", got)
	}

	// Sanity: the seeded ids still resolve and carry the claimed→unassigned
	// rewrite (i1/i2) — re-running down then up would re-rewrite, but here we
	// just confirm the ids we saved are readable.
	for _, id := range []string{i1, i2, i3} {
		r, err := app.FindRecordById(col.Id, id)
		if err != nil || r == nil {
			t.Errorf("seeded intake %s not readable after round-trip: %v", id, err)
		}
	}

	// Restore the final post-018 state: re-apply up018 so the intake status
	// field is removed again, proving 018 round-trips and leaving the schema
	// in the migrated shape (status absent).
	if err := upRemoveIntakeStatus(app); err != nil {
		t.Fatalf("upRemoveIntakeStatus: %v", err)
	}
	if f := findIntakeCol().Fields.GetByName("status"); f != nil {
		t.Fatal("intake still has status field after upRemoveIntakeStatus")
	}
}

// contains reports whether xs contains want.
func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
