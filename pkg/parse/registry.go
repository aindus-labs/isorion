// Package parse regroups the importers — the mirror of pkg/emit. Each
// importer lifts an existing infrastructure format (docker-compose.yml,
// k8s manifests...) back into the normalized model.Infra, which can then
// be emitted to any target — including the `isor` target for a canonical
// .isor document.
package parse

import (
	"fmt"
	"sort"
	"strings"

	"github.com/isorion-lang/isorion/pkg/model"
)

// Importer lifts a source format into the normalized model.
type Importer interface {
	Name() string
	Description() string
	Parse(data []byte, filename string) (*model.Infra, error)
}

var registry = map[string]Importer{}

// Register adds an importer to the registry (called from init()).
func Register(i Importer) {
	registry[i.Name()] = i
}

// Get returns an importer by name.
func Get(name string) (Importer, error) {
	i, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown import format %q (available: %s)", name, strings.Join(Names(), ", "))
	}
	return i, nil
}

// Names returns the sorted list of registered importers.
func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
