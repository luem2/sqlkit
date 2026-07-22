package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type MermaidBlock struct {
	Index int
	Name  string
	Body  string
}

type PreparedMarkdown struct {
	SourcePath string
	OutputDir  string
	Markdown   string
	Blocks     []MermaidBlock
}

var mermaidBlockRegex = regexp.MustCompile("(?s)```mermaid\\s*\\n(.*?)```")

func PrepareMarkdown(sourcePath string, outputDir string) (*PreparedMarkdown, error) {
	if strings.TrimSpace(sourcePath) == "" {
		return nil, fmt.Errorf("source markdown is required")
	}
	if strings.TrimSpace(outputDir) == "" {
		return nil, fmt.Errorf("output directory is required")
	}

	absoluteSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, err
	}

	contentBytes, err := os.ReadFile(absoluteSource)
	if err != nil {
		return nil, err
	}
	content := string(contentBytes)

	var blocks []MermaidBlock
	index := 0
	rendered := mermaidBlockRegex.ReplaceAllStringFunc(content, func(match string) string {
		index++
		submatch := mermaidBlockRegex.FindStringSubmatch(match)
		body := ""
		if len(submatch) > 1 {
			body = strings.TrimSpace(submatch[1])
		}

		name := fmt.Sprintf("diagram-%02d", index)
		blocks = append(blocks, MermaidBlock{
			Index: index,
			Name:  name,
			Body:  body,
		})

		return fmt.Sprintf("![Diagrama %d](diagrams/%s.svg)", index, name)
	})

	return &PreparedMarkdown{
		SourcePath: absoluteSource,
		OutputDir:  outputDir,
		Markdown:   rendered,
		Blocks:     blocks,
	}, nil
}

func WritePreparedMarkdown(prepared *PreparedMarkdown) (string, error) {
	if err := os.MkdirAll(prepared.OutputDir, 0755); err != nil {
		return "", err
	}

	diagramDir := filepath.Join(prepared.OutputDir, "diagrams")
	if err := os.MkdirAll(diagramDir, 0755); err != nil {
		return "", err
	}

	for _, block := range prepared.Blocks {
		path := filepath.Join(diagramDir, block.Name+".mmd")
		if err := os.WriteFile(path, []byte(block.Body+"\n"), 0644); err != nil {
			return "", err
		}
	}

	sourcePath := filepath.Join(prepared.OutputDir, "source.md")
	if err := os.WriteFile(sourcePath, []byte(prepared.Markdown), 0644); err != nil {
		return "", err
	}

	return sourcePath, nil
}

func WriteDefaultCSS(outputDir string) (string, error) {
	path := filepath.Join(outputDir, "print.css")
	content := `:root {
  color-scheme: light;
  font-family: Arial, sans-serif;
  font-size: 14px;
  line-height: 1.45;
}

body {
  margin: 32px auto;
  max-width: 920px;
  color: #202124;
}

h1, h2, h3 {
  break-after: avoid;
  color: #111827;
}

h1 {
  border-bottom: 1px solid #d1d5db;
  padding-bottom: 0.3rem;
}

pre, code {
  font-family: Consolas, "Liberation Mono", monospace;
}

pre {
  background: #f3f4f6;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  overflow-x: auto;
  padding: 12px;
}

table {
  border-collapse: collapse;
  width: 100%;
}

th, td {
  border: 1px solid #d1d5db;
  padding: 6px 8px;
}

img {
  break-inside: avoid;
  display: block;
  margin: 12px auto;
  max-height: 720px;
  max-width: 100%;
}

@page {
  margin: 16mm;
}
`
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func DefaultOutputDir(sourcePath string) string {
	base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	return filepath.Join(filepath.Dir(sourcePath), "generated", base)
}

func DefaultOutputPath(sourcePath string, extension string) string {
	return OutputPath(DefaultOutputDir(sourcePath), sourcePath, extension)
}

func OutputPath(outputDir string, sourcePath string, extension string) string {
	base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	return filepath.Join(outputDir, base+extension)
}
