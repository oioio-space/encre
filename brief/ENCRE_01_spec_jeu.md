# ENCRE — 01 · Spécification du jeu

> Jeu d'orthographe pour un enfant de **7 ans (CE1)**, construit sur la boucle de Balatro : une run courte, des jetons × un multiplicateur, des salles entre les manches, un boss. Le deck est la liste de mots de la semaine. La difficulté maximale du jeu est la dictée réelle de la semaine.
>
> Ce document fait autorité sur les **règles**. Le contenu (mots, règles CE1, textes) est dans `03`, la technique dans `04`, le visuel et le son dans `02`. Les constantes viennent de 2 400 saisons simulées (`simulation_dictee.go`, version V9) et sont des points de départ à régler sur l'enfant réel.

## Profil du joueur et conséquences

| Fait | Conséquence dans le jeu |
|---|---|
| 7 ans, CE1 | les Couleurs suivent le programme CE1 (`03`) ; les Masquées (sons complexes) sont centrales ; les Sosies se limitent à a/à, et/est, on/ont |
| lit seul une phrase courte | les phrases font **≤ 8 mots**, vocabulaire CE1 ; tout texte d'interface tient en **≤ 6 mots** et peut être **lu à voix haute** d'un tap |
| écrit en cursive, lit en script | les touches et les mots s'affichent en **script minuscule** ; l'Enluminure réécrit le mot en cursive |
| tape lentement | **6 mots par manche**, run de 8 à 9 minutes ; boss Pressé à 15 s ; Chronomètre à 10 s |
| joue sur tablette, téléphone et ordinateur | portrait sur mobile, paysage sur ordinateur, mêmes assets ; clavier tactile dessiné **et** clavier physique |
| 15 à 20 minutes par jour | limite par défaut **20 min**, soit deux runs ; fin de session nette |
| attention courte | zéro menu entre deux runs, une décision à la fois, jamais deux textes à l'écran |

## 1. Principes non négociables

1. **On apprend en jouant.** Le nom de la Couleur apparaît collé au score, à chaque lettre piège. Pas de tutoriel, pas d'écran de règles.
2. **Jamais d'orthographe fausse affichée.** En cas d'erreur : le mot juste s'écrit lettre par lettre, la lettre manquée clignote, l'enfant retape. Trois secondes, pas de message.
3. **Score et maîtrise sont deux compteurs séparés.** Le score est le plaisir, la maîtrise est la vérité.
4. **Prenant, pas compulsif.** Pas de série qui casse, pas de notification à l'enfant, pas de récompense de connexion, une fin de session nette.
5. **Jouable sans savoir bien lire.** Icône + mot court partout, voix sur tout ce qui est du texte.
6. **La première run se comprend sans explication.**
7. **Le jeu ne remplace pas le cahier.** Il prépare la main et l'œil ; l'écriture manuscrite reste à l'école et, en option, dans le rituel du Cahier (§14).

## 2. Les six Couleurs

| Couleur | Piège | Poids attendu en CE1 | Créature |
|---|---|---|---|
| **Masquées** | son complexe (ou, oi, on, an, in, ch, gn, ph, eau, ill, qu, gu, ç, ge, m devant m-b-p) | ★★★★ | Masqués |
| **Muettes** | lettre finale muette (chat, grand, gros, porte) | ★★★ | Fantômes |
| **Jumelles** | consonne double (pomme, belle, chatte) | ★★ | Jumeaux |
| **Accentuées** | é è ê à ç ù | ★★★ | Couronnés |
| **Accordées** | -s du pluriel, -e du féminin, -ent du verbe (en phrase seulement) | ★★ | Meutes |
| **Sosies** | a/à, et/est, on/ont, son/sont (activables par le parent) | ★ | Métamorphes |

Un mot porte une ou deux Couleurs. Les Accordées n'apparaissent jamais hors d'une phrase.

## 3. La run

**Deck d'une run** (jamais identique) :
- les **10 mots de la semaine**
- la **Garde** : 3 cartes dorées choisies par l'enfant (+1 emplacement tous les 2 rangs, max 5)
- **4 anciens mots** non dorés au hasard
- les **maudites** en cours

**Structure** :

| Étape | Contenu | Cible |
|---|---|---|
| Manche 1 · Écoute | 6 mots seuls (Accordées en phrase) | 2,2 × valeur du deck |
| Salle | Échoppe **ou** l'autre salle tirée (§5) | — |
| Manche 2 · Mot caché | 6 mots dans une phrase à trou (≤ 8 mots) | 5,5 × valeur du deck |
| Salle | idem | — |
| Manche 3 · Boss | 6 mots en phrases + modificateur de boss | 14 × valeur du deck |

