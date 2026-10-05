# ISORION YAML++ — Spécification du langage v0.1 (brouillon)

ISORION est un langage **YAML++** : un fichier `.isor` est un document YAML
valide étendu par des variables, des directives et des expressions. Tout ce
qui n'est pas une directive est du YAML standard à 100 %.

## 1. Structure du document

```yaml
version: "1.0"          # version du format du document (chaîne)

variables:              # optionnel : déclarations de variables
  name: value           # référencées via $name dans le reste du document

project:                # optionnel : métadonnées normalisées du projet
  name: my-app
  description: ...
  labels: {...}

services:               # modèle de services normalisé (model.Infra)
  api:
    image: nginx:1.25
    replicas: 3
    ports: ["80:8080"]
    environment: { KEY: value }
    depends_on: [db]
    volumes: [...]
    networks: [...]
    restart: always

networks: { ... }       # optionnel, propagé tel quel
volumes:  { ... }       # optionnel, propagé tel quel
```

Les sections de premier niveau et les champs de service inconnus sont
préservés (pass-through `Extra`) afin que les émetteurs puissent les
transférer quand la cible les supporte.

## 2. Variables

- Déclarées dans `variables:` — les clés peuvent s'écrire `env` ou `$env`
  (équivalent).
- Évaluées **séquentiellement** : une variable peut référencer les précédentes.
- Référencées par `$name` (scalaire entier → type natif) ou `$name`/`${expr}`
  à l'intérieur des chaînes (→ interpolation en chaîne).
- Échappement : `$$` produit un `$` littéral.
- Un scalaire entier `$name` préserve le type (int/bool/liste) ; `${expr}`
  évalue une expression quelconque et ne préserve le type que s'il couvre
  la totalité du scalaire.

### `profiles:` — variantes nommées de variables

Une section top-level `profiles:` déclare des jeux de variables alternatifs,
activés par `isorion fusion|validate --profile <name>`. Les variables du
profil s'évaluent contre `variables:` (et entre elles, séquentiellement),
puis **fusionnent par-dessus** `variables:` :

```yaml
variables:
  env: "dev"
  replicas: 1

profiles:
  prod:
    env: "production"
    replicas: ${$replicas + 4}
```

`--profile prod` → `env="production"`, `replicas=5`.

### Injection CLI — `--set`

`--set name=value` (répétable) injecte un littéral **typé YAML**
(`--set n=7` → entier, `--set env=staging` → chaîne) après les profiles —
précédence la plus haute. Disponible sur `fusion` et `validate`.

Ordre de précédence : `variables:` < `--profile` < `--set`.

## 3. Directives

Les directives sont des **clés** commençant par `@`. Elles sont prétraitées
avant le parsing YAML ; dans les fichiers sources, elles peuvent s'écrire
sans guillemets.

### `@if <expr>` / `@elif <expr>` / `@else`

En **contexte de mapping**, le bloc de la directive (un mapping) est fusionné
dans le mapping parent quand la condition est vraie :

```yaml
services:
  api:
    image: nginx:1.25
    @if $env == "production":
      replicas: 5
      restart: always
    @elif $env == "staging":
      replicas: 2
    @else:
      replicas: 1
```

En **contexte de séquence**, un élément directive à clé unique épisse sa
valeur (une séquence → éléments ajoutés, autre → élément unique ajouté) :

```yaml
ports:
  - "80:80"
  - @if $tls:
      - "443:443"
```

Règles : `@elif`/`@else` ne sont légaux qu'immédiatement après un groupe
`@if`/`@elif` (avant toute clé/élément normal) ; un `@else` sans `@if`
correspondant est une erreur.

### `@for $var in <expr>`

`<expr>` doit s'évaluer en une liste. En **contexte de mapping**, le bloc
(un mapping gabarit) est évalué une fois par élément puis **fusionné** —
ce qui génère plusieurs clés :

```yaml
services:
  @for $region in $regions:
    "api-${region}":
      image: myorg/api:latest
      environment: { REGION: $region }
```

En **contexte de séquence**, la valeur expansée du bloc est ajoutée à chaque
itération (les séquences sont épissées).

## 4. Expressions

Utilisées dans les conditions `@if`/`@elif`, les listes d'itération `@for`,
et `${...}`.

