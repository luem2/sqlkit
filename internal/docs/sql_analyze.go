package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	procedureNameRegex = regexp.MustCompile(`(?is)\bCREATE\s+(?:OR\s+ALTER\s+)?PROCEDURE\s+((?:\[[^\]]+\]|\w+)(?:\s*\.\s*(?:\[[^\]]+\]|\w+))?)`)
	procedureASRegex   = regexp.MustCompile(`(?im)^\s*AS\s*$`)
	parameterRegex     = regexp.MustCompile(`(?is)^\s*,?\s*(@[A-Za-z_][A-Za-z0-9_]*)\s+(.+?)\s*$`)
	typeBlockRegex     = regexp.MustCompile(`(?im)\bIF\s*\(\s*@iType\s*=\s*(\d+)\s*\)\s*(?:--\s*(.*))?$`)
	variableRegex      = regexp.MustCompile(`@[A-Za-z_][A-Za-z0-9_]*`)
)

func AnalyzeSQLProcedure(path string, onlyIType int) (*SQLProcedureDoc, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("source sql is required")
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	contentBytes, err := os.ReadFile(absolutePath)
	if err != nil {
		return nil, err
	}
	return AnalyzeSQLProcedureFromText(string(contentBytes), absolutePath, onlyIType), nil
}

func AnalyzeSQLProcedureFromText(content string, sourcePath string, onlyIType int) *SQLProcedureDoc {
	doc := &SQLProcedureDoc{
		SourcePath: sourcePath,
		Name:       detectProcedureName(content, sourcePath),
		Parameters: parseProcedureParameters(content),
	}
	doc.Blocks = parseTypeBlocks(content, doc.Parameters, onlyIType)
	return doc
}

func detectProcedureName(content string, path string) string {
	match := procedureNameRegex.FindStringSubmatch(content)
	if len(match) > 1 {
		return strings.Join(strings.Fields(match[1]), "")
	}
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func parseProcedureParameters(content string) []SQLParameter {
	nameMatch := procedureNameRegex.FindStringSubmatchIndex(content)
	if len(nameMatch) == 0 {
		return nil
	}
	asMatch := procedureASRegex.FindStringIndex(content[nameMatch[1]:])
	if len(asMatch) == 0 {
		return nil
	}
	signature := content[nameMatch[1] : nameMatch[1]+asMatch[0]]

	var params []SQLParameter
	for _, line := range strings.Split(signature, "\n") {
		clean, comment := splitSQLLineComment(line)
		param := parseParameterLine(clean)
		if param.Name == "" {
			continue
		}
		param.Observation = strings.TrimSpace(comment)
		params = append(params, param)
	}
	return params
}

func parseParameterLine(line string) SQLParameter {
	match := parameterRegex.FindStringSubmatch(strings.TrimSpace(line))
	if len(match) < 3 {
		return SQLParameter{}
	}

	name := strings.TrimSpace(match[1])
	rest := strings.TrimSpace(strings.TrimRight(match[2], ","))
	output := regexp.MustCompile(`(?i)\bOUTPUT\b`).MatchString(rest)
	rest = regexp.MustCompile(`(?i)\bOUTPUT\b`).ReplaceAllString(rest, "")
	typePart := strings.TrimSpace(rest)
	defaultPart := ""
	if eq := strings.Index(typePart, "="); eq >= 0 {
		defaultPart = strings.TrimSpace(typePart[eq+1:])
		typePart = strings.TrimSpace(typePart[:eq])
	}

	return SQLParameter{
		Name:     name,
		Type:     normalizeSpaces(typePart),
		Default:  normalizeSpaces(defaultPart),
		Output:   output,
		Required: strings.TrimSpace(defaultPart) == "" && !output,
	}
}

func parseTypeBlocks(content string, parameters []SQLParameter, onlyIType int) []SQLTypeBlock {
	matches := typeBlockRegex.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}

	var blocks []SQLTypeBlock
	for i, match := range matches {
		typeText := content[match[2]:match[3]]
		typeValue, err := strconv.Atoi(typeText)
		if err != nil {
			continue
		}
		if onlyIType > 0 && typeValue != onlyIType {
			continue
		}

		end := len(content)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		description := ""
		if match[4] >= 0 && match[5] >= 0 {
			description = strings.TrimSpace(content[match[4]:match[5]])
		}
		body := content[match[1]:end]
		prelude := content[:match[0]]
		used := parametersUsedInBody(body, prelude, parameters)
		outputs := outputsUsedInBody(body, parameters)
		resultSets := parseResultSets(body)

		blocks = append(blocks, SQLTypeBlock{
			Type:        typeValue,
			Description: description,
			Body:        body,
			Parameters:  used,
			Outputs:     outputs,
			ResultSets:  resultSets,
		})
	}

	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Type < blocks[j].Type
	})
	return blocks
}

