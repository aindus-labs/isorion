package lang

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// Value is a dynamically-typed expression value. It mirrors YAML scalars and
// collections: string, float64, int64, bool, []any, map[string]any, nil.
type Value = any

// ---------- Lexer ----------

type tokKind int

const (
	tEOF   tokKind = iota
	tVar           // $name
	tIdent         // bare word (function names, true/false)
	tNumber
	tString
	tOp // == != <= >= < > && || + - * / !
	tLParen
	tRParen
	tComma
)

type token struct {
	kind tokKind
	text string
}

func lexExpr(src string) ([]token, error) {
	var toks []token
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c == '$':
			j := i + 1
			for j < len(src) && (unicode.IsLetter(rune(src[j])) || unicode.IsDigit(rune(src[j])) || src[j] == '_') {
				j++
			}
			if j == i+1 {
				return nil, fmt.Errorf("lone '$' at offset %d", i)
			}
			toks = append(toks, token{tVar, src[i+1 : j]})
			i = j
		case c == '"' || c == '\'':
			q := c
			j := i + 1
			var sb strings.Builder
			for j < len(src) && src[j] != q {
				if src[j] == '\\' && j+1 < len(src) {
					j++
					switch src[j] {
					case 'n':
						sb.WriteByte('\n')
					case 't':
						sb.WriteByte('\t')
					default:
						sb.WriteByte(src[j])
					}
					j++
					continue
				}
				sb.WriteByte(src[j])
				j++
			}
			if j >= len(src) {
				return nil, fmt.Errorf("unterminated string at offset %d", i)
			}
			toks = append(toks, token{tString, sb.String()})
			i = j + 1
		case unicode.IsDigit(rune(c)) || (c == '.' && i+1 < len(src) && unicode.IsDigit(rune(src[i+1]))):
			j := i
			for j < len(src) && (unicode.IsDigit(rune(src[j])) || src[j] == '.') {
				j++
			}
			toks = append(toks, token{tNumber, src[i:j]})
			i = j
		case unicode.IsLetter(rune(c)) || c == '_':
			j := i
			for j < len(src) && (unicode.IsLetter(rune(src[j])) || unicode.IsDigit(rune(src[j])) || src[j] == '_' || src[j] == '-') {
				j++
			}
			toks = append(toks, token{tIdent, src[i:j]})
			i = j
		case c == '(':
			toks = append(toks, token{tLParen, "("})
			i++
		case c == ')':
			toks = append(toks, token{tRParen, ")"})
			i++
		case c == ',':
			toks = append(toks, token{tComma, ","})
			i++
		default:
			if i+1 < len(src) {
				two := src[i : i+2]
				if two == "==" || two == "!=" || two == "<=" || two == ">=" || two == "&&" || two == "||" {
					toks = append(toks, token{tOp, two})
					i += 2
					continue
				}
			}
			if c == '<' || c == '>' || c == '+' || c == '-' || c == '*' || c == '/' || c == '!' {
				toks = append(toks, token{tOp, string(c)})
				i++
				continue
			}
			return nil, fmt.Errorf("unexpected character %q at offset %d", string(c), i)
		}
	}
	return append(toks, token{tEOF, ""}), nil
}

// ---------- Parser ----------

type exprNode interface {
	eval(env map[string]Value) (Value, error)
}

type litNode struct{ v Value }
type varNode struct{ name string }
type unaryNode struct {
	op string
	x  exprNode
}
type binNode struct {
	op   string
	l, r exprNode
}
type callNode struct {
	name string
	args []exprNode
}

type exprParser struct {
	toks []token
	pos  int
}

func (p *exprParser) peek() token { return p.toks[p.pos] }
func (p *exprParser) next() token { t := p.toks[p.pos]; p.pos++; return t }
func (p *exprParser) isOp(op string) bool {
	t := p.peek()
	if t.kind == tOp && t.text == op {
		return true
	}
	// word operators
	return t.kind == tIdent && (t.text == op)
}

func (p *exprParser) parseOr() (exprNode, error) {
	l, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.isOp("||") || p.isOp("or") {
		p.next()
		r, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		l = binNode{op: "||", l: l, r: r}
	}
	return l, nil
}

func (p *exprParser) parseAnd() (exprNode, error) {
	l, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.isOp("&&") || p.isOp("and") {
		p.next()
		r, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		l = binNode{op: "&&", l: l, r: r}
	}
	return l, nil
}

func (p *exprParser) parseUnary() (exprNode, error) {
	if p.isOp("!") || p.isOp("not") {
		p.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return unaryNode{op: "!", x: x}, nil
	}
	if p.isOp("-") {
		p.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return unaryNode{op: "-", x: x}, nil
	}
	return p.parseCmp()
}

