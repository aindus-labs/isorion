package parse

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
	"gopkg.in/yaml.v3"
)

// ansibleImporter lifts a community.docker playbook back to model.Infra:
// each task's docker_container module becomes a service. Container names
// emitted as `<project>-<svc>` are split back on their common prefix.
type ansibleImporter struct{}

func (ansibleImporter) Name() string { return "ansible" }
func (ansibleImporter) Description() string {
	return "playbook.yml (community.docker.docker_container)"
}

var ansibleSecretRe = regexp.MustCompile(`SECRET_([A-Za-z0-9_]+)`)

func (ansibleImporter) Parse(data []byte, filename string) (*model.Infra, error) {
	var plays []map[string]any
	if err := yaml.Unmarshal(data, &plays); err != nil {
		return nil, fmt.Errorf("%s: YAML parse error: %w", filename, err)
	}

	infra := &model.Infra{
		Version:  "1.0",
		Services: map[string]*model.Service{},
		Networks: map[string]any{},
		Volumes:  map[string]any{},
		Other:    map[string]any{},
	}

	for _, play := range plays {
		for _, t := range listOf(play["tasks"]) {
			task, _ := t.(map[string]any)
			mod, ok := task["community.docker.docker_container"].(map[string]any)
			if !ok {
				continue // non-container tasks are not services
			}
			svc, err := ansibleTask(mod)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", filename, err)
			}
			infra.Services[svc.Name] = svc
			infra.ServiceNames = append(infra.ServiceNames, svc.Name)
		}
	}
	if len(infra.Services) == 0 {
		return nil, fmt.Errorf("%s: no docker_container tasks found", filename)
	}

	// Recover the project prefix: emitted container names are `<project>-<svc>`.
	if prefix := commonServicePrefix(infra.ServiceNames); prefix != "" {
		infra.ProjectName = prefix
		for _, n := range infra.ServiceNames {
			svc := infra.Services[n]
			short := strings.TrimPrefix(n, prefix+"-")
			delete(infra.Services, n)
			infra.Services[short] = svc
			svc.Name = short
		}
		infra.ServiceNames = infra.ServiceNames[:0]
		for n := range infra.Services {
			infra.ServiceNames = append(infra.ServiceNames, n)
		}
	}
	sort.Strings(infra.ServiceNames)
	return infra, nil
}

func init() { Register(ansibleImporter{}) }

func ansibleTask(mod map[string]any) (*model.Service, error) {
	s := &model.Service{Name: strOf(mod["name"]), Replicas: 1, Extra: map[string]any{}}
	if s.Name == "" {
		return nil, fmt.Errorf("docker_container task without name")
	}
	s.Image = strOf(mod["image"])
	s.Command = strList(mod["command"])
	for _, p := range listOf(mod["ports"]) {
		pm, err := model.ParsePort(strOf(p))
		if err != nil {
			return nil, fmt.Errorf("service %s: ports: %w", s.Name, err)
		}
		s.Ports = append(s.Ports, pm)
	}
	if em, ok := mod["env"].(map[string]any); ok {
		s.Environment = map[string]string{}
		for k, v := range em {
			val := strOf(v)
			if m := ansibleSecretRe.FindStringSubmatch(val); m != nil && strings.Contains(val, "lookup") {
				s.Environment[k] = lang.SecretMarker + m[1]
			} else {
				s.Environment[k] = val
			}
		}
	}
	s.Volumes = strList(mod["volumes"])
	s.Restart = strOf(mod["restart_policy"])
	for k, v := range mod {
		switch k {
		case "name", "image", "command", "ports", "env", "volumes",
			"restart_policy", "state", "container_default_behavior":
		default:
			s.Extra[k] = v
		}
	}
	if s.Image == "" {
		return nil, fmt.Errorf("service %s: no image", s.Name)
	}
	return s, nil
}

// commonServicePrefix returns the shared `<word>-` prefix when every
// service name looks like `<prefix>-<svc>` — emitted by the ansible and
// systemd targets for container naming.
func commonServicePrefix(names []string) string {
	if len(names) == 0 {
		return ""
	}
	prefix := names[0]
	if i := strings.IndexByte(prefix, '-'); i >= 0 {
		prefix = prefix[:i]
	} else {
		return ""
	}
	for _, n := range names[1:] {
		if !strings.HasPrefix(n, prefix+"-") {
			return ""
		}
	}
	return prefix
}
