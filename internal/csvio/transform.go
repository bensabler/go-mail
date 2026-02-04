package csvio

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"

	"github.com/bensabler/go-mail/internal/nulls"
)

type NullifyStats struct {
	RowsRead       int
	CellsChecked   int
	CellsNullified int
}

// NullifyFile reads input CSV and writes output CSV, converting values that match
// the null policy to empty strings (CSV representation of NULL).
func NullifyFile(inputPath, outputPath string, policy nulls.Policy) (NullifyStats, error) {
	in, err := os.Open(inputPath)
	if err != nil {
		return NullifyStats{}, fmt.Errorf("open input csv: %w", err)
	}
	defer in.Close()

	out, err := os.Create(outputPath)
	if err != nil {
		return NullifyStats{}, fmt.Errorf("create output csv: %w", err)
	}
	defer func() {
		_ = out.Close()
	}()

	r := csv.NewReader(in)
	r.FieldsPerRecord = -1

	w := csv.NewWriter(out)
	defer w.Flush()

	headers, err := r.Read()
	if err != nil {
		return NullifyStats{}, fmt.Errorf("read headers: %w", err)
	}
	if err := w.Write(headers); err != nil {
		return NullifyStats{}, fmt.Errorf("write headers: %w", err)
	}

	stats := NullifyStats{}

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return stats, fmt.Errorf("read row: %w", err)
		}

		stats.RowsRead++

		// Normalize row length to header length
		rec = normalizeRow(rec, len(headers))

		for i := range rec {
			stats.CellsChecked++
			if policy.IsNull(rec[i]) {
				// CSV NULL convention: empty field
				if rec[i] != "" {
					stats.CellsNullified++
				} else {
					// empty already, still counts as "checked" but not "changed"
					// (we could count it, but stats are more meaningful this way)
				}
				rec[i] = ""
			}
		}

		if err := w.Write(rec); err != nil {
			return stats, fmt.Errorf("write row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return stats, fmt.Errorf("flush output csv: %w", err)
	}

	return stats, nil
}
