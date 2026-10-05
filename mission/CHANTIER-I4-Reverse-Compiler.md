# 🎯 CHANTIER I4 : Reverse-Compiler — infra existante → `.isor`

**Priorité :** 🟢 Forte valeur (adoption/migration)
**Statut :** ✅ Livré et testé — 8 importers : round-trip complet du catalogue
**Responsable :** Équipe Lang

---

## 📋 PROBLÈME

ISORION compile `.isor` → cibles. Le chemin **inverse** manque : prendre
une infrastructure existante (un `docker-compose.yml`, des manifests k8s,
un playbook) et la remonter en un `.isor` déclaratif — pour ensuite
ré-émettre vers d'autres cibles ou la passer en pipeline ORION.

C'est la **porte d'entrée de migration** : pas besoin de réécrire son
infra, on la *capture*.

## Modèle

```
docker-compose.yml ─┐
k8s manifests     ──┤
playbook.yml      ──┼─►  pkg/parse (importers) ──► model.Infra
terraform .tf     ──┘                                    │
                          pkg/emit/isor.go ◄─────────────┘
                                   │
                                   ▼
                             app.isor (YAML++)
```

Le reverse-compiler est la **symétrie** du compilateur : un *importer*
par source (cible → `model.Infra`) et **un seul** émetteur `isor`
(`Infra` → YAML++). Le modèle normalisé est le pivot dans les deux sens.

## Cas d'usage

- **Migration** : capture d'un compose existant → réémission k8s.
- **Fédération** : plusieurs infra hétérogènes → un seul `.isor`.
- **Passage en capsule** : `infra réelle → .isor → huborion → .ocaps`
  signée — la chaîne la plus forte.
- **Round-trip** : `.isor → cible → .isor` doit être stable (test de
  non-perte).

## Importers prioritaires (par ordre de valeur)

| Source | Raison | Difficulté |
|---|---|---|
| `docker-compose` | cas le plus courant, mappage quasi 1:1 | Basse |
| `kubernetes` | pods/services/deployments → services | Moyenne |
| `huborion` | re-capture du dialecte ORION | Moyenne |

## Questions ouvertes de conception

1. **Fidélité vs normalisation** : garder le `Extra` pour round-trip
   fidèle, ou forcer le modèle canonique ?
2. **Variables** : détecter les valeurs répétées et les factoriser en
   `variables:`/`@for` — ou émettre du YAML++ « plat » ?
3. **Idempotence** : garantir `parse(emit(x)) == x` sur `model.Infra`.

## Livrables

- [x] `pkg/parse/` — package d'importers (miroir de `pkg/emit` : interface
  `Importer` + registre auto-enregistré) — 2026-10-05
- [x] `pkg/emit/isor.go` — émetteur `isor` (Infra → YAML++ canonique ;
  aussi visible via `fusion -t isor`) — 2026-10-05
- [x] Importer `docker-compose` (premier, mappage 1:1 : `deploy.replicas`,
  ports courts/longs, `K=V` listes, `build.context`) — 2026-10-05
- [x] Importer `kubernetes` — flux multi-doc `---` + `kind: List`,
  Deployment/Pod → service, Service → ports publiés (selector),
  `secretKeyRef` → marqueur secret — 2026-10-05
- [x] Importer `huborion` — `graph.nodes` → services, edges `depends_on`,
  `capsule.name`/`intent.description` → projet — 2026-10-05
- [x] Importer `ansible` — tâches `docker_container` → services, préfixe
  `<project>-<svc>` retrouvé, `lookup env SECRET_*` → marqueur — 2026-10-05
- [x] Importer `helm` — `values.yaml` (porteur du modèle) → services,
  `secretKeyRef` → marqueur ; `Chart.yaml` nu → projet — 2026-10-05
- [x] Importer `systemd` — unit `<prefix>-<svc>.service` → service unique
  (`docker run` flags, `Environment=`, After/Requires) — 2026-10-05
- [x] Importer `terraform` — parseur borné de la forme émise (pas un
  parseur HCL général) : `docker_container` → services, `var.secret_*` →
  marqueur, networks/volumes → sections — 2026-10-05
- [x] Importer `yaml` — `resolved.yaml` = identité (`FromBody`) — 2026-10-05
- [x] Test round-trip `.isor → compose → .isor` stable — `parse(emit(x)) ==
  x` sur `model.Infra` (isor + compose, byte-stable au 2e passage) — 2026-10-05
- [x] CLI : `isorion import <file> -f <format>` (stdout ou `-o`) — 2026-10-05
