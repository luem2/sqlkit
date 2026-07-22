package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/confirm"
	"github.com/luem2/sqlkit/internal/sqlserver"
)

func newDBSizeCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{}
	cmd := &cobra.Command{
		Use:   "size",
		Short: "List database data and log sizes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(flags.database) != "" {
				if err := sqlserver.ValidateDatabaseName(flags.database); err != nil {
					return err
				}
			}
			return runReadOnlySQL(cmd, app, flags, "master", sqlserver.DatabaseSizeSQL(flags.database))
		},
	}
	addEnvFlag(cmd, flags)
	cmd.Flags().StringVar(&flags.database, "database", "", "optional database name")
	return cmd
}

func newDBRecoveryCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{}
	cmd := &cobra.Command{
		Use:   "recovery",
		Short: "List database recovery models",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(flags.database) != "" {
				if err := sqlserver.ValidateDatabaseName(flags.database); err != nil {
					return err
				}
			}
			return runReadOnlySQL(cmd, app, flags, "master", sqlserver.RecoveryModelSQL(flags.database))
		},
	}
	addEnvFlag(cmd, flags)
	cmd.Flags().StringVar(&flags.database, "database", "", "optional database name")
	return cmd
}

func newDBFKReferencesCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{}
	cmd := &cobra.Command{
		Use:   "fk-references",
		Short: "List foreign keys referencing a table",
		RunE: func(cmd *cobra.Command, args []string) error {
			schema, table, err := sqlserver.ParseSchemaTable(flags.table)
			if err != nil {
				return err
			}
			return runReadOnlySQL(cmd, app, flags, flags.database, sqlserver.FKReferencesSQL(schema, table))
		},
	}
	addEnvDatabaseTableFlags(cmd, flags)
	return cmd
}

func newDBNullsCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{}
	cmd := &cobra.Command{
		Use:   "nulls",
		Short: "Return rows where nullable columns are null",
		RunE: func(cmd *cobra.Command, args []string) error {
			schema, table, err := sqlserver.ParseSchemaTable(flags.table)
			if err != nil {
				return err
			}
			return runReadOnlySQL(cmd, app, flags, flags.database, sqlserver.NullRowsSQL(schema, table))
		},
	}
	addEnvDatabaseTableFlags(cmd, flags)
	return cmd
}

func newDBCharScanCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{}
	cmd := &cobra.Command{
		Use:   "char-scan",
		Short: "Scan text columns for line breaks, tabs and edge spaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			schema, table, err := sqlserver.ParseSchemaTable(flags.table)
			if err != nil {
				return err
			}
			return runReadOnlySQL(cmd, app, flags, flags.database, sqlserver.CharScanSQL(schema, table))
		},
	}
	addEnvDatabaseTableFlags(cmd, flags)
	return cmd
}

func newDBCharCleanCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{}
	cmd := &cobra.Command{
		Use:   "char-clean",
		Short: "Clean problematic characters from text columns",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := sqlserver.ValidateUserDatabaseName(flags.database); err != nil {
				return err
			}
			if err := guardProd(flags); err != nil {
				return err
			}
			schema, table, err := sqlserver.ParseSchemaTable(flags.table)
			if err != nil {
				return err
			}
			sql := sqlserver.CharCleanSQL(schema, table)
			if !flags.yes {
				if err := requireInteractive(app, "--yes"); err != nil {
					return err
				}
				warnf(cmd.OutOrStdout(), "This will update text columns in %s.", flags.table)
				if err := confirm.Exact(cmd.InOrStdin(), cmd.OutOrStdout(), flags.table); err != nil {
					return err
				}
			}
			return runSQLInDatabase(cmd, app, flags, flags.database, sql)
		},
	}
	addEnvDatabaseTableFlags(cmd, flags)
	cmd.Flags().BoolVar(&flags.yes, "yes", false, "skip interactive confirmation")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	return cmd
}

func runReadOnlySQL(cmd *cobra.Command, app *appContext, flags *dbFlags, database string, statement sqlserver.Statement) error {
	db, err := newDBService(cmd, app, flags.env)
	if err != nil {
		return err
	}

	resultSets, err := db.queryIn(database, statement)
	if err != nil {
		return err
	}
	printResultSets(cmd.OutOrStdout(), resultSets)
	return nil
}
