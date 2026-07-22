package process

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Options struct {
	Redactions       []string
	Env              []string
	WorkingDirectory string
}

func Run(ctx context.Context, name string, args ...string) Result {
	return run(ctx, nil, nil, Options{}, name, args...)
}

func RunStreaming(ctx context.Context, stdout io.Writer, stderr io.Writer, name string, args ...string) Result {
	return run(ctx, stdout, stderr, Options{}, name, args...)
}

func RunOptions(ctx context.Context, options Options, name string, args ...string) Result {
	return run(ctx, nil, nil, options, name, args...)
}

func RunStreamingOptions(ctx context.Context, stdout io.Writer, stderr io.Writer, options Options, name string, args ...string) Result {
	return run(ctx, stdout, stderr, options, name, args...)
}

func run(ctx context.Context, stdoutWriter io.Writer, stderrWriter io.Writer, options Options, name string, args ...string) Result {
	cmd := exec.CommandContext(ctx, name, args...)
	if options.WorkingDirectory != "" {
		cmd.Dir = options.WorkingDirectory
	}
	if len(options.Env) > 0 {
		cmd.Env = append(os.Environ(), options.Env...)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = captureWriter(&stdout, stdoutWriter, options.Redactions)
	cmd.Stderr = captureWriter(&stderr, stderrWriter, options.Redactions)

	err := cmd.Run()
	result := Result{
		Stdout: strings.TrimRight(redact(decodeProcessOutput(stdout.Bytes()), options.Redactions), "\r\n"),
		Stderr: strings.TrimRight(redact(decodeProcessOutput(stderr.Bytes()), options.Redactions), "\r\n"),
	}

	if err == nil {
		return result
	}

	if exitError, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitError.ExitCode()
		return result
	}

	result.ExitCode = -1
	result.Stderr = strings.TrimSpace(err.Error() + "\n" + result.Stderr)
	return result
}

func captureWriter(buffer *bytes.Buffer, writer io.Writer, redactions []string) io.Writer {
	if writer == nil {
		return buffer
	}
	return io.MultiWriter(redactingWriter{writer: writer, redactions: redactions}, buffer)
}

type redactingWriter struct {
	writer     io.Writer
	redactions []string
}

func (w redactingWriter) Write(p []byte) (int, error) {
	_, err := w.writer.Write([]byte(redact(decodeProcessOutput(p), w.redactions)))
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func decodeProcessOutput(value []byte) string {
	if utf8.Valid(value) {
		return string(value)
	}
	decoded, err := charmap.CodePage850.NewDecoder().String(string(value))
	if err != nil {
		return string(value)
	}
	return decoded
}

func redact(value string, redactions []string) string {
	for _, secret := range redactions {
		if strings.TrimSpace(secret) == "" {
			continue
		}
		value = strings.ReplaceAll(value, secret, "[REDACTED]")
	}
	return value
}
