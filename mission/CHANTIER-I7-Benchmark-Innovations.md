# 🎯 CHANTIER I7 : Benchmark — innovations à intégrer

**Priorité :** 🟢 Stratégie produit
**Statut :** ✅ Recherche documentée (2026-10-05)
**Responsable :** Équipe Lang

---

## 📋 CE QUE L'ÉTAT DE L'ART APPORTE

Veille sur les langages/outils de configuration modernes : **Pkl** (Apple),
**KCL** (CNCF), **CUE**, **Jsonnet**, **Dhall**, **Carvel ytt**, **Yoke**
(wasm), **Holos**, **kpt**, **Nyl**, **Timoni**, **Celestra**.

## Innovations classées par valeur/effort pour ISORION

### A. Paramètres externes (`--set`, `-p env=prod`) — HAUTE valeur, FAIBLE effort

Inspiré de Pkl (`read?("prop:x")`, `-p`) et Nyl (profiles). Le CLI injecte
des valeurs dans `variables:` sans éditer le fichier :

```bash
isorion fusion app.isor -t kubernetes --set env=staging
isorion fusion app.isor -t docker-compose --profile prod
```

Implémentation : `--set` surcharge `doc.Variables` après `loadVariables`
(avant expansion) ; `--profile <name>` lit `profiles.<name>` comme overlay
de `variables:`. ~50 lignes dans `cmd/isorion` + `lang.Load`.

### B. Matrice de capacités par cible (avertissements, pas erreurs) — HAUTE valeur

