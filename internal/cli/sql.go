package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/config"
	"github.com/luem2/sqlkit/internal/sqlscripts"
)

type sqlFlags struct {
	env        string
	database   string
	logDir     string
	allowProd  bool
	variables  []string
	secretVars []string
}

var sqlcmdVariableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func newSQLCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql",
		Short: "Run SQL scripts with sqlcmd",
	}

	cmd.AddCommand(newSQLRunCommand(app))
	cmd.AddCommand(newSQLRunDirCommand(app))

	return cmd
}

func newSQLRunCommand(app *appContext) *cobra.Command {
	flags := &sqlFlags{}
	cmd := &cobra.Command{
		Use:   "run <script.sql>",
		Short: "Run one SQL script",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			script, err := sqlscripts.ResolveFile(app.cfg.Root, args[0])
			if err != nil {
				return err
			}
			return runSQLScripts(cmd, app, flags, []string{script})
		},
	}
	addSQLFlags(cmd, flags)
	return cmd
}

func newSQLRunDirCommand(app *appContext) *cobra.Command {
	flags := &sqlFlags{}
	cmd := &cobra.Command{
		Use:   "run-dir <directory>",
		Short: "Run all .sql scripts in a directory ordered by file name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scripts, err := sqlscripts.ResolveDir(app.cfg.Root, args[0])
			if err != nil {
				return err
			}
			return runSQLScripts(cmd, app, flags, scripts)
		},
	}
	addSQLFlags(cmd, flags)
	return cmd
}

func addSQLFlags(cmd *cobra.Command, flags *sqlFlags) {
	cmd.Flags().StringVar(&flags.env, "env", "", "environment: local, prod, prod-legacy or infra")
	cmd.Flags().StringVar(&flags.database, "database", "master", "database used as sqlcmd initial catalog")
	cmd.Flags().StringVar(&flags.logDir, "log-dir", "", "directory for per-script logs; defaults to paths.logs/sql")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	cmd.Flags().StringArrayVar(&flags.variables, "var", nil, "SQLCMD variable NAME=VALUE; repeatable and not for secrets")
	cmd.Flags().StringArrayVar(&flags.secretVars, "secret-var", nil, "SQLCMD variable NAME=KEYRING_SECRET_NAME; repeatable")
	_ = cmd.MarkFlagRequired("env")
}

func runSQLScripts(cmd *cobra.Command, app *appContext, flags *sqlFlags, scripts []string) error {
	if isProdLike(flags.env) && !flags.allowProd {
		return fmt.Errorf("--allow-prod is required for env %q", flags.env)
	}

	db, err := newDBService(cmd, app, flags.env)
	if err != nil {
		return err
	}
	variables, err := parseSQLCMDVariables(flags.variables)
	if err != nil {
		return fmt.Errorf("--var: %w", err)
	}
	secretRefs, err := parseSQLCMDVariables(flags.secretVars)
	if err != nil {
		return fmt.Errorf("--secret-var: %w", err)
	}
	secretVariables := make(map[string]string, len(secretRefs))
	secretRedactions := make([]string, 0, len(secretRefs)*2)
	for variable, secretName := range secretRefs {
		key, ok := app.cfg.Secrets[secretName]
		if !ok || strings.TrimSpace(key) == "" {
			return fmt.Errorf("keyring secret %q is not configured; run sqlkit config secret set %s", secretName, secretName)
		}
		value, err := config.Secret(key)
		if err != nil {
			return fmt.Errorf("load keyring secret %q: %w", secretName, err)
		}
		escaped := escapeSQLStringLiteral(value)
		secretVariables[variable] = escaped
		secretRedactions = append(secretRedactions, value, escaped)
	}

	runner := db.runner(app.cfg.ToolPath("sqlcmd"))
	for _, script := range scripts {
		infof(cmd.OutOrStdout(), "Running %s", script)
		ctx, cancel := timeoutContext(commandContext(cmd), app.processTimeout)
		result := runner.RunFileWithVariablesAndEnvironment(ctx, flags.database, script, variables, secretVariables, secretRedactions)
		cancel()
		if err := writeSQLLog(resolveSQLLogDir(app, flags), script, result.Stdout, result.Stderr); err != nil {
			return err
		}
		if result.Stdout != "" {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), result.Stdout); err != nil {
				return err
			}
		}
		if result.Stderr != "" {
			if _, err := fmt.Fprintln(cmd.ErrOrStderr(), result.Stderr); err != nil {
				return err
			}
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("script failed with exit code %d: %s", result.ExitCode, script)
		}
	}

	return nil
}

func escapeSQLStringLiteral(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func parseSQLCMDVariables(values []string) (map[string]string, error) {
	parsed := make(map[string]string, len(values))
	for _, value := range values {
		name, variableValue, ok := strings.Cut(value, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("expected NAME=VALUE, got %q", value)
		}
		if !sqlcmdVariableName.MatchString(name) {
			return nil, fmt.Errorf("invalid SQLCMD variable name %q", name)
		}
		if _, exists := parsed[name]; exists {
			return nil, fmt.Errorf("duplicate SQLCMD variable %q", name)
		}
		parsed[name] = variableValue
	}
	return parsed, nil
}

func writeSQLLog(logDir string, script string, stdout string, stderr string) error {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	name := strings.TrimSuffix(filepath.Base(script), filepath.Ext(script))
	name = fmt.Sprintf("%s-%s.log", time.Now().Format("20060102-150405"), name)
	path := filepath.Join(logDir, name)

	content := strings.TrimRight(stdout+"\n"+stderr, "\r\n")
	return os.WriteFile(path, []byte(content+"\n"), 0600)
}

func resolveSQLLogDir(app *appContext, flags *sqlFlags) string {
	if strings.TrimSpace(flags.logDir) != "" {
		return resolveRepoPath(app, flags.logDir)
	}
	return resolveLogsDir(app, "sql")
}
