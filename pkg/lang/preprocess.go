// Package lang implements the ISORION "YAML++" language front-end:
// directive preprocessing, expression evaluation, and tree expansion.
//
// A .isor file is a YAML document extended with:
//   - a `variables:` section (referenced as $name)
//   - directive keys: @if / @elif / @else / @for / @include / @extends
//   - ${expr} and $name interpolation inside scalars
//
// YAML reserves '@' as the first character of a plain scalar, so directive
// keys are rewritten to quoted form before yaml.v3 sees the document.
// This keeps 100% YAML compatibility for everything that is not a directive.
package lang

import (
	"regexp"
	"strings"
)

// dirKeyRe matches a directive key in mapping or sequence position:
//
//	@if $env == "prod":
//	@else:
//	@for $p in $ports:
//	  - @if cond:            (sequence item)
var dirKeyRe = regexp.MustCompile(`^(\s*(?:-\s+)?)(@(?:if|elif|else|for|include|extends)\b[^:#]*):(.*)$`)

// Preprocess rewrites directive keys into quoted keys so yaml.v3 accepts them.
// Comments on the same line after the directive are preserved.
func Preprocess(src string) string {
	var out strings.Builder
	out.Grow(len(src) + len(src)/8)
	for _, line := range strings.Split(src, "\n") {
		m := dirKeyRe.FindStringSubmatch(line)
		if m == nil {
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		indent, key, rest := m[1], strings.TrimSpace(m[2]), m[3]
		key = strings.ReplaceAll(key, `\`, `\\`)
		key = strings.ReplaceAll(key, `"`, `\"`)
		out.WriteString(indent)
		out.WriteByte('"')
		out.WriteString(key)
		out.WriteString(`":`)
		out.WriteString(rest)
		out.WriteByte('\n')
	}
	return out.String()
}

// DirectiveKind is the kind of a parsed directive key.
type DirectiveKind int

const (
	DirNone DirectiveKind = iota
	DirIf
	DirElif
	DirElse
	DirFor
	DirInclude
	DirExtends
)

var dirHeadRe = regexp.MustCompile(`^@(if|elif|else|for|include|extends)\b\s*(.*)$`)

// ParseDirective splits a (preprocessed, quoted) key into its directive kind
// and argument text. Returns DirNone for ordinary keys.
func ParseDirective(key string) (DirectiveKind, string) {
	m := dirHeadRe.FindStringSubmatch(strings.TrimSpace(key))
	if m == nil {
		return DirNone, ""
	}
	switch m[1] {
	case "if":
		return DirIf, strings.TrimSpace(m[2])
	case "elif":
		return DirElif, strings.TrimSpace(m[2])
	case "else":
		return DirElse, strings.TrimSpace(m[2])
	case "for":
		return DirFor, strings.TrimSpace(m[2])
	case "include":
		return DirInclude, strings.TrimSpace(m[2])
	case "extends":
		return DirExtends, strings.TrimSpace(m[2])
	}
	return DirNone, ""
}
