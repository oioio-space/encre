# ENCRE — 04 · Spécification technique

> Pour Claude Code. Langage : **Go** partout. Client : **Ebitengine v2** (décision prise), compilé en WASM pour le navigateur et en natif pour l'ordinateur. Serveur : un binaire, SQLite. Les règles de jeu sont dans `01`, le contenu dans `03`, les tickets dans `05`.

## 1. Architecture

```
┌──────────────────────────────┐   HTTPS/JSON   ┌────────────────────────────────────┐
│ Client ENCRE (Ebitengine v2) │◄──────────────►│ Serveur Go, un binaire              │
│  GOOS=js GOARCH=wasm         │                │  /api/v1  (net/http, mux Go 1.22)   │
│  natif linux/mac/windows     │                │  /        panneau parent (templates │
│  clavier dessiné, audio Ogg  │                │           + htmx)                   │
│  cache de run hors ligne     │                │  /static  WASM, assets, audio       │
└──────────────────────────────┘                │  SQLite WAL (modernc.org/sqlite)    │
┌──────────────────────────────┐                │  Litestream → Backblaze B2          │
│ Panneau parent (navigateur)  │◄──────────────►│  ffmpeg (transcodage voix)          │
│  MediaRecorder pour la voix  │                │  Piper (TTS de repli et Phalène)    │
└──────────────────────────────┘                └────────────────────────────────────┘
```

- **Hébergement** : VPS Hetzner CX22 (4 €/mois), Caddy devant (TLS auto, brotli, cache long sur `/static`). Sauvegarde continue Litestream vers B2 (gratuit à cette taille). Restauration testée une fois avant la mise en production.
- **Le moteur fait autorité** : le client calcule le score pour l'affichage, le serveur le **recalcule** à la fin de la run depuis la liste des tentatives et applique les transitions.
- **Hors ligne pendant la run** : tout le deck (mots, phrases, audio, p̂, cibles, boss, salles tirées) est reçu au démarrage ; les tentatives sont envoyées à la fin, rejouées en cas d'échec réseau (file persistante).

## 2. Contraintes Ebitengine à connaître

| Sujet | Décision |
|---|---|
| Version | Ebitengine v2.8+ ; Go 1.22+ |
| Résolution logique | `Layout` renvoie 390 × 844 en portrait, 1280 × 720 en paysage, choisi selon le ratio de la fenêtre ; échelle **entière** via `ebiten.DeviceScaleFactor()` (×2, ×3, ×4), pixel art net avec `FilterNearest` |
| Texte | `text/v2` avec des polices **TTF pixel** (`GoTextFace`) rendues à taille entière ; trois faces : Plume, Greffe, Cursive (`02` §5). Vérifier les glyphes accentués au démarrage en dev (`HasGlyph`) et échouer bruyamment s'il en manque |
| Saisie | clavier **dessiné** : rectangles + `inpututil.AppendJustPressedTouchIDs` / `IsMouseButtonJustPressed` ; clavier physique : `ebiten.AppendInputChars` + `inpututil.IsKeyJustPressed` (Backspace, Enter). Aucune dépendance à `exp/textinput`. Les caractères accentués du clavier physique arrivent par `AppendInputChars` |
| Audio | `audio.NewContext(48000)` ; formats **Ogg Vorbis** (`audio/vorbis`) et WAV ; le serveur transcode la voix du parent en Ogg Vorbis (ffmpeg) ; sur navigateur, le contexte audio ne démarre qu'après un geste : le premier tap sur *Jouer* le débloque |
| Shader | fond « encre vivante » en **Kage** ; repli sprite-sheet 8 frames si le GPU est faible |
| Réseau WASM | `net/http` fonctionne (fetch sous le capot) ; timeouts 10 s ; file de renvoi |
| Persistance WASM | `syscall/js` → `localStorage` pour la run en cours et la file d'envoi ; natif : fichier JSON dans `os.UserConfigDir()` |
| Taille | ~10 Mo de WASM non compressé, ~3 Mo brotli ; **service worker** qui met en cache le WASM, `wasm_exec.js` et les assets ; premier chargement affiché avec la goutte du logo |
| Mobile | `ebiten.SetWindowResizingMode`, pas de rotation forcée ; le layout suit l'orientation ; gyroscope non disponible en WASM Ebitengine → l'holo suit le **doigt/souris** |
| Touches | ≥ 48 px **physiques** : la taille logique des touches est calculée depuis `DeviceScaleFactor` |

