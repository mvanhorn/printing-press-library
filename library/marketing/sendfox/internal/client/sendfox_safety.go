package client

import (
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/contract"
	"net/url"
	"strings"
)

type sendfoxApprovalKey struct{}
type sendfoxApproval struct{ Write, Sensitive bool }

// WithMutationApproval is per invocation, never persisted or shared between MCP requests.
func WithMutationApproval(ctx context.Context, write, sensitive bool) context.Context {
	return context.WithValue(ctx, sendfoxApprovalKey{}, sendfoxApproval{write, sensitive})
}
func sendfoxPrepare(ctx context.Context, method, path string, params map[string]string, body any, dryRun bool) (any, error) {
	o, pp := contract.Match(method, path)
	if o == nil {
		return body, nil
	}
	m, _ := body.(map[string]any)
	if m != nil {
		copyBody := map[string]any{}
		for k, v := range m {
			copyBody[k] = v
		}
		m = copyBody
		body = m
		if method == "POST" && path == "/automations" {
			if _, ok := m["active"]; !ok {
				m["active"] = false
			}
		}
		if method == "POST" && path == "/contacts/bulk-actions" {
			if _, ok := m["dry_run"]; !ok {
				m["dry_run"] = true
			}
		}
	}
	q := map[string]string{}
	for k, v := range params {
		q[k] = v
	}
	if u, err := url.Parse(path); err == nil {
		for k, v := range u.Query() {
			if len(v) > 0 {
				q[k] = v[0]
			}
		}
	}
	if err := contract.ValidateRequest(o, pp, q, body); err != nil {
		return body, err
	}
	if !dryRun && method != "GET" && method != "HEAD" && method != "OPTIONS" {
		a, _ := ctx.Value(sendfoxApprovalKey{}).(sendfoxApproval)
		if !a.Write {
			return body, fmt.Errorf("mutation requires explicit approval: preview with --dry-run, then pass --yes (MCP: confirm=true); no request sent")
		}
		if contract.Sensitive(method, strings.SplitN(path, "?", 2)[0], body) && !a.Sensitive {
			return body, fmt.Errorf("send, scheduling, activation, form/domain or destructive action requires --approve-sensitive as well as --yes (MCP: approve_sensitive=true); no request sent")
		}
	}
	return body, nil
}