func parametersUsedInBody(body string, prelude string, parameters []SQLParameter) []SQLParameter {
	var used []SQLParameter
	variables := variablesInBody(body)
	for _, param := range parameters {
		if param.Output {
			continue
		}
		directlyUsed := variables[strings.ToLower(param.Name)]
		indirectlyUsed := parameterFeedsUsedVariable(prelude, variables, param.Name)
		if strings.EqualFold(param.Name, "@iType") || directlyUsed || indirectlyUsed {
			param.Required = isRequiredParameterInBlock(body, param)
			used = append(used, param)
		}
	}
	return used
}

func outputsUsedInBody(body string, parameters []SQLParameter) []SQLParameter {
	var outputs []SQLParameter
	variables := variablesInBody(body)
	for _, param := range parameters {
		if !param.Output {
			continue
		}
		if variables[strings.ToLower(param.Name)] {
			outputs = append(outputs, param)
		}
	}
	return outputs
}

func variablesInBody(body string) map[string]bool {
	variables := map[string]bool{}
	for _, name := range variableRegex.FindAllString(stripSQLComments(body), -1) {
		variables[strings.ToLower(name)] = true
	}
	return variables
}

func parameterFeedsUsedVariable(prelude string, blockVariables map[string]bool, paramName string) bool {
	clean := stripSQLComments(prelude)
	assignRegex := regexp.MustCompile(`(?im)\b(?:DECLARE|SET)\s+(@[A-Za-z_][A-Za-z0-9_]*)\b[^\n=]*=\s*(.*)$`)
	for _, match := range assignRegex.FindAllStringSubmatch(clean, -1) {
		if len(match) < 3 {
			continue
		}
		assigned := strings.ToLower(match[1])
		expression := strings.ToLower(match[2])
		if blockVariables[assigned] && variableMentioned(expression, paramName) {
			return true
		}
	}
	return false
}

func variableMentioned(text string, name string) bool {
	for _, variable := range variableRegex.FindAllString(text, -1) {
		if strings.EqualFold(variable, name) {
			return true
		}
	}
	return false
}

func isRequiredParameterInBlock(body string, param SQLParameter) bool {
	if strings.EqualFold(param.Name, "@iType") {
		return true
	}
	if param.Default != "" && !strings.EqualFold(param.Default, "NULL") {
		return false
	}

	lines := strings.Split(stripSQLComments(body), "\n")
	name := strings.ToLower(param.Name)
	hasOptionalFilter := hasOptionalNullFilter(body, name)
	optionalDepth := -1
	depth := 0
	pendingOptionalGuard := false

	for _, line := range lines {
		lower := strings.ToLower(line)
		hasParam := strings.Contains(lower, name)
		if hasParam && hasOptionalFilter && isOptionalFilterHelperLine(lower, name) {
			continue
		}

		if hasParam && isOptionalGuardLine(lower, name) {
			pendingOptionalGuard = true
		}

		if hasParam && optionalDepth < 0 && !isOptionalUsageLine(lower, name) {
			return true
		}

		beginCount := keywordCount(lower, "begin")
		endCount := keywordCount(lower, "end")
		if pendingOptionalGuard && beginCount > 0 {
			optionalDepth = depth + beginCount
			pendingOptionalGuard = false
		}
		depth += beginCount
		depth -= endCount
		if optionalDepth >= 0 && depth < optionalDepth {
			optionalDepth = -1
		}
	}

	return param.Default == ""
}

func hasOptionalNullFilter(body string, name string) bool {
	clean := strings.ToLower(stripSQLComments(body))
	return strings.Contains(clean, name+" is null or") ||
		strings.Contains(clean, "or "+name+" is null")
}

func isOptionalFilterHelperLine(line string, name string) bool {
	return strings.Contains(line, "string_split("+name)
}

func isOptionalGuardLine(line string, name string) bool {
	return strings.Contains(line, name+" is not null") ||
		strings.Contains(line, name+" <> ''") ||
		strings.Contains(line, "nullif("+name)
}

func isOptionalUsageLine(line string, name string) bool {
	return isOptionalGuardLine(line, name) ||
		strings.Contains(line, name+" is null or") ||
		strings.Contains(line, "or "+name+" is null") ||
		strings.Contains(line, "coalesce("+name) ||
		strings.Contains(line, "isnull("+name) ||
		strings.Contains(line, "nullif("+name)
}