Inspiré de Celestra (warn quand un champ n'est pas supporté par la cible).
Complète le pass-through `Extra` : au lieu d'ignorer silencieusement,
`fusion` **avertit** (« `deploy.placement` ignoré par la cible `ansible` »).

Implémentation : chaque émetteur déclare `Supports() map[string]bool` ou
retourne `[]Warning` dans `Emit` — changement d'interface mineur, gardé
rétrocompatible via type assertion.

### C. Étape de validation/politique entre modèle et émission — MOYENNE valeur

Inspiré de Holos (Generators→Transformers→Validators) et CUE (contraintes).
Une section `constraints:` dans `.isor` vérifiée sur `model.Infra` avant
émission :

```yaml
constraints:
  - "services.*.replicas >= 1"
  - "services.*.image != ''"
```

Implémentation : réutiliser l'évaluateur d'expressions de `pkg/lang` sur le
modèle sérialisé — cohérence linguistique, zéro nouveau langage.

### D. Directive `@include` — import de fichiers épinglés — MOYENNE valeur

Inspiré de Dhall (`freeze` → hash SHA de l'import) et Pkl (`amends`).
`@include "common.isor"` insère un sous-arbre ; option `@include <path> @sha256 <hex>`
épinglant le contenu — aligne ISORION sur la philosophie de provenance d'ORION.

Effort : préprocesseur lit le fichier, vérifie le hash, inline avant parse.

### E. `isorion diff` — HAUTE valeur adoption, FAIBLE effort

Inspiré de Nyl (`render/diff/apply`) : comparer l'émission actuelle à
l'output existant → rend la révision de la fusion auditable (lignes
ajoutées/supprimées par fichier, sans appliquer).

### F. Packages/émetteurs WASM — alignement ORION, effort ÉLEVÉ

Inspiré de Yoke (Flights = wasm via wazero). ORION a déjà un adaptateur
`wasm/` — émetteurs tiers en wasm créerait une symétrie forte et un modèle
de plugin sandboxé. À garder pour la v0.3+.

### G. Intégration GitOps — vision

Inspiré de Holos/Nyl : « rendered manifests pattern » — les artefacts émis
se committent (c'est déjà notre modèle). Ouverture : plugin ArgoCD CMP pour
fusionner à la volée.

---

## Recherche approfondie (vague 2 — Wing, Pkl, policy-as-code, secrets)

### H. Hooks d'émission + plateformes composables (Wing `IPlatform`)

Wing compile via des « platforms » empilables avec hooks
`preSynth`/`postSynth`/`validate`. Pour ISORION : les émetteurs gagnent des
points d'extension `PreEmit`/`PostEmit` — une « cible composite »
(`-t kubernetes+policy-pack`) devient possible sans modifier le langage.
Effort moyen, valeur architecturale (chaîne generate→transform→validate à
la Holos).

### I. Snapshots dorés de compilation (Wing cloud test snapshots)

`isorion fusion` produit des artefacts **déterministes** → tests golden :
`examples/*.isor` × cibles → `testdata/golden/` ; le test régénère et compare
en mémoire. En CI (`CI=1`) → assertion ; en local → régénération. Effort
faible, c'est LA garantie de non-régression d'un compilateur.

### J. Variables typées + contraintes (Pkl typealias/union types)

Pkl contraint les entrées à la déclaration (`typealias Region = "eu"|"us"`,
`Int8`, `matches(regex)`). Transposition YAML++ :

```yaml
variables:
  env: "production"
  "env!": "dev|staging|production"      # contrainte d'union
  replicas: 3
  "replicas!": "int >= 1 && <= 10"      # contrainte d'expression
```

La clé `name!` porte la contrainte évaluée au chargement — réutilise
`EvalExpr`. Erreurs précoces, messages précis. Effort faible.

### K. Champs cachés / privés (Pkl `hidden`, `local`)

Propriétés servant au calcul mais absentes de l'émission. En `.isor` :
préfixe `_` sur les clés (ex. `_tpl:` dans un service) → résolues puis
exclues du modèle/`Extra`. Effort trivial, simplifie les gabarits.

### L. `@extends` — composition par deep-merge (Pkl `amends`)

Au-delà d'`@include` (D) : `@extends "base.isor"` fusionne en profondeur
le document de base — les clés du fichier enfant **raffinent** celles du
parent (jamais l'inverse — philosophie CUE : on ne peut que préciser).
Couvre le cas multi-environnements mieux que `@if` dispersés.

### M. Politiques à niveaux `advisory|mandatory` (Terraform Sentinel/OPA)

La section `constraints:` (C) gagne des niveaux :

```yaml
constraints:
  - expr: "services.*.image != ''"    # mandatory par défaut → erreur
    level: mandatory
  - expr: "services.*.replicas <= 4"
    level: advisory                    # warning, n'empêche pas l'émission
```

Les politiques vivent **dans le fichier** (comme les pre/post-conditions
Terraform) et restent portables entre cibles — l'OPA externe reste possible
en aval (`fusion --stdout | conftest test -`).

### N. Secrets — jamais en clair dans les artefacts (SOPS/age)

Pattern sops-nix/age : section `secrets:` aux valeurs `ENC[age:...]`
(cryptées à l'écriture, jamais déchiffrées par `fusion`), références
`$secret("db_password")` dans `environment:` → les émetteurs produisent
des **références**, pas des valeurs :

- `kubernetes` → `Secret` + `secretKeyRef`
- `docker-compose` → `secrets:`/`env_file` ou `external` secrets
- `huborion` → capability `secrets` (aligné zero-trust ORION)

`isorion secret set <name>` écrit via age ; `ssh-to-age` permet de réutiliser
les clés Ed25519 SSH existantes — cohérent avec l'identité Ed25519 d'ORION.
Effort moyen, valeur sécurité différenciante.

### O. Erreurs avec positions ligne:colonne — DX compilateur

`yaml.v3` conserve `Line`/`Column` sur chaque nœud : propager ces positions
dans toutes les erreurs d'expansion (`@if` invalide, `$var` inconnue) →
`app.isor:23:5: undefined variable $replicass`. Effort faible, DX majeure.

### P. `fusion --stdout` / `-` piping

Écrire sur stdout (cible unique) pour `isorion fusion app.isor -t kubernetes
--stdout | kubectl apply -f -` ou `| conftest test -` — intégration native
des pipelines GitOps/policy existants. Effort trivial.

### Q. `isorion fmt` — formateur canonique

Normalisation d'écriture des `.isor` (indentation 2, clés de directives
normalisées, ordre stable) — gofmt-like. Effort moyen, adoption ++.

## Validation conceptuelle croisée : preflight/inflight (Wing)

Wing sépare **preflight** (compile-time → infra) et **inflight** (runtime →
comportement). C'est exactement la frontière **ISORION/ORION** : `.isor`
est du *preflight pur*, `.ocaps`+Reactor est l'*inflight gouverné*. La
séparation compile-time/runtime validée industriellement confirme que le
pont `huborion` est architecturalement correct — ISORION ne doit jamais
acquérir de sémantique d'exécution.

## Décisions retenues (hors scope)

- **Pkl/CUE comme front-end** : non — YAML++ assume sa position (lisible,
  pas de nouvelle syntaxe). La contrainte C entre dans `.isor`, pas dans
  un langage externe.
- **Immutabilité à la CUE** : partielle — la fusion de mapping dans `@if`
  garde la sémantique « merge » déjà livrée.
- **Jsonnet-style imports de code** : non — `@include` reste de la donnée,
  pas des fonctions.
- **Drift detection sur l'état live** : hors scope — c'est le domaine
  d'ORION (boucle intention→observation→ajustement). `isorion diff` (E)
  compare les artefacts, pas le runtime.
- **Code applicatif (inflight) dans le langage** : Wing le fait, ISORION
  non — frontière prouvée par la séparation preflight/inflight.

## Livrables — ordre suggéré (valeur/effort)

- [x] `--set` / `--profile` dans `fusion` + `validate` (A) — 2026-10-05
- [x] `--stdout` piping (P) — 2026-10-05
- [x] Erreurs ligne:colonne (O) — `file:line:col:` via `LineError`, 2026-10-05
- [x] Snapshots dorés `testdata/golden/` (I) — `go test ./pkg/emit -update`, 2026-10-05
- [x] Warnings de capacité par cible (B) — interface `emit.Warner`,
  `emit.Run` retourne `files, warnings, err`, affichés sur stderr — 2026-10-05
- [x] `constraints:` + niveaux advisory/mandatory (C+M) — section top-level,
  `model.CheckConstraints` sur le body expansé (formes chemin `a.*.b op expr`
  et expression), agrégation des violations — 2026-10-05
- [x] Contraintes de variables `name!` (J) — unions `in: a|b|c` et
  expressions `$it`, vérifiées après profile+set — 2026-10-05
- [x] Champs `_hidden` (K) — préfixe `_` résolu mais exclu de `Body` — 2026-10-05
- [x] `isorion diff` (E) — compare émission vs dir existant, `+`/`~`,
  `-v` diff unifié, exit 1 sur différence — 2026-10-05
- [x] `@include` sha256 + `@extends` deep-merge (D+L) — variables incluses
  = défauts, secrets/constraints remontés, cycles détectés, erreurs
  préfixées du bon fichier (`fileErr`) — 2026-10-05
- [x] `isorion fmt` (Q) — forme canonique : indent 2, sections préférées
  en tête, directives dé-quotées, `-w` in-place — 2026-10-05
- [x] `secrets:` + `$secret()` + émission de références (N) — `secret()`
  fonction d'expression → marqueur `isorion-secret://`, plaintext `ENC[`
  requis ; k8s `secretKeyRef`+`Secret`, compose `${SECRET_x}`+`external`,
  ansible `lookup env`, huborion `secret:` — 2026-10-05
- [x] Hooks `PreEmit`/`PostEmit`, cibles composites (H) — interface
  `emit.Transformer`, cibles `base+tx` (`kubernetes+annotate`), chaînables — 2026-10-05
- [x] Étude plugin wasm (F) — `docs/etudes/PLUGIN-WASM-I7-F.md` : modèle
  plugin-in-process (wazero, WASI sans fs), ABI `isorion-plugin-v1` JSON,
  secrets = marqueurs uniquement ; implémentation repoussée v0.3+ — 2026-10-05
