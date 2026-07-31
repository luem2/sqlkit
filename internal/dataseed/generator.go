package dataseed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/luem2/sqlkit/internal/sqlserver"
)

type Options struct {
	RepoRoot string
	Group    GroupConfig
}

type tableColumn struct {
	Name       string
	TypeName   string
	IsIdentity bool
}

type tableData struct {
	Config    TableConfig
	Schema    string
	Name      string
	Columns   []tableColumn
	Rows      []map[string]interface{}
	Lookups   map[string]columnLookup
	BatchSize int
}

type columnLookup struct {
	Config       ColumnLookupConfig
	Variable     string
	LookupSchema string
	LookupTable  string
}

func Generate(ctx context.Context, client *sqlserver.Client, options Options) (string, error) {
	group := options.Group
	tables := make([]tableData, 0, len(group.Tables))
	byName := make(map[string]tableData)

	for _, tableCfg := range group.Tables {
		schema, tableName, err := sqlserver.ParseSchemaTable(tableCfg.Name)
		if err != nil {
			return "", err
		}
		columns, err := loadColumns(ctx, client, schema, tableName)
		if err != nil {
			return "", err
		}
		rows, err := loadRows(ctx, client, tableCfg, schema, tableName, columns, byName)
		if err != nil {
			return "", err
		}
		lookups, err := buildColumnLookups(tableCfg, columns, len(tables)+1)
		if err != nil {
			return "", err
		}
		data := tableData{
			Config:    tableCfg,
			Schema:    schema,
			Name:      tableName,
			Columns:   columns,
			Rows:      rows,
			Lookups:   lookups,
			BatchSize: firstPositive(tableCfg.BatchSize, group.BatchSize),
		}
		tables = append(tables, data)
		byName[tableCfg.Name] = data
	}

	sql, err := render(group, tables)
	if err != nil {
		return "", err
	}

	output := group.Output
	if !filepath.IsAbs(output) {
		output = filepath.Join(options.RepoRoot, output)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(output, []byte(sql), 0644); err != nil {
		return "", err
	}
	return output, nil
}

func loadColumns(ctx context.Context, client *sqlserver.Client, schema string, table string) ([]tableColumn, error) {
	statement := sqlserver.Statement{
		Name: "data_seed_columns",
		Text: `
SELECT c.name,
    typ.name AS type_name,
    c.is_identity
FROM sys.columns AS c
    INNER JOIN sys.types AS typ ON typ.user_type_id = c.user_type_id
WHERE c.object_id = OBJECT_ID(QUOTENAME(@schema) + N'.' + QUOTENAME(@table))
ORDER BY c.column_id;`,
		Parameters: []sqlserver.Parameter{
			sqlserver.StringParam("schema", schema),
			sqlserver.StringParam("table", table),
		},
	}
	resultSets, err := client.QueryRaw(ctx, statement)
	if err != nil {
		return nil, err
	}
	if len(resultSets) == 0 || len(resultSets[0].Rows) == 0 {
		return nil, fmt.Errorf("table %s.%s not found or has no columns", schema, table)
	}
	var columns []tableColumn
	for _, row := range resultSets[0].Rows {
		columns = append(columns, tableColumn{
			Name:       fmt.Sprint(row[0]),
			TypeName:   strings.ToLower(fmt.Sprint(row[1])),
			IsIdentity: toBool(row[2]),
		})
	}
	return columns, nil
}

func loadRows(ctx context.Context, client *sqlserver.Client, cfg TableConfig, schema string, table string, columns []tableColumn, prior map[string]tableData) ([]map[string]interface{}, error) {
	query, err := selectRowsSQL(cfg, schema, table, columns, prior)
	if err != nil {
		return nil, err
	}
	resultSets, err := client.QueryRaw(ctx, sqlserver.Statement{Name: "data_seed_rows", Text: query})
	if err != nil {
		return nil, err
	}
	if len(resultSets) == 0 {
		return nil, nil
	}
	result := resultSets[0]
	rows := make([]map[string]interface{}, 0, len(result.Rows))
	for _, rawRow := range result.Rows {
		row := make(map[string]interface{}, len(result.Columns))
		for i, column := range result.Columns {
			row[column] = rawRow[i]
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func selectRowsSQL(cfg TableConfig, schema string, table string, columns []tableColumn, prior map[string]tableData) (string, error) {
	selectList := make([]string, 0, len(columns))
	for _, column := range columns {
		selectList = append(selectList, quoteName(column.Name))
	}

	var filters []string
	if strings.TrimSpace(cfg.Where) != "" {
		filters = append(filters, "("+cfg.Where+")")
	}
	if strings.TrimSpace(cfg.Parent) != "" {
		parent, ok := prior[cfg.Parent]
		if !ok {
			return "", fmt.Errorf("parent %q was not loaded", cfg.Parent)
		}
		filter := childFilter(cfg, parent)
		if filter == "" {
			filters = append(filters, "1 = 0")
		} else {
			filters = append(filters, filter)
		}
	}

	var builder strings.Builder
	builder.WriteString("SELECT ")
	builder.WriteString(strings.Join(selectList, ", "))
	builder.WriteString(" FROM ")
	builder.WriteString(qualifiedName(schema, table))
	if len(filters) > 0 {
		builder.WriteString(" WHERE ")
		builder.WriteString(strings.Join(filters, " AND "))
	}
	if len(cfg.Key) > 0 {
		order := make([]string, 0, len(cfg.Key))
		for _, key := range cfg.Key {
			order = append(order, quoteName(key))
		}
		builder.WriteString(" ORDER BY ")
		builder.WriteString(strings.Join(order, ", "))
	}
	builder.WriteString(";")
	return builder.String(), nil
}

func childFilter(cfg TableConfig, parent tableData) string {
	if len(parent.Rows) == 0 {
		return ""
	}
	tuples := make([]string, 0, len(parent.Rows))
	for _, row := range parent.Rows {
		values := make([]string, 0, len(cfg.ParentKey))
		for i, parentKey := range cfg.ParentKey {
			value := row[parentKey]
			typeName := parent.columnType(parentKey)
			values = append(values, sqlLiteral(value, typeName))
			if i >= len(cfg.ForeignKey) {
				return ""
			}
		}
		if len(values) == 1 {
			tuples = append(tuples, values[0])
		} else {
			tuples = append(tuples, "("+strings.Join(values, ", ")+")")
		}
	}
	if len(cfg.ForeignKey) == 1 {
		return quoteName(cfg.ForeignKey[0]) + " IN (" + strings.Join(tuples, ", ") + ")"
	}
	fks := make([]string, 0, len(cfg.ForeignKey))
	for _, fk := range cfg.ForeignKey {
		fks = append(fks, quoteName(fk))
	}
	return "(" + strings.Join(fks, ", ") + ") IN (" + strings.Join(tuples, ", ") + ")"
}

func render(group GroupConfig, tables []tableData) (string, error) {
	var builder strings.Builder
	builder.WriteString("/*\n")
	builder.WriteString("    Generated by sqlkit db data-script.\n")
	builder.WriteString("    Group: " + group.Name + "\n")
	builder.WriteString("    Source database: " + group.SourceDatabase + "\n")
	builder.WriteString("    Generated at: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
	builder.WriteString("*/\n\n")
	if company := companyGuard(group); company != "" {
		builder.WriteString("IF UPPER('$(Company)') = '" + company + "'\n")
		builder.WriteString("BEGIN\n")
	}
	builder.WriteString("SET NOCOUNT ON;\n")
	builder.WriteString("SET XACT_ABORT ON;\n\n")
	builder.WriteString("BEGIN TRY\n")
	builder.WriteString("    BEGIN TRANSACTION seed_" + safeToken(group.Name) + ";\n\n")

	renderColumnLookups(&builder, tables)

	if group.Mode == "replace" {
		for i := len(tables) - 1; i >= 0; i-- {
			builder.WriteString("    DELETE FROM " + tables[i].qualifiedName() + ";\n")
		}
		builder.WriteString("\n")
	}

	for _, table := range tables {
		if group.Mode == "merge" && group.Retention == "current" && group.Vigency.CloseUnseededOpen && strings.TrimSpace(table.Config.Where) != "" {
			renderCloseUnseededOpen(&builder, group, table)
		}
		if len(table.Rows) == 0 {
			builder.WriteString("    -- " + table.Config.Name + ": no rows exported.\n\n")
			continue
		}
		switch group.Mode {
		case "merge":
			renderMerge(&builder, table)
		case "replace":
			renderInsert(&builder, table, true)
		default:
			return "", fmt.Errorf("unsupported mode %q", group.Mode)
		}
	}

	builder.WriteString("    COMMIT TRANSACTION seed_" + safeToken(group.Name) + ";\n")
	builder.WriteString("END TRY\n")
	builder.WriteString("BEGIN CATCH\n")
	builder.WriteString("    IF XACT_STATE() <> 0\n")
	builder.WriteString("        ROLLBACK TRANSACTION seed_" + safeToken(group.Name) + ";\n\n")
	builder.WriteString("    THROW;\n")
	builder.WriteString("END CATCH;\n")
	if companyGuard(group) != "" {
		builder.WriteString("\nEND\n")
	}
	return builder.String(), nil
}

func companyGuard(group GroupConfig) string {
	if !strings.EqualFold(strings.TrimSpace(group.Scope), "company") {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(group.Company))
}

func buildColumnLookups(cfg TableConfig, columns []tableColumn, tableIndex int) (map[string]columnLookup, error) {
	lookups := make(map[string]columnLookup, len(cfg.ColumnLookups))
	for _, lookupCfg := range cfg.ColumnLookups {
		columnName := strings.TrimSpace(lookupCfg.Column)
		if !hasColumn(columns, columnName) {
			return nil, fmt.Errorf("table %q column_lookup column %q does not exist", cfg.Name, columnName)
		}
		schema, table, err := sqlserver.ParseSchemaTable(lookupCfg.LookupTable)
		if err != nil {
			return nil, fmt.Errorf("table %q column_lookup %q lookup_table: %w", cfg.Name, columnName, err)
		}
		key := strings.ToLower(columnName)
		if _, exists := lookups[key]; exists {
			return nil, fmt.Errorf("table %q contains duplicate column_lookup for %q", cfg.Name, columnName)
		}
		lookups[key] = columnLookup{
			Config:       lookupCfg,
			Variable:     "@seed_lookup_" + strconv.Itoa(tableIndex) + "_" + safeToken(columnName),
			LookupSchema: schema,
			LookupTable:  table,
		}
	}
	return lookups, nil
}

func renderColumnLookups(builder *strings.Builder, tables []tableData) {
	for _, table := range tables {
		for _, lookup := range table.sortedLookups() {
			builder.WriteString("    DECLARE " + lookup.Variable + " " + sqlVariableType(table.columnType(lookup.Config.Column)) + ";\n")
			builder.WriteString("    SELECT " + lookup.Variable + " = " + quoteName(lookup.Config.LookupColumn) + "\n")
			builder.WriteString("    FROM " + qualifiedName(lookup.LookupSchema, lookup.LookupTable) + "\n")
			builder.WriteString("    WHERE " + quoteName(lookup.Config.MatchColumn) + " = " + sqlLiteral(lookup.Config.MatchValue, "nvarchar") + ";\n")
			if !lookup.Config.Optional {
				builder.WriteString("\n")
				builder.WriteString("    IF " + lookup.Variable + " IS NULL\n")
				builder.WriteString("        THROW 51002, 'Seed lookup not found: " + lookup.Config.LookupTable + "." + lookup.Config.MatchColumn + " = " + escapeSQLErrorText(lookup.Config.MatchValue) + "', 1;\n")
			}
			builder.WriteString("\n")
		}
	}
}

func renderCloseUnseededOpen(builder *strings.Builder, group GroupConfig, table tableData) {
	if strings.TrimSpace(group.Vigency.ToColumn) == "" || strings.TrimSpace(group.Vigency.CurrentWhere) == "" {
		return
	}
	sourceDate := "CAST(GETDATE() AS DATE)"
	if strings.TrimSpace(group.Vigency.FromColumn) != "" && len(table.Rows) > 0 {
		sourceDate = sqlLiteral(table.Rows[0][group.Vigency.FromColumn], table.columnType(group.Vigency.FromColumn))
	}
	builder.WriteString("    UPDATE target\n")
	builder.WriteString("    SET " + quoteName(group.Vigency.ToColumn) + " = DATEADD(DAY, -1, " + sourceDate + ")\n")
	builder.WriteString("    FROM " + table.qualifiedName() + " AS target\n")
	builder.WriteString("    WHERE " + group.Vigency.CurrentWhere + "\n")
	builder.WriteString("        AND NOT EXISTS (\n")
	builder.WriteString("            SELECT 1\n")
	builder.WriteString("            FROM (VALUES\n")
	renderValuesRows(builder, table, table.Rows, "                ")
	builder.WriteString("            ) AS source (" + sourceColumnList(table) + ")\n")
	builder.WriteString("            WHERE " + joinKeyPredicate("target", "source", table.Config.Key) + "\n")
	builder.WriteString("        );\n\n")
}

func renderMerge(builder *strings.Builder, table tableData) {
	if len(table.Rows) == 0 {
		return
	}
	if table.hasIncludedIdentity() {
		builder.WriteString("    SET IDENTITY_INSERT " + table.qualifiedName() + " ON;\n\n")
	}
	for _, rows := range table.rowBatches() {
		builder.WriteString("    MERGE " + table.qualifiedName() + " WITH (HOLDLOCK) AS target\n")
		builder.WriteString("    USING (VALUES\n")
		renderValuesRows(builder, table, rows, "        ")
		builder.WriteString("    ) AS source (" + sourceColumnList(table) + ")\n")
		builder.WriteString("    ON " + joinKeyPredicate("target", "source", table.Config.Key) + "\n")
		if assignments := updateAssignments(table); assignments != "" {
			builder.WriteString("    WHEN MATCHED THEN\n")
			builder.WriteString("        UPDATE SET " + assignments + "\n")
		}
		builder.WriteString("    WHEN NOT MATCHED BY TARGET THEN\n")
		builder.WriteString("        INSERT (" + targetColumnList(table) + ")\n")
		builder.WriteString("        VALUES (" + prefixedColumnList("source", table.columnsForInsert()) + ");\n\n")
	}
	if table.hasIncludedIdentity() {
		builder.WriteString("    SET IDENTITY_INSERT " + table.qualifiedName() + " OFF;\n\n")
	}
}

func renderInsert(builder *strings.Builder, table tableData, reseed bool) {
	if len(table.Rows) == 0 {
		return
	}
	if table.hasIncludedIdentity() {
		builder.WriteString("    SET IDENTITY_INSERT " + table.qualifiedName() + " ON;\n\n")
	}
	for _, rows := range table.rowBatches() {
		builder.WriteString("    INSERT INTO " + table.qualifiedName() + " (" + targetColumnList(table) + ")\n")
		builder.WriteString("    VALUES\n")
		renderValuesRows(builder, table, rows, "        ")
		builder.WriteString("    ;\n\n")
	}
	if table.hasIncludedIdentity() {
		builder.WriteString("    SET IDENTITY_INSERT " + table.qualifiedName() + " OFF;\n")
		if reseed {
			builder.WriteString("    DBCC CHECKIDENT ('" + table.Schema + "." + table.Name + "', RESEED) WITH NO_INFOMSGS;\n")
		}
		builder.WriteString("\n")
	}
}

func renderValuesRows(builder *strings.Builder, table tableData, rows []map[string]interface{}, indent string) {
	columns := table.columnsForInsert()
	for rowIndex, row := range rows {
		values := make([]string, 0, len(columns))
		for _, column := range columns {
			values = append(values, table.sqlValue(row, column))
		}
		suffix := ","
		if rowIndex == len(rows)-1 {
			suffix = ""
		}
		builder.WriteString(indent + "(" + strings.Join(values, ", ") + ")" + suffix + "\n")
	}
}

func (t tableData) rowBatches() [][]map[string]interface{} {
	if t.BatchSize <= 0 || len(t.Rows) <= t.BatchSize {
		return [][]map[string]interface{}{t.Rows}
	}
	batches := make([][]map[string]interface{}, 0, (len(t.Rows)+t.BatchSize-1)/t.BatchSize)
	for start := 0; start < len(t.Rows); start += t.BatchSize {
		end := start + t.BatchSize
		if end > len(t.Rows) {
			end = len(t.Rows)
		}
		batches = append(batches, t.Rows[start:end])
	}
	return batches
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func (t tableData) sqlValue(row map[string]interface{}, column tableColumn) string {
	if lookup, ok := t.Lookups[strings.ToLower(column.Name)]; ok {
		return lookup.Variable
	}
	return sqlLiteral(row[column.Name], column.TypeName)
}

func updateAssignments(table tableData) string {
	keySet := make(map[string]struct{}, len(table.Config.Key))
	for _, key := range table.Config.Key {
		keySet[strings.ToLower(key)] = struct{}{}
	}
	var assignments []string
	for _, column := range table.columnsForInsert() {
		if _, ok := keySet[strings.ToLower(column.Name)]; ok {
			continue
		}
		assignments = append(assignments, "target."+quoteName(column.Name)+" = source."+quoteName(column.Name))
	}
	return strings.Join(assignments, ", ")
}

func joinKeyPredicate(leftAlias string, rightAlias string, keys []string) string {
	predicates := make([]string, 0, len(keys))
	for _, key := range keys {
		predicates = append(predicates, leftAlias+"."+quoteName(key)+" = "+rightAlias+"."+quoteName(key))
	}
	return strings.Join(predicates, " AND ")
}

func sourceColumnList(table tableData) string {
	return prefixedColumnList("", table.columnsForInsert())
}

func targetColumnList(table tableData) string {
	return prefixedColumnList("", table.columnsForInsert())
}

func prefixedColumnList(prefix string, columns []tableColumn) string {
	values := make([]string, 0, len(columns))
	for _, column := range columns {
		value := quoteName(column.Name)
		if prefix != "" {
			value = prefix + "." + value
		}
		values = append(values, value)
	}
	return strings.Join(values, ", ")
}

func (t tableData) columnsForInsert() []tableColumn {
	columns := make([]tableColumn, 0, len(t.Columns))
	for _, column := range t.Columns {
		if column.IsIdentity && !t.Config.IncludeIdentity {
			continue
		}
		columns = append(columns, column)
	}
	return columns
}

func (t tableData) columnType(name string) string {
	for _, column := range t.Columns {
		if strings.EqualFold(column.Name, name) {
			return column.TypeName
		}
	}
	return ""
}

func (t tableData) hasIncludedIdentity() bool {
	for _, column := range t.Columns {
		if column.IsIdentity && t.Config.IncludeIdentity {
			return true
		}
	}
	return false
}

func (t tableData) qualifiedName() string {
	return qualifiedName(t.Schema, t.Name)
}

func (t tableData) sortedLookups() []columnLookup {
	keys := make([]string, 0, len(t.Lookups))
	for key := range t.Lookups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lookups := make([]columnLookup, 0, len(keys))
	for _, key := range keys {
		lookups = append(lookups, t.Lookups[key])
	}
	return lookups
}

func hasColumn(columns []tableColumn, name string) bool {
	for _, column := range columns {
		if strings.EqualFold(column.Name, name) {
			return true
		}
	}
	return false
}

func sqlLiteral(value interface{}, typeName string) string {
	if value == nil {
		return "NULL"
	}
	switch typed := value.(type) {
	case []byte:
		value = string(typed)
	case time.Time:
		if isDateType(typeName) {
			return "'" + typed.Format("2006-01-02") + "'"
		}
		return "'" + typed.Format("2006-01-02 15:04:05") + "'"
	case bool:
		if typed {
			return "1"
		}
		return "0"
	}
	text := fmt.Sprint(value)
	if isNumericType(typeName) || isBitType(typeName) {
		return text
	}
	if isBinaryType(typeName) {
		return "0x" + fmt.Sprintf("%x", value)
	}
	return "N'" + strings.ReplaceAll(text, "'", "''") + "'"
}

func sqlVariableType(typeName string) string {
	switch strings.ToLower(typeName) {
	case "bigint", "int", "smallint", "tinyint", "bit", "date", "datetime", "datetime2", "smalldatetime", "datetimeoffset", "money", "smallmoney", "float", "real":
		return strings.ToUpper(typeName)
	case "decimal", "numeric":
		return strings.ToUpper(typeName) + "(38, 10)"
	default:
		return "NVARCHAR(MAX)"
	}
}

func escapeSQLErrorText(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func isNumericType(typeName string) bool {
	switch strings.ToLower(typeName) {
	case "tinyint", "smallint", "int", "bigint", "decimal", "numeric", "money", "smallmoney", "float", "real":
		return true
	default:
		return false
	}
}

func isBitType(typeName string) bool {
	return strings.EqualFold(typeName, "bit")
}

func isDateType(typeName string) bool {
	switch strings.ToLower(typeName) {
	case "date", "datetime", "datetime2", "smalldatetime", "datetimeoffset":
		return true
	default:
		return false
	}
}

func isBinaryType(typeName string) bool {
	switch strings.ToLower(typeName) {
	case "binary", "varbinary", "image":
		return true
	default:
		return false
	}
}

func qualifiedName(schema string, table string) string {
	return quoteName(schema) + "." + quoteName(table)
}

func quoteName(value string) string {
	return "[" + strings.ReplaceAll(strings.TrimSpace(value), "]", "]]") + "]"
}

func safeToken(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		} else {
			builder.WriteRune('_')
		}
	}
	token := builder.String()
	if token == "" {
		return "data_seed"
	}
	return token
}

func toBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case int64:
		return typed != 0
	case int:
		return typed != 0
	case []byte:
		parsed, _ := strconv.ParseBool(string(typed))
		return parsed
	case string:
		parsed, _ := strconv.ParseBool(typed)
		return parsed
	default:
		return fmt.Sprint(value) == "1" || strings.EqualFold(fmt.Sprint(value), "true")
	}
}

func sortedKeys(values map[string]interface{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
