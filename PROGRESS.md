# encre — state & roadmap

## Current state

- v0.0.0 — projet généré depuis go-starter le 2026-09-13 ; squelette + gates verts.
- Brief de conception dans `brief/` : règles du jeu, charte graphique, contenu
  pédagogique, spec technique, backlog, design.
- `cmd/client` ouvre une fenêtre Ebitengine v2.10.2 (résolution logique
  portrait/paysage de ENCRE_04 §2). Builds natif **et** `GOOS=js GOARCH=wasm`
  verts, sans CGO.

## Roadmap

- [ ] **Étape 0** (ENCRE_04 §2) — prototype clavier dessiné en WASM, dix mots
      accentués, sur la tablette *et* le téléphone réels : valide la taille des
      touches, la latence audio et le chargement avant tout le reste.
- [ ] Porter `brief/simulation_dictee.go` dans le paquet `engine` (ENCRE_04 §3-4) :
      pur Go, zéro dépendance, 100 % testé.
- [ ] Remplacer le placeholder `Greet` / `cmd/encre` par la structure de la spec
      (`cmd/client`, `cmd/server`, `engine/`, `lexique/`, `content/`).
- [ ] Remplir la section Architecture de CLAUDE.md, dont la ligne `HOT_PATHS:`
      une fois la boucle de jeu écrite.
- [ ] Écrire un vrai README (le gabarit du kit est encore en place).
- La suite vit dans **bd** : `bd ready` donne le travail débloqué, `bd list -t epic` la
      carte. `PROGRESS.md` garde le récit et le journal, plus les tâches.

## Log

| Date | Change |
|------|--------|
| 2026-09-13 | Bootstrap depuis go-starter (mise, hooks, agents, skills, CI). |
| 2026-09-13 | Brief de conception ajouté ; `simulation_dictee.go` sorti du build (`//go:build ignore`). |
| 2026-09-13 | Ebitengine v2.10.2 + squelette `cmd/client` ; natif et js/wasm verts, pure Go. |
| 2026-09-13 | Go 1.27.1 ; golangci-lint 2.13.2 (première build avec go1.27) ; plus aucune CVE stdlib. |
| 2026-09-13 | beads (bd) intégré ; 8 epics / 37 tâches depuis ENCRE_05 ; gate `Bead:` sur chaque commit. |
| 2026-09-13 | `check-gates-intact` (53 garde-fous) et `cycle-check` (7 signaux) portés de fk, avec leurs tests de morsure. |
| 2026-09-13 | Remote GitHub public `oioio-space/encre` ; CI verte au premier push ; beads synchronisés (`refs/dolt/data`). |
