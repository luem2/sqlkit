package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/lint"
)

type lintFlags struct {
	json      bool
	recursive bool
}

type unusedVarsResult struct {
	Path   string          `json:"path"`
	Usages []lint.VarUsage `json:"usages"`
}

func newLintCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Static SQL checks",
	}

	flags := &lintFlags{}
	unusedVarsCmd := &cobra.Command{
		Use:   "unused-vars <archivo-o-directorio>",
		Short: "Find possibly unused T-SQL variables",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := runUnusedVars(args[0], flags.recursive)
			if err != nil {
				return err
			}

			if flags.json {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(results); err != nil {
					return err
				}
			} else {
				printUnusedVarsResults(cmd, results)
			}

			if hasUnusedVars(results) {
				exitWithCode(1)
			}
			return nil
		},
	}
	unusedVarsCmd.Flags().BoolVar(&flags.json, "json", false, "print JSON output")
	unusedVarsCmd.Flags().BoolVar(&flags.recursive, "recursive", false, "scan .sql files recursively")
	cmd.AddCommand(unusedVarsCmd)

	return cmd
}

func runUnusedVars(path string, recursive bool) ([]unusedVarsResult, error) {
	if !recursive {
		usages, err := lint.UnusedVars(path)
		if err != nil {
			return nil, err
		}
		return []unusedVarsResult{{Path: path, Usages: usages}}, nil
	}

	var paths []string
	if err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(current), ".sql") {
			paths = append(paths, current)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(paths)

	results := make([]unusedVarsResult, 0, len(paths))
	for _, sqlPath := range paths {
		usages, err := lint.UnusedVars(sqlPath)
		if err != nil {
			return nil, err
		}
		results = append(results, unusedVarsResult{Path: sqlPath, Usages: usages})
	}
	return results, nil
}

func printUnusedVarsResults(cmd *cobra.Command, results []unusedVarsResult) {
	if len(results) == 1 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), lint.FormatUnused(results[0].Path, results[0].Usages))
		return
	}

	printed := false
	for _, result := range results {
		if len(result.Usages) == 0 {
			continue
		}
		if printed {
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), lint.FormatUnused(result.Path, result.Usages))
		printed = true
	}
	if !printed {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No possibly unused variables found.")
	}
}

func hasUnusedVars(results []unusedVarsResult) bool {
	for _, result := range results {
		if len(result.Usages) > 0 {
			return true
		}
	}
	return false
}
