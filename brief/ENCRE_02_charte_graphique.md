# ENCRE — 02 · Charte graphique et sonore

> Pour Claude Design. À lire avec `ENCRE_palette.png`. Les règles de jeu sont dans `01`, les textes dans `03`. Le joueur a **7 ans**, joue sur tablette, téléphone et ordinateur, et lit des phrases courtes.

## 1. Le monde en trois phrases

Il est tard. Dans un atelier de scribe éclairé à la bougie, les mots sont de petites créatures d'encre qui n'attendent qu'à être écrites correctement pour se fixer sur le parchemin et rejoindre le Bestiaire. Phalène, un papillon de nuit, veille sur la table et sur toi.

**Test de cohérence** : si un élément ne peut pas exister sur la table d'un scribe la nuit, il n'existe pas dans le jeu.

**Ton** : chaleureux, malicieux, un peu ancien. Pas de mignonnerie infantile. L'enfant a 7 ans et on l'a laissé entrer dans un lieu d'adulte : c'est ça qui le fait se sentir grand.

## 2. La signature : l'encre vivante

Comme le fond mouvant de Balatro, mais en encre. Une nappe d'indigo profond avec des volutes plus sombres et, par endroits, un reflet de braise. Shader Kage sous Ebitengine, ou animation de 8 frames en repli. Un cycle en 40 secondes. On doit la sentir respirer sans jamais la regarder.

**Règle** : l'encre vivante est la seule chose qui bouge en permanence. Tout le reste est au repos jusqu'à ce qu'on le touche.

## 3. La palette (32 couleurs)

Planche : `ENCRE_palette.png`. Une palette, une lumière, aucun ajout en cours de projet.

**Encres** `#0B0A14` Nuit · `#16152A` Encre profonde · `#242342` Encre · `#3A3866` Encre diluée · `#55558A` Encre lavée
**Parchemins** `#FBF3DE` Parchemin clair · `#EAD9B4` Parchemin · `#CDB48A` Parchemin vieilli · `#A88C63` Cuir clair · `#6E5738` Cuir
**Braise** `#FFE08A` Flamme · `#F5A742` Braise · `#D9622B` Ambre brûlé · `#8F2F1E` Brique
**Or** `#FFF1B0` Or clair · `#F2C14E` Or · `#B8860B` Or sombre
**Malédiction** `#B3202A` Sang · `#5A0F14` Sang séché
**Neutre** `#7FA69A` Vert-de-gris

**Les six Couleurs** (teinte / ombre), uniquement pour signaler une Couleur :

| Couleur | Teinte | Ombre |
|---|---|---|
| Muettes | `#9FC4E8` | `#4F7BA6` |
| Jumelles | `#B07CE0` | `#6C42A0` |
| Accentuées | `#D9525C` | `#8C2C38` |
| Masquées | `#6FB57A` | `#3E7A4A` |
| Sosies | `#5BC8C4` | `#2E8683` |
| Accordées | `#E8A94C` | `#A86E22` |

**Règles**
- L'**Or** ne colore jamais l'interface : il n'existe que sur ce qui a été gagné.
- La **Braise** est la chaleur et l'urgence : flamme du combo, compteur qui brûle, Revanche.
- Texte : Parchemin clair sur Encre, ou Encre sur Parchemin. Jamais de blanc pur ni de noir pur.
- Une Couleur n'est jamais un fond. Pas de dégradé hors holo et encre vivante.
- **Contraste** : tout texte lu par l'enfant respecte un ratio ≥ 4,5:1 (Parchemin clair sur Encre = 13:1 ; Encre lavée sur Encre profonde est **interdit** pour du texte, réservé aux détails).
- La palette est une base harmonieuse **non testée sur sprites** : la valider est le premier livrable.

## 4. La lumière

Une bougie en haut à gauche. Ombres portées en bas à droite, 1 px. Liseré de Flamme sur les arêtes tournées vers la bougie. Rien n'est éclairé par en dessous.

## 5. Typographie (contrainte critique)

