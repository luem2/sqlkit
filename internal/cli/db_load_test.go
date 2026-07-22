package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/luem2/sqlkit/internal/config"
)

func TestDetectLoadSourceTypeFromExtension(t *testing.T) {
	tests := map[string]loadSourceType{
		"backup.BAK":      loadSourceBak,
		"database.bacpac": loadSourceBacpac,
		"project.dacpac":  loadSourceDacpac,
	}
	for source, want := range tests {
		got, err := detectLoadSourceType(source, "")
		if err != nil {
			t.Fatalf("detectLoadSourceType(%q): %v", source, err)
		}
		if got != want {
			t.Fatalf("detectLoadSourceType(%q) = %q, want %q", source, got, want)
		}
	}
}

func TestDetectLoadSourceTypeUsesExplicitOverride(t *testing.T) {
	got, err := detectLoadSourceType("backup", "bak")
	if err != nil {
		t.Fatal(err)
	}
	if got != loadSourceBak {
		t.Fatalf("detectLoadSourceType() = %q, want %q", got, loadSourceBak)
	}
}

func TestDetectLoadSourceTypeRejectsUnknownExtension(t *testing.T) {
	if _, err := detectLoadSourceType("backup.zip", ""); err == nil {
		t.Fatal("expected unknown extension error")
	}
}

func TestResolveExistingHostSourceFindsRepoRelativeFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "data", "restore.bak")
	if err := makeTestFile(source); err != nil {
		t.Fatal(err)
	}
	app := &appContext{cfg: &config.Config{Root: root}}

	got, ok := resolveExistingHostSource(app, "data/restore.bak")
	if !ok {
		t.Fatal("expected host source")
	}
	if got != source {
		t.Fatalf("resolveExistingHostSource() = %q, want %q", got, source)
	}
}

func TestLoadStagedBakPathUsesSqlServerDataDirectory(t *testing.T) {
	app := &appContext{cfg: &config.Config{Paths: map[string]string{
		"sqlserver_backup_dir": "backups",
		"sqlserver_data":       "/var/opt/mssql/data",
	}}}

	got := loadStagedBakPath(app, &dbFlags{}, "/tmp/restore.bak")
	want := "/var/opt/mssql/data/backups/_load/restore.bak"
	if got != want {
		t.Fatalf("loadStagedBakPath() = %q, want %q", got, want)
	}
}

func makeTestFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("test"), 0644)
}
