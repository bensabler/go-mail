package csvio

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
)

// ReadHeaders reads only the header row from a CSV file.
func ReadHeaders(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // allow variable-length rows; we validate later

	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read headers: %w", err)
	}

	return headers, nil
}

// ReadHead returns headers + the first n rows.
// Rows are normalized to header length (padded with "" or truncated).
func ReadHead(path string, n int) ([]string, [][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1

	headers, err := r.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read headers: %w", err)
	}

	rows := make([][]string, 0, n)
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

func normalizeRow(row []string, width int) []string {
	if len(row) == width {
		return row
	}
	out := make([]string, width)

	// copy what we have
	copyLen := len(row)
	if copyLen > width {
		copyLen = width
	}
	copy(out, row[:copyLen])

	// remaining cells default to ""
	return out
}
