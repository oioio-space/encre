# encre — state & roadmap

## Current state

- v0.0.0 — projet généré depuis go-starter le 2026-09-13 ; squelette + gates verts.
- Brief de conception dans `brief/` : règles du jeu, charte graphique, contenu
  pédagogique, spec technique, backlog, design.
- **Prototype T00**, selon ENCRE_06 : thème clair parchemin, 7 colonnes
  alphabétiques sur téléphone (AZERTY 10 sur ordinateur), rangée d'accents,
  ourlet de touche, appui long. Vérifié en WASM — « cœur » écrit, le `œ` par
  appui long sur `o`. `client/ui` et `client/game` sont testés ; le reste de T00
  est la séance sur tablette et téléphone réels (`encre-1yv.1`).
- **Paquet `engine` complet** : Config, Score, PHat/Targets, transitions du mot,
  rang et XP, BuildDeck, Replay/Apply. Pur Go, zéro dépendance, 95,5 % couvert.
- **`sim`** rejoue 100 enfants sur 36 semaines à travers le vrai moteur et tient
  les seuils du brief : rétention 96 %, M1 3,3 %, boss 23,5 %, en 2,8 s.
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
| 2026-09-13 | T00 refait selon ENCRE_06 : thème clair, 7 colonnes alphabétiques, appui long pour `œ û ë â ï ö`. |
| 2026-09-13 | **Epic Moteur terminé** : `engine` complet (T01–T08) + `sim` — 100 enfants × 36 semaines sur le vrai moteur. |
| 2026-09-13 | **Epic Étape 0 levé** sur mesures émulées (décision de Mathieu) : 48 px → 165 px phys., latence 32 ms, 5,56 s en 4G. |
| 2026-09-13 | Disposition balayée sur 16 formats à chaque build ; relief des touches selon ENCRE_02 §4 et ENCRE_06 §4. |
| 2026-09-13 | README réel (ancré sur la simulation d'équilibrage), topics GitHub, licence MIT. |
