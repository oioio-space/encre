package parent

// Rule is one fine spelling rule the parent can switch on or off per child
// (ENCRE_03 §2). ID is what [Child.Settings] stores it under; Label is what
// the panel shows.
type Rule struct {
	ID    string
	Label string
}

// RuleGroup is one Couleur's set of fine rules, shown together on the
// réglages page.
type RuleGroup struct {
	// Category names the group, matching ENCRE_03 §2's section headers.
	Category string
	Rules    []Rule
	// DefaultOn says whether a rule in this group starts enabled for a
	// newly created child. Only Sosies starts disabled (ENCRE_03 §1: "les
	// Sosies sont désactivés par défaut, jusqu'à ce que la maîtresse les
	// aborde").
	DefaultOn bool
}

// ruleCatalog is every fine rule ENCRE_03 §2 lists, grouped by Couleur.
// [defaultRuleSettings] derives a new child's starting settings from it, and
// the réglages page renders one checkbox per entry.
var ruleCatalog = []RuleGroup{
	{
		Category: "Masquées (sons complexes)",
		Rules: []Rule{
			{ID: "masquees.ou", Label: "ou"},
			{ID: "masquees.oi", Label: "oi"},
			{ID: "masquees.on", Label: "on / om"},
			{ID: "masquees.an", Label: "an / en / am / em"},
			{ID: "masquees.in", Label: "in / ain / ein"},
			{ID: "masquees.ch", Label: "ch"},
			{ID: "masquees.gn", Label: "gn"},
			{ID: "masquees.ph", Label: "ph"},
			{ID: "masquees.eau", Label: "eau / au"},
			{ID: "masquees.ai", Label: "ai / ei"},
			{ID: "masquees.ill", Label: "ill / il"},
			{ID: "masquees.qu", Label: "qu"},
			{ID: "masquees.gu", Label: "gu"},
			{ID: "masquees.c_cedille", Label: "ç"},
			{ID: "masquees.ge_gi", Label: "ge / gi"},
			{ID: "masquees.m_mbp", Label: "m devant m, b, p"},
			{ID: "masquees.s_z", Label: "s entre voyelles = z"},
			{ID: "masquees.ss_s", Label: "ss / c / ç = s"},
		},
		DefaultOn: true,
	},
	{
		Category: "Muettes (lettres finales muettes)",
		Rules: []Rule{
			{ID: "muettes.t", Label: "t muet"},
			{ID: "muettes.d", Label: "d muet"},
			{ID: "muettes.s", Label: "s muet"},
			{ID: "muettes.e", Label: "e muet final"},
			{ID: "muettes.x", Label: "x muet"},
			{ID: "muettes.pc", Label: "p / c muet"},
		},
		DefaultOn: true,
	},
	{
		Category: "Jumelles (consonnes doubles)",
		Rules: []Rule{
			{ID: "jumelles.mm", Label: "mm"},
			{ID: "jumelles.nn", Label: "nn"},
			{ID: "jumelles.ll", Label: "ll"},
			{ID: "jumelles.tt", Label: "tt"},
			{ID: "jumelles.rr", Label: "rr"},
			{ID: "jumelles.pp", Label: "pp"},
			{ID: "jumelles.ss", Label: "ss"},
		},
		DefaultOn: true,
	},
	{
		Category: "Accentuées",
		Rules: []Rule{
			{ID: "accentuees.e_aigu", Label: "é"},
			{ID: "accentuees.e_grave", Label: "è"},
			{ID: "accentuees.e_circonflexe", Label: "ê"},
			{ID: "accentuees.a_grave", Label: "à"},
			{ID: "accentuees.c_cedille", Label: "ç"},
		},
		DefaultOn: true,
	},
	{
		Category: "Accordées (uniquement en phrase)",
		Rules: []Rule{
			{ID: "accordees.s_pluriel", Label: "-s du pluriel"},
			{ID: "accordees.x_pluriel", Label: "-x du pluriel"},
			{ID: "accordees.e_feminin", Label: "-e du féminin"},
			{ID: "accordees.ent_verbe", Label: "-ent du verbe"},
		},
		DefaultOn: true,
	},
	{
		Category: "Sosies (activables par le parent)",
		Rules: []Rule{
			{ID: "sosies.a_a", Label: "a / à"},
			{ID: "sosies.et_est", Label: "et / est"},
			{ID: "sosies.on_ont", Label: "on / ont"},
			{ID: "sosies.son_sont", Label: "son / sont"},
		},
		DefaultOn: false,
	},
}

// defaultRuleSettings returns every ruleCatalog rule's starting state for a
// newly created child, keyed by [Rule.ID]. Every rule starts on except the
// Sosies group, which ENCRE_03 §1 requires to start off.
func defaultRuleSettings() map[string]bool {
	settings := make(map[string]bool)
	for _, group := range ruleCatalog {
		for _, rule := range group.Rules {
			settings[rule.ID] = group.DefaultOn
		}
	}
	return settings
}
