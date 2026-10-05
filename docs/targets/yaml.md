# Target `yaml`

Émet `resolved.yaml` — le `model.Infra` sérialisé, tout YAML++ résolu
(directives exécutées, expressions évaluées, `@include` appliqués).
C'est le format pivot : machine-readable, diffable, rejouable.

## Mapping

Identité — `resolved.yaml` **est** `model.Infra` : `version`, `project`,
`services`, `networks`, `volumes`, secrets en marqueurs.

## Perdu

- Les directives YAML++ (`@if`, `@include`, expressions) — déjà
  résolues à l'émission ; le fichier est une forme finale.
- Les contraintes `constraints:` — consommées par `model.FromBody`.

## Round-trip

L'importer `yaml` (`pkg/parse/yaml.go`) réinjecte le document dans
`model.FromBody` — round-trip **exact** par construction, y compris les
marqueurs `isorion-secret://`.

## Exemple minimal

```bash
isorion fusion app.isor -t yaml -o out/        # forme résolue
isorion import out/resolved.yaml -f yaml       # retour modèle
isorion validate out/resolved.yaml             # directement validable
```
