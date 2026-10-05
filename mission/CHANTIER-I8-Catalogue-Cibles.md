# 🎯 CHANTIER I8 : Catalogue complet des cibles d'infrastructure

**Priorité :** 🔴 Référentiel produit
**Statut :** ✅ Inventaire documenté (2026-10-05)
**Responsable :** Équipe Lang

---

## 📋 OBJET

Liste **exhaustive** des formats d'infrastructure qu'ISORION doit prendre en
compte — l'univers « universel » n'est pas vague, il est inventoriable.
Cette note est la source de vérité des cibles ; [I3](CHANTIER-I3-Cibles.md)
reste le chantier d'implémentation.

Règles de statut (identiques à I3) :

- ✅ **livré** = émetteur avec test e2e vert
- 🟡 **roadmap** = prévu Community ou Pro, non implémenté
- ⚪ **évalué** = dans le périmètre raisonnable, pas de décision
- ❌ **hors scope** = explicitement refusé (avec raison)

## 1. Orchestration de conteneurs

| Cible | Artefact | Statut | Effort | Notes |
|---|---|---|---|---|
| `docker-compose` | `docker-compose.yml` | ✅ livré | — | mappage 1:1 du modèle |
| `podman-compose` | `podman-compose.yml` | 🟡 roadmap | Basse | quasi-identique compose |
| `docker-swarm` | stack file (deploy blocks) | ⚪ évalué | Basse | `deploy.replicas` existe déjà |
| `kubernetes` | Deployment + Service YAML | ✅ livré | — | pods/replicas/ports/env |
| `helm` | `templates/` + `values.yaml` | 🟡 roadmap | Moyenne | chart scellé : le .isor devient values |
| `kustomize` | `kustomization.yaml` + overlays | ⚪ évalué | Moyenne | patterns base/overlay ↔ `@extends` |
| `nomad` | `job.nomad` (HCL) | 🟡 roadmap | Moyenne | proche terraform |
| `openshift` | `DeploymentConfig`/`Route` | ⚪ évalué | Moyenne | variante k8s |
| `ecs` | task definitions JSON | 🟡 roadmap | Moyenne | AWS spécifique |
| `podman-quadlet` | `.container` units | ⚪ évalué | Basse | pont naturel systemd |
| `wasm` | manifest wasm/wasmCloud | ⚪ évalué | Moyenne | aligné adaptateur `wasm/` ORION |

## 2. IaC déclaratif — provisionnement

| Cible | Artefact | Statut | Effort | Notes |
|---|---|---|---|---|
| `terraform` | `.tf` HCL | 🟡 roadmap | Moyenne | **débloque aws/azure/gcp via providers** |
| `opentofu` | `.tf` (fork HCL) | ⚪ évalué | Basse | même émetteur que terraform |
| `pulumi` | `Pulumi.yaml` + programme | ⚪ évalué | Moyenne | Pulumi-YAML possible |
| `cloudformation` | template YAML/JSON | 🟡 roadmap | Moyenne | AWS natif |
| `azure` | Bicep / ARM | 🟡 roadmap | Haute | provider-specific |
| `gcp` | Deployment Manager | 🟡 roadmap | Haute | provider-specific |
| `aws` | (via terraform/cloudformation) | 🟡 roadmap | Haute | doublon avec les deux ci-dessus |
| `crossplane` | `Composition`/`Claim` CRDs | ⚪ évalué | Moyenne | k8s-native IaC |
| `cdktf` | `cdk.tf.json` (CDK for TF) | ⚪ évalué | Moyenne | même backend terraform |
| `aws-cdk` | `cdk synth` → CFN | ⚪ évalué | Moyenne | débouche sur cloudformation |

## 3. Gestion de configuration / provisionnement d'OS

| Cible | Artefact | Statut | Effort | Notes |
|---|---|---|---|---|
| `ansible` | `playbook.yml` | ✅ livré | — | community.docker |
| `cloud-init` | `cloud-config` | ⚪ évalué | Basse | bootstrap VM |
| `salt` | `.sls` states | ⚪ évalué | Moyenne | niche |
| `chef`/`puppet` | recipes/manifests | ❌ hors scope | — | déclin, YAML non natif |
| `vagrant` | `Vagrantfile` | ⚪ évalué | Basse | dev local |
| `powershell-dsc` | config `.ps1` DSC | ⚪ évalué | Moyenne | Windows natif |

