package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/luem2/sqlkit/internal/deps"
	"github.com/luem2/sqlkit/internal/docs"
	"github.com/luem2/sqlkit/internal/docs/sqlmetadata"
	"github.com/luem2/sqlkit/internal/docs/sqlresolve"
)

type docsFlags struct {
	output    string
	outputDir string
	css       string
	itype     int
	metadata  bool
	env       string
	database  string
}

func newDocsCommand(app *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Generate documentation from Markdown",
	}

	cmd.AddCommand(newDocsMermaidCommand(app))
	cmd.AddCommand(newDocsHTMLCommand(app))
	cmd.AddCommand(newDocsPDFCommand(app))
	cmd.AddCommand(newDocsSQLCommand(app))

	return cmd
}

func newDocsSQLCommand(app *appContext) *cobra.Command {
	flags := &docsFlags{}
	cmd := &cobra.Command{
		Use:   "sql <file.sql|procedure-name>",
		Short: "Generate Markdown documentation from a SQL Server procedure",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source, err := sqlresolve.Resolve(app.cfg.Root, args[0])
			if err != nil {
				return err
			}
			doc, err := docs.AnalyzeSQLProcedure(source, flags.itype)
			if err != nil {
				return err
			}
			shouldEnrich, err := shouldEnrichSQLDoc(flags)
			if err != nil {
				return err
			}
			if shouldEnrich && len(resultColumns(doc)) > 0 {
				if err := enrichSQLDocFromMetadata(cmd, app, flags, doc); err != nil {
					return err
				}
			}
			output := sqlDocOutputPath(app, flags, source)
			output, err = docs.WriteSQLMarkdown(doc, output)
			if err != nil {
				return err
			}
			successf(cmd.OutOrStdout(), "Markdown: %s", output)
			return nil
		},
	}
	addDocsOutputDirFlag(cmd, flags)
	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "output markdown file")
	cmd.Flags().IntVar(&flags.itype, "itype", 0, "document only one @iType block")
	cmd.Flags().BoolVar(&flags.metadata, "metadata", false, "query SQL Server metadata to resolve result column types")
	cmd.Flags().StringVar(&flags.env, "env", "", "environment for --metadata")
	cmd.Flags().StringVar(&flags.database, "database", "", "database name for --metadata")
	return cmd
}

func sqlDocOutputPath(app *appContext, flags *docsFlags, source string) string {
	if strings.TrimSpace(flags.output) != "" {
		return flags.output
	}
	outputDir := flags.outputDir
	if strings.TrimSpace(outputDir) == "" {
		outputDir = resolveRepoPath(app, filepath.Join(firstNonEmpty(app.cfg.Paths["docs"], "docs"), "generated"))
	}
	return docs.OutputPath(outputDir, source, ".md")
}

func shouldEnrichSQLDoc(flags *docsFlags) (bool, error) {
	hasEnv := strings.TrimSpace(flags.env) != ""
	hasDatabase := strings.TrimSpace(flags.database) != ""
	if flags.metadata || hasEnv || hasDatabase {
		if !hasEnv {
			return false, fmt.Errorf("--env is required when SQL metadata is requested")
		}
		if !hasDatabase {
			return false, fmt.Errorf("--database is required when SQL metadata is requested")
		}
		return true, nil
	}
	return false, nil
}

func resultColumns(doc *docs.SQLProcedureDoc) []docs.SQLResultColumn {
	var results []docs.SQLResultColumn
	for _, block := range doc.Blocks {
		for _, resultSet := range block.ResultSets {
			results = append(results, resultSet.Columns...)
		}
	}
	return results
}

func enrichSQLDocFromMetadata(cmd *cobra.Command, app *appContext, flags *docsFlags, doc *docs.SQLProcedureDoc) error {
	tables := docs.ReferencedSQLTables(doc)
	if len(tables) == 0 {
		return nil
	}
	db, err := newDBService(cmd, app, flags.env)
	if err != nil {
		return err
	}
	resultSets, err := db.queryIn(flags.database, sqlmetadata.Statement(tables))
	if err != nil {
		return err
	}
	columns := sqlmetadata.FromResultSets(resultSets)
	docs.EnrichSQLDocMetadata(doc, columns)
	return nil
}

func newDocsMermaidCommand(app *appContext) *cobra.Command {
	flags := &docsFlags{}
	cmd := &cobra.Command{
		Use:   "mermaid <file.md>",
		Short: "Extract Mermaid blocks and render them to SVG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outputDir := resolveDocsOutputDir(args[0], flags.outputDir)
			source, err := prepareAndRenderMermaid(cmd, app, args[0], outputDir)
			if err != nil {
				return err
			}
			successf(cmd.OutOrStdout(), "Prepared markdown: %s", source)
			return nil
		},
	}
	addDocsOutputDirFlag(cmd, flags)
	return cmd
}

