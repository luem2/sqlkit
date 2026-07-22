package docs

import (
	"regexp"
	"strconv"
	"strings"
)

func parseResultSets(body string) []SQLResultSet {
	return parseResultSetsWithMetadata(body, nil)
}

func parseResultSetsWithMetadata(body string, metadata map[string]string) []SQLResultSet {
	clean := stripSQLComments(body)
	var resultSets []SQLResultSet
	tempTables := map[string][]SQLResultColumn{}
	pos := 0
	for {
		selectStart := findTopLevelKeyword(clean[pos:], "SELECT")
		if selectStart < 0 {
			break
		}
		selectStart += pos
		fromStart := findTopLevelFrom(clean, selectStart+len("SELECT"))
		if fromStart < 0 {
			statementEnd := findStatementEnd(clean, selectStart+len("SELECT"))
			list := strings.TrimSpace(clean[selectStart+len("SELECT") : statementEnd])
			if !(strings.Contains(list, "=") && regexp.MustCompile(`(?i)^\s*@`).MatchString(list)) {
				resultSets = append(resultSets, SQLResultSet{Columns: parseSelectList(list, nil, nil, metadata)})
			}
			pos = statementEnd
			continue
		}
		if semicolon := strings.Index(clean[selectStart:fromStart], ";"); semicolon >= 0 {
			pos = selectStart + semicolon + 1
			continue
		}
		for name, columns := range cteColumnAliases(clean[pos:selectStart], tempTables, metadata) {
			tempTables[strings.ToLower(name)] = columns
		}
		list := strings.TrimSpace(clean[selectStart+len("SELECT") : fromStart])
		statementEnd := findStatementEnd(clean, fromStart)
		fromClause := clean[fromStart:statementEnd]
		tempAliases := tempTableAliases(fromClause, tempTables)
		realAliases := tableAliases(fromClause)
		for alias, columns := range applyColumnAliases(fromClause, tempTables, metadata) {
			tempAliases[alias] = columns
		}
		if tempName, tempColumns, ok := parseSelectIntoTempTable(list, tempAliases, realAliases, metadata); ok {
			tempTables[strings.ToLower(tempName)] = tempColumns
			pos = statementEnd
			continue
		}
		if strings.Contains(list, "=") && regexp.MustCompile(`(?i)^\s*@`).MatchString(list) {
			pos = statementEnd
			continue
		}
		if strings.TrimSpace(list) == "*" {
			table := firstTableAfterFrom(clean[fromStart+len("FROM"):])
			if columns, ok := tempTables[strings.ToLower(table)]; ok {
				resultSets = append(resultSets, SQLResultSet{Columns: dedupeResultColumns(columns)})
			} else {
				resultSets = append(resultSets, SQLResultSet{Columns: parseSelectList(list, tempAliases, realAliases, metadata)})
			}
		} else {
			resultSets = append(resultSets, SQLResultSet{Columns: parseSelectList(list, tempAliases, realAliases, metadata)})
		}
		pos = statementEnd
	}
	return resultSets
}

func parseSelectIntoTempTable(list string, tempAliases map[string][]SQLResultColumn, realAliases map[string]SQLTableRef, metadata map[string]string) (string, []SQLResultColumn, bool) {
	intoRegex := regexp.MustCompile(`(?is)\bINTO\s+(#\w+)\b`)
	match := intoRegex.FindStringSubmatchIndex(list)
	if len(match) == 0 {
		return "", nil, false
	}
	name := list[match[2]:match[3]]
	columns := parseSelectList(strings.TrimSpace(list[:match[0]]), tempAliases, realAliases, metadata)
	return name, columns, true
}

