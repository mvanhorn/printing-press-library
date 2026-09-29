// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package appscrape

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/client"
)

type AppPagePayload struct {
	Flows    []map[string]any
	Screens  []map[string]any
	Versions []map[string]any
	AppName  string
	Slug     string
}

var nextChunkRE = regexp.MustCompile(`self\.__next_f\.push\(\[1,\s*"((?:\\.|[^"\\])*)"\]\)`)

func Fetch(ctx context.Context, c *client.Client, slug string) (*AppPagePayload, error) {
	raw, err := c.Get(ctx, "https://mobbin.com/apps/"+slug+"/screens", nil)
	if err != nil {
		return nil, err
	}
	return Parse(string(raw), slug)
}

// Parse extracts flows, screens, and app versions from a Mobbin app page.
// Current pages carry them as React Server Component props
// ({appSlug, appInfo, appVersionId, screens, partialFlows}); older pages used
// a [{"value":[flows]},{"value":[screens]}] array, which is still accepted.
func Parse(html, slug string) (*AppPagePayload, error) {
	stream := flightStream(html)
	out, err := parseAppPageProps(stream, slug)
	if err != nil {
		legacy, legacyErr := parseLegacyValuePayload(stream, slug)
		if legacyErr != nil {
			if errors.Is(err, errNoAppProps) {
				return nil, legacyErr
			}
			return nil, err
		}
		out = legacy
	}
	if out.AppName == "" {
		out.AppName = findString(out.Screens, "appName", "app_name")
	}
	if out.AppName == "" {
		out.AppName = findString(out.Flows, "appName", "app_name")
	}
	if out.Flows == nil {
		out.Flows = []map[string]any{}
	}
	if out.Screens == nil {
		out.Screens = []map[string]any{}
	}
	if out.Versions == nil {
		out.Versions = []map[string]any{}
	}
	return out, nil
}

// flightStream joins the self.__next_f.push([1,"..."]) chunks into the raw
// React Server Component stream. Falls back to the HTML when none are found.
func flightStream(html string) string {
	var b strings.Builder
	for _, m := range nextChunkRE.FindAllStringSubmatch(html, -1) {
		var s string
		if err := json.Unmarshal([]byte(`"`+m[1]+`"`), &s); err == nil {
			b.WriteString(s)
		}
	}
	if b.Len() == 0 {
		return html
	}
	return b.String()
}

var errNoAppProps = errors.New("no app page props in RSC stream")

// parseAppPageProps returns errNoAppProps when the page has no props object,
// and a descriptive error when props exist but screens or partialFlows do not
// decode to arrays (for example a "$<row>:path" reference that points
// nowhere). Only literal arrays count as data; an empty array is a valid
// empty app, a missing one is a scrape failure.
func parseAppPageProps(stream, slug string) (*AppPagePayload, error) {
	fl := newFlightRows(stream)
	var empty *AppPagePayload
	var broken error
	for _, id := range fl.order {
		body := fl.raw[id]
		if !strings.Contains(body, `"partialFlows"`) {
			continue
		}
		props, ok := findAppProps(fl.parsed(id))
		if !ok {
			continue
		}
		screensV, screensOK := fl.decode(props["screens"], 0).([]any)
		flowsV, flowsOK := fl.decode(props["partialFlows"], 0).([]any)
		if !screensOK || !flowsOK {
			if broken == nil {
				broken = fmt.Errorf("app page props in RSC row %s have unresolvable screens (%v) or partialFlows (%v)", id, describeRef(props["screens"]), describeRef(props["partialFlows"]))
			}
			continue
		}
		screens, flows := toRows(screensV), toRows(flowsV)
		out := &AppPagePayload{Slug: slug, Screens: screens, Flows: flows}
		if len(screens) == 0 && len(flows) == 0 {
			// Keep looking for a copy that carries data, but an app page
			// with genuinely empty lists should report empty, not fail.
			if empty == nil {
				empty = out
				fillAppInfo(fl, props, out)
			}
			continue
		}
		if versionID, _ := fl.decode(props["appVersionId"], 0).(string); versionID != "" {
			for _, rows := range [][]map[string]any{out.Screens, out.Flows} {
				for _, r := range rows {
					if _, ok := r["appVersionId"]; !ok {
						r["appVersionId"] = versionID
					}
				}
			}
		}
		fillAppInfo(fl, props, out)
		return out, nil
	}
	if empty != nil {
		return empty, nil
	}
	if broken != nil {
		return nil, broken
	}
	return nil, errNoAppProps
}

