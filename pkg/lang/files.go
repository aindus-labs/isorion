// Multi-file composition (@include / @extends), secrets: section, variable
// constraints (`name!`), and the top-level section scanner shared by the
// root document and included/extended children.
package lang

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Reserved env keys (impossible as user variables — cannot contain \x00).
const (
	envLoaderKey      = "\x00isorion.loader"      // *loader — file resolution context
	envSecretsKey     = "\x00isorion.secrets"     // map[string]string — shared secrets store
	envConstraintsKey = "\x00isorion.constraints" // *[]Constraint — shared accumulator
)

// SecretMarker prefixes secret references produced by secret(). Emitters
// translate the marker into target-native secret references — the encrypted
// value never reaches artifacts.
const SecretMarker = "isorion-secret://"

// SecretName reports whether v is a secret reference marker and returns
// the secret name.
func SecretName(v any) (string, bool) {
	s, ok := v.(string)
	if !ok || !strings.HasPrefix(s, SecretMarker) {
		return "", false
	}
	return s[len(SecretMarker):], true
}

// Constraint is one `constraints:` entry evaluated on the expanded body.
type Constraint struct {
	Expr  string // "path.to.field <op> <expr>" or a bare boolean expression
	Level string // "" | "advisory" | "mandatory" (default: mandatory)
	Pos   Pos
}

// loader carries file-resolution context through env so include/extends
// need no signature changes in the expander.
type loader struct {
	dir     string          // directory of the file currently being expanded
	loading map[string]bool // absolute paths on the current include stack (cycle guard)
}

func envLoader(env map[string]Value) *loader {
	if ld, ok := env[envLoaderKey].(*loader); ok {
		return ld
	}
	return &loader{dir: ".", loading: map[string]bool{}}
}

// envSecrets returns the shared secrets store, creating one if absent.
func envSecrets(env map[string]Value) map[string]string {
	if m, ok := env[envSecretsKey].(map[string]string); ok {
		return m
	}
	m := map[string]string{}
	env[envSecretsKey] = m
	return m
}

// envConstraints returns the shared constraints accumulator.
func envConstraints(env map[string]Value) *[]Constraint {
	if p, ok := env[envConstraintsKey].(*[]Constraint); ok {
		return p
	}
	p := &[]Constraint{}
	env[envConstraintsKey] = p
	return p
}

// fileRef is a resolved @include/@extends target with optional sha256 pin.
type fileRef struct {
	Path   string
	SHA256 string
}

// extendsSpec is a top-level @extends directive.
type extendsSpec struct {
	ref  fileRef
	node *yaml.Node // position for errors
}

// topSections is the result of the first-pass scan over a document's
// top-level mapping.
type topSections struct {
	version     string
	vars        *yaml.Node
	profiles    *yaml.Node
	secrets     *yaml.Node
	constraints *yaml.Node
	ext         *extendsSpec
	bodyPairs   []*yaml.Node
}

// parseDoc preprocesses and YAML-parses a .isor source into its top node.
func parseDoc(filename string, src []byte) (*yaml.Node, error) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(Preprocess(string(src))), &root); err != nil {
		return nil, fmt.Errorf("%s: YAML parse error: %w", filename, err)
	}
	if len(root.Content) == 0 {
		return nil, fmt.Errorf("%s: empty document", filename)
	}
	return root.Content[0], nil
}

// scanTop splits the top-level mapping into reserved sections and body
// pairs. positions (nil-safe) records key positions for diagnostics.
func scanTop(top *yaml.Node, positions map[string]Pos) (*topSections, error) {
	sec := &topSections{}
	pairs := top.Content
	for i := 0; i+1 < len(pairs); i += 2 {
		k, v := pairs[i], pairs[i+1]
		if positions != nil {
			positions[k.Value] = Pos{Line: k.Line, Col: k.Column}
		}
		if kind, arg := ParseDirective(k.Value); kind == DirExtends {
			if sec.ext != nil {
				return nil, withLine(k, fmt.Errorf("only one @extends allowed per document"))
			}
			ref, err := parseFileRef(arg, v)
			if err != nil {
				return nil, withLine(k, err)
			}
			sec.ext = &extendsSpec{ref: ref, node: k}
			continue
		}
		switch k.Value {
		case "version":
			sec.version = scalarString(v)
		case "variables":
			sec.vars = v
		case "profiles":
			sec.profiles = v
		case "secrets":
			sec.secrets = v
		case "constraints":
			sec.constraints = v
		default:
			sec.bodyPairs = append(sec.bodyPairs, k, v)
		}
	}
	return sec, nil
}

