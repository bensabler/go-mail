package nulls

import "strings"

type Policy struct {
	TreatBlanks      bool
	TreatNA          bool
	TreatNULLLiteral bool
}

// IsNull reports whether s should be treated as NULL under the policy.
// Matching is case-insensitive for NA/N/A/NULL and ignores surrounding whitespace.
func (p Policy) IsNull(s string) bool {
	trimmed := strings.TrimSpace(s)

	if p.TreatBlanks && trimmed == "" {
		return true
	}

	upper := strings.ToUpper(trimmed)

	if p.TreatNA {
		if upper == "NA" || upper == "N/A" {
			return true
		}
	}

	if p.TreatNULLLiteral {
		if upper == "NULL" {
			return true
		}
	}

	return false
}
