package parse

import (
	"fmt"
	"sort"
	"strings"

	"github.com/isorion-lang/isorion/pkg/model"
	"gopkg.in/yaml.v3"
)

// docker-compose importer — the reference importer (near 1:1 mapping with
// the model). Handles both list and mapping forms (depends_on, networks,
// environment, labels) and `deploy.replicas`.
type composeImporter struct{}

func (composeImporter) Name() string        { return "docker-compose" }
func (composeImporter) Description() string { return "docker-compose.yml services/volumes/networks" }

func (composeImporter) Parse(data []byte, filename string) (*model.Infra, error) {
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

	for k, v := range doc {
		switch k {
		case "name": // compose project name
			if s, ok := v.(string); ok {
				infra.ProjectName = s
			}
		case "version": // legacy compose file version — not the .isor one
		case "services":
			m, ok := v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s: services: must be a mapping", filename)
			}
			for name, raw := range m {
				svc, err := composeService(name, raw)
				if err != nil {
					return nil, fmt.Errorf("%s: services.%s: %w", filename, name, err)
				}
				infra.Services[name] = svc
				infra.ServiceNames = append(infra.ServiceNames, name)
			}
			sort.Strings(infra.ServiceNames)
		case "networks":
			if m, ok := v.(map[string]any); ok {
				infra.Networks = m
			} else if l, ok := v.([]any); ok {
				for _, e := range l {
					infra.Networks[fmt.Sprint(e)] = map[string]any{}
				}
			}
		case "volumes":
			if m, ok := v.(map[string]any); ok {
				infra.Volumes = m
			}
		default:
			infra.Other[k] = v
		}
	}
	return infra, nil
}

func init() { Register(composeImporter{}) }

func composeService(name string, raw any) (*model.Service, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("service definition must be a mapping")
	}
	s := &model.Service{Name: name, Replicas: 1, Extra: map[string]any{}}
	for k, v := range m {
		switch k {
		case "image":
			s.Image = strOf(v)
		case "build":
			s.Build = buildRef(v)
		case "command", "entrypoint":
			s.Command = append(s.Command, strList(v)...)
		case "deploy":
			if dm, ok := v.(map[string]any); ok {
				if n, ok := toInt(dm["replicas"]); ok {
					s.Replicas = n
				}
			} else {
				s.Extra[k] = v
			}
		case "ports":
			list, ok := v.([]any)
			if !ok {
				return nil, fmt.Errorf("ports: must be a list")
			}
			for _, p := range list {
				pm, err := model.ParsePort(portString(p))
				if err != nil {
					return nil, fmt.Errorf("ports: %w", err)
				}
				s.Ports = append(s.Ports, pm)
			}
		case "environment":
			s.Environment = envMap(v)
		case "depends_on":
			s.DependsOn = nameList(v)
		case "volumes":
			s.Volumes = strList(v)
		case "networks":
			s.Networks = nameList(v)
		case "restart":
			s.Restart = strOf(v)
		case "labels":
			s.Labels = labelMap(v)
		default:
			s.Extra[k] = v
		}
	}
	if s.Image == "" && s.Build == "" {
		return nil, fmt.Errorf("either image or build is required")
	}
	return s, nil
}

// buildRef accepts the short (`./dir`) and long (`{context, dockerfile}`)
// build forms; the long form keeps context only (dockerfile lands in Extra
// via build.dockerfile loss is accepted — recorded in Extra? no: keep it
// simple, context wins as model.Build).
func buildRef(v any) string {
	switch b := v.(type) {
	case string:
		return b
	case map[string]any:
		return strOf(b["context"])
	}
	return ""
}

// portString normalizes a compose port entry to the "h:c[/proto]" form
// ParsePort understands. Compose long syntax {published, target, protocol}
// is flattened.
func portString(p any) string {
	if m, ok := p.(map[string]any); ok {
		var b strings.Builder
		if ip := strOf(m["host_ip"]); ip != "" {
			b.WriteString(ip + ":")
		}
		if pub := strOf(m["published"]); pub != "" {
			b.WriteString(pub + ":")
		}
		b.WriteString(strOf(m["target"]))
		if proto := strOf(m["protocol"]); proto != "" && proto != "tcp" {
			b.WriteString("/" + proto)
		}
		return b.String()
	}
	return strOf(p)
}

// envMap accepts both `{K: V}` and `- K=V` environment forms.
func envMap(v any) map[string]string {
	out := map[string]string{}
	switch e := v.(type) {
	case map[string]any:
		for k, x := range e {
			out[k] = strOf(x)
		}
	case []any:
		for _, item := range e {
			s := strOf(item)
			if i := strings.IndexByte(s, '='); i >= 0 {
				out[s[:i]] = s[i+1:]
			} else {
				out[s] = ""
			}
		}
	}
	return out
}

// nameList accepts `- name` lists and `{name: cfg}` maps (depends_on,
// networks) — map values are dropped, keys become the list.
func nameList(v any) []string {
	var out []string
	switch l := v.(type) {
	case []any:
		for _, e := range l {
			out = append(out, strOf(e))
		}
	case map[string]any:
		for k := range l {
			out = append(out, k)
		}
		sort.Strings(out)
	}
	return out
}

// labelMap accepts both `{K: V}` and `- K=V` label forms.
func labelMap(v any) map[string]any {
	out := map[string]any{}
	switch l := v.(type) {
	case map[string]any:
		return l
	case []any:
		for _, item := range l {
			s := strOf(item)
			if i := strings.IndexByte(s, '='); i >= 0 {
				out[s[:i]] = s[i+1:]
			} else {
				out[s] = true
			}
		}
	}
	return out
}

func strList(v any) []string {
	var out []string
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			out = append(out, strOf(e))
		}
	case string:
		out = append(out, x)
	}
	return out
}

func strOf(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func toInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	}
	return 0, false
}
