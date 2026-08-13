package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/iamcanturk/DoctorDock/internal/report"
	"github.com/iamcanturk/DoctorDock/internal/scanner"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

func newReportCommand(g *globals) *cobra.Command {
	var (
		output string
		failOn string
	)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Produce a complete report artifact",
		Long: "Produces a full report — every finding, every resource — for archiving, sharing\n" +
			"or feeding to another tool.\n\n" +
			"Unlike `scan`, which is tuned for reading in a terminal, `report` defaults to JSON\n" +
			"and always includes the complete resource lists. This is the document the macOS\n" +
			"app and CI integrations consume; its shape is documented in docs/JSON_SCHEMA.md.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// `report` is the artifact command, so JSON is the sensible
			// default here even though the rest of the CLI defaults to
			// terminal output. An explicit --format still wins.
			if !cmd.Flags().Changed("format") && !cmd.Root().PersistentFlags().Changed("format") {
				g.format = string(report.FormatJSON)
			}

			threshold, thresholdSet, err := g.resolveFailOn(failOn)
			if err != nil {
				return err
			}

			r, err := g.runScan(cmd, scanner.Config{
				IncludeResources: true,
				Tool:             toolInfo(cmd),
			})
			if err != nil {
				return err
			}

			if output != "" {
				if err := writeToFile(g, output, r); err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Report written to %s\n", output)
			} else if err := g.render(cmd, r, true); err != nil {
				return err
			}

			if code := exitFor(r, threshold, thresholdSet); code != ExitOK {
				return &failThreshold{code: code}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "write the report to a file instead of stdout")
	cmd.Flags().StringVar(&failOn, "fail-on", "",
		"exit non-zero when a finding of this severity or worse exists: info, low, medium, high, critical")
	return cmd
}

func writeToFile(g *globals, path string, r *model.Report) error {
	format, err := report.ParseFormat(g.format)
	if err != nil {
		return err
	}

	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	// A file is never a terminal, so colour is always off regardless of what
	// the surrounding shell looks like.
	renderer, err := report.New(format, report.Options{Color: false, ShowAll: true})
	if err != nil {
		return err
	}
	if err := renderer.Render(f, r); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return f.Close()
}
