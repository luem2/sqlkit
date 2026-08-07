package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	EnvLocal      = "local"
	EnvProd       = "prod"
	EnvProdLegacy = "prod-legacy"
	EnvInfra      = "infra"
)

var ValidEnvNames = []string{EnvLocal, EnvProd, EnvProdLegacy, EnvInfra}

type Config struct {
	Root     string
	Tools    map[string]string
	Paths    map[string]string
	Defaults map[string]string
	Envs     map[string]EnvConfig
	Secrets  map[string]string
}

type EnvConfig struct {
	Server                 string `toml:"server"`
	User                   string `toml:"user"`
	PasswordKey            string `toml:"password_key"`
	Encrypt                string `toml:"encrypt"`
	TrustServerCertificate *bool  `toml:"trust_server_certificate"`
}

type SQLConnection struct {
	Server                 string
	User                   string
	Password               string
	Encrypt                string
	TrustServerCertificate bool
}

type SQLConnectionOverrides struct {
	Server   string
	User     string
	Password string
}

func Load(root string) (*Config, error) {
	resolvedRoot, err := resolveRoot(root)
	if err != nil {
		return nil, err
	}

	cfg, err := LoadUserConfig()
	if err != nil {
		return nil, err
	}
	cfg.Root = resolvedRoot

	return cfg, nil
}

func resolveRoot(root string) (string, error) {
	if strings.TrimSpace(root) != "" {
		return filepath.Abs(root)
	}

	global, err := LoadUserConfig()
	if err != nil {
		return "", err
	}
	defaultRepo := strings.TrimSpace(global.Defaults["repo"])
	if defaultRepo == "" {
		return filepath.Abs(".")
	}
	return filepath.Abs(defaultRepo)
}

func UserConfigPath() (string, error) {
	base := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "sqlkit", "config.toml"), nil
}

func LoadUserConfig() (*Config, error) {
	cfg := newConfig()

	path, err := UserConfigPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := parseUserTOML(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func WriteUserDefaultRepo(repo string) (string, error) {
	resolvedRepo, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	cfg, err := LoadUserConfig()
	if err != nil {
		return "", err
	}
	cfg.Defaults["repo"] = resolvedRepo
	return WriteUserConfig(cfg)
}

func WriteUserConfig(cfg *Config) (string, error) {
	path, err := UserConfigPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(formatConfig(cfg)), 0600)
}

func DefaultUserConfig() *Config {
	cfg := newConfig()
	cfg.Paths["project"] = "BD_SISTEMA/BD_SISTEMA.sqlproj"
	cfg.Paths["dacpac"] = "BD_SISTEMA/bin/Debug/BD_SISTEMA.dacpac"
	cfg.Paths["bd_sistema_dacpac"] = "BD_SISTEMA/bin/Debug/BD_SISTEMA.dacpac"
	cfg.Paths["bd_sistema_test_profile"] = "BD_SISTEMA/Profiles/TEST.publish.xml"
	cfg.Paths["bd_sistema_prod_profile"] = "BD_SISTEMA/Profiles/PROD.publish.xml"
	cfg.Paths["grupo_central_dacpac"] = "GRUPO_CENTRAL/bin/Debug/GRUPO_CENTRAL.dacpac"
	cfg.Paths["grupo_central_test_profile"] = "GRUPO_CENTRAL/Profiles/TEST.publish.xml"
	cfg.Paths["grupo_central_prod_profile"] = "GRUPO_CENTRAL/Profiles/PROD.publish.xml"
	cfg.Paths["security_logins_script"] = "_infra/logins/apply.sql"
	cfg.Paths["bd_sistema_security_script"] = "_infra/security/bd-sistema.sql"
	cfg.Paths["grupo_central_security_script"] = "_infra/security/grupo-central.sql"
	cfg.Paths["bd_sistema_bootstrap_script"] = "BD_SISTEMA/bootstrap/bootstrap.sql"
	cfg.Paths["bd_sistema_bootstrap_core_script"] = "BD_SISTEMA/bootstrap/bootstrap-core.sql"
	cfg.Paths["bd_sistema_bootstrap_after_users_script"] = "BD_SISTEMA/bootstrap/bootstrap-after-users.sql"
	cfg.Paths["bd_sistema_seed_manifest"] = "BD_SISTEMA/seeds/data-seeds.manifest.toml"
	cfg.Paths["bd_sistema_migration_manifest"] = "BD_SISTEMA/migration/bd-sistema.toml"
	cfg.Paths["artifacts"] = "artifacts"
	cfg.Paths["docs"] = "docs"
	cfg.Paths["logs"] = "logs"
	cfg.Paths["secretspec"] = "secretspec.toml"
	cfg.Paths["secrets_file"] = ".local/sqlkit/secrets.env"
	cfg.Paths["backups"] = "data/backups"
	cfg.Paths["sqlserver_backup_dir"] = "backups"
	cfg.Paths["sqlserver_container"] = "mssql-db"
	cfg.Paths["sqlserver_data"] = "/var/opt/mssql/data"
	cfg.Paths["bacpacs"] = "data/bacpacs"
	return cfg
}

func NormalizeEnvName(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case EnvLocal, EnvProd, EnvProdLegacy, EnvInfra:
		return normalized, nil
	default:
		return "", fmt.Errorf("environment must be 'local', 'prod', 'prod-legacy' or 'infra'")
	}
}

