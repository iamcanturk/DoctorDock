package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iamcanturk/DoctorDock/internal/report"
	"github.com/iamcanturk/DoctorDock/internal/rules"
)

func newExplainCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "explain <rule-id>",
		Short: "Explain a rule in full: what it means, why it matters, how to fix it",
		Long: "Prints the long-form explanation of one rule.\n\n" +
			"A scan gives you a one-line finding, which is enough to know something is wrong and\n" +
			"not enough to decide what to do about it. This gives you the rest: what the rule\n" +
			"actually looks for, the concrete consequence, worked fixes you can copy, and an\n" +
			"honest note on when the finding is fine to ignore.",
		Example: "  doctordock explain DD005\n" +
			"  doctordock explain dd001          # case does not matter\n" +
			"  doctordock explain DD016 --format json",
		Args: cobra.ExactArgs(1),
		// Completing rule IDs matters more here than anywhere else: nobody
		// remembers that DD009 is the capabilities one.
		ValidArgsFunction: func(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
			var out []string
			for _, r := range rules.All() {
				if strings.HasPrefix(r.ID(), strings.ToUpper(prefix)) {
					out = append(out, r.ID()+"\t"+r.Name())
				}
			}
			return out, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToUpper(strings.TrimSpace(args[0]))

			rule, ok := rules.ByID(id)
			if !ok {
				return fmt.Errorf("unknown rule %q — run `doctordock rules` for the full list", args[0])
			}

			explanation, hasLongForm := rules.Explain(id)
			detail := report.RuleDetail{
				ID:          rule.ID(),
				Name:        rule.Name(),
				Severity:    rule.Severity(),
				Category:    rule.Category(),
				Description: rule.Description(),
				Explanation: explanation,
				HasLongForm: hasLongForm,
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
				return enc.Encode(detail)
			}

			return report.RenderExplanation(out, detail, report.Options{
				Color: !g.noColor && report.ShouldColor(out),
			})
		},
	}
}
