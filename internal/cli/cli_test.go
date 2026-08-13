package cli

import (
	"testing"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

func reportWith(severities ...model.Severity) *model.Report {
	r := &model.Report{}
	for _, s := range severities {
		r.Findings = append(r.Findings, model.Finding{Severity: s})
	}
	return r
}

// TestExitCodesAreOptIn is the behaviour CI depends on and the behaviour a
// developer at a shell prompt depends on — they pull in opposite directions.
func TestExitCodesAreOptIn(t *testing.T) {
	r := reportWith(model.SeverityCritical)

	if got := exitFor(r, "", false); got != ExitOK {
		t.Errorf("without --fail-on the exit code should be 0, got %d", got)
	}
}

func TestExitForThresholds(t *testing.T) {
	tests := []struct {
		name      string
		findings  []model.Severity
		threshold model.Severity
		want      int
	}{
		{"clean", nil, model.SeverityHigh, ExitOK},
		{"below threshold", []model.Severity{model.SeverityLow}, model.SeverityHigh, ExitOK},
		{"at threshold", []model.Severity{model.SeverityHigh}, model.SeverityHigh, ExitHigh},
		{"above threshold", []model.Severity{model.SeverityCritical}, model.SeverityHigh, ExitCritical},
		{"medium meets low threshold", []model.Severity{model.SeverityMedium}, model.SeverityLow, ExitWarning},
		{"info meets info threshold", []model.Severity{model.SeverityInfo}, model.SeverityInfo, ExitWarning},
		{"worst wins", []model.Severity{model.SeverityLow, model.SeverityCritical}, model.SeverityMedium, ExitCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exitFor(reportWith(tt.findings...), tt.threshold, true)
			if got != tt.want {
				t.Errorf("exitFor = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestResolveFailOn(t *testing.T) {
	g := &globals{}

	if _, set, err := g.resolveFailOn(""); err != nil || set {
		t.Errorf("no flag and no config should mean no threshold (set=%v, err=%v)", set, err)
	}

	sev, set, err := g.resolveFailOn("high")
	if err != nil || !set || sev != model.SeverityHigh {
		t.Errorf("--fail-on high = %q, %v, %v", sev, set, err)
	}

	if _, _, err := g.resolveFailOn("nonsense"); err == nil {
		t.Error("an invalid severity should be rejected")
	}

	// "none" is the documented way to opt out even when the config sets one.
	g.cfg.FailOn = "critical"
	if _, set, _ := g.resolveFailOn("none"); set {
		t.Error("--fail-on none should override a configured threshold")
	}
	// The flag wins over the config.
	if sev, _, _ := g.resolveFailOn("low"); sev != model.SeverityLow {
		t.Errorf("the flag should beat the config, got %q", sev)
	}
	// With no flag, the config applies.
	if sev, set, _ := g.resolveFailOn(""); !set || sev != model.SeverityCritical {
		t.Errorf("the config threshold should apply when no flag is given, got %q", sev)
	}
}

func TestIgnoreFlagValues(t *testing.T) {
	got, err := ignoreFlagValues([]string{"dd001", " DD005 ", "DD007,DD015"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"DD001", "DD005", "DD007", "DD015"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	if _, err := ignoreFlagValues([]string{"DD999"}); err == nil {
		t.Error("an unknown rule ID should be rejected rather than silently ignored")
	}
}

func TestNarrowTo(t *testing.T) {
	full := &model.Report{
		Containers: []model.Container{{ID: "c1", Name: "api"}},
		Images:     []model.Image{{ID: "i1"}},
		Volumes:    []model.Volume{{Name: "v1"}},
		Networks:   []model.Network{{ID: "n1"}},
		Summary:    model.Summary{Containers: model.ContainerSummary{Total: 1}},
		Findings: []model.Finding{
			{ID: "DD001", Resource: model.ResourceContainer},
			{ID: "DD015", Resource: model.ResourceImage},
		},
	}

	narrowed := narrowTo(full, model.ResourceImage)

	if len(narrowed.Images) != 1 || narrowed.Containers != nil ||
		narrowed.Volumes != nil || narrowed.Networks != nil {
		t.Error("only the requested resource list should be present")
	}
	if len(narrowed.Findings) != 1 || narrowed.Findings[0].ID != "DD015" {
		t.Errorf("findings = %v, want only image findings", narrowed.Findings)
	}
	// The summary stays whole: a client asking about images still wants to
	// know how many containers exist.
	if narrowed.Summary.Containers.Total != 1 {
		t.Error("the summary should not be narrowed")
	}
	// The original must not be mutated.
	if len(full.Findings) != 2 || full.Containers == nil {
		t.Error("narrowTo mutated its input")
	}
}

func TestCommandTreeIsWired(t *testing.T) {
	root := newRootCommand(&globals{}, "0.1.0", "abc123")

	want := []string{
		"scan", "security", "containers", "images", "volumes", "networks",
		"report", "rules", "version",
	}
	have := make(map[string]bool)
	for _, c := range root.Commands() {
		have[c.Name()] = true
	}
	for _, name := range want {
		if !have[name] {
			t.Errorf("command %q is not registered", name)
		}
	}

	// Bare `doctordock` must accept scan's flags, not just scan itself.
	for _, flag := range []string{"fail-on", "all", "ignore", "only"} {
		if root.Flags().Lookup(flag) == nil {
			t.Errorf("root command is missing the %q flag", flag)
		}
	}
	for _, flag := range []string{"format", "no-color", "config", "timeout"} {
		if root.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("root command is missing the persistent %q flag", flag)
		}
	}
}

func TestSelectRules(t *testing.T) {
	got, err := selectRules([]string{"DD001", "DD005"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("selected %d rules, want 2", len(got))
	}
	for _, r := range got {
		if r.ID() != "DD001" && r.ID() != "DD005" {
			t.Errorf("unexpected rule %s", r.ID())
		}
	}

	if _, err := selectRules([]string{"DD999"}, nil); err == nil {
		t.Error("--only with an unknown rule should be rejected")
	}
}

func TestRulesInCategory(t *testing.T) {
	security := rulesInCategory(model.CategorySecurity)
	if len(security) == 0 {
		t.Fatal("there should be security rules")
	}
	for _, r := range security {
		if r.Category() != model.CategorySecurity {
			t.Errorf("%s is %s, not SECURITY", r.ID(), r.Category())
		}
	}
}
