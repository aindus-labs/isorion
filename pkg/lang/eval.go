package lang

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Pos is a 1-based line/column position in the source file.
type Pos struct {
	Line, Col int
}

// LineError annotates an expansion error with the position of its node.
// Error() forwards the inner message so `file:line:col:` prefixes stay clean.
type LineError struct {
	Pos Pos
	Err error
}

func (e *LineError) Error() string { return e.Err.Error() }
func (e *LineError) Unwrap() error { return e.Err }

// withLine tags err with n's position; a deeper LineError already present wins.
func withLine(n *yaml.Node, err error) error {
	if err == nil {
		return nil
	}
	var le *LineError
	if errors.As(err, &le) {
		return err
	}
	return &LineError{Pos: Pos{Line: n.Line, Col: n.Column}, Err: err}
}

// formatErr renders err as `filename:line:col: msg` when it carries a
// position, `filename: msg` otherwise.
func formatErr(filename string, err error) error {
	var le *LineError
	if errors.As(err, &le) {
		return fmt.Errorf("%s:%d:%d: %w", filename, le.Pos.Line, le.Pos.Col, err)
	}
	return fmt.Errorf("%s: %w", filename, err)
}

// Document is a fully loaded and expanded .isor file.
type Document struct {
	Version     string            // top-level `version:` field (inherited via @extends)
	Variables   map[string]Value  // declared + injected variables (consumed)
	Body        map[string]Value  // expanded document, minus reserved sections
	Source      string            // original file path (diagnostics)
	Positions   map[string]Pos    // top-level key → position (diagnostics)
	Profile     string            // selected profile, "" if none
	Secrets     map[string]string // `secrets:` name → ENC[...] ciphertext
	Constraints []Constraint      // `constraints:` entries (own + merged)
}

// Options configures external injection into a .isor document (CLI
// --set/--profile). Values merge into `variables:` before expansion.
type Options struct {
	Profile string            // `profiles:` entry merged over `variables:`
	Set     map[string]string // literal overrides applied last (--set name=value)
}

// Load parses, expands and returns a Document from raw .isor source.
func Load(filename string, src []byte) (*Document, error) {
	return LoadWith(filename, src, Options{})
}

// LoadWith is Load with external variable injection (CLI --set/--profile).
// It also resolves @extends (deep-merge of a parent document) and
// initializes the shared secrets/constraints stores.
func LoadWith(filename string, src []byte, opts Options) (*Document, error) {
	top, err := parseDoc(filename, src)
	if err != nil {
		return nil, err
	}
	if top.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: top-level node must be a mapping", filename)
	}

	abs, _ := filepath.Abs(filename)
	ld := &loader{dir: filepath.Dir(abs), loading: map[string]bool{abs: true}}

	doc := &Document{
		Source:    filename,
		Variables: map[string]Value{},
		Positions: map[string]Pos{},
		Profile:   opts.Profile,
	}

	sec, err := scanTop(top, doc.Positions)
	if err != nil {
		return nil, formatErr(filename, err)
	}
	doc.Version = sec.version

	// Expansion env: variables + reserved keys (loader, secrets, constraints).
	env := map[string]Value{envLoaderKey: ld}

	var vcons []varConstraint
	if sec.vars != nil {
		vars, vc, err := loadVariables(sec.vars, env)
		if err != nil {
			return nil, formatErr(filename, err)
		}
		doc.Variables = vars
		vcons = vc
	}
	for k, v := range doc.Variables {
		env[k] = v
	}

	// --profile: merge the selected profiles: block over variables:.
	if opts.Profile != "" {
		vars, err := selectProfile(sec.profiles, opts.Profile, env)
		if err != nil {
			return nil, formatErr(filename, err)
		}
		for k, v := range vars {
			doc.Variables[k] = v
			env[k] = v
		}
	}

	// --set: literal overrides, applied last (highest precedence).
	for name, raw := range opts.Set {
		v := parseScalar(raw)
		doc.Variables[name] = v
		env[name] = v
	}

	// `name!` variable constraints checked against the final env.
	if err := checkVarConstraints(vcons, env); err != nil {
		return nil, formatErr(filename, err)
	}

	// Shared stores: secrets + constraints accumulate through includes.
	secrets := envSecrets(env)
	if sec.secrets != nil {
		own, err := parseSecrets(sec.secrets)
		if err != nil {
			return nil, formatErr(filename, err)
		}
		for k, v := range own {
			secrets[k] = v
		}
	}
	consAll := envConstraints(env)
	if sec.constraints != nil {
		own, err := parseConstraints(sec.constraints)
		if err != nil {
			return nil, formatErr(filename, err)
		}
		*consAll = append(*consAll, own...)
	}

	body, err := expandPairs(sec.bodyPairs, env)
	if err != nil {
		return nil, formatErr(filename, err)
	}

	// @extends: deep-merge the parent document (child refines).
	if sec.ext != nil {
		pfd, err := expandChildFile(sec.ext.ref, env)
		if err != nil {
			return nil, formatErr(filename, withLine(sec.ext.node, err))
		}
		pm, ok := pfd.body.(map[string]Value)
		if !ok {
			return nil, formatErr(filename, withLine(sec.ext.node,
				fmt.Errorf("@extends %s: document body must be a mapping", sec.ext.ref.Path)))
		}
		body = deepMerge(pm, body).(map[string]Value)
		if doc.Version == "" {
			doc.Version = pfd.version
		}
	}

	doc.Body = body
	doc.Secrets = secrets
	doc.Constraints = *consAll
	return doc, nil
}

