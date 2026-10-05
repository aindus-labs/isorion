package parse

import (
	"fmt"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
	"gopkg.in/yaml.v3"
)

// yamlImporter accepts a resolved.yaml (the `yaml` target output) — or any
// flat YAML++ body — and rebuilds the model through the normal path. This
// is the identity importer: parse(emit_yaml(x)) == x by construction.
type yamlImporter struct{}

func (yamlImporter) Name() string        { return "yaml" }
func (yamlImporter) Description() string { return "resolved.yaml / flat .isor body (identity import)" }

func (yamlImporter) Parse(data []byte, filename string) (*model.Infra, error) {
	var body map[string]any
	if err := yaml.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("%s: YAML parse error: %w", filename, err)
	}
	version := strOf(body["version"])
	delete(body, "version")
	infra, err := model.FromBody(version, body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}
	if len(infra.Services) == 0 && len(infra.Other) == 0 &&
		len(infra.Networks) == 0 && len(infra.Volumes) == 0 {
		return nil, fmt.Errorf("%s: no services or sections found", filename)
	}
	if infra.Version == "" {
		infra.Version = "1.0"
	}
	return infra, nil
}

func init() { Register(yamlImporter{}) }

// keep the lang import used (constraint/secrets flow through FromBody).
var _ = lang.SecretMarker
