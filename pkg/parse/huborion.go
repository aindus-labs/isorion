package parse

import (
	"fmt"
	"sort"
	"strings"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
	"gopkg.in/yaml.v3"
)

// huborionImporter re-captures the ORION dialect emitted by the `huborion`
// target: graph.nodes → services, `depends_on` edges → DependsOn,
// capsule.name → project. Non-docker entrypoints are kept in Extra.
type huborionImporter struct{}

func (huborionImporter) Name() string { return "huborion" }
func (huborionImporter) Description() string {
	return "ORION dialect .isor (capsule graph → services)"
}

func (huborionImporter) Parse(data []byte, filename string) (*model.Infra, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: YAML parse error: %w", filename, err)
	}
	if _, ok := doc["graph"]; !ok {
		return nil, fmt.Errorf("%s: not a huborion document (no graph:)", filename)
	}

	infra := &model.Infra{
		Version:  "1.0",
		Services: map[string]*model.Service{},
		Networks: map[string]any{},
		Volumes:  map[string]any{},
		Other:    map[string]any{},
	}

	if caps, ok := doc["capsule"].(map[string]any); ok {
		infra.ProjectName = strOf(caps["name"])
		if labels, ok := caps["labels"].(map[string]any); ok {
			if v := strOf(labels["source-version"]); v != "" {
				infra.Version = v
			}
		}
	}
	if intent, ok := doc["intent"].(map[string]any); ok {
		infra.Description = strOf(intent["description"])
	}

	graph, _ := doc["graph"].(map[string]any)
	for _, n := range listOf(graph["nodes"]) {
		nm, _ := n.(map[string]any)
		svc, err := hubNode(nm)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filename, err)
		}
		infra.Services[svc.Name] = svc
		infra.ServiceNames = append(infra.ServiceNames, svc.Name)
	}
	sort.Strings(infra.ServiceNames)

	for _, e := range listOf(graph["edges"]) {
		em, _ := e.(map[string]any)
		if strOf(em["type"]) != "depends_on" {
			continue
		}
		to := strOf(em["to"])
		if svc := infra.Services[to]; svc != nil {
			svc.DependsOn = append(svc.DependsOn, strOf(em["from"]))
		}
	}
	if len(infra.Services) == 0 {
		return nil, fmt.Errorf("%s: no nodes in graph:", filename)
	}
	return infra, nil
}

func init() { Register(huborionImporter{}) }

// hubNode converts one graph.nodes[] entry into a model.Service.
func hubNode(nm map[string]any) (*model.Service, error) {
	name := strOf(nm["id"])
	if name == "" {
		return nil, fmt.Errorf("node without id")
	}
	s := &model.Service{Name: name, Replicas: 1, Extra: map[string]any{}}
	if ep := strOf(nm["entrypoint"]); ep != "" && ep != "docker" {
		s.Extra["entrypoint"] = ep
	}
	params, _ := nm["params"].(map[string]any)
	for k, v := range params {
		switch k {
		case "image":
			s.Image = strOf(v)
		case "command":
			s.Command = strList(v)
		case "replicas":
			if n, ok := toInt(v); ok {
				s.Replicas = n
			}
		case "ports":
			for _, p := range listOf(v) {
				pm, err := model.ParsePort(strOf(p))
				if err != nil {
					return nil, fmt.Errorf("node %s: ports: %w", name, err)
				}
				s.Ports = append(s.Ports, pm)
			}
		case "env":
			if em, ok := v.(map[string]any); ok {
				s.Environment = map[string]string{}
				for ek, ev := range em {
					val := strOf(ev)
					if rest, ok := strings.CutPrefix(val, "secret:"); ok {
						s.Environment[ek] = lang.SecretMarker + rest
					} else {
						s.Environment[ek] = val
					}
				}
			}
		default:
			s.Extra[k] = v
		}
	}
	if s.Image == "" && s.Build == "" {
		return nil, fmt.Errorf("node %s: params.image missing", name)
	}
	return s, nil
}
