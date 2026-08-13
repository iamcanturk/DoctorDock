package report

import "github.com/iamcanturk/DoctorDock/pkg/model"

// Styler applies the same terminal styling the renderers use, for callers
// outside this package. It exists so that `doctordock rules` and
// `doctordock version` colour their output identically to a scan without
// reimplementing the escape codes or the NO_COLOR check.
type Styler struct{ p palette }

// NewStyler returns a Styler.
func NewStyler(color bool) Styler { return Styler{p: newPalette(color)} }

func (s Styler) Bold(v string) string  { return s.p.bold(v) }
func (s Styler) Dim(v string) string   { return s.p.dim(v) }
func (s Styler) Red(v string) string   { return s.p.red(v) }
func (s Styler) Green(v string) string { return s.p.green(v) }

// Severity colours text according to a severity level.
func (s Styler) Severity(sev model.Severity, text string) string {
	t := &TerminalRenderer{c: s.p}
	return t.severityColour(sev)(text)
}

// Wrap breaks text into lines no longer than width, on word boundaries.
func Wrap(text string, width int) []string { return wrap(text, width) }

// palette wraps text in ANSI escapes, or returns it untouched when colour is
// disabled. Every renderer goes through this so that NO_COLOR and piped output
// are handled in one place rather than at each call site.
type palette struct {
	enabled bool
}

func newPalette(enabled bool) palette { return palette{enabled: enabled} }

const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiCyan    = "\033[36m"
	ansiBoldRed = "\033[1;31m"
)

func (p palette) wrap(code, s string) string {
	if !p.enabled || s == "" {
		return s
	}
	return code + s + ansiReset
}

func (p palette) bold(s string) string    { return p.wrap(ansiBold, s) }
func (p palette) dim(s string) string     { return p.wrap(ansiDim, s) }
func (p palette) red(s string) string     { return p.wrap(ansiRed, s) }
func (p palette) boldRed(s string) string { return p.wrap(ansiBoldRed, s) }
func (p palette) green(s string) string   { return p.wrap(ansiGreen, s) }
func (p palette) yellow(s string) string  { return p.wrap(ansiYellow, s) }
func (p palette) cyan(s string) string    { return p.wrap(ansiCyan, s) }

// dimRaw is dim without a reset-safe wrapper, for strings that are already
// inside another colour sequence such as the score bar's unfilled cells.
func (p palette) dimRaw(s string) string { return p.wrap(ansiDim, s) }
