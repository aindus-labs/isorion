# 🎯 CHANTIER I6 : Relation ISORION ↔ ORION

**Priorité :** 🔴 Clarté critique (collision de nommage)
**Statut :** ✅ Documenté
**Responsable :** Équipe Lang

---

## 📋 LA COLLISION — à clarifier une fois pour toutes

Le nom « ISORION » et l'extension `.isor` désignent **deux choses
différentes**. Ce chantier existe pour que personne — humain ou agent —
ne les confonde.

| | ISORION (ce projet) | le `.isor` d'ORION |
|---|---|---|
| **Rôle** | Compilateur d'infrastructure universel | Langage d'intention d'ORION |
| **Grammaire** | `version:`, `variables:`, `services:`, `@if/@for` | `capsule:`, `intent:`, `payload:`, `graph:` |
| **Sortie** | Artefacts de cibles (compose, k8s, ansible…) | Capsule signée `.ocaps` |
| **Code** | `pkg/lang` + `pkg/model` + `pkg/emit` | `orion/pkg/compiler` + `orion/pkg/reactor` |
| **Référence** | `docs/spec/ISORION-YAML-PLUS-v0.1.md` | `orion/docs-orion/ISORION & ORION.md` |

## La cible `huborion` — le pont

`huborion` traduit le modèle normalisé d'ISORION-universel vers la
grammaire `.isor` qu'attend `orion/pkg/compiler` :

- `services:` → `graph.nodes` (entrypoint `docker`)
- `depends_on` → arêtes `type: depends_on`
- `environment`, `ports`, `replicas`, `image` → champs de nœud

```
app.isor  ─isorion fusion -t huborion─▶  app.huborion.isor
app.huborion.isor  ─orion build─▶  app.ocaps  (signée, attestée)
```

## Ce que chacun apporte

**ISORION** = *décrire une fois, émettre partout.*
**ORION** = *exécuter en prouvant que ça correspond à l'intention signée.*

`huborion` est la seule cible qui ajoute : signature Ed25519, racines de
confiance, sandbox (namespaces/seccomp), traçabilité (tlog/journal/
witness), attestation. Les 4 autres cibles déploient ; la cible ORION
déploie **et atteste**.

## Discipline de nommage

- Dans ISORION : « le dialecte ORION » ou « cible huborion » — jamais
  « .isor » seul quand le contexte est ambigu.
- Dans ORION : « le langage `.isor` » ou « ISORION-dialect » — jamais
  « ISORION » seul pour le compilateur universel.
- Les deux repos sont indépendants : Orion ne dépend pas d'isorion ;
  il consomme la sortie `huborion` en entrée.
- **Convention d'extension normative** (spec §6.1, décision 2026-10-05) :
  `*.isor` = YAML++ universel ; `*.huborion.isor` = dialecte ORION
  **généré** (jamais édité à la main — artefact, ré-importable via
  `isorion import --from huborion`). Option « deux schémas assumés »
  retenue ; l'unification via `model.Infra` a été écartée (pivot lossy :
  `intent`, `security`, `payload`, edges typés hors périmètre).

## Ouverture future

Le reverse-compiler (I4) rend la relation **bidirectionnelle** : une
capsule/manifeste ORION pourrait être re-capturée en `.isor` universel —
bouclant la boucle infra réelle ↔ intention ↔ capsule.
