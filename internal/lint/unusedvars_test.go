package lint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnusedVars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "proc.sql")
	content := `
CREATE PROCEDURE dbo.Test
AS
BEGIN
    DECLARE @used int;
    DECLARE @unused int;

    SELECT @used = 1;
    SELECT @used;
END
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	usages, err := UnusedVars(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(usages) != 1 {
		t.Fatalf("got %d unused vars, want 1: %#v", len(usages), usages)
	}
	if usages[0].Name != "@unused" {
		t.Fatalf("got %q, want @unused", usages[0].Name)
	}
}
