package confirm

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func Exact(reader io.Reader, writer io.Writer, expected string) error {
	fmt.Fprintf(writer, "Type %s to confirm: ", expected)

	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return err
		}
		return fmt.Errorf("confirmation canceled")
	}

	if strings.TrimSpace(scanner.Text()) != expected {
		return fmt.Errorf("confirmation mismatch")
	}

	return nil
}

func ExactFold(reader io.Reader, writer io.Writer, expected string) error {
	fmt.Fprintf(writer, "Type %s to confirm: ", expected)

	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return err
		}
		return fmt.Errorf("confirmation canceled")
	}

	if !strings.EqualFold(strings.TrimSpace(scanner.Text()), expected) {
		return fmt.Errorf("confirmation mismatch")
	}

	return nil
}
