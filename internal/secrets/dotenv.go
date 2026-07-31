package secrets

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func LoadDotEnv(path string) (map[string]string, error) {
	values := make(map[string]string)
	if strings.TrimSpace(path) == "" {
		return values, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected KEY=VALUE", path, lineNo)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("%s:%d: empty key", path, lineNo)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if unquoted, err := strconv.Unquote(value); err == nil {
				value = unquoted
			} else if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
				value = strings.TrimSuffix(strings.TrimPrefix(value, "'"), "'")
			}
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}
