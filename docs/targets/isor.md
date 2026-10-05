# Target `isor`

Réémet le modèle `Infra` en document `.isor` **canonique** — la cible « soi »
du compilateur et le back end du reverse-compiler (`isorion import`).

## Mapping `Infra → .isor`

| Champ model | Section .isor |
|---|---|
| `Version` (ou `"1.0"`) | `version:` |
| `ProjectName`/`Description`/`Labels` | `project:` (champs vides omis) |
| `services.<n>` | `services.<n>` — champs connus, `replicas` omis si `1` |
| `Ports` | `ports:` en syntaxe compose (`"h:c"`, `"/udp"`) |
| `Networks`/`Volumes` non vides | `networks:`/`volumes:` |
| `Other` | sections top-level préservées, triées |

## Perdu (par construction)

- Les directives (`@if`, `@for`), variables et expressions — l'émetteur ne
  voit que le modèle expansé (invariant 2). L'output est du YAML++ « plat ».
- `secrets:`/`constraints:` — consommés en amont ; les références restent
  des marqueurs `isorion-secret://` dans `environment` (à re-déclarer sous
  `secrets:` dans le document importé).

## Round-trip

`parse(emit(x)) == x` sur `model.Infra` — vérifié par
`pkg/emit/isor_test.go` (TestIsorRoundTrip) et les importers
`docker-compose`/`kubernetes`/`huborion`.

## Exemple

```bash
isorion import docker-compose.yml            # → .isor canonique sur stdout
isorion fusion app.isor -t isor              # → re-canonicalisation
```
