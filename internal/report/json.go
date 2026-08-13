package report

import (
	"encoding/json"
	"io"

	"github.com/iamcanturk/DoctorDock/pkg/model"
)

// JSONRenderer writes the report as the versioned JSON contract.
//
// This output is public API. The macOS app, CI pipelines and any other client
// decode it, so field names and their meanings are stable within a
// schema_version major. See docs/JSON_SCHEMA.md.
type JSONRenderer struct{}

// NewJSON returns a JSON renderer.
func NewJSON() *JSONRenderer { return &JSONRenderer{} }

// Render implements Renderer.
//
// Output is always indented. This is a diagnostics tool whose JSON is read by
// humans at least as often as by programs, and `jq` does not care either way.
func (JSONRenderer) Render(w io.Writer, r *model.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	// Escaping HTML would mangle image references and shell snippets inside
	// recommendations into < sequences.
	enc.SetEscapeHTML(false)
	return enc.Encode(r)
}
