package cli

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/luem2/sqlkit/internal/sqlserver"
)

func printResultSets(writer io.Writer, resultSets []sqlserver.ResultSet) {
	for index, resultSet := range resultSets {
		if len(resultSet.Columns) == 0 {
			continue
		}
		if index > 0 {
			_, _ = fmt.Fprintln(writer)
		}
		rows := make([][]string, 0, len(resultSet.Rows)+1)
		rows = append(rows, resultSet.Columns)
		rows = append(rows, resultSet.Rows...)
		printTable(writer, rows)
	}
}

func printTable(writer io.Writer, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	widths := tableWidths(rows)

	for index, row := range rows {
		printTableRow(writer, row, widths)
		if index == 0 {
			printTableSeparator(writer, widths)
		}
	}
}

func tableWidths(rows [][]string) []int {
	var widths []int
	for _, row := range rows {
		for index, value := range row {
			if index >= len(widths) {
				widths = append(widths, 0)
			}
			if width := utf8.RuneCountInString(value); width > widths[index] {
				widths[index] = width
			}
		}
	}
	return widths
}

func printTableRow(writer io.Writer, row []string, widths []int) {
	for index, width := range widths {
		value := ""
		if index < len(row) {
			value = row[index]
		}
		if index > 0 {
			_, _ = fmt.Fprint(writer, "  ")
		}
		_, _ = fmt.Fprintf(writer, "%-*s", width, value)
	}
	_, _ = fmt.Fprintln(writer)
}

func printTableSeparator(writer io.Writer, widths []int) {
	for index, width := range widths {
		if index > 0 {
			_, _ = fmt.Fprint(writer, "  ")
		}
		_, _ = fmt.Fprint(writer, strings.Repeat("-", width))
	}
	_, _ = fmt.Fprintln(writer)
}
