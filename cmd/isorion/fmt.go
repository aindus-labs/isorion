package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/isorion-lang/isorion/pkg/lang"
	"gopkg.in/yaml.v3"
)

// preferredSections come first in canonical form; other top-level keys
// keep their document order afterwards.
var preferredSections = []string{"version", "variables", "profiles", "secrets", "constraints"}

// unquoteRe restores bare directive keys after YAML encoding; directive
// args may contain escaped quotes (e.g. "@if $env == \"prod\"").
var unquoteRe = regexp.MustCompile(`(?m)^(\s*(?:-\s+)?)"(@(?:if|elif|else|for|include|extends)(?:[^"\\]|\\.)*)":`)

// unquoteKeys replaces `"@dir <arg>":` with the bare `@dir <arg>:` form.
func unquoteKeys(s string) string {
	return unquoteRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := unquoteRe.FindStringSubmatch(m)
		key := strings.ReplaceAll(sub[2], `\"`, `"`)
		key = strings.ReplaceAll(key, `\\`, `\`)
		return sub[1] + key + ":"
	})
}

// cmdFmt prints (or rewrites with -w) the canonical form of a .isor file:
// normalized indentation (2), preferred top-level section order, bare
// directive keys. Comments attached to nodes are preserved by yaml.v3.
func cmdFmt(args []string) error {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	write := fs.Bool("w", false, "write result back to the file (default: stdout)")

	flagArgs, posArgs := splitArgs(args)
	if err := fs.Parse(append(flagArgs, posArgs...)); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("fmt requires a .isor file\n\nusage: isorion fmt <file.isor> [-w]")
	}
	path := fs.Arg(0)

	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(lang.Preprocess(string(raw))), &doc); err != nil {
		return fmt.Errorf("%s: YAML parse error: %w", path, err)
	}
	if len(doc.Content) > 0 && doc.Content[0].Kind == yaml.MappingNode {
		reorderTopLevel(doc.Content[0])
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	enc.Close()

	out := unquoteKeys(buf.String())

	if *write {
		if string(raw) == out {
			fmt.Printf("fmt: %s unchanged\n", path)
			return nil
		}
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			return err
		}
		fmt.Printf("fmt: %s rewritten\n", path)
		return nil
	}
	fmt.Print(out)
	return nil
}

// reorderTopLevel moves preferred sections to the front, preserving their
// relative order; every other key follows in document order.
func reorderTopLevel(top *yaml.Node) {
	rank := func(k string) int {
		for i, p := range preferredSections {
			if k == p {
				return i
			}
		}
		return len(preferredSections) + 1
	}
	type pair struct {
		k, v *yaml.Node
		r    int
	}
	var pairs []pair
	for i := 0; i+1 < len(top.Content); i += 2 {
		k := top.Content[i]
		pairs = append(pairs, pair{k: k, v: top.Content[i+1], r: rank(k.Value)})
	}
	// stable sort by rank only
	for i := 1; i < len(pairs); i++ {
		for j := i; j > 0 && pairs[j].r < pairs[j-1].r; j-- {
			pairs[j], pairs[j-1] = pairs[j-1], pairs[j]
		}
	}
	pos := 0
	for _, p := range pairs {
		top.Content[pos] = p.k
		top.Content[pos+1] = p.v
		pos += 2
	}
}
