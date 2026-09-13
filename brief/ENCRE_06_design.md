# ENCRE — 06 · Remise du design à Claude Code

> Ce document accompagne les planches HTML. Il ne remplace pas `ENCRE_02_charte_graphique.md` : il enregistre les **écarts décidés** en cours de design et les **valeurs mesurées** à implémenter. En cas de contradiction avec `02`, ce document prime sur les points listés au §2 ; `01` reste seul maître des règles de jeu.

## 1. Les planches livrées

| Fichier | Contenu | Statut |
|---|---|---|
| `ENCRE - Telephone v5 fun.dc.html` | **Écran de jeu téléphone 390 × 844, trois états animés** (pari, écriture, réussite) | référence pour la V1 |
| `ENCRE - Planche v2 claire.dc.html` | Carte recto/verso, 9 états, planche d'animation, Enluminure en 8 vignettes, atelier, run paysage | référence |
| `ENCRE - Planche de design.dc.html` | Première planche, thème nuit | historique, ne pas suivre |

Les planches sont du HTML statique : ouvrir dans un navigateur pour voir les animations tourner en boucle. Les couleurs sont écrites en hexadécimal dans le markup, elles sont lisibles à l'inspecteur.

## 2. Écarts décidés par rapport à `02`

| `02` disait | Décidé | Raison |
|---|---|---|
| Fond d'écran en Encre (thème nuit) | **Fond Parchemin `#EAD9B4`, thème clair** | lisibilité à 7 ans, écran de téléphone tenu à bout de bras en journée |
| L'encre vivante en fond permanent | **Encre vivante uniquement dans la fenêtre de l'atelier** | en fond de run elle rendait l'écran illisible et inerte |
| Clavier AZERTY par défaut partout | **Téléphone : 7 colonnes, ordre alphabétique. Tablette et ordinateur : AZERTY 10 colonnes** | 10 colonnes sur 390 px donnent 36 px par touche, sous les 48 px exigés par `02` §11 |
| Braise pour la chaleur | **Braise `#F5A742` pour les formes, Brique `#8F2F1E` pour les textes chauds** | Ambre brûlé sur Parchemin mesure 2,65:1, sous le plancher de 4,5:1 |
| Or sombre `#B8860B` en annotation | **Cuir `#6E5738` pour tout texte secondaire** | 5,6:1 contre 2,6:1 pour Encre lavée |

Le reste de `02` est respecté : palette 32 couleurs sans ajout, lumière en haut à gauche, script minuscule, pas de blanc ni de noir purs, pas d'Or dans l'interface, pas de chiffres là où des points suffisent.

## 3. Jetons de couleur réellement utilisés

```
fond d'écran de run        #EAD9B4  Parchemin
surface (carte, pilule)    #FBF3DE  Parchemin clair
zone clavier               #CDB48A  Parchemin vieilli
bordure, ourlet de touche  #A88C63  Cuir clair
texte secondaire, labels   #6E5738  Cuir
texte principal, chiffres  #242342  Encre
verso de carte             #242342  Encre  (trame #16152A, filets #55558A)
texte chaud (×7, score)    #8F2F1E  Brique
formes chaudes (flamme)    #FFE08A → #F5A742 → #D9622B
sceau de cible             #8F2F1E  avec creux #5A0F14, lumière #D9622B
dorure (gouttes du pari)   #F2C14E  avec liseré #FFF1B0
```

Les six Couleurs gardent les valeurs de `02` §3 et ne servent qu'au ruban de carte, au glyphe losange et à l'étiquette de score.

## 4. Écran de jeu · téléphone 390 × 844

Bandes de haut en bas, hauteurs fixes :

| Bande | Hauteur | Contenu |
|---|---|---|
| en-tête | 104 px | score (gauche), combo (centre), cible (droite) — chacun avec son label en 14 px |
| barre de cible | 12 px | barre de 6 px, remplissage Braise, rayon 3 px |
| carte | 268 px | carte 180 × 240 centrée + label `le mot à écrire` |
| phrase et mot | 126 px | pilule d'écoute + le mot en cours à 48 px |
| clavier | 334 px | 5 rangées |

