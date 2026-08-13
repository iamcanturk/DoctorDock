package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/iamcanturk/DoctorDock/internal/cleanup"
	"github.com/iamcanturk/DoctorDock/internal/docker"
	"github.com/iamcanturk/DoctorDock/internal/report"
	"github.com/iamcanturk/DoctorDock/internal/scanner"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

type cleanupFlags struct {
	apply bool
	yes   bool

	containers bool
	images     bool
	networks   bool
	volumes    bool
	all        bool

	keepSince time.Duration
}

// targets turns the flags into a target set.
//
// --all covers containers, images and networks but never volumes: everything
// else can be recreated, and a volume's contents cannot. Volumes require
// typing --volumes, every time. See docs/adr/0006-cleanup-safety-model.md.
func (f *cleanupFlags) targets() cleanup.Targets {
	t := cleanup.Targets{}
	if f.all {
		t = cleanup.All()
	}

	t.Containers = t.Containers || f.containers
	t.Images = t.Images || f.images
	t.Networks = t.Networks || f.networks
	t.Volumes = f.volumes
	if t.Images {
		t.DanglingImages = true
	}

	// With no target flags at all, consider only what Docker itself calls safe.
	if !t.Any() {
		t = cleanup.DefaultTargets()
	}
	return t
}

func newCleanupCommand(g *globals) *cobra.Command {
	flags := &cleanupFlags{}

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove unused Docker resources",
		Long: "Finds resources that can be reclaimed and, with --apply, removes them.\n\n" +
			"Running `doctordock cleanup` on its own NEVER deletes anything. It prints what it\n" +
			"would remove and how much space that frees. Deleting requires --apply.\n\n" +
			"With no target flags it considers only what Docker itself calls safe to prune:\n" +
			"dangling images and unused networks. --all adds stopped containers and unused\n" +
			"images. --all does NOT include volumes: every other resource can be recreated,\n" +
			"and a volume's contents cannot, so removing one requires --volumes explicitly.",
		Example: "  doctordock cleanup                      # what would be removed, removes nothing\n" +
			"  doctordock cleanup --apply              # remove dangling images and unused networks\n" +
			"  doctordock cleanup --all --apply        # also stopped containers and unused images\n" +
			"  doctordock cleanup --all --keep-since 24h --apply\n" +
			"  doctordock cleanup --volumes            # review unused volumes (still a dry run)",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCleanup(g, cmd, flags)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&flags.apply, "apply", false, "actually remove the resources (default is a dry run)")
	f.BoolVar(&flags.yes, "yes", false, "skip the confirmation prompt (for scripts)")
	f.BoolVar(&flags.containers, "containers", false, "include stopped containers")
	f.BoolVar(&flags.images, "images", false, "include unused images, not just dangling ones")
	f.BoolVar(&flags.networks, "networks", false, "include unused networks")
	f.BoolVar(&flags.volumes, "volumes", false, "include unused volumes — this can destroy data")
	f.BoolVar(&flags.all, "all", false, "containers, images and networks (never volumes)")
	f.DurationVar(&flags.keepSince, "keep-since", 0,
		"protect resources created within this window, e.g. 24h")

	return cmd
}

func runCleanup(g *globals, cmd *cobra.Command, flags *cleanupFlags) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), g.timeout)
	defer cancel()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := docker.Connect(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	// The scanner only ever receives a Client, which has no mutating methods.
	env, err := scanner.New(client, scanner.Config{}).Collect(ctx)
	if err != nil {
		return err
	}

	items := cleanup.Plan(env, cleanup.Options{
		Targets:   flags.targets(),
		KeepSince: flags.keepSince,
	})

	format, err := report.ParseFormat(g.format)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	opts := report.Options{Color: !g.noColor && report.ShouldColor(out)}

	if !flags.apply || len(items) == 0 {
		plan := cleanup.NewPlan(toolInfo(cmd), items, false, time.Time{})
		return report.RenderPlan(out, plan, format, opts)
	}

	// Show the plan before asking. A confirmation prompt for an unseen list is
	// not a confirmation.
	preview := cleanup.NewPlan(toolInfo(cmd), items, false, time.Time{})
	if format != report.FormatJSON {
		if err := report.RenderPlan(out, preview, format, opts); err != nil {
			return err
		}
	}

	if !flags.yes {
		confirmed, err := confirm(cmd, items, opts)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintf(cmd.ErrOrStderr(), "\nCancelled. Nothing was removed.\n\n")
			return nil
		}
	}

	pruner, ok := docker.AsPruner(client)
	if !ok {
		return fmt.Errorf("this Docker client cannot remove resources")
	}

	applied := cleanup.Apply(ctx, pruner, items)
	plan := cleanup.NewPlan(toolInfo(cmd), applied, true, time.Time{})

	if err := report.RenderPlan(out, plan, format, opts); err != nil {
		return err
	}

	// A partial failure is a real outcome, not a crash: some resources were
	// removed. Exit 1 so a script notices, without the severity codes a scan
	// uses.
	if plan.Summary.Failed > 0 {
		return &failThreshold{code: ExitWarning}
	}
	return nil
}

// confirm asks before removing anything.
//
// A plan that can destroy data requires typing the word "delete". Muscle memory
// for pressing y is exactly the failure this guards against.
func confirm(cmd *cobra.Command, items []model.CleanupItem, opts report.Options) (bool, error) {
	in, ok := cmd.InOrStdin().(*os.File)
	if !ok || !isTerminal(in) {
		return false, fmt.Errorf(
			"cannot ask for confirmation: stdin is not a terminal. Pass --yes to proceed non-interactively")
	}

	p := report.NewStyler(opts.Color)
	summary := model.SummarizeCleanup(items)
	errOut := cmd.ErrOrStderr()

	dataLoss := summary.ByRisk[model.RiskDataLoss]
	if dataLoss > 0 {
		fmt.Fprintf(errOut, "  %s\n",
			p.Red(fmt.Sprintf("%d volume(s) will be removed. Their contents cannot be recovered.", dataLoss)))
		fmt.Fprintf(errOut, "  %s ", p.Bold(`Type "delete" to confirm:`))
		return readAnswer(cmd.InOrStdin(), "delete")
	}

	fmt.Fprintf(errOut, "  %s ",
		p.Bold(fmt.Sprintf("Remove %d resource(s), reclaiming %s? [y/N]",
			summary.Total, model.FormatBytes(summary.ReclaimableBytes))))
	return readAnswer(cmd.InOrStdin(), "y", "yes")
}

func readAnswer(r io.Reader, accepted ...string) (bool, error) {
	reader := bufio.NewReader(r)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		// EOF with no input is a decline, not an error: it is what a closed
		// pipe or a Ctrl-D looks like.
		return false, nil
	}

	answer := strings.ToLower(strings.TrimSpace(line))
	for _, ok := range accepted {
		if answer == ok {
			return true, nil
		}
	}
	return false, nil
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
