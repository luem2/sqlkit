package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/luem2/sqlkit/internal/config"
)

func TestResolveBackupHostPathDefaultsToBackupsDirectory(t *testing.T) {
	temp := t.TempDir()
	app := &appContext{cfg: &config.Config{Root: temp, Paths: map[string]string{}}}
	at := time.Date(2026, 6, 14, 12, 0, 0, 123, time.UTC)

	got := resolveBackupHostPath(app, "", "local", "P_BD_SISTEMA", at, "P_BD_SISTEMA_MANUAL_20260614_120000_000000123.bak")
	want := filepath.Join(temp, "data", "backups", "local", "P_BD_SISTEMA", "manual", "2026", "06", "14", "P_BD_SISTEMA_MANUAL_20260614_120000_000000123.bak")
	if got != want {
		t.Fatalf("resolveBackupHostPath() = %q, want %q", got, want)
	}
}

func TestResolveBackupHostPathTreatsOutputDirectoryAsHostDirectory(t *testing.T) {
	temp := t.TempDir()
	app := &appContext{cfg: &config.Config{Root: temp, Paths: map[string]string{}}}

	got := resolveBackupHostPath(app, temp, "local", "P_BD_SISTEMA", time.Time{}, "P_BD_SISTEMA.bak")
	want := filepath.Join(temp, "P_BD_SISTEMA.bak")
	if got != want {
		t.Fatalf("resolveBackupHostPath() = %q, want %q", got, want)
	}
}

func TestResolveBackupHostPathTreatsOutputAsHostFile(t *testing.T) {
	temp := t.TempDir()
	app := &appContext{cfg: &config.Config{Root: temp, Paths: map[string]string{}}}

	got := resolveBackupHostPath(app, filepath.Join(temp, "custom.bak"), "local", "P_BD_SISTEMA", time.Time{}, "P_BD_SISTEMA.bak")
	want := filepath.Join(temp, "custom.bak")
	if got != want {
		t.Fatalf("resolveBackupHostPath() = %q, want %q", got, want)
	}
}

func TestShouldMoveBackupToHostForLocalEvenWithoutContainer(t *testing.T) {
	if !shouldMoveBackupToHost(&dbFlags{env: "local"}) {
		t.Fatal("local backups should move to host by default")
	}
}

func TestBackupContainerUsesFlagBeforeConfig(t *testing.T) {
	app := &appContext{cfg: &config.Config{Paths: map[string]string{
		"sqlserver_container": "configured-container",
	}}}

	got := backupContainer(app, &dbFlags{container: "flag-container"})
	if got != "flag-container" {
		t.Fatalf("backupContainer() = %q, want flag-container", got)
	}
}

func TestResolveRestoreHostBakPathFindsRepoRelativeFile(t *testing.T) {
	temp := t.TempDir()
	app := &appContext{cfg: &config.Config{Root: temp, Paths: map[string]string{}}}
	path := filepath.Join(temp, "data", "backups", "P_BD_SISTEMA.bak")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("backup"), 0644); err != nil {
		t.Fatal(err)
	}

	got, ok := resolveRestoreHostBakPath(app, filepath.Join("data", "backups", "P_BD_SISTEMA.bak"))
	if !ok {
		t.Fatal("expected host bak path to be found")
	}
	if got != path {
		t.Fatalf("resolveRestoreHostBakPath() = %q, want %q", got, path)
	}
}

func TestResolveRestoreHostBakPathIgnoresSQLServerOnlyPath(t *testing.T) {
	temp := t.TempDir()
	app := &appContext{cfg: &config.Config{Root: temp, Paths: map[string]string{}}}

	if got, ok := resolveRestoreHostBakPath(app, "/var/opt/mssql/data/P_BD_SISTEMA.bak"); ok {
		t.Fatalf("resolveRestoreHostBakPath() = %q, true; want false", got)
	}
}

func TestRestoreContainerTempPathUsesSQLServerDataDir(t *testing.T) {
	app := &appContext{cfg: &config.Config{Paths: map[string]string{
		"sqlserver_data": "/var/opt/mssql/data",
	}}}
	at := time.Unix(0, 123456789)

	got := restoreContainerTempPath(app, "P_BD_SISTEMA2", "P_BD_SISTEMA.bak", at)
	want := "/var/opt/mssql/data/sqlkit-restore-P_BD_SISTEMA2-123456789.bak"
	if got != want {
		t.Fatalf("restoreContainerTempPath() = %q, want %q", got, want)
	}
}
