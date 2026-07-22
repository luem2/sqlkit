package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/luem2/sqlkit/internal/config"
)

func TestDefaultBacpacPathUsesDatedDatabaseStructure(t *testing.T) {
	root := t.TempDir()
	app := &appContext{cfg: &config.Config{
		Root:  root,
		Paths: map[string]string{"bacpacs": "data/bacpacs"},
	}}
	at := time.Date(2026, 6, 19, 14, 30, 5, 123, time.UTC)

	got := defaultBacpacPath(app, "prod", "P_BD_SISTEMA", at)
	want := filepath.Join(root, "data", "bacpacs", "prod", "P_BD_SISTEMA", "export", "2026", "06", "19", "P_BD_SISTEMA_BACPAC_20260619_143005_000000123.bacpac")
	if got != want {
		t.Fatalf("defaultBacpacPath() = %q, want %q", got, want)
	}
}

func TestResolveBacpacPathUsesExplicitPath(t *testing.T) {
	root := t.TempDir()
	app := &appContext{cfg: &config.Config{
		Root:  root,
		Paths: map[string]string{"bacpacs": "data/bacpacs"},
	}}

	got, err := resolveBacpacPath(app, "local", filepath.Join("custom", "db.bacpac"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "custom", "db.bacpac")
	if got != want {
		t.Fatalf("resolveBacpacPath() = %q, want %q", got, want)
	}
}

func TestResolveBacpacPathFindsFileNameUnderEnvironment(t *testing.T) {
	root := t.TempDir()
	app := &appContext{cfg: &config.Config{
		Root:  root,
		Paths: map[string]string{"bacpacs": "data/bacpacs"},
	}}
	want := filepath.Join(root, "data", "bacpacs", "local", "P_BD_SISTEMA", "export", "2026", "06", "19", "P_BD_SISTEMA.bacpac")
	if err := os.MkdirAll(filepath.Dir(want), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("bacpac"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveBacpacPath(app, "local", "P_BD_SISTEMA.bacpac")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolveBacpacPath() = %q, want %q", got, want)
	}
}

func TestResolveBacpacPathDoesNotSearchOtherEnvironments(t *testing.T) {
	root := t.TempDir()
	app := &appContext{cfg: &config.Config{
		Root:  root,
		Paths: map[string]string{"bacpacs": "data/bacpacs"},
	}}
	path := filepath.Join(root, "data", "bacpacs", "prod", "P_BD_SISTEMA", "export", "2026", "06", "19", "P_BD_SISTEMA.bacpac")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("bacpac"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := resolveBacpacPath(app, "local", "P_BD_SISTEMA.bacpac"); err == nil {
		t.Fatal("expected missing bacpac error")
	}
}

func TestResolveBacpacPathRejectsAmbiguousFileName(t *testing.T) {
	root := t.TempDir()
	app := &appContext{cfg: &config.Config{
		Root:  root,
		Paths: map[string]string{"bacpacs": "data/bacpacs"},
	}}
	for _, dir := range []string{
		filepath.Join(root, "data", "bacpacs", "local", "DB1", "export", "2026", "06", "19"),
		filepath.Join(root, "data", "bacpacs", "local", "DB2", "export", "2026", "06", "19"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "same.bacpac"), []byte("bacpac"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	_, err := resolveBacpacPath(app, "local", "same.bacpac")
	if err == nil {
		t.Fatal("expected ambiguous bacpac error")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("error = %q, want ambiguous", err)
	}
}
