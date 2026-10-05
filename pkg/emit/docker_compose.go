package emit

import (
	"fmt"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
	"gopkg.in/yaml.v3"
)

// composeEmitter emits a Docker Compose v3 file.
type composeEmitter struct{}

func init() { Register(composeEmitter{}) }

func (composeEmitter) Name() string        { return "docker-compose" }
func (composeEmitter) Description() string { return "Docker Compose file (docker-compose.yml)" }

func (composeEmitter) Emit(doc *lang.Document, infra *model.Infra) ([]File, error) {
	services := map[string]any{}
	for _, name := range infra.ServiceNames {
		s := infra.Services[name]
		svc := map[string]any{}
		if s.Image != "" {
			svc["image"] = s.Image
		}
		if s.Build != "" {
			svc["build"] = s.Build
		}
		if len(s.Command) > 0 {
			svc["command"] = s.Command
		}
		if s.Replicas != 1 {
			svc["deploy"] = map[string]any{"replicas": s.Replicas}
		}
		if len(s.Ports) > 0 {
			ports := make([]string, len(s.Ports))
			for i, p := range s.Ports {
				ports[i] = p.Compose()
			}
			svc["ports"] = ports
		}
		if len(s.Environment) > 0 {
			env := map[string]string{}
			for k, v := range s.Environment {
				if sn, ok := lang.SecretName(v); ok {
					// runtime env var: secret never lands in the file
					env[k] = "${SECRET_" + sn + "}"
				} else {
					env[k] = v
				}
			}
			svc["environment"] = env
		}
		if len(s.DependsOn) > 0 {
			svc["depends_on"] = s.DependsOn
		}
		if len(s.Volumes) > 0 {
			svc["volumes"] = s.Volumes
		}
		if len(s.Networks) > 0 {
			svc["networks"] = s.Networks
		}
		if s.Restart != "" {
			svc["restart"] = s.Restart
		}
		if len(s.Labels) > 0 {
			svc["labels"] = s.Labels
		}
		for k, v := range s.Extra {
			svc[k] = v // pass-through (healthcheck, logging, ...)
		}
		services[name] = svc
	}

	out := map[string]any{"services": services}
	if infra.ProjectName != "" {
		out["name"] = infra.ProjectName
	}
	if len(infra.Networks) > 0 {
		out["networks"] = infra.Networks
	}
	if len(infra.Volumes) > 0 {
		out["volumes"] = infra.Volumes
	}

	// secrets: → top-level external secrets (injected at run time by the
	// platform/sops, never decrypted here).
	if len(doc.Secrets) > 0 {
		decl := map[string]any{}
		for _, n := range sortedSecretNames(doc.Secrets) {
			decl[n] = map[string]any{"external": true}
		}
		out["secrets"] = decl
	}

	data, err := yaml.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("docker-compose emit: %w", err)
	}
	return []File{{Path: "docker-compose.yml", Content: data}}, nil
}