func parseSelectList(list string, tempAliases map[string][]SQLResultColumn, realAliases map[string]SQLTableRef, metadata map[string]string) []SQLResultColumn {
	if strings.TrimSpace(list) == "*" {
		return []SQLResultColumn{{Name: "*", Type: "TODO", Expression: "*"}}
	}
	items := splitTopLevelComma(list)
	var columns []SQLResultColumn
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || strings.HasPrefix(item, "@") {
			continue
		}
		name := detectColumnName(item)
		if name == "" {
			name = "TODO"
		}
		columns = append(columns, SQLResultColumn{
			Name:       name,
			Type:       inferSelectItemType(item, tempAliases, realAliases, metadata),
			Expression: normalizeSpaces(item),
		})
	}
	return columns
}

func inferSelectItemType(expr string, tempAliases map[string][]SQLResultColumn, realAliases map[string]SQLTableRef, metadata map[string]string) string {
	expr = expressionWithoutAlias(expr)
	if value := resolveTempColumnType(expr, tempAliases); value != "" {
		return value
	}
	if metadata != nil {
		if value := resolveResultColumnType(expr, realAliases, metadata); value != "" {
			return value
		}
	}
	if value := inferWrapperFunctionType(expr, tempAliases, realAliases, metadata); value != "" {
		return value
	}
	if value := inferCaseType(expr, tempAliases, realAliases, metadata); value != "" {
		return value
	}
	if value := inferArithmeticType(expr, tempAliases, realAliases, metadata); value != "" {
		return value
	}
	return inferExpressionType(expr)
}

func inferWrapperFunctionType(expr string, tempAliases map[string][]SQLResultColumn, realAliases map[string]SQLTableRef, metadata map[string]string) string {
	name, args, ok := functionCall(expr)
	if !ok {
		return ""
	}
	switch strings.ToUpper(name) {
	case "CONCAT", "CONCAT_WS", "FORMAT":
		return "VARCHAR/NVARCHAR"
	case "UPPER", "LOWER", "LTRIM", "RTRIM", "TRIM":
		if len(args) != 1 {
			return ""
		}
		if value := inferSelectItemType(args[0], tempAliases, realAliases, metadata); value != "TODO" {
			return value
		}
		return "VARCHAR/NVARCHAR"
	case "ISNULL", "COALESCE":
		for _, arg := range args {
			value := inferSelectItemType(arg, tempAliases, realAliases, metadata)
			if value != "" && value != "TODO" {
				return value
			}
		}
	case "IIF":
		if len(args) != 3 {
			return ""
		}
		return commonInferredType(args[1:], tempAliases, realAliases, metadata)
	case "SUM":
		if len(args) != 1 {
			return ""
		}
		value := inferSelectItemType(args[0], tempAliases, realAliases, metadata)
		if _, scale, ok := parseDecimalType(value); ok {
			return formatDecimalType(38, scale)
		}
		if value != "" && value != "TODO" {
			return value
		}
	case "MIN", "MAX":
		if len(args) != 1 {
			return ""
		}
		value := inferSelectItemType(args[0], tempAliases, realAliases, metadata)
		if value != "" && value != "TODO" {
			return value
		}
	}
	return ""
}

func detectColumnName(expr string) string {
	asRegex := regexp.MustCompile(`(?is)\s+AS\s+(\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*)\s*$`)
	if match := asRegex.FindStringSubmatch(expr); len(match) > 1 {
		return trimIdentifier(match[1])
	}
	if idx := lastTopLevelWhitespace(expr); idx > 0 {
		tail := strings.TrimSpace(expr[idx:])
		if regexp.MustCompile(`^(\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*)$`).MatchString(tail) {
			return trimIdentifier(tail)
		}
	}
	parts := strings.Split(expr, ".")
	last := strings.TrimSpace(parts[len(parts)-1])
	if regexp.MustCompile(`^(\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*)$`).MatchString(last) {
		return trimIdentifier(last)
	}
	return ""
}

