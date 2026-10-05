package parse

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
)

// terraformImporter lifts the HCL emitted by the terraform target back to
// model.Infra. It is a bounded parser for the emitted shape
// (resource "docker_container" "x" { ... } with flat attributes and
// ports/volumes/labels blocks) — not a general HCL parser. Hand-written
// tf files with expressions will fail loudly on unrecognized lines.
type terraformImporter struct{}

func (terraformImporter) Name() string { return "terraform" }
func (terraformImporter) Description() string {
	return "main.tf (docker provider, isorion-emitted shape)"
}

var (
	tfResourceRe  = regexp.MustCompile(`^resource "(docker_container|docker_network|docker_volume)" "([^"]+)" \{`)
	tfAttrRe      = regexp.MustCompile(`^([a-z_]+)\s*=\s*(.+)$`)
	tfStrListRe   = regexp.MustCompile(`"([^"]*)"`)
	tfSecretVarRe = regexp.MustCompile(`\$\{var\.secret_([^}]+)\}`)
	tfDependsRe   = regexp.MustCompile(`docker_container\.([A-Za-z0-9_-]+)`)
	nestedBlockRe = regexp.MustCompile(`^(ports|volumes|labels|networks_advanced) \{$`)
)

func (terraformImporter) Parse(data []byte, filename string) (*model.Infra, error) {
	infra := &model.Infra{
		Version:  "1.0",
		Services: map[string]*model.Service{},
		Networks: map[string]any{},
		Volumes:  map[string]any{},
		Other:    map[string]any{},
	}

	// block context: which resource we're inside, and which nested block.
	var cur *model.Service
	var resType, resName, nested string
	var nestedKV map[string]string
	depth := 0

	flush := func() {
		switch resType {
		case "docker_network":
			infra.Networks[resName] = map[string]any{}
		case "docker_volume":
			infra.Volumes[resName] = map[string]any{}
		case "docker_container":
			if cur != nil {
				infra.Services[resName] = cur
				infra.ServiceNames = append(infra.ServiceNames, resName)
			}
		}
		cur, resType, resName = nil, "", ""
	}

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if m := tfResourceRe.FindStringSubmatch(line); m != nil {
			resType, resName = m[1], m[2]
			if resType == "docker_container" {
				cur = &model.Service{Name: resName, Replicas: 1, Extra: map[string]any{}}
			}
			depth += strings.Count(line, "{") - strings.Count(line, "}")
			continue
		}
		if resType == "" {
			// terraform{} / provider / variable blocks — ignored wholesale.
			depth += strings.Count(line, "{") - strings.Count(line, "}")
			continue
		}
		if nested == "" && nestedBlockRe.MatchString(line) {
			nested = nestedBlockRe.FindStringSubmatch(line)[1]
			nestedKV = map[string]string{}
			depth += strings.Count(line, "{") - strings.Count(line, "}")
			continue
		}
		if line == "}" {
			depth--
			if nested != "" {
				if err := applyNested(cur, nested, nestedKV); err != nil {
					return nil, fmt.Errorf("%s: %s: %w", filename, resName, err)
				}
				nested, nestedKV = "", nil
			} else if depth == 0 {
				flush()
			}
			continue
		}
		if cur != nil {
			if m := tfAttrRe.FindStringSubmatch(line); m != nil {
				if nested != "" {
					nestedKV[m[1]] = m[2]
				} else if err := applyAttr(cur, m[1], m[2]); err != nil {
					return nil, fmt.Errorf("%s: %s: %w", filename, resName, err)
				}
			}
		}
	}
	if resType != "" {
		flush()
	}
	sort.Strings(infra.ServiceNames)
	if len(infra.Services) == 0 {
		return nil, fmt.Errorf("%s: no docker_container resources found", filename)
	}
	return infra, nil
}

func init() { Register(terraformImporter{}) }

// applyAttr sets a flat docker_container attribute on the service.
func applyAttr(s *model.Service, k, v string) error {
	switch k {
	case "name":
		s.Name = unquoteHCL(v)
	case "image":
		s.Image = unquoteHCL(v)
	case "command":
		s.Command = unquoteHCLList(v)
	case "env":
		if s.Environment == nil {
			s.Environment = map[string]string{}
		}
		for _, e := range unquoteHCLListKeepExpr(v) {
			kv, val, ok := strings.Cut(e, "=")
			if !ok {
				continue
			}
			if m := tfSecretVarRe.FindStringSubmatch(val); m != nil {
				s.Environment[kv] = lang.SecretMarker + m[1]
			} else {
				s.Environment[kv] = val
			}
		}
	case "depends_on":
		for _, m := range tfDependsRe.FindAllStringSubmatch(v, -1) {
			s.DependsOn = append(s.DependsOn, m[1])
		}
	case "restart":
		s.Restart = unquoteHCL(v)
	}
	return nil
}

// applyNested folds a completed ports/volumes/labels/networks_advanced
// block into the service.
func applyNested(s *model.Service, kind string, kv map[string]string) error {
	if s == nil {
		return nil
	}
	switch kind {
	case "ports":
		p := model.PortMap{Protocol: "tcp"}
		if v, ok := kv["internal"]; ok {
			if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
				p.Container = n
			}
		}
		if v, ok := kv["external"]; ok {
			if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
				p.Host = n
			}
		}
		if v := unquoteHCL(kv["protocol"]); v != "" {
			p.Protocol = v
		}
		s.Ports = append(s.Ports, p)
	case "volumes":
		src := unquoteHCL(kv["volume_name"])
		dst := unquoteHCL(kv["container_path"])
		if src != "" || dst != "" {
			s.Volumes = append(s.Volumes, src+":"+dst)
		}
	case "labels":
		if s.Labels == nil {
			s.Labels = map[string]any{}
		}
		s.Labels[unquoteHCL(kv["label"])] = unquoteHCL(kv["value"])
	case "networks_advanced":
		if v := unquoteHCL(kv["name"]); v != "" {
			// emitted name may be project-prefixed (<project>_<net>)
			s.Networks = append(s.Networks, v)
		}
	}
	return nil
}

func unquoteHCL(v string) string {
	return strings.Trim(v, `"`)
}

// unquoteHCLList keeps raw expressions too ("${var.secret_x}" survives
// because hclList wrote it inside a quoted string already).
func unquoteHCLList(v string) []string {
	var out []string
	for _, m := range tfStrListRe.FindAllStringSubmatch(v, -1) {
		out = append(out, m[1])
	}
	return out
}

func unquoteHCLListKeepExpr(v string) []string { return unquoteHCLList(v) }
