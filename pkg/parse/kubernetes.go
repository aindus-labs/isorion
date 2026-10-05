package parse

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
	"gopkg.in/yaml.v3"
)

// kubernetesImporter lifts k8s manifests back into model.Infra. Accepts a
// multi-document stream (`---` separated, e.g. `fusion --stdout` output or
// `kubectl get -o yaml`) and `kind: List`. Deployments (and Pod-like docs)
// become services; Service docs merge their published ports into the
// matching workload by selector. Other kinds (ConfigMap, Ingress,
// StatefulSet...) are skipped — document the loss in Warn, do not guess.
type kubernetesImporter struct{}

func (kubernetesImporter) Name() string { return "kubernetes" }
func (kubernetesImporter) Description() string {
	return "k8s manifests (Deployment/Service, multi-doc or List)"
}

func (kubernetesImporter) Parse(data []byte, filename string) (*model.Infra, error) {
	infra := &model.Infra{
		Version:  "1.0",
		Services: map[string]*model.Service{},
		Networks: map[string]any{},
		Volumes:  map[string]any{},
		Other:    map[string]any{},
	}
	var services []*k8sServiceDoc // published ports, applied after workloads

	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	for {
		var doc map[string]any
		if err := dec.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("%s: YAML parse error: %w", filename, err)
		}
		if len(doc) == 0 {
			continue
		}
		docs := []map[string]any{doc}
		if strOf(doc["kind"]) == "List" {
			docs = docs[:0]
			if items, ok := doc["items"].([]any); ok {
				for _, it := range items {
					if m, ok := it.(map[string]any); ok {
						docs = append(docs, m)
					}
				}
			}
		}
		for _, d := range docs {
			switch strOf(d["kind"]) {
			case "Deployment", "Pod":
				svc, err := k8sWorkload(d)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", filename, err)
				}
				infra.Services[svc.Name] = svc
				if p := partOf(d); p != "" {
					infra.ProjectName = p
				}
			case "Service":
				services = append(services, &k8sServiceDoc{doc: d})
			default:
				// ConfigMap/Secret/Ingress/... : no model counterpart — skipped.
			}
		}
	}

	// merge published ports: spec.ports[].port → Host, targetPort → Container.
	for _, sd := range services {
		target := sd.selectorName()
		svc := infra.Services[target]
		if svc == nil {
			svc = infra.Services[sd.name()]
		}
		if svc == nil {
			continue
		}
		for _, p := range sd.ports() {
			for i, pm := range svc.Ports {
				if pm.Container == p.Container && pm.Protocol == p.Protocol {
					if p.Host != pm.Container {
						svc.Ports[i].Host = p.Host
					}
				}
			}
		}
	}

	for name := range infra.Services {
		infra.ServiceNames = append(infra.ServiceNames, name)
	}
	sort.Strings(infra.ServiceNames)
	if len(infra.Services) == 0 {
		return nil, fmt.Errorf("%s: no Deployment/Pod found", filename)
	}
	return infra, nil
}

func init() { Register(kubernetesImporter{}) }

// k8sWorkload extracts a model.Service from a Deployment or Pod document.
func k8sWorkload(d map[string]any) (*model.Service, error) {
	meta, _ := d["metadata"].(map[string]any)
	name := strOf(meta["name"])
	if name == "" {
		return nil, fmt.Errorf("%s: missing metadata.name", strOf(d["kind"]))
	}
	spec, _ := d["spec"].(map[string]any)
	// Pod: containers at spec level; Deployment: under template.spec.
	podSpec := spec
	if tpl, ok := spec["template"].(map[string]any); ok {
		podSpec, _ = tpl["spec"].(map[string]any)
	}
	s := &model.Service{Name: name, Replicas: 1, Extra: map[string]any{}}
	if n, ok := toInt(spec["replicas"]); ok {
		s.Replicas = n
	}
	containers, _ := podSpec["containers"].([]any)
	if len(containers) == 0 {
		return nil, fmt.Errorf("%s %s: no containers", strOf(d["kind"]), name)
	}
	c, _ := containers[0].(map[string]any)
	s.Image = strOf(c["image"])
	s.Command = strList(c["command"])
	if len(containers) > 1 {
		var sidecars []string
		for _, extra := range containers[1:] {
			if em, ok := extra.(map[string]any); ok {
				sidecars = append(sidecars, strOf(em["name"]))
			}
		}
		s.Extra["sidecars_dropped"] = sidecars
	}
	for _, e := range listOf(c["env"]) {
		em, _ := e.(map[string]any)
		n := strOf(em["name"])
		if n == "" {
			continue
		}
		if s.Environment == nil {
			s.Environment = map[string]string{}
		}
		if vf, ok := em["valueFrom"].(map[string]any); ok {
			if ref, ok := vf["secretKeyRef"].(map[string]any); ok {
				s.Environment[n] = lang.SecretMarker + strOf(ref["key"])
				continue
			}
		}
		s.Environment[n] = strOf(em["value"])
	}
	for _, p := range listOf(c["ports"]) {
		pm, _ := p.(map[string]any)
		cp, ok := toInt(pm["containerPort"])
		if !ok {
			continue
		}
		s.Ports = append(s.Ports, model.PortMap{
			Container: cp,
			Protocol:  strings.ToLower(strOf(pm["protocol"])),
		})
	}
	for i := range s.Ports {
		if s.Ports[i].Protocol == "" {
			s.Ports[i].Protocol = "tcp"
		}
	}
	if s.Image == "" && s.Build == "" {
		return nil, fmt.Errorf("%s %s: containers[0].image missing", strOf(d["kind"]), name)
	}
	return s, nil
}

// k8sServiceDoc is a parsed `kind: Service` used to lift published ports.
type k8sServiceDoc struct {
	doc map[string]any
}

func (sd *k8sServiceDoc) name() string {
	meta, _ := sd.doc["metadata"].(map[string]any)
	return strOf(meta["name"])
}

// selectorName returns the app.kubernetes.io/name selector value, used to
// match the workload this Service fronts.
func (sd *k8sServiceDoc) selectorName() string {
	spec, _ := sd.doc["spec"].(map[string]any)
	sel, _ := spec["selector"].(map[string]any)
	return strOf(sel["app.kubernetes.io/name"])
}

func (sd *k8sServiceDoc) ports() []model.PortMap {
	spec, _ := sd.doc["spec"].(map[string]any)
	var out []model.PortMap
	for _, p := range listOf(spec["ports"]) {
		pm, _ := p.(map[string]any)
		tp, ok := toInt(pm["targetPort"])
		if !ok {
			continue
		}
		port, _ := toInt(pm["port"])
		out = append(out, model.PortMap{
			Host:      port,
			Container: tp,
			Protocol:  strings.ToLower(strOf(pm["protocol"])),
		})
	}
	return out
}

// partOf reads the app.kubernetes.io/part-of label (project grouping).
func partOf(d map[string]any) string {
	meta, _ := d["metadata"].(map[string]any)
	labels, _ := meta["labels"].(map[string]any)
	return strOf(labels["app.kubernetes.io/part-of"])
}

func listOf(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return nil
}