func newDocsHTMLCommand(app *appContext) *cobra.Command {
	flags := &docsFlags{}
	cmd := &cobra.Command{
		Use:   "html <file.md>",
		Short: "Generate HTML from Markdown with Mermaid diagrams",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outputDir := resolveDocsOutputDir(args[0], flags.outputDir)
			output := defaultString(flags.output, docs.OutputPath(outputDir, args[0], ".html"))
			source, err := prepareAndRenderMermaid(cmd, app, args[0], outputDir)
			if err != nil {
				return err
			}
			css, err := resolveDocsCSS(outputDir, flags.css)
			if err != nil {
				return err
			}
			if err := runPandoc(cmd, app, source, output, css); err != nil {
				return err
			}
			successf(cmd.OutOrStdout(), "HTML: %s", output)
			return nil
		},
	}
	addDocsOutputFlags(cmd, flags)
	return cmd
}

func newDocsPDFCommand(app *appContext) *cobra.Command {
	flags := &docsFlags{}
	cmd := &cobra.Command{
		Use:   "pdf <file.md>",
		Short: "Generate PDF from Markdown with Mermaid diagrams",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outputDir := resolveDocsOutputDir(args[0], flags.outputDir)
			htmlOutput := docs.OutputPath(outputDir, args[0], ".html")
			pdfOutput := defaultString(flags.output, docs.OutputPath(outputDir, args[0], ".pdf"))
			source, err := prepareAndRenderMermaid(cmd, app, args[0], outputDir)
			if err != nil {
				return err
			}
			css, err := resolveDocsCSS(outputDir, flags.css)
			if err != nil {
				return err
			}
			if err := runPandoc(cmd, app, source, htmlOutput, css); err != nil {
				return err
			}
			if err := runChromePDF(cmd, app, htmlOutput, pdfOutput); err != nil {
				return err
			}
			successf(cmd.OutOrStdout(), "PDF: %s", pdfOutput)
			return nil
		},
	}
	addDocsOutputFlags(cmd, flags)
	return cmd
}

func addDocsOutputDirFlag(cmd *cobra.Command, flags *docsFlags) {
	cmd.Flags().StringVar(&flags.outputDir, "output-dir", "", "directory for generated intermediate files")
}

func addDocsOutputFlags(cmd *cobra.Command, flags *docsFlags) {
	addDocsOutputDirFlag(cmd, flags)
	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "output file")
	cmd.Flags().StringVar(&flags.css, "css", "", "optional CSS file for Pandoc HTML output")
}

func prepareAndRenderMermaid(cmd *cobra.Command, app *appContext, sourcePath string, outputDir string) (string, error) {
	prepared, err := docs.PrepareMarkdown(sourcePath, outputDir)
	if err != nil {
		return "", err
	}
	source, err := docs.WritePreparedMarkdown(prepared)
	if err != nil {
		return "", err
	}
	if len(prepared.Blocks) == 0 {
		return source, nil
	}

	for _, block := range prepared.Blocks {
		input := filepath.Join(outputDir, "diagrams", block.Name+".mmd")
		output := filepath.Join(outputDir, "diagrams", block.Name+".svg")
		result := newProcessService(cmd, app).Run(app.cfg.ToolPath("mmdc"), "-i", input, "-o", output, "-b", "transparent")
		if result.ExitCode != 0 {
			return "", fmt.Errorf("mmdc failed for %s with exit code %d\n%s", input, result.ExitCode, firstNonEmpty(result.Stderr, result.Stdout))
		}
	}

	return source, nil
}

func runPandoc(cmd *cobra.Command, app *appContext, source string, output string, css string) error {
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}

	args := []string{source, "--standalone", "--metadata", "pagetitle=" + pageTitle(source), "--output", output}
	if strings.TrimSpace(css) != "" {
		args = append(args, "--css", css)
	}

	result := newProcessService(cmd, app).Run(app.cfg.ToolPath("pandoc"), args...)
	if result.ExitCode != 0 {
		return fmt.Errorf("pandoc failed with exit code %d\n%s", result.ExitCode, firstNonEmpty(result.Stderr, result.Stdout))
	}
	return nil
}

func runChromePDF(cmd *cobra.Command, app *appContext, htmlOutput string, pdfOutput string) error {
	if err := os.MkdirAll(filepath.Dir(pdfOutput), 0755); err != nil {
		return err
	}

	htmlAbs, err := filepath.Abs(htmlOutput)
	if err != nil {
		return err
	}
	pdfAbs, err := filepath.Abs(pdfOutput)
	if err != nil {
		return err
	}

	chrome := deps.ChromeCommand(app.cfg)
	result := newProcessService(cmd, app).Run(chrome,
		"--headless",
		"--disable-gpu",
		"--no-sandbox",
		"--no-pdf-header-footer",
		"--print-to-pdf="+pdfAbs,
		"file://"+htmlAbs,
	)
	if result.ExitCode != 0 {
		return fmt.Errorf("chrome failed with exit code %d\n%s", result.ExitCode, firstNonEmpty(result.Stderr, result.Stdout))
	}
	return nil
}

func resolveDocsOutputDir(sourcePath string, outputDir string) string {
	return defaultString(outputDir, docs.DefaultOutputDir(sourcePath))
}

func resolveDocsCSS(outputDir string, css string) (string, error) {
	if strings.TrimSpace(css) != "" {
		return css, nil
	}
	return docs.WriteDefaultCSS(outputDir)
}

func pageTitle(source string) string {
	base := filepath.Base(source)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
