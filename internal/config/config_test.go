package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesUserDefaultRepo(t *testing.T) {
	isolateUserConfig(t)
	repo := filepath.Join(t.TempDir(), "sql")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteUserDefaultRepo(repo); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Root != repo {
		t.Fatalf("Root = %q, want %q", cfg.Root, repo)
	}
}

func TestLoadUsesExplicitRepoBeforeUserDefault(t *testing.T) {
	isolateUserConfig(t)
	defaultRepo := filepath.Join(t.TempDir(), "default")
	explicitRepo := filepath.Join(t.TempDir(), "explicit")
	if err := os.MkdirAll(defaultRepo, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(explicitRepo, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteUserDefaultRepo(defaultRepo); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(explicitRepo)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Root != explicitRepo {
		t.Fatalf("Root = %q, want %q", cfg.Root, explicitRepo)
	}
}

func TestLoadSQLConnectionUsesUserConfigAndKeyring(t *testing.T) {
	isolateUserConfig(t)
	previousGet := keyringGet
	keyringGet = func(service string, user string) (string, error) {
		if service != keyringService || user != "env/local/password" {
			t.Fatalf("unexpected keyring lookup %s/%s", service, user)
		}
		return "keyring-password", nil
	}
	t.Cleanup(func() {
		keyringGet = previousGet
	})

	dir := t.TempDir()
	if _, err := WriteUserConfig(&Config{
		Tools:    map[string]string{},
		Paths:    map[string]string{},
		Defaults: map[string]string{"repo": dir},
		Envs: map[string]EnvConfig{
			EnvLocal: {
				Server:      "user-server",
				User:        "user-name",
				PasswordKey: "env/local/password",
			},
		},
		Secrets: map[string]string{},
	}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := cfg.LoadSQLConnection(EnvLocal)
	if err != nil {
		t.Fatal(err)
	}
	if conn.Server != "user-server" || conn.User != "user-name" || conn.Password != "keyring-password" {
		t.Fatalf("unexpected connection: %#v", conn)
	}
}

func TestNormalizeEnvNameAcceptsInfra(t *testing.T) {
	got, err := NormalizeEnvName(" INFRA ")
	if err != nil {
		t.Fatal(err)
	}
	if got != EnvInfra {
		t.Fatalf("NormalizeEnvName = %q, want %q", got, EnvInfra)
	}
}

func TestLoadSQLConnectionUsesExplicitOverridesFirst(t *testing.T) {
	isolateUserConfig(t)
	dir := t.TempDir()
	if _, err := WriteUserConfig(&Config{
		Tools:    map[string]string{},
		Paths:    map[string]string{},
		Defaults: map[string]string{"repo": dir},
		Envs: map[string]EnvConfig{
			EnvLocal: {
				Server:      "user-server",
				User:        "user-name",
				PasswordKey: "env/local/password",
			},
		},
		Secrets: map[string]string{},
	}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := cfg.LoadSQLConnectionWithOverrides(EnvLocal, SQLConnectionOverrides{
		Server:   "flag-server",
		User:     "flag-user",
		Password: "flag-password",
	})
	if err != nil {
		t.Fatal(err)
	}
	if conn.Server != "flag-server" || conn.User != "flag-user" || conn.Password != "flag-password" {
		t.Fatalf("unexpected connection: %#v", conn)
	}
}

func TestLoadSQLConnectionPasswordOverrideSkipsKeyring(t *testing.T) {
	isolateUserConfig(t)
	previousGet := keyringGet
	keyringGet = func(service string, user string) (string, error) {
		t.Fatal("keyring must not be called when password override is provided")
		return "", nil
	}
	t.Cleanup(func() {
		keyringGet = previousGet
	})

	dir := t.TempDir()
	if _, err := WriteUserConfig(&Config{
		Tools:    map[string]string{},
		Paths:    map[string]string{},
		Defaults: map[string]string{"repo": dir},
		Envs: map[string]EnvConfig{
			EnvProd: {
				Server:      "sql.example",
				User:        "backup-user",
				PasswordKey: "env/prod/password",
			},
		},
		Secrets: map[string]string{},
	}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := cfg.LoadSQLConnectionWithOverrides(EnvProd, SQLConnectionOverrides{Password: "credential-password"})
	if err != nil {
		t.Fatal(err)
	}
	if conn.Password != "credential-password" {
		t.Fatalf("Password = %q, want credential-password", conn.Password)
	}
}

func TestLoadEnvValuesLoadsGenericSecretsFromKeyring(t *testing.T) {
	isolateUserConfig(t)
	previousGet := keyringGet
	keyringGet = func(service string, user string) (string, error) {
		if service != keyringService || user != "secret/DBA_PWD" {
			t.Fatalf("unexpected keyring lookup %s/%s", service, user)
		}
		return "dba-password", nil
	}
	t.Cleanup(func() {
		keyringGet = previousGet
	})

	dir := t.TempDir()
	if _, err := WriteUserConfig(&Config{
		Tools:    map[string]string{},
		Paths:    map[string]string{},
		Defaults: map[string]string{"repo": dir},
		Envs:     map[string]EnvConfig{},
		Secrets:  map[string]string{"DBA_PWD": "secret/DBA_PWD"},
	}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	values, err := cfg.LoadEnvValues(EnvLocal)
	if err != nil {
		t.Fatal(err)
	}
	if values["DBA_PWD"] != "dba-password" {
		t.Fatalf("DBA_PWD = %q, want dba-password", values["DBA_PWD"])
	}
}

func TestLoadNamedSecretUsesConfiguredKeyringSecret(t *testing.T) {
	isolateUserConfig(t)
	previousGet := keyringGet
	keyringGet = func(service string, user string) (string, error) {
		if service != keyringService || user != "secret/telegram-bot-token" {
			t.Fatalf("unexpected keyring lookup %s/%s", service, user)
		}
		return "bot-token", nil
	}
	t.Cleanup(func() {
		keyringGet = previousGet
	})

	dir := t.TempDir()
	if _, err := WriteUserConfig(&Config{
		Tools:    map[string]string{},
		Paths:    map[string]string{},
		Defaults: map[string]string{"repo": dir},
		Envs:     map[string]EnvConfig{},
		Secrets:  map[string]string{"telegram-bot-token": "secret/telegram-bot-token"},
	}); err != nil {
		t.Fatal(err)
	}

	secret, err := LoadNamedSecret("telegram-bot-token")
	if err != nil {
		t.Fatal(err)
	}
	if secret != "bot-token" {
		t.Fatalf("LoadNamedSecret = %q, want bot-token", secret)
	}
}

func TestLoadSQLConnectionDoesNotLoadGenericSecrets(t *testing.T) {
	isolateUserConfig(t)
	previousGet := keyringGet
	keyringGet = func(service string, user string) (string, error) {
		if service != keyringService {
			t.Fatalf("unexpected keyring service %s", service)
		}
		if user == "env/local/password" {
			return "keyring-password", nil
		}
		t.Fatalf("unexpected generic secret lookup %s", user)
		return "", nil
	}
	t.Cleanup(func() {
		keyringGet = previousGet
	})

	dir := t.TempDir()
	if _, err := WriteUserConfig(&Config{
		Tools:    map[string]string{},
		Paths:    map[string]string{},
		Defaults: map[string]string{"repo": dir},
		Envs: map[string]EnvConfig{
			EnvLocal: {
				Server:      "user-server",
				User:        "user-name",
				PasswordKey: "env/local/password",
			},
		},
		Secrets: map[string]string{"DBA_PWD": "secret/DBA_PWD"},
	}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := cfg.LoadSQLConnection(EnvLocal)
	if err != nil {
		t.Fatal(err)
	}
	if conn.Password != "keyring-password" {
		t.Fatalf("Password = %q, want keyring-password", conn.Password)
	}
}

func TestLoadSQLConnectionUsesTLSConfig(t *testing.T) {
	isolateUserConfig(t)
	previousGet := keyringGet
	keyringGet = func(service string, user string) (string, error) {
		if service != keyringService || user != "env/local/password" {
			t.Fatalf("unexpected keyring lookup %s/%s", service, user)
		}
		return "keyring-password", nil
	}
	t.Cleanup(func() {
		keyringGet = previousGet
	})

	path, err := UserConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	content := `
[env.local]
server = "localhost"
user = "sa"
password_key = "env/local/password"
encrypt = "true"
trust_server_certificate = true
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadUserConfig()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := cfg.LoadSQLConnection(EnvLocal)
	if err != nil {
		t.Fatal(err)
	}
	if conn.Encrypt != "true" {
		t.Fatalf("Encrypt = %q, want true", conn.Encrypt)
	}
	if !conn.TrustServerCertificate {
		t.Fatal("TrustServerCertificate = false, want true")
	}
}

func TestLoadUserConfigSupportsToolPathTables(t *testing.T) {
	isolateUserConfig(t)

	path, err := UserConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}

	content := `
[tools.sqlpackage]
path = "/opt/sqlpackage/sqlpackage"

[tools.sqlcmd]
path = "/opt/mssql-tools/bin/sqlcmd"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadUserConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tools["sqlpackage"] != "/opt/sqlpackage/sqlpackage" {
		t.Fatalf("sqlpackage tool path = %q", cfg.Tools["sqlpackage"])
	}
	if cfg.Tools["sqlcmd"] != "/opt/mssql-tools/bin/sqlcmd" {
		t.Fatalf("sqlcmd tool path = %q", cfg.Tools["sqlcmd"])
	}
}

func TestLoadUserConfigRejectsLegacyToolPathStrings(t *testing.T) {
	isolateUserConfig(t)

	path, err := UserConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	content := `
[tools]
sqlcmd = "/opt/mssql-tools/bin/sqlcmd"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadUserConfig(); err == nil {
		t.Fatal("expected legacy tools format to fail")
	}
}

func TestWriteUserConfigWritesToolPathTables(t *testing.T) {
	isolateUserConfig(t)
	_, err := WriteUserConfig(&Config{
		Tools: map[string]string{
			"sqlcmd": "/opt/mssql-tools/bin/sqlcmd",
		},
		Paths:    map[string]string{},
		Defaults: map[string]string{},
		Envs:     map[string]EnvConfig{},
		Secrets:  map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}

	path, err := UserConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "[tools.sqlcmd]\npath = \"/opt/mssql-tools/bin/sqlcmd\"\n"
	if string(content) != want {
		t.Fatalf("config content = %q, want %q", string(content), want)
	}
}

func TestWriteUserConfigWritesTLSConfig(t *testing.T) {
	isolateUserConfig(t)
	trust := true
	_, err := WriteUserConfig(&Config{
		Tools:    map[string]string{},
		Paths:    map[string]string{},
		Defaults: map[string]string{},
		Envs: map[string]EnvConfig{
			EnvLocal: {
				Server:                 "localhost",
				User:                   "sa",
				Encrypt:                "true",
				TrustServerCertificate: &trust,
			},
		},
		Secrets: map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}

	path, err := UserConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `[env.local]
encrypt = "true"
server = "localhost"
user = "sa"
trust_server_certificate = true
`
	if string(content) != want {
		t.Fatalf("config content = %q, want %q", string(content), want)
	}
}

func isolateUserConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}