| Catégorie | Syntaxe |
|---|---|
| Littéraux | `"str"`, `'str'`, `123`, `1.5`, `true`, `false`, `null` |
| Variables | `$name` |
| Comparaison | `==`, `!=`, `<`, `<=`, `>`, `>=` (numérique si les deux opérandes sont numériques, sinon lexicographique) |
| Logique | `&&`/`and`, `\|\|`/`or`, `!`/`not` (court-circuit) |
| Arithmétique | `+` (addition numérique ou concaténation), `-`, `*`, `/` |
| Groupement | `( expr )` |
| Fonctions | `env("NOM")`, `len(x)`, `upper(s)`, `lower(s)`, `default(x, y)`, `int(x)`, `string(x)` |

Les identifiants nus dans les expressions se comportent comme des littéraux
chaîne : `$mode == fast` est légal.

Véracité (truthiness) : `false`/`null`/chaîne vide/`0`/liste vide/map vide
sont faux ; `"false"` et `"0"` sont aussi des chaînes fausses.

## 5. Contrat du modèle (infrastructure normalisée)

Les émetteurs traduisent le modèle normalisé, pas le YAML brut. Champs de
service :

| Champ | Type | Remarques |
|---|---|---|
| `image` | chaîne | référence d'image OCI (image ou build requis) |
| `build` | chaîne | contexte de build |
| `command` | liste\|chaîne | argv |
| `replicas` | int | défaut 1 |
| `ports` | liste | `"80"`, `"80:8080"`, `"8080/udp"`, `"127.0.0.1:80:8080"` |
| `environment`/`env` | map | variables d'environnement |
| `depends_on` | liste | noms de services → arêtes pour les cibles à graphe |
| `volumes`, `networks` | liste | références vers les sections de premier niveau |
| `restart` | chaîne | politique de redémarrage |
| `labels` | map | métadonnées |
| *(autre)* | any | préservé en pass-through `Extra` |

## 6. Relation avec ORION — collision de nommage

