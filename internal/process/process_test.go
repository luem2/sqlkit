package process

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRedactMasksConfiguredSecrets(t *testing.T) {
	got := redact("before secret after", []string{"secret"})
	if got != "before [REDACTED] after" {
		t.Fatalf("redact() = %q", got)
	}
}

func TestRedactSkipsEmptySecrets(t *testing.T) {
	got := redact("value", []string{""})
	if got != "value" {
		t.Fatalf("redact() = %q", got)
	}
}

func TestDecodeProcessOutputKeepsUTF8(t *testing.T) {
	got := decodeProcessOutput([]byte("sesión"))
	if got != "sesión" {
		t.Fatalf("decodeProcessOutput() = %q", got)
	}
}

func TestDecodeProcessOutputFallsBackToCodePage850(t *testing.T) {
	got := decodeProcessOutput([]byte{'s', 'e', 's', 'i', 0xA2, 'n'})
	if got != "sesión" {
		t.Fatalf("decodeProcessOutput() = %q", got)
	}
}

func TestRunOptionsUsesWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	var result Result
	if runtime.GOOS == "windows" {
		result = RunOptions(context.Background(), Options{WorkingDirectory: dir}, "cmd", "/c", "cd")
	} else {
		result = RunOptions(context.Background(), Options{WorkingDirectory: dir}, "pwd")
	}
	if result.ExitCode != 0 {
		t.Fatalf("command failed: %s", result.Stderr)
	}
	got, err := filepath.Abs(result.Stdout)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	gotInfo, gotErr := os.Stat(got)
	wantInfo, wantErr := os.Stat(want)
	if gotErr != nil || wantErr != nil || !os.SameFile(gotInfo, wantInfo) {
		t.Fatalf("working directory = %q, want %q", got, want)
	}
}
