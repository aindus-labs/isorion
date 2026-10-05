# Target `huborion`

Émet `<project>.huborion.isor` — le dialecte capsule ORION
(`graph.nodes` + `graph.edges`), pont direct vers le runtime ORION
(`orion build`/`run`).

## Mapping `Infra → huborion`

| Champ model | Artefact huborion |
|---|---|
| `project.name` / `description` | `capsule.name` / `intent.description` |
| `services.<n>` | `graph.nodes[]` (`entrypoint: docker`, `params.image`…) |
| `services.<n>.depends_on` | `graph.edges[] depends_on` |
| `services.<n>.environment` secret | `params` → marqueur `secret:` |
| reste des champs | `params` du nœud |

## Perdu

- Formes non représentables dans le dialecte — le nœud `params` reste
  le porte-parole générique.

## Round-trip

L'importer `huborion` (`pkg/parse/huborion.go`) relit `graph.nodes` →
services, `edges` → `depends_on`, `capsule.name`/`intent.description` →
projet. `parse(emit(x)) == x` testé.

## Exemple minimal

```bash
isorion fusion app.isor -t huborion -o out/
orion build out/webapp.huborion.isor && orion run webapp.ocaps
isorion import out/webapp.huborion.isor -f huborion
```
