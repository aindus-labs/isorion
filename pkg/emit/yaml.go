package emit

import (
	"fmt"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
	"gopkg.in/yaml.v3"
)

// yamlEmitter emits the resolved document as plain YAML — the reference
// target used to inspect expansion results.
type yamlEmitter struct{}

func init() { Register(yamlEmitter{}) }

func (yamlEmitter) Name() string        { return "yaml" }
func (yamlEmitter) Description() string { return "Resolved YAML document (reference output)" }

func (yamlEmitter) Emit(doc *lang.Document, infra *model.Infra) ([]File, error) {
	out := map[string]any{}
	if doc.Version != "" {
		out["version"] = doc.Version
	}
	for k, v := range doc.Body {
		out[k] = v
	}
	data, err := yaml.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("yaml emit: %w", err)
	}
	return []File{{Path: "resolved.yaml", Content: data}}, nil
}
