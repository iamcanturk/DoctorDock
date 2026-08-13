package cli

import (
	"github.com/spf13/cobra"

	"github.com/iamcanturk/DoctorDock/internal/report"
	"github.com/iamcanturk/DoctorDock/internal/scanner"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

func newContainersCommand(g *globals) *cobra.Command {
	return newResourceCommand(g, resourceSpec{
		use:   "containers",
		short: "List containers and the findings that concern them",
		long: "Prints every container with its state, health, published ports and issue count,\n" +
			"followed by the findings for those containers.",
		kind: model.ResourceContainer,
	})
}

func newImagesCommand(g *globals) *cobra.Command {
	return newResourceCommand(g, resourceSpec{
		use:   "images",
		short: "List images and the findings that concern them",
		long: "Prints every image with its size, age and referencing containers, followed by\n" +
			"the findings for those images.",
		kind: model.ResourceImage,
	})
}

func newVolumesCommand(g *globals) *cobra.Command {
	return newResourceCommand(g, resourceSpec{
		use:   "volumes",
		short: "List volumes and the findings that concern them",
		long: "Prints every volume with its driver, kind and mounting containers.\n\n" +
			"DoctorDock reports unused volumes but never removes one: an unused volume can\n" +
			"still hold the only copy of real data.",
		kind: model.ResourceVolume,
	})
}

func newNetworksCommand(g *globals) *cobra.Command {
	return newResourceCommand(g, resourceSpec{
		use:   "networks",
		short: "List networks and the findings that concern them",
		long: "Prints every network with its driver, scope, subnet and attached containers.\n" +
			"Docker's predefined networks are listed but never reported as unused.",
		kind: model.ResourceNetwork,
	})
}

type resourceSpec struct {
	use, short, long string
	kind             model.ResourceKind
}

func newResourceCommand(g *globals, spec resourceSpec) *cobra.Command {
	var showAll bool

	cmd := &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		Long:  spec.long,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := g.runScan(cmd, scanner.Config{
				IncludeResources: true,
				Tool:             toolInfo(cmd),
			})
			if err != nil {
				return err
			}

			format, err := report.ParseFormat(g.format)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			opts := report.Options{
				Color:   !g.noColor && report.ShouldColor(out),
				ShowAll: showAll,
			}

			if format == report.FormatJSON {
				// Narrow the document to the requested resource kind so that
				// `doctordock images --format json` is directly useful rather
				// than a full report the caller has to filter.
				return report.NewJSON().Render(out, narrowTo(r, spec.kind))
			}
			return report.NewResource(spec.kind, opts).Render(out, r)
		},
	}

	cmd.Flags().BoolVar(&showAll, "all", false, "show every finding instead of capping repeats of the same rule")
	return cmd
}

// narrowTo returns a copy of the report containing only one resource kind and
// the findings about it. The summary is left intact, because a client still
// wants to know how many containers exist when asking about images.
func narrowTo(r *model.Report, kind model.ResourceKind) *model.Report {
	out := *r

	out.Containers, out.Images, out.Volumes, out.Networks = nil, nil, nil, nil
	switch kind {
	case model.ResourceContainer:
		out.Containers = r.Containers
	case model.ResourceImage:
		out.Images = r.Images
	case model.ResourceVolume:
		out.Volumes = r.Volumes
	case model.ResourceNetwork:
		out.Networks = r.Networks
	}

	findings := make([]model.Finding, 0, len(r.Findings))
	for _, f := range r.Findings {
		if f.Resource == kind {
			findings = append(findings, f)
		}
	}
	out.Findings = findings

	return &out
}
