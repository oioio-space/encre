# encre — state & roadmap

## Current state

- v0.0.0 — projet généré depuis go-starter le 2026-09-13 ; squelette + gates verts.
- Brief de conception dans `brief/` : règles du jeu, charte graphique, contenu
  pédagogique, spec technique, backlog, design.
- **Prototype T00** : clavier AZERTY dessiné avec sa rangée d'accents, carte,
  mot qui s'écrit, son de plume. Vérifié dans un vrai navigateur en WASM —
  « école » tapé à la touche, `é` compris, contexte audio débloqué au premier
  appui. `client/ui` (géométrie, glyphes, échelle) et `client/game` (saisie)
  sont testés ; le reste de T00 est la séance sur tablette et téléphone réels.
- Builds natif **et** `GOOS=js GOARCH=wasm` verts, sans CGO. WASM : 18 Mo bruts,
  **3,2 Mo brotli** — le brief budgète ~10 Mo bruts / ~3 Mo brotli, donc le
  double sur le brut et la cible sur ce qui traverse vraiment le réseau.

## Roadmap

Les tâches vivent dans **bd**, pas ici : `bd ready` donne le travail débloqué,
`bd list -t epic` la carte, `bd status` les comptes. Ce fichier garde le récit.

L'ordre est celui du brief, encodé dans les priorités des epics : **Étape 0**
(le prototype clavier WASM sur tablette et téléphone réels, `encre-gol`) et le
**moteur** en P0 ; contenu, serveur et client en P1 ; panneau parent, mise en
production et le socle du dépôt en P2. Le dépôt ne prime pas sur le jeu.

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
| 2026-09-13 | Prototype T00 : clavier dessiné, accents, audio ; `client/ui` + `client/game` testés (35 tests, 96 %). |
| 2026-09-13 | README réel (ancré sur la simulation d'équilibrage), topics GitHub, licence MIT. |
