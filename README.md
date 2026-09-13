# ENCRE

> Un jeu d'orthographe pour un enfant de 7 ans (CE1), sur la boucle de Balatro,
> dans un atelier de scribe la nuit.

[![ci](https://github.com/oioio-space/encre/actions/workflows/ci.yml/badge.svg)](https://github.com/oioio-space/encre/actions/workflows/ci.yml)
![go](https://img.shields.io/badge/go-1.27-00ADD8?logo=go&logoColor=white)
![ebitengine](https://img.shields.io/badge/ebitengine-v2.10-d97757)
![cgo](https://img.shields.io/badge/cgo-interdit-4c1)

**Le dépôt en est au tout début.** Le squelette du client Ebitengine ouvre une
fenêtre ; le jeu lui-même est spécifié, chiffré et découpé en 33 tickets, mais
pas encore écrit. Le détail de ce qui existe est [plus bas](#ce-qui-tourne-vraiment).

## Sommaire

- [Pourquoi le jeu a cette forme](#pourquoi-le-jeu-a-cette-forme)
- [Ce qui tourne vraiment](#ce-qui-tourne-vraiment)
- [Démarrer](#démarrer)
- [Comment c'est construit](#comment-cest-construit)
- [Le suivi du travail](#le-suivi-du-travail)
- [Le brief](#le-brief)
- [Licence](#licence)

## Pourquoi le jeu a cette forme

Avant d'écrire une ligne du jeu, le concept a été passé dans un simulateur :
100 enfants, 36 semaines, neuf versions successives. La première a tué tout le
monde.

```
===== V1 — concept tel que décrit =====
Encore en jeu à la semaine 36 : 0%   (arrêts: 64 frustration, 36 ennui ; semaine médiane d'arrêt 8)
Corrélation score de jeu / dictée réelle : r = 0.27

===== V9 — 7 ans : 6 mots/manche =====
Encore en jeu à la semaine 36 : 71%  (arrêts: 26 frustration, 3 ennui ; semaine médiane d'arrêt 18)
Dictée réelle : sem 1-4 77% -> sem 33-36 72%   | témoin (école seule) 57% -> 57%
Corrélation score de jeu / dictée réelle : r = 0.52
```

Huit corrections plus tard, 71 % des enfants simulés jouent encore en fin
d'année et le score du jeu prédit deux fois mieux la vraie dictée. Le meilleur
prédicteur de l'abandon s'est révélé être la **fréquence de jeu**, pas le niveau
de l'enfant : trois sessions courtes par semaine valent mieux qu'une longue.

Reproduire le tableau complet (neuf versions, ~30 s) :

```bash
go run brief/simulation_dictee.go
```

La simulation ne modélise ni les graphismes, ni les copains, ni le transfert
vers l'écriture à la main. Un mois de jeu réel reste le seul vrai test — c'est
le dernier ticket du backlog, pas le premier.

## Ce qui tourne vraiment

| | État |
|---|---|
| `cmd/client` | ✅ ouvre une fenêtre Ebitengine, résolution logique portrait 390×844 / paysage 1280×720 selon l'orientation |
| Build natif + `js/wasm` | ✅ les deux compilent, **sans CGO** |
| Simulateur d'équilibrage | ✅ tourne, hors build du module (`//go:build ignore`), à porter dans `engine` |
| `engine`, `lexique`, `content`, `server`, `client` | ⛔ spécifiés dans le brief, pas écrits |
| `encre.go`, `cmd/encre` | ⛔ placeholders du kit de démarrage, à supprimer |

## Démarrer

Il faut [mise](https://mise.jdx.dev) ; il installe le reste (Go 1.27.1 et les
seize outils de la chaîne de qualité) à la bonne version.

```bash
git clone https://github.com/oioio-space/encre && cd encre
mise run setup      # outils + hooks git
mise run ci         # la porte complète — doit être verte
```

Ouvrir la fenêtre du client :

```bash
go run ./cmd/client
```

Compiler pour le navigateur :

```bash
GOOS=js GOARCH=wasm go build -o encre.wasm ./cmd/client
```

Ebitengine v2.10 est pure Go sur desktop Linux, macOS et Windows : ni
compilateur C, ni en-têtes de développement. Sous Linux il reste besoin des
bibliothèques X11/OpenGL **à l'exécution** (`libX11`, `libGL`, `libXcursor`,
`libXi`, `libXinerama`, `libXrandr`, `libasound`).

## Comment c'est construit

Client Ebitengine (natif + WASM) ↔ un binaire Go avec SQLite ; le serveur fait
autorité sur le score et le recalcule depuis les tentatives. Le détail est dans
[`brief/ENCRE_04_spec_technique.md`](brief/ENCRE_04_spec_technique.md).

<details>
<summary><b>La chaîne de qualité</b> — ce qui s'exécute à chaque commit</summary>

Chaque commit franchit, dans l'ordre : nettoyage des artefacts → secrets
(gitleaks) → vulnérabilités (gosec + govulncheck) → guide de style Go de Google
→ **interdiction de CGO** → attestation de revue `/simplify` → **trailer `Bead:`**
nommant un ticket qui existe vraiment.

Deux audits gardent les gardes :

| | |
|---|---|
| `mise run check-gates-intact` | compare 53 garde-fous vivants à `scripts/gates.manifest`. `COVER_MIN` est comparé **par sa valeur** : un gate dont le nom survit pendant que sa force est vidée sur place ne passe pas. |
| `mise run cycle:check` | mesure l'état à chaque prompt — hiérarchie des tickets, tickets livrés mais laissés ouverts, arbre en conflit, et si le processus se met à manger le produit. Silencieux quand rien n'est dû. |

Les deux ont leur test de morsure, dans les deux sens : `mise run gates:test`
(39 cas). Couverture minimale : 85 % (100 % aujourd'hui).

</details>

## Le suivi du travail

Les tâches vivent dans [beads](https://github.com/gastownhall/beads), pas dans
des TODO en markdown : 8 epics, 40 tickets, avec leurs dépendances.

```bash
bd ready            # ce qui est débloqué maintenant
bd list -t epic     # la carte
```

`PROGRESS.md` garde le récit et le journal. `.beads/issues.jsonl` est l'export
lisible qui voyage dans git ; la référence est la base Dolt, synchronisée par
`bd dolt push`.

Le premier ticket est `encre-gol` — un prototype de clavier dessiné en WASM,
testé sur une vraie tablette et un vrai téléphone. Il valide la taille des
touches, la latence audio et le temps de chargement avant que le reste de
l'architecture ne soit engagé.

## Le brief

Sept documents font autorité, chacun sur son domaine. En cas de contradiction :
`01` prime sur les règles, `04` sur la technique, `02` sur le visuel, `03` sur
les textes.

| Fichier | Contenu |
|---|---|
| [`ENCRE_00_LISEZMOI.md`](brief/ENCRE_00_LISEZMOI.md) | les décisions prises et leurs raisons |
| [`ENCRE_01_spec_jeu.md`](brief/ENCRE_01_spec_jeu.md) | les règles exactes, la roadmap V1/V2/V3 |
| [`ENCRE_02_charte_graphique.md`](brief/ENCRE_02_charte_graphique.md) | palette, typographie, cartes, créatures, mouvement, son |
| [`ENCRE_03_contenu_pedagogique.md`](brief/ENCRE_03_contenu_pedagogique.md) | le programme CE1 en Couleurs et règles, les textes exacts |
| [`ENCRE_04_spec_technique.md`](brief/ENCRE_04_spec_technique.md) | architecture, contraintes Ebitengine, paquets, API |
| [`ENCRE_05_backlog.md`](brief/ENCRE_05_backlog.md) | 33 tickets ordonnés avec critères d'acceptation (~70 h) |
| [`ENCRE_06_design.md`](brief/ENCRE_06_design.md) | le design |

Ce que le jeu refuse délibérément : séries, notifications à l'enfant,
récompense de connexion, orthographe fausse affichée, classement au score.

## Licence

Aucune licence n'est encore déclarée : tous droits réservés par défaut. Le code
est lisible publiquement, mais pas réutilisable tant que ce fichier n'existe pas.
