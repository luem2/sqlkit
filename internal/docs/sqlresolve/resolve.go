package sqlresolve

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/luem2/sqlkit/internal/docs"
	"github.com/luem2/sqlkit/internal/sqlscripts"
)

func Resolve(root string, input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("SQL procedure path or name is required")
	}

	if resolved, err := sqlscripts.ResolveFile(root, input); err == nil {
		return resolved, nil
	}
	if filepath.IsAbs(input) || strings.ContainsAny(input, `/\`) {
		return "", fmt.Errorf("SQL procedure file not found: %s", input)
	}

	matches, err := findProcedureFiles(root, input)
	if err != nil {
		return "", err
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("SQL procedure %q not found under %s", input, root)
	case 1:
		return matches[0], nil
	default:
		relative := make([]string, 0, len(matches))
		for _, match := range matches {
			if rel, err := filepath.Rel(root, match); err == nil {
				relative = append(relative, rel)
			} else {
				relative = append(relative, match)
			}
		}
		sort.Strings(relative)
		return "", fmt.Errorf("SQL procedure %q matched multiple files under %s:\n%s", input, root, strings.Join(relative, "\n"))
	}
}

func findProcedureFiles(root string, name string) ([]string, error) {
	expectedFile := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)) + ".sql")
	expectedProcedure := normalizeProcedureName(strings.TrimSuffix(name, filepath.Ext(name)))

	var filenameMatches []string
	var contentMatches []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch strings.ToLower(entry.Name()) {
			case ".git", "bin", "obj", "generated", "artifacts":
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if strings.ToLower(filepath.Ext(entry.Name())) != ".sql" {
			return nil
		}
		if strings.EqualFold(entry.Name(), expectedFile) {
			absolute, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			filenameMatches = append(filenameMatches, absolute)
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		procedureName := procedureName(string(content))
		if procedureName == expectedProcedure || strings.HasSuffix(procedureName, "."+expectedProcedure) {
			absolute, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			contentMatches = append(contentMatches, absolute)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(filenameMatches) > 0 {
		sort.Strings(filenameMatches)
		return filenameMatches, nil
	}
	sort.Strings(contentMatches)
	return contentMatches, nil
}

func procedureName(content string) string {
	doc := docs.AnalyzeSQLProcedureFromText(content, "", 0)
	return normalizeProcedureName(doc.Name)
}

func normalizeProcedureName(name string) string {
	name = strings.ReplaceAll(name, "].[", ".")
	name = strings.ReplaceAll(name, "[", "")
	name = strings.ReplaceAll(name, "]", "")
	return strings.ToLower(strings.TrimSpace(name))
}
