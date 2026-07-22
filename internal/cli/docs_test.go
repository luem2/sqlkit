package cli

import (
	"path/filepath"
	"testing"

	"github.com/luem2/sqlkit/internal/config"
)

func TestShouldEnrichSQLDocRequiresEnvAndDatabaseTogether(t *testing.T) {
	if _, err := shouldEnrichSQLDoc(&docsFlags{env: "local"}); err == nil {
		t.Fatal("expected database requirement")
	}
	if _, err := shouldEnrichSQLDoc(&docsFlags{database: "P_BD_SISTEMA"}); err == nil {
		t.Fatal("expected env requirement")
	}
	got, err := shouldEnrichSQLDoc(&docsFlags{env: "local", database: "P_BD_SISTEMA"})
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("expected metadata enrichment")
	}
}

func TestSQLDocOutputPathDefaultsToDocsGenerated(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "BD_SISTEMA", "vn", "StoredProcedures", "SP_SEARCH_Test.sql")
	app := &appContext{
		cfg: &config.Config{
			Root:  root,
			Paths: map[string]string{"docs": "docs"},
		},
	}

	got := sqlDocOutputPath(app, &docsFlags{}, source)
	want := filepath.Join(root, "docs", "generated", "SP_SEARCH_Test.md")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
