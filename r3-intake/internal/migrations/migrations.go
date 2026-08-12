// Package migrations embeds the PocketBase migration files into the binary.
// At runtime the embedded JS migrations are written to a temporary directory so
// PocketBase's jsvm plugin can load them; the Go migration stub is registered
// directly from code.
package migrations

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed all:pocketbase/migrations
var embeddedFS embed.FS

// Dir returns a path containing the JS migration files, ready to pass to
// jsvm.Config{MigrationsDir: ...}. If a writableDir is provided, the files
// are extracted there; otherwise a fresh temporary directory is used.
func Dir(writableDir string) (string, error) {
	if writableDir == "" {
		var err error
		writableDir, err = os.MkdirTemp("", "r3-migrations-*")
		if err != nil {
			return "", fmt.Errorf("create migrations temp dir: %w", err)
		}
	}
	root := "pocketbase/migrations"
	return writableDir, fs.WalkDir(embeddedFS, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := embeddedFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded migration %s: %w", path, err)
		}
		dest := filepath.Join(writableDir, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0750); err != nil {
			return fmt.Errorf("create migration dir %s: %w", filepath.Dir(dest), err)
		}
		if err := os.WriteFile(dest, data, 0640); err != nil {
			return fmt.Errorf("write migration %s: %w", dest, err)
		}
		return nil
	})
}

// Extract extracts the JS migrations into the provided directory. The Go
// migration stub is registered separately by importing pbmigrations in main.
func Extract(dir string) error {
	_, err := Dir(dir)
	return err
}
