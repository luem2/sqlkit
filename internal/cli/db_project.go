package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/sqlserver"
	"github.com/luem2/sqlkit/internal/ssdt"
)

func newDBBuildCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{configuration: ssdt.DefaultConfiguration}
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build a SQL Server Database Project",
		RunE: func(cmd *cobra.Command, args []string) error {
			project := resolveProjectPath(app, flags.project)
			if strings.TrimSpace(project) == "" {
				return fmt.Errorf("--project is required when paths.project is not configured")
			}
			buildArgs := ssdt.BuildArgs(project, flags.configuration)
			result := newProcessService(cmd, app).RunStreaming(app.cfg.ToolPath("dotnet"), buildArgs...)
			if result.ExitCode != 0 {
				return fmt.Errorf("dotnet build failed with exit code %d\n%s", result.ExitCode, firstNonEmpty(result.Stderr, result.Stdout))
			}
			successf(cmd.OutOrStdout(), "Build OK: %s", project)
			return nil
		},
	}
	addProjectFlags(cmd, flags)
	return cmd
}

func newDBScriptCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{configuration: ssdt.DefaultConfiguration}
	cmd := &cobra.Command{
		Use:   "script",
		Short: "Generate a publish script with SqlPackage",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := sqlserver.ValidateDatabaseName(flags.database); err != nil {
				return err
			}
			return runSQLPackageScript(cmd, app, flags)
		},
	}
	addEnvDatabaseFlags(cmd, flags)
	addProjectFlags(cmd, flags)
	cmd.Flags().StringVar(&flags.dacpac, "dacpac", "", "dacpac path; defaults to the build output for the project")
	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "publish script output path")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	return cmd
}

func addProjectFlags(cmd *cobra.Command, flags *dbFlags) {
	cmd.Flags().StringVar(&flags.project, "project", "", "SQL project path; defaults to paths.project in user config")
	cmd.Flags().StringVarP(&flags.configuration, "configuration", "c", ssdt.DefaultConfiguration, "build configuration")
}

func runSQLPackageScript(cmd *cobra.Command, app *appContext, flags *dbFlags) error {
	project := resolveProjectPath(app, flags.project)
	if strings.TrimSpace(flags.dacpac) == "" && strings.TrimSpace(app.cfg.Paths["dacpac"]) == "" && strings.TrimSpace(project) == "" {
		return fmt.Errorf("--dacpac or --project is required when paths.dacpac and paths.project are not configured")
	}
	dacpac := resolveDacpacPath(app, flags.dacpac, project, flags.configuration)
	output := strings.TrimSpace(flags.output)
	if output == "" {
		output = ssdt.ScriptOutputPath(resolveArtifactsDir(app), flags.database)
	}

	conn, err := loadConnection(app, flags.env)
	if err != nil {
		return err
	}

	args := ssdt.SQLPackageArgs(ssdt.ActionScript, dacpac, conn, flags.database, output)
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}

	result := newProcessService(cmd, app).RunStreamingRedacted([]string{conn.Password}, app.cfg.ToolPath("sqlpackage"), args...)
	if result.ExitCode != 0 {
		return sqlPackageError(strings.ToLower(string(ssdt.ActionScript)), result.ExitCode, firstNonEmpty(result.Stderr, result.Stdout))
	}
	successf(cmd.OutOrStdout(), "Script: %s", output)
	return nil
}

func resolveProjectPath(app *appContext, value string) string {
	project := firstNonEmpty(value, app.cfg.Paths["project"])
	if strings.TrimSpace(project) == "" {
		return ""
	}
	return resolveRepoPath(app, project)
}

func resolveDacpacPath(app *appContext, value string, project string, configuration string) string {
	dacpac := firstNonEmpty(value, app.cfg.Paths["dacpac"])
	if strings.TrimSpace(dacpac) != "" {
		return resolveRepoPath(app, dacpac)
	}
	return ssdt.DefaultDacpacPath(project, configuration)
}

func resolveArtifactsDir(app *appContext) string {
	return resolveRepoPath(app, firstNonEmpty(app.cfg.Paths["artifacts"], "artifacts"))
}
