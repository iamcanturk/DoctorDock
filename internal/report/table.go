package report

import (
	"fmt"
	"strings"
)

// table renders aligned columns.
//
// Cells may contain ANSI escapes, which have length but no visible width, so
// each cell carries its plain-text width alongside the text it prints.
type table struct {
	headers []string
	rows    [][]cell
	limits  []int
}

type cell struct {
	text  string
	width int
}

// plain returns a cell with no colour.
func plain(s string) cell { return cell{text: s, width: len([]rune(s))} }

// coloured returns a cell whose printed text differs from its visible width.
func coloured(text, visible string) cell {
	return cell{text: text, width: len([]rune(visible))}
}

func newTable(headers ...string) *table {
	return &table{headers: headers}
}

// limit sets a maximum visible width per column. Zero means unlimited.
func (t *table) limit(widths ...int) *table {
	t.limits = widths
	return t
}

func (t *table) add(cells ...cell) {
	for i := range cells {
		if i < len(t.limits) && t.limits[i] > 0 {
			cells[i] = truncate(cells[i], t.limits[i])
		}
	}
	t.rows = append(t.rows, cells)
}

// render writes the table with two spaces between columns.
func (t *table) render(b *strings.Builder, p palette, indent string) {
	widths := make([]int, len(t.headers))
	for i, h := range t.headers {
		widths[i] = len([]rune(h))
	}
	for _, row := range t.rows {
		for i, c := range row {
			if i < len(widths) && c.width > widths[i] {
				widths[i] = c.width
			}
		}
	}

	b.WriteString(indent)
	for i, h := range t.headers {
		b.WriteString(p.dim(h))
		if i < len(t.headers)-1 {
			b.WriteString(strings.Repeat(" ", widths[i]-len([]rune(h))+2))
		}
	}
	b.WriteString("\n")

	for _, row := range t.rows {
		b.WriteString(indent)
		for i, c := range row {
			b.WriteString(c.text)
			if i < len(row)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-c.width+2))
			}
		}
		b.WriteString("\n")
	}
}

// truncate shortens a cell to a visible width, marking the cut with an
// ellipsis. It only handles uncoloured cells, which is all the resource tables
// produce for the columns that can overflow.
func truncate(c cell, width int) cell {
	if c.width <= width || width < 2 {
		return c
	}
	runes := []rune(c.text)
	if len(runes) != c.width {
		// The cell contains escapes; truncating would cut a sequence in half.
		return c
	}
	return plain(string(runes[:width-1]) + "…")
}

func countLabel(n int, singular string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %ss", n, singular)
}
