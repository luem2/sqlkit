package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/luem2/sqlkit/internal/backups"
	"github.com/luem2/sqlkit/internal/config"
)

func TestRemoteCopySSHArgs(t *testing.T) {
	remote := backups.RemoteCopy{
		Host:         "10.0.0.10",
		User:         "sqlbackup",
		Port:         2222,
		IdentityFile: `C:\keys\sqlbackup`,
	}

	gotSSH := sshArgs(remote)
	wantSSH := []string{"-p", "2222", "-i", `C:\keys\sqlbackup`, "sqlbackup@10.0.0.10"}
	if !reflect.DeepEqual(gotSSH, wantSSH) {
		t.Fatalf("sshArgs() = %#v, want %#v", gotSSH, wantSSH)
	}

	gotSCP := scpArgs(remote)
	wantSCP := []string{"-P", "2222", "-i", `C:\keys\sqlbackup`}
	if !reflect.DeepEqual(gotSCP, wantSCP) {
		t.Fatalf("scpArgs() = %#v, want %#v", gotSCP, wantSCP)
	}
}

func TestRemoteSpecUsesOpenSSHFormat(t *testing.T) {
	remote := backups.RemoteCopy{Host: "aws-db", User: "sqlbackup"}

	got := remoteSpec(remote, "/opt/backup/prod/db/full/file.bak")
	want := "sqlbackup@aws-db:/opt/backup/prod/db/full/file.bak"
	if got != want {
		t.Fatalf("remoteSpec() = %q, want %q", got, want)
	}
}

func TestShellQuoteEscapesSingleQuotes(t *testing.T) {
	got := shellQuote("/opt/back'up/file.bak")
	want := `'/opt/back'\''up/file.bak'`
	if got != want {
		t.Fatalf("shellQuote() = %q, want %q", got, want)
	}
}

func TestRemoteRemoveFileCommandPrunesEmptyParentsWithinRoot(t *testing.T) {
	got := remoteRemoveFileCommand("/opt/backup/prod/db/log/2026/07/13/file.trn", "/opt/backup")
	mustContain(t, got, "rm -f -- '/opt/backup/prod/db/log/2026/07/13/file.trn'")
	mustContain(t, got, "root='/opt/backup'")
	mustContain(t, got, `case "$dir/" in "$root"/*)`)
	mustContain(t, got, `rmdir -- "$dir"`)
	mustContain(t, got, `[ "$dir" != "$root" ]`)
}

func mustContain(t *testing.T, value string, fragment string) {
	t.Helper()
	if !strings.Contains(value, fragment) {
		t.Fatalf("%q does not contain %q", value, fragment)
	}
}

func TestLoadBackupPolicyAppliesRuntimeOverrides(t *testing.T) {
	temp := t.TempDir()
	policyPath := filepath.Join(temp, "prod.toml")
	if err := os.WriteFile(policyPath, []byte(`
environment = "prod"
enabled = true
local_root = "E:\\BACKUPS"
sqlserver_root = "/opt/backup"
databases = ["P_BD_SISTEMA"]

[remote_copy]
enabled = true
host = "sql.example.com"
user = "sqlbackup"
`), 0644); err != nil {
		t.Fatal(err)
	}
	app := &appContext{cfg: &config.Config{Root: temp, Paths: map[string]string{}}}
	flags := &backupFlags{
		env:               "prod",
		policy:            "prod.toml",
		localRoot:         "/opt/backup",
		sqlServerRoot:     "/opt/backup",
		disableRemoteCopy: true,
	}

	policy, err := loadBackupPolicy(app, flags)
	if err != nil {
		t.Fatal(err)
	}
	if policy.LocalRoot != "/opt/backup" {
		t.Fatalf("LocalRoot = %q, want /opt/backup", policy.LocalRoot)
	}
	if policy.SQLServerRoot != "/opt/backup" {
		t.Fatalf("SQLServerRoot = %q, want /opt/backup", policy.SQLServerRoot)
	}
	if policy.RemoteCopy.Enabled {
		t.Fatal("RemoteCopy.Enabled = true, want false")
	}
}

func TestLoadBackupPolicyResolvesRelativeLocalRootOverride(t *testing.T) {
	temp := t.TempDir()
	policyPath := filepath.Join(temp, "prod-legacy.toml")
	if err := os.WriteFile(policyPath, []byte(`
environment = "prod-legacy"
enabled = true
local_root = "E:\\BACKUPS"
sqlserver_root = "E:\\BACKUPS"
databases = ["P_BD_SISTEMA"]
`), 0644); err != nil {
		t.Fatal(err)
	}
	app := &appContext{cfg: &config.Config{Root: temp, Paths: map[string]string{}}}
	flags := &backupFlags{
		env:           "prod-legacy",
		policy:        "prod-legacy.toml",
		localRoot:     "data/ops-backups",
		sqlServerRoot: "/opt/backup",
	}

	policy, err := loadBackupPolicy(app, flags)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(temp, "data", "ops-backups")
	if policy.LocalRoot != want {
		t.Fatalf("LocalRoot = %q, want %q", policy.LocalRoot, want)
	}
	if policy.SQLServerRoot != "/opt/backup" {
		t.Fatalf("SQLServerRoot = %q, want /opt/backup", policy.SQLServerRoot)
	}
}

func TestLoadBackupPolicyCanDisableInvalidRemoteCopy(t *testing.T) {
	temp := t.TempDir()
	policyPath := filepath.Join(temp, "prod.toml")
	if err := os.WriteFile(policyPath, []byte(`
environment = "prod"
enabled = true
local_root = "E:\\BACKUPS"
sqlserver_root = "/opt/backup"
databases = ["P_BD_SISTEMA"]

[remote_copy]
enabled = true
user = "sqlbackup"
`), 0644); err != nil {
		t.Fatal(err)
	}
	app := &appContext{cfg: &config.Config{Root: temp, Paths: map[string]string{}}}
	flags := &backupFlags{
		env:               "prod",
		policy:            "prod.toml",
		disableRemoteCopy: true,
	}

	policy, err := loadBackupPolicy(app, flags)
	if err != nil {
		t.Fatal(err)
	}
	if policy.RemoteCopy.Enabled {
		t.Fatal("RemoteCopy.Enabled = true, want false")
	}
}
