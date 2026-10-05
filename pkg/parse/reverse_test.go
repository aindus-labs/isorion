package parse

import (
	"reflect"
	"strings"
	"testing"

	"github.com/isorion-lang/isorion/pkg/emit"
	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
)

func loadInfra(t *testing.T, src string) (*lang.Document, *model.Infra) {
	t.Helper()
	doc, err := lang.LoadWith("x.isor", []byte(src), lang.Options{})
	if err != nil {
		t.Fatal(err)
	}
	infra, err := model.FromBody(doc.Version, doc.Body)
	if err != nil {
		t.Fatal(err)
	}
	return doc, infra
}

const k8sSrc = `version: "1.0"
project: { name: webapp }
services:
  api:
    image: "nginx:1.25"
    replicas: 3
    ports: ["80:8080"]
    environment: { APP_ENV: prod }
  db:
    image: "postgres:16"
`

// TestKubernetesRoundTrip: .isor → k8s manifests → model.
func TestKubernetesRoundTrip(t *testing.T) {
	doc, infra := loadInfra(t, k8sSrc)
	files, _, err := emit.Run("kubernetes", doc, infra)
	if err != nil {
		t.Fatal(err)
	}
	// concatenate as a multi-doc stream (like fusion --stdout)
	var stream strings.Builder
	for _, f := range files {
		stream.Write(f.Content)
		stream.WriteString("---\n")
	}
	infra2, err := (kubernetesImporter{}).Parse([]byte(stream.String()), "k8s.yaml")
	if err != nil {
		t.Fatalf("emitted manifests do not re-import: %v\n%s", err, stream.String())
	}
	if infra2.ProjectName != infra.ProjectName {
		t.Fatalf("project: %q vs %q", infra2.ProjectName, infra.ProjectName)
	}
	if !reflect.DeepEqual(infra2.ServiceNames, infra.ServiceNames) {
		t.Fatalf("names: %v vs %v", infra2.ServiceNames, infra.ServiceNames)
	}
	for _, n := range infra.ServiceNames {
		got, want := infra2.Services[n], infra.Services[n]
		if got.Image != want.Image || got.Replicas != want.Replicas ||
			!reflect.DeepEqual(got.Command, want.Command) ||
			!reflect.DeepEqual(got.Ports, want.Ports) ||
			!reflect.DeepEqual(got.Environment, want.Environment) {
			t.Fatalf("service %s changed:\n%+v\n%+v", n, got, want)
		}
	}
}

func TestKubernetesListKind(t *testing.T) {
	doc, infra := loadInfra(t, k8sSrc)
	files, _, err := emit.Run("kubernetes", doc, infra)
	if err != nil {
		t.Fatal(err)
	}
	// wrap in kind: List (kubectl get -o yaml shape)
	var b strings.Builder
	b.WriteString("kind: List\napiVersion: v1\nitems:\n")
	for _, f := range files {
		lines := strings.Split(strings.TrimSpace(string(f.Content)), "\n")
		b.WriteString("  - " + strings.Join(lines, "\n    ") + "\n")
	}
	raw := b.String()
	infra2, err := (kubernetesImporter{}).Parse([]byte(raw), "list.yaml")
	if err != nil {
		t.Fatalf("List parse: %v\n%s", err, raw)
	}
	if len(infra2.Services) != len(infra.Services) {
		t.Fatalf("services: %d vs %d", len(infra2.Services), len(infra.Services))
	}
}

func TestKubernetesErrors(t *testing.T) {
	cases := map[string]string{
		"empty":        "# nothing\n",
		"bad yaml":     "kind: [",
		"no workloads": "kind: ConfigMap\nmetadata: {name: x}\n",
		"no containers": `kind: Deployment
metadata: {name: a}
spec: {template: {spec: {containers: []}}}
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := (kubernetesImporter{}).Parse([]byte(src), "k.yaml"); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

// TestHuborionRoundTrip: .isor → huborion dialect → model.
func TestHuborionRoundTrip(t *testing.T) {
	src := `version: "1.0"
project: { name: webapp, description: "test capsule" }
services:
  api:
    image: "nginx:1.25"
    replicas: 3
    ports: ["80:8080"]
    environment: { APP_ENV: prod }
    depends_on: [db]
  db:
    image: "postgres:16"
`
	doc, infra := loadInfra(t, src)
	files, _, err := emit.Run("huborion", doc, infra)
	if err != nil {
		t.Fatal(err)
	}
	infra2, err := (huborionImporter{}).Parse(files[0].Content, "webapp.huborion.isor")
	if err != nil {
		t.Fatalf("emitted huborion does not re-import: %v\n%s", err, files[0].Content)
	}
	if infra2.ProjectName != "webapp" || infra2.Description != "test capsule" {
		t.Fatalf("meta: %+v", infra2)
	}
	if !reflect.DeepEqual(infra2.ServiceNames, infra.ServiceNames) {
		t.Fatalf("names: %v vs %v", infra2.ServiceNames, infra.ServiceNames)
	}
	for _, n := range infra.ServiceNames {
		got, want := infra2.Services[n], infra.Services[n]
		if got.Image != want.Image || got.Replicas != want.Replicas ||
			!reflect.DeepEqual(got.Ports, want.Ports) ||
			!reflect.DeepEqual(got.Environment, want.Environment) ||
			!reflect.DeepEqual(got.DependsOn, want.DependsOn) {
			t.Fatalf("service %s changed:\n%+v\n%+v", n, got, want)
		}
	}
}

func TestHuborionErrors(t *testing.T) {
	if _, err := (huborionImporter{}).Parse([]byte("services: {a: {}}\n"), "x.isor"); err == nil {
		t.Fatal("expected error for non-graph doc")
	}
}
