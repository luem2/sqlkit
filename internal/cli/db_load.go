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

type loadSourceType string

const (
	loadSourceBak    loadSourceType = "bak"
	loadSourceBacpac loadSourceType = "bacpac"
	loadSourceDacpac loadSourceType = "dacpac"
)

func newDBLoadCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{}
	cmd := &cobra.Command{
		Use:   "load",
		Short: "Load a bak, bacpac or dacpac into a database",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := sqlserver.ValidateDatabaseName(flags.database); err != nil {
				return err
			}
			if err := guardProd(flags); err != nil {
				return err
			}

			source := strings.TrimSpace(flags.source)
			if source == "" {
				return fmt.Errorf("--source is required")
			}
			sourceType, err := detectLoadSourceType(source, flags.sourceType)
			if err != nil {
				return err
			}

			switch sourceType {
			case loadSourceBak:
				return loadBakSource(cmd, app, flags, source)
			case loadSourceBacpac:
				return loadBacpacSource(cmd, app, flags, source)
			case loadSourceDacpac:
				return loadDacpacSource(cmd, app, flags, source)
			default:
				return fmt.Errorf("unsupported source type %q", sourceType)
			}
		},
	}
	addEnvDatabaseFlags(cmd, flags)
	cmd.Flags().StringVar(&flags.source, "source", "", "source file: .bak, .bacpac or .dacpac")
	cmd.Flags().StringVar(&flags.sourceType, "source-type", "", "source type override: bak, bacpac or dacpac")
	cmd.Flags().StringVar(&flags.bakDir, "bak-dir", "", "staging directory on the SQL Server host for bak files; defaults to paths.sqlserver_backup_dir or backups")
	cmd.Flags().StringVar(&flags.container, "container", "", "Docker SQL Server container name for local bak staging; defaults to paths.sqlserver_container")
	cmd.Flags().BoolVar(&flags.keepStaged, "keep-staged", false, "keep staged bak file inside the SQL Server container")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	_ = cmd.MarkFlagRequired("source")
	return cmd
}

func detectLoadSourceType(source string, explicit string) (loadSourceType, error) {
	normalized := strings.ToLower(strings.TrimSpace(explicit))
	if normalized != "" {
		switch loadSourceType(normalized) {
		case loadSourceBak, loadSourceBacpac, loadSourceDacpac:
			return loadSourceType(normalized), nil
		default:
			return "", fmt.Errorf("--source-type must be bak, bacpac or dacpac")
		}
	}

	switch strings.ToLower(filepath.Ext(source)) {
	case ".bak":
		return loadSourceBak, nil
	case ".bacpac":
		return loadSourceBacpac, nil
	case ".dacpac":
		return loadSourceDacpac, nil
	default:
		return "", fmt.Errorf("cannot infer source type from %q; pass --source-type bak, bacpac or dacpac", source)
	}
}

func loadBakSource(cmd *cobra.Command, app *appContext, flags *dbFlags, source string) error {
	if err := sqlserver.ValidateUserDatabaseName(flags.database); err != nil {
		return err
	}

	backupFile, cleanup, err := prepareLoadBakSource(cmd, app, flags, source)
	if err != nil {
		return err
	}
	if !flags.keepStaged {
		defer cleanup()
	}

	sql, err := sqlserver.RestoreDatabaseSQL(flags.database, backupFile)
	if err != nil {
		return err
	}
	return runBackupRestoreSQL(cmd, app, flags, sql, "Load restore", backupFile)
}

func prepareLoadBakSource(cmd *cobra.Command, app *appContext, flags *dbFlags, source string) (string, func(), error) {
	hostFile, ok := resolveExistingHostSource(app, source)
	if !ok {
		return resolveBakPath(app, source, flags.bakDir), func() {}, nil
	}

	container := backupContainer(app, flags)
	if container == "" {
		if strings.EqualFold(strings.TrimSpace(flags.env), "local") {
			return "", func() {}, fmt.Errorf("local bak source %q needs Docker staging, but no SQL Server container is configured; configure paths.sqlserver_container or pass --container", hostFile)
		}
		return resolveBakPath(app, source, flags.bakDir), func() {}, nil
	}

	serverFile := loadStagedBakPath(app, flags, hostFile)
	if err := prepareBackupDirectory(cmd, app, container, serverFile); err != nil {
		return "", func() {}, err
	}
	result := newProcessService(cmd, app).Run(app.cfg.ToolPath("docker"), "cp", hostFile, container+":"+serverFile)
	if result.ExitCode != 0 {
		return "", func() {}, fmt.Errorf("stage bak file %s: %s", hostFile, firstNonEmpty(result.Stderr, result.Stdout))
	}
	if err := fixContainerFileOwnership(cmd, app, container, serverFile); err != nil {
		return "", func() {}, err
	}
	successf(cmd.OutOrStdout(), "Staged bak: %s", serverFile)

	cleanup := func() {
		result := newProcessService(cmd, app).Run(app.cfg.ToolPath("docker"), "exec", container, "rm", "-f", serverFile)
		if result.ExitCode != 0 {
			warnf(cmd.ErrOrStderr(), "Could not remove staged bak file %s: %s", serverFile, firstNonEmpty(result.Stderr, result.Stdout))
		}
	}
	return serverFile, cleanup, nil
}

func loadBacpacSource(cmd *cobra.Command, app *appContext, flags *dbFlags, source string) error {
	bacpac, err := resolveBacpacPath(app, flags.env, source)
	if err != nil {
		return err
	}
	conn, err := loadConnection(app, flags.env)
	if err != nil {
		return err
	}
	result := newProcessService(cmd, app).RunStreamingRedacted(
		[]string{conn.Password},
		app.cfg.ToolPath("sqlpackage"),
		ssdt.BacpacImportArgs(conn, flags.database, bacpac)...,
	)
	if result.ExitCode != 0 {
		return sqlPackageError("import", result.ExitCode, firstNonEmpty(result.Stderr, result.Stdout))
	}
	successf(cmd.OutOrStdout(), "Load bacpac OK: %s", flags.database)
	return nil
}

func loadDacpacSource(cmd *cobra.Command, app *appContext, flags *dbFlags, source string) error {
	dacpac := resolveRepoPath(app, source)
	conn, err := loadConnection(app, flags.env)
	if err != nil {
		return err
	}
	result := newProcessService(cmd, app).RunStreamingRedacted(
		[]string{conn.Password},
		app.cfg.ToolPath("sqlpackage"),
		ssdt.SQLPackageArgs(ssdt.ActionPublish, dacpac, conn, flags.database, "")...,
	)
	if result.ExitCode != 0 {
		return sqlPackageError("publish", result.ExitCode, firstNonEmpty(result.Stderr, result.Stdout))
	}
	successf(cmd.OutOrStdout(), "Load dacpac OK: %s", flags.database)
	return nil
}

func resolveExistingHostSource(app *appContext, source string) (string, bool) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", false
	}
	candidates := []string{source}
	if !filepath.IsAbs(source) {
		candidates = append(candidates, resolveRepoPath(app, source))
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			absolute, err := filepath.Abs(candidate)
			if err == nil {
				return absolute, true
			}
			return candidate, true
		}
	}
	return "", false
}

func loadStagedBakPath(app *appContext, flags *dbFlags, hostFile string) string {
	relative := joinSQLServerPath(resolveSQLServerBakDir(app, flags.bakDir), joinSQLServerPath("_load", filepath.Base(hostFile)))
	return dockerBackupSource(app, relative)
}
