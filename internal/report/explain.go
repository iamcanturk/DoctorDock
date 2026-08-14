package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/iamcanturk/DoctorDock/internal/rules"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// RuleDetail is everything `doctordock explain DD005` prints.
//
// With --format json this is the document the macOS app decodes, so the field
// names follow the same snake_case convention as the rest of the contract.
type RuleDetail struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Severity    model.Severity    `json:"severity"`
	Category    model.Category    `json:"category"`
	Description string            `json:"description"`
	Explanation rules.Explanation `json:"explanation"`
	HasLongForm bool              `json:"has_long_form"`
}

// RenderExplanation writes the long-form view of one rule.
func RenderExplanation(w io.Writer, d RuleDetail, opts Options) error {
	t := &TerminalRenderer{opts: opts, width: 76, c: newPalette(opts.Color)}
	b := &strings.Builder{}

	b.WriteString("\n")
	fmt.Fprintf(b, "  %s  %s  %s\n",
		t.severityColour(d.Severity)(fmt.Sprintf("%-8s", d.Severity)),
		t.c.dim(d.ID),
		t.c.bold(d.Name))
	fmt.Fprintf(b, "  %s\n", t.c.dim(strings.ToLower(string(d.Category))))
	fmt.Fprintf(b, "  %s\n", t.c.dimRaw(strings.Repeat("─", t.width)))

	if !d.HasLongForm {
		b.WriteString("\n")
		writeParagraph(b, t, d.Description)
		b.WriteString("\n")
		_, err := io.WriteString(w, b.String())
		return err
	}

	e := d.Explanation

	section(b, t, "WHAT IT LOOKS FOR", e.What)
	section(b, t, "WHY IT MATTERS", e.Why)
	if e.Scenario != "" {
		section(b, t, "WHAT GOES WRONG", e.Scenario)
	}

	if len(e.Fixes) > 0 {
		fmt.Fprintf(b, "\n  %s\n\n", t.c.dim("HOW TO FIX IT"))
		for i, fix := range e.Fixes {
			fmt.Fprintf(b, "  %s %s\n", t.c.bold(fmt.Sprintf("%d.", i+1)), t.c.bold(fix.Title))
			b.WriteString("\n")
			writeCode(b, t, fix.Code)
			if i < len(e.Fixes)-1 {
				b.WriteString("\n")
			}
		}
	}

	if e.FalsePositives != "" {
		section(b, t, "WHEN THIS IS FINE TO IGNORE", e.FalsePositives)
	}

	fmt.Fprintf(b, "\n  %s\n\n", t.c.dim("SUPPRESSING IT"))
	writeCode(b, t, fmt.Sprintf("doctordock scan --ignore %s\n\n# or, for the whole team, in doctordock.yaml:\nignore:\n  - %s", d.ID, d.ID))

	if len(e.References) > 0 {
		fmt.Fprintf(b, "\n  %s\n\n", t.c.dim("FURTHER READING"))
		for _, ref := range e.References {
			fmt.Fprintf(b, "    %s\n      %s\n", ref.Title, t.c.dim(ref.URL))
		}
	}

	b.WriteString("\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func section(b *strings.Builder, t *TerminalRenderer, heading, body string) {
	fmt.Fprintf(b, "\n  %s\n\n", t.c.dim(heading))
	writeParagraph(b, t, body)
}

// writeParagraph wraps prose, preserving deliberate blank lines and indented
// blocks — some explanations embed a small config snippet mid-sentence.
func writeParagraph(b *strings.Builder, t *TerminalRenderer, text string) {
	for _, block := range strings.Split(text, "\n\n") {
		if strings.HasPrefix(block, "    ") {
			writeCode(b, t, strings.TrimLeft(block, " "))
			continue
		}
		for _, line := range wrap(block, t.width-4) {
			fmt.Fprintf(b, "    %s\n", line)
		}
		b.WriteString("\n")
	}
}

// writeCode prints a snippet indented and dimmed, so it is visually distinct
// from prose without needing syntax highlighting.
func writeCode(b *strings.Builder, t *TerminalRenderer, code string) {
	for _, line := range strings.Split(strings.TrimRight(code, "\n"), "\n") {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		fmt.Fprintf(b, "      %s\n", t.c.cyan(line))
	}
}
