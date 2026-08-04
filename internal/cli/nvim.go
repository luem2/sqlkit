package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/sqlserver"
)

func newNvimCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nvim",
		Short: "Neovim integration helpers",
	}

	cmd.AddCommand(newNvimDadbodURLCommand(app))

	return cmd
}

func newNvimDadbodURLCommand(app *appContext) *cobra.Command {
	flags := &dbFlags{}
	cmd := &cobra.Command{
		Use:   "dadbod-url",
		Short: "Print a Dadbod SQL Server URL from sqlkit config",
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := loadConnection(app, flags.env)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), sqlserver.DadbodURL(conn, flags.database))
			return err
		},
	}
	addEnvDatabaseFlags(cmd, flags)
	return cmd
}
