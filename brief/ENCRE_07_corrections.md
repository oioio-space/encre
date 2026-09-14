# ENCRE — 07 · Corrections apportées au brief

> Ce fichier corrige `00` à `06`. Le brief a été écrit avant le code ; ce qui suit a
> été **vérifié** en écrivant le code, en mesurant, ou en lisant la recherche. Chaque
> correction porte sa preuve. Quand une correction contredit une section antérieure,
> **c'est elle qui fait foi**, comme `06` §2 fait foi contre `02`.
>
> Rien n'est corrigé « au goût ». Une ligne n'entre ici qu'avec ce qui la démontre.

## 1. Contradictions internes du brief

### 1.1 `03` §4 contre §8 — une maudite ou trois ?

§8 liste « dompter une maudite » parmi les exploits cachés ; §4 débloque Le Tambour sur
« dompter 3 maudites ». Lus comme un seul exploit, les deux se contredisent.

**Décidé : ce sont deux exploits.** Une première récompense cachée pour en dompter une,
puis Le Tambour pour trois. Les deux sections deviennent vraies et l'enfant gagne un
palier intermédiaire. Figé par `TestTheTwoCursedExploitsAreDistinct` dans `content/`.

### 1.2 `03` §8 oublie trois exploits que §4 exige

§4 débloque La Meute et L'Écho sur « 25 dorées », Le Colosse sur « 50 dorées », Le
Miroir sur « 30 réussites à l'aveugle ». Ces trois conditions **n'apparaissent pas**
dans la liste des exploits de §8. `content/data/exploits.json` les ajoute en cachés.

### 1.3 `04` §10 ne dessine que la ligne gagnante

Le diagramme des scènes va de `Run` à `Salle` à `Boss` à `Enluminure`. Il n'a **aucune
arête pour une run perdue**, alors que `01` §3 est explicite : « cible non atteinte =
fin de run […] fin de run → récap ». Telle quelle, la carte des scènes ne permet pas de
perdre.

**Corrigé** : depuis `Run`, `Salle` et `Boss`, le récap est à un coup. Invariante posée
en test : *depuis toute scène où une run peut être en cours, le récap est atteignable*.
Perdre est la moitié qui arrive le plus souvent ; un enfant ne doit jamais rester bloqué
au milieu d'une run.

### 1.4 `02` §12 contre `06` §6 — les timings

Quatre valeurs diffèrent entre les deux tableaux d'animation.

| | `02` §12 | `06` §6 |
|---|---|---|
| `bave` | 150 ms | **120 ms** |
| gouttes de jetons | 400 ms, écart 40 ms | **1,8 s, écart 100 ms** |
| carte au repos | 2 px sur 2 s | **3,4 s, 3 px** |
| `tremble` | décroissance 200 ms | **500 ms** |

**`06` §6 fait foi.** Son en-tête dit que ces animations sont « visibles en boucle dans
les planches » et que « les noms correspondent aux `@keyframes` du markup » : ce sont les
valeurs des maquettes réellement livrées, qu'un développeur peut regarder tourner. `02`
§12 était l'intention. Là où `06` §6 est muet, `02` §12 complète.

Le `compteur` à 1,8 s de `06` n'est pas un conflit : c'est la valeur figée de la démo. La
règle reste celle de `02` — durée ∝ log(score), bornée à 0,3 s – 3 s.

### 1.5 `02` §5 se contredit sur La Plume

§5 demande « pixel **serif** » **et** « pas de `a` à double étage ». Les deux sont
incompatibles : le `a` à un étage est une marque des polices d'apprentissage, qui sont
toutes sans empattement. Toutes les serifs pixel libres testées ont un `a` à double
étage.

**Décidé : la lisibilité gagne**, parce que la raison que §5 donne lui-même pour exiger
le `a` à un étage est pédagogique — c'est la forme que l'enfant apprend à écrire. La
Plume est une pixel sans. Le caractère de scribe se déplace vers la cursive et le logo,
où il ne coûte pas de lisibilité.

## 2. Faits vérifiés qui contredisent le brief

### 2.1 Les accents ne sont pas à dessiner

