// Package csvio contains CSV-specific I/O helpers used by the df CLI.
//
// The df tool is intentionally CSV-first. These helpers focus on predictable,
// boring behavior: open a file, read headers/rows, and normalize records so the
// rest of the program can assume a stable column count.
//
// Design choices worth knowing:
//
//   - We set csv.Reader.FieldsPerRecord = -1 to accept "jagged" CSV rows.
//     Real-world customer lists often contain rows with missing or extra fields.
//     Instead of failing fast, df normalizes rows to the header width.
//   - Normalization is deterministic:
//   - short rows are padded with "" (empty string)
//   - long rows are truncated to the header width
//
// In other words: headers define the schema, and every row is coerced to match.
package csvio

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
)

// ReadHeaders reads and returns only the header row from a CSV file.
//
// The returned slice is the column names exactly as they appear in the file.
// This function does not attempt to trim whitespace, de-duplicate names, or
// validate "meaningful" headers; callers decide what to do with the result.
//
// Errors are wrapped with context (e.g. "open csv", "read headers") to make
// CLI error messages more actionable.
func ReadHeaders(path string) ([]string, error) {
	// Open the file for reading.
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	// Use the standard library CSV reader. FieldsPerRecord = -1 tells the reader
	// not to enforce a consistent field count per row. We normalize later based
	// on header width.
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1

	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read headers: %w", err)
	}

	return headers, nil
}

// ReadHead reads a CSV file and returns its headers along with the first n data rows.
//
// Rows are normalized to the header width via normalizeRow:
//
//   - If a row has fewer fields than headers, it is padded with "".
//   - If a row has more fields than headers, extra fields are discarded.
//
// This mirrors how many spreadsheet workflows behave: headers define the schema,
// and every record is forced to match that schema.
//
// Note: if n is 0, the function returns headers and an empty row slice.
func ReadHead(path string, n int) ([]string, [][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1

	// The first record is treated as headers, not data.
	headers, err := r.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read headers: %w", err)
	}

	// Pre-allocate capacity for n rows to reduce allocations when n is small.
	rows := make([][]string, 0, n)

	// Read up to n records, stopping early on EOF.
	for len(rows) < n {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("read row: %w", err)
		}

		rows = append(rows, normalizeRow(rec, len(headers)))
	}

	return headers, rows, nil
}

// normalizeRow coerces a CSV record to a fixed width.
//
// If row is already the desired width, it is returned as-is.
// Otherwise, a new slice of length width is allocated and populated:
//
//   - Values that exist are copied into the output slice.
//   - Missing fields remain as the zero value "".
//   - Extra fields beyond width are ignored.
//
// This function deliberately does not attempt to interpret values (types, NULLs,
// trimming) — it is purely structural normalization.
func normalizeRow(row []string, width int) []string {
	if len(row) == width {
		return row
	}

	out := make([]string, width)

	// Copy as many fields as will fit.
	copyLen := len(row)
	if copyLen > width {
		copyLen = width
	}
	copy(out, row[:copyLen])

	return out
}
