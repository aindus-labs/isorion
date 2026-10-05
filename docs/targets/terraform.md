# Target `terraform`

Émet du HCL pour le provider `kreuzwerker/docker` — le mapping le plus
direct du modèle `Infra` (conteneurs, réseaux, volumes déclaratifs).
Couvre le cas « infra-as-code local/hôte docker » ; les clouds
aws/azure/gcp passent par des modules dédiés (roadmap).

## Mapping `Infra → HCL`

| Champ model | Artefact Terraform |
|---|---|
| `services.<n>.image` | `docker_container.<n>.image` |
| `services.<n>.command` | `docker_container.<n>.command` |
| `services.<n>.ports` | blocs `ports { internal, external, protocol }` |
| `services.<n>.environment` | `env = [...]` — secrets → `var.secret_*` |
| `services.<n>.depends_on` | `depends_on = [docker_container.<dep>]` |
| `services.<n>.volumes` | blocs `volumes { volume_name, container_path }` |
| `services.<n>.networks` | `networks_advanced { name }` (1er seulement) |
| `services.<n>.restart` | `docker_container.<n>.restart` |
| `services.<n>.labels` | blocs `labels { label, value }` |
| `networks.<n>` | `docker_network.<n>` (nom préfixé `<project>_`) |
| `volumes.<n>` | `docker_volume.<n>` |
| `secrets` + env `${secret(x)}` | `variable "secret_x" { sensitive = true }` dans `variables.tf` |

## Perdu (warnings émis)

- `services.<n>.build` — le provider lance des images, ne build pas.
- `services.<n>.replicas != 1` — `docker_container` est une instance
  unique (utiliser `count` manuellement).
- `services.<n>.networks` au-delà du premier réseau.
- Champs `Extra` non reconnus — warning explicite par champ.

## Exemple minimal

```bash
isorion compile app.isor -t terraform -o out/
cd out && terraform init && terraform plan
```
