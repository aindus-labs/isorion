package emit

import (
	"fmt"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
	"gopkg.in/yaml.v3"
)

// traefikEmitter emits a Traefik file-provider config: one router+service
// per service exposing ports (host rule <name>.local, entryPoint web).
type traefikEmitter struct{}

func init() { Register(traefikEmitter{}) }

func (traefikEmitter) Name() string { return "traefik" }
func (traefikEmitter) Description() string {
	return "Traefik file provider (routers + loadBalancer services)"
}

func (traefikEmitter) Emit(_ *lang.Document, infra *model.Infra) ([]File, error) {
	routers := map[string]any{}
	services := map[string]any{}
	for _, name := range infra.ServiceNames {
		s := infra.Services[name]
		if len(s.Ports) == 0 {
			continue
		}
		port := s.Ports[0].Host
		if port == 0 {
			port = s.Ports[0].Container
		}
		routers[name] = map[string]any{
			"rule":        fmt.Sprintf("Host(`%s.local`)", name),
			"entryPoints": []string{"web"},
			"service":     name,
		}
		services[name] = map[string]any{
			"loadBalancer": map[string]any{
				"servers": []any{
					map[string]any{"url": fmt.Sprintf("http://%s:%d", name, port)},
				},
			},
		}
	}
	out := map[string]any{
		"http": map[string]any{
			"routers":  routers,
			"services": services,
		},
	}
	data, err := yaml.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("traefik emit: %w", err)
	}
	return []File{{Path: "traefik.yml", Content: data}}, nil
}

// Warn reports services skipped by the traefik target (no ports to route).
func (traefikEmitter) Warn(_ *lang.Document, infra *model.Infra) []string {
	var out []string
	for _, name := range infra.ServiceNames {
		if len(infra.Services[name].Ports) == 0 {
			out = append(out, fmt.Sprintf("services.%s skipped by target traefik (no ports to route)", name))
		}
	}
	return out
}
