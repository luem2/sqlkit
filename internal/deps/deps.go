package deps

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/luem2/sqlkit/internal/config"
	"github.com/luem2/sqlkit/internal/process"
)

type Dependency struct {
	Name       string
	Command    string
	Args       []string
	Suggestion string
}

type Status struct {
	Name       string
	Command    string
	Path       string
	OK         bool
	Version    string
	Suggestion string
}

type InstallPlan struct {
	Name     string
	Commands [][]string
	Notes    []string
}

func DefaultDependencies(cfg *config.Config) []Dependency {
	return []Dependency{
		{Name: "go", Command: cfg.ToolPath("go"), Args: []string{"version"}, Suggestion: "Install Go from https://go.dev/dl/ or your OS package manager."},
		{Name: "dotnet", Command: cfg.ToolPath("dotnet"), Args: []string{"--version"}, Suggestion: "Install .NET SDK from https://dotnet.microsoft.com/download."},
		{Name: "sqlcmd", Command: cfg.ToolPath("sqlcmd"), Args: []string{"-?"}, Suggestion: "Install sqlcmd with Microsoft instructions or your OS package manager."},
		{Name: "sqlpackage", Command: cfg.ToolPath("sqlpackage"), Args: []string{"/Version"}, Suggestion: "Install SqlPackage as a dotnet tool or from Microsoft downloads."},
		{Name: "pandoc", Command: cfg.ToolPath("pandoc"), Args: []string{"--version"}, Suggestion: "Install Pandoc from https://pandoc.org/installing.html."},
		{Name: "node", Command: cfg.ToolPath("node"), Args: []string{"--version"}, Suggestion: "Install Node.js from https://nodejs.org/ or your version manager."},
		{Name: "npm", Command: cfg.ToolPath("npm"), Args: []string{"--version"}, Suggestion: "Install npm with Node.js."},
		{Name: "mmdc", Command: cfg.ToolPath("mmdc"), Args: []string{"--version"}, Suggestion: "Install Mermaid CLI with npm install -g @mermaid-js/mermaid-cli."},
		{Name: "chrome", Command: ChromeCommand(cfg), Args: []string{"--version"}, Suggestion: "Install Google Chrome or Chromium."},
	}
}

func PlanInstall(name string) (InstallPlan, error) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch normalized {
	case "sqlpackage":
		return InstallPlan{
			Name: "sqlpackage",
			Commands: [][]string{
				{"dotnet", "tool", "install", "-g", "microsoft.sqlpackage", "--allow-roll-forward"},
			},
			Notes: []string{
				"Requires .NET SDK.",
				"If sqlpackage is already installed, use: dotnet tool update -g microsoft.sqlpackage",
			},
		}, nil
	case "mmdc", "mermaid", "mermaid-cli":
		return InstallPlan{
			Name: "mmdc",
			Commands: [][]string{
				{"npm", "install", "-g", "@mermaid-js/mermaid-cli"},
			},
			Notes: []string{"Requires Node.js and npm."},
		}, nil
	case "sqlcmd":
		return InstallPlan{
			Name: "sqlcmd",
			Notes: []string{
				"Install sqlcmd from Microsoft packages for your OS.",
				"Linux usually uses mssql-tools18; Windows can use winget or the Microsoft installer.",
			},
		}, nil
	case "pandoc":
		return InstallPlan{
			Name: "pandoc",
			Notes: []string{
				"Install Pandoc from https://pandoc.org/installing.html or your OS package manager.",
			},
		}, nil
	case "chrome", "chromium":
		return InstallPlan{
			Name: "chrome",
			Notes: []string{
				"Install Google Chrome or Chromium with your OS package manager.",
			},
		}, nil
	case "go":
		return InstallPlan{Name: "go", Notes: []string{"Install Go from https://go.dev/dl/."}}, nil
	case "dotnet":
		return InstallPlan{Name: "dotnet", Notes: []string{"Install the .NET SDK from https://dotnet.microsoft.com/download."}}, nil
	case "node", "npm":
		return InstallPlan{Name: normalized, Notes: []string{"Install Node.js from https://nodejs.org/ or your version manager."}}, nil
	default:
		return InstallPlan{}, fmt.Errorf("unsupported dependency %q", name)
	}
}

func Check(ctx context.Context, dep Dependency) Status {
	status := Status{
		Name:       dep.Name,
		Command:    dep.Command,
		Suggestion: dep.Suggestion,
	}

	path, err := exec.LookPath(dep.Command)
	if err != nil {
		return status
	}
	status.Path = path
	status.OK = true

	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result := process.Run(checkCtx, dep.Command, dep.Args...)
	status.Version = firstLine(firstNonEmpty(result.Stdout, result.Stderr))
	return status
}

func FormatStatus(status Status) string {
	if status.OK {
		version := status.Version
		if version == "" {
			version = "found"
		}
		return fmt.Sprintf("OK      %-12s %s", status.Name, version)
	}
	return fmt.Sprintf("MISSING %-12s %s", status.Name, status.Suggestion)
}

func ChromeCommand(cfg *config.Config) string {
	for _, name := range []string{"chrome", "google-chrome", "chromium", "chromium-browser", "google-chrome-stable"} {
		if value := cfg.ToolPath(name); value != name {
			return value
		}
	}

	if runtime.GOOS == "windows" {
		return "chrome"
	}

	for _, name := range []string{"google-chrome", "chromium", "chromium-browser", "google-chrome-stable"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}

	return "google-chrome"
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	line, _, _ := strings.Cut(value, "\n")
	return strings.TrimSpace(line)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