**Étape 0, avant tout** : un prototype d'une soirée, clavier dessiné en WASM, dix mots avec accents, sur la tablette **et** le téléphone réels. Il valide la taille des touches, la latence audio et le chargement. Si la saisie n'est pas fluide, on corrige le clavier ; le reste de l'architecture ne change pas.

## 3. Organisation des paquets

```
encre/
  cmd/server/          main du serveur
  cmd/client/          main Ebitengine (build js/wasm et natif)
  engine/              règles du jeu, pur Go, zéro dépendance, 100 % testé
  lexique/             détection des Couleurs et règles, familles
  content/             données embarquées : Talismans, boss, lignes de Phalène, exploits (JSON + embed)
  server/
    api/               handlers /api/v1
    parent/            panneau (templates, htmx)
    store/             SQLite, migrations embarquées
    auth/              argon2id, sessions, TOTP
    media/             transcodage ffmpeg, Piper
    gen/               génération de phrases (API Anthropic)
  client/
    game/              boucle, scènes, état
    ui/                carte, clavier, compteur, sceau, flamme, particules
    anim/              tweens, hitstop, shake, bave/séchage
    audio/             lecture, couches musicales
    net/               client API, file hors ligne
    assets/            embed : sprites, polices, sons, juice.json
  sim/                 simulation_dictee.go adapté au paquet engine (test d'intégration)
```

## 4. Le paquet `engine`

Reprend `simulation_dictee.go` (fonctions `pCorrect`, `chips`, `multBonus`, transitions, rang, Garde) et le rend propre.

```go
package engine

type Config struct {
    WordsPerManche  int         // 6
    K               [3]float64  // {2.2, 5.5, 14}
    RankMult        [6]float64  // {1, 1.3, 1.7, 2.2, 2.8, 3.5}
    RankUpWins      int         // 8
    RankUpMinWeeks  int         // 3
    RankDownFails   int         // 3
    KindnessStep    float64     // 0.03  (Bienveillance)
    KindnessFloor   float64     // 0.85
    BlindTokens     int         // 2
    BlindMult       float64     // 3
    ChipPerTrap     float64     // 10
    XPPerLevel      float64     // 12 × niveau
    LevelCooldownW  int         // 3
    LevelMax        int         // 10
    GoldDays        int         // 3
    GoldMinSpanDays int         // 7
    TarnishWeeks    int         // 4
    CurseFails      int         // 3
    CurseWeeks      int         // 3
    HoloOdds        float64     // 1.0/8
    PolyOdds        float64     // 1.0/40
    RevancheWindow  float64     // 0.85
    RecycleOld      int         // 4
    GardeSlots      int         // 3 (+ rang/2, max 5)
    RunMoney        int         // 4
    MancheMoney     int         // 4 + min(3, floor(score/cible))
    RerollCost      int         // 3
    ChronoSeconds   float64     // 10
    PresseSeconds   float64     // 15
    CahierBonus     float64     // 50
}

type Color int   // Masquees, Muettes, Jumelles, Accentuees, Accordees, Sosies
type Rule string // "ch", "t_muet", "mm", ...

type Word struct {
    ID       string
    Text     string
    Letters  int
    Traps    map[Color]int    // nombre de pièges par Couleur
    Rules    []Rule
    Family   string
    Kind     Kind             // Word, Sentence, Dictation
    Sentence string           // phrase à trou si Kind == Sentence
}

type WordState struct {
    Mastery      float64
    SuccessDays  []int32     // jours distincts (jours depuis epoch)
    FirstSuccess int32
    Fails        int
    FailWeeks    []int32
    ConsecOK     int
    Gold, Tarnished, Cursed, Seen bool
    Shine        int         // 0, 1 holo, 2 polychrome
    LastPlayedW  int32
}

type Child struct {
    Rank, BestRank, Prestige int
    BossWinsAtRank, WeeksAtRank, BossFailStreak int
    Kindness   float64            // 1.0 → 0.85
    Level      map[Color]int
    XP         map[Color]float64
    LevelUpW   map[Color]int32
    Base       RollingRate        // taux glissant sur mots nouveaux
    Unlocked   []TalismanID
    BossWinsTotal int
}

type Deck struct {
    Week    []Word
    Garde   []Word
    Old     []Word
    Cursed  []Word
    Rooms   [2][2]RoomID   // deux choix de salle × deux transitions
    Boss    BossID
    Seed    int64
}

type Attempt struct {
    WordID  string
    Manche  int
    Blind   bool
    Copy    bool   // Rencontre
    Correct bool
    Typed   string
    Millis  int
}

type Run struct {
    ID        string
    ChildID   string
    Deck      Deck
    Rank      int
    Targets   [3]float64
    Talismans []TalismanID
    Rooms     [2]RoomID
    Attempts  []Attempt
    Revanche  [3]bool
    Cahier    bool
}

// API du moteur
func BuildDeck(c *Child, week []Word, states map[string]*WordState, garde []string, now time.Time, rng *rand.Rand, cfg Config) Deck
func PHat(c *Child, w Word, st *WordState, ctx Ctx) float64
func Targets(c *Child, d Deck, states map[string]*WordState, cfg Config) [3]float64
func Score(a Attempt, w Word, st *WordState, owned Talismans, combo float64, ctx Ctx, cfg Config) (chips, mult float64)
func Replay(run Run, states map[string]*WordState, cfg Config) (Outcome, error)   // recalcule tout, déterministe
func Apply(c *Child, run Run, out Outcome, states map[string]*WordState, now time.Time, cfg Config) []Event
```

