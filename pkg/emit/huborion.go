package emit

import (
	"fmt"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
	"gopkg.in/yaml.v3"
)

// huborionEmitter bridges ISORION to the ORION ecosystem: it emits a file in
// the ORION .isor dialect (intention → capsule), consumable by `orion build`.
//
//	.isor (isorion universal) --fusion--> .isor (orion dialect) --orion build--> .ocaps
type huborionEmitter struct{}

func init() { Register(huborionEmitter{}) }

func (huborionEmitter) Name() string        { return "huborion" }
func (huborionEmitter) Description() string { return "ORION dialect .isor (intention capsule source)" }

func (huborionEmitter) Emit(doc *lang.Document, infra *model.Infra) ([]File, error) {
	if infra.ProjectName == "" {
		return nil, fmt.Errorf("huborion target requires project.name")
	}

	var nodes []any
	var edges []any
	for _, name := range infra.ServiceNames {
		s := infra.Services[name]
		params := map[string]any{}
		if s.Image != "" {
			params["image"] = s.Image
		}
		if len(s.Command) > 0 {
			params["command"] = s.Command
		}
		if s.Replicas != 1 {
			params["replicas"] = s.Replicas
		}
		if len(s.Ports) > 0 {
			var ports []string
			for _, p := range s.Ports {
				ports = append(ports, p.Compose())
			}
			params["ports"] = ports
		}
		if len(s.Environment) > 0 {
			env := map[string]string{}
			for k, v := range s.Environment {
				if sn, ok := lang.SecretName(v); ok {
					env[k] = "secret:" + sn // ORION secrets capability reference
				} else {
					env[k] = v
				}
			}
			params["env"] = env
		}
		nodes = append(nodes, map[string]any{
			"id":         name,
			"entrypoint": "docker",
			"params":     params,
		})
		for _, dep := range s.DependsOn {
			edges = append(edges, map[string]any{
				"from": dep,
				"to":   name,
				"type": "depends_on",
			})
		}
	}

	description := infra.Description
	if description == "" {
		description = "Intention compiled by ISORION (universal infrastructure compiler)"
	}

	out := map[string]any{
		"capsule": map[string]any{
			"name":    infra.ProjectName,
			"version": "0.1.0",
			"labels": map[string]any{
				"generated-by":    "isorion",
				"source-version":  doc.Version,
				"intent-language": "isorion-yaml++",
			},
		},
		"intent": map[string]any{
			"type":        "service",
			"description": description,
		},
		"payload": map[string]any{
			"runtime": "docker",
		},
		"security": map[string]any{
			"sandbox": "strict",
		},
	}
	if len(nodes) > 0 {
		out["graph"] = map[string]any{"nodes": nodes, "edges": edges}
	}

	data, err := yaml.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("huborion emit: %w", err)
	}
	return []File{{Path: infra.ProjectName + ".huborion.isor", Content: data}}, nil
}

// Warn reports model fields silently dropped by this target.
func (huborionEmitter) Warn(_ *lang.Document, infra *model.Infra) []string {
	var out []string
	for _, name := range infra.ServiceNames {
		s := infra.Services[name]
		for k := range s.Extra {
			out = append(out, fmt.Sprintf("services.%s.%s ignored by target huborion", name, k))
		}
		if s.Build != "" {
			out = append(out, fmt.Sprintf("services.%s.build ignored by target huborion", name))
		}
		if len(s.Volumes) > 0 {
			out = append(out, fmt.Sprintf("services.%s.volumes ignored by target huborion", name))
		}
		if len(s.Networks) > 0 {
			out = append(out, fmt.Sprintf("services.%s.networks ignored by target huborion", name))
		}
		if s.Restart != "" {
			out = append(out, fmt.Sprintf("services.%s.restart ignored by target huborion", name))
		}
	}
	return out
}
