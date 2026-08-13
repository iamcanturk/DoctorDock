package report

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/iamcanturk/DoctorDock/internal/score"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// lowSeverityCap limits how many findings of one rule are printed at MEDIUM
// and below. Twelve unused images are one fact, not twelve; printing them all
// buries the CRITICAL finding above them. --all lifts the cap.
const lowSeverityCap = 5

const defaultWidth = 74

// TerminalRenderer produces the human-facing report.
type TerminalRenderer struct {
	opts  Options
	width int
	c     palette
}

// NewTerminal returns a renderer for terminal output.
func NewTerminal(opts Options) *TerminalRenderer {
	width := opts.Width
	if width <= 0 {
		width = defaultWidth
	}
	return &TerminalRenderer{opts: opts, width: width, c: newPalette(opts.Color)}
}

// Render implements Renderer.
func (t *TerminalRenderer) Render(w io.Writer, r *model.Report) error {
	b := &strings.Builder{}

	t.header(b, r)
	t.scoreLine(b, r)
	t.resources(b, r)
	t.findings(b, r)
	t.nextSteps(b, r)

	_, err := io.WriteString(w, b.String())
	return err
}

func (t *TerminalRenderer) header(b *strings.Builder, r *model.Report) {
	b.WriteString("\n")
	title := t.c.bold("DoctorDock")
	if r.Tool.Version != "" {
		title += t.c.dim("  " + r.Tool.Version)
	}
	fmt.Fprintf(b, "  %s\n", title)

	d := r.Docker
	parts := make([]string, 0, 3)
	if d.ServerVersion != "" {
		parts = append(parts, "Docker "+d.ServerVersion)
	}
	if d.OperatingSystem != "" {
		parts = append(parts, d.OperatingSystem)
	}
	if d.OSType != "" && d.Architecture != "" {
		parts = append(parts, d.OSType+"/"+d.Architecture)
	}
	if len(parts) > 0 {
		fmt.Fprintf(b, "  %s\n", t.c.dim(strings.Join(parts, "  ·  ")))
	}
	b.WriteString("\n")
}

func (t *TerminalRenderer) scoreLine(b *strings.Builder, r *model.Report) {
	colour := t.scoreColour(r.Score)

	label := t.c.dim("HEALTH SCORE")
	value := colour(fmt.Sprintf("%d/100", r.Score))
	grade := t.c.dim(score.Grade(r.Score))

	fmt.Fprintf(b, "  %s   %s   %s\n", label, value, grade)
	fmt.Fprintf(b, "  %s\n", colour(t.scoreBar(r.Score)))
	b.WriteString("\n")
}

// scoreBar draws a 40-cell meter. A number alone gives no sense of scale; the
// bar makes "42" read as "less than half" at a glance.
func (t *TerminalRenderer) scoreBar(s int) string {
	const cells = 40
	filled := s * cells / 100
	if s > 0 && filled == 0 {
		filled = 1
	}
	return strings.Repeat("█", filled) + t.c.dimRaw(strings.Repeat("░", cells-filled))
}

func (t *TerminalRenderer) scoreColour(s int) func(string) string {
	switch {
	case s >= 75:
		return t.c.green
	case s >= 50:
		return t.c.yellow
	default:
		return t.c.red
	}
}