`Replay` est déterministe à partir de `Deck.Seed` et des tentatives. `Apply` est idempotent (une run déjà appliquée ne l'est pas deux fois). `Event` : `GoldEarned, GoldLost, Tarnished, Restored, Cursed, Tamed, LevelUp, RankUp, RankDown, Unlock, ExploitDone, Shine, Revanche, Enluminure`.

**Tests exigés** : table-driven sur `Score` (chaque Talisman, chaque synergie, aveugle, maudite, ternie) ; transitions de `WordState` (dorure, perte, ternissure, malédiction, domptage) ; rang (montée, descente, cooldown hebdo, Bienveillance) ; propriété : `Targets` ne dépend jamais des Talismans ; test d'intégration : `sim/` tourne 100 enfants × 36 semaines et vérifie M1 ≤ 4 %, boss entre 15 et 30 %, rétention simulée ≥ 60 %.

## 5. Le paquet `lexique`

- Données : lexique français avec phonétique embarqué en TSV compressé (`ortho, phon, lemme, cgram, freq`), filtré aux ~15 000 formes les plus fréquentes. **Vérifier la licence** (Lexique 3 sur lexique.org ; alternative Morphalou sur Ortolang) avant embarquement.
- `Analyze(word string) Analysis` → `Traps map[Color]int`, `Rules []Rule`, `Positions []int` (index des lettres pièges pour l'illumination), `Family string`, `Confidence float64`.
- `AnalyzeSentence(s string, targets []Span) []Analysis` → ajoute les Accordées par le contexte.
- Heuristiques : `03` §10. Mot inconnu → `Confidence < 0.6` → « à vérifier ».
- Jeu de test : `lexique/testdata/ce1_200.tsv`, exigence ≥ 90 %.

## 6. Modèle de données (SQLite)

```sql
parents(id TEXT PK, email TEXT UNIQUE, pass_hash BLOB, totp_secret BLOB, created_at INT)
children(id TEXT PK, parent_id TEXT, pseudo TEXT, pattern_hash BLOB, avatar INT,
         daily_limit_json TEXT, rank INT, best_rank INT, prestige INT, boss_wins_at_rank INT,
         weeks_at_rank INT, boss_fail_streak INT, kindness REAL, base_json TEXT,
         level_json TEXT, xp_json TEXT, levelup_w_json TEXT, unlocked_json TEXT,
         exploits_json TEXT, boss_wins_total INT, settings_json TEXT, created_at INT)
word_lists(id TEXT PK, child_id TEXT, label TEXT, share_code TEXT, due_date INT, validated INT, created_at INT)
items(id TEXT PK, list_id TEXT, kind INT, text TEXT, targets_json TEXT, colors_json TEXT,
      rules_json TEXT, family TEXT, audio_path TEXT, source INT, confidence REAL, enabled INT)
sentences(id TEXT PK, item_id TEXT, text TEXT, target_form TEXT, audio_path TEXT, approved INT)
word_states(child_id TEXT, item_id TEXT, state_json TEXT, PRIMARY KEY(child_id, item_id))
runs(id TEXT PK, child_id TEXT, started_at INT, finished_at INT, rank INT, deck_json TEXT,
     targets_json TEXT, talismans_json TEXT, rooms_json TEXT, failed_at INT, applied INT)
attempts(run_id TEXT, idx INT, item_id TEXT, manche INT, blind INT, copy INT, correct INT,
         typed TEXT, millis INT, chips REAL, mult REAL, PRIMARY KEY(run_id, idx))
dictee_results(id TEXT PK, list_id TEXT, item_id TEXT, correct INT, entered_at INT)
sessions(token TEXT PK, kind INT, subject_id TEXT, expires_at INT, totp_ok_until INT)
play_time(child_id TEXT, day INT, seconds INT, bonus_seconds INT, PRIMARY KEY(child_id, day))
```

Migrations SQL embarquées (`embed`), appliquées au démarrage. Index : `word_states(child_id)`, `runs(child_id, started_at)`, `items(list_id)`.

## 7. API `/api/v1`

Sessions par cookie `HttpOnly; Secure; SameSite=Lax`, une pour l'enfant (24 h), une pour le parent (7 jours) avec `totp_ok_until` (1 h) pour les actions sensibles.

**Enfant**
- `POST /child/login {pseudo, pattern}` → session
- `GET /child/me` → profil, rang, temps restant, dorées disponibles pour la Garde (avec ternies en tête), objets d'atelier
- `POST /run/start {gardeIDs}` → `Run` complet (deck, phrases, URLs audio, p̂, cibles, boss, salles) ; refuse si temps épuisé
- `POST /run/{id}/room {index, roomID}` → effet de la salle (Repos : liste de choix ; Encrier : pièces)
- `POST /run/{id}/finish {attempts, talismans, revanche, cahier}` → `Outcome` recalculé + `Events`
- `POST /run/{id}/heartbeat {seconds}` → temps joué (toutes les 30 s)
- `GET /child/bestiary`, `GET /child/rules`, `GET /child/exploits`, `GET /child/result-card/{runID}.png`

**Parent** (session + TOTP frais)
- `POST /parent/signup`, `POST /parent/login`, `POST /parent/totp/enroll` (QR), `POST /parent/totp/verify`
- `POST /children`, `PATCH /children/{id}`, `POST /children/{id}/bonus-time {minutes}`, `POST /children/{id}/rules {rule, enabled}`
- `POST /lists {childID, rawText, kind}` → liste analysée (Couleurs, règles, confiance, phrases générées)
- `PATCH /lists/{id}/items/{itemID}`, `POST /lists/{id}/sentences/{sid}/approve`, `POST /lists/{id}/sentences/{sid}/regenerate`
- `POST /lists/{id}/items/{itemID}/audio` (multipart webm/opus → ffmpeg → ogg) ; idem pour `sentences`
- `POST /lists/{id}/validate`
- `POST /lists/{id}/dictee-result {results}`
- `POST /children/{id}/quick-word {text}` (bouton « ajouter ce mot »)
- `GET /children/{id}/dashboard`, `GET /export`, `DELETE /account`

**Sécurité** : argon2id (t=3, m=64 Mo), TOTP fenêtre ±1, limitation 5 login/min/IP et 10 motifs/min/enfant, CSRF sur le panneau, en-têtes CSP stricts, aucune donnée personnelle d'enfant, export et suppression complets.

## 8. Média

- **Voix du parent** : MediaRecorder (webm/opus) → `ffmpeg -i in.webm -c:a libvorbis -q:a 4 -ar 48000 out.ogg` ; normalisation `loudnorm` ; silence coupé aux extrémités. Stockage `media/{listID}/{itemID}.ogg`.
- **TTS** : Piper en binaire local, voix `fr_FR-siwis-medium` pour le repli des mots, seconde voix pour Phalène/boss/interface. Les lignes de `content/` sont **pré-synthétisées au build** et embarquées dans le client (Ogg, ~60 lignes × 2 s). Les justifications et étymologies sont synthétisées à la validation.
- **Musique** : 4 pistes Ogg synchronisées (couches), démarrées ensemble, volume par couche piloté par le combo.

## 9. Génération de phrases

`server/gen` appelle l'API Anthropic à la validation avec le prompt de `03` §9, `max_tokens` 800, sortie JSON parsée strictement, 3 phrases par mot, rejet de toute phrase > 8 mots ou contenant un mot hors d'une liste blanche CE1 (embarquée, ~3 000 mots). Coût : quelques centimes par semaine. Repli : le parent tape la phrase.

## 10. Client : scènes et état

```
Boot → Atelier → Garde → Run(Manche 1) → Salle → Run(Manche 2) → Salle → Boss → Enluminure → Cahier → Récap → Atelier
                                                                    └─ Revanche ─┘
Atelier → Bestiaire | Fioles | Exploits | FinDeTemps
```

- **État de run** : struct sérialisable, sauvegardée à chaque mot (reprise après fin de temps ou fermeture).
- **Score affiché** = `engine.Score` côté client avec la même `Config` reçue du serveur.
- **Clavier** : composant `ui.Keyboard` ; layouts AZERTY et ABC ; rangée d'accents ; gestion tactile + physique ; taille des touches depuis `DeviceScaleFactor`.
- **Carte** : `ui.Card` avec les 9 états ; p̂ en points ; haut-parleur.
- **Compteur** : animation ∝ log(score), feu au-delà du seuil (`juice.json`).
- **Flamme** : hauteur = f(combo), vacillement sur faute.
- **Voix** : `audio.Speaker` ; tout texte affiché a un `Say()`.
- **juice.json** : tous les timings de `02` §12, rechargeable à chaud (touche F5 en natif).

## 11. Panneau parent

Templates Go + htmx, mobile-first (le parent est sur son téléphone). Pages : connexion (+TOTP), enfants, semaine (collage → validation → voix → date), mot rapide, résultat de dictée, réglages (limite, ABC/AZERTY, règles activées, Cahier, Sosies), tableau de bord, export/suppression. L'enregistrement audio utilise `MediaRecorder` avec un gros bouton par item et un compteur de progression « 7 / 20 ».

## 12. Déploiement

- `Dockerfile` multi-étapes (build WASM + natif + serveur) ou binaire statique + `systemd`.
- Caddyfile : TLS, `encode zstd gzip`, `header /static/* Cache-Control "public, max-age=31536000, immutable"` (assets versionnés par hash).
- Litestream : `replicate` continu vers B2 ; script de restauration testé.
- Journal `slog` JSON ; tableau minimal en SQL (`/admin/metrics`, parent seulement) : échec par manche et rang, répartition des rangs, dorées/semaine, temps/mot, part de l'aveugle, revanches.

## 13. Ordre de développement

1. **Étape 0** : prototype clavier WASM sur les appareils réels.
2. `engine` + tests + `sim/` en test d'intégration.
3. `lexique` + jeu de test 200 mots CE1.
4. `content/` : Talismans, boss, Phalène, exploits (JSON depuis `03`), pré-synthèse Piper.
5. Serveur : store, auth, TOTP, listes, média, `run/start`, `run/finish`, temps de jeu.
6. Panneau parent minimal (collage → validation → voix → date).
7. Client : écran de run en rectangles gris, clavier, carte, compteur, flamme, jetons.
8. Garde, Rencontre, salles, Échoppe, boss, Revanche, Enluminure, Cahier, récap, carte-résultat.
9. Intégration des assets de Claude Design, `juice.json`, sons, musique en couches.
10. Atelier, Bestiaire, fioles, exploits, fin de temps.
11. Déploiement, Litestream, service worker, premier mois de jeu réel.
