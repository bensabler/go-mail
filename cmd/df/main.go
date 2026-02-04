package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/bensabler/go-mail/internal/csvio"
	"github.com/bensabler/go-mail/internal/nulls"
	"github.com/bensabler/go-mail/internal/render"
)

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(argv []string, out, errOut io.Writer) int {
	if len(argv) < 2 {
		usage(errOut)
		return 2
	}

	switch argv[1] {
	case "cols":
		return runCols(argv[2:], out, errOut)
	case "head":
		return runHead(argv[2:], out, errOut)
	case "nullify":
		return runNullify(argv[2:], out, errOut)
	case "-h", "--help", "help":
		usage(out)
		return 0
	default:
		fmt.Fprintf(errOut, "unknown command: %q\n\n", argv[1])
		usage(errOut)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `df - a tiny CSV-first dataframe-ish CLI

Usage:
  df <command> [args]

Commands:
  cols <file.csv>                         Print column headers
  head <file.csv> [-n N]                  Print the first N rows (default 5)
  nullify <file.csv> -o out.csv [flags]   Convert empty/NA/NULL markers to NULL

Examples:
  df cols input.csv
  df head input.csv -n 10
  df head -n 5 input.csv
  df head input.csv -n 5
  df nullify input.csv -o cleaned.csv --blanks --na --null-literal
`)
}

func runCols(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("cols", flag.ContinueOnError)
	fs.SetOutput(errOut)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(errOut, "cols requires exactly one argument: <file.csv>")
		return 2
	}

	path := fs.Arg(0)
	headers, err := csvio.ReadHeaders(path)
	if err != nil {
		fmt.Fprintln(errOut, "error:", err)
		return 1
	}

	for i, h := range headers {
		fmt.Fprintf(out, "%d\t%s\n", i, h)
	}

	return 0
}

func runHead(args []string, out, errOut io.Writer) int {
	// Allow: df head file.csv -n 5 (stdlib flag normally rejects flags after positionals)
	args = reorderFlagsToFront(args, map[string]bool{
		"-n": true,
		"-w": true,
	})

	fs := flag.NewFlagSet("head", flag.ContinueOnError)
	fs.SetOutput(errOut)

	n := fs.Int("n", 5, "Number of rows to display")
	maxWidth := fs.Int("w", 32, "Max width per cell when printing")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(errOut, "head requires exactly one argument: <file.csv>")
		return 2
	}
	if *n < 0 {
		fmt.Fprintln(errOut, "-n must be >= 0")
		return 2
	}

	path := fs.Arg(0)
	headers, rows, err := csvio.ReadHead(path, *n)
	if err != nil {
		fmt.Fprintln(errOut, "error:", err)
		return 1
	}

	render.PrintTable(out, headers, rows, render.TableOptions{
		MaxCellWidth: *maxWidth,
		ShowRowIndex: true,
	})

	return 0
}

func runNullify(args []string, out, errOut io.Writer) int {
	_ = out // reserved for future “preview” mode; summary goes to stderr for now

	fs := flag.NewFlagSet("nullify", flag.ContinueOnError)
	fs.SetOutput(errOut)

	outPath := fs.String("o", "", "Output CSV path (required)")
	blanks := fs.Bool("blanks", true, "Treat empty/whitespace-only cells as NULL")
	na := fs.Bool("na", false, "Treat NA and N/A as NULL (case-insensitive)")
	nullLiteral := fs.Bool("null-literal", false, "Treat NULL as NULL (case-insensitive)")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(errOut, "nullify requires exactly one argument: <file.csv>")
		return 2
	}
	if *outPath == "" {
		fmt.Fprintln(errOut, "nullify requires -o <output.csv>")
		return 2
	}

	inPath := fs.Arg(0)

	stats, err := csvio.NullifyFile(inPath, *outPath, nulls.Policy{
		TreatBlanks:      *blanks,
		TreatNA:          *na,
		TreatNULLLiteral: *nullLiteral,
	})
	if err != nil {
		fmt.Fprintln(errOut, "error:", err)
		return 1
	}

	fmt.Fprintf(errOut, "Rows read: %d\n", stats.RowsRead)
	fmt.Fprintf(errOut, "Cells checked: %d\n", stats.CellsChecked)
	fmt.Fprintf(errOut, "Cells nullified (changed): %d\n", stats.CellsNullified)
	fmt.Fprintf(errOut, "Wrote: %s\n", *outPath)

	return 0
}

// reorderFlagsToFront lets users put flags after the file argument, e.g.
// df head file.csv -n 5
//
// Only supports the flags we whitelist in allowed.
// Supports "-n=5" and "-w=20" too.
func reorderFlagsToFront(args []string, allowed map[string]bool) []string {
	var flags []string
	var positionals []string

	i := 0
	for i < len(args) {
		a := args[i]

		// -n=5 form
		if eq := indexByte(a, '='); eq > 0 {
			name := a[:eq]
			if allowed[name] {
				flags = append(flags, a)
				i++
				continue
			}
		}

		// -n 5 form
		if allowed[a] {
			flags = append(flags, a)
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i += 2
				continue
			}
			// missing value; let flag.Parse complain
			i++
			continue
		}

		positionals = append(positionals, a)
		i++
	}

	return append(flags, positionals...)
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
