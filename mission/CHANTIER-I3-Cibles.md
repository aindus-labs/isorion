# 🎯 CHANTIER I3 : Cibles d'infrastructure — liste complète

**Priorité :** 🔴 Haute (croissance du compilateur)
**Statut :** ✅ Clôturé — 9 émetteurs livrés + `pkg/parse` (3 importers) ;
aws/azure/gcp explicitement différés (pro roadmap, décision ci-dessous)
**Responsable :** Équipe Lang

---

## 📋 CIBLES LIVRÉES (test e2e vert)

| Cible | Sortie | Fichier | Notes |
|---|---|---|---|
| `yaml` | `resolved.yaml` | `yaml.go` | Document expanded — référence/diff |
| `docker-compose` | `docker-compose.yml` | `docker_compose.go` | depends_on, ports, env, volumes |
| `kubernetes` | `<svc>-{deployment,service}.yaml` | `kubernetes.go` | replicas→spec, ports→Service |
| `ansible` | `playbook.yml` | `ansible.go` | community.docker |
| `huborion` | `<project>.huborion.isor` | `huborion.go` | dialecte ORION — voir I6 |
| `terraform` | `main.tf` + `variables.tf` | `terraform.go` | provider docker (kreuzwerker) |
| `helm` | `<chart>/` complet | `helm.go` | templates génériques sur `values.services` |
| `systemd` | `<project>-<svc>.service` | `systemd.go` | `docker run` par unit, `depends_on`→After/Requires |
| `isor` | `<project>.isor` | `isor.go` | .isor canonique normalisé — back-end du reverse-compiler |

## ROADMAP — non implémentée (comptée à 0)

| Cible | Artefact visé | Complexité | Notes |
|---|---|---|---|
| `aws` | CloudFormation / CDK | Haute | provider-specific — terraform couvre le chemin IaaS générique |
| `azure` | ARM / Bicep | Haute | provider-specific |
| `gcp` | Deployment Manager / Terraform | Haute | provider-specific |
| `nomad` | `job.nomad` | Moyenne | HCL, proche terraform |
| `dockerfile` | `Dockerfile` | Basse | build-only cible |
| `podman-compose` | `podman-compose.yml` | Basse | proche docker-compose |
| `ecs` | task definitions | Moyenne | AWS spécifique |

> **Décision 2026-10-05** : `aws`/`azure`/`gcp` **ne seront pas des émetteurs
> communautaires** — chaque cloud exige son propre schéma (provider-specific,
> effort par cloud) et le chemin IaaS est déjà couvert par `terraform`
> (modules dédiés possibles côté utilisateur). Statut : **pro roadmap
> différée**, réouverture uniquement sur demande concrète ou via le mécanisme
> de plugins wasm (étude I7-F).
>
> **Règle de comptage** : seul un émetteur avec test e2e vert passe en
> « livré ». Le « 13+ cibles » du marketing est une **vision**, pas un
> état — la doc doit refléter 9 livrées + roadmap.

## Prochaine étape prioritaire

1. **`nomad`** — HCL, réutilise le pattern terraform (proche `model.Infra`).
2. **`podman-compose`** — alias quasi-gratuit de `docker-compose`.
3. ~~**Importer `helm`/`terraform`**~~ — fait : `pkg/parse` couvre les 8
   formats émis (compose, k8s, huborion, ansible, helm, systemd, terraform,
   yaml) — round-trip complet du catalogue testé.
(`aws`/`azure`/`gcp` : voir décision ci-dessus — hors priorité communautaire.)

## Contrat par cible

Chaque émetteur doit documenter dans `docs/targets/<name>.md` :
- le mapping `Infra → artefact` (tableau champ→champ),
- ce qui est perdu (champs `Extra` non supportés),
- un exemple minimal émettable.
