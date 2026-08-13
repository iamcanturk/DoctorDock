// Package report renders a model.Report for a consumer.
//
// Rendering lives entirely here. Nothing in scanner, rules or pkg/model knows
// what a terminal or an ANSI escape is, so adding a renderer — a Bubble Tea TUI
// in v0.7, SARIF in v0.5 — means adding a file, not restructuring anything.
package report

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// Format selects an output renderer.
type Format string

const (
	FormatTerminal Format = "terminal"
	FormatJSON     Format = "json"
)

// ParseFormat converts a --format flag value into a Format.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "terminal", "text", "tty":
		return FormatTerminal, nil
	case "json":
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("unknown format %q (want: terminal, json)", s)
	}
}

// Options configure rendering.
type Options struct {
	// Color enables ANSI colour. Callers should derive it with ShouldColor.
	Color bool
	// ShowAll disables the per-rule cap on low-severity findings.
	ShowAll bool
	// Width is the target line width. Zero means the default.
	Width int
}

// Renderer writes a report to a stream.
type Renderer interface {
	Render(w io.Writer, r *model.Report) error
}

// New returns the renderer for a format.
func New(format Format, opts Options) (Renderer, error) {
	switch format {
	case FormatTerminal:
		return NewTerminal(opts), nil
	case FormatJSON:
		return NewJSON(), nil
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}
}

// ShouldColor decides whether to emit ANSI colour for a stream.
//
// It honours the NO_COLOR convention and FORCE_COLOR, and otherwise colours
// only a real terminal — piping to a file or into `jq` must not embed escape
// sequences.
func ShouldColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