**Règle de hiérarchie** : le mot à écrire est le plus gros caractère de l'écran (48 px). Le score descend à 30 px. Ce qui est grand est ce qu'on fait, pas ce qu'on a.

**Ne pas remettre à l'écran** : numéro de manche, points de progression 1..6, pilules `×1 / ×3`. Ces informations vont au récap de fin de manche.

### Clavier téléphone

7 colonnes, **touches 52 × 54 px, pas tactile 55 px**. Conteneur : `padding: 12px 5px 16px`, `gap: 4px` entre rangées, `gap: 3px` entre touches, `justify-content: space-evenly`.

```
é è ê à ç ù î        (rangée d'accents, fond Parchemin, 22 px)
a b c d e f g
h i j k l m n
o p q r s t u
v w x y z  [efface]  [valide]
```

Touche au repos : fond `#FBF3DE`, bordure 1 px `#CDB48A`, **ourlet bas 3 px `#A88C63`**, rayon 6 px, lettre 26 px. Appui : ourlet ramené à 1 px + `translateY(2px)`. `efface` porte une gomme inclinée à −20°, `valide` un sceau de cire ronde ; les deux portent leur mot en 13 px.

L'apostrophe et le tiret de `02` §11 ne tiennent pas en 7 colonnes : à placer sur appui long de la touche `é`, ou en 8ᵉ colonne si le développeur trouve 8 × 48 px sur l'appareil réel.

## 5. La carte

96 × 128 logiques, rendue ×1,875 sur téléphone (180 × 240).

**Recto** : fond Parchemin clair, ruban de Couleur 9 px en haut, créature 88 × 88 au centre, le mot en La Plume, p̂ en 10 points, puis glyphe + nom de Couleur + jetons. Ombre `0 3px 0 #CDB48A, 0 8px 16px rgba(110,87,56,.18)`.

**Verso** : fond Encre, trame croisée à 45° en `#16152A`, double filet `#55558A` puis `#3A3866`, quatre carrés d'angle, une goutte Parchemin vieilli dans un cercle au centre, mot `ENCRE` en 9 px espacé de 0,2 em.

Le verso se comprend comme un verso par trois moyens, à conserver : **clair contre sombre**, **symétrie** (aucun haut ni bas), **marque unique** répétée partout.

Retournement : 300 ms, `scaleX` de 1 → 0,06 → 1, la tranche visible à 150 ms est une bande `#6E5738`. Son : page tournée.

Les 9 états (normale, face cachée, Rencontre, dorée, ternie, maudite, enluminée, holo, polychrome) sont dans `ENCRE - Planche v2 claire.dc.html`, section « la carte, recto et verso ».

## 6. Animations à porter dans `assets/juice.json`

Toutes sont visibles en boucle dans les planches. Les noms correspondent aux `@keyframes` du markup.

| Nom | Durée | Courbe | Effet |
|---|---|---|---|
| `respire` | 3,2 s boucle | ease-in-out | créature au repos, `scale(1.04, .96)` à mi-course |
| `cligne` | 4,6 s boucle | steps(1) | `scaleY(.12)` sur 2 % du cycle |
| `joie` | 900 ms | cubic-bezier(.3,1.4,.5,1) | saut de 12 px, squash 92/110 puis 110/90 |
| `flotte` | 3,4 s boucle | ease-in-out | carte au repos, 3 px |
| `tremble` | 500 ms | ease-out | 2 px × Mult, plafonné 12 px (`02` §12) |
| `etiquette` | 1,8 s | ease-out | `MASQUÉE +30` monte de 10 px, sur-échelle 1,1 puis 1 |
| `jetons` | 1,8 s, décalage 100 ms | ease-in | une goutte par 10 jetons, arc vers le compteur |
| `compteur` | 1,8 s | ease-out | `scale(1.18)` à l'arrivée des gouttes, jamais avant |
| `craque` | 1,8 s | ease-in-out | sceau `scale(1.1) rotate(-3deg)` puis retour ; 2ᵉ fissure à 50 % |
| `vacille` | 1,2 s au repos, **0,7 s** à haut combo | ease-in-out | flamme, origine `50% 100%` |
| `sautille` | 1,2 s, décalage 140 ms | ease-in-out | les trois gouttes d'or du pari |
| `pulseValide` | 1,4 s | ease-in-out | la touche valide quand le mot est complet |
| `bave` | 120 ms | ease-out | 1 px qui déborde puis se résorbe, à chaque lettre |
| `sechage` | 200 ms | ease-in | fondu vers le Parchemin, jamais vers le noir |
| `vol` | 400 ms | cubic-bezier(.3,.7,.4,1) | la lettre part de la touche en arc, rebond 108 % |
| `encre` | 40 s boucle | ease-in-out | l'encre vivante, fenêtre de l'atelier uniquement |

