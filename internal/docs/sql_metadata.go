package docs

import (
	"regexp"
	"sort"
	"strings"
)

func ReferencedSQLTables(doc *SQLProcedureDoc) []SQLTableRef {
	seen := map[string]bool{}
	var tables []SQLTableRef
	for _, block := range doc.Blocks {
		for _, table := range tableAliases(block.Body) {
			key := strings.ToLower(table.Schema + "." + table.Table)
			if seen[key] {
				continue
			}
			seen[key] = true
			tables = append(tables, table)
		}
	}
	sort.Slice(tables, func(i, j int) bool {
		left := strings.ToLower(tables[i].Schema + "." + tables[i].Table)
		right := strings.ToLower(tables[j].Schema + "." + tables[j].Table)
		return left < right
	})
	return tables
}

func EnrichSQLDocMetadata(doc *SQLProcedureDoc, columns []SQLColumnMetadata) {
	metadata := map[string]string{}
	for _, column := range columns {
		key := metadataKey(column.Schema, column.Table, column.Column)
		metadata[key] = column.Type
	}

	for blockIndex := range doc.Blocks {
		doc.Blocks[blockIndex].ResultSets = parseResultSetsWithMetadata(doc.Blocks[blockIndex].Body, metadata)
	}
}

func tableAliases(body string) map[string]SQLTableRef {
	aliases := map[string]SQLTableRef{}
	clean := stripSQLComments(body)
	tableRegex := regexp.MustCompile(`(?is)\b(?:FROM|JOIN)\s+((?:\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*)\s*\.\s*(?:\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*))(?:\s+(?:AS\s+)?(\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*))?`)
	for _, match := range tableRegex.FindAllStringSubmatch(clean, -1) {
		if len(match) < 2 {
			continue
		}
		parts := strings.Split(strings.Join(strings.Fields(match[1]), ""), ".")
		if len(parts) != 2 {
			continue
		}
		ref := SQLTableRef{
			Schema: trimIdentifier(parts[0]),
			Table:  trimIdentifier(parts[1]),
		}
		if ref.Schema == "" || ref.Table == "" {
			continue
		}
		alias := ref.Table
		if len(match) > 2 && strings.TrimSpace(match[2]) != "" {
			alias = trimIdentifier(match[2])
		}
		if strings.EqualFold(alias, "WITH") || strings.EqualFold(alias, "ON") || strings.EqualFold(alias, "WHERE") {
			alias = ref.Table
		}
		aliases[strings.ToLower(alias)] = ref
		aliases[strings.ToLower(ref.Schema+"."+ref.Table)] = ref
	}
	return aliases
}

func resolveResultColumnType(expression string, aliases map[string]SQLTableRef, metadata map[string]string) string {
	expr := expressionWithoutAlias(expression)
	qualifier, column, ok := baseColumnRef(expr)
	if !ok {
		return ""
	}
	if qualifier != "" {
		if table, ok := aliases[strings.ToLower(qualifier)]; ok {
			return metadata[metadataKey(table.Schema, table.Table, column)]
		}
		return ""
	}

	found := ""
	for _, table := range aliases {
		value := metadata[metadataKey(table.Schema, table.Table, column)]
		if value == "" {
			continue
		}
		if found != "" && found != value {
			return ""
		}
		found = value
	}
	return found
}

func baseColumnRef(expr string) (string, string, bool) {
	expr = strings.TrimSpace(expr)
	parts := strings.Split(expr, ".")
	if len(parts) == 1 && isSimpleIdentifier(parts[0]) {
		return "", trimIdentifier(parts[0]), true
	}
	if len(parts) == 2 && isSimpleIdentifier(parts[0]) && isSimpleIdentifier(parts[1]) {
		return trimIdentifier(parts[0]), trimIdentifier(parts[1]), true
	}
	return "", "", false
}

func isSimpleIdentifier(value string) bool {
	value = strings.TrimSpace(value)
	return regexp.MustCompile(`^(\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*)$`).MatchString(value)
}

func metadataKey(schema string, table string, column string) string {
	return strings.ToLower(schema + "." + table + "." + column)
}
