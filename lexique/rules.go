// Package lexique reads a French word and names the traps it carries: the
// Couleurs of ENCRE_03 §4 and the fine rules of ENCRE_03 §2.
//
// Detection follows the heuristics of ENCRE_03 §10. A spelling pattern alone
// is not enough — "on" in bonne is not the nasal of pont — so every Masquée is
// confirmed against the word's pronunciation, taken from the embedded Lexique
// 3.83 database. A word the lexicon does not know is analysed on its patterns
// alone and comes back with a confidence below UnsureConfidence, which is what
// marks it « à vérifier » for the parent.
package lexique

import (
	"regexp"

	"github.com/oioio-space/encre/engine"
)

// The fine rules, named as ENCRE_03 §2 names them. The value is the id stored
// in the database and sent to the client; the justification said aloud lives in
// the content package, keyed by the same id.
const (
	// Masquées — a sound written several ways (ENCRE_03 §2).
	RuleOU      engine.Rule = "ou"
	RuleOI      engine.Rule = "oi"
	RuleON      engine.Rule = "on_om"
	RuleAN      engine.Rule = "an_en_am_em"
	RuleIN      engine.Rule = "in_ain_ein"
	RuleCH      engine.Rule = "ch"
	RuleGN      engine.Rule = "gn"
	RulePH      engine.Rule = "ph"
	RuleEAU     engine.Rule = "eau_au"
	RuleAI      engine.Rule = "ai_ei"
	RuleILL     engine.Rule = "ill_il"
	RuleQU      engine.Rule = "qu"
	RuleGU      engine.Rule = "gu"
	RuleGE      engine.Rule = "ge_gi"
	RuleEU      engine.Rule = "eu_oeu"
	RuleUN      engine.Rule = "un_um"
	RuleMdevant engine.Rule = "m_devant_mbp"
	RuleSZ      engine.Rule = "s_entre_voyelles"
	RuleSS      engine.Rule = "ss_c_c_cedille"

	// Muettes — a final letter written and not heard.
	RuleTMuet  engine.Rule = "t_muet"
	RuleDMuet  engine.Rule = "d_muet"
	RuleSMuet  engine.Rule = "s_muet"
	RuleEMuet  engine.Rule = "e_muet"
	RuleXMuet  engine.Rule = "x_muet"
	RulePCMuet engine.Rule = "p_c_muet"
	// RuleLettreMuette catches the other silent finals ENCRE_03 §2 does not
	// table — the r of chanter, the z of nez, the g of long. The Couleur is the
	// contract; the fine rules are allowed to grow under it, and a CE1 dictation
	// is full of infinitives.
	RuleLettreMuette engine.Rule = "lettre_muette"

	// Jumelles — a doubled consonant.
	RuleMM  engine.Rule = "mm"
	RuleNN  engine.Rule = "nn"
	RuleLL  engine.Rule = "ll"
	RuleTT  engine.Rule = "tt"
	RuleRR  engine.Rule = "rr"
	RulePP  engine.Rule = "pp"
	RuleSSJ engine.Rule = "ss_double"
	RuleCC  engine.Rule = "cc"
	RuleFF  engine.Rule = "ff"
	RuleBB  engine.Rule = "bb"
	RuleDD  engine.Rule = "dd"
	RuleGG  engine.Rule = "gg"

	// Accentuées — the marks above the letter.
	RuleEAigu       engine.Rule = "e_aigu"
	RuleEGrave      engine.Rule = "e_grave"
	RuleCirconflexe engine.Rule = "circonflexe"
	RuleAGrave      engine.Rule = "a_grave"
	RuleUGrave      engine.Rule = "u_grave"
	RuleTrema       engine.Rule = "trema"
	RuleCedille     engine.Rule = "c_cedille"

	// Accordées — agreement, and so only ever found in a sentence.
	RulePlurielS engine.Rule = "pluriel_s"
	RulePlurielX engine.Rule = "pluriel_x"
	RuleFemininE engine.Rule = "feminin_e"
	RuleVerbeEnt engine.Rule = "verbe_ent"

	// Sosies — the closed list of ENCRE_03 §2, off until the parent turns it on.
	RuleAA      engine.Rule = "a_a_accent"
	RuleEtEst   engine.Rule = "et_est"
	RuleOnOnt   engine.Rule = "on_ont"
	RuleSonSont engine.Rule = "son_sont"
)

// pattern is one rule looked for in the spelling and confirmed in the sound.
//
// The confirmation is what separates a trap from a coincidence: bonne and pont
// both spell "on", but only pont says it. A pattern with no sounds listed is
// confirmed by the spelling alone, which is all a doubled consonant needs.
//
// A regexp with no capture group is all trap. A group means the rest of the
// match is only context: the g of rouge is soft because of the e after it, but
// only the g is the trap — the e is still the silent letter the child forgets.
type pattern struct {
	rule   engine.Rule
	color  engine.Color
	re     *regexp.Regexp
	sounds []string
}

