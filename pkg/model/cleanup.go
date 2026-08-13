package model

import "time"

// Risk expresses what removing a resource could cost.
//
// It is separate from Severity because the two answer different questions:
// severity is how bad a problem is, risk is how bad removing it could be.
type Risk string

const (
	// RiskSafe means Docker's own prune would remove it and it cannot be
	// needed again — dangling images, unused networks.
	RiskSafe Risk = "safe"
	// RiskReview means removable, but the user may have wanted it — unused
	// tagged images, stopped containers.
	RiskReview Risk = "review"
	// RiskDataLoss means removal may destroy the only copy of real data. Only
	// volumes carry this.
	RiskDataLoss Risk = "data-loss"
)

// AllRisks lists risk levels from least to most dangerous.
var AllRisks = []Risk{RiskSafe, RiskReview, RiskDataLoss}

// String implements fmt.Stringer.
func (r Risk) String() string { return string(r) }

// CleanupItem is one resource a cleanup would remove, or did remove.
type CleanupItem struct {
	Resource ResourceKind `json:"resource"`
	ID       string       `json:"id"`
	Name     string       `json:"name"`

	// Reason explains why the resource is removable, in the user's terms.
	Reason string `json:"reason"`
	Risk   Risk   `json:"risk"`

	// Size is the disk space removal would reclaim, in bytes. -1 when the
	// daemon does not report it, which is normal for volumes and networks.
	Size int64 `json:"size"`

	// Removed is true only after a successful --apply. It is always false in a
	// dry run, which is what distinguishes the two documents.
	Removed bool `json:"removed"`

	// Error is the daemon's refusal when removal failed. A failure is reported,
	// never retried with force.
	Error string `json:"error,omitempty"`
}

// CleanupSummary aggregates a plan.
type CleanupSummary struct {
	Total      int                  `json:"total"`
	ByResource map[ResourceKind]int `json:"by_resource"`
	ByRisk     map[Risk]int         `json:"by_risk"`

	// ReclaimableBytes is what the plan would free. Items whose size is
	// unknown contribute nothing, so this is a lower bound.
	ReclaimableBytes int64 `json:"reclaimable_bytes"`

	// Removed, Failed and ReclaimedBytes are only meaningful after --apply.
	Removed        int   `json:"removed"`
	Failed         int   `json:"failed"`
	ReclaimedBytes int64 `json:"reclaimed_bytes"`
}

// CleanupPlan is what `doctordock cleanup` produces, in both dry-run and
// applied form. Like Report, it is a versioned contract for non-Go clients.
type CleanupPlan struct {
	SchemaVersion string    `json:"schema_version"`
	GeneratedAt   time.Time `json:"generated_at"`
	Tool          ToolInfo  `json:"tool"`

	// Applied is false for a dry run. A client must check this before telling
	// a user that anything was deleted.
	Applied bool `json:"applied"`

	// Items is always present and never null.
	Items   []CleanupItem  `json:"items"`
	Summary CleanupSummary `json:"summary"`
}

// SummarizeCleanup computes the aggregate view of a set of items.
func SummarizeCleanup(items []CleanupItem) CleanupSummary {
	s := CleanupSummary{
		Total:      len(items),
		ByResource: make(map[ResourceKind]int, 4),
		ByRisk:     make(map[Risk]int, len(AllRisks)),
	}
	// Every risk level is present even at zero, so clients can index safely.
	for _, r := range AllRisks {
		s.ByRisk[r] = 0
	}

	for _, item := range items {
		s.ByResource[item.Resource]++
		s.ByRisk[item.Risk]++
		if item.Size > 0 {
			s.ReclaimableBytes += item.Size
		}
		switch {
		case item.Removed:
			s.Removed++
			if item.Size > 0 {
				s.ReclaimedBytes += item.Size
			}
		case item.Error != "":
			s.Failed++
		}
	}
	return s
}

// HasRisk reports whether any item carries the given risk level.
func HasRisk(items []CleanupItem, risk Risk) bool {
	for _, item := range items {
		if item.Risk == risk {
			return true
		}
	}
	return false
}