Deux polices pixel au format **TTF** (Ebitengine `text/v2` les rend à taille entière), toutes deux avec la chaîne : `é è ê ë à â ù û î ï ô ö ç œ É È À Ç Œ`. **Tester avant tout autre travail** ; les accents seront très probablement à dessiner (FontForge).

- **La Plume** — titres et le mot en cours. Pixel serif, **lettres en script minuscule bien distinctes** (pas de a à double étage ambigu, un g simple, un l distinct du I et du 1). Capitale 16 px logiques ; le mot en cours s'affiche à **24 px logiques** minimum. L'encre **bave** à l'apparition de chaque lettre (1 px qui déborde puis se résorbe en 120 ms).
- **Le Greffe** — interface, chiffres. Pixel sans, 9 px, chiffres tabulaires. Jamais en dessous de 9 px logiques pour un texte que l'enfant lit.
- **La Cursive** — utilisée **uniquement** pour l'Enluminure (le mot réécrit en or) et l'écran Cahier. Une cursive scolaire française simplifiée, pas de pixel : rendue en vectoriel ou en sprite-sheet pré-tracée. À 7 ans, voir le mot réécrit dans l'écriture de l'école est un pont réel.

Pistes libres à vérifier : m5x7 et m3x6 (Daniel Linssen) pour le rythme ; pour la cursive, une police scolaire libre type « Écriture A » (licence à vérifier).

## 6. Le logo

**ENCRE** en La Plume, majuscules, la barre du E final se prolonge en coulure et forme une goutte. Une couleur. Animation : la coulure descend en 800 ms, Phalène se pose sur le C.

## 7. Anatomie de la carte (96 × 128 logiques)

```
┌──────────────────────────┐
│ ▐ ruban Couleur          │  4 px, teinte de la Couleur dominante
│      [créature 48×48]    │
│         chat             │  La Plume, script minuscule, Encre
│   ●●●●●●○○○○             │  p̂ en points, jamais en chiffres
│  ◆ MUETTE      18        │  glyphe de Couleur + jetons, Le Greffe
└──────────────────────────┘
```

Contour 1 px Encre profonde, coins 2 px, fond Parchemin. Seconde Couleur = second ruban en bas. Un **haut-parleur** discret en haut à droite : tap = le parent dit le mot.

| État | Fond | Cadre | Détail |
|---|---|---|---|
| Normale | Parchemin | Encre profonde | — |
| Face cachée | Encre | Encre lavée | logo en filigrane |
| Rencontre | Parchemin clair | Flamme | un œil ouvert en haut à droite |
| Dorée | Parchemin | **Or** 2 px | coins ornés, créature stade 2 |
| Ternie | Parchemin vieilli | Or sombre | voile de poussière |
| Maudite | Cuir | **Sang** 2 px | cornes hors cadre, yeux rouges |
| Enluminée (V2) | Parchemin | Or ornementé | créature stade 3 avec décor |
| Holo | — | — | balayage arc-en-ciel des six Couleurs, suit l'inclinaison |
| Polychrome | — | — | holo + fond qui scintille |

**Talismans** (72 × 96, sceau de cire) : commun cire Encre lavée, rare cire Bleu spectral, légendaire cire Or à reflet Flamme. **Icône grande** (32 × 32) au centre, nom en La Plume dessous, une ligne en Le Greffe. Haut-parleur : Phalène dit la saveur.

## 8. Les six familles de créatures

Des **créatures d'encre**, comme si une tache avait pris vie. Contour irrégulier, un ou deux yeux Parchemin clair, pas de bouche sauf expression. Pas d'animaux réels, pas de référence à une licence.

