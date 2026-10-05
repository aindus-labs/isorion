package emit

import (
	"fmt"
	"strings"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
)

// nomadEmitter emits a Nomad job (HCL): one group per service, docker task
// driver. depends_on → no direct equivalent (warned); secrets → env
// placeholders (Nomad templates/variables inject them at run time).
type nomadEmitter struct{}

func init() { Register(nomadEmitter{}) }

func (nomadEmitter) Name() string { return "nomad" }
func (nomadEmitter) Description() string {
	return "Nomad job (job.nomad HCL — docker driver, one group per service)"
}

func (nomadEmitter) Emit(doc *lang.Document, infra *model.Infra) ([]File, error) {
	if len(infra.Services) == 0 {
		return nil, fmt.Errorf("nomad target requires at least one service")
	}
	job := infra.ProjectName
	if job == "" {
		job = "isorion"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "job %q {\n  datacenters = [\"dc1\"]\n  type        = \"service\"\n", job)

	for _, name := range infra.ServiceNames {
		s := infra.Services[name]
		fmt.Fprintf(&b, "\n  group %q {\n    count = %d\n", name, s.Replicas)

		if len(s.Networks) > 0 || len(s.Ports) > 0 {
			b.WriteString("\n    network {\n")
			for i, p := range s.Ports {
				port := fmt.Sprintf("p%d", i)
				fmt.Fprintf(&b, "      port %q {\n", port)
				if p.Host != 0 {
					fmt.Fprintf(&b, "        static = %d\n", p.Host)
				}
				fmt.Fprintf(&b, "        to     = %d\n      }\n", p.Container)
			}
			b.WriteString("    }\n")
		}

		fmt.Fprintf(&b, "\n    task %q {\n      driver = \"docker\"\n\n      config {\n", name)
		if s.Image != "" {
			fmt.Fprintf(&b, "        image = %q\n", s.Image)
		}
		if len(s.Ports) > 0 {
			labels := make([]string, len(s.Ports))
			for i := range s.Ports {
				labels[i] = fmt.Sprintf("p%d", i)
			}
			fmt.Fprintf(&b, "        ports = %s\n", hclList(labels))
		}
		if len(s.Command) > 0 {
			fmt.Fprintf(&b, "        command = %q\n", s.Command[0])
			if len(s.Command) > 1 {
				fmt.Fprintf(&b, "        args    = %s\n", hclList(s.Command[1:]))
			}
		}
		if len(s.Volumes) > 0 {
			fmt.Fprintf(&b, "        volumes = %s\n", hclList(s.Volumes))
		}
		b.WriteString("      }\n")

		if len(s.Environment) > 0 {
			b.WriteString("\n      env {\n")
			plain, secret := splitSecretEnv(s.Environment)
			for _, k := range sortedEnvKeys(plain) {
				fmt.Fprintf(&b, "        %s = %q\n", envKey(k), plain[k])
			}
			for _, k := range sortedEnvKeys(secret) {
				fmt.Fprintf(&b, "        %s = \"${SECRET_%s}\" # injected via Nomad variable/template\n", envKey(k), secret[k])
			}
			b.WriteString("      }\n")
		}
		b.WriteString("    }\n  }\n")
	}
	b.WriteString("}\n")
	return []File{{Path: "job.nomad", Content: []byte(b.String())}}, nil
}

// Warn reports model fields silently dropped by the nomad target.
func (nomadEmitter) Warn(_ *lang.Document, infra *model.Infra) []string {
	var out []string
	for _, name := range infra.ServiceNames {
		s := infra.Services[name]
		for _, k := range sortedAnyKeys(s.Extra) {
			out = append(out, fmt.Sprintf("services.%s.%s ignored by target nomad", name, k))
		}
		if s.Build != "" {
			out = append(out, fmt.Sprintf("services.%s.build ignored by target nomad (runs images, does not build)", name))
		}
		if len(s.DependsOn) > 0 {
			out = append(out, fmt.Sprintf("services.%s.depends_on ignored by target nomad (no ordering primitive)", name))
		}
		if s.Restart != "" {
			out = append(out, fmt.Sprintf("services.%s.restart ignored by target nomad (use a restart {} block manually)", name))
		}
		if len(s.Labels) > 0 {
			out = append(out, fmt.Sprintf("services.%s.labels ignored by target nomad", name))
		}
		if len(s.Networks) > 0 {
			out = append(out, fmt.Sprintf("services.%s.networks ignored by target nomad (Nomad networking differs)", name))
		}
	}
	return out
}
