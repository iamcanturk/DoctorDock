package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// CleanupRenderer prints a cleanup plan.
type CleanupRenderer struct {
	opts Options
	c    palette
}

// NewCleanup returns a renderer for cleanup plans.
func NewCleanup(opts Options) *CleanupRenderer {
	return &CleanupRenderer{opts: opts, c: newPalette(opts.Color)}
}

// RenderPlan writes a plan in the requested format.
func RenderPlan(w io.Writer, plan *model.CleanupPlan, format Format, opts Options) error {
	if format == FormatJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(plan)
	}
	return NewCleanup(opts).Render(w, plan)
}

// Render writes the human-facing view.
func (r *CleanupRenderer) Render(w io.Writer, plan *model.CleanupPlan) error {
	b := &strings.Builder{}
	b.WriteString("\n")

	if len(plan.Items) == 0 {
		fmt.Fprintf(b, "  %s\n\n", r.c.green("Nothing to clean up."))
		_, err := io.WriteString(w, b.String())
		return err
	}

	r.header(b, plan)
	r.groups(b, plan)
	r.totals(b, plan)

	_, err := io.WriteString(w, b.String())
	return err
}

func (r *CleanupRenderer) header(b *strings.Builder, plan *model.CleanupPlan) {
	if plan.Applied {
		fmt.Fprintf(b, "  %s\n", r.c.bold("CLEANUP — applied"))
	} else {
		// The word "would" appears in every line of a dry run. A user must
		// never be able to skim this and think something was deleted.
		fmt.Fprintf(b, "  %s  %s\n", r.c.bold("CLEANUP — dry run"),
			r.c.dim("nothing has been removed"))
	}
	fmt.Fprintf(b, "  %s\n\n", r.c.dimRaw(strings.Repeat("─", defaultWidth)))
}

// groups prints items grouped by resource kind, most impactful first.
func (r *CleanupRenderer) groups(b *strings.Builder, plan *model.CleanupPlan) {
	order := []struct {
		kind  model.ResourceKind
		label string
	}{
		{model.ResourceContainer, "CONTAINERS"},
		{model.ResourceImage, "IMAGES"},
		{model.ResourceNetwork, "NETWORKS"},
		{model.ResourceVolume, "VOLUMES"},
	}

	for _, group := range order {
		items := itemsOfKind(plan.Items, group.kind)
		if len(items) == 0 {
			continue
		}

		var bytes int64
		for _, item := range items {
			if item.Size > 0 {
				bytes += item.Size
			}
		}

		heading := fmt.Sprintf("%s  %d", group.label, len(items))
		if bytes > 0 {
			heading += "  " + model.FormatBytes(bytes)
		}
		fmt.Fprintf(b, "  %s\n", r.c.dim(heading))

		for _, item := range items {
			r.item(b, item, plan.Applied)
		}
		b.WriteString("\n")
	}
}

func (r *CleanupRenderer) item(b *strings.Builder, item model.CleanupItem, applied bool) {
	marker := r.riskMarker(item.Risk)

	size := ""
	if item.Size > 0 {
		size = r.c.dim("  " + model.FormatBytes(item.Size))
	}

	status := ""
	if applied {
		switch {
		case item.Removed:
			status = r.c.green("  removed")
		case item.Error != "":
			status = r.c.red("  failed")
		}
	}

	fmt.Fprintf(b, "    %s %s%s%s\n", marker, item.Name, size, status)
	fmt.Fprintf(b, "      %s\n", r.c.dimRaw(item.Reason))

	if item.Error != "" {
		for _, line := range wrap(item.Error, defaultWidth-6) {
			fmt.Fprintf(b, "      %s\n", r.c.red(line))
		}
	}
}

func (r *CleanupRenderer) riskMarker(risk model.Risk) string {
	switch risk {
	case model.RiskDataLoss:
		return r.c.boldRed("!")
	case model.RiskReview:
		return r.c.yellow("·")
	default:
		return r.c.green("·")
	}
}

func (r *CleanupRenderer) totals(b *strings.Builder, plan *model.CleanupPlan) {
	s := plan.Summary

	fmt.Fprintf(b, "  %s\n", r.c.dimRaw(strings.Repeat("─", defaultWidth)))

	if plan.Applied {
		fmt.Fprintf(b, "  %s  %s\n",
			r.c.bold(fmt.Sprintf("%d removed", s.Removed)),
			r.c.dim(fmt.Sprintf("· %s reclaimed", model.FormatBytes(s.ReclaimedBytes))))
		if s.Failed > 0 {
			fmt.Fprintf(b, "  %s\n", r.c.red(fmt.Sprintf("%d failed — see the messages above", s.Failed)))
		}
		b.WriteString("\n")
		return
	}

	// Networks and most volumes report no size, so a plan can legitimately
	// reclaim nothing measurable. Claiming "0 B would be reclaimed" reads as a
	// pointless operation rather than an unmeasured one.
	if s.ReclaimableBytes > 0 {
		fmt.Fprintf(b, "  %s  %s\n",
			r.c.bold(fmt.Sprintf("%d items", s.Total)),
			r.c.dim(fmt.Sprintf("· %s would be reclaimed", model.FormatBytes(s.ReclaimableBytes))))
	} else {
		fmt.Fprintf(b, "  %s  %s\n",
			r.c.bold(fmt.Sprintf("%d items", s.Total)),
			r.c.dim("· the daemon does not report a size for these"))
	}

	// The risk breakdown is the number that decides whether the user should
	// think twice, so it is stated before the command that would do it.
	var parts []string
	if n := s.ByRisk[model.RiskSafe]; n > 0 {
		parts = append(parts, r.c.green(fmt.Sprintf("%d safe", n)))
	}
	if n := s.ByRisk[model.RiskReview]; n > 0 {
		parts = append(parts, r.c.yellow(fmt.Sprintf("%d worth reviewing", n)))
	}
	if n := s.ByRisk[model.RiskDataLoss]; n > 0 {
		parts = append(parts, r.c.boldRed(fmt.Sprintf("%d could destroy data", n)))
	}
	if len(parts) > 0 {
		fmt.Fprintf(b, "  %s\n", strings.Join(parts, r.c.dim("  ·  ")))
	}

	b.WriteString("\n")
	fmt.Fprintf(b, "  %s\n\n", r.c.dim("Nothing was removed. Run again with --apply to do it."))
}

func itemsOfKind(items []model.CleanupItem, kind model.ResourceKind) []model.CleanupItem {
	out := make([]model.CleanupItem, 0, len(items))
	for _, item := range items {
		if item.Resource == kind {
			out = append(out, item)
		}
	}
	return out
}
