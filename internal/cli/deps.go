package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/confirm"
	"github.com/luem2/sqlkit/internal/deps"
)

type depsFlags struct {
	yes bool
}

func newDepsCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deps",
		Short: "Check external dependencies",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Check required external tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			printDependencyStatus(cmd, app)
			return nil
		},
	})
	cmd.AddCommand(newDepsInstallCommand(app))

	return cmd
}

func newDepsInstallCommand(app *appContext) *cobra.Command {
	flags := &depsFlags{}
	cmd := &cobra.Command{
		Use:   "install <tool>",
		Short: "Install or print install steps for an external tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := deps.PlanInstall(args[0])
			if err != nil {
				return err
			}

			printInstallPlan(cmd, plan)
			if len(plan.Commands) == 0 {
				return nil
			}

			if !flags.yes {
				if err := requireInteractive(app, "--yes"); err != nil {
					return err
				}
				if err := confirm.ExactFold(cmd.InOrStdin(), cmd.OutOrStdout(), "install"); err != nil {
					return err
				}
			}

			proc := newProcessService(cmd, app)
			for _, command := range plan.Commands {
				result := proc.RunStreaming(command[0], command[1:]...)
				if result.ExitCode != 0 {
					return fmt.Errorf("%s failed with exit code %d\n%s", command[0], result.ExitCode, firstNonEmpty(result.Stderr, result.Stdout))
				}
			}
			successf(cmd.OutOrStdout(), "Installed: %s", plan.Name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flags.yes, "yes", false, "run install commands without confirmation")
	return cmd
}

func printDependencyStatus(cmd *cobra.Command, app *appContext) {
	for _, dep := range deps.DefaultDependencies(app.cfg) {
		status := deps.Check(commandContext(cmd), dep)
		fmt.Fprintln(cmd.OutOrStdout(), deps.FormatStatus(status))
	}
}

func printInstallPlan(cmd *cobra.Command, plan deps.InstallPlan) {
	fmt.Fprintf(cmd.OutOrStdout(), "Dependency: %s\n", plan.Name)
	for _, note := range plan.Notes {
		fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", note)
	}
	if len(plan.Commands) == 0 {
		return
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Commands:")
	for _, command := range plan.Commands {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", strings.Join(command, " "))
	}
}
