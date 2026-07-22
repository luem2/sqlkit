package cli

import (
	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/confirm"
	"github.com/luem2/sqlkit/internal/sqlserver"
)

func newDBExistsCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{}
	cmd := &cobra.Command{
		Use:   "exists",
		Short: "Check whether a database exists",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := sqlserver.ValidateDatabaseName(flags.database); err != nil {
				return err
			}

			db, err := newDBService(cmd, app, flags.env)
			if err != nil {
				return err
			}

			resultSets, err := db.queryIn("master", sqlserver.ExistsSQL(flags.database))
			if err != nil {
				return err
			}

			if databaseExists(resultSets) {
				successf(cmd.OutOrStdout(), "Database exists: %s", flags.database)
				return nil
			}

			warnf(cmd.OutOrStdout(), "Database does not exist: %s", flags.database)
			exitWithCode(1)
			return nil
		},
	}
	addEnvDatabaseFlags(cmd, flags)
	return cmd
}

func newDBSessionsCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{}
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "List sessions connected to a database",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := sqlserver.ValidateDatabaseName(flags.database); err != nil {
				return err
			}
			db, err := newDBService(cmd, app, flags.env)
			if err != nil {
				return err
			}

			resultSets, err := db.queryIn("master", sqlserver.SessionsSQL(flags.database))
			if err != nil {
				return err
			}
			printResultSets(cmd.OutOrStdout(), resultSets)
			return nil
		},
	}
	addEnvDatabaseFlags(cmd, flags)
	return cmd
}

func newDBKillSessionsCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{}
	cmd := &cobra.Command{
		Use:   "kill-sessions",
		Short: "Kill sessions connected to a database",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := sqlserver.ValidateUserDatabaseName(flags.database); err != nil {
				return err
			}

			sql := sqlserver.KillSessionsSQL(flags.database)

			if err := guardProd(flags); err != nil {
				return err
			}

			db, err := newDBService(cmd, app, flags.env)
			if err != nil {
				return err
			}

			return db.execIn("master", sql)
		},
	}
	addEnvDatabaseFlags(cmd, flags)
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	return cmd
}

func newDBDropCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{}
	cmd := &cobra.Command{
		Use:   "drop",
		Short: "Drop a database after forcing single-user mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			sql, err := sqlserver.DropDatabaseSQL(flags.database, flags.deleteBackupHistory)
			if err != nil {
				return err
			}

			if err := guardProd(flags); err != nil {
				return err
			}

			if !flags.yes {
				if err := requireInteractive(app, "--yes"); err != nil {
					return err
				}
				warnf(cmd.OutOrStdout(), "This will drop the database and rollback active sessions.")
				if err := confirm.Exact(cmd.InOrStdin(), cmd.OutOrStdout(), flags.database); err != nil {
					return err
				}
			}

			return runAdministrativeSQL(cmd, app, flags, sql)
		},
	}
	addEnvDatabaseFlags(cmd, flags)
	cmd.Flags().BoolVar(&flags.yes, "yes", false, "skip interactive confirmation")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	cmd.Flags().BoolVar(&flags.deleteBackupHistory, "delete-backup-history", false, "delete msdb backup history after dropping the database")
	return cmd
}

func newDBRenameCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{}
	cmd := &cobra.Command{
		Use:   "rename",
		Short: "Rename a database after forcing single-user mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			sql, err := sqlserver.RenameDatabaseSQL(flags.database, flags.newName)
			if err != nil {
				return err
			}
			if err := guardProd(flags); err != nil {
				return err
			}
			if !flags.yes {
				if err := requireInteractive(app, "--yes"); err != nil {
					return err
				}
				warnf(cmd.OutOrStdout(), "This will rename database %s to %s and rollback active sessions.", flags.database, flags.newName)
				if err := confirm.Exact(cmd.InOrStdin(), cmd.OutOrStdout(), flags.newName); err != nil {
					return err
				}
			}
			return runAdministrativeSQL(cmd, app, flags, sql)
		},
	}
	addEnvDatabaseFlags(cmd, flags)
	cmd.Flags().StringVar(&flags.newName, "new-name", "", "new database name")
	cmd.Flags().BoolVar(&flags.yes, "yes", false, "skip interactive confirmation")
	cmd.Flags().BoolVar(&flags.allowProd, "allow-prod", false, "allow execution against prod or prod-legacy")
	_ = cmd.MarkFlagRequired("new-name")
	return cmd
}

func runAdministrativeSQL(cmd *cobra.Command, app *appContext, flags *dbFlags, statement sqlserver.Statement) error {
	return runSQLInDatabase(cmd, app, flags, "master", statement)
}

func databaseExists(resultSets []sqlserver.ResultSet) bool {
	if len(resultSets) == 0 || len(resultSets[0].Rows) == 0 || len(resultSets[0].Rows[0]) == 0 {
		return false
	}
	return resultSets[0].Rows[0][0] == "1"
}

func runSQLInDatabase(cmd *cobra.Command, app *appContext, flags *dbFlags, database string, statement sqlserver.Statement) error {
	if err := guardProd(flags); err != nil {
		return err
	}

	db, err := newDBService(cmd, app, flags.env)
	if err != nil {
		return err
	}
	return db.execIn(database, statement)
}
