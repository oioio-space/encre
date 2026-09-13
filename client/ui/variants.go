package ui

// variants are the accented forms reachable by holding a letter down.
//
// WHY THIS EXISTS: the always-visible row of ENCRE_02 §11 holds ten characters,
// and written French needs more than that. The brief's own word lists settle
// it — û appears in flûte, brûle, sûr and coût, œ in cœur and œil, all of them
// CE1 vocabulary, and none of them typeable from those ten keys. Holding the
// base letter is what every mobile keyboard does about the same problem, and
// what ENCRE_06 §5 proposes here rather than an eighth column that would put
// every key back under 48 pixels.
//
// The order matters: the most frequent form comes first, because it is the one
// under the finger when the row opens.
var variants = map[rune][]rune{
	'a': {'à', 'â'},
	'c': {'ç'},
	'e': {'é', 'è', 'ê', 'ë'},
	'i': {'î', 'ï'},
	'o': {'ô', 'ö', 'œ'},
	// No ü: ENCRE_02 §5 leaves it out of the chain the fonts must carry, and
	// French only needs it for aiguë and proper nouns — nothing a CE1 word list
	// will hold. Offering it would put an empty box under the child's finger.
	'u': {'ù', 'û'},
}

// Variants returns the accented forms of r, most frequent first, or nothing
// when the letter takes no accent. The result must not be modified.
func Variants(r rune) []rune { return variants[r] }
