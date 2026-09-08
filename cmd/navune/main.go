// Command navune analyzes a codebase's structural quality.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/lcr/navune/internal/analysis"
	"github.com/lcr/navune/internal/config"
	"github.com/lcr/navune/internal/gate"
	"github.com/lcr/navune/internal/report"
)

const version = "0.1.0"

func main() {
	os.Exit(run(os.Args[1:]))
}

// helpText is the top-level help shown by `navune help` (and on usage errors).
func helpText() string {
	return `Navune — structural code-quality analysis in one CLI.

Usage:
  navune <command> [arguments]

Commands:
  analyze     Measure a codebase's structural quality against budgets
  init        Write a commented navune.yaml template
  version     Print version and supported languages
  help        Show help for a command

Run "navune help <command>" or "navune <command> --help" for details.

Examples:
  navune analyze .                    analyze the current directory
  navune analyze ./src --format json
  navune analyze . --format mermaid --out deps.mmd
  navune init

Exit codes:
  0  pass (all error-tier budgets satisfied)
  1  breach (at least one error-tier budget exceeded)
  2  usage or configuration error
  3  internal error (e.g. a source file could not be parsed)

Full reference: docs/usage.md in this repository.
`
}

// helpFor returns per-command help text, or "" for unknown commands.
func helpFor(cmd string) string {
	switch cmd {
	case "analyze":
		return `NAME
  navune analyze — measure a codebase's structural quality

USAGE
  navune analyze <path> [flags]

ARGUMENTS
  <path>          directory to analyze (default ".")

FLAGS
  --config FILE   navune.yaml to use; overrides upward discovery from <path>
  --format FMT    report format: text (default), json, or mermaid
  --out FILE      write the report to FILE instead of stdout

DESCRIPTION
  Analyzes the codebase at <path>, measures size, cyclomatic complexity,
  duplication, dependency cycles and coupling, evaluates budgets, and prints
  a report. Exit code 1 signals an error-tier budget breach; exit code 3
  signals an internal error (see "navune help" for all exit codes).

EXAMPLES
  navune analyze .
  navune analyze ./src --format json
  navune analyze . --config navune.yaml --format mermaid --out deps.mmd
`
	case "init":
		return `NAME
  navune init — write a commented navune.yaml template

USAGE
  navune init [path]

ARGUMENTS
  [path]          directory to write navune.yaml into (default ".")

DESCRIPTION
  Scaffolds a commented configuration file with the built-in budgets and
  weights. Fails if navune.yaml already exists in the directory.

EXAMPLES
  navune init
  navune init ./services/api
`
	case "version":
		return `NAME
  navune version — print version, platform and supported languages

USAGE
  navune version
`
	case "help":
		return helpText()
	}
	return ""
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, helpText())
		return gate.ExitUsage
	}
	switch args[0] {
	case "analyze":
		return runAnalyze(args[1:])
	case "init":
		return runInit(args[1:])
	case "version", "--version", "-v":
		return runVersion()
	case "help", "--help", "-h":
		return runHelp(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "navune: unknown command %q\n\n%s", args[0], helpText())
		return gate.ExitUsage
	}
}

// runHelp implements `navune help [command]`.
func runHelp(args []string) int {
	if len(args) == 0 {
		fmt.Print(helpText())
		return gate.ExitPass
	}
	h := helpFor(args[0])
	if h == "" {
		fmt.Fprintf(os.Stderr, "navune: unknown command %q\n\n%s", args[0], helpText())
		return gate.ExitUsage
	}
	fmt.Print(h)
	return gate.ExitPass
}

type analyzeOpts struct {
	path   string
	config string
	format report.Format
	out    string
}

func parseAnalyze(args []string) (*analyzeOpts, int, error) {
	opts := &analyzeOpts{format: report.Text}
	i := 0
	next := func(flag string) (string, bool) {
		if i+1 < len(args) {
			i++
			return args[i], true
		}
		return "", false
	}
	for i < len(args) {
		a := args[i]
		switch a {
		case "--config":
			v, ok := next(a)
			if !ok {
				return nil, gate.ExitUsage, fmt.Errorf("%s requires a value", a)
			}
			opts.config = v
		case "--format":
			v, ok := next(a)
			if !ok {
				return nil, gate.ExitUsage, fmt.Errorf("%s requires a value", a)
			}
			f, err := report.ParseFormat(v)
			if err != nil {
				return nil, gate.ExitUsage, err
			}
			opts.format = f
		case "--out":
			v, ok := next(a)
			if !ok {
				return nil, gate.ExitUsage, fmt.Errorf("%s requires a value", a)
			}
			opts.out = v
		case "-h", "--help":
			fmt.Print(helpFor("analyze"))
			return nil, gate.ExitPass, errHelp
		default:
			if opts.path != "" {
				return nil, gate.ExitUsage, fmt.Errorf("unexpected argument %q", a)
			}
			if a != "" && a[0] == '-' {
				return nil, gate.ExitUsage, fmt.Errorf("unknown flag %q", a)
			}
			opts.path = a
		}
		i++
	}
	if opts.path == "" {
		opts.path = "."
	}
	return opts, 0, nil
}

var errHelp = fmt.Errorf("help requested")

func runAnalyze(args []string) int {
	opts, _, err := parseAnalyze(args)
	if err != nil {
		if err == errHelp {
			return gate.ExitPass
		}
		fmt.Fprintf(os.Stderr, "navune analyze: %v\n\n%s", err, helpFor("analyze"))
		return gate.ExitUsage
	}

	// config: explicit file or upward discovery from the target.
	cfgPath := opts.config
	if cfgPath == "" {
		cfgPath, _ = config.Discover(opts.path)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "navune: config error: %v\n", err)
		return gate.ExitUsage
	}

	rpt, err := analysis.Analyze(opts.path, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "navune: %v\n", err)
		return gate.ExitInternal
	}

	out, err := report.Render(rpt, opts.format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "navune: %v\n", err)
		return gate.ExitInternal
	}
	if opts.out != "" {
		if err := os.WriteFile(opts.out, []byte(out), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "navune: %v\n", err)
			return gate.ExitInternal
		}
		return rpt.ExitCode
	}
	fmt.Println(out)
	return rpt.ExitCode
}

func runInit(args []string) int {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Print(helpFor("init"))
		return gate.ExitPass
	}
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "navune init: too many arguments\n\n%s", helpFor("init"))
		return gate.ExitUsage
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "navune init: %v\n", err)
		return gate.ExitUsage
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		fmt.Fprintf(os.Stderr, "navune init: %q is not a directory\n", dir)
		return gate.ExitUsage
	}
	out := filepath.Join(abs, config.ConfigName)
	if _, err := os.Stat(out); err == nil {
		fmt.Fprintf(os.Stderr, "navune init: %s already exists\n", out)
		return gate.ExitUsage
	}
	if err := os.WriteFile(out, []byte(config.Template()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "navune init: %v\n", err)
		return gate.ExitInternal
	}
	fmt.Printf("wrote %s\n", out)
	return gate.ExitPass
}

func runVersion() int {
	fmt.Printf("navune %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	fmt.Println("languages: go, typescript, javascript, python")
	return gate.ExitPass
}