// parseFileRef parses "@include path [@sha256 hex]" (key-arg form),
// "@extends: path" (scalar value) or "@extends: {file: p, sha256: h}".
func parseFileRef(arg string, vn *yaml.Node) (fileRef, error) {
	if arg != "" {
		fields := strings.Fields(arg)
		ref := fileRef{Path: fields[0]}
		for i := 1; i < len(fields); i++ {
			f := fields[i]
			switch {
			case f == "@sha256" && i+1 < len(fields):
				i++
				ref.SHA256 = fields[i]
			case strings.HasPrefix(f, "@sha256="):
				ref.SHA256 = strings.TrimPrefix(f, "@sha256=")
			default:
				return ref, fmt.Errorf("unexpected token %q in file directive", f)
			}
		}
		return ref, nil
	}
	if vn == nil {
		return fileRef{}, fmt.Errorf("file directive requires a path")
	}
	switch vn.Kind {
	case yaml.ScalarNode:
		if vn.Value == "" {
			return fileRef{}, fmt.Errorf("file directive requires a path")
		}
		return fileRef{Path: vn.Value}, nil
	case yaml.MappingNode:
		ref := fileRef{}
		for i := 0; i+1 < len(vn.Content); i += 2 {
			k, v := vn.Content[i], vn.Content[i+1]
			switch k.Value {
			case "file":
				ref.Path = v.Value
			case "sha256":
				ref.SHA256 = v.Value
			default:
				return ref, withLine(k, fmt.Errorf("unknown key %q in file directive (expected file/sha256)", k.Value))
			}
		}
		if ref.Path == "" {
			return ref, fmt.Errorf("file directive requires a path")
		}
		return ref, nil
	}
	return fileRef{}, fmt.Errorf("invalid file directive value")
}

// fileDoc is the expanded view of a child file (include/extends).
type fileDoc struct {
	version string
	body    any // map[string]Value, or []any for a sequence-top fragment
}

// fileErr renders an error inside a child file with that file's own name —
// otherwise a child's line:col would be printed next to the parent's name.
func fileErr(abs string, err error) error {
	var le *LineError
	if errors.As(err, &le) {
		return fmt.Errorf("%s:%d:%d: %w", abs, le.Pos.Line, le.Pos.Col, le.Err)
	}
	return fmt.Errorf("%s: %w", abs, err)
}

// expandChildFile loads and expands a referenced file. The caller's env
// provides resolution context (loader dir, shared secrets/constraints) and
// base variables: variables declared inside the child act as *defaults* —
// the caller's environment always wins.
func expandChildFile(ref fileRef, env map[string]Value) (*fileDoc, error) {
	ld := envLoader(env)
	abs, err := filepath.Abs(filepath.Join(ld.dir, ref.Path))
	if err != nil {
		return nil, err
	}
	if ld.loading[abs] {
		return nil, fmt.Errorf("include/extends cycle via %s", abs)
	}
	ld.loading[abs] = true
	defer delete(ld.loading, abs)

	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ref.Path, err)
	}
	if ref.SHA256 != "" {
		sum := sha256.Sum256(raw)
		got := hex.EncodeToString(sum[:])
		if !strings.EqualFold(got, ref.SHA256) {
			return nil, fmt.Errorf("%s: sha256 mismatch (declared %s, got %s)", ref.Path, ref.SHA256, got)
		}
	}

	top, err := parseDoc(abs, raw)
	if err != nil {
		return nil, err
	}

	childEnv := cloneEnv(env)
	childEnv[envLoaderKey] = &loader{dir: filepath.Dir(abs), loading: ld.loading}

	// Sequence/scalar fragment file: expand verbatim (interpolation and
	// directives still apply), no reserved sections.
	if top.Kind != yaml.MappingNode {
		v, err := expandNode(top, childEnv)
		if err != nil {
			return nil, fileErr(abs, err)
		}
		return &fileDoc{body: v}, nil
	}

	sec, err := scanTop(top, nil)
	if err != nil {
		return nil, fileErr(abs, err)
	}
	fd := &fileDoc{version: sec.version}

	// Own variables expanded against the caller env; they fill gaps only.
	if sec.vars != nil {
		vars, vcons, err := loadVariables(sec.vars, childEnv)
		if err != nil {
			return nil, fileErr(abs, err)
		}
		for k, v := range vars {
			if _, ok := childEnv[k]; !ok {
				childEnv[k] = v
			}
		}
		if err := checkVarConstraints(vcons, childEnv); err != nil {
			return nil, fileErr(abs, err)
		}
	}

	// secrets:/constraints: merge into the shared stores (visible to parent).
	if sec.secrets != nil {
		secrets, err := parseSecrets(sec.secrets)
		if err != nil {
			return nil, fileErr(abs, err)
		}
		store := envSecrets(childEnv)
		for k, v := range secrets {
			store[k] = v
		}
	}
	if sec.constraints != nil {
		cs, err := parseConstraints(sec.constraints)
		if err != nil {
			return nil, fileErr(abs, err)
		}
		*envConstraints(childEnv) = append(*envConstraints(childEnv), cs...)
	}

	fd.body, err = expandPairs(sec.bodyPairs, childEnv)
	if err != nil {
		return nil, fileErr(abs, err)
	}

	// @extends inside a child file: deep-merge its parent recursively.
	if sec.ext != nil {
		pfd, err := expandChildFile(sec.ext.ref, childEnv)
		if err != nil {
			return nil, err
		}
		pm, ok := pfd.body.(map[string]Value)
		if !ok {
			return nil, fmt.Errorf("@extends %s: document body must be a mapping", sec.ext.ref.Path)
		}
		fd.body = deepMerge(pm, fd.body)
		if fd.version == "" {
			fd.version = pfd.version
		}
	}
	return fd, nil
}

