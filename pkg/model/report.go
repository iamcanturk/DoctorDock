package model

import "time"

// SchemaVersion is the version of the JSON document produced by DoctorDock.
//
// It follows semver independently of the binary version:
//   - adding a field is a minor bump
//   - removing or renaming a field, or changing a field's meaning, is a major bump
//
// Clients should refuse a document whose major version they do not understand.
// See docs/JSON_SCHEMA.md.
const SchemaVersion = "1.0"

// ToolInfo identifies the binary that produced a report, so that a consumer
// looking at an old artifact knows exactly what generated it.
type ToolInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
}

// Report is the top-level document DoctorDock produces.
//
// This is the integration contract for every non-Go client: the macOS app, CI
// pipelines, editor extensions and any optional AI layer decode this shape.
// See docs/adr/0004-json-as-the-gui-contract.md.
type Report struct {
	SchemaVersion string    `json:"schema_version"`
	GeneratedAt   time.Time `json:"generated_at"`
	Tool          ToolInfo  `json:"tool"`

	Docker DockerInfo `json:"docker"`

	// Score is the health score in [0, 100], where 100 means no findings.
	Score   int     `json:"score"`
	Summary Summary `json:"summary"`

	// Findings is always present and never null, so clients can iterate
	// without a nil check.
	Findings []Finding `json:"findings"`

	// The resource lists let a client render the full environment without a
	// second invocation. They are omitted when the caller asked for a summary
	// only.
	Containers []Container `json:"containers,omitempty"`
	Images     []Image     `json:"images,omitempty"`
	Volumes    []Volume    `json:"volumes,omitempty"`
	Networks   []Network   `json:"networks,omitempty"`

	// SkippedRules lists rule IDs that were disabled for this run, so that a
	// reader can tell "no findings" apart from "not checked".
	SkippedRules []string `json:"skipped_rules,omitempty"`
}

// HighestSeverity returns the most severe finding level in the report, and
// false when the report is clean.
func (r *Report) HighestSeverity() (Severity, bool) {
	return HighestSeverity(r.Findings)
}

// FindingsBySeverity returns findings ordered most severe first. Within a
// severity, the original rule order is preserved so output is deterministic.
func (r *Report) FindingsBySeverity() []Finding {
	out := make([]Finding, 0, len(r.Findings))
	for i := len(AllSeverities) - 1; i >= 0; i-- {
		for _, f := range r.Findings {
			if f.Severity == AllSeverities[i] {
				out = append(out, f)
			}
		}
	}
	return out
}
