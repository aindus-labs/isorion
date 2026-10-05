# 🎯 CHANTIER I5 : Innovations d'ISORION

**Priorité :** 🟢 Différenciation
**Statut :** ✅ Documenté
**Responsable :** Équipe Lang

---

## 📋 CE QUE ISORION FAIT QUE LES AUTRES NE FONT PAS

### 1. YAML++ — langage, pas format

Les outils existants (Helm templates, envsubst, jsonnet, CUE, Dhall)
ajoutent de la logique *autour* de YAML. ISORION prétraite les
directives **avant le parse** : le document reste un arbre YAML
manipulable, pas du texte templété.

- `@if`/@for` opèrent sur la **structure** (merge de mapping, splice de
  séquence) — pas sur des chaînes → pas de YAML cassé par indentation.
- `$$` échappe proprement ; `Extra` préserve l'inconnu.

### 2. Un seul modèle normalisé — pivot universel

Le modèle `model.Infra` est le point d'équilibre : N cibles **et**
(importer I4) M sources convergent dessus. C'est ce qui rend le
reverse-compiler et la migration **structurellement** possibles, pas
bricolés.

### 3. Pass-through honnête — pas de perte silencieuse

Les champs que la cible ne comprend pas sont préservés en `Extra`,
propagés quand possibles, ignorés proprement sinon — **jamais d'erreur
opaque ni de drop silencieux**. La fidélité est un contrat, pas un
espoir.

### 4. Le pont vers l'exécution attestée

`huborion` est l'unique passerelle vers une exécution **signée et
attestée** : une infra déclarée peut devenir une capsule `.ocaps` dont
l'exécution prouve qu'elle correspond à l'intention. Aucun autre
compilateur d'infrastructure n'aboutit à un artefact vérifiable.

### 5. Directives contextuelles (mapping vs séquence)

La même directive `@if` a une sémantique structurelle différente selon
le contexte — merge de clés en mapping, épissage d'éléments en
séquence. C'est un choix de design qui rend les conditionnelles
*lisibles* là où Helm/Jsonnet produisent du bruit.

## Positionnement honnête

| Outil | Force | Limite |
|---|---|---|
| Terraform/Pulumi | providers cloud, état | infra cloud, pas runtime |
| Helm | écosystème k8s | templating texte fragile |
| Jsonnet/CUE/Dhall | vrais langages | écosystème niche |
| docker-compose | simplicité | mono-cible, pas de logique |
| **ISORION** | multi-cible + logique structurelle + pont .ocaps | jeune, 5 cibles |

## À documenter au fil

- Patterns d'abstraction par cible (ce qui se perd, ce qui gagne).
- Études round-trip (I4) comme preuve de non-perte.
