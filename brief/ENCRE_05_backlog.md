# ENCRE — 05 · Backlog V1 (ordre de réalisation)

> Tickets pour Claude Code, dans l'ordre. Chaque ticket a des critères d'acceptation vérifiables. Estimations en heures de soirée d'un développeur Go. Total V1 ≈ 70 h. Références : `01` règles, `03` contenu, `04` technique.

## Étape 0 — Valider le risque

**T00 · Prototype clavier WASM** (3 h)
Ebitengine, un écran : une carte grise, le clavier AZERTY dessiné avec la rangée d'accents, un mot à taper affiché en script. Build `GOOS=js GOARCH=wasm`, servi par un `http.FileServer`.
- ✅ ouvert sur la tablette et le téléphone réels, dix mots avec é è ç tapés sans erreur d'interface
- ✅ touches ≥ 48 px physiques mesurés (`DeviceScaleFactor`)
- ✅ latence tap → lettre < 50 ms perçue
- ✅ clavier physique sur ordinateur, y compris caractères accentués via `AppendInputChars`
- ✅ un Ogg lu après le premier tap (déblocage du contexte audio)
- ✅ rapport : taille WASM, temps de premier chargement en 4G

## Moteur

**T01 · Paquet `engine` : types et Config** (2 h)
- ✅ `Config` avec les valeurs de `04` §4, chargée depuis JSON, validée (pas de zéro)

**T02 · `Score`** (4 h)
- ✅ tests table-driven : lettres, chaque Couleur, niveaux 1 et 10, aveugle ×3 et ×4 (Sourd), maudite ×5, ternie ×2, Aimant, Bibliothécaire, chaque Talisman V1, combo, Gomme
- ✅ Voleur d'accents met les Accentuées à 0

**T03 · `PHat` et `Targets`** (3 h)
- ✅ p̂ suit `03` §7 ; conditions (une écoute, phrase, Brouillon)
- ✅ propriété : `Targets` identique quels que soient les Talismans
- ✅ Bienveillance : −3 % par run perdue tôt, plancher 0,85, remise à 1 sur victoire

**T04 · Transitions `WordState`** (4 h)
- ✅ dorure : 3 jours distincts, span ≥ 7 jours ; perte sur faute ; ternissure à 4 semaines ; restauration ; malédiction 3 fautes / 3 semaines ; domptage 3 réussites consécutives → dorée ; brillance 1/8 et 1/40 avec RNG injecté
- ✅ Rencontre : `Copy=true` n'affecte ni combo ni compteur de dorure, met `Seen` et +0,12 de maîtrise

**T05 · Rang et XP** (3 h)
- ✅ montée : 8 victoires, 3 semaines, une par semaine ; descente : 3 échecs ; `BestRank`
- ✅ XP seulement aveugle/boss ; coût 12 × n ; cooldown 3 semaines ; max 10

**T06 · `BuildDeck`, salles, boss** (3 h)
- ✅ 10 + Garde + 4 anciens non dorés + maudites ; Garde plafonnée ; ternies en tête de la liste proposée
- ✅ deux salles par transition, Échoppe toujours présente ; boss de la semaine déterministe (`semaine % nb`)
- ✅ `Seed` rend le deck reproductible

**T07 · `Replay` et `Apply`** (4 h)
- ✅ `Replay` recalcule un `Outcome` identique au client sur 50 runs enregistrées
- ✅ Revanche : manche rejouable une fois si ≥ 85 % de la cible, +1 Mult
- ✅ `Apply` idempotent ; émet les `Events` ; Cahier → +50 jetons de bonus stocké pour la prochaine run
- ✅ exploits V1 (`03` §8) déclenchés et débloquant

**T08 · `sim/` en test d'intégration** (3 h)
- ✅ 100 enfants × 36 semaines avec le paquet `engine` réel : M1 ≤ 4 %, boss 15-30 %, rétention simulée ≥ 60 %
- ✅ tourne en < 10 s, exécuté en CI

## Contenu et lexique

**T09 · Paquet `content`** (2 h)
- ✅ JSON embarqués : 8 Talismans V1 (+12 V2 marqués `locked`), 2 boss V1 (+3), 60 lignes de Phalène par contexte, exploits, textes d'interface ≤ 6 mots
- ✅ test : aucune ligne de Phalène > 8 mots, aucune ligne de Talisman > 8 mots

**T10 · Paquet `lexique`** (6 h)
- ✅ TSV embarqué compressé, licence vérifiée et citée dans `LICENSES.md`
- ✅ `Analyze` renvoie Couleurs, règles, positions, famille, confiance
- ✅ `AnalyzeSentence` détecte les Accordées par le contexte
- ✅ ≥ 90 % sur `testdata/ce1_200.tsv`

**T11 · Pré-synthèse Piper** (2 h)
- ✅ script `make voices` : toutes les lignes de `content` → Ogg, embarquées dans le client
- ✅ voix Phalène distincte de la voix de repli des mots

## Serveur

**T12 · Store SQLite et migrations** (3 h)
- ✅ schéma `04` §6, WAL, migrations embarquées, tests avec base en mémoire