func (c *Config) LoadSQLConnection(envName string) (*SQLConnection, error) {
	return c.LoadSQLConnectionWithOverrides(envName, SQLConnectionOverrides{})
}

func (c *Config) LoadEnvValues(envName string) (map[string]string, error) {
	values, err := c.loadSQLConnectionValues(envName, true)
	if err != nil {
		return nil, err
	}

	for name, key := range c.Secrets {
		if strings.TrimSpace(key) == "" {
			continue
		}
		secret, err := Secret(key)
		if err != nil {
			return nil, fmt.Errorf("load secret from keyring (%s): %w", key, err)
		}
		values[name] = secret
	}

	return values, nil
}

func LoadNamedSecret(name string) (string, error) {
	secretName := strings.TrimSpace(name)
	if secretName == "" {
		return "", fmt.Errorf("secret name is required")
	}
	cfg, err := LoadUserConfig()
	if err != nil {
		return "", err
	}
	key, ok := cfg.Secrets[secretName]
	if !ok || strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("keyring secret %q is not configured; run sqlkit config secret set %s", secretName, secretName)
	}
	secret, err := Secret(key)
	if err != nil {
		return "", fmt.Errorf("load keyring secret %q: %w", secretName, err)
	}
	return secret, nil
}

func (c *Config) LoadSQLConnectionWithOverrides(envName string, overrides SQLConnectionOverrides) (*SQLConnection, error) {
	values, err := c.loadSQLConnectionValues(envName, strings.TrimSpace(overrides.Password) == "")
	if err != nil {
		return nil, err
	}

	server := firstNonEmpty(overrides.Server, values["SQLSERVER"])
	user := firstNonEmpty(overrides.User, values["SQLUSER"])
	password := firstNonEmpty(overrides.Password, values["SQLPASSWORD"])

	missing := missingNames(map[string]string{
		"SQLSERVER":   server,
		"SQLUSER":     user,
		"SQLPASSWORD": password,
	})
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return &SQLConnection{
		Server:                 server,
		User:                   user,
		Password:               password,
		Encrypt:                connectionEncrypt(values["SQLENCRYPT"]),
		TrustServerCertificate: connectionTrustServerCertificate(values["SQLTRUSTSERVERCERTIFICATE"]),
	}, nil
}

func (c *Config) loadSQLConnectionValues(envName string, loadPasswordFromKeyring bool) (map[string]string, error) {
	normalized, err := NormalizeEnvName(envName)
	if err != nil {
		return nil, err
	}

	values := make(map[string]string)

	if envCfg, ok := c.Envs[normalized]; ok {
		if strings.TrimSpace(envCfg.Server) != "" {
			values["SQLSERVER"] = envCfg.Server
		}
		if strings.TrimSpace(envCfg.User) != "" {
			values["SQLUSER"] = envCfg.User
		}
		if loadPasswordFromKeyring && strings.TrimSpace(envCfg.PasswordKey) != "" {
			password, err := Secret(envCfg.PasswordKey)
			if err != nil {
				return nil, fmt.Errorf("load SQL password from keyring (%s): %w", envCfg.PasswordKey, err)
			}
			values["SQLPASSWORD"] = password
		}
		if strings.TrimSpace(envCfg.Encrypt) != "" {
			encrypt, err := NormalizeEncrypt(envCfg.Encrypt)
			if err != nil {
				return nil, err
			}
			values["SQLENCRYPT"] = encrypt
		}
		if envCfg.TrustServerCertificate != nil {
			values["SQLTRUSTSERVERCERTIFICATE"] = boolString(*envCfg.TrustServerCertificate)
		}
	}

	for _, name := range []string{"SQLSERVER", "SQLUSER", "SQLPASSWORD", "SQLENCRYPT", "SQLTRUSTSERVERCERTIFICATE"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			if name == "SQLENCRYPT" {
				normalized, err := NormalizeEncrypt(value)
				if err != nil {
					return nil, err
				}
				value = normalized
			}
			if name == "SQLTRUSTSERVERCERTIFICATE" {
				normalized, err := normalizeBoolString(value)
				if err != nil {
					return nil, err
				}
				value = normalized
			}
			values[name] = value
		}
	}

	return values, nil
}

