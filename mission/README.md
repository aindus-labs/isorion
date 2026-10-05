# ISORION — Notes de chantiers

Index des chantiers du projet ISORION (compilateur d'infrastructure universel).
Même discipline que `orion/mission/` : un chantier = un problème, une solution,
des livrables traçables, un statut honnête.

## Index

| Chantier | Sujet | Statut |
|---|---|---|
| [I1](CHANTIER-I1-Presentation.md) | Présentation d'ISORION — quoi, pour qui, pourquoi | ✅ Documenté |
| [I2](CHANTIER-I2-Compilateur-Universel.md) | Le compilateur : pipeline lang→model→emit, extension | ✅ Livré (5 émetteurs) |
| [I3](CHANTIER-I3-Cibles.md) | Liste complète des cibles infra (livrées vs roadmap) | 🟡 Partiel (5/13+) |
| [I4](CHANTIER-I4-Reverse-Compiler.md) | Reverse-compiler : infra existante → `.isor` | ⏳ Conception |
| [I5](CHANTIER-I5-Innovations.md) | Ce que ISORION apporte que les autres n'ont pas | ✅ Documenté |
| [I6](CHANTIER-I6-Relation-Orion.md) | Collision de nommage + pont `huborion` | ✅ Documenté |
| [I7](CHANTIER-I7-Benchmark-Innovations.md) | Veille innovations (Pkl/CUE/Yoke/Holos…) → features à intégrer | ✅ Recherche documentée |
| [I8](CHANTIER-I8-Catalogue-Cibles.md) | Catalogue exhaustif des cibles infra (~75 nommées, 14 catégories) | ✅ Documenté |

## Règles

- **Honnêteté des statuts** : un émetteur est ✅ quand il a un test e2e vert,
  pas quand son squelette existe. La roadmap n'est jamais comptée livrée.
- **Source de vérité** : la spec langue est `docs/spec/ISORION-YAML-PLUS-v0.1.md`
  (§7 = état chantier pour reprise par un agent).
- **Frontière avec ORION** : ISORION décrit et émet ; ORION signe, vérifie et
  exécute. Le pont est la cible `huborion`, pas une dépendance interne.
