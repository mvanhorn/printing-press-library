// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// This is a small hand-rolled parser and tree walker: it never executes
// user text as code, only evaluates the parsed comparison tree against a row.
//
// Screener expression language, modelled on the InvestorPaisa screener box:
//
//	close > sma50 and sma50 > sma200 and rs >= 70
//
// Supported: numbers, field names, 'quoted' strings, + - * /, comparisons
// (> >= < <= == = !=), and/or/not (also && || !), and parentheses. A field
// that is missing on a row makes any comparison against it false.

type ipExprKind int

const (
	kNum ipExprKind = iota
	kStr
	kField
	kUnary
	kBinary
)

type ipExpr struct {
	kind  ipExprKind
	num   float64
	str   string
	op    string
	left  *ipExpr
	right *ipExpr
}

type ipToken struct {
	kind string // num, str, ident, op, lparen, rparen
	text string
	num  float64
}

var ipFieldAliases = map[string]string{
	"price": "close", "ltp": "close", "rsi": "rsi14", "chg": "chg1d", "change": "chg1d",
	"vol": "volume", "avgvol": "avgvol20", "beta": "beta1y", "dd": "maxdd1y",
}

func ipTokenize(src string) ([]ipToken, error) {
	var out []ipToken
	rs := []rune(src)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case unicode.IsSpace(r):
			i++
		case unicode.IsDigit(r) || (r == '.' && i+1 < len(rs) && unicode.IsDigit(rs[i+1])):
			j := i
			for j < len(rs) && (unicode.IsDigit(rs[j]) || rs[j] == '.') {
				j++
			}
			if j+1 < len(rs) && (rs[j] == 'e' || rs[j] == 'E') && (unicode.IsDigit(rs[j+1]) || ((rs[j+1] == '+' || rs[j+1] == '-') && j+2 < len(rs) && unicode.IsDigit(rs[j+2]))) {
				j += 2
				for j < len(rs) && unicode.IsDigit(rs[j]) {
					j++
				}
			}
			n, err := strconv.ParseFloat(string(rs[i:j]), 64)
			if err != nil {
				return nil, fmt.Errorf("bad number %q", string(rs[i:j]))
			}
			out = append(out, ipToken{kind: "num", num: n})
			i = j
		case unicode.IsLetter(r) || r == '_':
			j := i
			for j < len(rs) && (unicode.IsLetter(rs[j]) || unicode.IsDigit(rs[j]) || rs[j] == '_') {
				j++
			}
			word := strings.ToLower(string(rs[i:j]))
			switch word {
			case "and", "or", "not":
				out = append(out, ipToken{kind: "op", text: word})
			default:
				out = append(out, ipToken{kind: "ident", text: word})
			}
			i = j
		case r == '\'' || r == '"':
			j := i + 1
			for j < len(rs) && rs[j] != r {
				j++
			}
			if j >= len(rs) {
				return nil, fmt.Errorf("unterminated string starting at position %d", i+1)
			}
			out = append(out, ipToken{kind: "str", text: string(rs[i+1 : j])})
			i = j + 1
		case r == '(':
			out = append(out, ipToken{kind: "lparen"})
			i++
		case r == ')':
			out = append(out, ipToken{kind: "rparen"})
			i++
		default:
			two := ""
			if i+1 < len(rs) {
				two = string(rs[i : i+2])
			}
			switch two {
			case ">=", "<=", "==", "!=", "&&", "||":
				op := two
				if op == "&&" {
					op = "and"
				} else if op == "||" {
					op = "or"
				}
				out = append(out, ipToken{kind: "op", text: op})
				i += 2
				continue
			}
			switch r {
			case '>', '<', '+', '-', '*', '/':
				out = append(out, ipToken{kind: "op", text: string(r)})
			case '=':
				out = append(out, ipToken{kind: "op", text: "=="})
			case '!':
				out = append(out, ipToken{kind: "op", text: "not"})
			default:
				return nil, fmt.Errorf("unexpected character %q at position %d", string(r), i+1)
			}
			i++
		}
	}
	return out, nil
}

type ipParser struct {
	toks []ipToken
	pos  int
}

// Binding powers: or < and < comparison < +,- < *,/ < unary.
func ipBinding(op string) int {
	switch op {
	case "or":
		return 1
	case "and":
		return 2
	case ">", ">=", "<", "<=", "==", "!=":
		return 4
	case "+", "-":
		return 5
	case "*", "/":
		return 6
	}
	return 0
}

func (p *ipParser) peek() *ipToken {
	if p.pos < len(p.toks) {
		return &p.toks[p.pos]
	}
	return nil
}

