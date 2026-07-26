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
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "sqlkit doctor"); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
				return err
			}

			printDependencyStatus(cmd, app)

			return nil
		},
	}
}
