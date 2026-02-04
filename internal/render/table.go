package render

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

type TableOptions struct {
	MaxCellWidth int
	ShowRowIndex bool
}

// PrintTable prints headers + rows as a readable fixed-width table.
// Simple on purpose: no external deps, predictable output.
func PrintTable(w io.Writer, headers []string, rows [][]string, opts TableOptions) {
	if opts.MaxCellWidth <= 0 {
		opts.MaxCellWidth = 32
	}

	// Determine column widths (bounded by MaxCellWidth)
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

	// Row index width if enabled
	idxWidth := 0
	if opts.ShowRowIndex {
		// Enough for up to 999999 rows displayed nicely
		idxWidth = 5
		fmt.Fprintf(w, "%-*s  ", idxWidth, "#")
	}

	// Print header row
	for i, h := range headers {
		fmt.Fprintf(w, "%-*s", widths[i], clip(h, opts.MaxCellWidth))
		if i < len(headers)-1 {
			fmt.Fprint(w, "  ")
		}
	}
	fmt.Fprintln(w)

	// Print separator
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

	// Print data rows
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

func clip(s string, max int) string {
	if max <= 0 {
		return s
	}
	if runeLen(s) <= max {
		return s
	}
	// leave room for ellipsis
	if max <= 1 {
		return "…"
	}
	return takeRunes(s, max-1) + "…"
}

func takeRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := strings.Builder{}
	out.Grow(len(s))
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

func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

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
