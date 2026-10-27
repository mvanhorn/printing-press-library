package main

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strings"
)

// Requiring a conjunct prevents a guard hidden behind || from certifying every
// branch. Review each retry boundary, never just the presence of a helper name.
func hasRetryConjunct(expr ast.Expr, want string, source func(ast.Node) string) bool {
	if source(expr) == want {
		return true
	}
	if paren, ok := expr.(*ast.ParenExpr); ok {
		return hasRetryConjunct(paren.X, want, source)
	}
	if binary, ok := expr.(*ast.BinaryExpr); ok && binary.Op == token.LAND {
		return hasRetryConjunct(binary.X, want, source) || hasRetryConjunct(binary.Y, want, source)
	}
	return false
}

// These predicates have narrower, reviewed contracts. The filename and helper
// contract must both match. Tests also pin their source definitions: a future
// widening must receive another review rather than silently inherit approval.
func reviewedRetryGuard(path, function string, data []byte) string {
	var guard, expected string
	var prefixes []string
	switch {
	case strings.HasSuffix(path, "/commerce/fedex/internal/client/client.go") && function == "send":
		guard, expected = "canRetry", "941dda8e9ffbc912aabe4c6b6ec8930971e55567abeff93307401615ec6b06d7"
		prefixes = []string{"var retryableReadOnlyPosts =", "func canRetryAmbiguousFailure("}
		if !strings.Contains(string(data), "canRetry := canRetryAmbiguousFailure(method, path)") {
			panic("FedEx retry binding changed; manual safety review required")
		}
	case strings.HasSuffix(path, "/project-management/paperclip-self-hosted/internal/client/client.go") && function == "doInternal":
		guard, expected = "requestCanRetry(method, readOnlyIntent)", "b08bd9972a5a63fed4c938021c20ac07efd4a7d3ac0c1ebf12b41dd18aa17ee1"
		prefixes = []string{"func requestCanRetry("}
	case strings.HasSuffix(path, "/marketing/dataforseo/internal/client/client.go") && function == "do" && !strings.Contains(string(data), "return req.Header.Get(\"Idempotency-Key\") != \"\""):
		guard, expected = "requestCanRetry(req)", "39dd6fd3b7d33f48052ce7059efc74df4a9223454d6e94f59253eb14d6d80bf2"
		prefixes = []string{"func requestCanRetry("}
	default:
		return ""
	}
	var policy []byte
	for _, prefix := range prefixes {
		policy = append(policy, regexp.MustCompile(`(?ms)^`+regexp.QuoteMeta(prefix)+`.*?^}\n`).Find(data)...)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(policy)); got != expected {
		panic(fmt.Sprintf("%s: reviewed retry policy changed (%s); manual safety review required", path, got))
	}
	return guard
}

func hasRetryNode(expr ast.Expr, want string, source func(ast.Node) string) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if n != nil && source(n) == want {
			found = true
		}
		return !found
	})
	return found
}

func hasRetryDisjunct(expr ast.Expr, want string, source func(ast.Node) string) bool {
	if source(expr) == want {
		return true
	}
	if paren, ok := expr.(*ast.ParenExpr); ok {
		return hasRetryDisjunct(paren.X, want, source)
	}
	if binary, ok := expr.(*ast.BinaryExpr); ok && binary.Op == token.LOR {
		return hasRetryDisjunct(binary.X, want, source) || hasRetryDisjunct(binary.Y, want, source)
	}
	return false
}

func transportStopsUnsafeWrite(body *ast.BlockStmt, approved string, source func(ast.Node) string) bool {
	for _, stmt := range body.List {
		branch, ok := stmt.(*ast.IfStmt)
		if !ok || len(branch.Body.List) == 0 {
			continue
		}
		if _, returns := branch.Body.List[len(branch.Body.List)-1].(*ast.ReturnStmt); !returns {
			continue
		}
		if hasRetryDisjunct(branch.Cond, "!canRetryAmbiguousFailure", source) || (approved != "" && hasRetryDisjunct(branch.Cond, "!"+approved, source)) {
			return true
		}
	}
	// Some current printers put the continue inside a positive safety gate
	// and return the error afterward instead of returning early on !safe.
	if len(body.List) == 0 {
		return false
	}
	if _, returns := body.List[len(body.List)-1].(*ast.ReturnStmt); !returns {
		return false
	}
	var guarded []*ast.BlockStmt
	ast.Inspect(body, func(n ast.Node) bool {
		if branch, ok := n.(*ast.IfStmt); ok && (hasRetryConjunct(branch.Cond, "canRetryAmbiguousFailure", source) || (approved != "" && hasRetryConjunct(branch.Cond, approved, source))) {
			guarded = append(guarded, branch.Body)
		}
		return true
	})
	allGuarded := true
	ast.Inspect(body, func(n ast.Node) bool {
		if branch, ok := n.(*ast.BranchStmt); ok && branch.Tok == token.CONTINUE {
			safe := false
			for _, block := range guarded {
				if block.Pos() < branch.Pos() && branch.End() < block.End() {
					safe = true
				}
			}
			if !safe {
				allGuarded = false
			}
		}
		return true
	})
	return allGuarded
}
