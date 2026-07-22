package sqlmetadata

import (
	"fmt"
	"strings"

	"github.com/luem2/sqlkit/internal/docs"
	"github.com/luem2/sqlkit/internal/sqlserver"
)

func Statement(tables []docs.SQLTableRef) sqlserver.Statement {
	var values []string
	for _, table := range tables {
		values = append(values, fmt.Sprintf("(N'%s', N'%s')", sqlLiteral(table.Schema), sqlLiteral(table.Table)))
	}
	text := fmt.Sprintf(`
WITH requested(schema_name, table_name) AS (
    SELECT schema_name, table_name
    FROM (VALUES %s) v(schema_name, table_name)
)
SELECT
    s.name AS schema_name,
    o.name AS table_name,
    c.name AS column_name,
    CASE
        WHEN ty.name IN (N'varchar', N'char', N'varbinary', N'binary') THEN ty.name + N'(' + CASE WHEN c.max_length = -1 THEN N'MAX' ELSE CONVERT(nvarchar(20), c.max_length) END + N')'
        WHEN ty.name IN (N'nvarchar', N'nchar') THEN ty.name + N'(' + CASE WHEN c.max_length = -1 THEN N'MAX' ELSE CONVERT(nvarchar(20), c.max_length / 2) END + N')'
        WHEN ty.name IN (N'decimal', N'numeric') THEN ty.name + N'(' + CONVERT(nvarchar(20), c.precision) + N', ' + CONVERT(nvarchar(20), c.scale) + N')'
        WHEN ty.name IN (N'datetime2', N'time', N'datetimeoffset') THEN ty.name + N'(' + CONVERT(nvarchar(20), c.scale) + N')'
        ELSE ty.name
    END AS data_type
FROM requested r
INNER JOIN sys.schemas s ON s.name = r.schema_name
INNER JOIN sys.objects o ON o.schema_id = s.schema_id AND o.name = r.table_name AND o.type IN (N'U', N'V')
INNER JOIN sys.columns c ON c.object_id = o.object_id
INNER JOIN sys.types ty ON ty.user_type_id = c.user_type_id
ORDER BY s.name, o.name, c.column_id;
`, strings.Join(values, ",\n        "))
	return sqlserver.Statement{Name: "docs metadata", Text: text}
}

func FromResultSets(resultSets []sqlserver.ResultSet) []docs.SQLColumnMetadata {
	if len(resultSets) == 0 {
		return nil
	}
	index := map[string]int{}
	for i, column := range resultSets[0].Columns {
		index[strings.ToLower(column)] = i
	}
	required := []string{"schema_name", "table_name", "column_name", "data_type"}
	for _, name := range required {
		if _, ok := index[name]; !ok {
			return nil
		}
	}

	var columns []docs.SQLColumnMetadata
	for _, row := range resultSets[0].Rows {
		columns = append(columns, docs.SQLColumnMetadata{
			Schema: row[index["schema_name"]],
			Table:  row[index["table_name"]],
			Column: row[index["column_name"]],
			Type:   strings.ToUpper(row[index["data_type"]]),
		})
	}
	return columns
}

func sqlLiteral(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
