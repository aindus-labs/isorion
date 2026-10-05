# Target `docker-compose`

Émet un `docker-compose.yml` unique — mappage 1:1 de `model.Infra`, cible
de référence du round-trip (`parse(emit(x)) == x`).

## Mapping `Infra → compose`

| Champ model | Artefact compose |
|---|---|
| `services.<n>.image` / `.build` | `image` / `build.context` |
| `services.<n>.command` | `command` |
| `services.<n>.replicas` | `deploy.replicas` (omis si 1) |
| `services.<n>.ports` | `ports` courts (`host:container[/proto]`) |
| `services.<n>.environment` | `environment` — secrets → `${SECRET_x}` + `secrets.external` |
| `services.<n>.depends_on` | `depends_on` (liste) |
| `services.<n>.volumes` | `volumes` |
| `services.<n>.networks` | `networks` + section `networks:` |
| `services.<n>.restart` | `restart` |
| `services.<n>.labels` | `labels` (map) |

## Perdu

- `Extra` non mappable — avertissement via `emit.Warner`.

## Round-trip

L'importer `docker-compose` (`pkg/parse/compose.go`) accepte les formes
courtes ET longues (`ports`, `environment` liste `K=V` ou map,
`build.context`/`build` string). `parse(emit(x)) == x` testé.

## Exemple minimal

```bash
isorion fusion app.isor -t docker-compose -o out/
isorion import out/docker-compose.yml -f docker-compose   # retour .isor
```