// resources prints the four resource blocks in a two-column grid, which fits a
// standard terminal without scrolling and keeps related numbers adjacent.
func (t *TerminalRenderer) resources(b *strings.Builder, r *model.Report) {
	s := r.Summary

	containers := block{"CONTAINERS", []row{
		{"Total", fmt.Sprintf("%d", s.Containers.Total), ""},
		{"Running", fmt.Sprintf("%d", s.Containers.Running), ""},
		{"Stopped", fmt.Sprintf("%d", s.Containers.Stopped), ""},
		{"Unhealthy", fmt.Sprintf("%d", s.Containers.Unhealthy), ""},
	}}

	images := block{"IMAGES", []row{
		{"Total", fmt.Sprintf("%d", s.Images.Total), model.FormatBytes(s.Images.TotalSize)},
		{"Unused", fmt.Sprintf("%d", s.Images.Unused), model.FormatBytes(s.Images.ReclaimableSize)},
		{"Dangling", fmt.Sprintf("%d", s.Images.Dangling), ""},
		{"", "", ""},
	}}

	volumes := block{"VOLUMES", []row{
		{"Total", fmt.Sprintf("%d", s.Volumes.Total), ""},
		{"Unused", fmt.Sprintf("%d", s.Volumes.Unused), ""},
		{"Anonymous", fmt.Sprintf("%d", s.Volumes.Anonymous), ""},
		{"", "", ""},
	}}

	networks := block{"NETWORKS", []row{
		{"Total", fmt.Sprintf("%d", s.Networks.Total), ""},
		{"Custom", fmt.Sprintf("%d", s.Networks.Custom), ""},
		{"Unused", fmt.Sprintf("%d", s.Networks.Unused), ""},
		{"", "", ""},
	}}

	t.grid(b, containers, images)
	b.WriteString("\n")
	t.grid(b, volumes, networks)
	b.WriteString("\n")
}

type row struct{ label, value, note string }

type block struct {
	title string
	rows  []row
}

const columnWidth = 36

func (t *TerminalRenderer) grid(b *strings.Builder, left, right block) {
	fmt.Fprintf(b, "  %s%s\n",
		pad(t.c.dim(left.title), columnWidth, visibleLen(left.title)),
		t.c.dim(right.title))

	n := max(len(left.rows), len(right.rows))
	for i := 0; i < n; i++ {
		l := t.formatRow(rowAt(left.rows, i))
		r := t.formatRow(rowAt(right.rows, i))
		// Blocks are padded to equal length so the two columns line up; a row
		// that is empty on both sides is padding, not content.
		if l.text == "" && r.text == "" {
			continue
		}
		line := pad(l.text, columnWidth, l.width) + r.text
		fmt.Fprintf(b, "  %s\n", strings.TrimRight(line, " "))
	}
}

type rendered struct {
	text  string
	width int
}

func (t *TerminalRenderer) formatRow(r row) rendered {
	if r.label == "" {
		return rendered{}
	}
	plain := fmt.Sprintf("%-12s %5s", r.label, r.value)
	text := fmt.Sprintf("%-12s %s", r.label, t.c.bold(fmt.Sprintf("%5s", r.value)))
	if r.note != "" {
		plain += "  " + r.note
		text += "  " + t.c.dim(r.note)
	}
	return rendered{text: text, width: len(plain)}
}

func rowAt(rows []row, i int) row {
	if i < len(rows) {
		return rows[i]
	}
	return row{}
}

func (t *TerminalRenderer) findings(b *strings.Builder, r *model.Report) {
	if len(r.Findings) == 0 {
		fmt.Fprintf(b, "  %s\n\n", t.c.green("No findings. This environment looks healthy."))
		return
	}

	counts := r.Summary.Findings.BySeverity
	tally := make([]string, 0, 5)
	for i := len(model.AllSeverities) - 1; i >= 0; i-- {
		sev := model.AllSeverities[i]
		if n := counts.Get(sev); n > 0 {
			tally = append(tally, t.severityColour(sev)(fmt.Sprintf("%d %s", n, sev)))
		}
	}

	fmt.Fprintf(b, "  %s  %s\n", t.c.dim("FINDINGS"), strings.Join(tally, t.c.dim("  ·  ")))
	fmt.Fprintf(b, "  %s\n\n", t.c.dimRaw(strings.Repeat("─", t.width)))

	t.renderFindings(b, r.Findings)

	if !t.opts.ShowAll && hasRepeats(r.Findings) {
		fmt.Fprintf(b, "  %s\n\n", t.c.dim(
			"Repeated findings are grouped. Run with --all for one entry per resource."))
	}
}

