package client

import (
	"net/url"
	"strings"
)

// PATCH(loops-dry-run-shows-full-redacted-endpoint): dry runs show the full
// route so subresources such as /metrics, /publish, or /guardian stay visible.
// Redaction is positional: the request path is matched against the Loops route
// templates and every {param} position becomes ":id" whatever its value, so a
// private value that happens to equal a route word (an event named "publish")
// is still hidden. Paths that match no template fail closed.
var loopsRouteTemplates = []string{
	"/v1/api-key",
	"/v1/audience-segments",
	"/v1/audience-segments/{audienceSegmentId}",
	"/v1/campaign-groups",
	"/v1/campaign-groups/{campaignGroupId}",
	"/v1/campaigns",
	"/v1/campaigns/{campaignId}",
	"/v1/campaigns/{campaignId}/metrics",
	"/v1/components",
	"/v1/components/{componentId}",
	"/v1/contacts/create",
	"/v1/contacts/delete",
	"/v1/contacts/find",
	"/v1/contacts/properties",
	"/v1/contacts/suppression",
	"/v1/contacts/update",
	"/v1/dedicated-sending-ips",
	"/v1/email-messages/{emailMessageId}",
	"/v1/email-messages/{emailMessageId}/guardian",
	"/v1/email-messages/{emailMessageId}/preview",
	"/v1/event-patterns",
	"/v1/event-patterns/by-name/{eventName}",
	"/v1/event-patterns/{eventPatternId}",
	"/v1/events/send",
	"/v1/lists",
	"/v1/themes",
	"/v1/themes/{themeId}",
	"/v1/transactional",
	"/v1/transactional-emails",
	"/v1/transactional-emails/{transactionalId}",
	"/v1/transactional-emails/{transactionalId}/draft",
	"/v1/transactional-emails/{transactionalId}/metrics",
	"/v1/transactional-emails/{transactionalId}/publish",
	"/v1/transactional-groups",
	"/v1/transactional-groups/{transactionalGroupId}",
	"/v1/uploads",
	"/v1/uploads/{emailAssetId}/complete",
	"/v1/workflows",
	"/v1/workflows/{workflowId}",
	"/v1/workflows/{workflowId}/mailing-list",
	"/v1/workflows/{workflowId}/nodes",
	"/v1/workflows/{workflowId}/nodes/{nodeId}",
	"/v1/workflows/{workflowId}/nodes/{nodeId}/add-branch",
	"/v1/workflows/{workflowId}/nodes/{nodeId}/metrics",
	"/v1/workflows/{workflowId}/nodes/{nodeId}/recursive",
	"/v1/workflows/{workflowId}/nodes/{nodeId}/reroute",
}

func splitRoute(path string) []string {
	return strings.Split(strings.Trim(path, "/"), "/")
}

func isRouteParam(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

// matchLoopsRoute returns the redacted segments for the matching template with
// the most literal segments, or false when no template matches.
func matchLoopsRoute(segments []string) ([]string, bool) {
	var best []string
	bestLiterals := -1
	for _, template := range loopsRouteTemplates {
		parts := splitRoute(template)
		if len(parts) != len(segments) {
			continue
		}
		literals := 0
		matched := true
		for index, part := range parts {
			if isRouteParam(part) {
				continue
			}
			if part != segments[index] {
				matched = false
				break
			}
			literals++
		}
		if !matched || literals <= bestLiterals {
			continue
		}
		bestLiterals = literals
		best = make([]string, len(parts))
		for index, part := range parts {
			if isRouteParam(part) {
				best[index] = ":id"
			} else {
				best[index] = part
			}
		}
	}
	return best, best != nil
}

func loopsRouteLiteralAt(index int, segment string) bool {
	for _, template := range loopsRouteTemplates {
		parts := splitRoute(template)
		if index < len(parts) && !isRouteParam(parts[index]) && parts[index] == segment {
			return true
		}
	}
	return false
}

func safeDryRunEndpoint(method, path string) string {
	path = strings.TrimSpace(path)
	if parsed, err := url.Parse(path); err == nil {
		path = parsed.EscapedPath()
	}
	segments := splitRoute(path)
	if redacted, ok := matchLoopsRoute(segments); ok {
		return strings.ToUpper(method) + " /" + strings.Join(redacted, "/")
	}
	// Unknown route: keep at most the version and a known top-level resource
	// word, then a single ":id" for anything deeper.
	out := make([]string, 0, 3)
	for index, segment := range segments {
		if index >= 2 {
			out = append(out, ":id")
			break
		}
		if loopsRouteLiteralAt(index, segment) {
			out = append(out, segment)
		} else {
			out = append(out, ":id")
		}
	}
	return strings.ToUpper(method) + " /" + strings.Join(out, "/")
}
