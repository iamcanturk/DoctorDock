package model

import (
	"fmt"
	"math"
	"time"
)

// FormatBytes renders a byte count the way Docker does: decimal units, one
// decimal place, e.g. "2.8 GB".
//
// It lives in pkg/model rather than in the terminal renderer so that every
// client — CLI, macOS app, editor extension — presents the same size the same
// way. A user comparing the app to the CLI should not see two different numbers.
func FormatBytes(b int64) string {
	if b < 0 {
		return "unknown"
	}
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTP"[exp])
}

// FormatDuration renders an age in the coarse, human form Docker uses:
// "3 days", "5 hours", "2 minutes".
func FormatDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "less than a minute"
	case d < time.Hour:
		return plural(int(d.Minutes()), "minute")
	case d < 24*time.Hour:
		return plural(int(d.Hours()), "hour")
	case d < 365*24*time.Hour:
		return plural(int(math.Round(d.Hours()/24)), "day")
	default:
		return plural(int(math.Round(d.Hours()/24/365)), "year")
	}
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
