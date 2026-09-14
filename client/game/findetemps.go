package game

import "github.com/oioio-space/encre/content"

// FinDeTempsLine returns one of the warm lines Phalène says when the day's
// play time runs out (content/data/phalene.json, context "fin_de_temps"):
// "La bougie s'éteint. À demain." — never "temps écoulé", never a failure.
// The run itself is not lost either: whatever RunSession.RecordAttempt saved
// is still there tomorrow, exactly where the child left it.
//
// i selects which of the lines is shown, so a caller can rotate through them
// across visits rather than always saying the same one; it wraps, so any i is
// safe to pass. It returns "" if the pack carries no fin_de_temps line at
// all — a content gap, not something FinDeTemps should ever crash over.
func FinDeTempsLine(pack *content.Pack, i int) string {
	lines := pack.Lines("fin_de_temps")
	if len(lines) == 0 {
		return ""
	}
	return lines[i%len(lines)]
}
