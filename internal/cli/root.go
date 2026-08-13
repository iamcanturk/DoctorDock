// Package cli wires the command tree, flags and exit codes.
//
// It is the only package that knows about Cobra, and it holds no analysis
// logic: every command collects options, calls the scanner, and hands the
// report to a renderer.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/iamcanturk/DoctorDock/internal/config"
	"github.com/iamcanturk/DoctorDock/internal/docker"
	"github.com/iamcanturk/DoctorDock/internal/report"
	"github.com/iamcanturk/DoctorDock/internal/rules"
	"github.com/iamcanturk/DoctorDock/internal/scanner"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// Exit codes. These are contract: CI pipelines branch on them.
const (
	// ExitOK means no findings met the --fail-on threshold.
	ExitOK = 0
	// ExitWarning means the worst finding was below HIGH.
	ExitWarning = 1
	// ExitHigh means at least one HIGH finding.
	ExitHigh = 2
	// ExitCritical means at least one CRITICAL finding.
	ExitCritical = 3
	// ExitError means DoctorDock itself failed — bad flags, no daemon,
	// unreadable config. Distinct from findings so that a broken pipeline is
	// never mistaken for an insecure environment.
	ExitError = 10
)

// globals holds the flags shared by every command.
type globals struct {
	format     string
	noColor    bool
	configPath string
	timeout    time.Duration

	cfg config.Config
}

// Execute runs the CLI and returns the process exit code.
func Execute(version, commit string) int {
	g := &globals{}
	root := newRootCommand(g, version, commit)

	if err := root.Execute(); err != nil {
		var fail *failThreshold
		if errors.As(err, &fail) {
			return fail.code
		}
		reportError(root.ErrOrStderr(), err)
		return ExitError
	}
	return ExitOK
}

// failThreshold is returned instead of a real error when findings crossed the
// --fail-on threshold. Cobra needs an error to stop, but this is a result, not
// a malfunction, so it must not print a usage message or an error banner.
type failThreshold struct {
	code int
}

func (f *failThreshold) Error() string {
	return fmt.Sprintf("findings exceeded threshold (exit %d)", f.code)
}

func reportError(w io.Writer, err error) {
	fmt.Fprintf(w, "\n  Error: %v\n", err)

	var connErr *docker.ConnectionError
	if errors.As(err, &connErr) {
		fmt.Fprintf(w, "\n  %s\n", strings.ReplaceAll(connErr.Hint(), "\n", "\n  "))
	}
	fmt.Fprintln(w)
}

func newRootCommand(g *globals, version, commit string) *cobra.Command {
	scan := newScanCommand(g)

	root := &cobra.Command{
		Use:   "doctordock",
		Short: "Docker environment diagnostics",
		Long: "DoctorDock inspects the local Docker environment and reports security problems,\n" +
			"misconfigurations and reclaimable resources.\n\n" +
			"It runs entirely locally: no network calls, no telemetry, no AI. Running it with\n" +
			"no arguments is the same as `doctordock scan`.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Bare `doctordock` is the dashboard. Making it an alias for scan means
		// there is one code path, not two that drift apart.
		RunE: scan.RunE,
		Args: cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(g.configPath)
			if err != nil {
				return err
			}
			g.cfg = cfg
			return nil
		},
	}

	flags := root.PersistentFlags()
	flags.StringVarP(&g.format, "format", "f", "terminal", "output format: terminal, json")
	flags.BoolVar(&g.noColor, "no-color", false, "disable coloured output")
	flags.StringVar(&g.configPath, "config", "", "path to a config file (default: search "+config.FileName+")")
	flags.DurationVar(&g.timeout, "timeout", 60*time.Second, "maximum time to spend talking to Docker")

	// Bare `doctordock` shares scan's flags, so they have to be registered on
	// root as well as on the subcommand.
	scan.Flags().VisitAll(func(f *pflag.Flag) { root.Flags().AddFlag(f) })

	root.AddCommand(
		scan,
		newSecurityCommand(g),
		newContainersCommand(g),
		newImagesCommand(g),
		newVolumesCommand(g),
		newNetworksCommand(g),
		newReportCommand(g),
		newCleanupCommand(g),
		newRulesCommand(g),
		newExplainCommand(g),
		newVersionCommand(g, version, commit),
	)

	root.SetVersionTemplate("doctordock {{.Version}}\n")
	return root
}

// runScan is the shared path behind every command that produces a report.
func (g *globals) runScan(cmd *cobra.Command, cfg scanner.Config) (*model.Report, error) {
	ctx, cancel := context.WithTimeout(cmd.Context(), g.timeout)
	defer cancel()

	// Ctrl-C during collection should stop cleanly rather than leaving the
	// terminal mid-render.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := docker.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	cfg.IgnoreRules = append(cfg.IgnoreRules, g.cfg.IgnoredRules()...)
	cfg.Options = g.cfg.RuleOptions()

	return scanner.New(client, cfg).Scan(ctx)
}

// render writes a report in the requested format.
func (g *globals) render(cmd *cobra.Command, r *model.Report, showAll bool) error {
	format, err := report.ParseFormat(g.format)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	renderer, err := report.New(format, report.Options{
		Color:   !g.noColor && report.ShouldColor(out),
		ShowAll: showAll,
	})
	if err != nil {
		return err
	}
	return renderer.Render(out, r)
}

// resolveFailOn returns the effective threshold, preferring the flag over the
// config file, and reports whether one is set at all.
func (g *globals) resolveFailOn(flagValue string) (model.Severity, bool, error) {
	value := flagValue
	if value == "" {
		value = g.cfg.FailOn
	}
	if value == "" || strings.EqualFold(value, "none") {
		return "", false, nil
	}
	sev, err := model.ParseSeverity(value)
	if err != nil {
		return "", false, fmt.Errorf("--fail-on: %w", err)
	}
	return sev, true, nil
}

// exitFor maps a report to an exit code, given a threshold.
//
// With no threshold the exit code is always zero. A developer running
// `doctordock` on their laptop should not get a non-zero status just for
// having unused images — that breaks shell prompts and `&&` chains for no
// benefit. CI opts in with --fail-on.
func exitFor(r *model.Report, threshold model.Severity, enabled bool) int {
	if !enabled {
		return ExitOK
	}
	highest, ok := r.HighestSeverity()
	if !ok || highest.Rank() < threshold.Rank() {
		return ExitOK
	}
	switch highest {
	case model.SeverityCritical:
		return ExitCritical
	case model.SeverityHigh:
		return ExitHigh
	default:
		return ExitWarning
	}
}

// ignoreFlagValues normalizes --ignore values, which users write in any case
// and sometimes as a single comma-separated string.
func ignoreFlagValues(values []string) ([]string, error) {
	var out []string
	for _, v := range values {
		for _, id := range strings.Split(v, ",") {
			id = strings.ToUpper(strings.TrimSpace(id))
			if id == "" {
				continue
			}
			if _, ok := rules.ByID(id); !ok {
				return nil, fmt.Errorf("--ignore: unknown rule %q (see `doctordock rules`)", id)
			}
			out = append(out, id)
		}
	}
	return out, nil
}

// newPaletteFor builds a styler for a command's output stream, honouring
// --no-color and whether the stream is a terminal.
func newPaletteFor(g *globals, cmd *cobra.Command) report.Styler {
	out := cmd.OutOrStdout()
	return report.NewStyler(!g.noColor && report.ShouldColor(out))
}
