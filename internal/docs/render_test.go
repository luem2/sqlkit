package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareMarkdownExtractsMermaid(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "doc.md")
	content := "# Doc\n\n```mermaid\nflowchart TD\nA-->B\n```\n"
	if err := os.WriteFile(source, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	prepared, err := PrepareMarkdown(source, filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}

	if len(prepared.Blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(prepared.Blocks))
	}
	if !strings.Contains(prepared.Markdown, "diagrams/diagram-01.svg") {
		t.Fatalf("prepared markdown did not link svg: %s", prepared.Markdown)
	}
}

func TestOutputPathUsesProvidedOutputDir(t *testing.T) {
	got := OutputPath(filepath.Join("custom", "out"), filepath.Join("docs", "refactor.md"), ".pdf")
	want := filepath.Join("custom", "out", "refactor.pdf")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteDefaultCSS(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteDefaultCSS(dir)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "@page") {
		t.Fatalf("default css does not contain print rules: %s", string(content))
	}
}
