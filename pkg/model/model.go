// Package model defines the normalized infrastructure model that all
// emitters translate from. A .isor document expands to a Body map; model.Infra
// is the typed view over the conventional sections:
//
//	project:   { name, description?, labels? }
//	services:  { <name>: { image|build, command?, replicas?, ports?, environment?,
//	                     depends_on?, volumes?, networks?, restart?, labels?, ... } }
//	networks:  { <name>: {...} }
//	volumes:   { <name>: {...} }
//
// Unknown fields are preserved verbatim in Extra so emitters can pass them
// through where the target supports them.
package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// PortMap is a host→container port binding (or plain published port).
type PortMap struct {
	Host      int64 // 0 = not specified
	Container int64
	Protocol  string // "tcp" (default) | "udp" | "sctp"
	HostIP    string // optional "127.0.0.1:80:8080" form
}

// Service is a normalized unit of deployment.
type Service struct {
	Name        string
	Image       string
	Build       string
	Command     []string
	Replicas    int64
	Ports       []PortMap
	Environment map[string]string
	DependsOn   []string
	Volumes     []string
	Networks    []string
	Restart     string
	Labels      map[string]any
	Extra       map[string]any
}

// Infra is the normalized view of an expanded .isor document.
type Infra struct {
	Version      string
	ProjectName  string
	Description  string
	Labels       map[string]any
	Services     map[string]*Service
	ServiceNames []string // deterministic order
	Networks     map[string]any
	Volumes      map[string]any
	Other        map[string]any // top-level sections not part of the model
}

// FromBody builds an Infra from an expanded document body.
func FromBody(version string, body map[string]any) (*Infra, error) {
	infra := &Infra{
		Version:  version,
		Services: map[string]*Service{},
		Networks: map[string]any{},
		Volumes:  map[string]any{},
		Other:    map[string]any{},
	}

	for k, v := range body {
		switch k {
		case "project":
			m, ok := v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("project: must be a mapping")
			}
			infra.ProjectName = strOf(m["name"])
			infra.Description = strOf(m["description"])
			infra.Labels = mapOf(m["labels"])
		case "services":
			m, ok := v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("services: must be a mapping")
			}
			for name, raw := range m {
				svc, err := parseService(name, raw)
				if err != nil {
					return nil, fmt.Errorf("services.%s: %w", name, err)
				}
				infra.Services[name] = svc
				infra.ServiceNames = append(infra.ServiceNames, name)
			}
			sort.Strings(infra.ServiceNames)
		case "networks":
			if m, ok := v.(map[string]any); ok {
				infra.Networks = m
			}
		case "volumes":
			if m, ok := v.(map[string]any); ok {
				infra.Volumes = m
			}
		case "constraints", "secrets":
			// reserved sections, consumed via doc.Constraints/doc.Secrets
		default:
			infra.Other[k] = v
		}
	}
	return infra, nil
}

func parseService(name string, raw any) (*Service, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("service definition must be a mapping")
	}
	s := &Service{Name: name, Replicas: 1, Extra: map[string]any{}}
	for k, v := range m {
		switch k {
		case "image":
			s.Image = strOf(v)
		case "build":
			s.Build = strOf(v)
		case "command":
			s.Command = strList(v)
		case "replicas":
			n, err := toInt64(v)
			if err != nil {
				return nil, fmt.Errorf("replicas: %w", err)
			}
			s.Replicas = n
		case "ports":
			list, ok := v.([]any)
			if !ok {
				return nil, fmt.Errorf("ports: must be a list")
			}
			for _, p := range list {
				pm, err := ParsePort(fmt.Sprint(p))
				if err != nil {
					return nil, fmt.Errorf("ports: %w", err)
				}
				s.Ports = append(s.Ports, pm)
			}
		case "environment", "env":
			s.Environment = strMap(v)
		case "depends_on":
			s.DependsOn = strList(v)
		case "volumes":
			s.Volumes = strList(v)
		case "networks":
			s.Networks = strList(v)
		case "restart":
			s.Restart = strOf(v)
		case "labels":
			s.Labels = mapOf(v)
		default:
			s.Extra[k] = v
		}
	}
	if s.Image == "" && s.Build == "" {
		return nil, fmt.Errorf("either image or build is required")
	}
	return s, nil
}

// ParsePort parses "80", "80:8080", "8080/udp", "127.0.0.1:80:8080".
func ParsePort(s string) (PortMap, error) {
	p := PortMap{Protocol: "tcp"}
	if idx := strings.Index(s, "/"); idx >= 0 {
		p.Protocol = s[idx+1:]
		s = s[:idx]
	}
	parts := strings.Split(s, ":")
	var nums []string
	for i, part := range parts {
		if i == 0 && len(parts) == 3 {
			p.HostIP = part // ip:host:container
			continue
		}
		nums = append(nums, part)
	}
	switch len(nums) {
	case 1:
		c, err := strconv.ParseInt(nums[0], 10, 64)
		if err != nil {
			return p, fmt.Errorf("invalid port %q", s)
		}
		p.Container = c
	case 2:
		h, err := strconv.ParseInt(nums[0], 10, 64)
		if err != nil {
			return p, fmt.Errorf("invalid host port %q", nums[0])
		}
		c, err := strconv.ParseInt(nums[1], 10, 64)
		if err != nil {
			return p, fmt.Errorf("invalid container port %q", nums[1])
		}
		p.Host, p.Container = h, c
	default:
		return p, fmt.Errorf("invalid port mapping %q", s)
	}
	return p, nil
}

// Compose renders the port in docker-compose syntax.
func (p PortMap) Compose() string {
	var b strings.Builder
	if p.HostIP != "" {
		b.WriteString(p.HostIP + ":")
	}
	if p.Host != 0 {
		fmt.Fprintf(&b, "%d:", p.Host)
	}
	fmt.Fprintf(&b, "%d", p.Container)
	if p.Protocol != "" && p.Protocol != "tcp" {
		fmt.Fprintf(&b, "/%s", p.Protocol)
	}
	return b.String()
}

func strOf(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func strList(v any) []string {
	var out []string
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			out = append(out, fmt.Sprint(e))
		}
	case string:
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

func strMap(v any) map[string]string {
	out := map[string]string{}
	if m, ok := v.(map[string]any); ok {
		for k, e := range m {
			out[k] = fmt.Sprint(e)
		}
	}
	return out
}

func mapOf(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func toInt64(v any) (int64, error) {
	switch x := v.(type) {
	case int64:
		return x, nil
	case int:
		return int64(x), nil
	case float64:
		return int64(x), nil
	case string:
		return strconv.ParseInt(x, 10, 64)
	}
	return 0, fmt.Errorf("not a number: %v", v)
}
