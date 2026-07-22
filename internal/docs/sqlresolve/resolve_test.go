package sqlresolve

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFindsProcedureByFileName(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "BD_SISTEMA", "vn", "StoredProcedures", "SP_SEARCH_Test.sql")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("CREATE PROCEDURE [vn].[SP_SEARCH_Test]\nAS\nSELECT 1;\n"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve(root, "SP_SEARCH_Test")
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("got %q, want %q", got, path)
	}
}

func TestResolveFindsProcedureByContent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "GRUPO_CENTRAL", "dbo", "StoredProcedures", "DifferentName.sql")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("CREATE PROCEDURE [dbo].[SP_EXACT_Test]\nAS\nSELECT 1;\n"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve(root, "SP_EXACT_Test")
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("got %q, want %q", got, path)
	}
}
