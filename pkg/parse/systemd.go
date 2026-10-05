package parse

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
)

// systemdImporter lifts an isorion-emitted `<prefix>-<svc>.service` unit
// back to a single-service Infra. One unit = one service (replicas are
// not representable — matching the emitter's own limitation).
type systemdImporter struct{}

func (systemdImporter) Name() string        { return "systemd" }
func (systemdImporter) Description() string { return "systemd unit file (<project>-<svc>.service)" }

var (
	unitRe      = regexp.MustCompile(`^([A-Za-z0-9_-]+)-([A-Za-z0-9_-]+)\.service$`)
	svcDescRe   = regexp.MustCompile(`^isorion service ([A-Za-z0-9_-]+)`)
	containerRe = regexp.MustCompile(`--name ([A-Za-z0-9_-]+)`)
	dependsRe   = regexp.MustCompile(`^([A-Za-z0-9_-]+)-([A-Za-z0-9_-]+)\.service$`)
)

func (systemdImporter) Parse(data []byte, filename string) (*model.Infra, error) {
	text := string(data)
	if !strings.Contains(text, "[Unit]") || !strings.Contains(text, "docker run") {
		return nil, fmt.Errorf("%s: not an isorion-style docker .service unit", filename)
	}
	infra := &model.Infra{
		Version:  "1.0",
		Services: map[string]*model.Service{},
		Networks: map[string]any{},
		Volumes:  map[string]any{},
		Other:    map[string]any{},
	}
	s := &model.Service{Replicas: 1, Extra: map[string]any{}}

	var envKeys []string
	var container string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Description="):
			if m := svcDescRe.FindStringSubmatch(strings.TrimPrefix(line, "Description=")); m != nil {
				s.Name = m[1]
			}
		case strings.HasPrefix(line, "After=") || strings.HasPrefix(line, "Requires="):
			for _, tok := range strings.Fields(strings.SplitN(line, "=", 2)[1]) {
				if m := unitRe.FindStringSubmatch(tok); m != nil &&
					tok != "docker.service" && !strings.HasPrefix(tok, "network-online") {
					if m[2] != s.Name {
						s.DependsOn = append(s.DependsOn, m[2])
					}
					if infra.ProjectName == "" {
						infra.ProjectName = m[1]
					}
				}
			}
		case strings.HasPrefix(line, "Restart="):
			s.Restart = strings.TrimPrefix(line, "Restart=")
		case strings.HasPrefix(line, "Environment="):
			kv := strings.TrimPrefix(line, "Environment=")
			if i := strings.IndexByte(kv, '='); i >= 0 {
				if s.Environment == nil {
					s.Environment = map[string]string{}
				}
				s.Environment[kv[:i]] = systemdUnquote(kv[i+1:])
				envKeys = append(envKeys, kv[:i])
			}
		case strings.HasPrefix(line, "ExecStart="):
			parseDockerRun(strings.TrimPrefix(line, "ExecStart="), s, &envKeys, &container)
		}
	}

	if s.Image == "" {
		return nil, fmt.Errorf("%s: no docker run image found", filename)
	}
	if s.Name == "" {
		s.Name = container // fall back to the container name `<prefix>-<svc>`
	}
	// container <prefix>-<svc> + service <svc> → project prefix
	if container != "" && s.Name != "" && infra.ProjectName == "" {
		if suffix := "-" + s.Name; strings.HasSuffix(container, suffix) {
			infra.ProjectName = strings.TrimSuffix(container, suffix)
		}
	}
	// env keys passed via `-e K` without an `Environment=` line are
	// secret references (loaded from the env file at start time).
	if s.Environment == nil {
		s.Environment = map[string]string{}
	}
	for _, k := range envKeys {
		if _, ok := s.Environment[k]; !ok {
			s.Environment[k] = secretEnv(k)
		}
	}
	// container name <prefix>-<svc> → project + service name
	if i := strings.LastIndexByte(s.Name, '-'); i > 0 && infra.ProjectName == "" {
		infra.ProjectName, s.Name = s.Name[:i], s.Name[i+1:]
	}
	if s.Name == "" {
		return nil, fmt.Errorf("%s: could not determine service name", filename)
	}
	infra.Services[s.Name] = s
	infra.ServiceNames = []string{s.Name}
	return infra, nil
}

func init() { Register(systemdImporter{}) }

// parseDockerRun extracts ports/volumes/networks/env/image/command from
// the emitted `/usr/bin/docker run --rm --name X [flags] image [cmd...]`.
func parseDockerRun(cmd string, s *model.Service, envKeys *[]string, container *string) {
	toks := strings.Fields(cmd)
	var positional []string
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		next := func() string {
			if i+1 < len(toks) {
				i++
				return toks[i]
			}
			return ""
		}
		switch {
		case t == "-p" || t == "--publish":
			if pm, err := model.ParsePort(next()); err == nil {
				s.Ports = append(s.Ports, pm)
			}
		case t == "-v" || t == "--volume":
			s.Volumes = append(s.Volumes, next())
		case t == "--network":
			s.Networks = append(s.Networks, next())
		case t == "-e" || t == "--env":
			*envKeys = append(*envKeys, next())
		case t == "--name":
			*container = next()
		case t == "--rm" || t == "-d" || t == "-i" || t == "-t":
		case strings.HasPrefix(t, "-"):
			// unknown flag, skip (value may follow — bounded parser, best effort)
		case t != "/usr/bin/docker" && t != "docker" && t != "run":
			positional = append(positional, t)
		}
	}
	if len(positional) > 0 {
		s.Image = positional[0]
		s.Command = positional[1:]
	}
}

// secretEnv marks a `-e K` env var whose value never lands in the unit —
// the secret name is not recoverable from the unit, K is used verbatim.
func secretEnv(k string) string {
	return lang.SecretMarker + k
}

func systemdUnquote(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return strings.ReplaceAll(v[1:len(v)-1], `\"`, `"`)
	}
	return v
}
