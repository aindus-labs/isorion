# ISORION — Universal Infrastructure Compiler

> Write infrastructure once in `.isor`, deploy to many targets.

ISORION compiles a single declarative **YAML++** source into target-specific
artifacts: Docker Compose, Kubernetes manifests, Ansible playbooks, plain YAML,
and the ORION intention dialect (`.isor` → `.ocaps` capsules).

```
app.isor ──fusion──▶ docker-compose.yml
                  ──▶ kubernetes/*.yaml
                  ──▶ playbook.yml (ansible)
                  ──▶ resolved.yaml (reference)
                  ──▶ app.huborion.isor → orion build → .ocaps
```

## Quick start

```bash
go build -o bin/isorion ./cmd/isorion

isorion fusion examples/app.isor -t docker-compose -o output/app
isorion fusion examples/app.isor -t kubernetes,ansible,yaml
isorion fusion examples/app.isor              # all targets
isorion fusion examples/app.isor -t kubernetes+annotate   # composite target
isorion validate examples/app.isor
isorion diff examples/app.isor -o output      # exit 1 if outputs drift
isorion fmt examples/app.isor                 # canonical form
isorion import docker-compose.yml             # lift existing infra → .isor
isorion targets
```

## YAML++ in 30 seconds

```yaml
version: "1.0"

variables:
  env: "production"

services:
  api:
    image: "nginx:1.25"
    @if $env == "production":
      replicas: 5
    @else:
      replicas: 1
```

- `variables:` → referenced as `$env`, `${expr}` ; `name!` declares a
  constraint (`in: a|b` or `$it >= 1 && $it <= 10`)
- `profiles:` → named variable packs (`--profile prod`)
- `@if` / `@elif` / `@else` — conditional blocks (mapping merge / sequence splice)
- `@for $x in $list` — repeated blocks
- `@include file.isor [@sha256 hex]` — spliced file (fragments, cycle-guarded)
- `@extends base.isor` — document deep-merge, child refines parent
- `_hidden:` keys — resolved, never emitted
- `constraints:` — `services.*.replicas >= 1`, `level: advisory|mandatory`
- `secrets:` — `ENC[...]` ciphertext only; `${secret("name")}` emits native
  references (`secretKeyRef`, `${SECRET_x}`, env lookup — never the value)
- `--set name=value` — typed literal injection, highest precedence
- `--stdout` — pipe one target's output (`| kubectl apply -f -`)
- Per-target capability warnings on stderr (ignored fields are never silent)
- Errors carry `file:line:col` positions, including across includes
- Expressions: comparisons, `&&`/`||`/`!`, `+ - * /`, `env()`, `len()`,
  `upper()`, `lower()`, `default()`, `int()`, `string()`, `secret()`

Full spec: [docs/spec/ISORION-YAML-PLUS-v0.1.md](docs/spec/ISORION-YAML-PLUS-v0.1.md)

## Targets

| Target | Output | Status |
|---|---|---|
| `yaml` | `resolved.yaml` — expanded document | ✅ |
| `docker-compose` | `docker-compose.yml` | ✅ |
| `kubernetes` | `<svc>-deployment.yaml` + `<svc>-service.yaml` | ✅ |
| `ansible` | `playbook.yml` (community.docker) | ✅ |
| `huborion` | `<project>.huborion.isor` (ORION dialect) | ✅ |
| `helm` | chart `Chart.yaml`/`values.yaml`/templates | ✅ |
| `terraform` | `main.tf` (HCL) | ✅ |
| `systemd` | unit files | ✅ |
| `isor` | canonical `.isor` (reverse-compiler back end) | ✅ |
| aws, azure, gcp, … | — | Pro roadmap |

**Import** (reverse-compiler) : `isorion import <file> -f <format>` lifts
existing infra back to a canonical `.isor` — formats: `docker-compose`,
`kubernetes` (multi-doc/`kind: List`), `huborion`, `ansible`, `helm`
(values.yaml), `systemd`, `terraform`, `yaml`. Round-trips
`parse(emit(x)) == x` are tested for every emitted format.

## Interfaces (3 levels)

- **Go library** — `import "github.com/isorion-lang/isorion/pkg/isorion"`:
  `Compile`, `Import`, `Validate`, `Targets`, `Importers`. The CLI is a
  thin client over this package; emitters/importers extend via the
  `emit.Emitter` / `parse.Importer` registries.
- **CLI** — `fusion · validate · diff · fmt · import · serve · targets ·
  version`; `fusion --stdout` pipes to `kubectl`/`conftest`.
- **HTTP API** — `isorion serve -addr :8080` exposes JSON endpoints:
  `POST /v1/compile` `{source, targets[]}` → `{files[], warnings[]}`,
  `POST /v1/import` `{format, data}` → `{isor}`, `POST /v1/validate`,
  `GET /v1/targets|importers|version`, `GET /healthz`. Stdlib-only,
  read-only compiler — no state, no auth by design (put it behind your
  own proxy).

## Project layout

```
cmd/isorion/    CLI (fusion, validate, diff, fmt, import, serve, targets, version)
pkg/isorion/    Public API: Compile/Import/Validate facade + HTTP handler
pkg/lang/       YAML++ front-end: preprocess, expressions, tree expansion,
                multi-file composition (@include/@extends), secrets
pkg/model/      Normalized infrastructure model + constraint checking
pkg/emit/       Target emitters, capability warnings (Warner),
                transformers (composite targets base+annotate)
pkg/parse/      Importers — the mirror of emit: source formats → Infra
examples/       .isor samples
docs/spec/      Language specification
```

## Relation to ORION

ISORION (universal compiler) and the `.isor` dialect of
[ORION](../orion) (intention → signed capsule `.ocaps`) are two grammars on the
same extension. The `huborion` target bridges them: services become graph
nodes consumable by `orion build`.

## License

Apache-2.0 — © OFONTAINES
