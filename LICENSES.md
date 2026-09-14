# Licences

Le code d'`encre` est sous licence MIT — voir [LICENSE](LICENSE).

Les données embarquées ont leur propre licence, qui voyage avec elles.

## Lexique 3.83

`lexique/data/lexique.tsv.gz` est dérivé de **Lexique 3.83**, base de données
lexicales du français.

- Auteurs : Boris New, Christophe Pallier, Ludovic Ferrand, Rafael Matos,
  Ronald Peereman, Sophie Dufour, Christian Lachaud et autres contributeurs.
- Source : <http://www.lexique.org> — <http://www.lexique.org/databases/Lexique383/>
- Licence : **CC BY-SA 4.0** (<https://creativecommons.org/licenses/by-sa/4.0/>),
  telle que déclarée par le `README-Lexique.txt` de l'archive distribuée.

Publications à citer :

- New, B., Pallier, C., Brysbaert, M., & Ferrand, L. (2004). *Lexique 2: A New
  French Lexical Database.* Behavior Research Methods, Instruments, & Computers,
  36(3), 516–524.
- New, B., Pallier, C., Ferrand, L., & Matos, R. (2001). *Une base de données
  lexicales du français contemporain sur internet : LEXIQUE.* L'Année
  Psychologique, 101(3), 447–462.

**Ce que le partage à l'identique implique ici.** La clause *ShareAlike* porte
sur la base de données et ses dérivés, pas sur le programme qui la lit : le code
d'`encre` reste sous MIT. Le fichier `lexique/data/lexique.tsv.gz` — un extrait
filtré et augmenté d'une colonne `famille` — reste sous CC BY-SA 4.0, et toute
copie ou modification de ce fichier doit être redistribuée sous la même licence
avec la présente attribution.

**Comment il est fabriqué.** Le fichier n'est pas recopié tel quel :
`cmd/lexique-gen` garde les 20 000 formes les plus fréquentes, fusionne les
homographes, et calcule pour chaque mot un mot de la même famille où la lettre
finale muette s'entend (chat → chatte). Pour le régénérer :

```bash
mise run lexique:build   # nécessite Lexique383.tsv dans le dossier courant
```

## Ark Pixel

`client/assets/fonts/ArkPixel10-Latin.ttf` et `ArkPixel16-Latin.ttf` sont La
Greffe et La Plume de brief/ENCRE_02 §5, choisies par ticket encre-amh après
avoir téléchargé et testé chaque candidate avec `client/ui.MissingGlyphs`.

- Auteur : TakWolf (<https://takwolf.com>).
- Source : <https://github.com/TakWolf/ark-pixel-font>, release `2026.09.01`,
  variantes `10px` et `16px` proportionnelles, sous-jeu `latin`.
- Licence : **SIL Open Font License 1.1**
  (<https://scripts.sil.org/OFL>), telle que publiée par le projet à
  <https://github.com/TakWolf/ark-pixel-font/blob/master/LICENSE-OFL>. Le
  texte exact accompagne les fichiers de police dans
  `client/assets/fonts/OFL-ArkPixel.txt`.
- **Pas de Reserved Font Name** : Ark Pixel n'en déclare aucun, donc les
  fichiers embarqués gardent leur nom d'origine même après sous-ensemblage.

**Ce que voyage avec le fichier.** L'OFL exige que la licence accompagne toute
copie ou dérivé de la police ; `OFL-ArkPixel.txt` reste dans le même dossier
que les `.ttf` pour cette raison, même si le binaire du client n'embarque que
les octets de la police elle-même.

**Comment ils sont fabriqués.** Chaque fichier est un sous-ensemble de la
police d'origine, réduit avec `pyftsubset` (fonttools) aux runes que le
clavier dessiné peut produire (`ui.RequiredRunes(ui.Phone) +
ui.RequiredRunes(ui.AZERTY)`), à la chaîne d'acceptation d'ENCRE_02 §5, aux
chiffres, à la ponctuation courante et à l'espace — vérifié avec
`client/ui.MissingGlyphs` avant et après le sous-ensemblage. La grille native
de chacune (10 px, 16 px) borne les tailles auxquelles `client/ui.Registry`
a le droit de les charger (ENCRE_02 §15) : voir `ui.GreffeGrid` / `ui.PlumeGrid`.

## Marelle (embarquée sous le nom EncreCursive)

`client/assets/fonts/EncreCursive.ttf` est La Cursive de brief/ENCRE_02 §5 —
l'Enluminure et l'écran Cahier, rien d'autre.

- Auteurs : Ministère de l'Éducation nationale, de l'Enseignement supérieur
  et de la Recherche, Laurent Bourcellier, Jonathan Fabreguettes et Rosalie
  Wagner.
- Source : <https://marelle.forge.apps.education.fr/>, TTF v1.005 :
  `https://forge.apps.education.fr/api/v4/projects/10339/packages/generic/marelle/v1.005/marelle-ttf.zip`.
- Licence : **SIL Open Font License 1.1** (<https://scripts.sil.org/OFL>),
  avec le **Reserved Font Name « Marelle »**. Le texte exact de la licence
  distribuée par le ministère accompagne le fichier dans
  `client/assets/fonts/OFL-Marelle.txt`.

**Pourquoi le fichier embarqué ne s'appelle pas Marelle.** Le nom « Marelle »
est un Reserved Font Name au sens de l'OFL : une version modifiée ne peut pas
le porter. `EncreCursive.ttf` **est** une version modifiée — un
sous-ensemble produit par `pyftsubset` sur les mêmes runes qu'Ark Pixel —
donc elle est renommée : la table `name` de la police (familles, nom complet,
nom PostScript) a été réécrite de « Marelle » vers « EncreCursive » avec un
script `fontTools.ttLib` (`pyftsubset` ne touche pas ces champs), et l'entrée
de copyright (nameID 0) précise qu'il s'agit d'une version modifiée et
renommée, dérivée de la police originale sous OFL, et cite le ministère de
l'Éducation nationale comme le fait sa propre notice de copyright ci-dessus.
L'OFL permet cette modification et cette redistribution ; elle interdit
seulement de garder le nom réservé sur un fichier qui n'est plus l'original
bit pour bit.

**Ce qui n'est pas embarqué dans le binaire du client.** La recherche a
aussi téléchargé, sous-ensemblé et vérifié `MarelleLIGNES` (le lignage Seyes
de l'écran Cahier, en glyphes SVG couleur — qu'Ebitengine v2.10 sait
effectivement rendre) et `MarelleBaton` (capitales bâtons). Les deux couvrent
les 47 runes obligatoires ; leurs sous-ensembles et licences vivent à côté
des autres dans `client/assets/fonts/` pour le jour où l'écran Cahier existe,
mais ils ne sont **pas** `//go:embed`és : rien ne les consomme encore, et le
budget WASM d'ENCRE_04 §2 est déjà tendu par le coût du moteur de mise en
forme de texte qu'embarquer la première police réelle a révélé — voir la note
du ticket encre-amh. Les embarquer sans usage n'aurait fait que payer ce coût
en pure perte.
