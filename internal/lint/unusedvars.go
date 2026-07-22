package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type VarUsage struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func UnusedVars(path string) ([]VarUsage, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	contentBytes, err := os.ReadFile(absolutePath)
	if err != nil {
		return nil, err
	}
	content := string(contentBytes)

	declRegex := regexp.MustCompile(`(?i)\bDECLARE\s+(@[A-Za-z_][A-Za-z0-9_]*)`)
	matches := declRegex.FindAllStringSubmatch(content, -1)

	declared := make(map[string]string)
	for _, match := range matches {
		if len(match) > 1 {
			key := strings.ToLower(match[1])
			if _, ok := declared[key]; !ok {
				declared[key] = match[1]
			}
		}
	}

	var usages []VarUsage
	for _, name := range declared {
		useRegex := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(name) + `\b`)
		count := len(useRegex.FindAllString(content, -1))
		if count <= 1 {
			usages = append(usages, VarUsage{Name: name, Count: count})
		}
	}

	sort.Slice(usages, func(i, j int) bool {
		return strings.ToLower(usages[i].Name) < strings.ToLower(usages[j].Name)
	})

	return usages, nil
}

func FormatUnused(path string, usages []VarUsage) string {
	if len(usages) == 0 {
		return fmt.Sprintf("No possibly unused variables found in %s.", path)
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "Possibly unused variables in %s:\n", path)
	fmt.Fprintf(&builder, "%-24s %s\n", "Variable", "Uses")
	fmt.Fprintf(&builder, "%s\n", strings.Repeat("-", 34))
	for _, usage := range usages {
		fmt.Fprintf(&builder, "%-24s %d\n", usage.Name, usage.Count)
	}

	return strings.TrimRight(builder.String(), "\n")
}