// masquees lists the sound-spelling rules of ENCRE_02 §2, together with the eu
// and un of ENCRE_03 §1 that its table leaves out. Order matters inside an
// alternation: the longest spelling comes first, so that "ille" wins over "ill"
// and "ain" over "in".
var masquees = []pattern{
	{RuleILL, engine.Masquees, regexp.MustCompile(`ille|ill|il`), []string{"j"}},
	{RuleEAU, engine.Masquees, regexp.MustCompile(`eau|au`), []string{"o", "O"}},
	{RuleIN, engine.Masquees, regexp.MustCompile(`ain|aim|ein|ien|yen|in|im|ym|yn`), []string{"5"}},
	{RuleAN, engine.Masquees, regexp.MustCompile(`an|en|am|em`), []string{"@"}},
	{RuleON, engine.Masquees, regexp.MustCompile(`on|om`), []string{"§"}},
	{RuleOU, engine.Masquees, regexp.MustCompile(`ou`), []string{"u", "w"}},
	{RuleOI, engine.Masquees, regexp.MustCompile(`oi|oî|oy`), []string{"w"}},
	{RuleEU, engine.Masquees, regexp.MustCompile(`œu|eu`), []string{"2", "9"}},
	{RuleUN, engine.Masquees, regexp.MustCompile(`un|um`), []string{"1"}},
	{RuleCH, engine.Masquees, regexp.MustCompile(`ch`), []string{"S"}},
	{RuleGN, engine.Masquees, regexp.MustCompile(`gn`), []string{"N"}},
	{RulePH, engine.Masquees, regexp.MustCompile(`ph`), []string{"f"}},
	{RuleAI, engine.Masquees, regexp.MustCompile(`ai|aî|ei|ay`), []string{"E", "e"}},
	{RuleQU, engine.Masquees, regexp.MustCompile(`qu|q`), []string{"k"}},
	{RuleGU, engine.Masquees, regexp.MustCompile(`(gu)[eéèêi]`), []string{"g"}},
	{RuleGE, engine.Masquees, regexp.MustCompile(`(g)[eéèêiy]`), []string{"Z"}},
	{RuleMdevant, engine.Masquees, regexp.MustCompile(`[aeiouy](m)[mbp]`), []string{"@", "§", "5", "1"}},
	{RuleSZ, engine.Masquees, regexp.MustCompile(`[aeiouyéèêëàâôöûùî](s)[aeiouyéèêëàâôöûùî]`), []string{"z"}},
	{RuleSS, engine.Masquees, regexp.MustCompile(`ss`), []string{"s"}},
}

// jumelles are the doubled consonants of ENCRE_03 §2, matched on the spelling
// alone — a double letter is visible, which is the whole point of the Couleur.
var jumelles = []pattern{
	{RuleMM, engine.Jumelles, regexp.MustCompile(`mm`), nil},
	{RuleNN, engine.Jumelles, regexp.MustCompile(`nn`), nil},
	{RuleLL, engine.Jumelles, regexp.MustCompile(`ll`), nil},
	{RuleTT, engine.Jumelles, regexp.MustCompile(`tt`), nil},
	{RuleRR, engine.Jumelles, regexp.MustCompile(`rr`), nil},
	{RulePP, engine.Jumelles, regexp.MustCompile(`pp`), nil},
	{RuleSSJ, engine.Jumelles, regexp.MustCompile(`ss`), nil},
	{RuleCC, engine.Jumelles, regexp.MustCompile(`cc`), nil},
	{RuleFF, engine.Jumelles, regexp.MustCompile(`ff`), nil},
	{RuleBB, engine.Jumelles, regexp.MustCompile(`bb`), nil},
	{RuleDD, engine.Jumelles, regexp.MustCompile(`dd`), nil},
	{RuleGG, engine.Jumelles, regexp.MustCompile(`gg`), nil},
}

// accents maps a written mark to its rule. ENCRE_03 §2 tables é, è, ê, à and ç;
// the circumflex teaches the same "petit chapeau" on any vowel, and the tréma
// and the ù of « où » round out what a CE1 list can actually contain.
var accents = map[rune]engine.Rule{
	'é': RuleEAigu,
	'è': RuleEGrave,
	'ê': RuleCirconflexe, 'â': RuleCirconflexe, 'î': RuleCirconflexe,
	'ô': RuleCirconflexe, 'û': RuleCirconflexe,
	'à': RuleAGrave,
	'ù': RuleUGrave,
	'ë': RuleTrema, 'ï': RuleTrema, 'ü': RuleTrema,
	'ç': RuleCedille,
}

// sosies is the closed list of ENCRE_03 §2: words that sound alike and are only
// told apart by what the sentence means.
var sosies = map[string]engine.Rule{
	"a": RuleAA, "à": RuleAA,
	"et": RuleEtEst, "est": RuleEtEst,
	"on": RuleOnOnt, "ont": RuleOnOnt,
	"son": RuleSonSont, "sont": RuleSonSont,
}