// renderFindings prints findings grouped by rule.
//
// Grouping is the default because a real environment produces the same finding
// over and over — fifteen containers running as root is one problem, not
// fifteen, and printing it fifteen times pushes the CRITICAL entry off the
// screen. --all restores one entry per resource.
func (t *TerminalRenderer) renderFindings(b *strings.Builder, findings []model.Finding) {
	if t.opts.ShowAll {
		for _, f := range findings {
			t.finding(b, f)
		}
		return
	}
	for _, g := range groupFindings(findings) {
		t.group(b, g)
	}
}

// findingGroup is every finding produced by one rule at one severity.
type findingGroup struct {
	id        string
	rule      string
	severity  model.Severity
	resources []string
	example   model.Finding
}

// groupFindings collapses findings by rule ID, preserving the severity order
// the scanner already established.
//
// Severity is part of the key, not just the ID: DD004 reports a read-only /etc
// mount at HIGH and a writable host root at CRITICAL, and merging those into
// one entry would hide the one that matters.
func groupFindings(findings []model.Finding) []*findingGroup {
	var ordered []*findingGroup
	index := make(map[string]*findingGroup, len(findings))

	for _, f := range findings {
		key := f.ID + "|" + string(f.Severity)
		g, ok := index[key]
		if !ok {
			g = &findingGroup{
				id:       f.ID,
				rule:     f.Rule,
				severity: f.Severity,
				example:  f,
			}
			index[key] = g
			ordered = append(ordered, g)
		}
		g.resources = append(g.resources, f.ResourceName)
	}
	return ordered
}

func hasRepeats(findings []model.Finding) bool {
	for _, g := range groupFindings(findings) {
		if len(g.resources) > 1 {
			return true
		}
	}
	return false
}

// group prints one rule's findings as a single block.
func (t *TerminalRenderer) group(b *strings.Builder, g *findingGroup) {
	colour := t.severityColour(g.severity)

	headline := g.rule
	if headline == "" {
		headline = g.example.Title
	}

	fmt.Fprintf(b, "  %s  %s  %s  %s\n",
		colour(fmt.Sprintf("%-8s", g.severity)),
		t.c.dim(g.id),
		t.c.bold(headline),
		t.c.dim(affectedLabel(g.example.Resource, len(g.resources))))

	// A single finding already names its resource in the title, so repeating
	// the resource list would be noise.
	if len(g.resources) == 1 {
		for _, line := range wrap(g.example.Title, t.width-4) {
			fmt.Fprintf(b, "    %s\n", line)
		}
	} else {
		for _, line := range wrap(summarizeResources(g.resources, resourceListLimit), t.width-6) {
			fmt.Fprintf(b, "      %s\n", t.c.dimRaw(line))
		}
	}

	for _, line := range wrap(g.example.Recommendation, t.width-6) {
		fmt.Fprintf(b, "      %s\n", t.c.dimRaw(line))
	}
	b.WriteString("\n")
}

// resourceListLimit is how many resource names are printed before the rest are
// summarized as a count. Five fits on one line at the default width.
const resourceListLimit = 5

func summarizeResources(names []string, limit int) string {
	if len(names) <= limit {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:limit], ", ") +
		fmt.Sprintf(" and %d more", len(names)-limit)
}

