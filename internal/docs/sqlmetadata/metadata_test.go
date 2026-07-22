package sqlmetadata

import (
	"strings"
	"testing"

	"github.com/luem2/sqlkit/internal/docs"
	"github.com/luem2/sqlkit/internal/sqlserver"
)

func TestStatementQueriesSQLServerMetadata(t *testing.T) {
	statement := Statement([]docs.SQLTableRef{{Schema: "vn", Table: "Solicitud"}})
	for _, want := range []string{"sys.schemas", "sys.objects", "sys.columns", "sys.types", "N'vn'", "N'Solicitud'"} {
		if !strings.Contains(statement.Text, want) {
			t.Fatalf("statement does not contain %q:\n%s", want, statement.Text)
		}
	}
}

func TestFromResultSetsMapsColumnMetadata(t *testing.T) {
	got := FromResultSets([]sqlserver.ResultSet{
		{
			Columns: []string{"schema_name", "table_name", "column_name", "data_type"},
			Rows: [][]string{
				{"vn", "Solicitud", "id", "int"},
				{"vn", "Solicitud", "nombre", "varchar(45)"},
			},
		},
	})

	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2: %#v", len(got), got)
	}
	if got[0] != (docs.SQLColumnMetadata{Schema: "vn", Table: "Solicitud", Column: "id", Type: "INT"}) {
		t.Fatalf("unexpected first row: %#v", got[0])
	}
	if got[1].Type != "VARCHAR(45)" {
		t.Fatalf("type was not normalized to upper-case: %#v", got[1])
	}
}