| Famille | Silhouette | Signe | Idle | Stade 3 (V2) |
|---|---|---|---|---|
| Masqués (Masquées) | goutte ronde | masque de parchemin devant le visage | le masque glisse et se replace | masque levé, visage visible |
| Fantômes (Muettes) | goutte allongée flottante | translucide, une lettre flotte à côté | monte et descend 2 px / 2 s | la lettre posée en couronne |
| Jumeaux (Jumelles) | deux gouttes collées | symétrie parfaite, clignent ensemble | se penchent l'un vers l'autre | main dans la main, un petit troisième |
| Couronnés (Accentuées) | goutte trapue | pointes en forme d'accents, aigrette | l'aigrette oscille | manteau et sceptre |
| Métamorphes (Sosies) | goutte floue | deux silhouettes décalées | s'écartent puis se rejoignent | une silhouette nette, un miroir |
| Meutes (Accordées) | trois petites gouttes | jamais seuls | se poussent | cinq en formation, un étendard |

V1 : 6 familles × 2 stades (normale, dorée) + une seconde silhouette du stade 1 = **18 sprites**. V2 : + 6 stades 3.

## 9. Phalène

Papillon de nuit d'encre, 32 × 32, ailes Parchemin vieilli à deux ocelles Braise, corps Encre. Quatre poses : posé, en vol (2 frames), penché (parle), endormi. Bulle Parchemin, Le Greffe, **≤ 8 mots**, et **toujours dite à voix haute** (voix TTS de personnage, un peu grave, lente). Il ne fait jamais de geste explicatif.

## 10. Le vocabulaire d'interface

| Élément | Objet de l'atelier |
|---|---|
| Score | compteur à rouleaux de cuivre |
| Cible | sceau de cire qui se fissure à 50 % et éclate |
| Combo | la flamme de la bougie, grandit et vacille |
| Multiplicateur | loupe de laiton sur le compteur |
| Jetons | gouttes d'encre qui volent |
| Argent | pièces dans un encrier |
| Talismans | sceaux de cire |
| Garde | étui de cuir à trois cartes |
| Bestiaire | grand livre relié |
| Temps du jour | petit sablier |
| Bandeaux (aveugle) | deux rubans de soie sur la table |
| Boss | un buvard noir se déploie, la créature en sort |
| Niveaux de Couleurs | fioles d'encre remplies au niveau |
| Exploits | médailles de cire au mur |
| Salles | deux portes dessinées sur la table, une lampe devant chacune |
| Cahier | un vrai cahier à carreaux posé à droite |

## 11. Le clavier intégré (le composant le plus important après la carte)

