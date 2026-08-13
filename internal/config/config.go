// Package config loads DoctorDock's optional configuration file.
//
// DoctorDock is designed to need no configuration: every value here has a
// working default and a missing file is not an error. The file exists for the
// two things a team genuinely needs to share — which rules to suppress and
// where the thresholds sit — so that those live in version control rather than
// in everyone's shell history.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/iamcanturk/DoctorDock/internal/rules"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// FileName is the config file name looked for in each search location.
const FileName = "doctordock.yaml"

// Config is the file's contents.
type Config struct {
	// Ignore lists rule IDs to skip entirely.
	Ignore []string `yaml:"ignore"`

	// FailOn is the default --fail-on threshold. Empty means "never fail".
	FailOn string `yaml:"fail_on"`

	Thresholds Thresholds `yaml:"thresholds"`

	// Path records where this config was loaded from. Empty means defaults.
	Path string `yaml:"-"`
}

// Thresholds are the tunable rule limits.
type Thresholds struct {
	// LargeImageBytes is the size at which DD016 reports an image as oversized.
	LargeImageBytes int64 `yaml:"large_image_bytes"`
	// RestartLoop is the restart count at which DD013 fires.
	RestartLoop int `yaml:"restart_loop"`
}

// Default returns the configuration used when no file is present.
func Default() Config {
	return Config{}
}

// Load reads the configuration.
//
// When path is non-empty it must exist — an explicit --config that points at a
// missing file is a mistake worth reporting, unlike the absence of an optional
// file nobody asked for. Otherwise the search locations are tried in order and
// the first hit wins.
func Load(path string) (Config, error) {
	if path != "" {
		return loadFile(path)
	}

	for _, candidate := range SearchPaths() {
		cfg, err := loadFile(candidate)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return Default(), err
		}
		return cfg, nil
	}

	return Default(), nil
}

// SearchPaths returns the locations checked for a config file, nearest first.
// A project-local file beats a user-level one, so a repository can pin its own
// suppressions for everyone who works on it.
func SearchPaths() []string {
	paths := []string{
		FileName,
		filepath.Join(".doctordock", FileName),
	}
	if dir, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(dir, "doctordock", FileName))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, "."+FileName))
	}
	return paths
}

func loadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Default(), err
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.Path = path

	if err := cfg.Validate(); err != nil {
		return Default(), fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Validate rejects values that would silently do nothing.
//
// A typo in a rule ID is the failure mode that matters: a suppression for
// "DD05" looks like it works, produces no error, and quietly leaves the rule
// enabled forever.
func (c Config) Validate() error {
	for _, id := range c.Ignore {
		if _, ok := rules.ByID(strings.ToUpper(id)); !ok {
			return fmt.Errorf("ignore: unknown rule %q (see `doctordock rules`)", id)
		}
	}
	if c.FailOn != "" && !strings.EqualFold(c.FailOn, "none") {
		if _, err := model.ParseSeverity(c.FailOn); err != nil {
			return fmt.Errorf("fail_on: %w", err)
		}
	}
	if c.Thresholds.LargeImageBytes < 0 {
		return errors.New("thresholds.large_image_bytes must not be negative")
	}
	if c.Thresholds.RestartLoop < 0 {
		return errors.New("thresholds.restart_loop must not be negative")
	}
	return nil
}

// IgnoredRules returns the suppressed rule IDs, upper-cased.
func (c Config) IgnoredRules() []string {
	out := make([]string, 0, len(c.Ignore))
	for _, id := range c.Ignore {
		out = append(out, strings.ToUpper(strings.TrimSpace(id)))
	}
	return out
}

// RuleOptions converts the configured thresholds into rule options, leaving
// unset values at their defaults.
func (c Config) RuleOptions() rules.Options {
	return rules.Options{
		LargeImageBytes:      c.Thresholds.LargeImageBytes,
		RestartLoopThreshold: c.Thresholds.RestartLoop,
	}.Normalize()
}
