package dataseed

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Manifest struct {
	Version  int           `toml:"version"`
	Defaults Defaults      `toml:"defaults"`
	Groups   []GroupConfig `toml:"groups"`
}

type Defaults struct {
	SourceEnv string `toml:"source_env"`
}

type GroupConfig struct {
	Name           string        `toml:"name"`
	Description    string        `toml:"description"`
	Scope          string        `toml:"scope"`
	Company        string        `toml:"company"`
	SourceDatabase string        `toml:"source_database"`
	Phase          string        `toml:"phase"`
	Output         string        `toml:"output"`
	Mode           string        `toml:"mode"`
	Retention      string        `toml:"retention"`
	BatchSize      int           `toml:"batch_size"`
	Vigency        VigencyConfig `toml:"vigency"`
	Tables         []TableConfig `toml:"tables"`
}

type VigencyConfig struct {
	FromColumn        string `toml:"from_column"`
	ToColumn          string `toml:"to_column"`
	StateColumn       string `toml:"state_column"`
	CurrentWhere      string `toml:"current_where"`
	CloseUnseededOpen bool   `toml:"close_unseeded_open"`
}

type TableConfig struct {
	Name            string               `toml:"name"`
	Key             []string             `toml:"key"`
	IncludeIdentity bool                 `toml:"include_identity"`
	Where           string               `toml:"where"`
	Parent          string               `toml:"parent"`
	ParentKey       []string             `toml:"parent_key"`
	ForeignKey      []string             `toml:"foreign_key"`
	BatchSize       int                  `toml:"batch_size"`
	ColumnLookups   []ColumnLookupConfig `toml:"column_lookups"`
}

type ColumnLookupConfig struct {
	Column       string `toml:"column"`
	LookupTable  string `toml:"lookup_table"`
	LookupColumn string `toml:"lookup_column"`
	MatchColumn  string `toml:"match_column"`
	MatchValue   string `toml:"match_value"`
	Optional     bool   `toml:"optional"`
}

func LoadManifest(root string, path string) (*Manifest, error) {
	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(root, path)
	}
	var manifest Manifest
	if _, err := toml.DecodeFile(resolved, &manifest); err != nil {
		return nil, err
	}
	if manifest.Version == 0 {
		return nil, fmt.Errorf("manifest version is required")
	}
	return &manifest, nil
}

func (m *Manifest) Group(name string) (GroupConfig, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return GroupConfig{}, fmt.Errorf("group is required")
	}
	for _, group := range m.Groups {
		if group.Name == name {
			if err := validateGroup(group); err != nil {
				return GroupConfig{}, err
			}
			return group, nil
		}
	}
	return GroupConfig{}, fmt.Errorf("manifest group %q not found", name)
}

func FilterGroupTables(group GroupConfig, tableNames []string) (GroupConfig, error) {
	if len(tableNames) == 0 {
		return group, nil
	}
	selected := make(map[string]struct{}, len(tableNames))
	for _, tableName := range tableNames {
		normalized := strings.ToLower(strings.TrimSpace(tableName))
		if normalized == "" {
			return GroupConfig{}, fmt.Errorf("table filter cannot be empty")
		}
		selected[normalized] = struct{}{}
	}

	filtered := group
	filtered.Tables = nil
	known := make(map[string]struct{}, len(group.Tables))
	for _, table := range group.Tables {
		known[strings.ToLower(table.Name)] = struct{}{}
		if _, ok := selected[strings.ToLower(table.Name)]; ok {
			filtered.Tables = append(filtered.Tables, table)
		}
	}
	for tableName := range selected {
		if _, ok := known[tableName]; !ok {
			return GroupConfig{}, fmt.Errorf("table %q not found in group %q", tableName, group.Name)
		}
	}
	if len(filtered.Tables) == 0 {
		return GroupConfig{}, fmt.Errorf("no tables selected for group %q", group.Name)
	}

	included := make(map[string]struct{}, len(filtered.Tables))
	for _, table := range filtered.Tables {
		included[table.Name] = struct{}{}
		if strings.TrimSpace(table.Parent) == "" {
			continue
		}
		if _, ok := included[table.Parent]; !ok {
			return GroupConfig{}, fmt.Errorf("table %q depends on parent %q; include the parent or generate the full group", table.Name, table.Parent)
		}
	}
	if err := validateGroup(filtered); err != nil {
		return GroupConfig{}, err
	}
	return filtered, nil
}

func validateGroup(group GroupConfig) error {
	if strings.TrimSpace(group.Name) == "" {
		return fmt.Errorf("group name is required")
	}
	if strings.TrimSpace(group.SourceDatabase) == "" {
		return fmt.Errorf("group %q source_database is required", group.Name)
	}
	if strings.TrimSpace(group.Output) == "" {
		return fmt.Errorf("group %q output is required", group.Name)
	}
	switch strings.TrimSpace(group.Phase) {
	case "", "postdeploy", "bootstrap", "bootstrap-sensitive":
	default:
		return fmt.Errorf("group %q phase must be postdeploy, bootstrap or bootstrap-sensitive", group.Name)
	}
	switch strings.TrimSpace(group.Mode) {
	case "merge", "replace":
	default:
		return fmt.Errorf("group %q mode must be merge or replace", group.Name)
	}
	if len(group.Tables) == 0 {
		return fmt.Errorf("group %q must include at least one table", group.Name)
	}
	names := make(map[string]struct{})
	for _, table := range group.Tables {
		if strings.TrimSpace(table.Name) == "" {
			return fmt.Errorf("group %q contains a table without name", group.Name)
		}
		if len(table.Key) == 0 {
			return fmt.Errorf("table %q key is required", table.Name)
		}
		if table.BatchSize < 0 {
			return fmt.Errorf("table %q batch_size cannot be negative", table.Name)
		}
		if strings.TrimSpace(table.Parent) != "" {
			if len(table.ParentKey) == 0 || len(table.ForeignKey) == 0 || len(table.ParentKey) != len(table.ForeignKey) {
				return fmt.Errorf("table %q parent_key and foreign_key must have the same length", table.Name)
			}
			if _, ok := names[table.Parent]; !ok {
				return fmt.Errorf("table %q parent %q must appear before child", table.Name, table.Parent)
			}
		}
		for _, lookup := range table.ColumnLookups {
			if strings.TrimSpace(lookup.Column) == "" {
				return fmt.Errorf("table %q contains a column_lookup without column", table.Name)
			}
			if strings.TrimSpace(lookup.LookupTable) == "" {
				return fmt.Errorf("table %q column_lookup %q lookup_table is required", table.Name, lookup.Column)
			}
			if strings.TrimSpace(lookup.LookupColumn) == "" {
				return fmt.Errorf("table %q column_lookup %q lookup_column is required", table.Name, lookup.Column)
			}
			if strings.TrimSpace(lookup.MatchColumn) == "" {
				return fmt.Errorf("table %q column_lookup %q match_column is required", table.Name, lookup.Column)
			}
			if strings.TrimSpace(lookup.MatchValue) == "" {
				return fmt.Errorf("table %q column_lookup %q match_value is required", table.Name, lookup.Column)
			}
		}
		names[table.Name] = struct{}{}
	}
	if group.BatchSize < 0 {
		return fmt.Errorf("group %q batch_size cannot be negative", group.Name)
	}
	return nil
}
