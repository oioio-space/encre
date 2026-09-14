# encre — state & roadmap

## Current state

Le jeu existe de bout en bout : un parent colle une liste, l'enfant la joue, le
serveur fait autorité, et l'écran de run se dessine. Ce qui manque est nommé plus
bas, pas caché.

- **`engine`** — Config, Score, PHat/Targets, transitions du mot, rang et XP,
  BuildDeck, `Draw` (tirage autour de p̂), Replay/Apply. Pur Go, zéro dépendance,
  score de mutation **100 %** (244 mutants, aucun survivant).
- **`lexique`** — nomme les pièges d'un mot et les **confirme sur la prononciation**
  de Lexique 3.83 : le « on » de *bonne* n'est pas la nasale de *pont*. **100 %**
  sur 211 mots de CE1 étiquetés à la main.
- **`content`** — les textes dits : 20 Talismans, 5 boss, 60 lignes de Phalène,
  22 exploits. Deux contradictions du brief tranchées et figées par des tests.
- **`sim`** — 100 enfants × 36 semaines sur le vrai moteur, avec les Talismans, la
  Revanche, la Rencontre et les conditions de rang. Rétention 100 %, M1 2,1 %,
  boss 24,3 %, **80 % des enfants au-dessus du rang Blanc**.
- **`server`** — `store` (schéma ENCRE_04 §6, WAL, migrations), `auth` (argon2id
  poivré, TOTP anti-rejeu atomique, sessions), `api` (les six endpoints de run,
  lecture enfant, dictée, tableau de bord), `parent` (connexion, enfants, réglages,
  la semaine, export et suppression), `gen` (phrases), `media` (ffmpeg sans shell).
- **`client`** — scènes, seize animations avec leurs courbes, trois vraies polices
  libres, clavier qui pardonne les frappes ratées, écran de run dessiné, hors ligne
  et reprise.
- **Déploiement** — Caddyfile, unités systemd, Litestream **chiffré et restauré pour
  de vrai**, service worker, `/admin/metrics`.

### Ce qui n'est pas fait, dit clairement

- **Les assets d'ENCRE_06 §9 n'existent pas** : 18 créatures, 8 icônes de Talismans,
  le shader d'encre vivante. La carte dessine un cadre nommé, pas une créature.
- **Aucune voix** : ni Piper pré-synthétisé, ni enregistrement parent joué en run.
- **`finish` n'enregistre pas les tentatives** (`encre-qpx.5`) — la table `attempts`
  reste vide, donc le serveur qui fait autorité ne garde pas ses pièces.
- **Les 85 % de Wilson ne sont pas atteints** (67 % de mots justes) et ne le seront
  pas par un réglage : voir `brief/ENCRE_07` §4 ter, qui le démontre par un balayage.
- Atelier, Bestiaire, fioles, musique en couches, retour haptique : non commencés.

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
| 2026-09-13 | Le brief a désormais son registre de corrections : `brief/ENCRE_07_corrections.md`, avec la preuve de chaque écart. |
| 2026-09-14 | **Le serveur et l'écran de run existent** : six endpoints, panneau parent complet, carte dessinée, hors ligne, déploiement. |
| 2026-09-14 | Un audit de sécurité adverse ferme trois failles hautes, dont une escalade de privilège sur les données d'enfants. |
| 2026-09-14 | Le moteur vit : quatre mécaniques de V1 ne s'exécutaient nulle part, 0 % → 80 % des enfants montent de rang. |
| 2026-09-14 | `brief/ENCRE_07_corrections.md` : le brief a son registre de corrections, chaque écart avec sa preuve. |
| 2026-09-13 | **T10 — paquet `lexique`** : détection des Couleurs confirmée sur la phonétique ; 100 % sur 211 mots CE1 ; Lexique 3.83 embarqué, attribué dans `LICENSES.md`. |