// describeRef names a props value for error messages without dumping data.
func describeRef(v any) string {
	switch n := v.(type) {
	case string:
		return fmt.Sprintf("%q", n)
	case nil:
		return "null"
	case []any:
		return "array"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func fillAppInfo(fl *flightRows, props map[string]any, out *AppPagePayload) {
	if info, ok := fl.decode(props["appInfo"], 0).(map[string]any); ok {
		out.AppName, _ = info["appName"].(string)
		out.Versions = toRows(info["appVersions"])
	}
}

// findAppProps walks a decoded RSC row for the app page's client component
// props: the object that holds partialFlows next to screens.
func findAppProps(node any) (map[string]any, bool) {
	switch n := node.(type) {
	case map[string]any:
		if _, ok := n["partialFlows"]; ok {
			if _, ok := n["screens"]; ok {
				return n, true
			}
		}
		for _, v := range n {
			if p, ok := findAppProps(v); ok {
				return p, true
			}
		}
	case []any:
		for _, v := range n {
			if p, ok := findAppProps(v); ok {
				return p, true
			}
		}
	}
	return nil, false
}

// flightRows indexes an RSC stream by row id ("<hex>:<json>\n" or
// "<hex>:T<hexlen>,<text>") and decodes rows lazily.
type flightRows struct {
	raw   map[string]string
	order []string
	cache map[string]any
}

func newFlightRows(stream string) *flightRows {
	fl := &flightRows{raw: map[string]string{}, cache: map[string]any{}}
	pos := 0
	for pos < len(stream) {
		colon := strings.IndexByte(stream[pos:], ':')
		if colon < 0 {
			break
		}
		id := stream[pos : pos+colon]
		start := pos + colon + 1
		if !isFlightRowID(id) {
			nl := strings.IndexByte(stream[pos:], '\n')
			if nl < 0 {
				break
			}
			pos += nl + 1
			continue
		}
		if start < len(stream) && stream[start] == 'T' {
			comma := strings.IndexByte(stream[start:], ',')
			if comma > 0 {
				if n, err := strconv.ParseInt(stream[start+1:start+comma], 16, 64); err == nil {
					textStart := start + comma + 1
					end := textStart + int(n)
					if end > len(stream) {
						end = len(stream)
					}
					fl.add(id, stream[textStart:end])
					pos = end
					continue
				}
			}
		}
		end := len(stream)
		if nl := strings.IndexByte(stream[start:], '\n'); nl >= 0 {
			end = start + nl
		}
		fl.add(id, stream[start:end])
		pos = end + 1
	}
	return fl
}

func (fl *flightRows) add(id, body string) {
	if _, seen := fl.raw[id]; !seen {
		fl.order = append(fl.order, id)
	}
	fl.raw[id] = body
}

func isFlightRowID(s string) bool {
	if s == "" || len(s) > 8 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func (fl *flightRows) parsed(id string) any {
	if v, ok := fl.cache[id]; ok {
		return v
	}
	var v any
	if err := json.Unmarshal([]byte(fl.raw[id]), &v); err != nil {
		v = nil
	}
	fl.cache[id] = v
	return v
}

const (
	maxFlightDepth = 64
	maxFlightHops  = 16
)

// decode returns a copy of v with RSC string encodings resolved:
// "$<row>[:path]" references are followed, "$undefined" becomes nil, and
// "$$..." is unescaped to a literal "$...". Other "$" markers (lazy
// components, dates, etc.) are left as-is.
func (fl *flightRows) decode(v any, depth int) any {
	if depth > maxFlightDepth {
		return nil
	}
	switch n := v.(type) {
	case string:
		if !strings.HasPrefix(n, "$") {
			return n
		}
		if n == "$undefined" {
			return nil
		}
		if strings.HasPrefix(n, "$$") {
			return n[1:]
		}
		if target, ok := fl.resolveRef(n, 0); ok {
			return fl.decode(target, depth+1)
		}
		return n
	case map[string]any:
		out := make(map[string]any, len(n))
		for k, val := range n {
			out[k] = fl.decode(val, depth+1)
		}
		return out
	case []any:
		out := make([]any, len(n))
		for i, val := range n {
			out[i] = fl.decode(val, depth+1)
		}
		return out
	default:
		return v
	}
}

// resolveRef follows a "$<row>:seg:seg" reference. A "props" segment on a
// React element tuple ["$", type, key, props] selects the props object.
func (fl *flightRows) resolveRef(ref string, hops int) (any, bool) {
	if hops > maxFlightHops {
		return nil, false
	}
	parts := strings.Split(ref[1:], ":")
	if !isFlightRowID(parts[0]) {
		return nil, false
	}
	if _, ok := fl.raw[parts[0]]; !ok {
		return nil, false
	}
	node := fl.parsed(parts[0])
	for _, seg := range parts[1:] {
		if s, ok := node.(string); ok && strings.HasPrefix(s, "$") && !strings.HasPrefix(s, "$$") {
			next, ok := fl.resolveRef(s, hops+1)
			if !ok {
				return nil, false
			}
			node = next
		}
		switch n := node.(type) {
		case map[string]any:
			v, ok := n[seg]
			if !ok {
				return nil, false
			}
			node = v
		case []any:
			if len(n) == 4 && n[0] == "$" {
				switch seg {
				case "type":
					node = n[1]
					continue
				case "key":
					node = n[2]
					continue
				case "props":
					node = n[3]
					continue
				}
			}
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(n) {
				return nil, false
			}
			node = n[i]
		default:
			return nil, false
		}
	}
	return node, true
}

func toRows(v any) []map[string]any {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	rows := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			rows = append(rows, m)
		}
	}
	return rows
}

func parseLegacyValuePayload(stream, slug string) (*AppPagePayload, error) {
	arr, err := extractPayloadArray(stream)
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal([]byte(arr), &payload); err != nil {
		return nil, fmt.Errorf("parsing app payload: %w", err)
	}
	out := &AppPagePayload{Slug: slug}
	if len(payload) > 0 {
		out.Flows = rawArray(payload[0].Value)
	}
	if len(payload) > 1 {
		out.Screens = rawArray(payload[1].Value)
	}
	return out, nil
}

func rawArray(raw json.RawMessage) []map[string]any {
	var rows []map[string]any
	_ = json.Unmarshal(raw, &rows)
	return rows
}

func extractPayloadArray(s string) (string, error) {
	idx := strings.Index(s, `[{"value":[`)
	if idx < 0 {
		idx = strings.Index(s, `[{"value":`)
	}
	if idx < 0 {
		idx = strings.Index(s, `[{"value"`)
	}
	if idx < 0 {
		return "", fmt.Errorf("could not find app flows/screens data in Mobbin app page (no partialFlows props or legacy value payload)")
	}
	depth := 0
	inStr := false
	esc := false
	for i := idx; i < len(s); i++ {
		ch := s[i]
		if inStr {
			if esc {
				esc = false
			} else if ch == '\\' {
				esc = true
			} else if ch == '"' {
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '[', '{':
			depth++
		case ']', '}':
			depth--
			if depth == 0 {
				// Flight chunks are already unescaped before scanning.
				return s[idx : i+1], nil
			}
		}
	}
	return "", fmt.Errorf("unterminated app payload")
}

func findString(rows []map[string]any, keys ...string) string {
	for _, r := range rows {
		for _, k := range keys {
			if s, ok := r[k].(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}
