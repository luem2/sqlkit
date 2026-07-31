package dataseed

import (
	"strings"
	"testing"
)

func TestFilterGroupTables(t *testing.T) {
	group := GroupConfig{
		Name:           "g",
		SourceDatabase: "db",
		Output:         "out.sql",
		Mode:           "merge",
		Tables: []TableConfig{
			{Name: "ba.A", Key: []string{"id"}},
			{Name: "ba.B", Key: []string{"id"}},
		},
	}

	filtered, err := FilterGroupTables(group, []string{"ba.B"})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Tables) != 1 || filtered.Tables[0].Name != "ba.B" {
		t.Fatalf("unexpected tables: %#v", filtered.Tables)
	}
}

func TestFilterGroupTablesRequiresParent(t *testing.T) {
	group := GroupConfig{
		Name:           "g",
		SourceDatabase: "db",
		Output:         "out.sql",
		Mode:           "merge",
		Tables: []TableConfig{
			{Name: "ba.Parent", Key: []string{"id"}},
			{Name: "ba.Child", Key: []string{"id"}, Parent: "ba.Parent", ParentKey: []string{"id"}, ForeignKey: []string{"id_parent"}},
		},
	}

	if _, err := FilterGroupTables(group, []string{"ba.Child"}); err == nil {
		t.Fatal("expected parent dependency error")
	}
}

func TestValidateGroupAllowsBootstrapSensitivePhase(t *testing.T) {
	group := GroupConfig{
		Name:           "company-sensitive-P",
		SourceDatabase: "P_BD_SISTEMA",
		Phase:          "bootstrap-sensitive",
		Output:         "BD_SISTEMA/bootstrap-sensitive/generated/company/P.sql",
		Mode:           "merge",
		Tables: []TableConfig{
			{Name: "auth.Usuario", Key: []string{"id"}},
		},
	}

	if err := validateGroup(group); err != nil {
		t.Fatal(err)
	}
}

func TestRenderMergeUsesBatchSize(t *testing.T) {
	table := tableData{
		Config: TableConfig{
			Name: "ba.Calle",
			Key:  []string{"id"},
		},
		Schema: "ba",
		Name:   "Calle",
		Columns: []tableColumn{
			{Name: "id", TypeName: "int"},
			{Name: "nombre", TypeName: "nvarchar"},
		},
		Rows: []map[string]interface{}{
			{"id": 1, "nombre": "A"},
			{"id": 2, "nombre": "B"},
			{"id": 3, "nombre": "C"},
		},
		BatchSize: 2,
	}

	var builder strings.Builder
	renderMerge(&builder, table)
	sql := builder.String()

	if got := strings.Count(sql, "MERGE [ba].[Calle]"); got != 2 {
		t.Fatalf("expected 2 MERGE statements, got %d:\n%s", got, sql)
	}
	if strings.Contains(sql, "(1, N'A'),\n        (2, N'B'),\n        (3, N'C')") {
		t.Fatalf("expected rows to be split across batches:\n%s", sql)
	}
}
