package emit

import (
	"fmt"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
	"gopkg.in/yaml.v3"
)

// ansibleEmitter emits an Ansible playbook using community.docker.docker_container.
type ansibleEmitter struct{}

func init() { Register(ansibleEmitter{}) }

func (ansibleEmitter) Name() string { return "ansible" }
func (ansibleEmitter) Description() string {
	return "Ansible playbook (community.docker.docker_container)"
}

func (ansibleEmitter) Emit(doc *lang.Document, infra *model.Infra) ([]File, error) {
	if len(infra.Services) == 0 {
		return nil, fmt.Errorf("ansible target requires at least one service")
	}
	var tasks []any
	for _, name := range infra.ServiceNames {
		s := infra.Services[name]
		containerName := name
		if infra.ProjectName != "" {
			containerName = infra.ProjectName + "-" + name
		}
		mod := map[string]any{
			"name":  containerName,
			"state": "started",
		}
		if s.Image != "" {
			mod["image"] = s.Image
		}
		if len(s.Command) > 0 {
			mod["command"] = s.Command
		}
		if len(s.Ports) > 0 {
			ports := make([]string, len(s.Ports))
			for i, p := range s.Ports {
				ports[i] = p.Compose()
			}
			mod["ports"] = ports
		}
		if len(s.Environment) > 0 {
			env := map[string]string{}
			for k, v := range s.Environment {
				if sn, ok := lang.SecretName(v); ok {
					// resolved from SECRET_<name> at playbook run time
					env[k] = "{{ lookup('ansible.builtin.env', 'SECRET_" + sn + "') }}"
				} else {
					env[k] = v
				}
			}
			mod["env"] = env
		}
		if len(s.Volumes) > 0 {
			mod["volumes"] = s.Volumes
		}
		if s.Restart != "" {
			mod["restart_policy"] = s.Restart
		}
		tasks = append(tasks, map[string]any{
			"name":                              "Deploy " + containerName,
			"community.docker.docker_container": mod,
		})
	}

	playbook := []any{
		map[string]any{
			"name":  "Deploy " + infra.ProjectName,
			"hosts": "all",
			"tasks": tasks,
		},
	}
	data, err := yaml.Marshal(playbook)
	if err != nil {
		return nil, fmt.Errorf("ansible emit: %w", err)
	}
	return []File{{Path: "playbook.yml", Content: data}}, nil
}

// Warn reports model fields silently dropped by this target.
func (ansibleEmitter) Warn(_ *lang.Document, infra *model.Infra) []string {
	var out []string
	for _, name := range infra.ServiceNames {
		s := infra.Services[name]
		for k := range s.Extra {
			out = append(out, fmt.Sprintf("services.%s.%s ignored by target ansible", name, k))
		}
		if s.Build != "" {
			out = append(out, fmt.Sprintf("services.%s.build ignored by target ansible (docker_container needs an image)", name))
		}
		if len(s.Networks) > 0 {
			out = append(out, fmt.Sprintf("services.%s.networks not emitted by target ansible", name))
		}
		if len(s.Labels) > 0 {
			out = append(out, fmt.Sprintf("services.%s.labels not emitted by target ansible", name))
		}
	}
	return out
}
