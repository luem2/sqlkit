package sqlscripts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDirOrdersSQLFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"02-b.sql", "01-a.sql", "ignore.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	scripts, err := ResolveDir(".", dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(scripts) != 2 {
		t.Fatalf("got %d scripts, want 2", len(scripts))
	}
	if filepath.Base(scripts[0]) != "01-a.sql" || filepath.Base(scripts[1]) != "02-b.sql" {
		t.Fatalf("scripts not ordered: %#v", scripts)
	}
}

func TestResolveFileRejectsNonSQL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "script.txt")
	if err := os.WriteFile(path, []byte("SELECT 1;"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := ResolveFile(".", path); err == nil {
		t.Fatal("expected non-SQL file to be rejected")
	}
}
