package sqlserver

import (
	"fmt"
	"strings"
)

var systemDatabases = map[string]struct{}{
	"master": {},
	"model":  {},
	"msdb":   {},
	"tempdb": {},
}

func ValidateDatabaseName(database string) error {
	database = strings.TrimSpace(database)
	if database == "" {
		return fmt.Errorf("database is required")
	}
	if strings.Contains(database, "\x00") {
		return fmt.Errorf("database contains invalid null byte")
	}
	return nil
}

func ValidateUserDatabaseName(database string) error {
	if err := ValidateDatabaseName(database); err != nil {
		return err
	}
	if _, ok := systemDatabases[strings.ToLower(database)]; ok {
		return fmt.Errorf("refusing to operate on system database %q", database)
	}
	return nil
}

func ValidateIdentifierLabel(label string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", label)
	}
	if strings.Contains(value, "\x00") {
		return fmt.Errorf("%s contains invalid null byte", label)
	}
	return nil
}

func ParseSchemaTable(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", fmt.Errorf("table is required")
	}
	parts := strings.Split(value, ".")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("table must use schema.table format")
	}
	for _, part := range parts {
		if strings.Contains(part, "\x00") {
			return "", "", fmt.Errorf("table contains invalid null byte")
		}
	}
	return strings.Trim(parts[0], "[] "), strings.Trim(parts[1], "[] "), nil
}
