# Étude I7-F — Plugins émetteurs en WASM (v0.3+)

**Statut :** étude livrée — implémentation repoussée à v0.3+
**Date :** 2026-10-05

## Problème

Ajouter une cible exige aujourd'hui : du Go, un rebuild du binaire,
l'interface `emit.Emitter`, l'inscription via `init()`. Le chantier I7
benchmarque ce que les compilateurs d'infra font pour l'extensibilité :

| Outil | Mécanisme |
|---|---|
| Pulumi | plugins provider = processus gRPC séparés |
| Bicep/Terraform | providers externes, protocole réseau |
| Caddy/Nginx | modules compilés dans le binaire |
| OPA | wasm comme cible de *policy evaluation* |

Deux modèles : **plugins processus** (résilience, langage libre, poids
I/O+protocole) ou **plugins in-process sandboxés** (wasm — isolation sans
conteneur, langages compilables vers wasm, ~zéro overhead réseau).

## Pourquoi wasm plutôt que des sous-processus

- **Sandbox native** : le plugin ne touche ni fs ni réseau sauf capability
  explicite — un émetteur malveillant ou buggé ne sort pas de sa boîte.
- **Un seul binaire `isorion`** : pas de `isorion-emit-<target>` à
  distribuer/découvrir ; les plugins sont des `.wasm` déposés dans
  `~/.isorion/plugins/`.
- **Multi-langage** : Rust/TinyGo/AssemblyScript → même ABI.
- **Modèle de données déjà sérialisable** : `model.Infra` tient en un JSON.

## Design proposé

```
isorion fusion app.isor -t <plugin>          ex. -t wasm://nomad.wasm
        │
        ▼
emit.Run résout : registre natif → sinon lookup plugin
        │
        ▼
plugin.Invoke(InfraJSON, optionsJSON) → {files:[{path,content}],
                                          warnings:[...], error?}
```

- **Runtime** : `wazero` (pure Go, pas de CGO — cohérent avec la chaîne
  actuelle) ou `wasmtime-go`. wazero recommandé.
- **WASI** : `wasi_snapshot_preview1` **sans** fs/preopens par défaut ;
  stdin/stdout ou mémoire linéaire + guest `alloc` pour l'I/O.
- **ABI minimale** (contrat versionné `isorion-plugin-v1`) :
  - guest exporte `isorion_emit(ptr,len) -> (ptr,len)` ;
  - entrée = `{"infra": <Infra JSON>, "doc": {"version","secrets","constraints"}}` ;
  - sortie = `{"files":[...], "warnings":[...]}` ou `{"error":"..."}`.
- **Capability warnings** : pas d'interface `Warner` côté plugin — les
  warnings voyagent dans la réponse (unifié avec `emit.Run`).
- **Découverte** : `-t wasm:<path>` explicite en v0.3 ; registre
  `~/.isorion/plugins/*.wasm` + `isorion targets` les listant en v0.4.

## Coûts / risques

- **ABI = contrat public** : toute évolution de `model.Infra` doit rester
  rétro-compatible JSON (champs optionnels, `Extra` permissif — déjà vrai).
- **Perf** : marshal/unmarshal JSON + démarrage module ~ms — négligeable
  vs réseau cible. Cache de module compilé par fusion multi-cibles.
- **Secrets** : le plugin reçoit les **marqueurs** `isorion-secret://`
  dans env, jamais le ciphertext — la règle « ciphertext hors artifacts »
  vaut aussi pour les plugins (à graver dans la spec ABI).
- **Poids** : wazero ajoute ~1-2 Mo au binaire — acceptable.
- **Écosystème guest** : SDK minuscule à fournir (Rust+TinyGo) — sinon la
  fonctionnalité reste lettre morte.

## Décision

**Repoussé à v0.3+, design figé** : modèle plugin-in-process wazero, ABI
`isorion-plugin-v1` entrée/sortie JSON, secrets sous forme de marqueurs.
Le point de friction réel n'est pas technique mais écosystémique (SDK
guest + doc) — d'où le classement v0.3+ plutôt que maintenant, alors que
9 émetteurs natifs couvrent les cibles courantes.

Critère de déclenchement : première demande d'une cible que l'équipe ne
veut pas maintenir dans le repo (ex. cible propriétaire interne).
