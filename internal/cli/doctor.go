package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDoctorCommand(app *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check external dependencies",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "sqlkit doctor")
			fmt.Fprintln(cmd.OutOrStdout())

			printDependencyStatus(cmd, app)

			return nil
		},
	}
}