func (p *exprParser) parseCmp() (exprNode, error) {
	l, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	for _, op := range []string{"==", "!=", "<=", ">=", "<", ">"} {
		if p.isOp(op) {
			p.next()
			r, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			return binNode{op: op, l: l, r: r}, nil
		}
	}
	return l, nil
}

func (p *exprParser) parseAdd() (exprNode, error) {
	l, err := p.parseMul()
	if err != nil {
		return nil, err
	}
	for p.isOp("+") || p.isOp("-") {
		op := p.next().text
		r, err := p.parseMul()
		if err != nil {
			return nil, err
		}
		l = binNode{op: op, l: l, r: r}
	}
	return l, nil
}

func (p *exprParser) parseMul() (exprNode, error) {
	l, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for p.isOp("*") || p.isOp("/") {
		op := p.next().text
		r, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		l = binNode{op: op, l: l, r: r}
	}
	return l, nil
}

func (p *exprParser) parsePrimary() (exprNode, error) {
	t := p.next()
	switch t.kind {
	case tNumber:
		if strings.Contains(t.text, ".") {
			f, _ := strconv.ParseFloat(t.text, 64)
			return litNode{f}, nil
		}
		n, _ := strconv.ParseInt(t.text, 10, 64)
		return litNode{n}, nil
	case tString:
		return litNode{t.text}, nil
	case tVar:
		return varNode{name: t.text}, nil
	case tIdent:
		switch t.text {
		case "true":
			return litNode{true}, nil
		case "false":
			return litNode{false}, nil
		case "null", "nil":
			return litNode{nil}, nil
		}
		// function call
		if p.peek().kind == tLParen {
			p.next()
			var args []exprNode
			if p.peek().kind != tRParen {
				for {
					a, err := p.parseOr()
					if err != nil {
						return nil, err
					}
					args = append(args, a)
					if p.peek().kind == tComma {
						p.next()
						continue
					}
					break
				}
			}
			if p.peek().kind != tRParen {
				return nil, fmt.Errorf("expected ')' after arguments of %s", t.text)
			}
			p.next()
			return callNode{name: t.text, args: args}, nil
		}
		// bare identifier = string literal (convenient: $mode == fast)
		return litNode{t.text}, nil
	case tLParen:
		x, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tRParen {
			return nil, fmt.Errorf("expected ')'")
		}
		p.next()
		return x, nil
	}
	return nil, fmt.Errorf("unexpected token %q", t.text)
}

// ---------- Eval ----------

func (n litNode) eval(env map[string]Value) (Value, error) { return n.v, nil }

func (n varNode) eval(env map[string]Value) (Value, error) {
	if v, ok := env[n.name]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("undefined variable $%s", n.name)
}

func (n unaryNode) eval(env map[string]Value) (Value, error) {
	v, err := n.x.eval(env)
	if err != nil {
		return nil, err
	}
	switch n.op {
	case "!":
		return !truthy(v), nil
	case "-":
		f, ok := toFloat(v)
		if !ok {
			return nil, fmt.Errorf("cannot negate %v", v)
		}
		return -f, nil
	}
	return nil, fmt.Errorf("unknown unary op %s", n.op)
}

func (n binNode) eval(env map[string]Value) (Value, error) {
	switch n.op {
	case "&&":
		l, err := n.l.eval(env)
		if err != nil {
			return nil, err
		}
		if !truthy(l) {
			return false, nil
		}
		r, err := n.r.eval(env)
		if err != nil {
			return nil, err
		}
		return truthy(r), nil
	case "||":
		l, err := n.l.eval(env)
		if err != nil {
			return nil, err
		}
		if truthy(l) {
			return true, nil
		}
		r, err := n.r.eval(env)
		if err != nil {
			return nil, err
		}
		return truthy(r), nil
	}

	l, err := n.l.eval(env)
	if err != nil {
		return nil, err
	}
	r, err := n.r.eval(env)
	if err != nil {
		return nil, err
	}

	switch n.op {
	case "==":
		return deepEqual(l, r), nil
	case "!=":
		return !deepEqual(l, r), nil
	case "<", "<=", ">", ">=":
		lf, lok := toFloat(l)
		rf, rok := toFloat(r)
		if lok && rok {
			switch n.op {
			case "<":
				return lf < rf, nil
			case "<=":
				return lf <= rf, nil
			case ">":
				return lf > rf, nil
			case ">=":
				return lf >= rf, nil
			}
		}
		ls, rs := fmt.Sprint(l), fmt.Sprint(r)
		switch n.op {
		case "<":
			return ls < rs, nil
		case "<=":
			return ls <= rs, nil
		case ">":
			return ls > rs, nil
		case ">=":
			return ls >= rs, nil
		}
	case "+":
		lf, lok := toFloat(l)
		rf, rok := toFloat(r)
		if lok && rok {
			if isInt(l) && isInt(r) {
				return int64(lf) + int64(rf), nil
			}
			return lf + rf, nil
		}
		return fmt.Sprint(l) + fmt.Sprint(r), nil
	case "-", "*", "/":
		lf, lok := toFloat(l)
		rf, rok := toFloat(r)
		if !lok || !rok {
			return nil, fmt.Errorf("operator %s requires numbers, got %v and %v", n.op, l, r)
		}
		var res float64
		switch n.op {
		case "-":
			res = lf - rf
		case "*":
			res = lf * rf
		case "/":
			if rf == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			res = lf / rf
		}
		if isInt(l) && isInt(r) && n.op != "/" {
			return int64(res), nil
		}
		return res, nil
	}
	return nil, fmt.Errorf("unknown operator %s", n.op)
}

