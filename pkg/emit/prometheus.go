package emit

import (
	"fmt"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
	"gopkg.in/yaml.v3"
)

// prometheusEmitter emits a scrape config: one job per service with ports,
// targeting <service>:<container-port> (in-cluster DNS / compose network).
type prometheusEmitter struct{}

func init() { Register(prometheusEmitter{}) }

func (prometheusEmitter) Name() string { return "prometheus" }
func (prometheusEmitter) Description() string {
	return "Prometheus scrape config (one job per service with ports)"
}

func (prometheusEmitter) Emit(_ *lang.Document, infra *model.Infra) ([]File, error) {
	var jobs []any
	for _, name := range infra.ServiceNames {
		s := infra.Services[name]
		for _, p := range s.Ports {
			jobs = append(jobs, map[string]any{
				"job_name": name,
				"static_configs": []any{
					map[string]any{
						"targets": []string{fmt.Sprintf("%s:%d", name, p.Container)},
					},
				},
			})
		}
	}
	out := map[string]any{
		"global":         map[string]any{"scrape_interval": "15s"},
		"scrape_configs": jobs,
	}
	data, err := yaml.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("prometheus emit: %w", err)
	}
	return []File{{Path: "prometheus.yml", Content: data}}, nil
}

// Warn reports services skipped by the prometheus target (no scrape target).
func (prometheusEmitter) Warn(_ *lang.Document, infra *model.Infra) []string {
	var out []string
	for _, name := range infra.ServiceNames {
		if len(infra.Services[name].Ports) == 0 {
			out = append(out, fmt.Sprintf("services.%s skipped by target prometheus (no ports to scrape)", name))
		}
	}
	return out
}
