# ENCRE — 00 · Lisez-moi

Jeu d'orthographe pour un enfant de 7 ans (CE1), sur la boucle de Balatro, dans un atelier de scribe la nuit. Ce dossier contient tout ce qu'il faut pour le dessiner (Claude Design) puis le coder (Claude Code, en Go avec Ebitengine).

## Les fichiers

| Fichier | Pour qui | Contenu |
|---|---|---|
| `ENCRE_01_spec_jeu.md` | tous | les règles exactes du jeu, la roadmap V1/V2/V3 |
| `ENCRE_02_charte_graphique.md` + `ENCRE_palette.png` | Claude Design | monde, palette, typographie, cartes, créatures, Phalène, interface, mouvement, son, écrans, livrables |
| `ENCRE_03_contenu_pedagogique.md` | Claude Code, parent | programme CE1 en Couleurs et règles, listes d'exemple, textes exacts (Talismans, boss, 60 lignes de Phalène), exploits, modèle p̂, prompt de génération, heuristiques de détection |
| `ENCRE_04_spec_technique.md` | Claude Code | architecture, contraintes Ebitengine, paquets, moteur, lexique, base, API, média, déploiement |
| `ENCRE_05_backlog.md` | Claude Code | 32 tickets ordonnés avec critères d'acceptation (~70 h) |
| `simulation_dictee.go` + `resultats_simulation.txt` | Claude Code | le simulateur d'équilibrage (9 versions, 2 400 saisons) à reprendre comme base du paquet `engine` et comme test d'intégration |

En cas de contradiction : `01` prime sur les règles, `04` sur la technique, `02` sur le visuel, `03` sur les textes.

## Les décisions prises

| Décision | Choix | Raison |
|---|---|---|
| Nom | **ENCRE** | court, un enfant de 7 ans le dit, il nomme le thème |
| Public | 7 ans, CE1, lit seul une phrase courte | Couleurs alignées sur le programme CE1, texte ≤ 6 mots, tout est dit à voix haute |
| Appareils | tablette, téléphone, ordinateur | portrait/paysage, clavier dessiné + clavier physique, touches ≥ 48 px |
| Temps | 20 min/jour, deux runs de 8-9 min | 3 manches × **6 mots** |
| Client | **Ebitengine v2**, WASM + natif | choix du développeur ; risque validé par le ticket T00 |
| Serveur | un binaire Go, SQLite, Litestream, Hetzner 4 €/mois | simplicité, coût |
| Voix | **celle du parent**, enregistrée à la validation ; Piper en repli | compréhension et chaleur à 7 ans |
| Phalène, boss, interface | voix TTS distincte | tout doit pouvoir être entendu |
| Phrases | générées par l'API Claude à la validation, validées par le parent | deux minutes par semaine pour le parent |
| Cibles | `valeur du deck × K × rang × Bienveillance`, K = {2,2 ; 5,5 ; 14} | simulation : les cibles absolues tuaient les enfants moyens |
| Rangs | Blanc → Diamant, montée 8 victoires + 3 semaines, descente après 3 échecs | simulation : sans descente, tout le monde plafonne et s'ennuie |
| Ce qu'on refuse | séries, notifications à l'enfant, récompense de connexion, orthographe fausse affichée, classement au score | santé de l'enfant et efficacité d'apprentissage |

## Ce que la simulation a appris (résumé)

100 enfants simulés, 36 semaines, 9 versions. Le concept initial faisait abandonner 100 % des enfants avant la semaine 10. Huit corrections plus tard, 70 à 82 % jouent encore en fin d'année, l'ennui a disparu, et la corrélation entre le score et la vraie dictée est passée de 0,40 à 0,50. Le prédicteur de l'abandon est la **fréquence de jeu**, pas le niveau de l'enfant : trois sessions courtes par semaine valent mieux qu'une longue. La simulation ne modélise ni les graphismes, ni les copains, ni le transfert vers l'écriture manuscrite : un mois de jeu réel reste le seul vrai test.

## Ordre de travail

1. **Claude Design** avec `02` + `ENCRE_palette.png` (+ `01` pour comprendre). Premier livrable : polices validées sur les accents et palette testée sur trois sprites.
2. **Claude Code** avec `01`, `03`, `04`, `05`, le simulateur, puis les assets de Design quand ils arrivent (ticket T30). Premier ticket : **T00**, le prototype clavier sur les appareils réels.
3. **Un mois de jeu réel** avant toute V2.

## Prompts à coller

**Claude Design**
> Voici la charte graphique et la palette d'un jeu d'orthographe pixel art pour un enfant de 7 ans, dans un atelier de scribe la nuit. Lis `ENCRE_02_charte_graphique.md` en entier et les sections 1 à 3 et 9 de `ENCRE_01_spec_jeu.md` pour le contexte. Produis les livrables du §16 dans l'ordre, en commençant par les trois polices validées sur la chaîne d'accents du §5 et la palette testée sur trois sprites réels. Respecte strictement : une palette de 32 couleurs, une lumière en haut à gauche, sprites 48 × 48 et cartes 96 × 128, script minuscule, touches ≥ 48 px, aucun personnage ni style emprunté à un jeu existant. Le fond mouvant, le clavier et la carte sont les trois éléments qui comptent le plus.

**Claude Code**
> Voici la spécification complète d'un jeu d'orthographe en Go avec Ebitengine. Lis `ENCRE_01_spec_jeu.md`, `ENCRE_03_contenu_pedagogique.md`, `ENCRE_04_spec_technique.md` et `ENCRE_05_backlog.md`. Le simulateur `simulation_dictee.go` est la base du paquet `engine` et devient un test d'intégration. Réalise les tickets de `05` dans l'ordre, un par un, chacun avec ses tests et ses critères d'acceptation cochés, en commençant par T00. Le serveur fait autorité sur le score. Ne propose aucune mécanique absente de `01` : la V1 est volontairement réduite et sera jouée un mois avant la V2. Quand tu as un doute sur une règle, cite la section de `01` et pose la question.