Cible non atteinte = fin de run, sauf Revanche (§10). Fin de run → récap 3 s → *Encore* en un tap.

**Avant chaque mot** : **Sûr** (deux écoutes, ×1) ou **À l'aveugle** (une écoute, jetons ×3, le combo tombe si faux). **2 bandeaux** (paris) par manche.

**Durée** : 18 mots × ~20 s + salles + Enluminure ≈ **8 à 9 minutes**.

## 4. Le mode Rencontre

Première apparition d'un mot pour cet enfant : la carte se retourne face visible, l'enfant **recopie** le mot pendant qu'il le voit. Aucun échec possible, gain = jetons ×1, combo inchangé. La carte se retourne ensuite et rejoint le deck. Le lundi devient une run de découverte : en simulation, l'échec en manche 1 passe de 5 % à 3 %.

## 5. Les salles

Entre deux manches, deux salles sont proposées, **l'Échoppe toujours** plus une tirée au hasard. L'enfant choisit d'un tap.

| Salle | Effet | Version |
|---|---|---|
| **L'Échoppe** | 3 Talismans au choix, une relance à 3 pièces | V1 |
| **Le Repos** | restaurer une ternie sans la jouer, ou récupérer une dorée perdue cette semaine | V1 |
| **L'Encrier** | +6 pièces | V1 |
| **L'Antre** | affronter une maudite seule : jetons ×8 si réussie, rien sinon | V2 |
| **La Bibliothèque** | révéler une règle en silhouette | V2 |

## 6. Le score

```
jetons(mot) = nb_lettres × 1
            + Σ pièges × 10 × niveau_de_la_Couleur      (1..10)
            × 1,5 si dorée avec Aimant ; × 2 si dorée avec Bibliothécaire
            × 2 si ternie restaurée (ce passage seulement)
            × 5 si maudite
            × 3 si à l'aveugle (× 4 avec Sourd)
mult(mot)   = combo + Σ bonus des Talismans
score(mot)  = jetons × mult                              (0 si faux)
```

**Combo** : 1 au départ, +1 par mot juste, **retombe à 1** sur une faute (sauf Gomme, première faute de la manche). Il **se conserve entre les manches** : le boss se joue naturellement à haut multiplicateur.

Les lettres scorent **une par une** de gauche à droite ; chaque lettre piège s'illumine de sa Couleur avec son nom (`MASQUÉE +30`). Les scores s'affichent avec un espace des milliers (`1 250`).

## 7. La cible : la valeur du deck

```
valeur_du_deck = Σ jetons(mot, sans Talisman) × p̂(mot)
cible(manche)  = valeur_du_deck × K[manche] × multiplicateur_de_rang × Bienveillance
```

- **p̂(mot)** : taux de réussite estimé de l'enfant sur ce mot (`03` §7), affiché sur la carte en **points**, jamais en chiffres à 7 ans (●●●●●○○○○○).
- **K = {2,2 ; 5,5 ; 14}** pour 6 mots par manche.
- **Bienveillance** (invisible) : chaque run perdue en manche 1 ou 2 baisse les cibles suivantes de 3 %, cumulable jusqu'à −15 %, remise à 1 dès une run gagnée.

Cibles de réglage : échec manche 1 ≤ 3 %, manche 2 ≈ 10 %, boss 15 à 25 %. En simulation, changer K déplace surtout la répartition des rangs, pas les taux d'échec : les taux d'échec se règlent par les **conditions de rang**.

## 8. Les rangs

| Rang | Mult. | Contraintes |
|---|---|---|
| Blanc | 1,0 | deux écoutes, pas de modificateur de boss |
| Bronze | 1,3 | le boss a un modificateur |
| Argent | 1,7 | manches 2 et 3 : 60 % des mots n'ont qu'une écoute |
| Or | 2,2 | manche 2 : une phrase complète **courte** (≤ 5 mots) à taper |
| Platine | 2,8 | une seule écoute partout |
| Diamant | 3,5 | phrases partout, modificateur systématique : la dictée de la maîtresse |

**Montée** : 8 victoires au boss **et** 3 semaines au rang minimum, une montée par semaine.
**Descente** : 3 échecs consécutifs au boss → rang −1. Le meilleur rang reste affiché comme badge.
Le rang ne s'applique pas au mode Rencontre. En simulation, la répartition finale des rangs est une courbe en cloche centrée sur Bronze-Argent : c'est voulu.

## 9. Les cartes

