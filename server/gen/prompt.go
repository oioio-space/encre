package gen

import (
	"strings"

	"github.com/oioio-space/encre/engine"
)

// promptTemplate is brief/ENCRE_03_contenu_pedagogique.md §9's prompt,
// reproduced byte for byte up to (not including) "{{liste avec Couleurs}}"
// — see TestPromptMatchesENCRE03Exactly, which reads that file and checks
// this constant plus [wordsLine]'s output against it directly, so the two
// can never quietly drift apart.
//
// [BuildPrompt] appends [wordsLine]'s output where "{{liste avec
// Couleurs}}" stood; nothing else in the template is ever substituted.
const promptTemplate = `Tu écris des phrases pour un enfant de CE1 (7 ans) qui apprend l'orthographe.
Pour chaque mot donné, propose 3 phrases à trou.

Contraintes strictes :
- 5 à 8 mots par phrase, vocabulaire du quotidien d'un enfant de 7 ans
- le mot cible apparaît une fois, tel quel ou accordé si la liste le demande
- aucun autre mot rare ou difficile à lire
- pas de nom propre, pas de négation complexe, pas de subjonctif
- phrase affirmative, sujet simple, présent ou passé composé
- si la Couleur "Accordées" est demandée, le contexte doit imposer l'accord
  (déterminant pluriel visible, sujet féminin visible, "ils/elles" visible)
- aucune orthographe fautive nulle part

Réponds UNIQUEMENT en JSON :
{"mot": "...", "phrases": [{"texte": "Le ___ dort sur le lit.", "cible": "chat", "forme": "chat"}, ...]}

Mots : `

// WordRequest is one word to ask sentences for: its text and the Couleurs
// its traps belong to, the way ENCRE_03 §9's "liste avec Couleurs" names
// them.
type WordRequest struct {
	Mot      string
	Couleurs []engine.Color
}

// BuildPrompt fills [promptTemplate] for a single word.
//
// ENCRE_04 §9 asks for exactly three phrases per word ("3 phrases par
// mot"), and the JSON shape the template itself demands — one "mot" key,
// not an array — only has room for one: this package therefore calls the
// API once per word rather than batching a list into one request, however
// plural "Mots :" reads on its own. The alternative (change the response
// shape to an array and diverge from the prompt ENCRE_03 §9 spells out
// verbatim) was rejected precisely because that text must never be
// paraphrased.
func BuildPrompt(word WordRequest) string {
	return promptTemplate + wordsLine(word) + "\n"
}

// wordsLine formats one WordRequest as "{{liste avec Couleurs}}" would show
// it for a single word: "mot (Couleur1, Couleur2)", or bare "mot" when it
// carries no Couleur.
func wordsLine(word WordRequest) string {
	if len(word.Couleurs) == 0 {
		return word.Mot
	}
	names := make([]string, len(word.Couleurs))
	for i, c := range word.Couleurs {
		names[i] = c.String()
	}
	return word.Mot + " (" + strings.Join(names, ", ") + ")"
}
