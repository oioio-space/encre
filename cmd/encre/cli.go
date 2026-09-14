package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/oioio-space/encre/engine"
	"github.com/oioio-space/encre/lexique"
)

// usage is the short French help shown with no argument, -h, or a bad command.
const usage = `Usage : encre <commande> [options] [arguments]

Commandes :
  analyse <mot ou phrase...>   analyse les pièges orthographiques d'un mot ou d'une phrase
  version                      affiche la version d'encre

Options de « analyse » :
  -json   sortie au format JSON

Exemples :
  encre analyse chat
  encre analyse les chats dorment
`

// errWriter wraps an io.Writer and keeps the first error any write to it
// returns, so a long run of Fprint calls can be written without checking each
// one — the classic pattern from https://go.dev/blog/errors-are-values.
//
// A write attempted after the first error is a no-op: once the destination is
// broken (a closed pipe, say), retrying more writes to it would just report
// the same failure again.
type errWriter struct {
	w   io.Writer
	err error
}

// printf writes a formatted line, recording the error if the write fails.
func (ew *errWriter) printf(format string, a ...any) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintf(ew.w, format, a...)
}

// println writes a line, recording the error if the write fails.
func (ew *errWriter) println(a ...any) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintln(ew.w, a...)
}

// print writes s as-is, recording the error if the write fails.
func (ew *errWriter) print(s string) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprint(ew.w, s)
}

// run executes the CLI for args, writing normal output to stdout and errors
// to stderr, and returns the process exit code: 0 on success, 2 on misuse, 1
// when a write to stdout or stderr itself failed.
func run(args []string, stdout, stderr io.Writer) int {
	out := &errWriter{w: stdout}
	errOut := &errWriter{w: stderr}

	code := dispatch(args, out, errOut)

	if out.err != nil || errOut.err != nil {
		return 1
	}
	return code
}

// dispatch parses the command name and runs it, or reports the usage.
func dispatch(args []string, out, errOut *errWriter) int {
	if len(args) == 0 {
		errOut.print(usage)
		return 2
	}
	if args[0] == "-h" || args[0] == "--help" {
		out.print(usage)
		return 0
	}

	switch cmd, rest := args[0], args[1:]; cmd {
	case "analyse":
		return runAnalyse(rest, out, errOut)
	case "version":
		out.println(version())
		return 0
	default:
		errOut.printf("encre : commande inconnue %q\n\n", cmd)
		errOut.print(usage)
		return 2
	}
}

// runAnalyse implements the "analyse" subcommand: it names the spelling
// traps of a single word, or of every word in a sentence when more than one
// is given.
func runAnalyse(args []string, out, errOut *errWriter) int {
	fs := flag.NewFlagSet("analyse", flag.ContinueOnError)
	fs.SetOutput(errOut.w)
	asJSON := fs.Bool("json", false, "sortie au format JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	words := fs.Args()
	if len(words) == 0 {
		errOut.print("encre analyse : au moins un mot est requis\n\n")
		errOut.print(usage)
		return 2
	}

	lex := lexique.Embedded()
	var analyses []lexique.Analysis
	if len(words) == 1 {
		analyses = []lexique.Analysis{lex.Analyze(words[0])}
	} else {
		analyses = lex.AnalyzeSentence(strings.Join(words, " "), nil)
	}

	reports := make([]wordReport, len(analyses))
	for i, a := range analyses {
		reports[i] = newWordReport(a)
	}

	if *asJSON {
		enc := json.NewEncoder(out.w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(reports); err != nil {
			errOut.printf("encre analyse : %v\n", err)
			return 1
		}
		return 0
	}

	for _, r := range reports {
		r.writeText(out)
	}
	return 0
}

// version reports the tool's version, taken from the build info the Go
// toolchain embeds — the module has no version ldflags to reuse (see
// .goreleaser.yaml).
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "encre (version inconnue, build local)"
	}
	return "encre " + info.Main.Version
}

// wordReport is one analysed word, shaped for both the text and the JSON
// output of "analyse".
type wordReport struct {
	// Word is the form that was analysed, exactly as given on the command line.
	Word string `json:"word"`
	// Colors counts the traps per Couleur, keyed by its French name.
	Colors map[string]int `json:"colors"`
	// Rules lists the fine rules found, in the order they are written.
	Rules []string `json:"rules"`
	// Positions lists the rune index of every trap letter.
	Positions []int `json:"positions"`
	// Family is a word of the same family that justifies a silent letter.
	Family string `json:"family,omitzero"`
	// Confidence is how sure the analysis is, between 0 and 1.
	Confidence float64 `json:"confidence"`
	// Known says whether the lexicon had the word.
	Known bool `json:"known"`
	// Unsure says whether the parent should confirm this analysis.
	Unsure bool `json:"unsure"`
}

// newWordReport shapes an Analysis for display.
func newWordReport(a lexique.Analysis) wordReport {
	rules := make([]string, 0, len(a.Rules()))
	for _, r := range a.Rules() {
		rules = append(rules, string(r))
	}

	colors := make(map[string]int, len(a.Traps))
	for c, n := range a.Traps {
		colors[c.String()] = n
	}

	return wordReport{
		Word:       a.Word,
		Colors:     colors,
		Rules:      rules,
		Positions:  a.Positions(),
		Family:     a.Family,
		Confidence: a.Confidence,
		Known:      a.Known,
		Unsure:     a.Unsure(),
	}
}

// writeText writes the word's report in the human-readable form "analyse" shows
// by default.
func (r wordReport) writeText(out *errWriter) {
	out.println(r.Word)
	if r.Unsure {
		out.println("  ⚠ à vérifier")
	}
	for _, c := range engine.Colors() {
		n := r.Colors[c.String()]
		if n == 0 {
			continue
		}
		out.printf("  %-10s: %d piège(s)\n", c.String(), n)
	}
	if len(r.Rules) > 0 {
		out.println("  règles    :", strings.Join(r.Rules, ", "))
	}
	if len(r.Positions) > 0 {
		out.println("  lettres   :", joinInts(r.Positions))
	}
	if r.Family != "" {
		out.println("  famille   :", r.Family)
	}
	out.printf("  confiance : %.2f\n", r.Confidence)
}

// joinInts formats positions as a comma-separated list, e.g. "0, 1, 3".
func joinInts(ints []int) string {
	parts := make([]string, len(ints))
	for i, n := range ints {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ", ")
}
