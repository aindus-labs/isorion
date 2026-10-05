# Target `ansible`

Émet un `playbook.yml` d'un play, une tâche
`community.docker.docker_container` par service — hôte `localhost`,
état `present`.

## Mapping `Infra → playbook`

| Champ model | Module ansible |
|---|---|
| `project.name` + `<svc>` | container `name: <project>-<svc>` |
| `services.<n>.image` | `image` |
| `services.<n>.command` | `command` |
| `services.<n>.ports` | `ports` (`host:container[/proto]`) |
| `services.<n>.environment` | `env` — secrets → `lookup('ansible.builtin.env','SECRET_x')` |
| `services.<n>.volumes` | `volumes` |
| `services.<n>.restart` | `restart_policy` |

## Perdu (warnings émis)

- `replicas` — `docker_container` = instance unique.
- `depends_on` — ordre des tâches seulement, pas de dépendance runtime.
- `build`, `networks` — non mappés.

## Round-trip

L'importer `ansible` (`pkg/parse/ansible.go`) relit les tâches
`docker_container`, retrouve le préfixe `<project>-` commun aux noms,
et reconstitue les marqueurs secrets depuis `lookup env SECRET_*`.
`depends_on` est une perte connue (non émis).

## Exemple minimal

```bash
isorion fusion app.isor -t ansible -o out/
SECRET_db_password=... ansible-playbook out/playbook.yml
isorion import out/playbook.yml -f ansible
```