func (p *ipParser) parse(minBP int) (*ipExpr, error) {
	t := p.peek()
	if t == nil {
		return nil, fmt.Errorf("expression ended unexpectedly")
	}
	p.pos++
	var left *ipExpr
	switch {
	case t.kind == "num":
		left = &ipExpr{kind: kNum, num: t.num}
	case t.kind == "str":
		left = &ipExpr{kind: kStr, str: t.text}
	case t.kind == "ident":
		name := t.text
		if alias, ok := ipFieldAliases[name]; ok {
			name = alias
		}
		left = &ipExpr{kind: kField, str: name}
	case t.kind == "lparen":
		inner, err := p.parse(0)
		if err != nil {
			return nil, err
		}
		if n := p.peek(); n == nil || n.kind != "rparen" {
			return nil, fmt.Errorf("missing closing parenthesis")
		}
		p.pos++
		left = inner
	case t.kind == "op" && (t.text == "not" || t.text == "-"):
		bp := 3
		if t.text == "-" {
			bp = 7
		}
		operand, err := p.parse(bp)
		if err != nil {
			return nil, err
		}
		left = &ipExpr{kind: kUnary, op: t.text, left: operand}
	default:
		return nil, fmt.Errorf("unexpected %q", ipTokenText(*t))
	}
	for {
		n := p.peek()
		if n == nil || n.kind != "op" {
			return left, nil
		}
		bp := ipBinding(n.text)
		if bp == 0 || bp <= minBP {
			return left, nil
		}
		p.pos++
		right, err := p.parse(bp)
		if err != nil {
			return nil, err
		}
		left = &ipExpr{kind: kBinary, op: n.text, left: left, right: right}
	}
}

func ipTokenText(t ipToken) string {
	switch t.kind {
	case "num":
		return strconv.FormatFloat(t.num, 'g', -1, 64)
	case "lparen":
		return "("
	case "rparen":
		return ")"
	}
	return t.text
}

// parseIPExpr compiles a screener expression.
func parseIPExpr(src string) (*ipExpr, error) {
	if strings.TrimSpace(src) == "" {
		return nil, fmt.Errorf("expression is empty")
	}
	toks, err := ipTokenize(src)
	if err != nil {
		return nil, err
	}
	p := &ipParser{toks: toks}
	e, err := p.parse(0)
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t != nil {
		return nil, fmt.Errorf("unexpected %q after the expression", ipTokenText(*t))
	}
	return e, nil
}

// fields returns the distinct field names the expression reads.
func (e *ipExpr) fields() []string {
	seen := map[string]bool{}
	var walk func(*ipExpr)
	walk = func(n *ipExpr) {
		if n == nil {
			return
		}
		if n.kind == kField {
			seen[n.str] = true
		}
		walk(n.left)
		walk(n.right)
	}
	walk(e)
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// validate reports an unknown field with the list of valid ones.
func (e *ipExpr) validate(valid map[string]bool) error {
	for _, f := range e.fields() {
		if !valid[f] {
			names := make([]string, 0, len(valid))
			for k := range valid {
				names = append(names, k)
			}
			sort.Strings(names)
			return fmt.Errorf("unknown field %q; valid fields: %s", f, strings.Join(names, ", "))
		}
	}
	return nil
}

// ipValue is float64, string, bool or nil (missing).
type ipValue any

func (e *ipExpr) eval(row map[string]any) ipValue {
	switch e.kind {
	case kNum:
		return e.num
	case kStr:
		return e.str
	case kField:
		v, ok := row[e.str]
		if !ok {
			return nil
		}
		switch t := v.(type) {
		case float64:
			return t
		case int:
			return float64(t)
		case string:
			return t
		case bool:
			return t
		}
		return nil
	case kUnary:
		v := e.left.eval(row)
		if e.op == "-" {
			if f, ok := v.(float64); ok {
				return -f
			}
			return nil
		}
		if v == nil {
			return nil
		}
		return !ipTruthy(v)
	}
	// binary
	switch e.op {
	case "and":
		return ipTruthy(e.left.eval(row)) && ipTruthy(e.right.eval(row))
	case "or":
		return ipTruthy(e.left.eval(row)) || ipTruthy(e.right.eval(row))
	}
	l, r := e.left.eval(row), e.right.eval(row)
	switch e.op {
	case "+", "-", "*", "/":
		lf, lok := l.(float64)
		rf, rok := r.(float64)
		if !lok || !rok {
			return nil
		}
		switch e.op {
		case "+":
			return lf + rf
		case "-":
			return lf - rf
		case "*":
			return lf * rf
		default:
			if rf == 0 {
				return nil
			}
			return lf / rf
		}
	}
	if l == nil || r == nil {
		return nil // unknown: stays false even under "not"
	}
	if lb, ok := l.(bool); ok {
		rb, ok := r.(bool)
		if !ok {
			return false
		}
		switch e.op {
		case "==":
			return lb == rb
		case "!=":
			return lb != rb
		}
		return false
	}
	if lf, ok := l.(float64); ok {
		rf, ok := r.(float64)
		if !ok || math.IsNaN(lf) || math.IsNaN(rf) {
			return false
		}
		return ipCompareFloat(e.op, lf, rf)
	}
	ls, lok := l.(string)
	rs, rok := r.(string)
	if !lok || !rok {
		return false
	}
	return ipCompareString(e.op, strings.ToLower(ls), strings.ToLower(rs))
}

func ipCompareFloat(op string, a, b float64) bool {
	switch op {
	case ">":
		return a > b
	case ">=":
		return a >= b
	case "<":
		return a < b
	case "<=":
		return a <= b
	case "==":
		return a == b
	case "!=":
		return a != b
	}
	return false
}

func ipCompareString(op string, a, b string) bool {
	switch op {
	case ">":
		return a > b
	case ">=":
		return a >= b
	case "<":
		return a < b
	case "<=":
		return a <= b
	case "==":
		return a == b
	case "!=":
		return a != b
	}
	return false
}

func ipTruthy(v ipValue) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	}
	return false
}

// matches evaluates the expression as a boolean filter for a row.
func (e *ipExpr) matches(row map[string]any) bool { return ipTruthy(e.eval(row)) }
