# Target `kubernetes`

Émet un manifest par service (`<svc>-deployment.yaml`), plus
`<svc>-service.yaml` quand le service publie des ports, et `secrets.yaml`
pour les secrets — sous `output/kubernetes/`. Labels
`app.kubernetes.io/name|part-of` posés partout.

## Mapping `Infra → k8s`

| Champ model | Artefact k8s |
|---|---|
| `project.name` | label `app.kubernetes.io/part-of` |
| `services.<n>.image` | `container.image` |
| `services.<n>.command` | `container.command` |
| `services.<n>.replicas` | `spec.replicas` |
| `services.<n>.ports` | `container.ports` + `Service` ClusterIP |
| `services.<n>.environment` | `env` — secrets → `secretKeyRef` (`<project>-secrets`) |
| `services.<n>.labels` | `metadata.labels` additionnels |

## Perdu (warnings émis)

- `build` — k8s déploie des images poussées.
- `depends_on` — pas d'ordre garanti (initContainers via `Extra`).
- `restart` — `restartPolicy` toujours `Always`.
- `volumes`, `networks` — pas mappés sur la forme de base.

## Round-trip

L'importer `kubernetes` (`pkg/parse/kubernetes.go`) accepte les flux
multi-doc (`---`) et `kind: List` : Deployment/Pod → service, Service →
ports publiés (selector), `secretKeyRef` → marqueur `isorion-secret://`.

## Exemple minimal

```bash
isorion fusion app.isor -t kubernetes --stdout | kubectl apply -f -
isorion import api-deployment.yaml -f kubernetes
```
