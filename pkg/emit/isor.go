package emit

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
	"gopkg.in/yaml.v3"
)

// isorTarget re-emits the normalized model as a canonical .isor document —
// the "self" target, and the back end of the reverse-compiler pipeline
// (imported infra → .isor). Emitted files re-parse cleanly: no directives,
// no variables, plain YAML++ data.
type isorTarget struct{}

func (isorTarget) Name() string        { return "isor" }
func (isorTarget) Description() string { return "canonical .isor (YAML++) document" }

func (isorTarget) Emit(_ *lang.Document, infra *model.Infra) ([]File, error) {
	root := &yaml.Node{Kind: yaml.MappingNode}

	version := infra.Version
	if version == "" {
		version = "1.0"
	}
	setPair(root, "version", version)

	if infra.ProjectName != "" || infra.Description != "" || len(infra.Labels) > 0 {
		proj := map[string]any{}
		if infra.ProjectName != "" {
			proj["name"] = infra.ProjectName
		}
		if infra.Description != "" {
			proj["description"] = infra.Description
		}
		if len(infra.Labels) > 0 {
			proj["labels"] = infra.Labels
		}
		setPair(root, "project", proj)
	}
	if len(infra.Services) > 0 {
		svcs := map[string]any{}
		for _, name := range infra.ServiceNames {
			svcs[name] = serviceMap(infra.Services[name])
		}
		setPair(root, "services", svcs)
	}
	if len(infra.Networks) > 0 {
		setPair(root, "networks", infra.Networks)
	}
	if len(infra.Volumes) > 0 {
		setPair(root, "volumes", infra.Volumes)
	}
	for _, k := range sortedKeys(infra.Other) {
		setPair(root, k, infra.Other[k])
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	enc.Close()

	name := infra.ProjectName
	if name == "" {
		name = "app"
	}
	return []File{{Path: name + ".isor", Content: buf.Bytes()}}, nil
}

func init() { Register(isorTarget{}) }

// serviceMap renders one Service as a plain mapping: known fields first
// (canonical order), Extra keys appended after.
func serviceMap(s *model.Service) map[string]any {
	m := map[string]any{}
	if s.Image != "" {
		m["image"] = s.Image
	}
	if s.Build != "" {
		m["build"] = s.Build
	}
	if len(s.Command) > 0 {
		m["command"] = s.Command
	}
	if s.Replicas != 1 {
		m["replicas"] = s.Replicas
	}
	if len(s.Ports) > 0 {
		ports := make([]any, len(s.Ports))
		for i, p := range s.Ports {
			ports[i] = p.Compose()
		}
		m["ports"] = ports
	}
	if len(s.Environment) > 0 {
		m["environment"] = s.Environment
	}
	if len(s.DependsOn) > 0 {
		m["depends_on"] = s.DependsOn
	}
	if len(s.Volumes) > 0 {
		m["volumes"] = s.Volumes
	}
	if len(s.Networks) > 0 {
		m["networks"] = s.Networks
	}
	if s.Restart != "" {
		m["restart"] = s.Restart
	}
	if len(s.Labels) > 0 {
		m["labels"] = s.Labels
	}
	for k, v := range s.Extra {
		m[k] = v
	}
	return m
}

// setPair appends key:value to a mapping node, encoding v via yaml.
func setPair(root *yaml.Node, key string, v any) {
	kn := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	vn := &yaml.Node{}
	if err := vn.Encode(v); err != nil {
		vn = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: fmt.Sprint(v)}
	}
	root.Content = append(root.Content, kn, vn)
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