func inferExpressionType(expr string) string {
	expr = expressionWithoutAlias(expr)
	upper := strings.ToUpper(expr)
	if strings.HasPrefix(upper, "CONCAT(") || strings.HasPrefix(upper, "CONCAT_WS(") || strings.HasPrefix(upper, "FORMAT(") {
		return "VARCHAR/NVARCHAR"
	}
	if match := regexp.MustCompile(`(?is)^\s*CAST\s*\(.+\s+AS\s+([A-Za-z0-9_]+(?:\s*\([^)]*\))?)\s*\)\s*$`).FindStringSubmatch(expr); len(match) > 1 {
		return normalizeSpaces(match[1])
	}
	if match := regexp.MustCompile(`(?is)^\s*CONVERT\s*\(\s*([A-Za-z0-9_]+(?:\s*\([^)]*\))?)\s*,`).FindStringSubmatch(expr); len(match) > 1 {
		return normalizeSpaces(match[1])
	}
	if match := regexp.MustCompile(`(?is)^IIF\s*\(.+,\s*(\d+)\s*,\s*(\d+)\s*\)`).FindStringSubmatch(expr); len(match) > 2 {
		return "INT"
	}
	if regexp.MustCompile(`^N?'[^']*'`).MatchString(expr) {
		return "VARCHAR/NVARCHAR"
	}
	if regexp.MustCompile(`^\d+$`).MatchString(expr) {
		return "INT"
	}
	if regexp.MustCompile(`^\d+\.\d+`).MatchString(expr) {
		return "DECIMAL"
	}
	return "TODO"
}

func inferCaseType(expr string, tempAliases map[string][]SQLResultColumn, realAliases map[string]SQLTableRef, metadata map[string]string) string {
	if !regexp.MustCompile(`(?is)^\s*CASE\b`).MatchString(expr) {
		return ""
	}
	return commonInferredType(caseResultExpressions(expr), tempAliases, realAliases, metadata)
}

func caseResultExpressions(expr string) []string {
	upper := strings.ToUpper(expr)
	var values []string
	searchFrom := 0
	for {
		thenPos := strings.Index(upper[searchFrom:], "THEN")
		if thenPos < 0 {
			break
		}
		thenPos += searchFrom
		if !isKeywordAt(upper, thenPos, "THEN") {
			searchFrom = thenPos + len("THEN")
			continue
		}
		start := thenPos + len("THEN")
		end := nextCaseBoundary(upper, start)
		if end < 0 {
			break
		}
		values = append(values, strings.TrimSpace(expr[start:end]))
		searchFrom = end
	}
	if elsePos := strings.Index(upper, "ELSE"); elsePos >= 0 && isKeywordAt(upper, elsePos, "ELSE") {
		start := elsePos + len("ELSE")
		end := nextCaseBoundary(upper, start)
		if end > start {
			values = append(values, strings.TrimSpace(expr[start:end]))
		}
	}
	return values
}

func nextCaseBoundary(upper string, start int) int {
	end := -1
	for _, keyword := range []string{"WHEN", "ELSE", "END"} {
		searchFrom := start
		for {
			pos := strings.Index(upper[searchFrom:], keyword)
			if pos < 0 {
				break
			}
			pos += searchFrom
			if isKeywordAt(upper, pos, keyword) {
				if end < 0 || pos < end {
					end = pos
				}
				break
			}
			searchFrom = pos + len(keyword)
		}
	}
	return end
}

func commonInferredType(values []string, tempAliases map[string][]SQLResultColumn, realAliases map[string]SQLTableRef, metadata map[string]string) string {
	found := ""
	allStrings := len(values) > 0
	for _, value := range values {
		inferred := inferSelectItemType(value, tempAliases, realAliases, metadata)
		if inferred == "" || inferred == "TODO" {
			allStrings = false
			continue
		}
		if inferred != "VARCHAR/NVARCHAR" && !strings.HasPrefix(strings.ToUpper(inferred), "VARCHAR") && !strings.HasPrefix(strings.ToUpper(inferred), "NVARCHAR") {
			allStrings = false
		}
		if found == "" {
			found = inferred
			continue
		}
		if !strings.EqualFold(found, inferred) {
			if allStrings {
				return "VARCHAR/NVARCHAR"
			}
			return found
		}
	}
	if allStrings {
		return "VARCHAR/NVARCHAR"
	}
	return found
}

