package parse

import (
	"reflect"
	"testing"

	"github.com/isorion-lang/isorion/pkg/emit"
	"github.com/isorion-lang/isorion/pkg/model"
)

// roundTrip runs src → target → importer and returns both Infra.
func roundTrip(t *testing.T, src, target string, imp Importer) (*model.Infra, *model.Infra, []emit.File) {
	t.Helper()
	doc, infra := loadInfra(t, src)
	files, _, err := emit.Run(target, doc, infra)
	if err != nil {
		t.Fatalf("emit %s: %v", target, err)
	}
	var data []byte
	for _, f := range files {
		// pick the primary payload file per target
		data = f.Content
		break
	}
	infra2, err := imp.Parse(data, target+".out")
	if err != nil {
		t.Fatalf("import %s: %v\n%s", target, err, data)
	}
	return infra, infra2, files
}

const rtSrc = `version: "1.0"
project: { name: webapp }
services:
  api:
    image: "nginx:1.25"
    ports: ["80:8080"]
    environment: { APP_ENV: prod }
    depends_on: [db]
    restart: unless-stopped
    volumes: ["./conf:/etc/nginx"]
    networks: [front]
  db:
    image: "postgres:16"
networks: { front: {} }
volumes: { data: {} }
`

// TestAnsibleRoundTrip: .isor → playbook → Infra.
func TestAnsibleRoundTrip(t *testing.T) {
	infra, infra2, _ := roundTrip(t, rtSrc, "ansible", ansibleImporter{})
	if infra2.ProjectName != "webapp" {
		t.Fatalf("project: %q", infra2.ProjectName)
	}
	if !reflect.DeepEqual(infra2.ServiceNames, infra.ServiceNames) {
		t.Fatalf("names: %v vs %v", infra2.ServiceNames, infra.ServiceNames)
	}
	api := infra2.Services["api"]
	want := infra.Services["api"]
	if api.Image != want.Image || !reflect.DeepEqual(api.Ports, want.Ports) ||
		!reflect.DeepEqual(api.Environment, want.Environment) ||
		!reflect.DeepEqual(api.Volumes, want.Volumes) || api.Restart != want.Restart {
		t.Fatalf("api changed:\n%+v\n%+v", api, want)
	}
	// depends_on is not emitted by ansible — loss is documented, not a bug.
}

// TestYamlRoundTrip: resolved.yaml is the identity importer.
func TestYamlRoundTrip(t *testing.T) {
	doc, infra := loadInfra(t, rtSrc)
	files, _, err := emit.Run("yaml", doc, infra)
	if err != nil {
		t.Fatal(err)
	}
	infra2, err := (yamlImporter{}).Parse(files[0].Content, "resolved.yaml")
	if err != nil {
		t.Fatalf("yaml import: %v", err)
	}
	if !reflect.DeepEqual(infra2.ServiceNames, infra.ServiceNames) {
		t.Fatalf("names: %v vs %v", infra2.ServiceNames, infra.ServiceNames)
	}
	for _, n := range infra.ServiceNames {
		if !reflect.DeepEqual(infra2.Services[n], infra.Services[n]) {
			t.Fatalf("service %s changed:\n%+v\n%+v", n, infra2.Services[n], infra.Services[n])
		}
	}
}

// TestHelmRoundTrip: .isor → values.yaml → Infra.
func TestHelmRoundTrip(t *testing.T) {
	infra, infra2, files := roundTrip(t, rtSrc, "helm", helmImporter{})
	var valuesFile []byte
	for _, f := range files {
		if f.Path == "webapp/values.yaml" {
			valuesFile = f.Content
		}
	}
	if valuesFile == nil {
		t.Fatal("values.yaml not emitted")
	}
	var err error
	infra2, err = (helmImporter{}).Parse(valuesFile, "values.yaml")
	if err != nil {
		t.Fatal(err)
	}
	api, want := infra2.Services["api"], infra.Services["api"]
	if api == nil || api.Image != want.Image || !reflect.DeepEqual(api.Ports, want.Ports) {
		t.Fatalf("api changed:\n%+v\n%+v", api, want)
	}
}

// TestSystemdRoundTrip: .isor → unit file → Infra (single unit).
func TestSystemdRoundTrip(t *testing.T) {
	src := `version: "1.0"
project: { name: webapp }
services:
  api:
    image: "nginx:1.25"
    ports: ["80:8080"]
    environment: { APP_ENV: prod }
    restart: unless-stopped
    volumes: ["./conf:/etc/nginx"]
`
	infra, infra2, files := roundTrip(t, src, "systemd", systemdImporter{})
	var unit []byte
	for _, f := range files {
		if f.Path == "webapp-api.service" {
			unit = f.Content
		}
	}
	if unit == nil {
		t.Fatal("unit file not emitted")
	}
	var err error
	infra2, err = (systemdImporter{}).Parse(unit, "webapp-api.service")
	if err != nil {
		t.Fatal(err)
	}
	if infra2.ProjectName != "webapp" {
		t.Fatalf("project: %q", infra2.ProjectName)
	}
	got, want := infra2.Services["api"], infra.Services["api"]
	if got == nil || got.Image != want.Image ||
		!reflect.DeepEqual(got.Ports, want.Ports) ||
		!reflect.DeepEqual(got.Volumes, want.Volumes) {
		t.Fatalf("api changed:\n%+v\n%+v", got, want)
	}
	if got.Environment["APP_ENV"] != "prod" {
		t.Fatalf("env: %+v", got.Environment)
	}
}

// TestTerraformRoundTrip: .isor → main.tf → Infra.
func TestTerraformRoundTrip(t *testing.T) {
	src := `version: "1.0"
project: { name: webapp }
services:
  api:
    image: "nginx:1.25"
    ports: ["80:8080"]
    environment: { APP_ENV: prod }
    depends_on: [db]
    volumes: ["./conf:/etc/nginx"]
    networks: [front]
    restart: unless-stopped
  db:
    image: "postgres:16"
networks: { front: {} }
volumes: { data: {} }
`
	infra, infra2, _ := roundTrip(t, src, "terraform", terraformImporter{})
	if !reflect.DeepEqual(infra2.ServiceNames, infra.ServiceNames) {
		t.Fatalf("names: %v vs %v", infra2.ServiceNames, infra.ServiceNames)
	}
	got, want := infra2.Services["api"], infra.Services["api"]
	if got.Image != want.Image || !reflect.DeepEqual(got.Ports, want.Ports) ||
		!reflect.DeepEqual(got.DependsOn, want.DependsOn) ||
		!reflect.DeepEqual(got.Volumes, want.Volumes) ||
		got.Restart != want.Restart || got.Environment["APP_ENV"] != "prod" {
		t.Fatalf("api changed:\n%+v\n%+v", got, want)
	}
	if _, ok := infra2.Volumes["data"]; !ok {
		t.Fatal("docker_volume data lost")
	}
	// networks land project-prefixed (webapp_front) — documented emitter
	// behaviour, recovered as emitted.
	if _, ok := infra2.Networks["front"]; !ok {
		t.Fatal("docker_network front lost")
	}
}
