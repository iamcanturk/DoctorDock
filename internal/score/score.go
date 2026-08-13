// Package score turns a set of findings into a single number.
//
// It is a separate package because the scoring model is the part of DoctorDock
// most likely to be replaced. Isolating it behind Scorer means a better model
// can land without touching the scanner, the rules or the renderers.
package score

import (
	"math"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// Scorer converts findings into a health score in [0, 100].
type Scorer interface {
	Calculate(findings []model.Finding) int
}

// Weights are the point penalties per severity.
type Weights map[model.Severity]float64

// DefaultWeights is the baseline penalty for the first finding of a given rule.
var DefaultWeights = Weights{
	model.SeverityCritical: 25,
	model.SeverityHigh:     15,
	model.SeverityMedium:   8,
	model.SeverityLow:      3,
	model.SeverityInfo:     0,
}

// Scale controls how quickly the score falls as penalty accumulates. At
// penalty == Scale the score is 100/e ≈ 37.
const Scale = 100.0

// Weighted is the default scorer.
//
// It accumulates a penalty per finding using the severity weights, then maps
// that penalty onto [0, 100] with `100 * exp(-penalty/Scale)`.
//
// Two things drove this shape, both discovered by running against real
// environments rather than fixtures:
//
// Repeats of the same rule decay harmonically. On a developer machine it is
// normal for fifteen containers to run as root; charging full price for each
// says the same thing fifteen times. The first occurrence of a rule costs full
// weight, the tenth costs a tenth. Breadth still matters, but one systemic
// pattern cannot drown out everything else.
//
// The total is then mapped exponentially rather than subtracted. Plain
// subtraction bottoms out: a working developer laptop easily accumulates 200
// penalty points, which pins it at 0 alongside a machine that is genuinely ten
// times worse. A score that reads 0 for every real environment carries no
// information and cannot show improvement. The exponential curve keeps the
// ordering, never saturates, and means fixing the Docker socket mount always
// moves the number — which is the only thing a health score is actually for.
//
// The numbers are not a risk model and should not be read as one. They are a
// direction of travel between two scans of the same machine.
type Weighted struct {
	Weights Weights
	// Scale overrides the decay constant. Zero means Scale.
	Scale float64
}

// Default returns the standard scorer.
func Default() Weighted {
	return Weighted{Weights: DefaultWeights, Scale: Scale}
}

// Penalty returns the accumulated penalty before it is mapped to a score. It
// is exported because it is the quantity that is actually additive, and a
// caller comparing two scans wants to compare penalties, not scores.
func (w Weighted) Penalty(findings []model.Finding) float64 {
	weights := w.Weights
	if weights == nil {
		weights = DefaultWeights
	}

	// Occurrences are counted per rule, not globally, so that ten unused
	// images decay against each other while an unrelated privileged container
	// still lands at full weight.
	occurrences := make(map[string]int, len(findings))

	penalty := 0.0
	for _, f := range findings {
		base, ok := weights[f.Severity]
		if !ok || base == 0 {
			continue
		}
		occurrences[f.ID]++
		penalty += base / float64(occurrences[f.ID])
	}
	return penalty
}

// Calculate implements Scorer.
func (w Weighted) Calculate(findings []model.Finding) int {
	if len(findings) == 0 {
		return 100
	}

	scale := w.Scale
	if scale <= 0 {
		scale = Scale
	}

	return clamp(int(math.Round(100 * math.Exp(-w.Penalty(findings)/scale))))
}

func clamp(v int) int {
	switch {
	case v < 0:
		return 0
	case v > 100:
		return 100
	default:
		return v
	}
}

// Grade maps a score to a short label, so that output can say something
// meaningful without the reader having to calibrate against an arbitrary
// number.
func Grade(score int) string {
	switch {
	case score >= 90:
		return "excellent"
	case score >= 75:
		return "good"
	case score >= 50:
		return "needs attention"
	case score >= 25:
		return "poor"
	default:
		return "critical"
	}
}
