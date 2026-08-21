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

func usage() string {
	return `Navune — structural code-quality analysis.

Usage:
  navune analyze <path> [--config file] [--format text|json|mermaid] [--out file]
  navune init    [path]     write a commented navune.yaml template
  navune version            print version and supported languages

Exit codes:
  0  pass (all error-tier budgets satisfied)
  1  breach (at least one error-tier budget exceeded)
  2  usage or configuration error
`
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage())
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
		fmt.Print(usage())
		return gate.ExitPass
	default:
		fmt.Fprintf(os.Stderr, "navune: unknown command %q\n\n%s", args[0], usage())
		return gate.ExitUsage
	}
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
			fmt.Print(usage())
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
		fmt.Fprintf(os.Stderr, "navune analyze: %v\n", err)
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
		fmt.Println(out) // keep stdout useful too
	} else {
		fmt.Println(out)
	}
	return rpt.ExitCode
}

func runInit(args []string) int {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "navune init: too many arguments\n")
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
	fmt.Println("languages: go")
	return gate.ExitPass
}