- **Dorée** : 3 réussites sur **3 jours différents**, **7 jours** au moins entre la première et la dernière. Entre au Bestiaire.
- **Perte de la dorure** : une faute → carte normale, compteur remis à zéro.
- **Ternie** : dorée non jouée depuis **4 semaines**. Une réussite la restaure, ce passage vaut jetons ×2.
- **Maudite** : mot non doré raté **3 fois sur 3 semaines différentes**. Jetons ×5, casse le combo si ratée. **3 réussites consécutives** → domptée, devient dorée avec cérémonie.
- **Enluminée** (V2) : quand toute sa **famille de mots** est dorée (chat, chaton, chatte), la créature passe au stade 3.
- **Brillante** : à la dorure, 1/8 holographique, 1/40 polychrome. Aucun effet.
- **Encres rares** (V2) : au boss, 1/20 qu'une lettre piège soit écrite dans une encre spéciale, conservée pour toujours. Aucun effet.

## 10. La Garde et la Revanche

**La Garde** : avant la run, choix de 3 dorées à emporter (les ternies sont présentées en premier avec leur ×2). C'est le deck-building **et** la répétition espacée. En simulation, 170 cartes distinctes révisées par an via la Garde.

**La Revanche** : manche ratée à **moins de 15 %** de la cible → « Il manquait 87 points. » → un bouton : la manche est rejouée une fois avec **+1 Mult**. Se déclenche ~19 fois par an, se gagne à 85 %.

## 11. Les niveaux de Couleurs

Niveau 1 à 10 par Couleur (et par règle fine en V2), permanent, affiché en **fioles d'encre**.
- XP seulement sur les réussites **à l'aveugle ou au boss**.
- Niveau n → n+1 : `12 × n` XP.
- **Un niveau par Couleur toutes les 3 semaines maximum.**
Les règles jamais rencontrées apparaissent en silhouette **?**.

## 12. Les Talismans

Achetés à l'Échoppe, perdus en fin de run. Trois raretés : commun (cire grise), rare (cire bleue), légendaire (cire or, son unique, 1 fois sur 12). **Texte : icône + nom + une ligne de ≤ 8 mots**, la « saveur » est dite par Phalène d'un tap. Textes exacts dans `03` §4.

**V1 (8)** : Perroquet, Chronomètre (dorées, 10 s), Jumeau, Loupe, Gomme, Collectionneur, Aimant, Sourd.
**V2 (12 par exploit)** : Fantôme, Horloge, Bibliothécaire, Couronne, Meute, Écho, Colosse, Miroir, Tambour, Phare, Banquier, Alchimiste.
**Compte-gouttes** : 12 de plus sur l'année, débloqués aux victoires cumulées au boss (4, 9, 15, 22, 30, 39, 49, 60, 72, 85, 99, 114). Sans ce flux, l'ennui devient la première cause d'abandon dès la semaine 15.

**Fusions** (V2) : deux Talismans synergiques possédés dans une run gagnée débloquent leur fusion (un seul emplacement, effets ×1,5) : Jumeau + Loupe → Les Siamois ; Perroquet + Chronomètre → Le Métronome ; Collectionneur + Aimant → L'Archiviste ; Fantôme + Phare → Le Veilleur.

**Plumes** (V2) : trois Plumes proposées au départ de la run, on en prend une (Corbeau : +1 bandeau, −1 écoute ; Oie : Gomme incluse, −1 pièce par manche ; Paon : Accentuées ×1,3, Muettes ×0,8 ; Fer : manche 1 en phrase, manche 1 ×2).

**Pacte** (V2, dès Argent) : contraintes cochées avant la run contre plus de cible et plus de pièces.

## 13. Les boss

Modificateur de la manche 3 dès Bronze. Le boss de la semaine est fixé pour tous ; les autres restent tirables.

| Boss | Effet | Compétence | Version |
|---|---|---|---|
| Le Chuchoteur | le mot n'est dit qu'une fois | attention | V1 |
| Le Brouillon | le mot apparaît 1 seconde puis disparaît, on l'écrit de mémoire | photographie mentale | V1 |
| Le Voleur d'accents | les lettres accentuées valent 0 | le score vient d'ailleurs | V2 |
| Le Pressé | 15 secondes par mot | automatisation | V2 |
| Le Miroir | chaque phrase contient un Sosie piégé | homophones | V2 |

Chaque boss dit une ligne à l'arrivée et une à la défaite (`03` §5).

## 14. Autour de la run

