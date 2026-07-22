package cli

import (
	"fmt"
	"strings"

	"github.com/luem2/sqlkit/internal/ssdt"
)

func sqlPackageError(action string, exitCode int, output string) error {
	output = strings.TrimSpace(output)
	hint := ssdt.ExplainSQLPackageError(output)
	if hint != "" {
		hint = "\nHint: " + hint
	}
	if output == "" {
		return fmt.Errorf("sqlpackage %s failed with exit code %d%s", action, exitCode, hint)
	}
	return fmt.Errorf("sqlpackage %s failed with exit code %d\n%s%s", action, exitCode, output, hint)
}
