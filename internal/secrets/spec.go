package secrets

import (
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

var currentSQLKitNamePattern = regexp.MustCompile(`(?i)current sqlkit secret name:\s*([a-z0-9][a-z0-9-]*)`)

type Spec struct {
	Project  map[string]string       `toml:"project"`
	Profiles map[string]ProfileSpec  `toml:"profiles"`
	byName   map[string][]SecretSpec `toml:"-"`
}

type ProfileSpec map[string]SecretSpec

type SecretSpec struct {
	Name        string `toml:"-"`
	Profile     string `toml:"-"`
	Description string `toml:"description"`
	Required    bool   `toml:"required"`
	SQLKitName  string `toml:"sqlkit_name"`
	Env         string `toml:"env"`
}

func LoadSpec(path string) (*Spec, error) {
	var spec Spec
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	if _, err := toml.DecodeFile(path, &spec); err != nil {
		return nil, err
	}
	spec.index()
	return &spec, nil
}

func (s *Spec) EnvNames(secretName string) []string {
	if s == nil {
		return nil
	}
	normalized := normalizeName(secretName)
	var values []string
	seen := make(map[string]bool)
	for _, item := range s.byName[normalized] {
		envName := item.Env
		if strings.TrimSpace(envName) == "" {
			envName = item.Name
		}
		if strings.TrimSpace(envName) == "" || seen[envName] {
			continue
		}
		seen[envName] = true
		values = append(values, envName)
	}
	sort.Strings(values)
	return values
}

func (s *Spec) Required(profile string) []SecretSpec {
	if s == nil {
		return nil
	}
	profile = strings.TrimSpace(profile)
	merged := make(map[string]SecretSpec)
	for _, profileName := range []string{"default", profile} {
		if strings.TrimSpace(profileName) == "" {
			continue
		}
		for name, item := range s.Profiles[profileName] {
			if base, ok := merged[name]; ok {
				item = mergeSecretSpec(base, item)
			}
			item.Name = name
			item.Profile = profileName
			merged[name] = item
		}
	}
	var values []SecretSpec
	for _, item := range merged {
		if item.Required {
			values = append(values, item)
		}
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].Name < values[j].Name
	})
	return values
}

func mergeSecretSpec(base SecretSpec, override SecretSpec) SecretSpec {
	if strings.TrimSpace(override.Description) == "" {
		override.Description = base.Description
	}
	if strings.TrimSpace(override.SQLKitName) == "" {
		override.SQLKitName = base.SQLKitName
	}
	if strings.TrimSpace(override.Env) == "" {
		override.Env = base.Env
	}
	return override
}

func (s *Spec) All() []SecretSpec {
	if s == nil {
		return nil
	}
	var values []SecretSpec
	for profileName, profile := range s.Profiles {
		for name, item := range profile {
			item.Name = name
			item.Profile = profileName
			values = append(values, item)
		}
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Profile == values[j].Profile {
			return values[i].Name < values[j].Name
		}
		return values[i].Profile < values[j].Profile
	})
	return values
}

func (s *Spec) index() {
	s.byName = make(map[string][]SecretSpec)
	for profileName, profile := range s.Profiles {
		for name, item := range profile {
			item.Name = name
			item.Profile = profileName
			for _, alias := range item.aliases() {
				normalized := normalizeName(alias)
				if normalized == "" {
					continue
				}
				s.byName[normalized] = append(s.byName[normalized], item)
			}
		}
	}
}

func (s SecretSpec) aliases() []string {
	aliases := []string{s.Name, s.SQLKitName}
	if match := currentSQLKitNamePattern.FindStringSubmatch(s.Description); len(match) == 2 {
		aliases = append(aliases, match[1])
	}
	return aliases
}

func normalizeName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