L'Enluminure est découpée en 8 temps dans la planche v2 : ralenti, silence, effacement, cursive 1, cursive 2, cercle des créatures, Phalène, sceau brisé. Total 6 s, non désactivable.

## 7. Le pari, sans lecture

Deux boutons de même taille, hauteur ~86 px, rayon 12 px.

- **j'écoute 2 fois** : fond Encre, deux oreilles dessinées en Parchemin clair, une goutte de cuivre.
- **1 seule fois** : fond Parchemin clair, bordure Brique, bandeau sur les yeux, **trois gouttes d'or qui sautillent**.

Le rapport de gain est porté par le nombre de gouttes, pas par un chiffre. Le libellé reste en 19 px, la conséquence en 14 px.

## 8. Plancher de lisibilité à tenir en code

- Tout texte lu par l'enfant : **≥ 14 px logiques**, contraste **≥ 4,5:1**.
- Le mot à écrire : 48 px. Les noms de Couleurs au score : 16 px, capitales, lettrage +0,08 em.
- Toute cible tactile : **≥ 48 px**, ici 52 × 54 px.
- Chaque objet d'interface porte son nom une fois, en Le Greffe 14 px, en Cuir.
- Jamais deux textes simultanés à l'écran (`01` profil joueur).
- Tout texte a un haut-parleur atteignable.

Contrastes mesurés sur les couples retenus : Encre sur Parchemin 11:1 · Encre sur Parchemin clair 13:1 · Cuir sur Parchemin 5,6:1 · Brique sur Parchemin 5,9:1 · Parchemin clair sur Encre 13:1.

## 9. Ce qui n'est pas dessiné

À produire hors HTML, avant le ticket T30 :

1. Les deux polices pixel TTF avec la chaîne d'accents `é è ê ë à â ù û î ï ô ö ç œ É È À Ç Œ` — les planches utilisent des substituts web (Pixelify Sans, VT323). Les accents seront à dessiner.
2. La cursive scolaire française vectorielle pour l'Enluminure et le Cahier — substituée par Dancing Script dans les planches, qui n'est pas une écriture d'école.
3. Les 18 sprites de créatures 48 × 48 et les 8 icônes de Talismans 32 × 32. Les planches ne montrent que des **emplacements rayés** et une silhouette de goutte générique. Les six familles de `02` §8 restent la spécification.
4. Le shader d'encre vivante en Kage, ou les 8 frames de repli.
5. Écrans non dessinés : Échoppe, annonce de boss, récap de manche, carte-résultat, fin de temps, Bestiaire en version claire, Rencontre.

## 10. Questions ouvertes pour le développeur

1. **L'apostrophe et le tiret** sur le clavier téléphone : appui long ou 8ᵉ colonne ? À trancher sur l'appareil réel (T00).
2. **Ordre alphabétique contre AZERTY** sur téléphone : l'ordre alphabétique a été retenu pour tenir 48 px. Si le parent active l'option AZERTY, les touches retombent à 36 px — faut-il l'interdire en portrait ?
3. **La lisibilité du p̂ en points** au-delà de 6 points sur 10 : à vérifier avec l'enfant, les points pleins et vides se distinguent mal en petit.
4. **La taille de la carte** : 180 × 240 est un agrandissement de 1,875 depuis 96 × 128, ce qui n'est pas une échelle entière. `02` §15 interdit les échelles fractionnaires. Soit la carte passe à 192 × 256 (×2) et la bande gagne 16 px sur le clavier, soit l'interdiction est levée pour la carte.
