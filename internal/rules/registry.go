package rules

import (
	"fmt"
	"sort"
)

// registry is the complete set of rules DoctorDock ships.
//
// Adding a rule means adding one line here. Nothing else in the codebase
// changes: the scanner iterates whatever this returns, and the renderers work
// from the findings.
//
// Rule IDs are sequential and are never reused, even if a rule is removed —
// a suppression referring to DD007 must not silently start suppressing
// something else after an upgrade.
var registry = []Rule{
	// Security
	RootUser{},              // DD001
	PrivilegedContainer{},   // DD002
	HostNetwork{},           // DD003
	SensitiveHostMount{},    // DD004
	DockerSocketMount{},     // DD005
	ExposedSensitivePort{},  // DD006
	DangerousCapabilities{}, // DD009

	// Configuration
	MissingHealthcheck{},   // DD007
	MissingRestartPolicy{}, // DD008
	MutableImageTag{},      // DD011

	// Resource
	NoMemoryLimit{},  // DD010
	OversizedImage{}, // DD016

	// Performance
	UnhealthyContainer{}, // DD012
	RestartLoop{},        // DD013

	// Cleanup
	DanglingImage{}, // DD014
	UnusedImage{},   // DD015
	UnusedVolume{},  // DD017
	UnusedNetwork{}, // DD018
}

// All returns every rule, ordered by ID so that output is deterministic.
func All() []Rule {
	out := make([]Rule, len(registry))
	copy(out, registry)
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// ByID returns the rule with the given ID.
func ByID(id string) (Rule, bool) {
	for _, r := range registry {
		if r.ID() == id {
			return r, true
		}
	}
	return nil, false
}

// Validate checks the registry's internal consistency. It runs as a test, and
// at startup would be wasted work — but it is exported so that the test and any
// future plugin loader share one definition of "valid".
func Validate(list []Rule) error {
	seen := make(map[string]bool, len(list))
	for _, r := range list {
		id := r.ID()
		switch {
		case id == "":
			return fmt.Errorf("rule %T has an empty ID", r)
		case seen[id]:
			return fmt.Errorf("duplicate rule ID %s", id)
		case r.Name() == "":
			return fmt.Errorf("rule %s has an empty name", id)
		case r.Description() == "":
			return fmt.Errorf("rule %s has an empty description", id)
		case !r.Severity().Valid():
			return fmt.Errorf("rule %s has invalid severity %q", id, r.Severity())
		case !r.Category().Valid():
			return fmt.Errorf("rule %s has invalid category %q", id, r.Category())
		}
		seen[id] = true
	}
	return nil
}
