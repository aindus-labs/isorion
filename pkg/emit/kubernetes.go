package emit

import (
	"fmt"
	"sort"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
	"gopkg.in/yaml.v3"
)

// k8sEmitter emits Kubernetes manifests: one Deployment per service,
// plus a ClusterIP Service when ports are declared.
type k8sEmitter struct{}

func init() { Register(k8sEmitter{}) }

func (k8sEmitter) Name() string { return "kubernetes" }
func (k8sEmitter) Description() string {
	return "Kubernetes manifests (Deployment + Service per service)"
}

func (k8sEmitter) Emit(doc *lang.Document, infra *model.Infra) ([]File, error) {
	if len(infra.Services) == 0 {
		return nil, fmt.Errorf("kubernetes target requires at least one service")
	}
	var files []File
	secretName := secretObjectName(infra.ProjectName)

	for _, name := range infra.ServiceNames {
		s := infra.Services[name]
		labels := map[string]any{
			"app.kubernetes.io/name":       name,
			"app.kubernetes.io/managed-by": "isorion",
		}
		if infra.ProjectName != "" {
			labels["app.kubernetes.io/part-of"] = infra.ProjectName
		}

		var env []any
		for _, k := range sortedEnvKeys(s.Environment) {
			v := s.Environment[k]
			if sn, ok := lang.SecretName(v); ok {
				env = append(env, map[string]any{
					"name": k,
					"valueFrom": map[string]any{
						"secretKeyRef": map[string]any{"name": secretName, "key": sn},
					},
				})
			} else {
				env = append(env, map[string]any{"name": k, "value": v})
			}
		}
		var cports []any
		for _, p := range s.Ports {
			cports = append(cports, map[string]any{
				"containerPort": p.Container,
				"protocol":      upper(p.Protocol),
			})
		}

		container := map[string]any{"name": name}
		if s.Image != "" {
			container["image"] = s.Image
		}
		if len(s.Command) > 0 {
			container["command"] = s.Command
		}
		if len(env) > 0 {
			container["env"] = env
		}
		if len(cports) > 0 {
			container["ports"] = cports
		}

		deployment := map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]any{"name": name, "labels": labels},
			"spec": map[string]any{
				"replicas": s.Replicas,
				"selector": map[string]any{
					"matchLabels": map[string]any{"app.kubernetes.io/name": name},
				},
				"template": map[string]any{
					"metadata": map[string]any{"labels": labels},
					"spec": map[string]any{
						"containers": []any{container},
					},
				},
			},
		}
		data, err := yaml.Marshal(deployment)
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: name + "-deployment.yaml", Content: data})

		if len(s.Ports) > 0 {
			var sport []any
			for _, p := range s.Ports {
				port := p.Host
				if port == 0 {
					port = p.Container
				}
				sport = append(sport, map[string]any{
					"port":       port,
					"targetPort": p.Container,
					"protocol":   upper(p.Protocol),
				})
			}
			service := map[string]any{
				"apiVersion": "v1",
				"kind":       "Service",
				"metadata":   map[string]any{"name": name, "labels": labels},
				"spec": map[string]any{
					"type":     "ClusterIP",
					"selector": map[string]any{"app.kubernetes.io/name": name},
					"ports":    sport,
				},
			}
			data, err = yaml.Marshal(service)
			if err != nil {
				return nil, err
			}
			files = append(files, File{Path: name + "-service.yaml", Content: data})
		}
	}

	// secrets: → one Opaque Secret holding the ENC[...] ciphertext (sops/age
	// decrypts at deploy time — plaintext never reaches the artifact).
	if len(doc.Secrets) > 0 {
		data := map[string]any{}
		for _, n := range sortedSecretNames(doc.Secrets) {
			data[n] = doc.Secrets[n]
		}
		secret := map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":   secretName,
				"labels": map[string]any{"app.kubernetes.io/managed-by": "isorion"},
			},
			"type":       "Opaque",
			"stringData": data,
		}
		data2, err := yaml.Marshal(secret)
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: secretName + "-secret.yaml", Content: data2})
	}
	return files, nil
}

// Warn reports model fields silently dropped by this target.
func (k8sEmitter) Warn(_ *lang.Document, infra *model.Infra) []string {
	var out []string
	for _, name := range infra.ServiceNames {
		s := infra.Services[name]
		for _, k := range sortedAnyKeys(s.Extra) {
			out = append(out, fmt.Sprintf("services.%s.%s ignored by target kubernetes", name, k))
		}
		if len(s.DependsOn) > 0 {
			out = append(out, fmt.Sprintf("services.%s.depends_on ignored by target kubernetes (orchestrator-managed)", name))
		}
		if s.Restart != "" {
			out = append(out, fmt.Sprintf("services.%s.restart ignored by target kubernetes (restartPolicy is always Always)", name))
		}
		if len(s.Volumes) > 0 {
			out = append(out, fmt.Sprintf("services.%s.volumes ignored by target kubernetes", name))
		}
		if len(s.Networks) > 0 {
			out = append(out, fmt.Sprintf("services.%s.networks ignored by target kubernetes", name))
		}
	}
	return out
}

func sortedEnvKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedAnyKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func upper(s string) string {
	out := ""
	for _, c := range s {
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out += string(c)
	}
	return out
}
