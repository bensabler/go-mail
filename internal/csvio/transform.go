// Package csvio contains CSV-specific I/O helpers used by the df CLI.
//
// This file focuses on *transforming* CSV data rather than merely reading it.
// In particular, it implements normalization of NULL-like values in a streaming,
// row-by-row fashion so large files can be processed without loading everything
// into memory.
package csvio

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"

	"github.com/bensabler/go-mail/internal/nulls"
)

// NullifyStats captures a summary of a nullify operation.
//
// These statistics are intended for operator visibility and auditing rather than
// strict accounting. In particular:
//
//   - RowsRead counts data rows processed (header excluded).
//   - CellsChecked counts every cell inspected against the null policy.
//   - CellsNullified counts cells whose value changed as a result of nullification.
//
// A cell that is already empty ("") and matches the null policy is considered
// "checked" but not "nullified".
type NullifyStats struct {
	RowsRead       int
	CellsChecked   int
	CellsNullified int
}

// NullifyFile reads an input CSV file and writes a new CSV file with NULL-like
// values normalized according to the provided policy.
//
// In this tool, CSV "NULL" is represented as an empty field ("") on output.
// The function operates in a streaming manner:
//
//   - The input file is read row-by-row.
//   - Each row is normalized to the header width.
//   - Each cell is checked against the null policy.
//   - Matching values are replaced with "".
//   - The transformed row is written immediately.
//
// This design keeps memory usage low and makes behavior predictable for large
// mailing lists.
//
// The header row is copied verbatim from input to output and is not modified.
//
// Errors are wrapped with contextual information to make CLI error messages
// actionable (e.g., distinguishing read errors from write errors).
func NullifyFile(inputPath, outputPath string, policy nulls.Policy) (NullifyStats, error) {
	// Open the input CSV for reading.
	in, err := os.Open(inputPath)
	if err != nil {
		return NullifyStats{}, fmt.Errorf("open input csv: %w", err)
	}
	defer in.Close()

	// Create (or truncate) the output CSV.
	out, err := os.Create(outputPath)
	if err != nil {
		return NullifyStats{}, fmt.Errorf("create output csv: %w", err)
	}
	// Best-effort close; write errors are handled via csv.Writer.
	defer func() {
		_ = out.Close()
	}()

	// Configure CSV reader to allow variable-length rows.
	// Structural normalization happens explicitly via normalizeRow.
	r := csv.NewReader(in)
	r.FieldsPerRecord = -1

	// csv.Writer buffers output; Flush is required to surface write errors.
	w := csv.NewWriter(out)
	defer w.Flush()

	// Read and write headers unchanged.
	headers, err := r.Read()
	if err != nil {
		return NullifyStats{}, fmt.Errorf("read headers: %w", err)
	}
	if err := w.Write(headers); err != nil {
		return NullifyStats{}, fmt.Errorf("write headers: %w", err)
	}

	stats := NullifyStats{}

	// Process data rows until EOF.
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return stats, fmt.Errorf("read row: %w", err)
		}

		stats.RowsRead++

		// Normalize the record to match the header width.
		// Short rows are padded with "", long rows are truncated.
		rec = normalizeRow(rec, len(headers))

		// Apply null policy cell-by-cell.
		for i := range rec {
			stats.CellsChecked++

			if policy.IsNull(rec[i]) {
				// CSV NULL convention: empty field.
				// Only count as "nullified" if the value actually changed.
				if rec[i] != "" {
					stats.CellsNullified++
				}
				rec[i] = ""
			}
		}

		if err := w.Write(rec); err != nil {
			return stats, fmt.Errorf("write row: %w", err)
		}
	}

	// Flush buffered output and check for write errors.
	w.Flush()
	if err := w.Error(); err != nil {
		return stats, fmt.Errorf("flush output csv: %w", err)
	}

	return stats, nil
}
