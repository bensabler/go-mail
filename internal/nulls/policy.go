// Package nulls defines how the df tool interprets "NULL-like" values in input data.
//
// CSV has no native NULL concept, so df uses a policy-driven approach to decide
// which string values should be treated as NULL during transformations.
//
// This package is intentionally small and explicit:
//   - No heuristics
//   - No type inference
//   - No column-specific logic
//
// All NULL semantics are opt-in via Policy fields so behavior is predictable,
// testable, and easy to explain to operators and customers.
package nulls

import "strings"

// Policy describes which values should be interpreted as NULL.
//
// Each field enables recognition of a specific class of "NULL-like" inputs.
// Policies are typically constructed directly from CLI flags.
//
// The policy does not define *how* NULLs are represented in output; it only
// answers the question: "Should this input value be considered NULL?"
type Policy struct {
	// TreatBlanks controls whether empty or whitespace-only values are treated
	// as NULL. When enabled, values like "", " ", and "\t" are considered NULL.
	TreatBlanks bool

	// TreatNA controls whether common "not available" markers are treated as NULL.
	// Matching is case-insensitive and currently includes:
	//   - "NA"
	//   - "N/A"
	TreatNA bool

	// TreatNULLLiteral controls whether the literal string "NULL" should be
	// treated as NULL. Matching is case-insensitive.
	TreatNULLLiteral bool
}

// IsNull reports whether the input string s should be treated as NULL under
// the current policy.
//
// The check is performed in the following order:
//
//  1. Trim surrounding whitespace.
//  2. If TreatBlanks is enabled and the trimmed value is empty, return true.
//  3. Convert the trimmed value to upper case.
//  4. If TreatNA is enabled and the value matches "NA" or "N/A", return true.
//  5. If TreatNULLLiteral is enabled and the value matches "NULL", return true.
//  6. Otherwise, return false.
//
// Important notes:
//
//   - Matching is case-insensitive.
//   - Surrounding whitespace is ignored for all checks.
//   - The function does not modify the input string.
//   - No attempt is made to detect numeric sentinels (e.g., "0", "-1").
//
// This function is intentionally conservative: only explicitly enabled markers
// are treated as NULL to avoid accidental data loss.
func (p Policy) IsNull(s string) bool {
	// Normalize whitespace before applying any rules.
	trimmed := strings.TrimSpace(s)

	// Empty or whitespace-only values.
	if p.TreatBlanks && trimmed == "" {
		return true
	}

	// Case-insensitive comparisons for sentinel values.
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