- **Dessiné dans le jeu**, jamais le clavier système.
- **AZERTY** par défaut (cohérence avec le clavier physique de l'ordinateur), option **ABC** dans le panneau parent pour les premières semaines.
- **Script minuscule** sur les touches, La Plume 12 px.
- **Rangée d'accents toujours visible** au-dessus des lettres : `é è ê à ç ù î ô -` et l'apostrophe.
- **Touches ≥ 48 × 48 px physiques** sur tactile : à 7 ans le doigt est imprécis. Sur téléphone en portrait, ça impose 3 rangées de 10 touches sur toute la largeur et le clavier occupe ~45 % de l'écran.
- Deux touches spéciales : **Effacer** (une gomme) et **Valider** (un sceau), grandes, à droite.
- Feedback : la touche s'enfonce de 3 px, la lettre vole vers le mot, atterrit avec un micro-rebond, grattement de plume.
- Sur ordinateur, le clavier physique fonctionne et le clavier dessiné s'illumine en écho.

## 12. Le langage du mouvement

**Encre et papier.** Ce qui apparaît **bave** (masque d'encre qui s'étend, 150 ms). Ce qui disparaît **sèche** (fondu vers le Parchemin, 200 ms). Les cartes glissent et se posent. Seul le jus du score a le droit de rebondir.

**Timings** : hitstop 80 ms sur lettre piège, 150 ms sur légendaire · squash 85 % en 60 ms, rebond 108 %, retour 180 ms · tremblement 2 px × Mult plafonné 12 px, décroissance 200 ms · gouttes 400 ms en arc, une par 10 jetons, 40 ms d'écart, le compteur ne monte qu'à l'arrivée · compteur : durée ∝ log(score), 0,3 s à 3 s, prend feu au-delà d'un seuil · dernière lettre du boss à demi-vitesse 400 ms · cartes au repos : 2 px sur 2 s désynchronisées.

**L'Enluminure** (6 s) : ralenti → le mot se réécrit **en cursive et en or** lettre par lettre → les créatures du deck en cercle → musique à pleines couches → Phalène se pose → sceau brisé.

Toutes les valeurs dans `assets/juice.json`, rechargeables à chaud.

## 13. Le son

Chaque objet a son son, rien n'en a deux.

Lettre tapée : grattement de plume (6 variantes) · lettre piège : goutte dans l'encrier, +1 demi-ton par combo · combo cassé : souffle sur la flamme · cible : cire qui craque · Talisman : sceau pressé · dorée : clochette de laiton · maudite domptée : cloche grave puis clochette · écran : page tournée · Phalène : froissement d'ailes · fin de temps : bougie soufflée, silence.

**Musique** : une pièce par saison, 84 bpm, **en couches** ajoutées par le combo (1 : boîte à musique et nappe · 3 : percussion feutrée · 6 : flûte basse · 10 : voix sans paroles), retirées une à une sur deux mesures quand il casse. Le silence est utilisé : l'annonce du boss coupe tout une seconde. Format Ogg Vorbis (Ebitengine), boucles sans couture, une piste par couche synchronisée.

**Voix** : les mots et phrases = la voix du parent. L'interface, Phalène, les boss = TTS Piper voix française, ton posé, vitesse 0,9. Un enfant de 7 ans doit pouvoir **tout** entendre d'un tap.

## 14. Les trois écrans clés

**L'atelier (accueil)** : plan large, bougie à gauche, Bestiaire à droite, étui de la Garde au centre, sablier, Cahier. Les objets de rang sur les étagères, les 10 dernières dorées sur le bureau. Phalène près de la bougie. Un seul bouton : un sceau *Jouer*. L'encre vivante à travers la fenêtre.

**La run**, portrait (390 × 844) de haut en bas : compteur + sceau-cible (12 %) · loupe + flamme (6 %) · la carte, grande (28 %) · le mot qui s'écrit + les deux rubans (9 %) · le clavier (45 %). Paysage (1280 × 720) : clavier à droite (55 % de la largeur), carte à gauche, compteur au-dessus, Phalène en bas au centre.

**Le Bestiaire** : le livre ouvert plein écran. Gauche : grille 3 × 4 de cartes, fioles en marge pour filtrer. Droite : la carte en grand, la famille dessous (dorées en Or, manquantes en silhouette), deux lignes d'étymologie **dites à voix haute**, l'historique en icônes. Phalène tient la page.

## 15. Ce qu'on ne fait pas

Pas de blanc ni de noir purs. Pas d'Or sur l'interface. Pas d'animal réel ni de style emprunté. Pas de rebond cartoon hors jus. Pas de mignonnerie bébé. Pas d'échelle fractionnaire (×2, ×3, ×4). Pas de majuscules hors logo et noms de Couleurs au score. Pas de texte < 9 px, pas de touche < 48 px, pas de chiffres là où des points suffisent.

## 16. Livrables

1. Les trois polices validées sur la chaîne d'accents, rendu ×3.
2. La palette testée sur trois sprites réels, ajustée si besoin.
3. La carte en 9 états, le Talisman en 3 raretés, les 8 icônes de Talismans V1.
4. Les 18 sprites de créatures V1.
5. Phalène, 4 poses.
6. L'atelier en 6 états de rang, la fenêtre en 3 saisons, les 16 objets d'interface.
7. Le clavier en portrait et paysage, états repos/appui/écho.
8. Les trois écrans clés en 390 × 844 et 1280 × 720, plus l'Échoppe, l'annonce de boss, le récap et la fin de temps.
9. Planche d'animation : bave, séchage, flamme à 4 niveaux, Enluminure en 8 vignettes.
10. Le logo, statique et 6 frames.
