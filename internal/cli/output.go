package cli

import (
	"io"

	"github.com/pterm/pterm"
)

func infof(writer io.Writer, format string, args ...interface{}) {
	pterm.Info.WithWriter(writer).Printfln(format, args...)
}

func successf(writer io.Writer, format string, args ...interface{}) {
	pterm.Success.WithWriter(writer).Printfln(format, args...)
}

func warnf(writer io.Writer, format string, args ...interface{}) {
	pterm.Warning.WithWriter(writer).Printfln(format, args...)
}