func scalarString(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	return n.Value
}

// loadVariables expands `variables:` sequentially (later vars see earlier
// ones) on top of base. Keys ending in `!` declare constraints checked
// after the final environment is known (see checkVarConstraints).
func loadVariables(n *yaml.Node, base map[string]Value) (map[string]Value, []varConstraint, error) {
	if n.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("variables must be a mapping")
	}
	env := cloneEnv(base)
	out := map[string]Value{}
	var cons []varConstraint
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		name := strings.TrimPrefix(k.Value, "$")
		if strings.HasSuffix(name, "!") {
			cons = append(cons, varConstraint{
				name: strings.TrimSuffix(name, "!"),
				spec: scalarString(v),
				pos:  Pos{Line: k.Line, Col: k.Column},
			})
			continue
		}
		val, err := expandNode(v, env)
		if err != nil {
			return nil, nil, withLine(k, fmt.Errorf("variable %s: %w", name, err))
		}
		env[name] = val
		out[name] = val
	}
	return out, cons, nil
}

// selectProfile expands the `profiles:` entry named `name`. Its variables
// expand against the already-loaded variables: and can reference each other.
func selectProfile(profiles *yaml.Node, name string, env map[string]Value) (map[string]Value, error) {
	if profiles == nil {
		return nil, fmt.Errorf("profile %q requested but the document has no `profiles:` section", name)
	}
	if profiles.Kind != yaml.MappingNode {
		return nil, withLine(profiles, fmt.Errorf("profiles must be a mapping"))
	}
	var names []string
	for i := 0; i+1 < len(profiles.Content); i += 2 {
		k, v := profiles.Content[i], profiles.Content[i+1]
		names = append(names, k.Value)
		if k.Value != name {
			continue
		}
		if v.Kind != yaml.MappingNode {
			return nil, withLine(k, fmt.Errorf("profile %q must be a mapping", name))
		}
		out := map[string]Value{}
		sub := cloneEnv(env)
		for j := 0; j+1 < len(v.Content); j += 2 {
			k2, v2 := v.Content[j], v.Content[j+1]
			vname := strings.TrimPrefix(k2.Value, "$")
			val, err := expandNode(v2, sub)
			if err != nil {
				return nil, withLine(k2, fmt.Errorf("profile %s: variable %s: %w", name, vname, err))
			}
			out[vname] = val
			sub[vname] = val
		}
		return out, nil
	}
	return nil, fmt.Errorf("unknown profile %q (available: %s)", name, strings.Join(names, ", "))
}

// parseScalar turns a --set literal into a typed value (int, float, bool,
// list, map…) via YAML scalar decoding; falls back to the raw string.
func parseScalar(s string) Value {
	var v any
	if err := yaml.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}

// ---------- expansion ----------

