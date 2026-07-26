package confirm

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func Exact(reader io.Reader, writer io.Writer, expected string) error {
	if _, err := fmt.Fprintf(writer, "Type %s to confirm: ", expected); err != nil {
		return err
	}

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
	if _, err := fmt.Fprintf(writer, "Type %s to confirm: ", expected); err != nil {
		return err
	}

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
