package cli

import (
	"github.com/spf13/cobra"

	"github.com/iamcanturk/DoctorDock/internal/rules"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

func newSecurityCommand(g *globals) *cobra.Command {
	flags := &scanFlags{}

	cmd := &cobra.Command{
		Use:   "security",
		Short: "Run only the security rules",
		Long: "Runs the SECURITY category rules and nothing else, so that cleanup and\n" +
			"configuration findings do not dilute a security review.\n\n" +
			"The score in this mode reflects security findings only.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runScanCommand(g, cmd, flags, rulesInCategory(model.CategorySecurity))
		},
	}

	flags.register(cmd)
	return cmd
}

func rulesInCategory(category model.Category) []rules.Rule {
	all := rules.All()
	out := make([]rules.Rule, 0, len(all))
	for _, r := range all {
		if r.Category() == category {
			out = append(out, r)
		}
	}
	return out
}