- **L'Enluminure** (victoire du boss, 6 s, non désactivable) : ralenti sur la dernière lettre, le mot se réécrit **en cursive et en or**, les créatures du deck apparaissent en cercle, musique à pleines couches, Phalène se pose.
- **Le Cahier** (optionnel, activé par défaut) : après l'Enluminure, Phalène propose : « Écris tes 3 dorées sur ton cahier. » Un bouton *C'est fait* que le parent peut confirmer plus tard. +50 jetons de bonus la prochaine run. C'est le pont vers l'écriture manuscrite.
- **La carte-résultat** : le récap génère une image : 18 carrés aux Couleurs (juste = plein, faux = vide, aveugle = bord or), le score, le rang. À montrer au parent.
- **La vraie dictée** : le parent saisit le résultat mot par mot. Bonus fixe pour l'avoir faite, bonus par mot du deck juste. **Jamais de perte.**
- **L'atelier se meuble** : chaque rang ajoute un objet sur l'écran d'accueil ; les 10 dernières dorées sont posées sur le bureau.
- **Saisons** : trois par an (toutes les 12 semaines) : nouveaux boss, cadres, Talismans, fenêtre de l'atelier.
- **Prestige** : 8 victoires à Diamant → retour à Argent avec badge permanent, tout le reste conservé.

## 15. Exploits

~30, la moitié en **???**. Chacun est une compétence orthographique. Ils débloquent Talismans, cadres, titres. Liste dans `03` §6.

## 16. Ce qu'on ne fait pas

Pas de série quotidienne. Pas de notification à l'enfant. Pas de récompense de connexion. Pas de classement au score brut (V3 : progression seulement). Pas de bonus de vitesse hors dorées. Pas d'orthographe fausse. Pas de session sans fin. Pas de texte de plus de 8 mots sans bouton de lecture. Pas de chiffres là où des points suffisent.

## 17. Panneau parent et comptes

- **Compte parent** : e-mail + mot de passe, **TOTP** obligatoire pour le panneau. Le TOTP ne sert jamais au quotidien de l'enfant.
- **Profil enfant** : pseudo (créé par le parent) + **motif de 4 points** sur une grille 3×3. Aucune donnée personnelle. Plusieurs profils par parent.
- **Limite de temps** : 20 min/jour par défaut, réglable par jour ; sablier discret ; à zéro, écran chaleureux, run sauvegardée et reprise le lendemain ; temps bonus depuis le téléphone du parent.
- **Saisie de la semaine (< 2 min)** : collage de la liste → détection des Couleurs et règles → 2 à 3 phrases à trou par mot générées → écran de validation corrigeable d'un tap → **enregistrement de la voix du parent** (mots et phrases, 2 minutes) → date de la dictée. Trois types de contenu : mots seuls, phrases avec mots cibles surlignés, dictée complète découpée en segments.
- **La voix du parent est le mode normal.** À 7 ans, une voix synthétique se comprend moins bien et n'a pas la même chaleur. La synthèse (Piper) n'est que le repli.
- **Bouton « ajouter ce mot »** en trois secondes, depuis n'importe où → deck permanent « mes mots ».
- **Rappel** au parent le dimanche soir si la liste n'est pas saisie.
- **Tableau de bord** : dorées, rangs, fioles des Couleurs, temps joué, résultats de dictée, et le message : *« Trois sessions courtes par semaine valent mieux qu'une longue. »* (la fréquence prédit l'abandon bien plus que le niveau, en simulation).

## 18. Roadmap

**V1 — le cœur (60 à 80 h)** : run complète 3 × 6, score, combo, 2 bandeaux, salles Échoppe/Repos/Encrier, 8 Talismans, boss Chuchoteur et Brouillon, cibles et rangs Blanc→Or avec descente et Bienveillance, Rencontre, Revanche, dorées/ternies/maudites/brillantes, Garde 3, Bestiaire minimal, fioles des 6 Couleurs, Phalène (60 lignes, voix TTS), Enluminure, Cahier, carte-résultat, clavier intégré, 12 sprites (6 × 2 stades), 6 objets d'atelier, panneau parent complet, détection des Couleurs, un binaire Go.
**Un mois de jeu réel avant toute V2.**

**V2 — la profondeur** : règles fines et silhouettes, familles et stade Enluminée (18 sprites), exploits, 12 Talismans, tous les boss, Antre et Bibliothèque, fusions, Plumes, Pacte, Platine et Diamant, Prestige, première saison, fantôme de soi-même, duel local à deux.

**V3 — les copains** : partage de liste par code, groupes, classement par progression, titres variés, pièges créés par l'enfant.

## 19. Métriques à suivre dès la V1

Par tentative : mot, manche, aveugle, copie, juste, texte tapé, millisecondes, jetons, mult. Agrégats : échec par manche et par rang, répartition des rangs, dorées par semaine, temps par mot, part de l'aveugle, revanches. Cibles après 3 semaines réelles : M1 ≤ 3 %, M2 ≈ 10 %, boss 15-25 %, aveugle 25-35 %, temps médian par mot ≤ 25 s.
