// Package render contains small, dependency-free helpers for rendering output in
// a human-friendly way.
//
// The df CLI aims to be usable in terminals first. This package implements a
// simple fixed-width table renderer that:
//
//   - computes column widths from headers + visible rows
//   - truncates long cell values with an ellipsis (…)
//   - optionally prepends a row index column
//
// The output is designed for quick inspection and copy/paste, not for perfect
// alignment in every terminal/font scenario.
package render

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// TableOptions controls how tables are rendered.
//
// MaxCellWidth limits the number of runes printed per cell. Values longer than
// this limit are clipped and suffixed with an ellipsis (…).
//
// ShowRowIndex adds a leading "#" column with a zero-based row index. This is
// useful when discussing records with coworkers or comparing against spreadsheet
// row numbers during troubleshooting.
type TableOptions struct {
	MaxCellWidth int
	ShowRowIndex bool
}

// PrintTable prints headers and rows as a readable fixed-width table.
//
// The renderer is intentionally small and deterministic:
//   - No external dependencies
//   - No color / ANSI formatting
//   - No multiline cells
//
// Column widths are computed from the supplied headers and rows, bounded by
// opts.MaxCellWidth. If a given row is shorter than the header count, missing
// cells are treated as empty strings.
//
// Note: This renderer counts width in runes (not bytes), which works well for
// Unicode text but does not account for terminal display width nuances such as
// combining characters or East Asian wide glyphs. For df's current use cases,
// rune width is a practical and stable approximation.
func PrintTable(w io.Writer, headers []string, rows [][]string, opts TableOptions) {
	// Default width cap if not specified or invalid.
	if opts.MaxCellWidth <= 0 {
		opts.MaxCellWidth = 32
	}

	// Determine per-column widths (bounded by MaxCellWidth). We consider:
	//   1) header text
	//   2) each cell in the provided rows
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = min(opts.MaxCellWidth, runeLen(h))
	}
	for _, row := range rows {
		for i := range headers {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			widths[i] = max(widths[i], min(opts.MaxCellWidth, runeLen(cell)))
		}
	}

	// Row index width if enabled.
	// This is a fixed width to keep output stable and avoid recomputing based on
	// the number of displayed rows.
	idxWidth := 0
	if opts.ShowRowIndex {
		// Enough for up to 99999 displayed rows without breaking alignment.
		// (The tool currently prints small previews like head/tail.)
		idxWidth = 5
		fmt.Fprintf(w, "%-*s  ", idxWidth, "#")
	}

	// Header row.
	for i, h := range headers {
		fmt.Fprintf(w, "%-*s", widths[i], clip(h, opts.MaxCellWidth))
		if i < len(headers)-1 {
			fmt.Fprint(w, "  ")
		}
	}
	fmt.Fprintln(w)

	// Separator row.
	if opts.ShowRowIndex {
		fmt.Fprintf(w, "%s  ", strings.Repeat("-", idxWidth))
	}
	for i := range headers {
		fmt.Fprint(w, strings.Repeat("-", widths[i]))
		if i < len(headers)-1 {
			fmt.Fprint(w, "  ")
		}
	}
	fmt.Fprintln(w)

	// Data rows.
	for ri, row := range rows {
		if opts.ShowRowIndex {
			fmt.Fprintf(w, "%-*d  ", idxWidth, ri)
		}
		for ci := range headers {
			cell := ""
			if ci < len(row) {
				cell = row[ci]
			}
			fmt.Fprintf(w, "%-*s", widths[ci], clip(cell, opts.MaxCellWidth))
			if ci < len(headers)-1 {
				fmt.Fprint(w, "  ")
			}
		}
		fmt.Fprintln(w)
	}
}

// clip truncates s to at most max runes. If truncation occurs, the result ends
// with an ellipsis (…).
//
// The function is rune-aware (Unicode-safe) and is used to keep table layout
// stable even when cells contain very long strings.
func clip(s string, max int) string {
	if max <= 0 {
		return s
	}
	if runeLen(s) <= max {
		return s
	}

	// Leave room for the ellipsis.
	if max <= 1 {
		return "…"
	}
	return takeRunes(s, max-1) + "…"
}

// takeRunes returns the first n runes of s (Unicode-safe).
// If n <= 0, it returns an empty string.
func takeRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := strings.Builder{}
	out.Grow(len(s)) // best-effort; bytes != runes but helps reduce reallocations
	count := 0
	for _, r := range s {
		out.WriteRune(r)
		count++
		if count >= n {
			break
		}
	}
	return out.String()
}

// runeLen returns the number of runes in s.
// This is used for approximate, Unicode-safe width calculations.
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// min and max are small helpers used when computing column widths.
// They are kept local to avoid pulling in additional packages.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
