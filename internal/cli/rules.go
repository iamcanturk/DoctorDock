package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iamcanturk/DoctorDock/internal/report"
	"github.com/iamcanturk/DoctorDock/internal/rules"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// ruleDoc is the JSON shape of a rule listing. It is separate from the rule
// interface so that the documented catalogue is stable even if the interface
// gains methods.
type ruleDoc struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Severity    model.Severity `json:"severity"`
	Category    model.Category `json:"category"`
	Description string         `json:"description"`
}

func newRulesCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "rules",
		Short: "List the rules DoctorDock checks",
		Long: "Lists every rule with its ID, default severity and what it looks for.\n\n" +
			"Rule IDs are stable and are never reused, so a suppression written today keeps\n" +
			"meaning the same thing after an upgrade.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			all := rules.All()
			docs := make([]ruleDoc, 0, len(all))
			for _, r := range all {
				docs = append(docs, ruleDoc{
					ID:          r.ID(),
					Name:        r.Name(),
					Severity:    r.Severity(),
					Category:    r.Category(),
					Description: r.Description(),
				})
			}

			format, err := report.ParseFormat(g.format)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if format == report.FormatJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				enc.SetEscapeHTML(false)
				return enc.Encode(docs)
			}

			p := newPaletteFor(g, cmd)
			b := &strings.Builder{}
			b.WriteString("\n")
			for _, d := range docs {
				fmt.Fprintf(b, "  %s  %s  %s\n",
					p.Severity(d.Severity, fmt.Sprintf("%-8s", d.Severity)),
					p.Dim(d.ID),
					p.Bold(d.Name))
				for _, line := range report.Wrap(d.Description, 70) {
					fmt.Fprintf(b, "      %s\n", p.Dim(line))
				}
				b.WriteString("\n")
			}
			fmt.Fprintf(b, "  %s\n\n", p.Dim(fmt.Sprintf(
				"%d rules. Suppress one with --ignore DD007, or in %s.",
				len(docs), "doctordock.yaml")))

			_, err = out.Write([]byte(b.String()))
			return err
		},
	}
}
