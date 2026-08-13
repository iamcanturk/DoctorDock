package score

import (
	"testing"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

func findings(n int, id string, sev model.Severity) []model.Finding {
	out := make([]model.Finding, n)
	for i := range out {
		out[i] = model.Finding{ID: id, Severity: sev}
	}
	return out
}

func TestPerfectEnvironmentScores100(t *testing.T) {
	if got := Default().Calculate(nil); got != 100 {
		t.Errorf("no findings scored %d, want 100", got)
	}
	if got := Default().Calculate([]model.Finding{}); got != 100 {
		t.Errorf("empty findings scored %d, want 100", got)
	}
}

func TestInfoFindingsDoNotAffectScore(t *testing.T) {
	if got := Default().Calculate(findings(20, "DD015", model.SeverityInfo)); got != 100 {
		t.Errorf("20 INFO findings scored %d, want 100", got)
	}
}

func TestScoreIsBoundedAndOrdered(t *testing.T) {
	tests := []struct {
		name     string
		findings []model.Finding
	}{
		{"one low", findings(1, "DD007", model.SeverityLow)},
		{"one medium", findings(1, "DD006", model.SeverityMedium)},
		{"one high", findings(1, "DD001", model.SeverityHigh)},
		{"one critical", findings(1, "DD005", model.SeverityCritical)},
	}

	prev := 101
	for _, tt := range tests {
		got := Default().Calculate(tt.findings)
		if got < 0 || got > 100 {
			t.Fatalf("%s scored %d, outside [0,100]", tt.name, got)
		}
		if got >= prev {
			t.Errorf("%s scored %d, which is not worse than the previous %d", tt.name, got, prev)
		}
		prev = got
	}
}

// TestRepeatsDecay is the behaviour that keeps a developer laptop from
// scoring the same as a genuinely compromised host.
func TestRepeatsDecay(t *testing.T) {
	one := Default().Penalty(findings(1, "DD001", model.SeverityHigh))
	ten := Default().Penalty(findings(10, "DD001", model.SeverityHigh))

	if ten <= one {
		t.Fatal("more findings should cost more")
	}
	if ten >= one*10 {
		t.Errorf("ten repeats cost %.1f, which is not less than ten times %.1f — decay is not applied", ten, one*10)
	}
}

// TestDecayIsPerRule guards against a global counter, which would let a pile
// of unused images discount an unrelated critical finding.
func TestDecayIsPerRule(t *testing.T) {
	noise := findings(10, "DD015", model.SeverityLow)
	critical := model.Finding{ID: "DD005", Severity: model.SeverityCritical}

	alone := Default().Penalty([]model.Finding{critical})
	after := Default().Penalty(append(noise, critical)) - Default().Penalty(noise)

	if alone != after {
		t.Errorf("a critical finding cost %.2f alone but %.2f after unrelated findings", alone, after)
	}
}

// TestScoreNeverFloorsOnRealisticInput is the reason the mapping is
// exponential rather than subtractive: a busy developer machine must still
// produce a number that can go up when things are fixed.
func TestScoreNeverFloorsOnRealisticInput(t *testing.T) {
	var busy []model.Finding
	busy = append(busy, findings(17, "DD001", model.SeverityHigh)...)
	busy = append(busy, findings(10, "DD012", model.SeverityMedium)...)
	busy = append(busy, findings(22, "DD007", model.SeverityLow)...)
	busy = append(busy, findings(49, "DD015", model.SeverityInfo)...)

	score := Default().Calculate(busy)
	if score <= 0 {
		t.Fatalf("a realistic messy environment scored %d; the score carries no information at 0", score)
	}

	// Adding a critical finding must visibly move it.
	worse := Default().Calculate(append(busy, model.Finding{ID: "DD005", Severity: model.SeverityCritical}))
	if worse >= score {
		t.Errorf("adding a CRITICAL finding moved the score from %d to %d", score, worse)
	}
}

func TestGrade(t *testing.T) {
	tests := []struct {
		score int
		want  string
	}{
		{100, "excellent"},
		{90, "excellent"},
		{89, "good"},
		{75, "good"},
		{74, "needs attention"},
		{50, "needs attention"},
		{49, "poor"},
		{25, "poor"},
		{24, "critical"},
		{0, "critical"},
	}
	for _, tt := range tests {
		if got := Grade(tt.score); got != tt.want {
			t.Errorf("Grade(%d) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestScorerInterfaceIsSatisfied(t *testing.T) {
	var _ Scorer = Default()
	var _ Scorer = Weighted{}
}

// TestNilWeightsFallBackToDefaults keeps a zero-value Weighted usable.
func TestNilWeightsFallBackToDefaults(t *testing.T) {
	zero := Weighted{}
	if zero.Calculate(findings(1, "DD005", model.SeverityCritical)) !=
		Default().Calculate(findings(1, "DD005", model.SeverityCritical)) {
		t.Error("a zero-value Weighted should behave like Default()")
	}
}
