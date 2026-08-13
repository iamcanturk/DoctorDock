package model

import (
	"fmt"
	"strings"
)

// Severity expresses how much a finding should worry the operator.
//
// It is a string rather than an integer so that the JSON output stays readable
// and stable: adding a level between two existing ones must never renumber the
// others. Use Rank for ordering.
type Severity string

const (
	// SeverityInfo is worth knowing but is not a problem.
	SeverityInfo Severity = "INFO"
	// SeverityLow is a minor deviation from good practice.
	SeverityLow Severity = "LOW"
	// SeverityMedium weakens the setup but needs another factor to be exploited.
	SeverityMedium Severity = "MEDIUM"
	// SeverityHigh is a real weakness that should be fixed.
	SeverityHigh Severity = "HIGH"
	// SeverityCritical means reaching this container is equivalent to owning the host.
	SeverityCritical Severity = "CRITICAL"
)

// AllSeverities lists every severity from least to most severe.
var AllSeverities = []Severity{
	SeverityInfo,
	SeverityLow,
	SeverityMedium,
	SeverityHigh,
	SeverityCritical,
}

var severityRanks = map[Severity]int{
	SeverityInfo:     0,
	SeverityLow:      1,
	SeverityMedium:   2,
	SeverityHigh:     3,
	SeverityCritical: 4,
}

// Rank returns the ordering position of s, with INFO lowest. An unrecognised
// severity ranks lowest so that malformed input never escalates.
func (s Severity) Rank() int {
	return severityRanks[s]
}

// String implements fmt.Stringer.
func (s Severity) String() string { return string(s) }

// Valid reports whether s is one of the defined severities.
func (s Severity) Valid() bool {
	_, ok := severityRanks[s]
	return ok
}

// ParseSeverity converts user input such as a --fail-on flag value into a
// Severity. It is case-insensitive.
func ParseSeverity(s string) (Severity, error) {
	sev := Severity(strings.ToUpper(strings.TrimSpace(s)))
	if !sev.Valid() {
		return "", fmt.Errorf("unknown severity %q (want one of: info, low, medium, high, critical)", s)
	}
	return sev, nil
}

// Category groups findings by the kind of concern they represent, so that
// output can be filtered by what the reader currently cares about.
type Category string

const (
	// CategorySecurity covers weaknesses an attacker could exploit.
	CategorySecurity Category = "SECURITY"
	// CategoryPerformance covers settings that degrade runtime behaviour.
	CategoryPerformance Category = "PERFORMANCE"
	// CategoryResource covers limits, quotas and consumption.
	CategoryResource Category = "RESOURCE"
	// CategoryConfiguration covers setups that are simply wrong or fragile.
	CategoryConfiguration Category = "CONFIGURATION"
	// CategoryCleanup covers reclaimable, unused resources.
	CategoryCleanup Category = "CLEANUP"
)

// AllCategories lists every defined category.
var AllCategories = []Category{
	CategorySecurity,
	CategoryPerformance,
	CategoryResource,
	CategoryConfiguration,
	CategoryCleanup,
}

// Valid reports whether c is one of the defined categories.
func (c Category) Valid() bool {
	for _, known := range AllCategories {
		if c == known {
			return true
		}
	}
	return false
}

// String implements fmt.Stringer.
func (c Category) String() string { return string(c) }

// ParseCategory converts user input into a Category. It is case-insensitive.
func ParseCategory(s string) (Category, error) {
	cat := Category(strings.ToUpper(strings.TrimSpace(s)))
	if !cat.Valid() {
		return "", fmt.Errorf("unknown category %q (want one of: security, performance, resource, configuration, cleanup)", s)
	}
	return cat, nil
}

// ResourceKind identifies which sort of Docker object a finding is about.
type ResourceKind string

const (
	ResourceContainer ResourceKind = "container"
	ResourceImage     ResourceKind = "image"
	ResourceVolume    ResourceKind = "volume"
	ResourceNetwork   ResourceKind = "network"
	// ResourceSystem is used for findings about the daemon or host as a whole.
	ResourceSystem ResourceKind = "system"
)

// String implements fmt.Stringer.
func (r ResourceKind) String() string { return string(r) }
