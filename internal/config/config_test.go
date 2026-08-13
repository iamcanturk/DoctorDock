package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMissingConfigIsNotAnError(t *testing.T) {
	// Search from an empty directory so no real config is picked up.
	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("a missing config file must not be an error: %v", err)
	}
	if cfg.Path != "" {
		t.Errorf("loaded %q when nothing should have been found", cfg.Path)
	}
}

// An explicitly requested file that does not exist is a mistake worth
// reporting, unlike the absence of an optional file.
func TestExplicitMissingConfigIsAnError(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("an explicit --config pointing at a missing file should fail")
	}
}

func TestLoadValidConfig(t *testing.T) {
	path := write(t, `
ignore:
  - DD007
  - dd015
fail_on: high
thresholds:
  large_image_bytes: 2000000000
  restart_loop: 3
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got := cfg.IgnoredRules(); len(got) != 2 || got[0] != "DD007" || got[1] != "DD015" {
		t.Errorf("IgnoredRules() = %v, want normalized upper-case IDs", got)
	}
	if cfg.FailOn != "high" {
		t.Errorf("FailOn = %q", cfg.FailOn)
	}

	opts := cfg.RuleOptions()
	if opts.LargeImageBytes != 2_000_000_000 || opts.RestartLoopThreshold != 3 {
		t.Errorf("RuleOptions() = %+v", opts)
	}
}

// A typo in a rule ID is the failure mode that matters: it looks like it
// works and silently leaves the rule enabled forever.
func TestUnknownRuleIsRejected(t *testing.T) {
	path := write(t, "ignore:\n  - DD05\n")

	_, err := Load(path)
	if err == nil {
		t.Fatal("an unknown rule ID should be rejected, not ignored")
	}
	if !strings.Contains(err.Error(), "DD05") {
		t.Errorf("error should name the offending ID: %v", err)
	}
}

func TestInvalidFailOnIsRejected(t *testing.T) {
	if _, err := Load(write(t, "fail_on: catastrophic\n")); err == nil {
		t.Fatal("an unknown severity should be rejected")
	}
	// "none" is the documented way to disable failing.
	if _, err := Load(write(t, "fail_on: none\n")); err != nil {
		t.Fatalf("fail_on: none should be accepted: %v", err)
	}
}

func TestMalformedYAMLIsRejected(t *testing.T) {
	if _, err := Load(write(t, "ignore: [unclosed\n")); err == nil {
		t.Fatal("malformed YAML should be rejected")
	}
}

func TestNegativeThresholdsAreRejected(t *testing.T) {
	if _, err := Load(write(t, "thresholds:\n  restart_loop: -1\n")); err == nil {
		t.Fatal("a negative threshold should be rejected")
	}
}

func TestDefaultsFillUnsetThresholds(t *testing.T) {
	cfg, err := Load(write(t, "ignore: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	opts := cfg.RuleOptions()
	if opts.LargeImageBytes <= 0 || opts.RestartLoopThreshold <= 0 {
		t.Errorf("unset thresholds should take their defaults, got %+v", opts)
	}
}

// A project-local file must win over a user-level one, so a repository can
// pin its own suppressions for everyone who works on it.
func TestSearchPathsPreferProjectLocal(t *testing.T) {
	paths := SearchPaths()
	if len(paths) < 2 || paths[0] != FileName {
		t.Fatalf("SearchPaths() = %v, want the working directory first", paths)
	}
}

func TestLoadFindsProjectLocalFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("fail_on: medium\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FailOn != "medium" {
		t.Errorf("FailOn = %q, want the project-local file to be used", cfg.FailOn)
	}
}
