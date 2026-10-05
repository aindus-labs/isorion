// Constraint checking between expansion and emission (I7 items C+M).
// Constraints live in the document (`constraints:`) and are evaluated on
// the expanded body — before any emitter runs.
//
//	expr: "services.*.replicas >= 1"   → path form, evaluated per match
//	expr: "len($ports) > 0"            → expression form, evaluated once
//	level: advisory                    → warning, never fails the pipeline
//	level: mandatory                   → error (default)
package model

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/isorion-lang/isorion/pkg/lang"
)

// cmpSplitRe finds the leftmost comparison operator in a constraint expr.
var cmpSplitRe = regexp.MustCompile(`^(.*?)\s*(==|!=|<=|>=|>|<)\s*(.*)$`)

// pathRe matches the LHS path form: services.*.replicas
var pathRe = regexp.MustCompile(`^[A-Za-z_][\w]*(\.[A-Za-z_*][\w*]*)*$`)

type hit struct {
	val  any
	name string // last `*` map key, or the last path segment
}

// CheckConstraints evaluates every constraint on the expanded body.
// Mandatory failures are aggregated into the returned error; advisory
// failures are returned as warnings.
func CheckConstraints(body map[string]any, cs []lang.Constraint) (warnings []string, err error) {
	var failures []string
	for _, c := range cs {
		violations := checkOne(body, c.Expr)
		for _, v := range violations {
			msg := fmt.Sprintf("%d:%d: %s", c.Pos.Line, c.Pos.Col, v)
			if c.Level == "advisory" {
				warnings = append(warnings, "advisory constraint: "+msg)
			} else {
				failures = append(failures, msg)
			}
		}
	}
	if len(failures) > 0 {
		return warnings, fmt.Errorf("%d mandatory constraint(s) violated:\n  %s",
			len(failures), strings.Join(failures, "\n  "))
	}
	return warnings, nil
}

func checkOne(body map[string]any, expr string) []string {
	m := cmpSplitRe.FindStringSubmatch(strings.TrimSpace(expr))
	if m != nil && pathRe.MatchString(strings.TrimSpace(m[1])) {
		return checkPath(body, strings.TrimSpace(m[1]), m[2], strings.TrimSpace(m[3]), expr)
	}
	// expression form: whole string evaluated once (env has no vars → $it=body)
	v, err := lang.EvalExpr(expr, map[string]lang.Value{"it": body})
	if err != nil {
		return []string{fmt.Sprintf("constraint %q: %v", expr, err)}
	}
	if !lang.Truthy(v) {
		return []string{fmt.Sprintf("constraint %q evaluated to %v", expr, v)}
	}
	return nil
}

// checkPath resolves a dotted path with `*` wildcards over the body and
// evaluates `<op> <rhs>` per matched value (`$it` = value, `$name` = key).
func checkPath(body map[string]any, path, op, rhs, expr string) []string {
	segs := strings.Split(path, ".")
	hits, missing := walk(body, segs)
	if len(hits) == 0 {
		if missing {
			return []string{fmt.Sprintf("constraint %q: path %s not found", expr, path)}
		}
		return []string{fmt.Sprintf("constraint %q: path %s matched no value", expr, path)}
	}
	var out []string
	for _, h := range hits {
		eenv := map[string]lang.Value{"it": h.val, "name": h.name}
		v, err := lang.EvalExpr("$it "+op+" "+rhs, eenv)
		if err != nil {
			out = append(out, fmt.Sprintf("constraint %q: %v (at %s=%v)", expr, err, h.name, h.val))
			continue
		}
		if !lang.Truthy(v) {
			out = append(out, fmt.Sprintf("constraint %q violated at %s (value: %v)", expr, h.name, h.val))
		}
	}
	return out
}

// walk resolves path segments over nested maps; `*` iterates map keys
// (sorted for determinism). Returns matched hits; missing reports whether
// a concrete segment was absent.
func walk(cur any, segs []string) (hits []hit, missing bool) {
	if len(segs) == 0 {
		return []hit{{val: cur}}, false
	}
	seg, rest := segs[0], segs[1:]
	m, ok := cur.(map[string]any)
	if !ok {
		return nil, true
	}
	if seg == "*" {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sub, miss := walk(m[k], rest)
			if !miss {
				for _, h := range sub {
					h.name = k // the wildcard key names the context (service name…)
					hits = append(hits, h)
				}
			}
		}
		return hits, len(hits) == 0
	}
	v, exists := m[seg]
	if !exists {
		return nil, true
	}
	sub, miss := walk(v, rest)
	if len(segs) == 1 {
		for i := range sub {
			if sub[i].name == "" {
				sub[i].name = seg
			}
		}
	}
	return sub, miss
}
