package parse

import (
	"fmt"
	"sort"
	"strings"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
	"gopkg.in/yaml.v3"
)

// helmImporter lifts a chart's values.yaml (the model carrier emitted by
// the helm target) back to Infra. Pass `import <chart>/values.yaml -f helm`.
// A bare Chart.yaml yields project metadata only.
type helmImporter struct{}

func (helmImporter) Name() string { return "helm" }
func (helmImporter) Description() string {
	return "Helm chart values.yaml (pass the chart's values.yaml)"
}

func (helmImporter) Parse(data []byte, filename string) (*model.Infra, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: YAML parse error: %w", filename, err)
	}
	infra := &model.Infra{
		Version:  "1.0",
		Services: map[string]*model.Service{},
		Networks: map[string]any{},
		Volumes:  map[string]any{},
		Other:    map[string]any{},
	}

	// Chart.yaml shape: {apiVersion, name, description, ...} without services.
	if _, hasServices := doc["services"]; !hasServices && strOf(doc["name"]) != "" {
		infra.ProjectName = strOf(doc["name"])
		infra.Description = strOf(doc["description"])
		return infra, nil
	}

	svcs, ok := doc["services"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: no services: section (pass the chart's values.yaml)", filename)
	}
	for name, raw := range svcs {
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s: services.%s: must be a mapping", filename, name)
		}
		svc, err := helmService(name, m)
		if err != nil {
			return nil, fmt.Errorf("%s: services.%s: %w", filename, name, err)
		}
		infra.Services[name] = svc
		infra.ServiceNames = append(infra.ServiceNames, name)
	}
	sort.Strings(infra.ServiceNames)
	for k, v := range doc {
		if k != "services" && k != "secrets" {
			infra.Other[k] = v
		}
	}
	return infra, nil
}

func init() { Register(helmImporter{}) }

func helmService(name string, m map[string]any) (*model.Service, error) {
	s := &model.Service{Name: name, Replicas: 1, Extra: map[string]any{}}
	for k, v := range m {
		switch k {
		case "image":
			s.Image = strOf(v)
		case "build":
			s.Build = strOf(v)
		case "command":
			s.Command = strList(v)
		case "replicas":
			if n, ok := toInt(v); ok {
				s.Replicas = n
			}
		case "ports":
			for _, p := range listOf(v) {
				pm, _ := p.(map[string]any)
				port := model.PortMap{Protocol: "tcp"}
				if c, ok := toInt(pm["container"]); ok {
					port.Container = c
				}
				if h, ok := toInt(pm["host"]); ok {
					port.Host = h
				}
				if proto := strOf(pm["protocol"]); proto != "" {
					port.Protocol = strings.ToLower(proto)
				}
				s.Ports = append(s.Ports, port)
			}
		case "env":
			if em, ok := v.(map[string]any); ok {
				s.Environment = map[string]string{}
				for ek, ev := range em {
					if ref, ok := ev.(map[string]any); ok {
						if skr, ok := ref["secretKeyRef"].(map[string]any); ok {
							s.Environment[ek] = lang.SecretMarker + strOf(skr["key"])
							continue
						}
					}
					s.Environment[ek] = strOf(ev)
				}
			}
		case "depends_on":
			s.DependsOn = strList(v)
		case "volumes":
			s.Volumes = strList(v)
		case "networks":
			s.Networks = strList(v)
		case "restart":
			s.Restart = strOf(v)
		case "labels":
			if lm, ok := v.(map[string]any); ok {
				s.Labels = lm
			}
		default:
			s.Extra[k] = v
		}
	}
	if s.Image == "" && s.Build == "" {
		return nil, fmt.Errorf("either image or build is required")
	}
	return s, nil
}
