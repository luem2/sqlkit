package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/backups"
	"github.com/luem2/sqlkit/internal/sqlserver"
	"github.com/luem2/sqlkit/internal/ssdt"
)

type bacpacFlags struct {
	env       string
	database  string
	output    string
	allowProd bool
}

func newBacpacCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bacpac",
		Short: "Export bacpac files with SqlPackage",
	}

	cmd.AddCommand(newBacpacExportCommand(app))

	return cmd
}

func newBacpacExportCommand(app *appContext) *cobra.Command {
	flags := &bacpacFlags{}
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export a database to a bacpac file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := sqlserver.ValidateDatabaseName(flags.database); err != nil {
				return err
			}
			if isProdLike(flags.env) && !flags.allowProd {
				return fmt.Errorf("--allow-prod is required for env %q", flags.env)
			}

			output := strings.TrimSpace(flags.output)
			if output == "" {
				output = defaultBacpacPath(app, flags.env, flags.database, time.Now())
			} else {
				output = resolveRepoPath(app, output)
			}

			conn, err := loadConnection(app, flags.env)
			if err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
				return err
			}

			result := newProcessService(cmd, app).RunStreamingRedacted(
				[]string{conn.Password},
				app.cfg.ToolPath("sqlpackage"),
				ssdt.BacpacExportArgs(conn, flags.database, output)...,
			)
			if result.ExitCode != 0 {
				return sqlPackageError("export", result.ExitCode, firstNonEmpty(result.Stderr, result.Stdout))
			}
			successf(cmd.OutOrStdout(), "Bacpac export: %s", output)
			return nil
		},
	}
	addBacpacEnvDatabaseFlags(cmd, flags)
	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "bacpac output file; defaults to data/bacpacs/<env>/<database>/export/YYYY/MM/DD")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	return cmd
}

func addBacpacEnvDatabaseFlags(cmd *cobra.Command, flags *bacpacFlags) {
	cmd.Flags().StringVar(&flags.env, "env", "", "environment: local, prod, prod-legacy or infra")
	cmd.Flags().StringVar(&flags.database, "database", "", "database name")
	_ = cmd.MarkFlagRequired("env")
	_ = cmd.MarkFlagRequired("database")
}

func defaultBacpacPath(app *appContext, environment string, database string, at time.Time) string {
	fileName := fmt.Sprintf("%s_BACPAC_%s_%09d.bacpac", ssdt.SafeFileName(database), at.Format("20060102_150405"), at.Nanosecond())
	return backups.DatedPath(resolveBacpacDir(app), environment, database, "export", at, fileName)
}

func resolveBacpacDir(app *appContext) string {
	if value := strings.TrimSpace(app.cfg.Paths["bacpacs"]); value != "" {
		return resolveRepoPath(app, value)
	}
	return resolveRepoPath(app, filepath.Join("data", "bacpacs"))
}

func resolveBacpacPath(app *appContext, environment string, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if filepath.IsAbs(value) || strings.Contains(value, "/") || strings.Contains(value, `\`) {
		return resolveRepoPath(app, value), nil
	}
	return findBacpacByName(app, environment, value)
}

func findBacpacByName(app *appContext, environment string, fileName string) (string, error) {
	root := filepath.Join(resolveBacpacDir(app), environment)
	var matches []string
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("bacpac %q was not found under %s", fileName, root)
		}
		return "", err
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(entry.Name(), fileName) {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("bacpac %q was not found under %s", fileName, root)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("bacpac %q is ambiguous under %s; pass the full path", fileName, root)
	}
	return matches[0], nil
}