func inferArithmeticType(expr string, tempAliases map[string][]SQLResultColumn, realAliases map[string]SQLTableRef, metadata map[string]string) string {
	left, right, op, ok := splitTopLevelArithmetic(expr)
	if !ok {
		return ""
	}
	leftType := inferSelectItemType(left, tempAliases, realAliases, metadata)
	rightType := inferSelectItemType(right, tempAliases, realAliases, metadata)
	leftPrecision, leftScale, leftDecimal := parseDecimalType(leftType)
	rightPrecision, rightScale, rightDecimal := parseDecimalType(rightType)
	if leftDecimal || rightDecimal {
		if !leftDecimal {
			leftPrecision, leftScale = numericFallbackPrecisionScale(leftType)
		}
		if !rightDecimal {
			rightPrecision, rightScale = numericFallbackPrecisionScale(rightType)
		}
		switch op {
		case "+", "-":
			scale := maxInt(leftScale, rightScale)
			precision := maxInt(leftPrecision-leftScale, rightPrecision-rightScale) + scale + 1
			if precision > 38 {
				precision = 38
			}
			return formatDecimalType(precision, scale)
		case "*", "/":
			return formatDecimalType(38, maxInt(leftScale, rightScale))
		}
	}
	if isIntegerType(leftType) && isIntegerType(rightType) {
		return "INT"
	}
	if leftType != "" && leftType != "TODO" {
		return leftType
	}
	if rightType != "" && rightType != "TODO" {
		return rightType
	}
	return ""
}

func splitTopLevelArithmetic(expr string) (string, string, string, bool) {
	depth := 0
	inString := false
	for i := len(expr) - 1; i >= 0; i-- {
		ch := expr[i]
		if ch == '\'' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case ')':
			depth++
		case '(':
			if depth > 0 {
				depth--
			}
		case '+', '-', '*', '/':
			if depth == 0 && i > 0 {
				left := strings.TrimSpace(expr[:i])
				right := strings.TrimSpace(expr[i+1:])
				if left != "" && right != "" {
					return left, right, string(ch), true
				}
			}
		}
	}
	return "", "", "", false
}

func parseDecimalType(value string) (int, int, bool) {
	match := regexp.MustCompile(`(?is)^\s*(?:DECIMAL|NUMERIC)\s*\(\s*(\d+)\s*,\s*(\d+)\s*\)\s*$`).FindStringSubmatch(value)
	if len(match) < 3 {
		return 0, 0, false
	}
	precision := atoiDefault(match[1], 18)
	scale := atoiDefault(match[2], 0)
	return precision, scale, true
}

func numericFallbackPrecisionScale(value string) (int, int) {
	if isIntegerType(value) {
		return 10, 0
	}
	return 18, 2
}

func isIntegerType(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "TINYINT", "SMALLINT", "INT", "BIGINT":
		return true
	default:
		return false
	}
}

func formatDecimalType(precision int, scale int) string {
	return "DECIMAL(" + strconv.Itoa(precision) + ", " + strconv.Itoa(scale) + ")"
}

func atoiDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func expressionWithoutAlias(expr string) string {
	expr = strings.TrimSpace(expr)
	asRegex := regexp.MustCompile(`(?is)\s+AS\s+(\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*)\s*$`)
	if loc := asRegex.FindStringIndex(expr); len(loc) > 0 {
		return strings.TrimSpace(expr[:loc[0]])
	}
	if idx := lastTopLevelWhitespace(expr); idx > 0 {
		tail := strings.TrimSpace(expr[idx:])
		if regexp.MustCompile(`^(\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*)$`).MatchString(tail) {
			return strings.TrimSpace(expr[:idx])
		}
	}
	return expr
}

func functionCall(expr string) (string, []string, bool) {
	expr = strings.TrimSpace(expr)
	open := strings.Index(expr, "(")
	if open <= 0 || !strings.HasSuffix(expr, ")") {
		return "", nil, false
	}
	name := strings.TrimSpace(expr[:open])
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(name) {
		return "", nil, false
	}
	argsText := strings.TrimSpace(expr[open+1 : len(expr)-1])
	if argsText == "" {
		return name, nil, true
	}
	return name, splitTopLevelComma(argsText), true
}

