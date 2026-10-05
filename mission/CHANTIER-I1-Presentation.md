# 🎯 CHANTIER I1 : Présentation d'ISORION

**Priorité :** 🟢 Fondation
**Statut :** ✅ Documenté
**Responsable :** Équipe Lang

---

## 📋 QU'EST-CE QU'ISORION ?

ISORION est un **compilateur d'infrastructure universel**. On écrit
l'infrastructure une seule fois dans un langage déclaratif **YAML++**
(extension `.isor`), et le compilateur `fusion` émet les artefacts natifs
de la cible choisie : docker-compose, manifestes Kubernetes, playbooks
Ansible, YAML résolu, ou le dialecte ORION (cible `huborion`) — cibles
livrées aujourd'hui ; Terraform, Helm et les clouds sont roadmap (voir
[I3](CHANTIER-I3-Cibles.md)).

> L'intention n'est pas « comment déployer sur X », mais « voici mon
> infrastructure » — le compilateur résout le « comment » par cible.

## Le langage YAML++ (résumé spec §1-4)

- **YAML 100 % valide** — tout non-directive est du YAML standard.
- `variables:` → références `$name`, interpolation `${expr}`.
- Directives prétraitées **avant parsing** : `@if/@elif/@else`
  (merge de mapping / splice de séquence), `@for $x in list`.
- Expressions : comparaisons, `&&`/`||`/`!`, arithmétique, fonctions
  (`env()`, `len()`, `upper/lower`, `default()`, `int()`, `string()`).
- Pass-through : champs inconnus préservés en `Extra` pour les cibles
  qui les supportent.

## Pour qui ?

- **Équipes multi-cibles** : même infra sur compose en dev, k8s en prod.
- **Migration** : un source `.isor` → réémission vers une autre cible.
- **Pipeline de confiance** : via `huborion`, l'infra devient capsule
  ORION signée (exécution attestée).
- **Revue/diff** : un fichier déclaratif reviewable au lieu de N artefacts
  générés à la main.

## Ce que ISORION n'est PAS

- Pas un orchestrateur runtime — il émet des artefacts, il ne déploie
  pas (orion run / kubectl apply / ansible-playbook le font).
- Pas le même `.isor` qu'ORION — voir
  [CHANTIER-I6](CHANTIER-I6-Relation-Orion.md) (collision de nommage).
- Pas un lock-in : les artefacts émis sont standards, consommables par
  les outils natifs de chaque cible.

## Commandes

```bash
isorion fusion app.isor -t docker-compose -o out/
isorion fusion app.isor -t kubernetes,ansible   # multi-cible
isorion validate app.isor
isorion targets
```