## 4. OS natif / runtime de base

| Cible | Artefact | Statut | Effort | Notes |
|---|---|---|---|---|
| `systemd` | units `.service` | 🟡 roadmap | Basse | aligné adaptateur natif ORION |
| `launchd` | plist `.plist` | ⚪ évalué | Basse | macOS |
| `windows-service` | manifests/service registration | ⚪ évalué | Moyenne | Windows |
| `cron` | crontab | ⚪ évalué | Basse | jobs planifiés |

## 5. Serverless / FaaS

| Cible | Artefact | Statut | Effort | Notes |
|---|---|---|---|---|
| `knative` | `Service.serving.knative.dev` | ⚪ évalué | Moyenne | k8s serverless |
| `openfaas` | `stack.yml` functions | ⚪ évalué | Basse | proche compose |
| `sam`/`serverless` | `template.yaml`/`serverless.yml` | ⚪ évalué | Moyenne | AWS/serverless framework |
| `dapr` | component YAML | ⚪ évalué | Moyenne | sidecar distribué |
| `cloudflare-workers` | `wrangler.toml` | ⚪ évalué | Basse | TOML natif, edge |
| `azure-functions` | `host.json`/`function.json` | ⚪ évalué | Basse | |

## 6. GitOps & packaging k8s

| Cible | Artefact | Statut | Effort | Notes |
|---|---|---|---|---|
| `argocd` | `Application` CRD | ⚪ évalué | Basse | pointe sur manifests émis |
| `flux` | `Kustomization`/`HelmRelease` | ⚪ évalué | Basse | idem |
| `kpt` | `Kptfile` + package | ⚪ évalué | Moyenne | config-as-data |
| `timoni` | module CUE | ⚪ évalué | Haute | pont CUE |

## 7. Build & artefacts

| Cible | Artefact | Statut | Effort | Notes |
|---|---|---|---|---|
| `dockerfile` | `Dockerfile`/`Containerfile` | 🟡 roadmap | Basse | cible build-only |
| `nix` | `.nix` flakes | ⚪ évalué | Haute | reproductible |
| `packer` | `.pkr.hcl` | ⚪ évalué | Moyenne | images |
| `ko`/`buildpacks` | `project.toml` | ⚪ évalué | Basse | build source→image |

## 8. PaaS / manifests de plateformes

| Cible | Artefact | Statut | Effort | Notes |
|---|---|---|---|---|
| `fly-io` | `fly.toml` | ⚪ évalué | Basse | TOML déclaratif |
| `render` | `render.yaml` Blueprint | ⚪ évalué | Basse | YAML natif |
| `railway` | `railway.json`/TOML | ⚪ évalué | Basse | |
| `heroku` | `app.json`/`heroku.yml` | ⚪ évalué | Basse | ubiquitaire historique |
| `dokku`/`coolify` | compose-compatible | ⚪ évalué | Trivial | réutilise `docker-compose` |

## 9. Réseau / gateways / proxies

| Cible | Artefact | Statut | Effort | Notes |
|---|---|---|---|---|
| `traefik` | file provider YAML | ⚪ évalué | Basse | YAML natif, proche du modèle |
| `nginx` | `nginx.conf` | ⚪ évalué | Moyenne | syntaxe dédiée |
| `envoy` | `envoy.yaml` | ⚪ évalué | Moyenne | YAML, verbeux |
| `haproxy` | `haproxy.cfg` | ⚪ évalué | Moyenne | syntaxe dédiée |
| `caddy` | `Caddyfile`/JSON | ⚪ évalué | Basse | JSON natif disponible |
| `gateway-api` | Gateway/HTTPRoute CRDs | ⚪ évalué | Moyenne | successeur ingress k8s |

## 10. Service mesh

| Cible | Artefact | Statut | Effort | Notes |
|---|---|---|---|---|
| `istio` | VirtualService/DestinationRule | ⚪ évalué | Moyenne | CRDs k8s |
| `linkerd` | annotations/Server CRDs | ⚪ évalué | Basse | |
| `consul` | service defs HCL/JSON | ⚪ évalué | Moyenne | |
| `kuma` | policies CRDs | ⚪ évalué | Moyenne | |

