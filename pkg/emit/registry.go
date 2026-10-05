// Package emit regroups the target emitters: each translates the normalized
// model.Infra into target-specific files (docker-compose.yml, k8s manifests,
// Ansible playbooks, ORION .isor dialect...).
package emit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
)

// File is one emitted artifact.
type File struct {
	Path    string // path relative to the output directory
	Content []byte
}

// Emitter compiles the normalized model into target files.
type Emitter interface {
	Name() string
	Description() string
	Emit(doc *lang.Document, infra *model.Infra) ([]File, error)
}

var registry = map[string]Emitter{}

// Register adds an emitter to the registry (called from init()).
func Register(e Emitter) {
	registry[e.Name()] = e
}

// Get returns an emitter by name.
func Get(name string) (Emitter, error) {
	e, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown target %q (available: %s)", name, joinNames())
	}
	return e, nil
}

// Names returns the sorted list of registered targets.
func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Warner is implemented by emitters that report capability warnings —
// fields present in the model but ignored by the target.
type Warner interface {
	Warn(doc *lang.Document, infra *model.Infra) []string
}

// Transformer post-processes an emitter's files — the building block of
// composite targets (`-t kubernetes+annotate`).
type Transformer interface {
	Name() string
	Apply(target string, files []File) ([]File, error)
}

var transformers = map[string]Transformer{}

// RegisterTransformer adds a transformer (called from init()).
func RegisterTransformer(t Transformer) { transformers[t.Name()] = t }

// TransformerNames returns the sorted list of registered transformers.
func TransformerNames() []string {
	names := make([]string, 0, len(transformers))
	for n := range transformers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Run resolves a target (possibly composite `base+tx1+tx2`), emits its
// files, collects capability warnings and applies transformers in order.
func Run(target string, doc *lang.Document, infra *model.Infra) (files []File, warnings []string, err error) {
	parts := strings.Split(target, "+")
	e, err := Get(parts[0])
	if err != nil {
		return nil, nil, err
	}
	files, err = e.Emit(doc, infra)
	if err != nil {
		return nil, nil, err
	}
	if w, ok := e.(Warner); ok {
		warnings = w.Warn(doc, infra)
	}
	for _, extra := range parts[1:] {
		tx, ok := transformers[extra]
		if !ok {
			return nil, nil, fmt.Errorf("unknown transformer %q (available: %s)", extra, strings.Join(TransformerNames(), ", "))
		}
		files, err = tx.Apply(parts[0], files)
		if err != nil {
			return nil, nil, fmt.Errorf("transformer %s: %w", extra, err)
		}
	}
	return files, warnings, nil
}

func joinNames() string {
	names := Names()
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}
