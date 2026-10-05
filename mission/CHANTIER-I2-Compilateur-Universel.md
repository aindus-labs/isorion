# 🎯 CHANTIER I2 : Le Compilateur Universel

**Priorité :** 🔴 Cœur
**Statut :** ✅ Livré et testé
**Responsable :** Équipe Lang

---

## 📋 ARCHITECTURE

Pipeline en trois couches, séparation stricte :

```
.isor (YAML++)
   │
   ▼  pkg/lang — front-end du langage
preprocess.go   directives @if/@for prétraitées AVANT le parse YAML
expr.go         évaluateur d'expressions (comp, logique, fonctions)
eval.go         expansion de l'arbre : vars $name, ${expr}, $$ escape
   │
   ▼  pkg/model — modèle normalisé (contrat unique)
model.Infra     project + services + networks + volumes + Extra
   │
   ▼  pkg/emit — backends par cible
registry.go     interface Emitter + registre auto-enregistré (init)
yaml | docker_compose | kubernetes | ansible | huborion
```

## Invariants du compilateur

1. **Les émetteurs ne voient que `model.Infra`** — jamais de directives,
   jamais de `$vars` : tout est résolu en amont par `pkg/lang`.
2. **Un seul front-end** : ajouter un émetteur ne touche jamais `lang/`.
3. **Pass-through `Extra`** : champ inconnu → préservé ou ignoré
   proprement, jamais d'erreur.
4. **`depends_on` → sémantique de la cible** : ordre compose/Ansible,
   arêtes `type: depends_on` en huborion.
5. **Test e2e obligatoire** : doc → infra → fichiers dans
   `pkg/emit/emit_test.go`.

## Extension : ajouter une cible

```go
// pkg/emit/ma_cible.go
type maCible struct{}
func (maCible) Name() string        { return "ma-cible" }
func (maCible) Description() string { return "..." }
func (maCible) Emit(doc *lang.Document, infra *model.Infra) ([]File, error) {
    // traduire infra -> fichiers natifs
}
func init() { Register(maCible{}) }   // auto-registration
```

La cible devient immédiatement visible dans `isorion targets` et
`fusion -t`.

## Livrables

- [x] Front-end `pkg/lang` (preprocess/expr/eval) — couvert par tests
- [x] Modèle `pkg/model` normalisé
- [x] Registre `pkg/emit` + 9 émetteurs (yaml, docker-compose, kubernetes,
  ansible, huborion, helm, terraform, systemd, isor)
- [x] CLI `cmd/isorion` : `fusion`, `validate`, `diff`, `fmt`, `import`,
  `targets`, `version`
- [x] Tests `cmd/` et `pkg/model` — `cmd/isorion/main_test.go` (splitArgs,
  parseSets, validate/fusion/diff/fmt e2e), `pkg/model/model_test.go`
  (ParsePort, Compose, FromBody, helpers) — 2026-10-05
- [x] Émetteurs roadmap — `helm`, `terraform`, `systemd` livrés + `isor`
  (reverse-compiler) ; reste `aws`/`azure`/`gcp` → [I3](CHANTIER-I3-Cibles.md)
