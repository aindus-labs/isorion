package emit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
)

// terraformEmitter emits HCL for the kreuzwerker/docker provider: one
// docker_container per service, docker_network/docker_volume for the
// top-level sections, and sensitive variables for secrets.
type terraformEmitter struct{}

func init() { Register(terraformEmitter{}) }

func (terraformEmitter) Name() string { return "terraform" }
func (terraformEmitter) Description() string {
	return "Terraform HCL (docker provider — containers, networks, volumes)"
}

func (terraformEmitter) Emit(doc *lang.Document, infra *model.Infra) ([]File, error) {
	if len(infra.Services) == 0 {
		return nil, fmt.Errorf("terraform target requires at least one service")
	}
	var main strings.Builder
	main.WriteString("terraform {\n  required_providers {\n    docker = {\n      source  = \"kreuzwerker/docker\"\n      version = \"~> 3.0\"\n    }\n  }\n}\n\nprovider \"docker\" {}\n")

	// Secret refs → sensitive variables (plaintext never lands in main.tf).
	var secrets map[string]string
	for _, name := range infra.ServiceNames {
		_, sec := splitSecretEnv(infra.Services[name].Environment)
		if len(sec) > 0 {
			if secrets == nil {
				secrets = map[string]string{}
			}
			for _, sn := range sec {
				secrets[sn] = ""
			}
		}
	}
	for _, n := range sortedSecretNames(doc.Secrets) {
		if secrets == nil {
			secrets = map[string]string{}
		}
		secrets[n] = ""
	}
	var vars strings.Builder
	for _, sn := range sortedSecretNames(secrets) {
		fmt.Fprintf(&vars, "variable %q {\n  type      = string\n  sensitive = true\n}\n\n", "secret_"+sn)
		fmt.Fprintf(&main, "\n# secret %q is injected via var.secret_%s\n", sn, sn)
	}

	for _, n := range sortedAnyKeys(infra.Networks) {
		fmt.Fprintf(&main, "\nresource \"docker_network\" %q {\n  name = %q\n}\n", n, tfResName(infra, n))
	}
	for _, n := range sortedAnyKeys(infra.Volumes) {
		fmt.Fprintf(&main, "\nresource \"docker_volume\" %q {\n  name = %q\n}\n", n, n)
	}

	for _, name := range infra.ServiceNames {
		s := infra.Services[name]
		fmt.Fprintf(&main, "\nresource \"docker_container\" %q {\n", name)
		fmt.Fprintf(&main, "  name  = %q\n", name)
		if s.Image != "" {
			fmt.Fprintf(&main, "  image = %q\n", s.Image)
		}
		if len(s.Command) > 0 {
			fmt.Fprintf(&main, "  command = %s\n", hclList(s.Command))
		}
		if len(s.Ports) > 0 {
			for _, p := range s.Ports {
				main.WriteString("  ports {\n")
				fmt.Fprintf(&main, "    internal = %d\n", p.Container)
				if p.Host != 0 {
					fmt.Fprintf(&main, "    external = %d\n", p.Host)
				}
				if p.Protocol != "" && p.Protocol != "tcp" {
					fmt.Fprintf(&main, "    protocol = %q\n", p.Protocol)
				}
				main.WriteString("  }\n")
			}
		}
		plain, secret := splitSecretEnv(s.Environment)
		if len(plain)+len(secret) > 0 {
			var envs []string
			for _, k := range sortedEnvKeys(plain) {
				envs = append(envs, k+"="+plain[k])
			}
			for _, k := range sortedEnvKeys(secret) {
				envs = append(envs, fmt.Sprintf("%s=${var.secret_%s}", k, secret[k]))
			}
			fmt.Fprintf(&main, "  env = %s\n", hclList(envs))
		}
		if len(s.Volumes) > 0 {
			for _, v := range s.Volumes {
				src, dst, _ := strings.Cut(v, ":")
				main.WriteString("  volumes {\n")
				fmt.Fprintf(&main, "    volume_name    = %q\n    container_path = %q\n  }\n", src, dst)
			}
		}
		if len(s.Networks) > 0 {
			fmt.Fprintf(&main, "  networks_advanced {\n    name = %q\n  }\n", s.Networks[0])
		}
		if s.Restart != "" {
			fmt.Fprintf(&main, "  restart = %q\n", s.Restart)
		}
		if len(s.Labels) > 0 {
			for _, k := range sortedAnyKeys(s.Labels) {
				main.WriteString("  labels {\n")
				fmt.Fprintf(&main, "    label = %q\n    value = %q\n  }\n", k, fmt.Sprint(s.Labels[k]))
			}
		}
		if len(s.DependsOn) > 0 {
			refs := make([]string, len(s.DependsOn))
			for i, d := range s.DependsOn {
				refs[i] = "docker_container." + d
			}
			fmt.Fprintf(&main, "  depends_on = %s\n", hclRefs(refs))
		}
		main.WriteString("}\n")
	}

	files := []File{{Path: "main.tf", Content: []byte(main.String())}}
	if vars.Len() > 0 {
		files = append(files, File{Path: "variables.tf", Content: []byte(vars.String())})
	}
	return files, nil
}

// Warn reports model fields silently dropped by the docker provider.
func (terraformEmitter) Warn(_ *lang.Document, infra *model.Infra) []string {
	var out []string
	for _, name := range infra.ServiceNames {
		s := infra.Services[name]
		for _, k := range sortedAnyKeys(s.Extra) {
			out = append(out, fmt.Sprintf("services.%s.%s ignored by target terraform", name, k))
		}
		if s.Build != "" {
			out = append(out, fmt.Sprintf("services.%s.build ignored by target terraform (docker provider runs images, it does not build)", name))
		}
		if s.Replicas != 1 {
			out = append(out, fmt.Sprintf("services.%s.replicas=%d ignored by target terraform (docker_container is a single instance — use terraform count/module manually)", name, s.Replicas))
		}
		if len(s.Networks) > 1 {
			out = append(out, fmt.Sprintf("services.%s.networks: only the first network is emitted by target terraform", name))
		}
	}
	return out
}

// hclList renders a quoted string list: ["a", "b"].
func hclList(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// hclRefs renders a bare resource-ref list: [docker_container.a, ...].
func hclRefs(refs []string) string {
	return "[" + strings.Join(refs, ", ") + "]"
}

// tfResName picks the terraform-side name for a network (project-prefixed
// when a project name exists, matching compose conventions).
func tfResName(infra *model.Infra, n string) string {
	if infra.ProjectName != "" {
		return infra.ProjectName + "_" + n
	}
	return n
}

var _ = sort.Strings // keep import if helpers change