`02` §5 et `06` §9 annoncent que « les accents seront très probablement à dessiner
(FontForge) ». **C'est faux.** Trente polices ont été téléchargées et passées à
`client/ui.MissingGlyphs`, l'outil du dépôt, sur les 47 runes obligatoires
(`é è ê ë à â ù û î ï ô ö ç œ É È À Ç Œ` plus l'alphabet, l'apostrophe et le tiret) :
une vingtaine couvrent **100 %**. Zéro accent à dessiner.

### 2.2 `m3x6` n'a aucun accent

Piste citée par `02` §5. Mesuré : **19 runes manquantes**, la police est en ASCII pur.
Inutilisable telle quelle. `m5x7`, l'autre piste citée, les a toutes — mais **ne déclare
aucune licence** : le CC0 qu'on lit ailleurs vient d'un tiers, pas de l'auteur.
Indéfendable dans `LICENSES.md`.

### 2.3 Les pistes de cursive du brief sont non libres

`02` §5 évoque « une police scolaire libre type Écriture A ». **Belle Allure** et
**Écolier** sont gratuites *hors usage commercial* — incompatibles avec une distribution
MIT. « Écriture A » est introuvable sous licence vérifiable.

**Trouvé mieux : Marelle**, la cursive scolaire du **ministère de l'Éducation
nationale**, SIL OFL 1.1, v1.005 (août 2026), développée par une équipe de sept
enseignants et designers. Sa variante `MarelleLIGNES` **intègre le lignage Seyes** :
c'est l'écran Cahier de `01` §14, gratuitement. Nom réservé — un fichier sous-ensemblé
doit être renommé.

### 2.4 Les trois faces retenues

| Rôle | Police | Licence | Grille |
|---|---|---|---|
| La Plume | **Ark Pixel 16 px proportional latin** | SIL OFL 1.1, sans nom réservé | 16 px |
| Le Greffe | **Ark Pixel 10 px proportional latin** | idem | 10 px |
| La Cursive | **Marelle** (+ `LIGNES` pour le Cahier) | SIL OFL 1.1, **nom réservé** | vectoriel |

Environ 195 Ko sous-ensemblées pour les trois. Chiffres tabulaires confirmés sur Ark
Pixel, `a` et `g` à un étage, `l` / `I` / `1` nettement distincts.

## 3. Ce que la recherche impose de changer

### 3.1 Les touches sont à la limite adulte, pas à la limite enfant

`02` §11 et `06` §8 posent un plancher de 48 px, soit environ 9 mm. C'est le minimum
**adulte** d'Android.

> Un enfant de 7 à 10 ans rate une cible de 7 mm dans environ 30 % des essais, et rate
> encore le minimum de 9 mm **une fois sur six**, jusqu'à 17 ans.
> — Anthony et al., *Physical dimensions of children's touchscreen interactions*,
> IJHCS 2019 — 116 enfants, plus de 55 000 touches.
> <https://www.sciencedirect.com/science/article/abs/pii/S1071581918302441>

Le Nielsen Norman Group recommande **20 × 20 mm** pour les moins de 9 ans.
<https://www.nngroup.com/articles/children-ux-physical-development/>

**C'est géométriquement impossible** : sept colonnes sur 390 pt plafonnent à 9,2 mm par
touche même sans aucun espacement. Mais la même recherche désigne **l'espacement**, plus
que la taille, comme la cause des frappes sur la voisine.

**Corrigé dans le code** : la zone active n'est plus le rectangle dessiné. Ce qui est
sous le doigt gagne ; en dehors, la touche **la plus proche** répond jusqu'à 12 px ;
au-delà, rien. Les espaces cessent d'être des trous. Prouvé par propriété : aucun point
du bloc de touches n'est mort, et le pardon ne vole jamais une frappe à une voisine mieux
placée.

### 3.2 L'interlettrage manque au brief

> Un espacement inter-lettres élargi d'environ 18 % du corps **double la précision** de
> lecture et gagne plus de 20 % de vitesse chez l'enfant. Effet immédiat, sans
> entraînement.
> — Zorzi, Barbiero, Facoetti et al., *PNAS* 2012.
> <https://www.pnas.org/doi/10.1073/pnas.1209921109>