// expandPairs expands a sequence of [key,value,key,value,...] yaml nodes into
// a map, honouring @if/@elif/@else (merge/omit) and @for (repeat-merge).
func expandPairs(pairs []*yaml.Node, env map[string]Value) (map[string]Value, error) {
	out := map[string]Value{}
	condTaken := false // a preceding @if/@elif was satisfied
	condOpen := false  // inside an @if group (elif/else legal)

	flushCond := func() { condTaken, condOpen = false, false }

	for i := 0; i+1 < len(pairs); i += 2 {
		kn, vn := pairs[i], pairs[i+1]
		kind, arg := ParseDirective(kn.Value)

		switch kind {
		case DirIf:
			ok, err := evalCond(arg, env)
			if err != nil {
				return nil, withLine(kn, err)
			}
			condTaken, condOpen = ok, true
			if ok {
				if err := mergeBlock(vn, env, out); err != nil {
					return nil, err
				}
			}
		case DirElif:
			if !condOpen {
				return nil, withLine(kn, fmt.Errorf("@elif without preceding @if"))
			}
			if condTaken {
				continue
			}
			ok, err := evalCond(arg, env)
			if err != nil {
				return nil, withLine(kn, err)
			}
			if ok {
				condTaken = true
				if err := mergeBlock(vn, env, out); err != nil {
					return nil, err
				}
			}
		case DirElse:
			if !condOpen {
				return nil, withLine(kn, fmt.Errorf("@else without preceding @if"))
			}
			if !condTaken {
				if err := mergeBlock(vn, env, out); err != nil {
					return nil, err
				}
			}
			flushCond()
		case DirFor:
			flushCond()
			if err := forEachMapping(arg, vn, env, out); err != nil {
				return nil, withLine(kn, err)
			}
		case DirInclude:
			flushCond()
			ref, err := parseFileRef(arg, vn)
			if err != nil {
				return nil, withLine(kn, err)
			}
			fd, err := expandChildFile(ref, env)
			if err != nil {
				return nil, withLine(kn, err)
			}
			m, ok := fd.body.(map[string]Value)
			if !ok {
				return nil, withLine(kn, fmt.Errorf("@include %s in mapping context requires a mapping document", ref.Path))
			}
			for k, v := range m {
				out[k] = v
			}
		case DirExtends:
			return nil, withLine(kn, fmt.Errorf("@extends is only allowed at document top level"))
		default:
			flushCond()
			key, err := interpScalar(kn.Value, env)
			if err != nil {
				return nil, withLine(kn, err)
			}
			ks := fmt.Sprint(key)
			if kind2, _ := ParseDirective(ks); kind2 != DirNone {
				return nil, withLine(kn, fmt.Errorf("key %q resolves to a directive", ks))
			}
			val, err := expandNode(vn, env)
			if err != nil {
				return nil, withLine(kn, fmt.Errorf("key %q: %w", ks, err))
			}
			if strings.HasPrefix(ks, "_") {
				continue // _hidden field: resolved, never emitted
			}
			out[ks] = val
		}
	}
	return out, nil
}

// mergeBlock merges a directive's block value into a parent mapping.
// The value must be a mapping (its children are lifted one level up).
func mergeBlock(vn *yaml.Node, env map[string]Value, out map[string]Value) error {
	if vn.Kind != yaml.MappingNode {
		return withLine(vn, fmt.Errorf("conditional block must be a mapping, got %s", nodeKind(vn)))
	}
	sub, err := expandPairs(vn.Content, env)
	if err != nil {
		return err
	}
	for k, v := range sub {
		out[k] = v
	}
	return nil
}

// forEachMapping implements `@for $x in <expr>` in mapping context:
// the block (a mapping template) is evaluated once per item and merged.
func forEachMapping(arg string, vn *yaml.Node, env map[string]Value, out map[string]Value) error {
	varName, expr, err := parseFor(arg)
	if err != nil {
		return err
	}
	items, err := EvalExpr(expr, env)
	if err != nil {
		return fmt.Errorf("@for %s: %w", expr, err)
	}
	list, ok := items.([]any)
	if !ok {
		return fmt.Errorf("@for %s: expression must evaluate to a list, got %v", expr, items)
	}
	if vn.Kind != yaml.MappingNode {
		return fmt.Errorf("@for in mapping context requires a mapping block")
	}
	for _, item := range list {
		env2 := cloneEnv(env)
		env2[varName] = item
		sub, err := expandPairs(vn.Content, env2)
		if err != nil {
			return err
		}
		for k, v := range sub {
			out[k] = v
		}
	}
	return nil
}

