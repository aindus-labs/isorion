# Target `helm`

Émet un chart Helm minimal : `Chart.yaml`, `values.yaml` (modèle
normalisé complet), et des templates **génériques** qui itèrent
`.Values.services` — le même chart sert toutes les apps, la donnée
pilote le rendu.

## Mapping `Infra → chart`

| Champ model | Artefact Helm |
|---|---|
| `project.name` | `Chart.name` (minuscules) |
| `project.description` | `Chart.description` |
| `services.<n>` | entrée `values.services.<n>` → Deployment |
| `services.<n>.image` | `container.image` |
| `services.<n>.command` | `container.command` |
| `services.<n>.replicas` | `spec.replicas` (défaut 1) |
| `services.<n>.ports` | `container.ports` + Service ClusterIP si non vide |
| `services.<n>.environment` | `env` — secrets → `secretKeyRef` vers `<chart>-secrets` |
| `secrets:` | `values.secrets` (vides) + `templates/secret.yaml` |

## Perdu (warnings émis)

- `services.<n>.build` — les charts déploient des images prêtes.
- `services.<n>.depends_on` — k8s n'ordonne pas (utiliser initContainers via Extra).
- `services.<n>.restart` — `restartPolicy` est toujours `Always`.
- `services.<n>.volumes` — à déclarer via `Extra`/templates custom.
- `services.<n>.networks` — pas d'équivalent k8s direct.

## Exemple minimal

```bash
isorion compile app.isor -t helm -o charts/
helm install webapp charts/webapp --set secrets.db_password=...
```
