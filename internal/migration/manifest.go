package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

type Manifest struct {
	Version     int                      `toml:"version"`
	Name        string                   `toml:"name"`
	Description string                   `toml:"description"`
	Companies   map[string]CompanyConfig `toml:"companies"`
	Steps       []Step                   `toml:"steps"`
}

type CompanyConfig struct {
	IDEmpresa int `toml:"id_empresa"`
}

type Step struct {
	Name        string `toml:"name"`
	Order       int    `toml:"order"`
	Description string `toml:"description"`
	Script      string `toml:"script"`
}

func Load(path string) (*Manifest, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("migration manifest path is required")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	var manifest Manifest
	if _, err := toml.DecodeFile(path, &manifest); err != nil {
		return nil, err
	}
	if err := manifest.validate(); err != nil {
		return nil, err
	}
	sort.SliceStable(manifest.Steps, func(i, j int) bool {
		return manifest.Steps[i].Order < manifest.Steps[j].Order
	})
	return &manifest, nil
}

func (m *Manifest) CompanyID(company string) (int, bool) {
	normalized := strings.ToUpper(strings.TrimSpace(company))
	for code, cfg := range m.Companies {
		if strings.ToUpper(strings.TrimSpace(code)) == normalized && cfg.IDEmpresa > 0 {
			return cfg.IDEmpresa, true
		}
	}
	return 0, false
}

func (m *Manifest) SelectSteps(stepName string, from string, to string, all bool) ([]Step, error) {
	selectedModes := 0
	if strings.TrimSpace(stepName) != "" {
		selectedModes++
	}
	if strings.TrimSpace(from) != "" || strings.TrimSpace(to) != "" {
		selectedModes++
	}
	if all {
		selectedModes++
	}
	if selectedModes != 1 {
		return nil, fmt.Errorf("select exactly one of --step, --from/--to or --all")
	}
	if all {
		return append([]Step(nil), m.Steps...), nil
	}
	if strings.TrimSpace(stepName) != "" {
		step, ok := m.findStep(stepName)
		if !ok {
			return nil, fmt.Errorf("migration step not found: %s", stepName)
		}
		return []Step{step}, nil
	}

	fromIndex := 0
	toIndex := len(m.Steps) - 1
	if strings.TrimSpace(from) != "" {
		index, ok := m.findStepIndex(from)
		if !ok {
			return nil, fmt.Errorf("migration --from step not found: %s", from)
		}
		fromIndex = index
	}
	if strings.TrimSpace(to) != "" {
		index, ok := m.findStepIndex(to)
		if !ok {
			return nil, fmt.Errorf("migration --to step not found: %s", to)
		}
		toIndex = index
	}
	if fromIndex > toIndex {
		return nil, fmt.Errorf("--from must be before or equal to --to")
	}
	return append([]Step(nil), m.Steps[fromIndex:toIndex+1]...), nil
}

func ResolveStepScripts(root string, steps []Step) ([]string, error) {
	scripts := make([]string, 0, len(steps))
	for _, step := range steps {
		script := strings.TrimSpace(step.Script)
		if script == "" {
			return nil, fmt.Errorf("migration step %q has empty script", step.Name)
		}
		if !filepath.IsAbs(script) {
			script = filepath.Join(root, script)
		}
		info, err := os.Stat(script)
		if err != nil {
			return nil, fmt.Errorf("migration step %q script not found: %w", step.Name, err)
		}
		if info.IsDir() || strings.ToLower(filepath.Ext(info.Name())) != ".sql" {
			return nil, fmt.Errorf("migration step %q script must be a .sql file", step.Name)
		}
		absolute, err := filepath.Abs(script)
		if err != nil {
			return nil, err
		}
		scripts = append(scripts, absolute)
	}
	return scripts, nil
}

func (m *Manifest) validate() error {
	if m.Version <= 0 {
		return fmt.Errorf("migration manifest version is required")
	}
	if len(m.Steps) == 0 {
		return fmt.Errorf("migration manifest must define at least one step")
	}

	names := make(map[string]bool, len(m.Steps))
	orders := make(map[int]bool, len(m.Steps))
	for _, step := range m.Steps {
		name := strings.TrimSpace(step.Name)
		if name == "" {
			return fmt.Errorf("migration step name is required")
		}
		key := strings.ToLower(name)
		if names[key] {
			return fmt.Errorf("duplicate migration step name: %s", name)
		}
		names[key] = true
		if step.Order <= 0 {
			return fmt.Errorf("migration step %q order must be positive", name)
		}
		if orders[step.Order] {
			return fmt.Errorf("duplicate migration step order: %d", step.Order)
		}
		orders[step.Order] = true
		if strings.TrimSpace(step.Script) == "" {
			return fmt.Errorf("migration step %q script is required", name)
		}
	}
	return nil
}

func (m *Manifest) findStep(name string) (Step, bool) {
	index, ok := m.findStepIndex(name)
	if !ok {
		return Step{}, false
	}
	return m.Steps[index], true
}

func (m *Manifest) findStepIndex(name string) (int, bool) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	for i, step := range m.Steps {
		if strings.ToLower(strings.TrimSpace(step.Name)) == normalized {
			return i, true
		}
	}
	return 0, false
}