func affectedLabel(kind model.ResourceKind, n int) string {
	noun := string(kind)
	if noun == "" {
		noun = "resource"
	}
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// selectFindings caps repeats of the same rule at MEDIUM and below, for the
// resource views which list findings without grouping them.
func (t *TerminalRenderer) selectFindings(findings []model.Finding) ([]model.Finding, int) {
	if t.opts.ShowAll {
		return findings, 0
	}

	seen := make(map[string]int, len(findings))
	shown := make([]model.Finding, 0, len(findings))
	hidden := 0

	for _, f := range findings {
		if f.Severity.Rank() >= model.SeverityHigh.Rank() {
			shown = append(shown, f)
			continue
		}
		seen[f.ID]++
		if seen[f.ID] > lowSeverityCap {
			hidden++
			continue
		}
		shown = append(shown, f)
	}
	return shown, hidden
}

func (t *TerminalRenderer) finding(b *strings.Builder, f model.Finding) {
	colour := t.severityColour(f.Severity)

	fmt.Fprintf(b, "  %s  %s  %s\n",
		colour(fmt.Sprintf("%-8s", f.Severity)),
		t.c.dim(f.ID),
		t.c.bold(f.ResourceName))

	for _, line := range wrap(f.Title, t.width-4) {
		fmt.Fprintf(b, "    %s\n", line)
	}
	for _, line := range wrap(f.Recommendation, t.width-6) {
		fmt.Fprintf(b, "      %s\n", t.c.dimRaw(line))
	}
	b.WriteString("\n")
}

func (t *TerminalRenderer) severityColour(s model.Severity) func(string) string {
	switch s {
	case model.SeverityCritical:
		return t.c.boldRed
	case model.SeverityHigh:
		return t.c.red
	case model.SeverityMedium:
		return t.c.yellow
	case model.SeverityLow:
		return t.c.cyan
	default:
		return t.c.dim
	}
}

// nextSteps collapses findings into one actionable line per rule, ordered by
// how much score each is costing. A list of 40 findings is a diagnosis; this
// is the treatment plan.
func (t *TerminalRenderer) nextSteps(b *strings.Builder, r *model.Report) {
	if len(r.Findings) == 0 {
		return
	}

	type group struct {
		id       string
		severity model.Severity
		count    int
		example  model.Finding
	}

	groups := make(map[string]*group)
	for _, f := range r.Findings {
		g, ok := groups[f.ID]
		if !ok {
			groups[f.ID] = &group{id: f.ID, severity: f.Severity, count: 1, example: f}
			continue
		}
		g.count++
		if f.Severity.Rank() > g.severity.Rank() {
			g.severity = f.Severity
		}
	}

	ordered := make([]*group, 0, len(groups))
	for _, g := range groups {
		ordered = append(ordered, g)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].severity.Rank() != ordered[j].severity.Rank() {
			return ordered[i].severity.Rank() > ordered[j].severity.Rank()
		}
		return ordered[i].id < ordered[j].id
	})

	fmt.Fprintf(b, "  %s\n", t.c.dim("WHAT TO FIX FIRST"))
	fmt.Fprintf(b, "  %s\n\n", t.c.dimRaw(strings.Repeat("─", t.width)))

	limit := min(len(ordered), 6)
	for _, g := range ordered[:limit] {
		subject := g.example.ResourceName
		if g.count > 1 {
			subject = affectedLabel(g.example.Resource, g.count)
		}
		fmt.Fprintf(b, "  %s %s  %s\n",
			t.severityColour(g.severity)("•"),
			t.c.bold(g.example.Rule),
			t.c.dim("— "+subject))
	}

	b.WriteString("\n")
	fmt.Fprintf(b, "  %s\n", t.c.dim("Run `doctordock scan --all` for every finding, "+
		"or `--format json` for machine-readable output."))
	b.WriteString("\n")
}

// wrap breaks text into lines no longer than width, on word boundaries.
func wrap(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	if width < 20 {
		width = 20
	}

	var (
		lines []string
		line  strings.Builder
	)
	for _, word := range words {
		if line.Len() > 0 && line.Len()+1+len(word) > width {
			lines = append(lines, line.String())
			line.Reset()
		}
		if line.Len() > 0 {
			line.WriteByte(' ')
		}
		line.WriteString(word)
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return lines
}

// pad right-pads a possibly-coloured string to a visible width. ANSI escapes
// have no width on screen but do have length in the string, so the caller
// supplies the visible length.
func pad(s string, width, visible int) string {
	if visible >= width {
		return s + " "
	}
	return s + strings.Repeat(" ", width-visible)
}

func visibleLen(s string) int { return len(s) }
