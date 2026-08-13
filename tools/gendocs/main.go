// Command gendocs regenerates docs/RULES.md from the rule registry.
//
// The catalogue is generated rather than hand-written because a hand-written
// one drifts: someone adds a rule, forgets the table row, and the documented
// severity is wrong six months later. Run it with `make docs`.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/iamcanturk/DoctorDock/internal/rules"
	"github.com/iamcanturk/DoctorDock/internal/score"
	"github.com/iamcanturk/DoctorDock/pkg/model"
)

const outputPath = "docs/RULES.md"

// categoryOrder puts the categories in the order a reader cares about them.
var categoryOrder = []model.Category{
	model.CategorySecurity,
	model.CategoryConfiguration,
	model.CategoryResource,
	model.CategoryPerformance,
	model.CategoryCleanup,
}

var categoryBlurb = map[model.Category]string{
	model.CategorySecurity:      "Weaknesses an attacker could exploit.",
	model.CategoryConfiguration: "Setups that are fragile or simply wrong.",
	model.CategoryResource:      "Limits, quotas and consumption.",
	model.CategoryPerformance:   "Settings and states that degrade runtime behaviour.",
	model.CategoryCleanup:       "Reclaimable, unused resources.",
}

const header = `# Rule catalogue

Every check DoctorDock runs, with its default severity and what it looks for.

**This file is generated from the rule registry** by ` + "`make docs`" + `, so it cannot
drift from the code.

Rule IDs are stable and are **never reused**, even if a rule is removed — a
suppression written today keeps meaning the same thing after an upgrade.

## Severity

| Level | Meaning | Score weight |
|---|---|---|
| ` + "`CRITICAL`" + ` | Reaching this container is equivalent to owning the host | %.0f |
| ` + "`HIGH`" + ` | A real weakness that should be fixed | %.0f |
| ` + "`MEDIUM`" + ` | Weakens the setup, but needs another factor to be exploited | %.0f |
| ` + "`LOW`" + ` | A minor deviation from good practice | %.0f |
| ` + "`INFO`" + ` | Worth knowing, not a problem | %.0f |

Weights are the cost of the *first* finding from a given rule; repeats of the
same rule decay. See [SCORING.md](SCORING.md).

A rule's severity below is its **default**. Some rules escalate for specific
situations — DD004 reports a writable host-root mount as ` + "`CRITICAL`" + ` rather than
` + "`HIGH`" + `, and DD006 does the same for an exposed Docker API port.

## Suppressing a rule

` + "```bash" + `
doctordock scan --ignore DD007,DD015
` + "```" + `

or, for a whole team, in ` + "`doctordock.yaml`" + `:

` + "```yaml" + `
ignore:
  - DD007
` + "```" + `

Suppressed rules are listed in the JSON output under ` + "`skipped_rules`" + `, so a
reader can always tell "clean" apart from "not checked".

## Understanding a rule

This table is a summary. Any rule explains itself in full:

` + "```bash" + `
doctordock explain DD005
` + "```" + `

That covers what it looks for, why it matters, a worked scenario, fixes you can
copy, when it is fine to ignore, and links to the upstream documentation.
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "gendocs: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	all := rules.All()
	if err := rules.Validate(all); err != nil {
		return err
	}

	w := &strings.Builder{}
	fmt.Fprintf(w, header,
		score.DefaultWeights[model.SeverityCritical],
		score.DefaultWeights[model.SeverityHigh],
		score.DefaultWeights[model.SeverityMedium],
		score.DefaultWeights[model.SeverityLow],
		score.DefaultWeights[model.SeverityInfo],
	)

	written := 0
	for _, category := range categoryOrder {
		group := filter(all, category)
		if len(group) == 0 {
			continue
		}
		written += len(group)

		fmt.Fprintf(w, "\n## %s\n\n%s\n\n", title(category), categoryBlurb[category])
		w.WriteString("| ID | Severity | Rule | What it looks for |\n")
		w.WriteString("|---|---|---|---|\n")
		for _, r := range group {
			fmt.Fprintf(w, "| `%s` | `%s` | %s | %s |\n",
				r.ID(), r.Severity(), r.Name(), escapePipes(r.Description()))
		}
	}

	if written != len(all) {
		return fmt.Errorf("%d rules were not written: a category is missing from categoryOrder",
			len(all)-written)
	}

	fmt.Fprintf(w, "\n---\n\n%d rules in total. Adding one is documented in "+
		"[CONTRIBUTING.md](../CONTRIBUTING.md#adding-a-rule).\n", len(all))

	return os.WriteFile(outputPath, []byte(w.String()), 0o644)
}

func filter(all []rules.Rule, category model.Category) []rules.Rule {
	out := make([]rules.Rule, 0, len(all))
	for _, r := range all {
		if r.Category() == category {
			out = append(out, r)
		}
	}
	return out
}

func title(c model.Category) string {
	s := strings.ToLower(string(c))
	return strings.ToUpper(s[:1]) + s[1:]
}

// escapePipes keeps a description containing a pipe from breaking the table.
func escapePipes(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}
