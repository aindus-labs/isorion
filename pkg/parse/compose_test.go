package parse

import (
	"reflect"
	"strings"
	"testing"

	"github.com/isorion-lang/isorion/pkg/emit"
	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
)

const composeFixture = `name: webapp
services:
  api:
    image: "nginx:1.25"
    deploy: { replicas: 5 }
    command: ["nginx", "-g", "daemon off;"]
    ports:
      - "80:8080"
      - target: 443
        published: 443
        protocol: tcp
      - "127.0.0.1:9090:9090"
    environment:
      - APP_ENV=production
      - DEBUG
    depends_on:
      db: { condition: service_started }
    volumes:
      - "./conf:/etc/nginx"
    networks:
      - front
    restart: unless-stopped
    labels:
      - "traefik.enable=true"
    healthcheck: { test: "curl -f localhost" }
  db:
    build:
      context: ./db
      dockerfile: Dockerfile.dev
networks:
  front: {}
volumes:
  data: { driver: local }
`

func TestComposeImport(t *testing.T) {
	infra, err := composeImporter{}.Parse([]byte(composeFixture), "docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	if infra.ProjectName != "webapp" {
		t.Fatalf("project name = %q", infra.ProjectName)
	}
	if !reflect.DeepEqual(infra.ServiceNames, []string{"api", "db"}) {
		t.Fatalf("names = %v", infra.ServiceNames)
	}
	api := infra.Services["api"]
	if api.Image != "nginx:1.25" || api.Replicas != 5 {
		t.Fatalf("api = %+v", api)
	}
	if len(api.Ports) != 3 || api.Ports[0].Host != 80 || api.Ports[0].Container != 8080 {
		t.Fatalf("ports = %+v", api.Ports)
	}
	if api.Ports[1].Host != 443 || api.Ports[1].Container != 443 || api.Ports[1].Protocol != "tcp" {
		t.Fatalf("long port form = %+v", api.Ports[1])
	}
	if api.Ports[2].HostIP != "127.0.0.1" {
		t.Fatalf("host_ip lost: %+v", api.Ports[2])
	}
	if api.Environment["APP_ENV"] != "production" {
		t.Fatalf("env list form = %v", api.Environment)
	}
	if _, ok := api.Environment["DEBUG"]; !ok {
		t.Fatalf("keyless env var lost: %v", api.Environment)
	}
	if !reflect.DeepEqual(api.DependsOn, []string{"db"}) {
		t.Fatalf("depends_on map form = %v", api.DependsOn)
	}
	if api.Restart != "unless-stopped" || api.Labels["traefik.enable"] != "true" {
		t.Fatalf("restart/labels = %+v", api)
	}
	if api.Extra["healthcheck"] == nil {
		t.Fatal("healthcheck not preserved in Extra")
	}
	if infra.Services["db"].Build != "./db" {
		t.Fatalf("long build context lost: %+v", infra.Services["db"])
	}
	if _, ok := infra.Networks["front"]; !ok || infra.Volumes["data"] == nil {
		t.Fatalf("top-level networks/volumes lost")
	}
}

func TestComposeImportErrors(t *testing.T) {
	cases := map[string]string{
		"bad yaml":            "services: [",
		"services not map":    "services: 42",
		"service not mapping": "services: {a: 42}",
		"no image or build":   "services: {a: {ports: [\"80\"]}}",
		"bad port":            "services: {a: {image: x, ports: [\"x:y\"]}}",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := (composeImporter{}).Parse([]byte(src), "c.yml"); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

// TestComposeRoundTrip: .isor → compose → model — the importer must
// recover what the emitter wrote (fields the model and compose share).
func TestComposeRoundTrip(t *testing.T) {
	src := `version: "1.0"
project: { name: webapp }
services:
  api:
    image: "nginx:1.25"
    replicas: 3
    ports: ["80:8080"]
    environment: { APP_ENV: prod }
    depends_on: [db]
    restart: unless-stopped
  db:
    image: "postgres:16"
networks: { front: {} }
`
	doc, err := lang.LoadWith("x.isor", []byte(src), lang.Options{})
	if err != nil {
		t.Fatal(err)
	}
	infra, err := model.FromBody(doc.Version, doc.Body)
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := emit.Run("docker-compose", doc, infra)
	if err != nil {
		t.Fatal(err)
	}
	infra2, err := composeImporter{}.Parse(files[0].Content, "docker-compose.yml")
	if err != nil {
		t.Fatalf("emitted compose does not re-import: %v\n%s", err, files[0].Content)
	}
	if infra2.ProjectName != infra.ProjectName {
		t.Fatalf("name: %q vs %q", infra2.ProjectName, infra.ProjectName)
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

// Full loop: .isor → compose → .isor → compose must be byte-stable on the
// second pass (the .isor doc is canonical after one normalization).
func TestComposeFullRoundTrip(t *testing.T) {
	src := `version: "1.0"
project: { name: webapp }
services:
  api:
    image: "nginx:1.25"
    ports: ["80:8080"]
`
	doc, _ := lang.LoadWith("x.isor", []byte(src), lang.Options{})
	infra, _ := model.FromBody(doc.Version, doc.Body)
	c1, _, _ := emit.Run("docker-compose", doc, infra)

	i2, err := composeImporter{}.Parse(c1[0].Content, "docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	doc2 := &lang.Document{Version: i2.Version}
	c2, _, err := emit.Run("docker-compose", doc2, i2)
	if err != nil {
		t.Fatal(err)
	}
	if string(c1[0].Content) != string(c2[0].Content) {
		t.Fatalf("compose not stable:\n--- pass1 ---\n%s\n--- pass2 ---\n%s", c1[0].Content, c2[0].Content)
	}
	// sanity: the emitted compose mentions the image
	if !strings.Contains(string(c1[0].Content), "nginx") {
		t.Fatalf("unexpected compose:\n%s", c1[0].Content)
	}
}
