// Command df is a tiny, CSV-first "dataframe-ish" CLI.
//
// The goal is to provide fast, dependable inspection and light transformation
// operations for tabular data commonly encountered in mailing/automation work.
//
// Current commands:
//   - cols: print header names
//   - head: show the first N rows (like pandas .head())
//   - nullify: normalize empty/placeholder values to NULL (empty fields in CSV)
//
// Design notes:
//
//   - Exit codes are conventional:
//     0 = success
//     1 = runtime error (I/O, parse, etc.)
//     2 = usage error (bad args/flags)
//
//   - Command handlers accept writers (stdout/stderr) to make behavior easy
//     to unit test without forking subprocesses.
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

// main delegates to run and exits with the returned process code.
// Keeping logic in run makes the CLI easier to test.
func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

// run is the top-level command dispatcher.
//
// argv is expected to look like os.Args (argv[0] is the program name).
// Output is written to out and errOut rather than using os.Stdout/os.Stderr
// directly, which keeps commands testable.
//
// Returns an exit code:
//
//	0 on success
//	1 on runtime error (I/O, parsing)
//	2 on usage error (bad args, missing required flags)
func run(argv []string, out, errOut io.Writer) int {
	// With no subcommand, print usage and return "usage error".
	if len(argv) < 2 {
		usage(errOut)
		return 2
	}

	// argv[1] is the subcommand (cols/head/nullify/etc).
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
		// For unknown commands, return usage error and show help.
		fmt.Fprintf(errOut, "unknown command: %q\n\n", argv[1])
		usage(errOut)
		return 2
	}
}

// usage prints help text. It intentionally writes to an io.Writer so callers
// can decide whether it belongs on stdout (help) or stderr (usage errors).
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

// runCols implements the "cols" subcommand.
//
// It reads only the header row and prints one header per line, prefixed with
// a zero-based column index for quick reference in spreadsheets and scripts.
func runCols(args []string, out, errOut io.Writer) int {
	// Each command uses its own FlagSet so parsing is isolated by subcommand.
	fs := flag.NewFlagSet("cols", flag.ContinueOnError)
	fs.SetOutput(errOut)

	// Parse command args; on parse error, treat as usage error.
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// cols requires exactly one positional argument: the input CSV path.
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

// runHead implements the "head" subcommand (similar to pandas DataFrame.head()).
//
// Users often expect to be able to place flags after positional arguments,
// e.g. "df head file.csv -n 5". The standard library flag package does not
// support that by default, so we reorder a small whitelist of flags.
func runHead(args []string, out, errOut io.Writer) int {
	// Allow: df head file.csv -n 5
	// (stdlib flag normally stops parsing flags once it sees a positional arg)
	args = reorderFlagsToFront(args, map[string]bool{
		"-n": true,
		"-w": true,
	})

	fs := flag.NewFlagSet("head", flag.ContinueOnError)
	fs.SetOutput(errOut)

	// -n controls how many rows are printed; -w caps printed cell width.
	n := fs.Int("n", 5, "Number of rows to display")
	maxWidth := fs.Int("w", 32, "Max width per cell when printing")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	// head requires exactly one positional argument: the input CSV path.
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

	// Print a simple fixed-width table suitable for terminal viewing and copy/paste.
	render.PrintTable(out, headers, rows, render.TableOptions{
		MaxCellWidth: *maxWidth,
		ShowRowIndex: true,
	})

	return 0
}

// runNullify implements the "nullify" subcommand.
//
// CSV does not have a true NULL value; in this tool, NULL is represented as an
// empty field on output.
//
// The null policy is configurable via flags. In addition to empty/whitespace-only
// values, callers may opt into treating NA/N/A or the literal string "NULL"
// (case-insensitive) as NULL.
func runNullify(args []string, out, errOut io.Writer) int {
	// out is reserved for future “preview” output; the command summary is written
	// to errOut so stdout can remain machine-readable if needed later.
	_ = out

	fs := flag.NewFlagSet("nullify", flag.ContinueOnError)
	fs.SetOutput(errOut)

	// -o is required; other flags control which sentinel values count as NULL.
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

	// Summary is written to stderr to keep stdout free for future "data output" modes.
	fmt.Fprintf(errOut, "Rows read: %d\n", stats.RowsRead)
	fmt.Fprintf(errOut, "Cells checked: %d\n", stats.CellsChecked)
	fmt.Fprintf(errOut, "Cells nullified (changed): %d\n", stats.CellsNullified)
	fmt.Fprintf(errOut, "Wrote: %s\n", *outPath)

	return 0
}

// reorderFlagsToFront moves a limited set of flags (defined by allowed) in front
// of positional arguments.
//
// This exists to support the common CLI expectation that users may place flags
// after the file argument:
//
//	df head file.csv -n 5
//
// The standard library flag package typically stops parsing flags once it sees the
// first non-flag argument. Rather than pulling in a full CLI framework, we reorder
// only the specific flags we support for this subcommand.
//
// Supported forms:
//   - "-n 5" / "-w 20"
//   - "-n=5" / "-w=20"
//
// Unknown flags are treated as positional arguments and left untouched; flag.Parse
// will error if such flags are actually intended as flags for the command.
func reorderFlagsToFront(args []string, allowed map[string]bool) []string {
	var flags []string
	var positionals []string

	i := 0
	for i < len(args) {
		a := args[i]

		// Handle "-n=5" style arguments.
		if eq := indexByte(a, '='); eq > 0 {
			name := a[:eq]
			if allowed[name] {
				flags = append(flags, a)
				i++
				continue
			}
		}

		// Handle "-n 5" style arguments.
		if allowed[a] {
			flags = append(flags, a)
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i += 2
				continue
			}
			// Missing value; allow flag.Parse to produce a helpful error.
			i++
			continue
		}

		// Anything else is treated as positional.
		positionals = append(positionals, a)
		i++
	}

	return append(flags, positionals...)
}

// indexByte is a tiny helper to avoid importing strings just for IndexByte.
// It returns the index of b in s, or -1 if not present.
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
