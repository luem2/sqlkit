package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type dbFlags struct {
	env                 string
	database            string
	newName             string
	table               string
	source              string
	sourceType          string
	project             string
	configuration       string
	dacpac              string
	output              string
	serverOutput        string
	bakDir              string
	container           string
	moveToHost          bool
	keepStaged          bool
	yes                 bool
	allowProd           bool
	deleteBackupHistory bool
}

func addEnvDatabaseFlags(cmd *cobra.Command, flags *dbFlags) {
	addEnvFlag(cmd, flags)
	cmd.Flags().StringVar(&flags.database, "database", "", "database name")
	_ = cmd.MarkFlagRequired("database")
}

func addEnvFlag(cmd *cobra.Command, flags *dbFlags) {
	cmd.Flags().StringVar(&flags.env, "env", "", "environment: local, prod, prod-legacy or infra")
	_ = cmd.MarkFlagRequired("env")
}

func addEnvDatabaseTableFlags(cmd *cobra.Command, flags *dbFlags) {
	addEnvDatabaseFlags(cmd, flags)
	cmd.Flags().StringVar(&flags.table, "table", "", "table name in schema.table format")
	_ = cmd.MarkFlagRequired("table")
}

func guardProd(flags *dbFlags) error {
	if isProdLike(flags.env) && !flags.allowProd {
		return fmt.Errorf("--allow-prod is required for env %q", flags.env)
	}
	return nil
}

func resolveRepoPath(app *appContext, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(app.cfg.Root, value)
}

func resolveLogsDir(app *appContext, parts ...string) string {
	base := resolveRepoPath(app, firstNonEmpty(app.cfg.Paths["logs"], "logs"))
	if len(parts) == 0 {
		return base
	}
	values := append([]string{base}, parts...)
	return filepath.Join(values...)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
