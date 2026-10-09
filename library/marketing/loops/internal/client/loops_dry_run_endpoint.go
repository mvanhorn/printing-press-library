package client

import (
	"net/url"
	"strings"
)

// PATCH(loops-dry-run-full-endpoint): dry runs show the full endpoint path so
// subresources such as /metrics, /publish, or /guardian stay visible, while
// every path segment that is not a known literal Loops route word is replaced
// with ":id". The allowlist fails closed: an unknown segment is treated as a
// private value, never echoed.
var loopsStaticPathSegments = map[string]bool{
	"v1": true, "add-branch": true, "api-key": true, "audience-segments": true,
	"by-name": true, "campaign-groups": true, "campaigns": true, "complete": true,
	"components": true, "contacts": true, "create": true, "dedicated-sending-ips": true,
	"delete": true, "draft": true, "email-messages": true, "event-patterns": true,
	"events": true, "find": true, "guardian": true, "lists": true, "mailing-list": true,
	"metrics": true, "nodes": true, "preview": true, "properties": true, "publish": true,
	"recursive": true, "reroute": true, "send": true, "suppression": true, "themes": true,
	"transactional": true, "transactional-emails": true, "transactional-groups": true,
	"update": true, "uploads": true, "workflows": true,
}

func safeDryRunEndpoint(method, path string) string {
	path = strings.TrimSpace(path)
	if parsed, err := url.Parse(path); err == nil {
		path = parsed.EscapedPath()
	}
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for index, segment := range segments {
		if !loopsStaticPathSegments[segment] {
			segments[index] = ":id"
		}
	}
	return strings.ToUpper(method) + " /" + strings.Join(segments, "/")
}