Le brief ne fixe aucun interlettrage. **À ajouter** : environ +15 à +20 % du corps sur le
mot en cours et sur les lettres des touches. Coût d'implémentation nul, gain mesuré.

### 3.3 Le plancher de 9 px est trop bas pour un texte vraiment lu

`02` §5 pose « jamais en dessous de 9 px logiques ». À 9 px, Le Greffe donne une hauteur
d'x de 1,4 à 1,5 mm — **sous le plancher adulte de 2 mm**, très loin des 4 mm que
Wilkins et al. recommandent pour un lecteur débutant.

Le seuil protège un texte *reconnu* globalement (un score, un libellé). Il ne suffit pas
dès que l'enfant doit **lire lettre à lettre** : nom de Couleur au score, ligne d'un
Talisman. Ces textes-là montent à 20 px logiques (2 × la grille d'Ark Pixel 10).

Le mot en cours à 48 px donne une hauteur d'x de 7 à 7,6 mm — généreux, ne pas y toucher.

### 3.4 Un losange recoloré ne suffit pas pour un enfant daltonien

**5,6 % des garçons** d'âge préscolaire sont daltoniens (JAMA Ophthalmology), et le type
le plus courant confond précisément le **rouge des Accentuées** (`#D9525C`) et le **vert
des Masquées** (`#6FB57A`) — deux des six Couleurs.

Le losange unique plus le nom écrit satisfait la lettre de WCAG 1.4.1, mais la technique
G111 recommande des **motifs différents** par catégorie. Un enfant de sept ans qui ne lit
pas encore couramment devra **lire** au lieu de reconnaître, à chaque mot.

**À corriger** : six glyphes distincts, reconnaissables en niveaux de gris.

### 3.5 OpenDyslexic : ne pas y aller

Trois études contrôlées (Kuster et al. 2018, Marinus et al. 2016, Wery & Diliberto 2017)
ne trouvent **aucun gain** par rapport à une bonne sans-serif. Le seul effet mesuré vient
probablement de l'espacement, pas du dessin des lettres — c'est-à-dire du §3.2 ci-dessus.

### 3.6 L'Enluminure doit être réductible

`01` §14 la dit « non désactivable ». Six secondes plein écran avec ralenti et cercle
rotatif : c'est le point du jeu le plus éloigné de l'esprit de WCAG 2.3.3, et la rotation
plein écran est le motif le plus documenté comme déclencheur de malaise.

**À corriger** : un réglage « moins d'animations » qui coupe d'abord le **tremblement**,
puis retire de l'Enluminure le cercle rotatif et le ralenti — **en gardant le mot en or**,
qui est l'information. La flamme (0,8 à 1,4 Hz) reste : très sous le seuil de 3 Hz de
WCAG 2.3.1.

## 4. Questions ouvertes de `06` §10, maintenant tranchées

### 4.1 La carte passe à 192 × 256 — échelle ×2

`06` §10.4 posait la question : 96 × 128 rendu en 180 × 240 est un facteur **×1,875**,
que `02` §15 interdit. **Décidé : la carte passe à 192 × 256**, et la bande clavier rend
16 px (334 → 318), ce qui laisse encore 54 px par touche — au-dessus du plancher.

Quatre raisons, dont la dernière est la plus embarrassante :

1. **L'artefact est réel, pas une convention.** À ratio non entier en nearest-neighbor,
   les texels source se répartissent inégalement : certains deviennent 1 × 1 physique,
   d'autres 2 × 2 dans la même image. C'est le *pixel shimmering*, documenté
   (<https://tanalin.com/en/articles/integer-scaling/>).
2. **180/96 = 15/8.** Aucun `DeviceScaleFactor` réel — 2 ou 3 — ne rattrape ça ; il
   faudrait un multiple de 8. Le fractionnaire persiste au pixel physique sur tout
   appareil réaliste.
3. **C'est sur la carte que se trouve le texte le plus fin** lu par un enfant qui
   déchiffre encore : le mot, les chiffres, les points de p̂. Le jitter touche exactement
   les arêtes de ces glyphes — et la carte **bouge** (`flotte`, retournement, `joie`), ce
   qui est le cas documenté comme le pire.
4. **Le dépôt se contredirait lui-même.** `client/ui/scale.go` porte déjà la doctrine mot
   pour mot dans son commentaire : *« A fractional scale resamples every glyph and every
   sprite off the pixel grid, which is what turns pixel art into mush »*. Laisser la carte
   y déroger ferait dire au même code deux choses opposées.

### 4.2 AZERTY en portrait : non

`06` §10.2 demandait s'il faut interdire l'option AZERTY en portrait, qui ferait tomber
les touches à 36 px. **Décidé : oui, interdite en portrait.** 36 px ≈ 6 mm, sous *tous*
les planchers documentés, adulte comme enfant. L'option reste offerte en paysage et sur
tablette, où la largeur la permet.

### 4.3 Le thème clair, le soir — toujours ouverte

`06` §2 a choisi le fond Parchemin pour « la lisibilité, écran tenu à bout de bras **en
journée** ». Mais `01` dit que le jeu se joue **le soir**. Un fond clair maintient le
rétroéclairage à pleine puissance quel que soit le contenu, donc émet plus de lumière
qu'un fond Encre à réglage égal, et l'effet documenté sur l'endormissement vient surtout
de la luminosité émise.

Aucune étude ne chiffre le cas précis parchemin contre encre : **ce n'est pas un
changement à faire aveuglément**. Piste plutôt qu'un retour au thème nuit, déjà écarté
pour de bonnes raisons : un mode « soir » optionnel, parchemin atténué.

## 4 bis. Ce que la recherche sur le *game feel* apporte

### Le juice se paie en précision — et le brief l'avait déjà deviné

> Version « juicy » jugée plus plaisante (3,74 contre 3,26), mais performance en baisse
> de **19 %**. Et aucune amélioration de la facilité *perçue*, contrairement à
> l'hypothèse de Norman.
> — Juul & Begy, *Good Feedback for bad Players?*, FDG/DiGRA 2016, N = 46.
> <https://www.jesperjuul.net/text/juiciness.pdf>

C'est la seule mesure contrôlée du corpus, et elle est un avertissement : pour un enfant
de sept ans qui apprend à écrire, perdre en précision est exactement ce qu'on ne veut pas.

`02` §12 pose déjà la bonne discipline — *« Seul le jus du score a le droit de rebondir »*
— qui concentre l'effet sur un seul canal au lieu de le diffuser. **À tenir
rigoureusement**, et à opposer à toute envie d'en rajouter.

### Le son d'abord, pas l'image

> *« Audio, haptic feedback, particle systems, and animation are the most important
> sources of juiciness »* — dans cet ordre.
> — Pichlmair & Johansen, *Designing Game Feel: A Survey*, 2020.
> <https://arxiv.org/pdf/2011.09201>

Le **retour haptique** est classé deuxième et **n'apparaît nulle part dans le brief**.
À ajouter sur la lettre piège, le combo cassé et la dorée : un canal de plus, sans une
seule pixel de bruit visuel, et particulièrement adapté à une motricité encore imprécise.

### Ce que Balatro fait et qu'ENCRE peut copier gratuitement

LocalThunk explique le score caché comme un choix délibéré : *« le jeu est plus amusant
quand on monte sa machine de Rube Goldberg et qu'on la regarde tourner avant de savoir si
la main passe »* (<https://gmtk.substack.com/p/balatros-cursed-design-problem>).

**À ajouter : une demi-seconde de silence** après la dernière lettre, avant que le total
ne parte vers le compteur. Un réglage dans `juice.json`, aucun asset, et c'est le
mécanisme le mieux documenté du modèle explicite du jeu.

Note : l'escalade sonore d'un demi-ton par combo que `02` §13 décrit **est déjà** le
mécanisme de Balatro. Le brief l'avait converti correctement sans le dire.

### Le haut-parleur est au mauvais endroit

Hoober, 1 333 observations de terrain : 49 % des gens tiennent leur téléphone d'une main,
le bas de l'écran est la zone facile, **les coins hauts la plus dure**
(<https://www.uxmatters.com/mt/archives/2013/02/how-do-users-really-hold-mobile-devices.php>).

Le haut-parleur de réécoute est en haut à droite de la carte — la zone la plus dure de
tout l'écran — alors qu'un CE1 hésitant va le taper à presque chaque mot. Les boutons de
pari sont en zone d'étirement alors qu'ils sont tapés à chaque mot eux aussi. **À
déplacer vers le bas.**

### Le nombre d'or est un mythe, et le brief ne l'utilise pas

Aucune preuve de lien avec l'esthétique perçue
(<https://www.ncbi.nlm.nih.gov/pmc/articles/PMC10792139/>). Les bandes d'ENCRE dérivent de
contraintes fonctionnelles — mot à 48 px, touches ≥ 48 px — et non d'un ratio esthétique.
C'est la bonne méthode : ne pas y toucher.

### 4.2 Le thème clair, le soir

`06` §2 a choisi le fond Parchemin pour « la lisibilité, écran tenu à bout de bras **en
journée** ». Mais `01` dit que le jeu se joue **le soir**. Un fond clair maintient le
rétroéclairage à pleine puissance quel que soit le contenu, donc émet plus de lumière
qu'un fond Encre à réglage égal, et l'effet documenté sur l'endormissement vient surtout
de la luminosité émise.

Aucune étude ne chiffre le cas précis parchemin contre encre : **ce n'est pas un
changement à faire aveuglément**. Piste plutôt qu'un retour au thème nuit, déjà écarté
pour de bonnes raisons : un mode « soir » optionnel, parchemin atténué.

## 4 ter. Les 85 % de Wilson ne sont pas atteignables dans ce modèle

`03` §7 fixe le modèle d'apprentissage : `maîtrise += 0,20 × (1 − maîtrise)` à la
réussite, `×= 0,93` par semaine sans jeu. La question était de savoir si un réglage de
ces deux nombres suffirait à amener l'enfant au taux de réussite optimal de 85 %
([Wilson et al., *Nature Communications* 2019](https://www.nature.com/articles/s41467-019-12552-4)).

**Réponse mesurée : non.** Balayage sur cohorte de 100 enfants, 36 semaines, graine fixe.

| `LearnFloor` | `ForgetFloor` | mots justes | M1 | boss | rétention | > Blanc |
|---|---|---|---|---|---|---|
| 0,12 | 0,88 | 67,1 % | 2,1 % | 24,3 % | 100 % | 80 % |
| 0,20 | 0,90 | 70,1 % | 2,0 % | 23,5 % | 100 % | 89 % |
| 0,36 | 0,90 | 73,1 % | 1,5 % | 22,8 % | 100 % | 90 % |
| 0,52 | 0,90 | 75,6 % | 1,6 % | 22,3 % | 100 % | 97 % |
| 0,70 | 0,90 | 77,3 % | 1,2 % | 22,0 % | 99 % | 99 % |

**Aucun garde-fou ne mord** sur toute la grille — M1, boss, rétention et progression de
rang restent sains partout. C'est le taux de mots justes lui-même qui plafonne, vers
**75 à 77 %**.

Le cas dégénéré le dit mieux que le tableau : avec `LearnFloor = 1,0` — *une réussite
suffit à maîtriser un mot, il n'y a plus d'apprentissage progressif du tout* — on atteint
**79 %**. Il faut donc **détruire le modèle** pour franchir 78 %, et détruire le modèle
retire précisément la courbe de difficulté dans laquelle l'optimum de Wilson est censé
se placer. Le remède annulerait la raison du remède.

**Décidé** : on ne règle pas `LearnRate` et `Forget` pour ce chiffre. Le seuil de
`sim_test.go` est **65–80 %**, avec cette mesure comme justification.

**Ce qui reste à explorer**, si l'écart compte un jour : la composition du pool, pas la
vitesse d'apprentissage. Le diagnostic d'origine montre que le p̂ moyen du pool plafonne
à 0,60 et que même les mots **dorés** ne dépassent pas 0,736 — un afflux constant de mots
neufs et peu maîtrisés dilue la moyenne. Ce sont `WordsPerWeek`, `RecycleOld` et la taille
de la Garde qu'il faudrait regarder, pas la courbe d'un mot isolé.

Note au passage sur la valeur du brief : les nombres exacts de `03` §7 (`LearnRate = 0,20`,
`Forget = 0,93`, sans dispersion individuelle) donnent **66,8 %** — à peine différent du
défaut actuel. La dispersion par enfant que le code ajoute suit donc bien la moyenne que
le brief prescrit.

## 4 quater. Le budget WASM passe de 3 à 4 Mo brotli, et Litestream ne chiffre plus

### Le budget

`04` §2 budgète « ~10 Mo de WASM non compressé, ~3 Mo brotli ». Mesuré après intégration
des vraies polices : **~25 Mo brut, ~7,2 Mo gzip, ~4,1 Mo brotli**.

Ce ne sont **pas** les polices — 265 Ko pour les trois, sous-ensemblées — mais
`go-text/typesetting`, le moteur de composition que `GoTextFace` entraîne dès qu'une vraie
TTF est chargée. Les leviers ont été essayés et mesurés :

- `-trimpath -ldflags="-s -w"` : **3 %**. Appliqué, mais anecdotique.
- **TinyGo** : refuse net — `tinygo 0.41.1` exige Go 1.19 à 1.26, le projet est en 1.27.
  Il faudrait rétrograder tout le dépôt pour un gain incertain.

**Décidé : le budget passe à ~4 Mo brotli.** Une décision assumée vaut mieux qu'un chiffre
qu'on fait semblant de tenir. Le premier chargement est mis en cache par un service worker,
donc l'écart ne se paie qu'une fois par version.

### Litestream ne sait plus chiffrer

`04` §12 prévoit « Litestream : replicate continu vers B2 » et `ENCRE_07` §3 exige que la
cible soit chiffrée, puisque le fichier SQLite contient les hash de mots de passe, les
motifs des enfants et les secrets TOTP.

**Vérifié en le faisant tourner** : Litestream **v0.5.17**, la version courante, **refuse
le chiffrement age** — « age encryption is not currently supported… revert back to v0.3.x ».
Ce n'est pas une supposition, c'est le message du binaire.

**Décidé : épingler Litestream à v0.3.14**, la dernière version où `age:` fonctionne, et
faire tourner l'aller-retour complet. Il est un **processus séparé**, donc son besoin de
CGO ne concerne pas ce module.

Et la restauration est **prouvée, pas espérée** — `deploy/restore-test.sh` vérifie les
trois choses qui comptent : l'instantané commence bien par l'en-tête
`age-encryption.org/v1` ; la restauration **échoue** sans l'identité ; elle **réussit et
correspond** avec elle. C'est le jour où on en a besoin qu'on découvre qu'un script de
restauration ne marche pas.

### Deux des six mesures d'`/admin/metrics` ne sont pas calculables

`04` §12 demande six mesures. Quatre sortent du schéma actuel : répartition des rangs,
échec par rang et par manche, temps moyen par mot, part de l'aveugle. **Les dorées par
semaine et les revanches ne s'en déduisent pas** — rien ne les horodate. Documenté dans le
code plutôt que simulé.

## 5. Décisions techniques qui ne sont pas dans le brief

- **SQL** : `ent` (entgo.io) écarté — ses clés primaires composites ne marchent que pour
  les schémas d'arête M2M, or `04` §6 en a trois, dont deux (`attempts(run_id, idx)` et
  `play_time(child_id, day)`) ne sont pas des arêtes. Retenu : `database/sql` +
  `modernc.org/sqlite` (pur Go) + migrations `goose` embarquées.
- **Écosystème Ebitengine** : aucune bibliothèque adoptée. `gween` n'exprime pas les
  cubic-bezier arbitraires dont `06` §6 a besoin (`joie` monte à 1,4), et les courbes
  doivent rester des **données** puisque `juice.json` est rechargeable à chaud. `furex`
  arrondit correctement mais tire 24 modules transitifs, dont un inlineur CSS pour
  e-mails. L'évaluateur de courbes est écrit à la main, ~60 lignes, algorithme de WebKit.
- **Qualité des tests** : le score de mutation (`mise run mutate`, gremlins) est la mesure
  retenue plutôt que la couverture. Premier relevé : MSI **100 %** sur `engine` et
  `lexique`, zéro mutant survivant.
