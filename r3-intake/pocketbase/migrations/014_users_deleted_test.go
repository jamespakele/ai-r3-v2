package migrations

import (
	"path/filepath"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/jsvm"
)

func TestUsersDeletedMigration(t *testing.T) {
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

	col, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("find users collection: %v", err)
	}
	f := col.Fields.GetByName("deleted")
	if f == nil {
		t.Fatal("users collection missing deleted field after migrations")
	}
	if _, ok := f.(*core.BoolField); !ok {
		t.Fatalf("users.deleted is %T, want *core.BoolField", f)
	}

	// Idempotent up: second call must be a no-op and not error.
	if err := upUsersDeleted(app); err != nil {
		t.Fatalf("upUsersDeleted idempotent: %v", err)
	}
	if col.Fields.GetByName("deleted") == nil {
		t.Fatal("users.deleted removed by idempotent up call")
	}

	// Down removes the field.
	if err := downUsersDeleted(app); err != nil {
		t.Fatalf("downUsersDeleted: %v", err)
	}
	col, err = app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("find users collection after down: %v", err)
	}
	if col.Fields.GetByName("deleted") != nil {
		t.Fatal("users.deleted still present after down migration")
	}

	// Idempotent down: second call must be a no-op and not error.
	if err := downUsersDeleted(app); err != nil {
		t.Fatalf("downUsersDeleted idempotent: %v", err)
	}
}