## 11. Observabilité (config-as-infra)

| Cible | Artefact | Statut | Effort | Notes |
|---|---|---|---|---|
| `prometheus` | `prometheus.yml` scrape config | ⚪ évalué | Basse | YAML natif |
| `alertmanager` | `alertmanager.yml` | ⚪ évalué | Basse | YAML natif |
| `otel-collector` | `otelcol.yaml` pipelines | ⚪ évalué | Moyenne | YAML natif |
| `vector`/`fluentbit` | `vector.toml`/`.conf` | ⚪ évalué | Moyenne | pipelines logs |
| `grafana` | provisioning datasources/dashboards | ⚪ évalué | Basse | YAML provisioning |

## 12. Secrets & PKI

| Cible | Artefact | Statut | Effort | Notes |
|---|---|---|---|---|
| `external-secrets` | ExternalSecret CRD | ⚪ évalué | Basse | paire avec `secrets:` (I7-N) |
| `sealed-secrets` | SealedSecret | ⚪ évalué | Basse | nécessite kubeseal |
| `sops` | fichiers `ENC[...]` | ⚪ évalué | Moyenne | complète I7-N |
| `vault` | policies/config HCL | ⚪ évalué | Moyenne | |

## 13. Références internes / ponts

| Cible | Artefact | Statut | Effort | Notes |
|---|---|---|---|---|
| `yaml` | `resolved.yaml` | ✅ livré | — | référence expanded |
| `huborion` | dialecte `.isor` ORION | ✅ livré | — | pont exécution attestée (I6) |
| `json` | `resolved.json` | ⚪ évalué | Trivial | interop outillage |
| `toml` | `resolved.toml` | ⚪ évalué | Trivial | interop outillage |
| `dotenv` | `.env` | ⚪ évalué | Trivial | extraction env/ports |
| `cue`/`jsonnet` | export langage | ⚪ évalué | Moyenne | interop langages de config |

## 14. Hors scope (refusés, avec raison)

| Non-cible | Raison |
|---|---|
| `apply`/`deploy` runtime | ISORION émet, n'exécute pas — frontière preflight (I7) |
| Drift detection live | domaine ORION — `isorion diff` compare les artefacts |
| Chef/Puppet | écosystèmes déclinants, non-YAML natif |
| Moteurs CI (gha/gitlab-ci) | périmètre « infrastructure », pas pipelines de build |
| Mesos/Marathon | projet mort |
| Bases de données managées | état runtime, pas artefacts — via terraform |

## Comptage honnête (mise à jour automatique par `isorion targets`)

- **Livrées : 5** — `yaml`, `docker-compose`, `kubernetes`, `ansible`, `huborion`
- **Roadmap prochaine** : `terraform` → `helm` → `systemd` (déjà priorisé I3)
- **Inventaire total : ~75 cibles nommées** (70 lignes, entrées fusionnées
  décomptées), 6 hors scope
- Le « 13+ cibles » d'isorion.org correspond à : docker, compose, k8s, helm,
  terraform, cloudformation, aws, azure, gcp, ansible, systemd, nomad +
  huborion → atteint quand les 🟡 roadmap sont livrées

## Priorisation rationnelle (ordre de déblocage)

```
1. terraform     → HCL déclaratif, couvre aws/azure/gcp par providers
2. helm          → demande k8s dominante
3. systemd       → natif, cohérent avec ORION native adapter
4. podman-compose→ quasi-gratuit (fork compose)
5. nomad         → HCL, mêmes mécanismes que terraform
6. json          → trivial, interop
7. gitops (argocd/flux) → wrappers légers sur sorties existantes
8. traefik/prometheus → YAML natif, quasi-gratuit
9. json/toml/dotenv → triviaux, interop
```

> Beaucoup d'entrées ⚪ deviennent triviales une fois `terraform`, `helm`
> et les émetteurs de wrappers (gitops, PaaS) en place — le modèle
> `model.Infra` ne change pas, seuls les émetteurs s'ajoutent (invariant I2-1).
