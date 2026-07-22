package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func RenderSQLMarkdown(doc *SQLProcedureDoc) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# %s\n\n", doc.Name)

	if len(doc.Blocks) == 0 {
		fmt.Fprintf(&builder, "> No se detectaron bloques `IF (@iType = N)`.\n")
		return strings.TrimRight(builder.String(), "\n") + "\n"
	}

	for _, block := range doc.Blocks {
		fmt.Fprintf(&builder, "## `@iType = %d`\n\n", block.Type)
		fmt.Fprintf(&builder, "### Descripcion\n\n")
		if strings.TrimSpace(block.Description) != "" {
			fmt.Fprintf(&builder, "> %s\n\n", block.Description)
		} else {
			fmt.Fprintf(&builder, "> TODO: Explicar que hace este bloque.\n\n")
		}

		fmt.Fprintf(&builder, "### Parametros de entrada usados\n\n")
		renderParameterTable(&builder, block.Parameters)
		fmt.Fprintf(&builder, "\n")

		fmt.Fprintf(&builder, "### Parametros de salida / escalares\n\n")
		renderParameterTable(&builder, block.Outputs)
		fmt.Fprintf(&builder, "\n")

		fmt.Fprintf(&builder, "### Resultados `SELECT`\n\n")
		renderResultSets(&builder, block)
		fmt.Fprintf(&builder, "\n")
	}

	return strings.TrimRight(builder.String(), "\n") + "\n"
}

func WriteSQLMarkdown(doc *SQLProcedureDoc, output string) (string, error) {
	if strings.TrimSpace(output) == "" {
		output = DefaultOutputPath(doc.SourcePath, ".md")
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(output, []byte(RenderSQLMarkdown(doc)), 0644); err != nil {
		return "", err
	}
	return output, nil
}

func renderParameterTable(builder *strings.Builder, params []SQLParameter) {
	if len(params) == 0 {
		fmt.Fprintf(builder, "> No detectado.\n")
		return
	}
	fmt.Fprintf(builder, "| Parametro | Tipo | Requerido | Default | Observaciones |\n")
	fmt.Fprintf(builder, "|---|---|---|---|---|\n")
	for _, param := range params {
		required := "Si"
		if param.Output {
			required = "Salida"
		} else if !param.Required {
			required = "No"
		}
		defaultValue := param.Default
		if defaultValue == "" {
			defaultValue = "-"
		}
		fmt.Fprintf(builder, "| `%s` | `%s` | %s | `%s` | %s |\n",
			escapeMarkdown(param.Name),
			escapeMarkdown(param.Type),
			required,
			escapeMarkdown(defaultValue),
			escapeMarkdown(param.Observation),
		)
	}
}

func renderResultSets(builder *strings.Builder, block SQLTypeBlock) {
	if len(block.ResultSets) == 0 {
		renderResultTable(builder, nil)
		return
	}
	if len(block.ResultSets) == 1 {
		renderResultTable(builder, block.ResultSets[0].Columns)
		return
	}
	for i, resultSet := range block.ResultSets {
		fmt.Fprintf(builder, "#### Result set %d\n\n", i+1)
		renderResultTable(builder, resultSet.Columns)
		if i+1 < len(block.ResultSets) {
			fmt.Fprintf(builder, "\n")
		}
	}
}

func renderResultTable(builder *strings.Builder, results []SQLResultColumn) {
	if len(results) == 0 {
		fmt.Fprintf(builder, "> No se detecto un `SELECT` de salida.\n")
		return
	}
	fmt.Fprintf(builder, "| Campo | Tipo | Expresion |\n")
	fmt.Fprintf(builder, "|---|---|---|\n")
	for _, result := range results {
		fmt.Fprintf(builder, "| `%s` | `%s` | `%s` |\n",
			escapeMarkdown(result.Name),
			escapeMarkdown(result.Type),
			escapeMarkdown(result.Expression),
		)
	}
}
