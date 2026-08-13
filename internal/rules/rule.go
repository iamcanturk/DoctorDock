// Package rules holds the checks DoctorDock runs against a Docker environment.
//
// A rule is a self-contained type implementing Rule. Adding one means writing
// the type and adding a line to the registry — the engine, the scanner and the
// renderers do not change.
package rules

import (
	"context"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// Rule is a single check.
//
// Severity is the rule's *default* severity, used for documentation and for
// `doctordock rules`. A rule may emit a finding at a higher severity when the
// specific situation warrants it — DD004 escalates when the mounted path is
// the host root, for example.
type Rule interface {
	ID() string
	Name() string
	Category() model.Category
	Severity() model.Severity
	// Description explains, in one or two sentences, what the rule looks for.
	Description() string
	Check(ctx context.Context, target Target) []model.Finding
}

// Target is what a rule evaluates.
//
// It carries the whole environment rather than a single resource because rules
// such as "unused image" and "unused network" inherently need to cross-
// reference containers. Passing the snapshot to every rule keeps one interface
// instead of two plus a bridge. See
// docs/adr/0003-whole-environment-rule-target.md.
//
// It is a struct, not a bare *model.Environment, so that Dockerfile and Compose
// analysis can be added in v0.3/v0.4 without changing any existing rule.
type Target struct {
	Environment *model.Environment
	Options     Options
}

// Options holds the thresholds a user can tune. Zero values are replaced with
// the defaults by DefaultOptions/Normalize, so a partially-filled Options is
// safe to pass.
type Options struct {
	// LargeImageBytes is the size at or above which an image is reported as
	// oversized.
	LargeImageBytes int64
	// RestartLoopThreshold is the restart count at or above which a container
	// is treated as looping.
	RestartLoopThreshold int
}

// DefaultOptions returns the tuned defaults.
func DefaultOptions() Options {
	return Options{
		// 1.5 GB. Below this, large images are common enough in normal use
		// (JDK, CUDA, full Node toolchains) that flagging them is noise.
		LargeImageBytes: 1_500_000_000,
		// Docker's own `on-failure` default gives up at 5 attempts, which
		// makes it a natural line between "restarted a few times" and "stuck".
		RestartLoopThreshold: 5,
	}
}

// Normalize fills any unset field with its default.
func (o Options) Normalize() Options {
	d := DefaultOptions()
	if o.LargeImageBytes <= 0 {
		o.LargeImageBytes = d.LargeImageBytes
	}
	if o.RestartLoopThreshold <= 0 {
		o.RestartLoopThreshold = d.RestartLoopThreshold
	}
	return o
}

// newContainerFinding starts a finding pre-filled with the rule's identity and
// the container's identity. The rule fills in Title, Description,
// Recommendation and any Details.
func newContainerFinding(r Rule, c model.Container) model.Finding {
	return model.Finding{
		ID:           r.ID(),
		Rule:         r.Name(),
		Severity:     r.Severity(),
		Category:     r.Category(),
		Resource:     model.ResourceContainer,
		ResourceID:   c.ID,
		ResourceName: c.Name,
	}
}

func newImageFinding(r Rule, img model.Image) model.Finding {
	return model.Finding{
		ID:           r.ID(),
		Rule:         r.Name(),
		Severity:     r.Severity(),
		Category:     r.Category(),
		Resource:     model.ResourceImage,
		ResourceID:   img.ID,
		ResourceName: img.DisplayName(),
	}
}

func newVolumeFinding(r Rule, v model.Volume) model.Finding {
	return model.Finding{
		ID:           r.ID(),
		Rule:         r.Name(),
		Severity:     r.Severity(),
		Category:     r.Category(),
		Resource:     model.ResourceVolume,
		ResourceID:   v.Name,
		ResourceName: v.Name,
	}
}

func newNetworkFinding(r Rule, n model.Network) model.Finding {
	return model.Finding{
		ID:           r.ID(),
		Rule:         r.Name(),
		Severity:     r.Severity(),
		Category:     r.Category(),
		Resource:     model.ResourceNetwork,
		ResourceID:   n.ID,
		ResourceName: n.Name,
	}
}
