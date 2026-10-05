# Target `systemd`

Émet un unit `.service` par service : `<project>-<name>.service`.
Chaque unit pilote un conteneur via `docker run --rm` — aligné avec
l'adaptateur `native` d'ORION (un service = un cycle de vie conteneur
géré par systemd).

## Mapping `Infra → unit`

| Champ model | Directive systemd |
|---|---|
| `services.<n>.image` | `ExecStart=/usr/bin/docker run ... <image>` |
| `services.<n>.command` | arguments après l'image dans `ExecStart` |
| `services.<n>.ports` | `-p host:container[/proto]` |
| `services.<n>.environment` | `Environment=K=V` + `-e K` |
| `services.<n>.depends_on` | `After=` + `Requires=` sur `<project>-<dep>.service` |
| `services.<n>.volumes` | `-v src:dst` |
| `services.<n>.networks` | `--network <net>` |
| `services.<n>.restart` | `Restart=` (`unless-stopped`→`always`, `no`→`no`) |
| `secrets` | `EnvironmentFile=-/etc/isorion/<project>-secrets.env` + exemple `.env.example` |

## Perdu (warnings émis)

- `services.<n>.build` — les units lancent des images (`docker build` avant).
- `services.<n>.replicas != 1` — un unit = un conteneur (template
  `<svc>@.service` à faire à la main).
- `services.<n>.labels` — pas d'équivalent systemd.
- Champs `Extra` — warning explicite par champ.

## Exemple minimal

```bash
isorion compile app.isor -t systemd -o units/
sudo cp units/*.service /etc/systemd/system/
sudo cp units/webapp-secrets.env.example /etc/isorion/webapp-secrets.env
sudo chmod 600 /etc/isorion/webapp-secrets.env
sudo systemctl enable --now webapp-api.service
```
