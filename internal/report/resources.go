package report

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// ResourceRenderer prints one resource kind as a table, followed by the
// findings that concern it.
//
// It reuses model.Report rather than defining its own input, so the resource
// commands and the scan command are guaranteed to agree about what exists and
// what is wrong with it.
type ResourceRenderer struct {
	Kind model.ResourceKind
	opts Options
	c    palette
}

// NewResource returns a renderer for one resource kind.
func NewResource(kind model.ResourceKind, opts Options) *ResourceRenderer {
	return &ResourceRenderer{Kind: kind, opts: opts, c: newPalette(opts.Color)}
}

// Render implements Renderer.
func (r *ResourceRenderer) Render(w io.Writer, rep *model.Report) error {
	b := &strings.Builder{}
	b.WriteString("\n")

	switch r.Kind {
	case model.ResourceContainer:
		r.containers(b, rep)
	case model.ResourceImage:
		r.images(b, rep)
	case model.ResourceVolume:
		r.volumes(b, rep)
	case model.ResourceNetwork:
		r.networks(b, rep)
	default:
		return fmt.Errorf("no table for resource kind %q", r.Kind)
	}

	r.findings(b, rep)

	_, err := io.WriteString(w, b.String())
	return err
}

func (r *ResourceRenderer) containers(b *strings.Builder, rep *model.Report) {
	if len(rep.Containers) == 0 {
		r.empty(b, "No containers found.")
		return
	}

	issues := issueCounts(rep, model.ResourceContainer)
	t := newTable("NAME", "IMAGE", "STATE", "HEALTH", "PORTS", "ISSUES").
		limit(28, 34, 0, 0, 24, 0)

	for _, c := range rep.Containers {
		t.add(
			plain(c.Name),
			plain(c.Image),
			r.stateCell(c.State),
			r.healthCell(c),
			plain(portSummary(c.Ports)),
			r.issueCell(issues[c.ID]),
		)
	}
	t.render(b, r.c, "  ")

	s := rep.Summary.Containers
	fmt.Fprintf(b, "\n  %s\n\n", r.c.dim(fmt.Sprintf(
		"%d total · %d running · %d stopped · %d unhealthy",
		s.Total, s.Running, s.Stopped, s.Unhealthy)))
}

func (r *ResourceRenderer) images(b *strings.Builder, rep *model.Report) {
	if len(rep.Images) == 0 {
		r.empty(b, "No images found.")
		return
	}

	issues := issueCounts(rep, model.ResourceImage)
	t := newTable("REPOSITORY:TAG", "ID", "SIZE", "CREATED", "USED BY", "ISSUES").
		limit(40, 0, 0, 0, 26, 0)

	now := time.Now()
	for _, img := range rep.Images {
		usedBy := strings.Join(img.UsedBy, ", ")
		if usedBy == "" {
			usedBy = r.c.dim("—")
			t.add(
				plain(img.DisplayName()),
				plain(img.ShortID()),
				plain(model.FormatBytes(img.Size)),
				plain(model.FormatDuration(img.Age(now))),
				coloured(usedBy, "—"),
				r.issueCell(issues[img.ID]),
			)
			continue
		}
		t.add(
			plain(img.DisplayName()),
			plain(img.ShortID()),
			plain(model.FormatBytes(img.Size)),
			plain(model.FormatDuration(img.Age(now))),
			plain(usedBy),
			r.issueCell(issues[img.ID]),
		)
	}
	t.render(b, r.c, "  ")

	s := rep.Summary.Images
	fmt.Fprintf(b, "\n  %s\n\n", r.c.dim(fmt.Sprintf(
		"%d total (%s) · %d unused (%s reclaimable) · %d dangling",
		s.Total, model.FormatBytes(s.TotalSize),
		s.Unused, model.FormatBytes(s.ReclaimableSize), s.Dangling)))
}

func (r *ResourceRenderer) volumes(b *strings.Builder, rep *model.Report) {
	if len(rep.Volumes) == 0 {
		r.empty(b, "No volumes found.")
		return
	}

	issues := issueCounts(rep, model.ResourceVolume)
	t := newTable("NAME", "DRIVER", "KIND", "USED BY", "ISSUES").
		limit(44, 0, 0, 26, 0)

	for _, v := range rep.Volumes {
		kind := "named"
		if v.IsAnonymous() {
			kind = "anonymous"
		}
		usedBy := strings.Join(v.UsedBy, ", ")
		usedByCell := plain(usedBy)
		if usedBy == "" {
			usedByCell = coloured(r.c.dim("—"), "—")
		}
		t.add(plain(v.Name), plain(v.Driver), plain(kind), usedByCell, r.issueCell(issues[v.Name]))
	}
	t.render(b, r.c, "  ")

	s := rep.Summary.Volumes
	fmt.Fprintf(b, "\n  %s\n\n", r.c.dim(fmt.Sprintf(
		"%d total · %d unused · %d anonymous", s.Total, s.Unused, s.Anonymous)))
	fmt.Fprintf(b, "  %s\n\n", r.c.dim(
		"DoctorDock never deletes a volume. Check the contents before removing anything."))
}