func findStatementEnd(s string, start int) int {
	depth := 0
	inString := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if ch == '\'' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				return i
			}
		default:
			if depth == 0 && i > start && isStatementStartAt(s, i) {
				return i
			}
		}
	}
	return len(s)
}

func cteColumnAliases(prefix string, tempTables map[string][]SQLResultColumn, metadata map[string]string) map[string][]SQLResultColumn {
	aliases := map[string][]SQLResultColumn{}
	withStart := findLastTopLevelKeyword(prefix, "WITH")
	if withStart < 0 {
		return aliases
	}
	pos := withStart + len("WITH")
	for pos < len(prefix) {
		pos = skipSpacesAndCommas(prefix, pos)
		name, next, ok := readIdentifier(prefix, pos)
		if !ok {
			break
		}
		pos = next
		pos = skipSpaces(prefix, pos)
		if pos < len(prefix) && prefix[pos] == '(' {
			close := matchingParen(prefix, pos)
			if close < 0 {
				break
			}
			pos = close + 1
			pos = skipSpaces(prefix, pos)
		}
		if !isKeywordAt(prefix, pos, "AS") {
			break
		}
		pos += len("AS")
		pos = skipSpaces(prefix, pos)
		if pos >= len(prefix) || prefix[pos] != '(' {
			break
		}
		close := matchingParen(prefix, pos)
		if close < 0 {
			break
		}
		columns := parseSingleSelectColumns(prefix[pos+1:close], tempTables, metadata)
		if len(columns) > 0 {
			aliases[strings.ToLower(name)] = columns
		}
		pos = close + 1
	}
	return aliases
}

func findLastTopLevelKeyword(s string, keyword string) int {
	last := -1
	searchFrom := 0
	for searchFrom < len(s) {
		found := findTopLevelKeyword(s[searchFrom:], keyword)
		if found < 0 {
			break
		}
		last = searchFrom + found
		searchFrom = last + len(keyword)
	}
	return last
}

func skipSpacesAndCommas(s string, pos int) int {
	for pos < len(s) && (isWhitespaceByte(s[pos]) || s[pos] == ',' || s[pos] == ';') {
		pos++
	}
	return pos
}

func skipSpaces(s string, pos int) int {
	for pos < len(s) && isWhitespaceByte(s[pos]) {
		pos++
	}
	return pos
}

func readIdentifier(s string, pos int) (string, int, bool) {
	if pos >= len(s) {
		return "", pos, false
	}
	if s[pos] == '[' {
		end := strings.IndexByte(s[pos+1:], ']')
		if end < 0 {
			return "", pos, false
		}
		raw := s[pos : pos+end+2]
		return trimIdentifier(raw), pos + end + 2, true
	}
	if !isIdentifierStartByte(s[pos]) {
		return "", pos, false
	}
	start := pos
	for pos < len(s) && isIdentifierPartByte(s[pos]) {
		pos++
	}
	return s[start:pos], pos, true
}

func isIdentifierStartByte(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_'
}

func isIdentifierPartByte(ch byte) bool {
	return isIdentifierStartByte(ch) || (ch >= '0' && ch <= '9')
}

func isStatementStartAt(s string, pos int) bool {
	for _, keyword := range []string{"SELECT", "SET", "RETURN", "IF", "INSERT", "UPDATE", "DELETE", "DECLARE", "END"} {
		if !isKeywordAt(s, pos, keyword) {
			continue
		}
		for j := pos - 1; j >= 0; j-- {
			if s[j] == '\n' || s[j] == '\r' || s[j] == ';' {
				return true
			}
			if !isWhitespaceByte(s[j]) {
				return false
			}
		}
		return true
	}
	return false
}

