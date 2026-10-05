package model

import (
	"strings"
	"testing"
)

func TestParsePort(t *testing.T) {
	cases := []struct {
		in              string
		host, container int64
		proto, hostIP   string
		wantErr         bool
	}{
		{"80", 0, 80, "tcp", "", false},
		{"8080:80", 8080, 80, "tcp", "", false},
		{"53:53/udp", 53, 53, "udp", "", false},
		{"127.0.0.1:80:8080", 80, 8080, "tcp", "127.0.0.1", false},
		{"9000/sctp", 0, 9000, "sctp", "", false},
		{"abc", 0, 0, "", "", true},
		{"x:y", 0, 0, "", "", true},
		{"1:2:3:4", 0, 0, "", "", true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			p, err := ParsePort(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", c.in)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if p.Host != c.host || p.Container != c.container ||
				p.Protocol != c.proto || p.HostIP != c.hostIP {
				t.Fatalf("got %+v", p)
			}
		})
	}
}

func TestPortMapCompose(t *testing.T) {
	cases := map[string]string{
		"80":                "80",
		"8080:80":           "8080:80",
		"53:53/udp":         "53:53/udp",
		"127.0.0.1:80:8080": "127.0.0.1:80:8080",
	}
	for in, want := range cases {
		p, err := ParsePort(in)
		if err != nil {
			t.Fatal(err)
		}
		if got := p.Compose(); got != want {
			t.Fatalf("Compose(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFromBodyProjectAndServices(t *testing.T) {
	infra, err := FromBody("1.0", map[string]any{
		"project": map[string]any{"name": "app", "description": "d", "labels": map[string]any{"tier": "x"}},
		"services": map[string]any{
			"api": map[string]any{
				"image":      "nginx",
				"replicas":   float64(3),
				"command":    "nginx -g",
				"ports":      []any{"8080:80"},
				"env":        map[string]any{"A": "1"},
				"depends_on": []any{"db"},
				"custom_x":   true,
			},
			"db": map[string]any{"build": "./db"},
		},
		"constraints": []any{}, // reserved: must not leak into Other
		"secrets":     map[string]any{},
		"extra_top":   "keep",
	})
	if err != nil {
		t.Fatal(err)
	}
	if infra.ProjectName != "app" || infra.Description != "d" || infra.Labels["tier"] != "x" {
		t.Fatalf("bad project: %+v", infra)
	}
	if got := infra.ServiceNames; len(got) != 2 || got[0] != "api" || got[1] != "db" {
		t.Fatalf("ServiceNames not sorted: %v", got)
	}
	api := infra.Services["api"]
	if api.Replicas != 3 || api.Image != "nginx" {
		t.Fatalf("bad api: %+v", api)
	}
	if len(api.Ports) != 1 || api.Ports[0].Host != 8080 {
		t.Fatalf("bad ports: %+v", api.Ports)
	}
	if api.Environment["A"] != "1" || api.DependsOn[0] != "db" {
		t.Fatalf("bad env/deps: %+v", api)
	}
	if api.Extra["custom_x"] != true {
		t.Fatalf("Extra passthrough lost: %+v", api.Extra)
	}
	if _, ok := infra.Other["constraints"]; ok {
		t.Fatal("constraints leaked into Other")
	}
	if infra.Other["extra_top"] != "keep" {
		t.Fatal("unknown top-level section not preserved")
	}
}

func TestFromBodyErrors(t *testing.T) {
	cases := map[string]map[string]any{
		"project not mapping":     {"project": "x"},
		"services not mapping":    {"services": "x"},
		"service not mapping":     {"services": map[string]any{"a": 42}},
		"missing image and build": {"services": map[string]any{"a": map[string]any{"replicas": 1}}},
		"bad replicas":            {"services": map[string]any{"a": map[string]any{"image": "i", "replicas": "x"}}},
		"ports not list":          {"services": map[string]any{"a": map[string]any{"image": "i", "ports": "80"}}},
		"bad port value":          {"services": map[string]any{"a": map[string]any{"image": "i", "ports": []any{"x:y"}}}},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := FromBody("1.0", body); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestFromBodyNetworksVolumes(t *testing.T) {
	infra, err := FromBody("", map[string]any{
		"networks": map[string]any{"front": map[string]any{}},
		"volumes":  map[string]any{"data": map[string]any{"driver": "local"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := infra.Networks["front"]; !ok {
		t.Fatal("networks lost")
	}
	if v, ok := infra.Volumes["data"].(map[string]any); !ok || v["driver"] != "local" {
		t.Fatalf("volumes lost: %+v", infra.Volumes)
	}
}

func TestHelpers(t *testing.T) {
	if got := strList("cmd --flag"); len(got) != 1 || got[0] != "cmd --flag" {
		t.Fatalf("strList(string) = %v", got)
	}
	if got := strList(42); got != nil {
		t.Fatalf("strList(int) = %v, want nil", got)
	}
	if m := strMap(map[string]any{"a": 1, "b": true}); m["a"] != "1" || m["b"] != "true" {
		t.Fatalf("strMap = %v", m)
	}
	if n, err := toInt64("7"); err != nil || n != 7 {
		t.Fatalf("toInt64(string) = %d, %v", n, err)
	}
	if _, err := toInt64([]any{}); err == nil || !strings.Contains(err.Error(), "not a number") {
		t.Fatalf("toInt64(list) should fail, got %v", err)
	}
}