func (r *ResourceRenderer) networks(b *strings.Builder, rep *model.Report) {
	if len(rep.Networks) == 0 {
		r.empty(b, "No networks found.")
		return
	}

	issues := issueCounts(rep, model.ResourceNetwork)
	t := newTable("NAME", "DRIVER", "SCOPE", "SUBNET", "CONTAINERS", "ISSUES").
		limit(32, 0, 0, 20, 30, 0)

	for _, n := range rep.Networks {
		attached := strings.Join(n.Containers, ", ")
		attachedCell := plain(attached)
		if attached == "" {
			attachedCell = coloured(r.c.dim("—"), "—")
		}
		t.add(
			plain(n.Name),
			plain(n.Driver),
			plain(n.Scope),
			plain(firstOrDash(n.Subnets)),
			attachedCell,
			r.issueCell(issues[n.ID]),
		)
	}
	t.render(b, r.c, "  ")

	s := rep.Summary.Networks
	fmt.Fprintf(b, "\n  %s\n\n", r.c.dim(fmt.Sprintf(
		"%d total · %d custom · %d unused", s.Total, s.Custom, s.Unused)))
}

// findings prints the findings for this resource kind beneath the table.
func (r *ResourceRenderer) findings(b *strings.Builder, rep *model.Report) {
	var relevant []model.Finding
	for _, f := range rep.Findings {
		if f.Resource == r.Kind {
			relevant = append(relevant, f)
		}
	}
	if len(relevant) == 0 {
		return
	}

	fmt.Fprintf(b, "  %s\n", r.c.dim(strings.ToUpper(countLabel(len(relevant), "finding"))))
	fmt.Fprintf(b, "  %s\n\n", r.c.dimRaw(strings.Repeat("─", defaultWidth)))

	term := &TerminalRenderer{opts: r.opts, width: defaultWidth, c: r.c}
	shown, hidden := term.selectFindings(relevant)
	for _, f := range shown {
		term.finding(b, f)
	}
	if hidden > 0 {
		fmt.Fprintf(b, "  %s\n\n", r.c.dim(fmt.Sprintf(
			"%d more hidden. Run with --all to see everything.", hidden)))
	}
}

func (r *ResourceRenderer) empty(b *strings.Builder, msg string) {
	fmt.Fprintf(b, "  %s\n\n", r.c.dim(msg))
}

func (r *ResourceRenderer) stateCell(state string) cell {
	switch state {
	case model.StateRunning:
		return coloured(r.c.green(state), state)
	case model.StateRestarting, model.StateDead, model.StateRemoving:
		return coloured(r.c.red(state), state)
	case model.StatePaused:
		return coloured(r.c.yellow(state), state)
	default:
		return coloured(r.c.dim(state), state)
	}
}

func (r *ResourceRenderer) healthCell(c model.Container) cell {
	// A stopped container keeps the health status it had when it stopped.
	// Colouring that red reads as a live failure, so past health is dimmed.
	if !c.IsRunning() {
		return coloured(r.c.dim(c.Health), c.Health)
	}
	switch c.Health {
	case model.HealthHealthy:
		return coloured(r.c.green("healthy"), "healthy")
	case model.HealthUnhealthy:
		return coloured(r.c.red("unhealthy"), "unhealthy")
	case model.HealthStarting:
		return coloured(r.c.yellow("starting"), "starting")
	default:
		return coloured(r.c.dim("none"), "none")
	}
}

// issueCell renders the finding count, coloured by whether there are any, so
// that a clean row is visually quiet.
func (r *ResourceRenderer) issueCell(counts severityTally) cell {
	if counts.total == 0 {
		return coloured(r.c.dim("—"), "—")
	}
	text := fmt.Sprintf("%d", counts.total)
	switch {
	case counts.worst.Rank() >= model.SeverityCritical.Rank():
		return coloured(r.c.boldRed(text), text)
	case counts.worst.Rank() >= model.SeverityHigh.Rank():
		return coloured(r.c.red(text), text)
	case counts.worst.Rank() >= model.SeverityMedium.Rank():
		return coloured(r.c.yellow(text), text)
	default:
		return coloured(r.c.cyan(text), text)
	}
}

type severityTally struct {
	total int
	worst model.Severity
}

func issueCounts(rep *model.Report, kind model.ResourceKind) map[string]severityTally {
	out := make(map[string]severityTally)
	for _, f := range rep.Findings {
		if f.Resource != kind {
			continue
		}
		t := out[f.ResourceID]
		t.total++
		if f.Severity.Rank() > t.worst.Rank() || t.total == 1 {
			t.worst = f.Severity
		}
		out[f.ResourceID] = t
	}
	return out
}

// portSummary condenses published ports to what fits in a table column.
func portSummary(ports []model.Port) string {
	if len(ports) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		if p.IsPublished() {
			parts = append(parts, fmt.Sprintf("%d→%d/%s", p.PublicPort, p.PrivatePort, p.Type))
			continue
		}
		parts = append(parts, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
	}
	return strings.Join(parts, " ")
}

func firstOrDash(values []string) string {
	if len(values) == 0 {
		return "—"
	}
	return values[0]
}