> **Attention** : le nom « ISORION » et l'extension `.isor` désignent
> **deux langages différents**.
>
> | | Ce projet (`isorion/`) | Le langage d'ORION (`orion/`) |
> |---|---|---|
> | Rôle | Compilateur d'infrastructure universel | Langage & compilateur d'intention |
> | Grammaire | `version:`, `variables:`, `services:`, `@if`/`@for` | `capsule:`, `intent:`, `payload:`, `graph:` |
> | Sortie | Manifestes de cibles (compose, k8s, ansible…) | Capsule signée `.ocaps` via `orion build` |
> | Référence | [isorion.org](https://isorion.org/) | `docs-orion/ISORION & ORION.md` |
>
> La cible `huborion` est le **pont** entre les deux : elle traduit le modèle
> normalisé d'ISORION-universel vers la grammaire `.isor` qu'attend
> `orion/pkg/compiler` — `services:` deviennent des `graph.nodes`
> (entrypoint `docker`), `depends_on` devient des arêtes
> `type: depends_on`.

Pipeline complet :

```
app.isor (ce dialecte)  --isorion fusion -t huborion-->  app.huborion.isor
app.huborion.isor       --orion build-->                 app.ocaps (capsule signée)
```

### 6.1 Convention d'extension — OFFICIELLE

Pour lever l'ambiguïté `.isor`, la convention suivante est normative :

| Extension | Contenu | Produit par | Écrit à la main ? |
|---|---|---|---|
| `*.isor` | YAML++ universel (ce document) | humain, ou cible `isor` (`isorion import` / `fusion -t isor`) | **oui** |
| `*.huborion.isor` | dialecte ORION (`capsule:`, `graph:`, `intent:`…) | **uniquement** `fusion -t huborion` | **non** — artefact généré |
| `*.ocaps` | capsule ORION signée | `orion build` | non |

Règles :

- Un fichier `*.isor` sans infixe est **toujours** du YAML++ universel. Le
  dialecte ORION généré porte **obligatoirement** l'infixe `.huborion`.
- `*.huborion.isor` est un **artefact de compilation** : on ne le modifie
  pas à la main (la source de vérité est le `*.isor` universel), comme on
  ne modifie pas un `.o`. Il reste lisible et ré-importable
  (`isorion import --from huborion`) pour le round-trip.
- Justification : le dialecte ORION n'est jamais rédigé par un humain —
  une convention d'extension suffit ; il n'y a pas de seconde courbe
  d'apprentissage. Si un besoin d'édition manuelle du dialecte émerge,
  cette décision sera réouverte (voir `mission/CHANTIER-I6`).

## 7. État du chantier (reprise par un agent)

État réel au 2026-10-05 — **ce qui existe vs ce qui est roadmap**.

### Implémenté et testé (`go test ./...` vert)

| Émetteur | Sortie | Fichier |
|---|---|---|
| `yaml` | `resolved.yaml` (référence expanded) | `pkg/emit/yaml.go` |
| `docker-compose` | `docker-compose.yml` | `pkg/emit/docker_compose.go` |
| `kubernetes` | `<svc>-deployment.yaml` + `<svc>-service.yaml` (sous `output/kubernetes/`) | `pkg/emit/kubernetes.go` |
| `ansible` | `playbook.yml` (community.docker) | `pkg/emit/ansible.go` |
| `huborion` | `<project>.huborion.isor` (dialecte ORION) | `pkg/emit/huborion.go` |
| `helm` | chart `Chart.yaml`/`values.yaml`/templates | `pkg/emit/helm.go` |
| `terraform` | `main.tf` (HCL) | `pkg/emit/terraform.go` |
| `systemd` | unit files | `pkg/emit/systemd.go` |
| `isor` | document `.isor` canonique (back end du reverse-compiler) | `pkg/emit/isor.go` |

Sens inverse : `pkg/parse` (importers, miroir de `pkg/emit` — interface
`Importer` + registre) — 8 formats : `docker-compose`, `kubernetes`
(multi-doc/`List`), `huborion`, `ansible`, `helm` (values.yaml), `systemd`
(unit docker), `terraform` (forme émise, parseur borné), `yaml`
(= identité). `parse(emit(x)) == x` sur `model.Infra` vérifié par
round-trips sur chaque format.

Pipeline `pkg/lang` (preprocess → expressions → expansion de l'arbre) →
`pkg/model` (`model.Infra` normalisé + `CheckConstraints`) → `pkg/emit`
(registre d'émetteurs + transformers).
CLI `cmd/isorion` : `fusion`, `validate`, `diff`, `fmt`, `import`,
`targets`, `version`.
Flags `fusion`/`validate`/`diff` : `--set k=v` (littéraux typés, précédence
max), `--profile <name>` (section `profiles:`), `fusion --stdout` (mono-cible,
`---` séparés, pipeable vers `kubectl`/`conftest`).
Erreurs positionnées `file:line:col:` (`LineError` + `Document.Positions`,
chaînées à travers `@include`/`@extends` via `fileErr`).
Tests : `pkg/lang`, `pkg/model`, `pkg/emit`, `pkg/parse` couverts, dont **golden tests**
(`go test ./pkg/emit -update` régénère `testdata/golden/`) ; `cmd` couvert
(helpers + commandes e2e sur `t.TempDir`).

### Roadmap (pas implémenté)

`aws`, `azure`, `gcp` — **différés en pro roadmap** (décision I3 :
provider-specific, terraform couvre l'IaaS générique). Évolutif via les
plugins wasm (étude `docs/etudes/PLUGIN-WASM-I7-F.md`, v0.3+). Les contrats
ci-dessous s'appliquent à tout nouvel émetteur/importer.

### Invariants à respecter pour ajouter une cible

1. **Interface** : tout émetteur implémente `emit.Emitter` (`Name()`,
   `Description()`, `Emit(doc *lang.Document, infra *model.Infra) ([]File, error)`)
   et s'auto-enregistre via `Register` dans `init()`.
2. **Modèle seulement** : l'émetteur traduit `model.Infra` (déjà expansé —
   les directives `@if`/`@for` et les `$vars` sont résolues en amont par
   `pkg/lang`). Ne jamais re-parser les directives.
3. **Pass-through** : les champs inconnus de service sont dans `Extra` —
   les propager quand la cible les supporte, sinon les ignorer proprement
   (pas d'erreur).
4. **`depends_on`** : devient la sémantique propre de la cible
   (`depends_on` compose, ordre Ansible, arêtes `type: depends_on` huborion).
5. **Tests** : tout nouvel émetteur a un test de bout en bout dans
   `pkg/emit/emit_test.go` (doc → infra → fichiers émis).
6. **Honnêteté** : tout champ du modèle ignoré par la cible est signalé
   via `Warn` (interface `emit.Warner`) — jamais de perte silencieuse.

### Prochaine étape naturelle

L'émetteur `terraform` (HCL) ou `helm` (chart) — le modèle `Infra` fournit
`replicas`, `ports`, `image`, `env`, `depends_on`/`extra` ; une cible
déclarative HCL mappa proprement dessus. Ou, transverse : corriger la doc
(`isorion.org` en lien externe vs source de vérité interne).

## 8. Extensions du langage (chantier I7)

### 8.1 Champs cachés `_name`

Toute clé commençant par `_` est **résolue mais jamais émise** : elle
disparaît de `Document.Body` après expansion. Usage : gabarits internes,
valeurs réutilisables sans sortie, documentation privée.

```yaml
services:
  api:
    _retry_template: { attempts: 3 }
    image: nginx
```

### 8.2 Contraintes de variables `name!`

Dans `variables:`, un suffixe `!` sur le nom déclare une contrainte vérifiée
sur la valeur finale (après `profiles:` et `--set`) :

```yaml
variables:
  env: "prod"
  "env!": "in: dev|staging|production"   # union littérale
  replicas: 3
  "replicas!": "$it >= 1 && $it <= 10"   # expression, $it = valeur
```

Violation → erreur positionnée `file:line:col`.

### 8.3 Contraintes de document `constraints:`

Section top-level de garde, évaluée sur le body expansé entre le modèle et
l'émission (`model.CheckConstraints`) :

```yaml
constraints:
  - expr: "services.*.replicas >= 1"           # forme chemin, wildcard *
  - expr: "services.*.image != \"\"" 
  - expr: "services.*.healthcheck != \"\""     # chemin manquant = violation
    level: advisory                            # warning, ne bloque pas
```

- **forme chemin** `a.b.*.c <op> <expr>` : évaluée par valeur matchée
  (`$it` = valeur, `$name` = clé wildcard) ;
- **forme expression** : évaluée une fois (`$it` = body entier) ;
- `level` : `mandatory` (défaut, erreur agrégée) ou `advisory` (warning stderr).

### 8.4 Secrets `secrets:` + `secret()`

```yaml
secrets:
  db_password: "ENC[age:...]"   # ciphertext sops/age — plaintext rejeté
services:
  api:
    environment:
      DB_PASS: ${secret("db_password")}
```

`secret("name")` produit une **référence**, jamais la valeur : les émetteurs
la traduisent nativement — kubernetes : `secretKeyRef` + manifeste `Secret`
(`stringData` = blob ENC, un pipeline sops/age déchiffre au déploiement) ;
docker-compose : `${SECRET_<name>}` + bloc `secrets: { external: true }` ;
ansible : `lookup('ansible.builtin.env', 'SECRET_<name>')` ; huborion :
`secret:<name>`.

### 8.5 Composition `@include` / `@extends`

```yaml
# dans une section mapping ou un item de séquence :
@include fragment.isor:
@include lib/net.isor @sha256 <hex>:     # épinglage d'intégrité

# top-level seulement, un seul :
@extends base.isor:
```

- **`@include`** : insère le body expansé du fichier (mapping → fusionné,
  séquence → items épissés). Les `variables:` du fichier inclus sont des
  **défauts** (l'appelant gagne), `secrets:`/`constraints:` remontent au
  document parent. Chemins relatifs au fichier inclus ; cycles détectés ;
  `@sha256` vérifie l'empreinte avant expansion.
- **`@extends`** : le fichier enfant est deep-mergé **sur** le parent
  (l'enfant raffine, jamais l'inverse), `version:` héritée si absente.

### 8.6 Warnings de capacité par cible (emit `Warner`)

Les émetteurs rapportent les champs du modèle **silencieusement ignorés** —
`emit.Run` retourne `files, warnings, err`, la CLI les imprime sur stderr :
`services.db.volumes ignored by target kubernetes`, `services.api.depends_on
ignored by target kubernetes (orchestrator-managed)`, etc.

### 8.7 Cibles composites `base+tx` (emit `Transformer`)

`-t kubernetes+annotate` : `Run` émet via l'émetteur de base puis applique
chaque transformer enchaîné. `annotate` (référence) préfixe chaque fichier
d'un header `# generated by isorion`.

### 8.8 Commandes `diff` et `fmt`

- `isorion diff f.isor [-t targets] [-o dir] [-v]` : compare l'émission avec
  les fichiers existants (`+` nouveau, `~` modifié `+a/-d`, `-v` diff
  unifié), **exit 1** s'il y a des différences.
- `isorion fmt f.isor [-w]` : forme canonique — indentation 2, sections
  préférées en tête (`version`, `variables`, `profiles`, `secrets`,
  `constraints`), clés directives dé-quotées (`"@if x":` → `@if x:`).

### 8.8 Commandes `diff` et `fmt`

- `isorion diff f.isor [-t targets] [-o dir] [-v]` : compare l'émission avec
  les fichiers existants (`+` nouveau, `~` modifié `+a/-d`, `-v` diff
  unifié), **exit 1** s'il y a des différences.
- `isorion fmt f.isor [-w]` : forme canonique — indentation 2, sections
  préférées en tête (`version`, `variables`, `profiles`, `secrets`,
  `constraints`), clés directives dé-quotées (`"@if x":` → `@if x:`).