func (c *Config) ToolPath(name string) string {
	if value := strings.TrimSpace(c.Tools[name]); value != "" {
		return value
	}
	return name
}

type userConfigFile struct {
	Tools    map[string]toolConfig `toml:"tools"`
	Paths    map[string]string     `toml:"paths"`
	Defaults map[string]string     `toml:"defaults"`
	Secrets  map[string]string     `toml:"secrets"`
	Envs     map[string]EnvConfig  `toml:"env"`
}

type toolConfig struct {
	Path string `toml:"path"`
}

func parseUserTOML(path string, cfg *Config) error {
	var file userConfigFile
	if _, err := toml.DecodeFile(path, &file); err != nil {
		return err
	}

	for name, tool := range file.Tools {
		cfg.Tools[name] = tool.Path
	}
	copyStringMap(cfg.Paths, file.Paths)
	copyStringMap(cfg.Defaults, file.Defaults)
	copyStringMap(cfg.Secrets, file.Secrets)

	for envName, envCfg := range file.Envs {
		normalized, err := NormalizeEnvName(envName)
		if err != nil {
			return err
		}
		cfg.Envs[normalized] = envCfg
	}
	return nil
}

func NormalizeEncrypt(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "disable", "false", "true", "strict":
		return normalized, nil
	default:
		return "", fmt.Errorf("encrypt must be 'disable', 'false', 'true' or 'strict'")
	}
}

func newConfig() *Config {
	return &Config{
		Tools:    make(map[string]string),
		Paths:    make(map[string]string),
		Defaults: make(map[string]string),
		Envs:     make(map[string]EnvConfig),
		Secrets:  make(map[string]string),
	}
}

func copyStringMap(dst map[string]string, src map[string]string) {
	for key, value := range src {
		dst[key] = value
	}
}

func formatConfig(cfg *Config) string {
	var builder strings.Builder
	writeSection(&builder, "defaults", cfg.Defaults)
	writeToolSections(&builder, cfg.Tools)
	writeSection(&builder, "paths", cfg.Paths)
	writeSection(&builder, "secrets", cfg.Secrets)

	envNames := sortedKeysEnv(cfg.Envs)
	for _, envName := range envNames {
		writeEnvSection(&builder, envName, cfg.Envs[envName])
	}
	return builder.String()
}

func writeToolSections(builder *strings.Builder, values map[string]string) {
	for _, name := range sortedKeys(values) {
		writeSection(builder, "tools."+name, map[string]string{"path": values[name]})
	}
}

func writeEnvSection(builder *strings.Builder, envName string, envCfg EnvConfig) {
	values := map[string]string{
		"server":       envCfg.Server,
		"user":         envCfg.User,
		"password_key": envCfg.PasswordKey,
		"encrypt":      envCfg.Encrypt,
	}
	if len(sortedKeys(values)) == 0 && envCfg.TrustServerCertificate == nil {
		return
	}
	writeSectionHeader(builder, "env."+envName)
	for _, key := range sortedKeys(values) {
		builder.WriteString(key)
		builder.WriteString(" = ")
		builder.WriteString(strconv.Quote(values[key]))
		builder.WriteString("\n")
	}
	if envCfg.TrustServerCertificate != nil {
		builder.WriteString("trust_server_certificate = ")
		builder.WriteString(boolString(*envCfg.TrustServerCertificate))
		builder.WriteString("\n")
	}
}

func writeSection(builder *strings.Builder, name string, values map[string]string) {
	keys := sortedKeys(values)
	if len(keys) == 0 {
		return
	}
	writeSectionHeader(builder, name)
	for _, key := range keys {
		if strings.TrimSpace(values[key]) == "" {
			continue
		}
		builder.WriteString(key)
		builder.WriteString(" = ")
		builder.WriteString(strconv.Quote(values[key]))
		builder.WriteString("\n")
	}
}

func writeSectionHeader(builder *strings.Builder, name string) {
	if builder.Len() > 0 {
		builder.WriteString("\n")
	}
	builder.WriteString("[")
	builder.WriteString(name)
	builder.WriteString("]\n")
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key, value := range values {
		if strings.TrimSpace(value) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysEnv(values map[string]EnvConfig) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func missingNames(values map[string]string) []string {
	var missing []string
	for name, value := range values {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func connectionEncrypt(value string) string {
	if strings.TrimSpace(value) == "" {
		return "disable"
	}
	return value
}

func connectionTrustServerCertificate(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	return value == "true"
}

func normalizeBoolString(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "y", "yes", "on":
		return "true", nil
	case "0", "f", "false", "n", "no", "off":
		return "false", nil
	default:
		return "", fmt.Errorf("boolean value must be true or false")
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
