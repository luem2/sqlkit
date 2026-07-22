package sqlscripts

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ResolveFile(root string, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("script path is required")
	}

	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(root, resolved)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("script path is a directory: %s", path)
	}
	if strings.ToLower(filepath.Ext(info.Name())) != ".sql" {
		return "", fmt.Errorf("script path must end with .sql: %s", path)
	}

	return filepath.Abs(resolved)
}

func ResolveDir(root string, dir string) ([]string, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("scripts directory is required")
	}

	resolved := dir
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(root, resolved)
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, err
	}

	var scripts []string
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".sql" {
			continue
		}
		scripts = append(scripts, filepath.Join(resolved, entry.Name()))
	}

	sort.Strings(scripts)

	for i, script := range scripts {
		absolute, err := filepath.Abs(script)
		if err != nil {
			return nil, err
		}
		scripts[i] = absolute
	}

	if len(scripts) == 0 {
		return nil, fmt.Errorf("no .sql scripts found in %s", dir)
	}

	return scripts, nil
}
