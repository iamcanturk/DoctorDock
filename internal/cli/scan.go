package cli

import (
	"github.com/spf13/cobra"

	"github.com/iamcanturk/DoctorDock/internal/report"
	"github.com/iamcanturk/DoctorDock/internal/rules"
	"github.com/iamcanturk/DoctorDock/internal/scanner"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

type scanFlags struct {
	failOn  string
	showAll bool
	ignore  []string
	only    []string
}

func (s *scanFlags) register(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&s.failOn, "fail-on", "",
		"exit non-zero when a finding of this severity or worse exists: info, low, medium, high, critical")
	f.BoolVar(&s.showAll, "all", false, "show every finding instead of capping repeats of the same rule")
	f.StringSliceVar(&s.ignore, "ignore", nil, "rule IDs to skip, e.g. --ignore DD007,DD015")
	f.StringSliceVar(&s.only, "only", nil, "run only these rule IDs")
}

func newScanCommand(g *globals) *cobra.Command {
	flags := &scanFlags{}

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan the Docker environment and report findings",
		Long: "Runs every rule against the local Docker environment and prints a health score,\n" +
			"resource summary and the findings.\n\n" +
			"Exit status is 0 unless --fail-on is given. With a threshold set, the exit code is\n" +
			"1 for findings below HIGH, 2 for HIGH and 3 for CRITICAL, so a pipeline can gate a\n" +
			"deployment with `doctordock scan --fail-on high`.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runScanCommand(g, cmd, flags, nil)
		},
	}

	flags.register(cmd)
	return cmd
}

// runScanCommand is shared by `scan`, the bare root command, `security` and
// `report`. ruleSet nil means the full registry.
func runScanCommand(g *globals, cmd *cobra.Command, flags *scanFlags, ruleSet []rules.Rule) error {
	ignore, err := ignoreFlagValues(flags.ignore)
	if err != nil {
		return err
	}

	if len(flags.only) > 0 {
		selected, err := selectRules(flags.only, ruleSet)
		if err != nil {
			return err
		}
		ruleSet = selected
	}

	threshold, thresholdSet, err := g.resolveFailOn(flags.failOn)
	if err != nil {
		return err
	}

	// JSON consumers want the whole environment; the terminal view only needs
	// the summaries it prints, and attaching megabytes of resource data to a
	// human-facing render would be waste.
	format, err := report.ParseFormat(g.format)
	if err != nil {
		return err
	}

	r, err := g.runScan(cmd, scanner.Config{
		IgnoreRules:      ignore,
		Rules:            ruleSet,
		IncludeResources: format == report.FormatJSON,
		Tool:             toolInfo(cmd),
	})
	if err != nil {
		return err
	}

	if err := g.render(cmd, r, flags.showAll); err != nil {
		return err
	}

	if code := exitFor(r, threshold, thresholdSet); code != ExitOK {
		return &failThreshold{code: code}
	}
	return nil
}

func selectRules(ids []string, from []rules.Rule) ([]rules.Rule, error) {
	if from == nil {
		from = rules.All()
	}
	wanted, err := ignoreFlagValues(ids) // same normalization and validation
	if err != nil {
		return nil, err
	}

	want := make(map[string]bool, len(wanted))
	for _, id := range wanted {
		want[id] = true
	}

	out := make([]rules.Rule, 0, len(want))
	for _, r := range from {
		if want[r.ID()] {
			out = append(out, r)
		}
	}
	return out, nil
}

func toolInfo(cmd *cobra.Command) model.ToolInfo {
	return model.ToolInfo{
		Name:    "doctordock",
		Version: cmd.Root().Version,
		Commit:  buildCommit,
	}
}
