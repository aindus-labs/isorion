package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/isorion-lang/isorion/pkg/emit"
	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
)

// strList collects repeatable string flags (--set a=1 --set b=2).
type strList []string

func (s *strList) String() string { return strings.Join(*s, ",") }
func (s *strList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// boolFlags take no value: the next arg must not be swallowed as one.
var boolFlags = map[string]bool{"stdout": true}

// splitArgs reorders args so flags work before and after the positional
// file argument (flag.Parse stops at the first positional).
func splitArgs(args []string) (flagArgs, posArgs []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flagArgs = append(flagArgs, a)
			name := strings.TrimLeft(a, "-")
			if i := strings.IndexByte(name, '='); i >= 0 {
				name = name[:i]
			}
			if !strings.Contains(a, "=") && !boolFlags[name] &&
				i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
		} else {
			posArgs = append(posArgs, a)
		}
	}
	return flagArgs, posArgs
}

// parseSets turns `--set name=value` literals into an Options.Set map.
func parseSets(raw []string) (map[string]string, error) {
	m := map[string]string{}
	for _, s := range raw {
		i := strings.IndexByte(s, '=')
		if i <= 0 {
			return nil, fmt.Errorf("--set expects name=value, got %q", s)
		}
		m[strings.TrimPrefix(s[:i], "$")] = s[i+1:]
	}
	return m, nil
}

// posify prefixes a model-build error with the source position of the
// offending top-level section when known: `file:line:col: model: …`.
func posify(doc *lang.Document, err error) error {
	section := err.Error()
	if i := strings.IndexAny(section, ".:"); i >= 0 {
		section = section[:i]
	}
	if pos, ok := doc.Positions[section]; ok {
		return fmt.Errorf("%s:%d:%d: model: %w", doc.Source, pos.Line, pos.Col, err)
	}
	return fmt.Errorf("model: %w", err)
}

// printWarnings sends capability + advisory warnings to stderr (stdout
// stays clean for --stdout piping).
func printWarnings(warnings []string) {
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "isorion: warning: %s\n", w)
	}
}

// emitToStdout prints one target's files as a --- separated stream
// (kubectl/conftest-compatible).
func emitToStdout(target string, doc *lang.Document, infra *model.Infra) error {
	files, warnings, err := emit.Run(target, doc, infra)
	if err != nil {
		return fmt.Errorf("target %s: %w", target, err)
	}
	printWarnings(warnings)
	for _, f := range files {
		fmt.Printf("# source: %s/%s\n---\n", target, f.Path)
		if _, err := os.Stdout.Write(f.Content); err != nil {
			return err
		}
		if n := len(f.Content); n > 0 && f.Content[n-1] != '\n' {
			fmt.Println()
		}
	}
	return nil
}

func cmdFusion(args []string) error {
	fs := flag.NewFlagSet("fusion", flag.ContinueOnError)
	targetsFlag := fs.String("t", "", "comma-separated targets (default: all)")
	fs.StringVar(targetsFlag, "target", "", "comma-separated targets")
	outDir := fs.String("o", "output", "output directory")
	fs.StringVar(outDir, "output", "output", "output directory")
	var sets strList
	fs.Var(&sets, "set", "override a variable: --set name=value (repeatable)")
	profile := fs.String("profile", "", "merge a profiles: entry into variables:")
	toStdout := fs.Bool("stdout", false, "print emitted files to stdout (single target required)")

	flagArgs, posArgs := splitArgs(args)
	if err := fs.Parse(append(flagArgs, posArgs...)); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("fusion requires a .isor file\n\nusage: isorion fusion <file.isor> [-t targets] [-o dir|--stdout] [--set k=v] [--profile name]")
	}
	srcPath := fs.Arg(0)

	setMap, err := parseSets(sets)
	if err != nil {
		return err
	}
	doc, err := loadDoc(srcPath, lang.Options{Profile: *profile, Set: setMap})
	if err != nil {
		return err
	}
	infra, err := model.FromBody(doc.Version, doc.Body)
	if err != nil {
		return posify(doc, err)
	}
	cwarns, err := model.CheckConstraints(doc.Body, doc.Constraints)
	printWarnings(cwarns)
	if err != nil {
		return fmt.Errorf("%s: %w", doc.Source, err)
	}

	targets := emit.Names()
	if *targetsFlag != "" {
		targets = strings.Split(*targetsFlag, ",")
	}

	if *toStdout {
		if len(targets) != 1 || strings.TrimSpace(targets[0]) == "" {
			return fmt.Errorf("--stdout requires exactly one target: -t <name>")
		}
		return emitToStdout(strings.TrimSpace(targets[0]), doc, infra)
	}

	written := 0
	for _, t := range targets {
		t = strings.TrimSpace(t)
		files, warnings, err := emit.Run(t, doc, infra)
		if err != nil {
			return fmt.Errorf("target %s: %w", t, err)
		}
		printWarnings(warnings)
		for _, f := range files {
			dest := filepath.Join(*outDir, t, f.Path)
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(dest, f.Content, 0o644); err != nil {
				return err
			}
			fmt.Printf("  %-16s → %s\n", t, dest)
			written++
		}
	}
	fmt.Printf("fusion: %d file(s) written to %s\n", written, *outDir)
	return nil
}

func cmdValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	var sets strList
	fs.Var(&sets, "set", "override a variable: --set name=value (repeatable)")
	profile := fs.String("profile", "", "merge a profiles: entry into variables:")

	flagArgs, posArgs := splitArgs(args)
	if err := fs.Parse(append(flagArgs, posArgs...)); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("validate requires a .isor file")
	}
	srcPath := fs.Arg(0)

	setMap, err := parseSets(sets)
	if err != nil {
		return err
	}
	doc, err := loadDoc(srcPath, lang.Options{Profile: *profile, Set: setMap})
	if err != nil {
		return err
	}
	if _, err := model.FromBody(doc.Version, doc.Body); err != nil {
		return posify(doc, err)
	}
	cwarns, err := model.CheckConstraints(doc.Body, doc.Constraints)
	printWarnings(cwarns)
	if err != nil {
		return fmt.Errorf("%s: %w", doc.Source, err)
	}
	fmt.Printf("validate: %s OK (version=%q, %d variable(s), %d top-level section(s))\n",
		srcPath, doc.Version, len(doc.Variables), len(doc.Body))
	return nil
}

func cmdTargets(_ []string) error {
	fmt.Println("Available targets (community edition):")
	for _, name := range emit.Names() {
		e, _ := emit.Get(name)
		fmt.Printf("  %-16s %s\n", name, e.Description())
	}
	return nil
}

func loadDoc(path string, opts lang.Options) (*lang.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return lang.LoadWith(path, data, opts)
}
