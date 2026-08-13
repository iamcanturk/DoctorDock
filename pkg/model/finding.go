package model

// Finding is a single detected issue.
//
// Field names in the JSON encoding are public API: the macOS app, CI
// integrations and any other client decode this shape. Renaming a field is a
// breaking change that requires a schema_version bump. See
// docs/adr/0004-json-as-the-gui-contract.md.
type Finding struct {
	// ID is the stable rule identifier, e.g. "DD005". IDs are never reused.
	ID string `json:"id"`

	// Rule is the human-readable name of the rule that produced this finding,
	// e.g. "Docker socket exposed". Clients group by it; unlike Title it is
	// identical across every finding from the same rule.
	Rule string `json:"rule"`

	// Severity is how much this should worry the operator.
	Severity Severity `json:"severity"`

	// Category is the kind of concern this represents.
	Category Category `json:"category"`

	// Resource is the kind of Docker object affected.
	Resource ResourceKind `json:"resource"`

	// ResourceID is the full Docker identifier of the affected object:
	// container ID, image ID, volume name or network ID.
	ResourceID string `json:"resource_id"`

	// ResourceName is the human-readable name used in output, e.g. "api" or
	// "backend:latest". Falls back to a short ID when the object is unnamed.
	ResourceName string `json:"resource_name"`

	// Title is a one-line statement of what is wrong.
	Title string `json:"title"`

	// Description explains why it matters.
	Description string `json:"description"`

	// Recommendation states what the operator should actually do about it.
	Recommendation string `json:"recommendation"`

	// Details carries rule-specific structured data, for example the offending
	// host path for DD004 or the image size for DD016. Clients may render it;
	// they must tolerate keys they do not recognise.
	Details map[string]string `json:"details,omitempty"`
}

// SeverityCounts tallies findings per severity level. Every level is always
// present in the JSON so that clients can index without nil checks.
type SeverityCounts struct {
	Info     int `json:"info"`
	Low      int `json:"low"`
	Medium   int `json:"medium"`
	High     int `json:"high"`
	Critical int `json:"critical"`
}

// Total returns the sum across all severities.
func (c SeverityCounts) Total() int {
	return c.Info + c.Low + c.Medium + c.High + c.Critical
}

// Get returns the count for a single severity.
func (c SeverityCounts) Get(s Severity) int {
	switch s {
	case SeverityInfo:
		return c.Info
	case SeverityLow:
		return c.Low
	case SeverityMedium:
		return c.Medium
	case SeverityHigh:
		return c.High
	case SeverityCritical:
		return c.Critical
	default:
		return 0
	}
}

// CountSeverities tallies findings by severity.
func CountSeverities(findings []Finding) SeverityCounts {
	var c SeverityCounts
	for _, f := range findings {
		switch f.Severity {
		case SeverityInfo:
			c.Info++
		case SeverityLow:
			c.Low++
		case SeverityMedium:
			c.Medium++
		case SeverityHigh:
			c.High++
		case SeverityCritical:
			c.Critical++
		}
	}
	return c
}

// HighestSeverity returns the most severe level present in findings, and false
// if there are no findings at all.
func HighestSeverity(findings []Finding) (Severity, bool) {
	if len(findings) == 0 {
		return "", false
	}
	highest := findings[0].Severity
	for _, f := range findings[1:] {
		if f.Severity.Rank() > highest.Rank() {
			highest = f.Severity
		}
	}
	return highest, true
}
