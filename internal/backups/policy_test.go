package backups

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadPolicyAppliesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prod.toml")
	if err := os.WriteFile(path, []byte(`
environment = "prod"
enabled = true
databases = ["P_BD_SISTEMA"]
`), 0644); err != nil {
		t.Fatal(err)
	}

	policy, err := LoadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}

	if policy.LocalRoot != "backups" {
		t.Fatalf("LocalRoot = %q, want backups", policy.LocalRoot)
	}
	if policy.SQLServerRoot != "backups" {
		t.Fatalf("SQLServerRoot = %q, want backups", policy.SQLServerRoot)
	}
	if policy.Retention.FullDays != 60 || policy.Retention.LogDays != 15 {
		t.Fatalf("unexpected retention: %#v", policy.Retention)
	}
	if policy.RestoreDrill.DatabaseSuffix != "_RESTORE_DRILL" {
		t.Fatalf("DatabaseSuffix = %q", policy.RestoreDrill.DatabaseSuffix)
	}
}

func TestSQLServerBackupPathSupportsWindowsRoot(t *testing.T) {
	policy := &Policy{Environment: "prod-legacy", SQLServerRoot: `E:\BACKUP_BD`}
	at := time.Date(2026, 6, 17, 23, 0, 1, 0, time.UTC)

	got := SQLServerBackupPath(policy, "P_BD_SISTEMA", TypeLog, at)
	want := `E:\BACKUP_BD\prod-legacy\P_BD_SISTEMA\log\2026\06\17\P_BD_SISTEMA_LOG_20260617_230001.trn`
	if got != want {
		t.Fatalf("SQLServerBackupPath = %q, want %q", got, want)
	}
}

func TestLoadPolicyRejectsDuplicateDatabases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prod.toml")
	if err := os.WriteFile(path, []byte(`
environment = "prod"
enabled = true
databases = ["P_BD_SISTEMA", "p_bd_sistema"]
`), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadPolicy(path); err == nil {
		t.Fatal("expected duplicate database error")
	}
}

func TestLoadPolicySupportsRemoteCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prod.toml")
	if err := os.WriteFile(path, []byte(`
environment = "prod"
enabled = true
local_root = "E:\\BACKUPS"
sqlserver_root = "/opt/backup"
databases = ["P_BD_SISTEMA"]

[remote_copy]
enabled = true
host = "10.0.0.10"
user = "sqlbackup"
identity_file = "C:\\keys\\sqlbackup"
`), 0644); err != nil {
		t.Fatal(err)
	}

	policy, err := LoadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	if !policy.RemoteCopy.Enabled {
		t.Fatalf("RemoteCopy.Enabled = false, want true")
	}
	if policy.RemoteCopy.Port != 22 {
		t.Fatalf("RemoteCopy.Port = %d, want 22", policy.RemoteCopy.Port)
	}
	if policy.RemoteCopy.Host != "10.0.0.10" || policy.RemoteCopy.User != "sqlbackup" {
		t.Fatalf("unexpected remote copy config: %#v", policy.RemoteCopy)
	}
}

func TestLoadPolicyRejectsInvalidRemoteCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prod.toml")
	if err := os.WriteFile(path, []byte(`
environment = "prod"
enabled = true
databases = ["P_BD_SISTEMA"]

[remote_copy]
enabled = true
user = "sqlbackup"
`), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadPolicy(path); err == nil {
		t.Fatal("expected remote_copy.host validation error")
	}
}

func TestBackupAndManifestPaths(t *testing.T) {
	policy := &Policy{Environment: "prod", LocalRoot: "/var/backups/sql"}
	at := time.Date(2026, 6, 17, 23, 0, 1, 0, time.UTC)

	backupPath := BackupPath(policy, "P_BD_SISTEMA", TypeFull, at)
	wantBackup := "/var/backups/sql/prod/P_BD_SISTEMA/full/2026/06/17/P_BD_SISTEMA_FULL_20260617_230001.bak"
	if backupPath != wantBackup {
		t.Fatalf("BackupPath = %q, want %q", backupPath, wantBackup)
	}

	manifestPath := ManifestPath(policy, "P_BD_SISTEMA", TypeLog, at)
	wantManifest := "/var/backups/sql/prod/P_BD_SISTEMA/manifest/2026/06/17/P_BD_SISTEMA_LOG_20260617_230001.json"
	if manifestPath != wantManifest {
		t.Fatalf("ManifestPath = %q, want %q", manifestPath, wantManifest)
	}
}

func TestS3URI(t *testing.T) {
	policy := &Policy{Environment: "prod", S3Prefix: "s3://bucket/sql"}
	at := time.Date(2026, 6, 17, 23, 0, 1, 0, time.UTC)

	got := S3URI(policy, "P_BD_SISTEMA", TypeFull, at, "/backups/P_BD_SISTEMA_FULL_20260617_230001.bak")
	want := "s3://bucket/sql/prod/P_BD_SISTEMA/full/2026/06/17/P_BD_SISTEMA_FULL_20260617_230001.bak"
	if got != want {
		t.Fatalf("S3URI = %q, want %q", got, want)
	}
}

func TestS3ManifestURI(t *testing.T) {
	policy := &Policy{Environment: "prod", S3Prefix: "s3://bucket/sql"}
	at := time.Date(2026, 6, 17, 23, 0, 1, 0, time.UTC)

	got := S3ManifestURI(policy, "P_BD_SISTEMA", TypeFull, at, "/backups/P_BD_SISTEMA_FULL_20260617_230001.json")
	want := "s3://bucket/sql/prod/P_BD_SISTEMA/manifest/2026/06/17/P_BD_SISTEMA_FULL_20260617_230001.json"
	if got != want {
		t.Fatalf("S3ManifestURI = %q, want %q", got, want)
	}
}