var forRe = regexp.MustCompile(`^\$?([A-Za-z_][A-Za-z0-9_]*)\s+in\s+(.+)$`)

func parseFor(arg string) (string, string, error) {
	m := forRe.FindStringSubmatch(arg)
	if m == nil {
		return "", "", fmt.Errorf("invalid @for syntax %q, expected '@for $var in <expr>'", arg)
	}
	return m[1], m[2], nil
}

func evalCond(expr string, env map[string]Value) (bool, error) {
	if expr == "" {
		return false, fmt.Errorf("directive requires a condition expression")
	}
	v, err := EvalExpr(expr, env)
	if err != nil {
		return false, fmt.Errorf("condition %q: %w", expr, err)
	}
	return truthy(v), nil
}

// expandNode recursively expands a yaml node to a plain Go value.
func expandNode(n *yaml.Node, env map[string]Value) (Value, error) {
	switch n.Kind {
	case yaml.MappingNode:
		return expandPairs(n.Content, env)
	case yaml.SequenceNode:
		var out []any
		condTaken, condOpen := false, false
		for _, item := range n.Content {
			// A directive in sequence context is a single-key mapping item.
			if dir, arg, inner, isDir := seqDirective(item); isDir {
				switch dir {
				case DirIf:
					ok, err := evalCond(arg, env)
					if err != nil {
						return nil, withLine(item, err)
					}
					condTaken, condOpen = ok, true
					if ok {
						var err error
						out, err = spliceValue(inner, env, out)
						if err != nil {
							return nil, withLine(item, err)
						}
					}
				case DirElif:
					if !condOpen {
						return nil, withLine(item, fmt.Errorf("@elif without preceding @if in sequence"))
					}
					if condTaken {
						continue
					}
					ok, err := evalCond(arg, env)
					if err != nil {
						return nil, withLine(item, err)
					}
					if ok {
						condTaken = true
						var err error
						out, err = spliceValue(inner, env, out)
						if err != nil {
							return nil, withLine(item, err)
						}
					}
				case DirElse:
					if !condOpen {
						return nil, withLine(item, fmt.Errorf("@else without preceding @if in sequence"))
					}
					if !condTaken {
						var err error
						out, err = spliceValue(inner, env, out)
						if err != nil {
							return nil, withLine(item, err)
						}
					}
					condTaken, condOpen = false, false
				case DirFor:
					condTaken, condOpen = false, false
					varName, expr, err := parseFor(arg)
					if err != nil {
						return nil, withLine(item, err)
					}
					items, err := EvalExpr(expr, env)
					if err != nil {
						return nil, withLine(item, err)
					}
					list, ok := items.([]any)
					if !ok {
						return nil, withLine(item, fmt.Errorf("@for %s: must evaluate to a list", expr))
					}
					for _, it := range list {
						env2 := cloneEnv(env)
						env2[varName] = it
						var err error
						out, err = spliceValue(inner, env2, out)
						if err != nil {
							return nil, withLine(item, err)
						}
					}
				case DirInclude:
					condTaken, condOpen = false, false
					ref, err := parseFileRef(arg, inner)
					if err != nil {
						return nil, withLine(item, err)
					}
					fd, err := expandChildFile(ref, env)
					if err != nil {
						return nil, withLine(item, err)
					}
					if list, ok := fd.body.([]any); ok {
						out = append(out, list...)
					} else {
						out = append(out, fd.body)
					}
				case DirExtends:
					return nil, withLine(item, fmt.Errorf("@extends is only allowed at document top level"))
				}
				continue
			}
			condTaken, condOpen = false, false
			v, err := expandNode(item, env)
			if err != nil {
				return nil, withLine(item, err)
			}
			out = append(out, v)
		}
		return out, nil
	case yaml.ScalarNode:
		return expandScalar(n, env)
	case yaml.AliasNode:
		return expandNode(n.Alias, env)
	}
	return nil, nil
}