// deepMerge overlays b over a: maps merge recursively, everything else is
// replaced by b (child refines, never the reverse).
func deepMerge(a, b any) any {
	am, aok := a.(map[string]Value)
	bm, bok := b.(map[string]Value)
	if aok && bok {
		out := make(map[string]Value, len(am)+len(bm))
		for k, v := range am {
			out[k] = v
		}
		for k, v := range bm {
			if av, exists := out[k]; exists {
				out[k] = deepMerge(av, v)
			} else {
				out[k] = v
			}
		}
		return out
	}
	return b
}

// ---------- secrets: ----------

// parseSecrets reads `secrets:` — opaque ENC[...] ciphertext entries.
func parseSecrets(n *yaml.Node) (map[string]string, error) {
	if n.Kind != yaml.MappingNode {
		return nil, withLine(n, fmt.Errorf("secrets must be a mapping"))
	}
	out := map[string]string{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		blob := v.Value
		if !strings.HasPrefix(blob, "ENC[") {
			return nil, withLine(k, fmt.Errorf("secrets.%s must be ENC[...] ciphertext (sops/age) — plaintext is rejected", k.Value))
		}
		out[k.Value] = blob
	}
	return out, nil
}

// ---------- constraints: ----------

// parseConstraints reads `constraints:` entries: plain strings (mandatory)
// or {expr, level: advisory|mandatory} mappings.
func parseConstraints(n *yaml.Node) ([]Constraint, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, withLine(n, fmt.Errorf("constraints must be a list"))
	}
	var out []Constraint
	for _, item := range n.Content {
		pos := Pos{Line: item.Line, Col: item.Column}
		switch item.Kind {
		case yaml.ScalarNode:
			out = append(out, Constraint{Expr: item.Value, Level: "mandatory", Pos: pos})
		case yaml.MappingNode:
			c := Constraint{Level: "mandatory", Pos: pos}
			for i := 0; i+1 < len(item.Content); i += 2 {
				k, v := item.Content[i], item.Content[i+1]
				switch k.Value {
				case "expr":
					c.Expr = v.Value
				case "level":
					c.Level = v.Value
				default:
					return nil, withLine(k, fmt.Errorf("unknown constraint key %q (expected expr/level)", k.Value))
				}
			}
			if c.Expr == "" {
				return nil, withLine(item, fmt.Errorf("constraint entry requires 'expr'"))
			}
			if c.Level != "mandatory" && c.Level != "advisory" {
				return nil, withLine(item, fmt.Errorf("constraint level must be advisory|mandatory, got %q", c.Level))
			}
			out = append(out, c)
		default:
			return nil, withLine(item, fmt.Errorf("constraint entry must be a string or {expr, level}"))
		}
	}
	return out, nil
}

// ---------- variable constraints (`name!`) ----------

// varConstraint binds a `name!` declaration to its check expression.
type varConstraint struct {
	name string
	spec string
	pos  Pos
}

// checkVarConstraints verifies `name!` declarations against the final env.
// Spec forms: `in: a|b|c` (union literal) or a boolean expression where
// `$it` is the variable's value (`$it >= 1 && $it <= 10`).
func checkVarConstraints(cons []varConstraint, env map[string]Value) error {
	for _, c := range cons {
		val, ok := env[c.name]
		spec := strings.TrimSpace(c.spec)
		var fail error
		switch {
		case strings.HasPrefix(spec, "in:"):
			allowed := strings.Split(strings.TrimSpace(spec[3:]), "|")
			found := false
			for _, a := range allowed {
				if fmt.Sprint(val) == strings.TrimSpace(a) {
					found = true
					break
				}
			}
			if !found {
				fail = fmt.Errorf("variable %s = %v not in (%s)", c.name, val, spec[3:])
			}
		case strings.Contains(spec, "|") && !strings.ContainsAny(spec, `"'()&$`):
			// bare union literal without `in:` prefix (e.g. dev|staging|prod)
			found := false
			for _, a := range strings.Split(spec, "|") {
				if fmt.Sprint(val) == strings.TrimSpace(a) {
					found = true
					break
				}
			}
			if !found {
				fail = fmt.Errorf("variable %s = %v not in (%s)", c.name, val, spec)
			}
		default:
			eenv := cloneEnv(env)
			eenv["it"] = val
			v, err := EvalExpr(spec, eenv)
			if err != nil {
				fail = fmt.Errorf("constraint for variable %s: %w", c.name, err)
			} else if !truthy(v) {
				fail = fmt.Errorf("constraint violated: %s! = %q (value: %v)", c.name, spec, val)
			}
		}
		if fail != nil {
			if !ok {
				fail = fmt.Errorf("constraint on undeclared variable %s: %v", c.name, fail)
			}
			return fmt.Errorf("%d:%d: %v", c.pos.Line, c.pos.Col, fail)
		}
	}
	return nil
}

// sortedMapKeys is a small helper for deterministic output.
func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