func isWhitespaceByte(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func tempTableAliases(fromClause string, tempTables map[string][]SQLResultColumn) map[string][]SQLResultColumn {
	aliases := map[string][]SQLResultColumn{}
	tableRegex := regexp.MustCompile(`(?is)\b(?:FROM|JOIN)\s+(#\w+|\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*)(?:\s+(?:AS\s+)?(\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*))?`)
	for _, match := range tableRegex.FindAllStringSubmatch(fromClause, -1) {
		if len(match) < 2 {
			continue
		}
		tableName := strings.ToLower(trimIdentifier(match[1]))
		columns := tempTables[tableName]
		if len(columns) == 0 {
			continue
		}
		aliases[tableName] = columns
		if len(match) > 2 && strings.TrimSpace(match[2]) != "" {
			aliases[strings.ToLower(trimIdentifier(match[2]))] = columns
		}
	}
	return aliases
}

func applyColumnAliases(fromClause string, tempTables map[string][]SQLResultColumn, metadata map[string]string) map[string][]SQLResultColumn {
	aliases := map[string][]SQLResultColumn{}
	applyRegex := regexp.MustCompile(`(?is)\b(?:CROSS|OUTER)\s+APPLY\s*\(`)
	searchFrom := 0
	for {
		loc := applyRegex.FindStringIndex(fromClause[searchFrom:])
		if loc == nil {
			break
		}
		open := searchFrom + loc[1] - 1
		close := matchingParen(fromClause, open)
		if close < 0 {
			break
		}
		aliasMatch := regexp.MustCompile(`(?is)^\s+(?:AS\s+)?(\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*)`).FindStringSubmatch(fromClause[close+1:])
		if len(aliasMatch) > 1 {
			subquery := fromClause[open+1 : close]
			columns := parseSingleSelectColumns(subquery, tempTables, metadata)
			if len(columns) > 0 {
				aliases[strings.ToLower(trimIdentifier(aliasMatch[1]))] = columns
			}
		}
		searchFrom = close + 1
	}
	return aliases
}

func parseSingleSelectColumns(query string, tempTables map[string][]SQLResultColumn, metadata map[string]string) []SQLResultColumn {
	selectStart := findTopLevelKeyword(query, "SELECT")
	if selectStart < 0 {
		return nil
	}
	fromStart := findTopLevelFrom(query, selectStart+len("SELECT"))
	if fromStart < 0 {
		return nil
	}
	list := strings.TrimSpace(query[selectStart+len("SELECT") : fromStart])
	statementEnd := findStatementEnd(query, fromStart)
	fromClause := query[fromStart:statementEnd]
	tempAliases := tempTableAliases(fromClause, tempTables)
	realAliases := tableAliases(fromClause)
	for alias, columns := range applyColumnAliases(fromClause, tempTables, metadata) {
		tempAliases[alias] = columns
	}
	return parseSelectList(list, tempAliases, realAliases, metadata)
}

func matchingParen(s string, open int) int {
	if open < 0 || open >= len(s) || s[open] != '(' {
		return -1
	}
	depth := 0
	inString := false
	for i := open; i < len(s); i++ {
		ch := s[i]
		if ch == '\'' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func resolveTempColumnType(expr string, tempAliases map[string][]SQLResultColumn) string {
	qualifier, column, ok := baseColumnRef(expressionWithoutAlias(expr))
	if !ok {
		return ""
	}
	if qualifier != "" {
		return columnTypeFromTempColumns(tempAliases[strings.ToLower(qualifier)], column)
	}

	found := ""
	for _, columns := range tempAliases {
		value := columnTypeFromTempColumns(columns, column)
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

func columnTypeFromTempColumns(columns []SQLResultColumn, column string) string {
	for _, candidate := range columns {
		if strings.EqualFold(candidate.Name, column) {
			return candidate.Type
		}
	}
	return ""
}

func dedupeResultColumns(results []SQLResultColumn) []SQLResultColumn {
	seen := map[string]bool{}
	var deduped []SQLResultColumn
	for _, result := range results {
		key := strings.ToLower(result.Name + "|" + result.Expression)
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, result)
	}
	return deduped
}
