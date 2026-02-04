package main

import (
	"bytes"
	"strings"
	"testing"
)

const test_mail_data = "../../data/test_mail_data.csv"

func TestHead_FlagBeforeFile_Works(t *testing.T) {
	var out, errOut bytes.Buffer
	path := test_mail_data
	code := run([]string{"df", "head", "-n", "5", path}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%s", code, errOut.String())
	}

	// Table prints: header line + separator + N data rows => 2 + N lines
	lines := nonEmptyLines(out.String())
	want := 2 + 5
	if len(lines) != want {
		t.Fatalf("expected %d output lines, got %d\nOUTPUT:\n%s", want, len(lines), out.String())
	}
}

func TestHead_FileBeforeFlag_Works(t *testing.T) {
	var out, errOut bytes.Buffer
	path := test_mail_data

	code := run([]string{"df", "head", path, "-n", "5"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%s", code, errOut.String())
	}

	lines := nonEmptyLines(out.String())
	want := 2 + 5
	if len(lines) != want {
		t.Fatalf("expected %d output lines, got %d\nOUTPUT:\n%s", want, len(lines), out.String())
	}
}

func TestHead_InvalidN(t *testing.T) {
	var out, errOut bytes.Buffer
	path := test_mail_data

	code := run([]string{"df", "head", "-n", "-1", path}, &out, &errOut)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "-n must be >=") {
		t.Fatalf("expected -n validation message; stderr=%s", errOut.String())
	}
}

func nonEmptyLines(s string) []string {
	raw := strings.Split(s, "\n")
	out := make([]string, 0, len(raw))
	for _, ln := range raw {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		out = append(out, ln)
	}
	return out
}