// seqDirective detects a sequence item that is a single-key directive mapping.
func seqDirective(item *yaml.Node) (DirectiveKind, string, *yaml.Node, bool) {
	if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
		return DirNone, "", nil, false
	}
	kind, arg := ParseDirective(item.Content[0].Value)
	if kind == DirNone {
		return DirNone, "", nil, false
	}
	return kind, arg, item.Content[1], true
}

// spliceValue expands a directive block value and appends it to out:
// sequences splice their items, anything else appends as one element.
func spliceValue(vn *yaml.Node, env map[string]Value, out []any) ([]any, error) {
	v, err := expandNode(vn, env)
	if err != nil {
		return nil, err
	}
	if list, ok := v.([]any); ok {
		return append(out, list...), nil
	}
	return append(out, v), nil
}

// expandScalar resolves a scalar node: typed YAML scalars stay typed,
// strings are interpolated ($name / ${expr}).
func expandScalar(n *yaml.Node, env map[string]Value) (Value, error) {
	switch n.Tag {
	case "!!int":
		if i, err := strconv.ParseInt(n.Value, 0, 64); err == nil {
			return i, nil
		}
	case "!!float":
		if f, err := strconv.ParseFloat(n.Value, 64); err == nil {
			return f, nil
		}
	case "!!bool":
		return n.Value == "true" || n.Value == "True" || n.Value == "TRUE", nil
	case "!!null":
		return nil, nil
	}
	// !!str (and untagged scalars) → interpolation
	return interpScalar(n.Value, env)
}

var interpExprRe = regexp.MustCompile(`\$\{([^}]+)\}`)

// interpScalar performs $name and ${expr} interpolation in a string.
// A scalar that is exactly `$name` or `${expr}` yields the native value.
func interpScalar(s string, env map[string]Value) (Value, error) {
	// escape hatch: $$ → literal $
	if strings.Contains(s, "$$") {
		s = strings.ReplaceAll(s, "$$", "\x00")
		defer func() {}()
	}

	// whole-scalar expression → native type
	if strings.HasPrefix(s, "$") && !strings.Contains(s, "${") {
		name := s[1:]
		if isIdent(name) {
			v, ok := env[name]
			if !ok {
				return nil, fmt.Errorf("undefined variable $%s", name)
			}
			return v, nil
		}
	}
	if m := interpExprRe.FindStringSubmatch(s); m != nil && m[0] == s {
		expr := strings.TrimSpace(m[1])
		if isIdent(expr) {
			if v, ok := env[expr]; ok {
				return v, nil
			}
		}
		return EvalExpr(expr, env)
	}

	// inner resolves a ${...} expression; bare identifiers bound in env are
	// variables (${r} == ${$r}), anything else goes through EvalExpr.
	inner := func(expr string) (string, error) {
		expr = strings.TrimSpace(expr)
		if isIdent(expr) {
			if v, ok := env[expr]; ok {
				return fmt.Sprint(v), nil
			}
		}
		v, e := EvalExpr(expr, env)
		if e != nil {
			return "", e
		}
		return fmt.Sprint(v), nil
	}

	// inline interpolation → always a string
	var err error
	res := interpExprRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := interpExprRe.FindStringSubmatch(match)
		v, e := inner(sub[1])
		if e != nil {
			err = e
			return ""
		}
		return v
	})
	if err != nil {
		return nil, err
	}

	// bare $name tokens inside a longer string
	res = regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`).ReplaceAllStringFunc(res, func(match string) string {
		name := match[1:]
		if v, ok := env[name]; ok {
			return fmt.Sprint(v)
		}
		return match
	})

	res = strings.ReplaceAll(res, "\x00", "$")
	return res, nil
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || (i > 0 && r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func cloneEnv(env map[string]Value) map[string]Value {
	out := make(map[string]Value, len(env)+1)
	for k, v := range env {
		out[k] = v
	}
	return out
}

func nodeKind(n *yaml.Node) string {
	switch n.Kind {
	case yaml.MappingNode:
		return "mapping"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.ScalarNode:
		return "scalar"
	}
	return "unknown"
}
