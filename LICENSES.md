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
