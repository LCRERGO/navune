package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcr/navune/internal/config"
	"github.com/lcr/navune/internal/gate"
)

// runCLI executes the built binary against args and returns exit code + output.
func runCLI(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "navune")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	c := exec.Command(bin, args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	return code, string(out)
}

func TestVersion(t *testing.T) {
	code, out := runCLI(t, ".", "version")
	if code != gate.ExitPass {
		t.Fatalf("version exit want 0, got %d", code)
	}
	if !strings.Contains(out, "navune") {
		t.Errorf("version output missing name: %q", out)
	}
}

func TestInitWritesConfig(t *testing.T) {
	dir := t.TempDir()
	code, out := runCLI(t, dir, "init")
	if code != gate.ExitPass {
		t.Fatalf("init exit want 0, got %d\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, config.ConfigName)); err != nil {
		t.Errorf("navune.yaml missing: %v", err)
	}
}

func TestUnknownCommand(t *testing.T) {
	code, _ := runCLI(t, ".", "frobnicate")
	if code != gate.ExitUsage {
		t.Fatalf("unknown command want exit %d, got %d", gate.ExitUsage, code)
	}
}

func TestAnalyzeRepoDefaultBudgetsPass(t *testing.T) {
	code, out := runCLI(t, "..", "analyze", ".", "--format", "json")
	if code != gate.ExitPass {
		t.Fatalf("analyze of repo should pass default budgets, got exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "\"schema_version\": 1") {
		t.Errorf("json output missing schema_version")
	}
}

func TestAnalyzeBudgetBreachExitsOne(t *testing.T) {
	// Build a config with an impossible budget, then analyze the fixture.
	dir := filepath.Join("..", "..", "test", "fixture")
	cfgFile := filepath.Join(t.TempDir(), config.ConfigName)
	cfg := `version: 1
budgets:
  worst_complexity: { limit: 1, tier: error }
`
	if err := os.WriteFile(cfgFile, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := runCLI(t, dir, "analyze", ".", "--config", cfgFile, "--format", "json")
	if code != gate.ExitBreach {
		t.Fatalf("want exit %d for breach, got %d\n%s", gate.ExitBreach, code, out)
	}
	if !strings.Contains(out, "\"exit_code\": 1") {
		t.Errorf("json should report exit_code 1")
	}
}

func TestBadFormatIsUsageError(t *testing.T) {
	code, _ := runCLI(t, "..", "analyze", ".", "--format", "xml")
	if code != gate.ExitUsage {
		t.Fatalf("bad format want exit %d, got %d", gate.ExitUsage, code)
	}
}

func TestOutWritesFileWithoutEcho(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "report.json")
	code, stdout := runCLI(t, "..", "analyze", ".", "--format", "json", "--out", out)
	if code != gate.ExitPass {
		t.Fatalf("analyze with --out want exit 0, got %d", code)
	}
	if stdout != "" {
		t.Errorf("with --out, report must not be echoed to stdout, got:\n%s", stdout)
	}
	if b, err := os.ReadFile(out); err != nil || !strings.Contains(string(b), `"schema_version": 1`) {
		t.Errorf("report file missing or invalid: %v", err)
	}
}

func TestHelpRoot(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}} {
		code, out := runCLI(t, ".", args...)
		if code != gate.ExitPass {
			t.Errorf("%v exit want 0, got %d", args, code)
		}
		for _, want := range []string{"analyze", "init", "version", "Exit codes", "navune help <command>"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v output should contain %q", args, want)
			}
		}
	}
}

func TestHelpForEachCommand(t *testing.T) {
	for _, cmd := range []string{"analyze", "init", "version", "help"} {
		code, out := runCLI(t, ".", "help", cmd)
		if code != gate.ExitPass {
			t.Errorf("help %s exit want 0, got %d", cmd, code)
		}
		if !strings.Contains(out, "navune "+cmd) {
			t.Errorf("help %s should describe the command, got:\n%s", cmd, out)
		}
	}
	// --help on a subcommand renders the same per-command help
	code, out := runCLI(t, ".", "analyze", "--help")
	if code != gate.ExitPass {
		t.Errorf("analyze --help exit want 0, got %d", code)
	}
	for _, want := range []string{"--format FMT", "--out FILE", "EXAMPLES", "navune analyze ."} {
		if !strings.Contains(out, want) {
			t.Errorf("analyze --help should contain %q, got:\n%s", want, out)
		}
	}
}

func TestHelpUnknownTopicIsUsageError(t *testing.T) {
	code, _ := runCLI(t, ".", "help", "bogus")
	if code != gate.ExitUsage {
		t.Fatalf("help bogus want exit %d, got %d", gate.ExitUsage, code)
	}
}

func TestNoArgsPrintsHelpToStderr(t *testing.T) {
	code, out := runCLI(t, ".")
	if code != gate.ExitUsage {
		t.Fatalf("no args want exit %d, got %d", gate.ExitUsage, code)
	}
	if !strings.Contains(out, "Usage:") {
		t.Errorf("no-args output should include usage, got:\n%s", out)
	}
}