func (n callNode) eval(env map[string]Value) (Value, error) {
	args := make([]Value, len(n.args))
	for i, a := range n.args {
		v, err := a.eval(env)
		if err != nil {
			return nil, err
		}
		args[i] = v
	}
	switch n.name {
	case "env":
		if len(args) != 1 {
			return nil, fmt.Errorf("env() takes 1 argument")
		}
		return os.Getenv(fmt.Sprint(args[0])), nil
	case "len":
		if len(args) != 1 {
			return nil, fmt.Errorf("len() takes 1 argument")
		}
		switch c := args[0].(type) {
		case string:
			return int64(len(c)), nil
		case []any:
			return int64(len(c)), nil
		case map[string]any:
			return int64(len(c)), nil
		}
		return nil, fmt.Errorf("len() unsupported type %T", args[0])
	case "upper":
		return strings.ToUpper(fmt.Sprint(args[0])), nil
	case "lower":
		return strings.ToLower(fmt.Sprint(args[0])), nil
	case "default":
		if len(args) != 2 {
			return nil, fmt.Errorf("default() takes 2 arguments")
		}
		if args[0] == nil || args[0] == "" {
			return args[1], nil
		}
		return args[0], nil
	case "int":
		if f, ok := toFloat(args[0]); ok {
			return int64(f), nil
		}
		return nil, fmt.Errorf("int() cannot convert %v", args[0])
	case "string":
		return fmt.Sprint(args[0]), nil
	case "secret":
		if len(args) != 1 {
			return nil, fmt.Errorf("secret() takes 1 argument")
		}
		name := fmt.Sprint(args[0])
		secrets, _ := env[envSecretsKey].(map[string]string)
		if _, ok := secrets[name]; !ok {
			return nil, fmt.Errorf("undefined secret %q (declare it under secrets:)", name)
		}
		// Emitters receive a reference marker, never the encrypted value.
		return SecretMarker + name, nil
	}
	return nil, fmt.Errorf("unknown function %s()", n.name)
}

// ---------- helpers ----------

// Truthy reports whether v is truthy under YAML++ rules (exposed for
// constraint checking in pkg/model).
func Truthy(v Value) bool { return truthy(v) }

func truthy(v Value) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != "" && x != "false" && x != "0"
	case int64:
		return x != 0
	case float64:
		return x != 0
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}

func isInt(v Value) bool {
	switch v.(type) {
	case int64, int, int32:
		return true
	}
	return false
}

func toFloat(v Value) (float64, bool) {
	switch x := v.(type) {
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	case float64:
		return x, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	}
	return 0, false
}

func deepEqual(a, b Value) bool {
	if fa, ok := toFloat(a); ok {
		if fb, ok2 := toFloat(b); ok2 {
			return fa == fb
		}
	}
	switch x := a.(type) {
	case string:
		y, ok := b.(string)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case nil:
		return b == nil
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !deepEqual(x[i], y[i]) {
				return false
			}
		}
		return true
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

// EvalExpr parses and evaluates a YAML++ expression against env.
func EvalExpr(src string, env map[string]Value) (Value, error) {
	toks, err := lexExpr(src)
	if err != nil {
		return nil, err
	}
	p := &exprParser{toks: toks}
	n, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tEOF {
		return nil, fmt.Errorf("trailing tokens in expression %q", src)
	}
	return n.eval(env)
}
