package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationDirUsesEnvOverride(t *testing.T) {
	t.Setenv("PULLSING_MIGRATIONS_DIR", "/tmp/custom-migrations")

	if got := migrationDir(); got != "/tmp/custom-migrations" {
		t.Fatalf("migrationDir() = %q, want %q", got, "/tmp/custom-migrations")
	}
}

func TestMigrationDirUsesWorkingDirectory(t *testing.T) {
	t.Setenv("PULLSING_MIGRATIONS_DIR", "")

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tempDir := t.TempDir()
	migrationsDir := filepath.Join(tempDir, "migrations")
	if err := os.Mkdir(migrationsDir, 0o755); err != nil {
		t.Fatalf("mkdir migrations: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	got, err := filepath.EvalSymlinks(migrationDir())
	if err != nil {
		t.Fatalf("eval migration dir: %v", err)
	}
	want, err := filepath.EvalSymlinks(migrationsDir)
	if err != nil {
		t.Fatalf("eval expected migration dir: %v", err)
	}

	if got != want {
		t.Fatalf("migrationDir() = %q, want %q", got, want)
	}
}