**T13 · Auth parent, TOTP, sessions** (4 h)
- ✅ argon2id ; enrôlement TOTP avec QR ; `totp_ok_until` 1 h ; limitation de débit ; cookies sécurisés
- ✅ login enfant pseudo + motif (hash salé), 10 essais/min

**T14 · Listes et validation** (4 h)
- ✅ collage → items analysés avec confiance ; PATCH corrections ; validation refuse un item « à vérifier » non confirmé
- ✅ trois `Kind` : mot, phrase avec cibles, dictée découpée aux ponctuations

**T15 · Génération de phrases** (3 h)
- ✅ appel API à la validation, prompt `03` §9, JSON strict, filtre liste blanche CE1, ≤ 8 mots
- ✅ approbation / régénération par phrase ; repli saisie manuelle

**T16 · Média** (3 h)
- ✅ upload webm/opus → ffmpeg → ogg normalisé ; servi en statique avec cache ; suppression avec la liste
- ✅ repli Piper si aucun audio à la validation

**T17 · `run/start`, `room`, `finish`, `heartbeat`** (5 h)
- ✅ `start` refuse si temps épuisé ; renvoie le `Run` complet avec URLs audio
- ✅ `finish` : `Replay` + `Apply`, rejette une run déjà appliquée, renvoie `Outcome` + `Events`
- ✅ `heartbeat` : temps du jour, bonus du parent

**T18 · Endpoints enfant de lecture** (2 h)
- ✅ bestiaire, fioles, exploits, `result-card.png` (image générée avec `image/png`)

**T19 · Résultat de dictée et mot rapide** (2 h)
- ✅ bonus positif uniquement ; mot rapide → deck « mes mots » ; tableau de bord JSON

## Panneau parent

**T20 · Panneau : connexion, enfants, réglages** (3 h)
- ✅ mobile-first, htmx ; limite par jour ; ABC/AZERTY ; règles activables (Sosies off par défaut) ; Cahier on/off

**T21 · Panneau : semaine** (4 h)
- ✅ collage → validation (Couleurs corrigeables d'un tap) → enregistrement voix (gros bouton, « 7 / 20 ») → date
- ✅ < 2 minutes chronométrées sur une liste de 10 mots avec la voix

**T22 · Panneau : dictée, mot rapide, tableau de bord, export/suppression, rappel dimanche** (3 h)

## Client

**T23 · Squelette Ebitengine** (3 h)
- ✅ `Layout` portrait/paysage, échelle entière, scènes, `juice.json` rechargeable, polices TTF avec vérification des glyphes au démarrage

**T24 · Écran de run en gris** (6 h)
- ✅ carte avec p̂ en points, clavier (T00 intégré), mot qui s'écrit lettre par lettre avec bave, compteur ∝ log, sceau-cible avec fissure, flamme = combo, gouttes qui volent, hitstop, tremblement
- ✅ Sûr / À l'aveugle avec 2 rubans ; correction sans texte (mot juste lettre par lettre, lettre manquée clignote, on retape)
- ✅ score client = `engine.Score` au jeton près

**T25 · Garde, Rencontre, salles, Échoppe** (5 h)
- ✅ éventail des dorées, ternies en tête ; Rencontre face visible ; deux portes ; Échoppe 3 sceaux + relance ; Repos et Encrier

**T26 · Boss, Revanche, Enluminure, Cahier, récap** (5 h)
- ✅ buvard noir + ligne du boss dite ; Chuchoteur (une écoute) ; Brouillon (mot 1 s puis effacé)
- ✅ Revanche « Il manquait N points » ; Enluminure 6 s avec cursive et or ; Cahier avec *C'est fait* ; récap + carte-résultat + *Encore* sans chargement

**T27 · Hors ligne et reprise** (3 h)
- ✅ run sauvegardée à chaque mot (localStorage / fichier) ; reprise après fermeture ; file d'envoi avec rejeu ; fin de temps → écran chaleureux, run conservée

**T28 · Voix et musique** (3 h)
- ✅ haut-parleur sur tout texte > 3 mots ; Phalène dit ses lignes ; 4 couches musicales pilotées par le combo ; sons de `02` §13 (placeholders acceptés)

**T29 · Atelier, Bestiaire, fioles, exploits** (5 h)
- ✅ atelier meublé par le rang, 10 dorées sur le bureau ; Bestiaire deux pages avec filtres et étymologie dite ; fioles ; médailles avec ???

**T30 · Intégration des assets Design** (4 h)
- ✅ palette, polices, 18 sprites, Phalène, cartes 9 états, sceaux, objets, fond Kage ; capture des trois écrans clés conforme aux maquettes

## Mise en production

**T31 · Build et déploiement** (3 h)
- ✅ `make wasm native server` ; Dockerfile ou systemd ; Caddy ; Litestream vers B2 ; restauration testée ; service worker ; `/admin/metrics`

**T32 · Premier mois** (—)
- ✅ observer 3 semaines : M1 ≤ 3 %, M2 ≈ 10 %, boss 15-25 %, temps médian par mot ≤ 25 s, aveugle 25-35 %
- ✅ régler `K`, `juice.json`, taille des touches ; **aucune nouvelle mécanique avant la fin du mois**
