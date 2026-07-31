package cli

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/config"
)

type appContext struct {
	repoRoot          string
	sqlServer         string
	sqlUser           string
	sqlPassword       string
	sqlPasswordFile   string
	sqlPasswordSecret string
	nonInteractive    bool
	connectTimeout    time.Duration
	sqlTimeout        time.Duration
	processTimeout    time.Duration
	cfg               *config.Config
}

var buildVersion = "dev"

func NewRootCommand() *cobra.Command {
	app := &appContext{
		connectTimeout: defaultConnectTimeout,
		sqlTimeout:     defaultSQLTimeout,
		processTimeout: defaultProcessTimeout,
	}

	root := &cobra.Command{
		Use:           "sqlkit",
		Short:         "SQL Server project toolkit",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if app.cfg != nil {
				return nil
			}

			cfg, err := config.Load(app.repoRoot)
			if err != nil {
				return err
			}
			app.cfg = cfg
			return nil
		},
	}

	root.PersistentFlags().StringVar(&app.repoRoot, "repo", "", "repository root; defaults to [defaults].repo in user config")
	root.PersistentFlags().StringVar(&app.sqlServer, "server", "", "SQL Server override")
	root.PersistentFlags().StringVar(&app.sqlUser, "user", "", "SQL user override")
	root.PersistentFlags().StringVar(&app.sqlPassword, "password", "", "SQL password override")
	root.PersistentFlags().StringVar(&app.sqlPasswordFile, "password-file", "", "read SQL password override from a file")
	root.PersistentFlags().StringVar(&app.sqlPasswordSecret, "password-secret", "", "read SQL password override from a named keyring secret")
	root.PersistentFlags().BoolVar(&app.nonInteractive, "non-interactive", false, "fail instead of prompting for input")
	root.PersistentFlags().DurationVar(&app.connectTimeout, "connect-timeout", defaultConnectTimeout, "SQL connection timeout; use 0 to disable")
	root.PersistentFlags().DurationVar(&app.sqlTimeout, "sql-timeout", defaultSQLTimeout, "SQL statement timeout; use 0 to disable")
	root.PersistentFlags().DurationVar(&app.processTimeout, "process-timeout", defaultProcessTimeout, "external process timeout; use 0 to disable")
	root.Version = version()

	root.AddCommand(newDoctorCommand(app))
	root.AddCommand(newBackupCommand(app))
	root.AddCommand(newBacpacCommand(app))
	root.AddCommand(newConfigCommand(app))
	root.AddCommand(newDepsCommand(app))
	root.AddCommand(newDBCommand(app))
	root.AddCommand(newDocsCommand(app))
	root.AddCommand(newEnvCommand(app))
	root.AddCommand(newSQLCommand(app))
	root.AddCommand(newLintCommand())
	root.AddCommand(newLocksCommand(app))
	root.AddCommand(newPublishCommand(app))
	root.AddCommand(newBootstrapCommand(app))
	root.AddCommand(newMigrateCommand(app))
	root.AddCommand(newTUICommand(app))

	return root
}

func commandContext(cmd *cobra.Command) context.Context {
	if cmd.Context() != nil {
		return cmd.Context()
	}
	return context.Background()
}

func loadConnection(app *appContext, envName string) (*config.SQLConnection, error) {
	if strings.TrimSpace(envName) == "" {
		return nil, fmt.Errorf("--env is required")
	}
	password := app.sqlPassword
	if strings.TrimSpace(password) == "" {
		passwordFile := firstNonEmpty(app.sqlPasswordFile, os.Getenv("SQLPASSWORD_FILE"))
		if strings.TrimSpace(passwordFile) != "" {
			data, err := os.ReadFile(passwordFile)
			if err != nil {
				return nil, fmt.Errorf("read SQL password file: %w", err)
			}
			password = strings.TrimRight(string(data), "\r\n")
		}
	}
	if strings.TrimSpace(password) == "" && strings.TrimSpace(app.sqlPasswordSecret) != "" {
		name := strings.TrimSpace(app.sqlPasswordSecret)
		key, ok := app.cfg.Secrets[name]
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("keyring secret %q is not configured; run sqlkit config secret set %s", name, name)
		}
		value, err := config.Secret(key)
		if err != nil {
			return nil, fmt.Errorf("load keyring secret %q: %w", name, err)
		}
		password = value
	}
	password = firstNonEmpty(password, os.Getenv("SQLPASSWORD"))
	return app.cfg.LoadSQLConnectionWithOverrides(envName, config.SQLConnectionOverrides{
		Server:   app.sqlServer,
		User:     app.sqlUser,
		Password: password,
	})
}

func requireInteractive(app *appContext, requirement string) error {
	if !app.nonInteractive {
		return nil
	}
	return fmt.Errorf("%s is required with --non-interactive", requirement)
}

func isProdLike(envName string) bool {
	normalized, err := config.NormalizeEnvName(envName)
	if err != nil {
		return false
	}
	return normalized == config.EnvProd || normalized == config.EnvProdLegacy
}

func exitWithCode(code int) {
	os.Exit(code)
}

func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return buildVersion
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" && len(setting.Value) >= 12 {
			return buildVersion + " (" + setting.Value[:12] + ")"
		}
	}
	return buildVersion
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
