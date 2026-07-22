package cli

import "github.com/spf13/cobra"

func newDBCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "SQL Server database utilities",
	}

	cmd.AddCommand(newDBBuildCommand(app))
	cmd.AddCommand(newDBLoadCommand(app))
	cmd.AddCommand(newDBScriptCommand(app))
	cmd.AddCommand(newDBBackupCommand(app))
	cmd.AddCommand(newDBExistsCommand(app))
	cmd.AddCommand(newDBSessionsCommand(app))
	cmd.AddCommand(newDBFKReferencesCommand(app))
	cmd.AddCommand(newDBNullsCommand(app))
	cmd.AddCommand(newDBCharScanCommand(app))
	cmd.AddCommand(newDBCharCleanCommand(app))
	cmd.AddCommand(newDBSizeCommand(app))
	cmd.AddCommand(newDBRecoveryCommand(app))
	cmd.AddCommand(newDBKillSessionsCommand(app))
	cmd.AddCommand(newDBRenameCommand(app))
	cmd.AddCommand(newDBDropCommand(app))

	return cmd
}
