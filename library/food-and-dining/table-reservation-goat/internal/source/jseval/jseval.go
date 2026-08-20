// Package jseval extracts JavaScript object literals from inline <script>
// blocks and converts them to JSON-parseable bytes. Both OpenTable and Tock
// SSR-render their store state as JS object literals (not pure JSON), with
// `undefined`, function expressions, regex literals, NaN, and Infinity
// scattered through. Pure JSON parsing rejects all of these; balanced-brace +
// regex strip handles the easy cases but misses functions and bare globals.
//
// We use a real JS evaluator (goja, pure Go) to evaluate the assignment, then
// serialize the captured object back via JSON.stringify. JSON.stringify drops
// undefined values, replaces NaN/Infinity with null, and turns functions into
// undefined (then dropped). The result is a clean JSON byte slice the caller
// can json.Unmarshal directly.
package jseval

// PATCH: cross-network-source-clients — see .printing-press-patches.json for the change-set rationale.

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/dop251/goja"
)

// ExtractObjectLiteral locates the first JS object literal that appears after
// `lhsAnchor` in `body`, evaluates it inside a sandboxed goja runtime, and
// returns the result as JSON. The runtime exposes `window`, `document`,
// `console`, and `performance` as no-op stubs — enough that defensive
// feature-checks inside SSR-rendered state objects don't throw, but not so
// much that arbitrary scripts could do useful work.
//
// Returns an error if the anchor is not present, the literal is malformed,
// or evaluation fails (typically because the literal references a runtime
// API the sandbox doesn't expose).
func ExtractObjectLiteral(body []byte, lhsAnchor *regexp.Regexp) ([]byte, error) {
	source, err := isolateLiteral(body, lhsAnchor)
	if err != nil {
		return nil, err
	}
	rt := newSandboxRuntime()
	prog := []byte("var __pp_capture__ = ")
	prog = append(prog, source...)
	prog = append(prog, ';')
	if _, err := rt.RunString(string(prog)); err != nil {
		return nil, fmt.Errorf("evaluating isolated literal: %w", err)
	}
	val := rt.Get("__pp_capture__")
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return nil, errors.New("isolated literal evaluated to undefined/null")
	}
	stringify, ok := goja.AssertFunction(rt.Get("JSON").ToObject(rt).Get("stringify"))
	if !ok {
		return nil, errors.New("JSON.stringify unavailable in goja runtime")
	}
	res, err := stringify(goja.Undefined(), val)
	if err != nil {
		return nil, fmt.Errorf("JSON.stringify: %w", err)
	}
	if res == nil || goja.IsUndefined(res) || goja.IsNull(res) {
		return nil, errors.New("isolated literal serialized to undefined/null")
	}
	return []byte(res.String()), nil
}

// isolateLiteral walks bytes after `lhsAnchor` to find the first balanced
// `{...}` literal. The walker is string-aware (handles `"`, `'`, and template
// backticks) and escape-aware. Returns the literal bytes (including the outer
// braces) or an error.
func isolateLiteral(body []byte, lhsAnchor *regexp.Regexp) ([]byte, error) {
	loc := lhsAnchor.FindIndex(body)
	if loc == nil {
		return nil, errors.New("anchor not found in body")
	}
	i := loc[1]
	for i < len(body) && body[i] != '{' {
		i++
	}
	if i >= len(body) {
		return nil, errors.New("no JSON body after anchor")
	}
	depth := 0
	inString := false
	escape := false
	stringQuote := byte(0)
	end := -1
	for j := i; j < len(body); j++ {
		ch := body[j]
		if escape {
			escape = false
			continue
		}
		if ch == '\\' {
			escape = true
			continue
		}
		if inString {
			if ch == stringQuote {
				inString = false
			}
			continue
		}
		switch ch {
		case '"', '\'', '`':
			inString = true
			stringQuote = ch
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = j + 1
			}
		}
		if end != -1 {
			break
		}
	}
	if end == -1 {
		return nil, errors.New("unbalanced braces after anchor")
	}
	return body[i:end], nil
}

// newSandboxRuntime constructs a goja runtime with the minimum surface that
// SSR-rendered state assignments tend to need: a `window` global aliased to
// the goja globalThis, no-op `document` and `console` stubs for defensive
// feature-checks, and a `performance.now()` stub.
//
// Any reference to fetch, XHR, navigator, or location beyond these stubs will
// throw — that's intentional: we want the failure to surface rather than
// silently allow third-party-style script behavior.
func newSandboxRuntime() *goja.Runtime {
	rt := goja.New()
	_ = rt.Set("window", rt.GlobalObject())
	doc := rt.NewObject()
	_ = doc.Set("querySelector", func(call goja.FunctionCall) goja.Value { return goja.Null() })
	_ = doc.Set("getElementById", func(call goja.FunctionCall) goja.Value { return goja.Null() })
	_ = rt.Set("document", doc)
	console := rt.NewObject()
	for _, m := range []string{"log", "warn", "error", "info", "debug"} {
		_ = console.Set(m, func(call goja.FunctionCall) goja.Value { return goja.Undefined() })
	}
	_ = rt.Set("console", console)
	perf := rt.NewObject()
	_ = perf.Set("now", func(call goja.FunctionCall) goja.Value { return rt.ToValue(0) })
	_ = rt.Set("performance", perf)
	return rt
}
