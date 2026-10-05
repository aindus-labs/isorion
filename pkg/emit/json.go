package emit

import (
	"encoding/json"
	"fmt"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
)

// jsonEmitter emits the resolved document as plain JSON — same payload as
// the `yaml` reference target, for JSON-only toolchains.
type jsonEmitter struct{}

func init() { Register(jsonEmitter{}) }

func (jsonEmitter) Name() string        { return "json" }
func (jsonEmitter) Description() string { return "Resolved JSON document (interop output)" }

func (jsonEmitter) Emit(doc *lang.Document, _ *model.Infra) ([]File, error) {
	out := map[string]any{}
	if doc.Version != "" {
		out["version"] = doc.Version
	}
	for k, v := range doc.Body {
		out[k] = v
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("json emit: %w", err)
	}
	return []File{{Path: "resolved.json", Content: append(data, '\n')}}, nil
}
