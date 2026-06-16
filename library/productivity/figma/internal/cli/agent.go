// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written read-only agent node discovery commands (see .printing-press-patches/agent-node-discovery.json).

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/figma/internal/client"
	"github.com/spf13/cobra"
)

// figmaRef is a parsed Figma reference: either a raw file key or a full
// Figma URL with an optional node id.
type figmaRef struct {
	FileKey string
	NodeID  string // normalized ("123:456"); empty when absent
	Raw     string
}

// parseFigmaRef accepts:
//   - raw file keys like "JZyB6K6Z22YyObBdj1r4v1"
//   - https://www.figma.com/design/<file_key>/<title>?node-id=123-456
//   - https://www.figma.com/file/<file_key>/<title>?node_id=123-456
//   - https://www.figma.com/proto/<file_key>/... (prototype links share the key shape)
//
// node-id / node_id query values are normalized via normalizeNodeID. A
// usage-style error is returned for URLs that do not contain a file key.
func parseFigmaRef(raw string) (figmaRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return figmaRef{}, fmt.Errorf("empty Figma URL or file key")
	}
	ref := figmaRef{Raw: raw}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err != nil {
			return figmaRef{}, fmt.Errorf("invalid Figma URL %q: %w", raw, err)
		}
		host := strings.ToLower(u.Hostname())
		host = strings.TrimPrefix(host, "www.")
		if host != "figma.com" {
			return figmaRef{}, fmt.Errorf("not a Figma URL (host %q)", u.Host)
		}

		// Path shape: /design/<key>/..., /file/<key>/..., /proto/<key>/..., /board/<key>/...
		key := fileKeyFromPath(u.Path)
		if key == "" {
			return figmaRef{}, fmt.Errorf("could not find a file key in Figma URL %q", raw)
		}
		ref.FileKey = key

		q := u.Query()
		var nodeRaw string
		for _, p := range []string{"node-id", "node_id"} {
			if v := q.Get(p); v != "" {
				nodeRaw = v
				break
			}
		}
		if nodeRaw != "" {
			ref.NodeID = normalizeNodeID(nodeRaw)
		}
		return ref, nil
	}

	// Raw file key form. Reject characters that indicate a malformed URL/path
	// rather than a key. Figma file keys are alphanumeric tokens without
	// slashes, query separators, fragments, or whitespace.
	if strings.ContainsAny(raw, "/?# \t\r\n") {
		return figmaRef{}, fmt.Errorf("invalid Figma file key %q", raw)
	}
	ref.FileKey = raw
	return ref, nil
}

// fileKeyFromPath extracts the file key from a Figma URL path. Supported
// prefixes: /design, /file, /proto, /board. Returns "" when no key is found.
func fileKeyFromPath(path string) string {
	segments := strings.Split(path, "/")
	// strings.Split on "/design/abc/Title" → ["", "design", "abc", "Title"]
	var clean []string
	for _, s := range segments {
		if s != "" {
			clean = append(clean, s)
		}
	}
	if len(clean) < 2 {
		return ""
	}
	switch strings.ToLower(clean[0]) {
	case "design", "file", "proto", "board":
		return clean[1]
	default:
		return ""
	}
}

// parseProjectTeamRef extracts a Figma project or team id from a Figma files URL.
func parseProjectTeamRef(raw string) (kind string, id string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("empty Figma project/team URL")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", "", fmt.Errorf("expected a Figma project or team URL")
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host != "figma.com" {
		return "", "", fmt.Errorf("not a Figma URL (host %q)", u.Host)
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i < len(segments)-1; i++ {
		switch strings.ToLower(segments[i]) {
		case "project":
			if segments[i+1] != "" {
				return "project", segments[i+1], nil
			}
		case "team":
			if segments[i+1] != "" {
				return "team", segments[i+1], nil
			}
		}
	}
	return "", "", fmt.Errorf("could not find a project or team id in Figma URL %q", raw)
}

func slugifyAlias(name string) string {
	alias := strings.ToLower(sanitizeFilename(name))
	alias = strings.ReplaceAll(alias, ".", "-")
	alias = strings.Trim(alias, "-")
	alias = regexp.MustCompile(`-+`).ReplaceAllString(alias, "-")
	if alias == "" {
		return "file"
	}
	return alias
}

type knownFile struct {
	FileKey      string   `json:"file_key"`
	Name         string   `json:"name"`
	URL          string   `json:"url"`
	LastModified string   `json:"last_modified,omitempty"`
	Project      string   `json:"project,omitempty"`
	Aliases      []string `json:"aliases"`
}

type figmaFileMeta struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	LastModified string `json:"last_modified"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
}

type figmaProjectMeta struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func buildKnownFilesEntries(files []figmaFileMeta, project string) map[string]knownFile {
	entries := make(map[string]knownFile, len(files))
	for _, f := range files {
		base := slugifyAlias(f.Name)
		alias := base
		for n := 2; alias == "" || entries[alias].FileKey != ""; n++ {
			if base == "" {
				base = "file"
			}
			alias = fmt.Sprintf("%s-%d", base, n)
		}
		aliases := dedupeStrings([]string{base, strings.ToLower(strings.TrimSpace(f.Name))})
		entry := knownFile{
			FileKey:      f.Key,
			Name:         f.Name,
			URL:          "https://www.figma.com/design/" + f.Key + "/" + url.PathEscape(f.Name),
			LastModified: f.LastModified,
			Project:      project,
			Aliases:      aliases,
		}
		entries[alias] = entry
	}
	return entries
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// agentNodeSummary is a compact, agent-friendly projection of a Figma node.
type agentNodeSummary struct {
	ID                  string         `json:"id"`
	Name                string         `json:"name"`
	Type                string         `json:"type"`
	Path                []string       `json:"path"`
	Label               string         `json:"label"`
	ParentID            string         `json:"parent_id,omitempty"`
	ChildCount          int            `json:"child_count"`
	AbsoluteBoundingBox map[string]any `json:"absolute_bounding_box,omitempty"`
}

// buildAgentNodeIndex walks a Figma document tree depth-first and returns
// compact summaries for every node whose type is in allowedTypes. The
// document root is walked but never itself indexed; its children (pages) are
// the first indexed level, so path labels start at the page name.
//
// maxDepth controls pruning of the in-memory tree: depth is counted as edges
// below the document root (pages = 1). maxDepth <= 0 means unlimited. The
// Figma API request depth is the primary tree-limiter; this guards against
// runaway traversal when the caller already pruned via the request.
func buildAgentNodeIndex(document map[string]any, allowedTypes map[string]bool, maxDepth int) []agentNodeSummary {
	out := []agentNodeSummary{}
	if document == nil {
		return out
	}
	for _, child := range getNodeChildren(document) {
		walkAgentNode(child, nil, 1, maxDepth, allowedTypes, &out)
	}
	return out
}

func walkAgentNode(node map[string]any, path []string, depth, maxDepth int, allowed map[string]bool, out *[]agentNodeSummary) {
	name, _ := node["name"].(string)
	nodeType, _ := node["type"].(string)
	id, _ := node["id"].(string)

	// Fresh slice copy so stored paths cannot be mutated by sibling recursion.
	currentPath := make([]string, len(path)+1)
	copy(currentPath, path)
	currentPath[len(path)] = name

	children := getNodeChildren(node)

	if nodeType != "DOCUMENT" && (allowed == nil || allowed[nodeType]) {
		summary := agentNodeSummary{
			ID:         id,
			Name:       name,
			Type:       nodeType,
			Path:       append([]string(nil), currentPath...),
			Label:      strings.Join(currentPath, " / "),
			ChildCount: len(children),
		}
		if pid, ok := node["parentId"].(string); ok && pid != "" {
			summary.ParentID = pid
		}
		if abb, ok := node["absoluteBoundingBox"].(map[string]any); ok {
			summary.AbsoluteBoundingBox = abb
		}
		*out = append(*out, summary)
	}

	// Descend unless the caller imposed a depth cap we just reached.
	if maxDepth > 0 && depth+1 > maxDepth {
		return
	}
	for _, child := range children {
		walkAgentNode(child, currentPath, depth+1, maxDepth, allowed, out)
	}
}

// getNodeChildren returns the node's children as typed maps (empty when absent).
func getNodeChildren(node map[string]any) []map[string]any {
	if node == nil {
		return nil
	}
	raw, ok := node["children"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, c := range raw {
		if cm, ok := c.(map[string]any); ok {
			out = append(out, cm)
		}
	}
	return out
}

// parseTypeSet parses a comma-separated node-type allowlist into a set of
// upper-cased type names. Returns nil for an empty input (meaning: include
// every non-document node).
func parseTypeSet(s string) map[string]bool {
	set := map[string]bool{}
	for _, p := range strings.Split(s, ",") {
		p = strings.ToUpper(strings.TrimSpace(p))
		if p != "" {
			set[p] = true
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

// scoreAgentNode returns a deterministic relevance score (higher is better)
// for a query against a node. Exact name wins, then exact label, then
// substring name/label, then token matches across label. Token matching
// splits both the query and the label into words and scores when every query
// word is found in some label word — this catches multi-word queries like
// "hero card" that are not a contiguous substring of "Hero Primary Card".
// Returns 0 for no match.
func scoreAgentNode(query string, node agentNodeSummary) int {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return 0
	}
	name := strings.ToLower(node.Name)
	label := strings.ToLower(node.Label)

	if name == q {
		return 100
	}
	if label == q {
		return 90
	}
	if name != "" && strings.Contains(name, q) {
		return 70
	}
	if label != "" && strings.Contains(label, q) {
		return 60
	}
	// Token matches across label: every query word is contained in some
	// label word. Reachable when the multi-word query is not contiguous.
	qTokens := strings.Fields(q)
	if len(qTokens) > 0 {
		labelTokens := strings.Fields(label)
		allMatch := true
		for _, qt := range qTokens {
			found := false
			for _, lt := range labelTokens {
				if strings.Contains(lt, qt) {
					found = true
					break
				}
			}
			if !found {
				allMatch = false
				break
			}
		}
		if allMatch {
			return 40
		}
	}
	return 0
}

func sanitizeFilename(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.TrimSpace(s) {
		ok := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if !ok {
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
			continue
		}
		b.WriteRune(r)
		lastDash = r == '-'
	}
	out := strings.Trim(b.String(), "-.")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	if len(out) > 60 {
		out = strings.Trim(out[:60], "-.")
	}
	if out == "" {
		return "node"
	}
	return strings.ToLower(out)
}

var (
	numericNodeNameRE = regexp.MustCompile(`^\d+$`)
	groupNodeNameRE   = regexp.MustCompile(`^group \d+$`)
	frameNodeNameRE   = regexp.MustCompile(`^frame \d+$`)
)

func isLikelyScreenNode(n agentNodeSummary) bool {
	switch strings.ToUpper(strings.TrimSpace(n.Type)) {
	case "GROUP", "VECTOR", "BOOLEAN_OPERATION":
		return false
	}
	name := strings.ToLower(strings.TrimSpace(n.Name))
	if numericNodeNameRE.MatchString(name) || groupNodeNameRE.MatchString(name) || frameNodeNameRE.MatchString(name) {
		return false
	}
	stop := map[string]bool{
		"status bar": true, "home indicator": true, "scrims": true, "wordmark": true,
		"content container": true, "bottom nav (pain)": true, "button groups": true, "top": true,
	}
	return !stop[name]
}

func isRenderTimeoutError(err error) bool {
	var apiErr *client.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(apiErr.Body), "render timeout")
}

func isRenderTimeoutMessage(message string) bool {
	return strings.Contains(strings.ToLower(message), "render timeout")
}

type renderImageResult struct {
	Images map[string]*string
	Errors map[string]string
}

// renderImages renders ids via /v1/images in batches, splitting a batch and
// lowering scale once for single-id timeout failures. Render timeouts are
// surfaced as nil URLs for unresolved ids; other errors are returned.
func renderImages(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, fileKey, format string, scale float64, ids []string, batch int) (map[string]*string, error) {
	result, err := renderImagesDetailed(c, fileKey, format, scale, ids, batch)
	if err != nil {
		return nil, err
	}
	return result.Images, nil
}

func renderImagesDetailed(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, fileKey, format string, scale float64, ids []string, batch int) (renderImageResult, error) {
	if batch <= 0 {
		batch = 1
	}
	result := renderImageResult{
		Images: make(map[string]*string, len(ids)),
		Errors: make(map[string]string),
	}
	var renderChunk func([]string, float64, bool) error
	renderChunk = func(chunk []string, chunkScale float64, downgraded bool) error {
		if len(chunk) == 0 {
			return nil
		}
		raw, err := c.Get("/v1/images/"+fileKey, map[string]string{"ids": strings.Join(chunk, ","), "format": format, "scale": fmt.Sprintf("%g", chunkScale)})
		if err != nil {
			if isRenderTimeoutError(err) {
				if len(chunk) > 1 {
					mid := len(chunk) / 2
					if err := renderChunk(chunk[:mid], chunkScale, downgraded); err != nil {
						return err
					}
					return renderChunk(chunk[mid:], chunkScale, downgraded)
				}
				if !downgraded && chunkScale > 1 {
					return renderChunk(chunk, 1, true)
				}
				result.Images[chunk[0]] = nil
				result.Errors[chunk[0]] = "Render timeout, try requesting fewer or smaller images"
				return nil
			}
			return err
		}
		var render struct {
			Err    *string            `json:"err"`
			Images map[string]*string `json:"images"`
		}
		if err := json.Unmarshal(raw, &render); err != nil {
			return fmt.Errorf("decoding images response: %w", err)
		}
		if render.Err != nil && isRenderTimeoutMessage(*render.Err) {
			if len(chunk) > 1 {
				mid := len(chunk) / 2
				if err := renderChunk(chunk[:mid], chunkScale, downgraded); err != nil {
					return err
				}
				return renderChunk(chunk[mid:], chunkScale, downgraded)
			}
			if !downgraded && chunkScale > 1 {
				return renderChunk(chunk, 1, true)
			}
			result.Images[chunk[0]] = nil
			result.Errors[chunk[0]] = *render.Err
			return nil
		}
		for id, url := range render.Images {
			result.Images[id] = url
		}
		if render.Err != nil && *render.Err != "" {
			for _, id := range chunk {
				if url := result.Images[id]; url == nil || *url == "" {
					result.Errors[id] = *render.Err
				}
			}
		}
		return nil
	}
	for start := 0; start < len(ids); start += batch {
		end := start + batch
		if end > len(ids) {
			end = len(ids)
		}
		if err := renderChunk(ids[start:end], scale, false); err != nil {
			return renderImageResult{}, err
		}
	}
	return result, nil
}

func downloadToFile(httpClient *http.Client, srcURL, destPath string) (int64, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := httpClient.Get(srcURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("download failed: HTTP %s", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return 0, err
	}
	tmp := destPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	const maxDownloadBytes = int64(40 << 20)
	n, copyErr := io.Copy(f, io.LimitReader(resp.Body, maxDownloadBytes+1))
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return n, copyErr
	}
	if n > maxDownloadBytes {
		_ = os.Remove(tmp)
		return 0, fmt.Errorf("render exceeded %d MiB cap", maxDownloadBytes>>20)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return n, closeErr
	}
	if err := os.Rename(tmp, destPath); err != nil {
		_ = os.Remove(tmp)
		return n, err
	}
	return n, nil
}

// agentMatch is the score-bearing projection used by find-node output.
type agentMatch struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Label string `json:"label"`
	Score int    `json:"score"`
}

func newAgentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent-friendly Figma node discovery (label → id resolution).",
		Long: `Read-only helpers that accept full Figma URLs or raw file keys and return
compact node indexes with human-readable path labels. Use 'agent outline' to
list a file's structure and 'agent find-node' to resolve a label like
"Prototype" into a Figma node id you can pass to files nodes, images, frame
extract, or dev-mode dump.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newAgentOutlineCmd(flags))
	cmd.AddCommand(newAgentFindNodeCmd(flags))
	cmd.AddCommand(newAgentShotCmd(flags))
	cmd.AddCommand(newAgentIndexFilesCmd(flags))
	return cmd
}

func newAgentIndexFilesCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	var teamID string
	var mergeInto string
	var force bool

	cmd := &cobra.Command{
		Use:   "index-files [project-or-team-url]",
		Short: "Build known-files entries from Figma projects or teams.",
		Long: `List Figma files for a project, or walk every project in a team, and emit
an agent-friendly known-files.json shape. This indexes files only; node/screen
labels stay live through agent outline, find-node, and shot.`,
		Example: strings.Trim(`
  figma-pp-cli agent index-files --project 123 --agent
  figma-pp-cli agent index-files --team 456 --merge-into ./known-files.json
  figma-pp-cli agent index-files "https://www.figma.com/files/project/123/App" --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			kind, id := "", ""
			switch {
			case strings.TrimSpace(projectID) != "":
				kind, id = "project", strings.TrimSpace(projectID)
			case strings.TrimSpace(teamID) != "":
				kind, id = "team", strings.TrimSpace(teamID)
			case len(args) > 0:
				var err error
				kind, id, err = parseProjectTeamRef(args[0])
				if err != nil {
					return usageErr(err)
				}
			default:
				return usageErr(fmt.Errorf("provide --project, --team, or a Figma project/team URL"))
			}
			if dryRunOK(flags) {
				paths := []string{"/v1/projects/" + id + "/files"}
				if kind == "team" {
					paths = []string{"/v1/teams/" + id + "/projects", "/v1/projects/<project_id>/files"}
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "command": "agent index-files", "source": map[string]string{"kind": kind, "id": id}, "method": "GET", "paths": paths}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			entries := map[string]knownFile{}
			errs := []map[string]string{}
			sourceName := ""
			if kind == "project" {
				name, files, err := fetchProjectFiles(c, id)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				sourceName = name
				entries = buildKnownFilesEntries(files, name)
			} else {
				name, projects, err := fetchTeamProjects(c, id)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				sourceName = name
				for _, p := range projects {
					_, files, err := fetchProjectFiles(c, p.ID)
					if err != nil {
						errs = append(errs, map[string]string{"project_id": p.ID, "project": p.Name, "error": err.Error()})
						continue
					}
					mergeKnownFileMaps(entries, buildKnownFilesEntries(files, p.Name))
				}
			}

			out := map[string]any{"_comment": "Generated by figma-pp-cli agent index-files. File-level only; use agent outline/find-node/shot for live node labels.", "source": map[string]string{"kind": kind, "id": id, "name": sourceName}, "generated_at": time.Now().UTC().Format(time.RFC3339), "files": entries}
			if len(errs) > 0 {
				out["errors"] = errs
			}
			if mergeInto == "" {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			added, skipped, updated, final, err := mergeKnownFiles(mergeInto, entries, force)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"merged_into": mergeInto, "added": added, "skipped": skipped, "updated": updated, "files": final}, flags)
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "Figma project id to index")
	cmd.Flags().StringVar(&teamID, "team", "", "Figma team id to walk")
	cmd.Flags().StringVar(&mergeInto, "merge-into", "", "Known-files JSON path to update additively")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing aliases when merging, preserving notes when possible")
	return cmd
}

func fetchProjectFiles(c *client.Client, id string) (string, []figmaFileMeta, error) {
	raw, err := c.Get("/v1/projects/"+id+"/files", nil)
	if err != nil {
		return "", nil, err
	}
	var env struct {
		Name  string          `json:"name"`
		Files []figmaFileMeta `json:"files"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", nil, fmt.Errorf("decoding project files response: %w", err)
	}
	return env.Name, env.Files, nil
}

func fetchTeamProjects(c *client.Client, id string) (string, []figmaProjectMeta, error) {
	raw, err := c.Get("/v1/teams/"+id+"/projects", nil)
	if err != nil {
		return "", nil, err
	}
	var env struct {
		Name     string             `json:"name"`
		Projects []figmaProjectMeta `json:"projects"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", nil, fmt.Errorf("decoding team projects response: %w", err)
	}
	return env.Name, env.Projects, nil
}

func mergeKnownFileMaps(dst, src map[string]knownFile) {
	keys := make([]string, 0, len(src))
	for k := range src {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		alias := k
		base := k
		for n := 2; dst[alias].FileKey != ""; n++ {
			alias = fmt.Sprintf("%s-%d", base, n)
		}
		dst[alias] = src[k]
	}
}

func mergeKnownFiles(path string, generated map[string]knownFile, force bool) (added, skipped, updated int, final map[string]json.RawMessage, err error) {
	type knownFilesDoc struct {
		Comment string                     `json:"_comment,omitempty"`
		Files   map[string]json.RawMessage `json:"files"`
		Extra   map[string]json.RawMessage `json:"-"`
	}
	var doc knownFilesDoc
	doc.Files = map[string]json.RawMessage{}
	doc.Extra = map[string]json.RawMessage{}
	if b, readErr := os.ReadFile(path); readErr == nil {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(b, &raw); err != nil {
			return 0, 0, 0, nil, fmt.Errorf("decoding %s: %w", path, err)
		}
		for k, v := range raw {
			switch k {
			case "_comment":
				_ = json.Unmarshal(v, &doc.Comment)
			case "files":
				if err := json.Unmarshal(v, &doc.Files); err != nil {
					return 0, 0, 0, nil, fmt.Errorf("decoding %s files: %w", path, err)
				}
			default:
				doc.Extra[k] = v
			}
		}
	} else if !os.IsNotExist(readErr) {
		return 0, 0, 0, nil, readErr
	}
	if doc.Comment == "" {
		doc.Comment = "Known Figma files for agent resolution. Generated entries are file-level only."
	}

	keys := make([]string, 0, len(generated))
	for k := range generated {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, alias := range keys {
		gen := generated[alias]
		genRaw, err := json.Marshal(gen)
		if err != nil {
			return 0, 0, 0, nil, err
		}
		existing, ok := doc.Files[alias]
		if !ok {
			doc.Files[alias] = genRaw
			added++
			continue
		}
		if !force {
			skipped++
			continue
		}
		var existingObj map[string]json.RawMessage
		if json.Unmarshal(existing, &existingObj) == nil {
			if notes, ok := existingObj["notes"]; ok {
				var genObj map[string]json.RawMessage
				if json.Unmarshal(genRaw, &genObj) == nil {
					if _, hasNotes := genObj["notes"]; !hasNotes {
						genObj["notes"] = notes
						genRaw, err = json.Marshal(genObj)
						if err != nil {
							return 0, 0, 0, nil, err
						}
					}
				}
			}
		}
		doc.Files[alias] = genRaw
		updated++
	}

	out := map[string]any{"_comment": doc.Comment, "files": doc.Files}
	for k, v := range doc.Extra {
		out[k] = v
	}
	body, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return 0, 0, 0, nil, err
	}
	body = append(body, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, 0, 0, nil, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return 0, 0, 0, nil, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return 0, 0, 0, nil, err
	}
	return added, skipped, updated, doc.Files, nil
}

func newAgentOutlineCmd(flags *rootFlags) *cobra.Command {
	var depth int
	var types string
	var limit int

	cmd := &cobra.Command{
		Use:   "outline <figma-url-or-key>",
		Short: "List compact path labels for a Figma file's nodes.",
		Long: `Fetch a shallow Figma file tree and return a compact list of nodes with
human-readable path labels (e.g. "🕹️ Prototype / Prototype / Signup") and
their node ids. Agents use this to discover ids before calling files nodes,
images, frame extract, or dev-mode dump.`,
		Example: strings.Trim(`
  # Outline by raw file key
  figma-pp-cli agent outline abc123XyZ --depth 2 --agent

  # Outline by full Figma URL
  figma-pp-cli agent outline "https://www.figma.com/design/abc123XyZ/My-File" --depth 2 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			ref, err := parseFigmaRef(args[0])
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":  true,
					"command":  "agent outline",
					"file_key": ref.FileKey,
					"node_id":  ref.NodeID,
					"method":   "GET",
					"path":     "/v1/files/" + ref.FileKey,
					"params":   map[string]string{"depth": fmt.Sprintf("%d", depth)},
				}, flags)
			}

			allowedTypes := parseTypeSet(types)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get("/v1/files/"+ref.FileKey, map[string]string{"depth": fmt.Sprintf("%d", depth)})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var env struct {
				Name     string         `json:"name"`
				Document map[string]any `json:"document"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				return fmt.Errorf("decoding file response: %w", err)
			}

			// maxDepth <= 0 means unlimited: show everything the API returned.
			nodes := buildAgentNodeIndex(env.Document, allowedTypes, 0)

			total := len(nodes)
			truncated := false
			if limit > 0 && total > limit {
				nodes = nodes[:limit]
				truncated = true
			}

			out := map[string]any{
				"file_key":  ref.FileKey,
				"file_name": env.Name,
				"depth":     depth,
				"count":     len(nodes),
				"truncated": truncated,
				"nodes":     nodes,
			}
			if truncated {
				out["total_before_limit"] = total
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().IntVar(&depth, "depth", 2, "Max tree depth to request from the Figma file API")
	cmd.Flags().StringVar(&types, "types", "CANVAS,SECTION,FRAME,INSTANCE,COMPONENT,GROUP", "Comma-separated node types to include")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max nodes to return (0 = unlimited)")
	return cmd
}

func newAgentShotCmd(flags *rootFlags) *cobra.Command {
	var max int
	var scale float64
	var format string
	var depth int
	var types string
	var outDir string
	var noDownload bool
	var batch int
	var children bool
	var root string

	cmd := &cobra.Command{
		Use:   "shot <figma-url-or-key> [query]",
		Short: "Resolve, render, and optionally download Figma node screenshots.",
		Long: `Resolve a label (or node-id in a Figma URL), filter to screen-like nodes,
render via /v1/images, and download render bytes to local files. Downloading is
best-effort: when the render CDN is unreachable, output keeps the expiring URL
and records download_error without failing the command.`,
		Example: strings.Trim(`
  figma-pp-cli agent shot abc123XyZ "Cash transfer Intro" --max 3 --agent
  figma-pp-cli agent shot "https://www.figma.com/design/abc123XyZ/T?node-id=123-456" --agent
  figma-pp-cli agent shot abc123XyZ Signup --no-download --agent
  figma-pp-cli agent shot abc123XyZ Welcome --root 1:10 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			ref, err := parseFigmaRef(args[0])
			if err != nil {
				return usageErr(err)
			}
			query := ""
			if len(args) >= 2 {
				query = strings.Join(args[1:], " ")
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": true, "command": "agent shot", "file_key": ref.FileKey,
					"node_id": ref.NodeID, "query": query, "method": "GET",
					"path":   "/v1/images/" + ref.FileKey,
					"params": map[string]string{"format": format, "scale": fmt.Sprintf("%g", scale)},
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			matches, ambiguous, err := resolveAgentShotMatches(flags, c, ref, query, types, depth, max, children, root)
			if err != nil {
				return err
			}
			if len(matches) == 0 || ambiguous {
				out := map[string]any{
					"file_key": ref.FileKey, "query": query, "count": 0, "ambiguous": ambiguous,
					"images": []any{}, "next_steps": agentShotNextSteps(ref.FileKey, query, depth),
				}
				if ambiguous {
					out["matches"] = matches
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			ids := make([]string, 0, len(matches))
			for _, m := range matches {
				ids = append(ids, m.ID)
			}
			renderResult, err := renderImagesDetailed(c, ref.FileKey, format, scale, ids, batch)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			timeout := flags.timeout
			if timeout == 0 {
				timeout = 60 * time.Second
			}
			dlClient := &http.Client{Timeout: timeout}
			images := make([]map[string]any, 0, len(matches))
			ext := strings.TrimPrefix(strings.ToLower(format), ".")
			if ext == "" {
				ext = "png"
			}
			for _, m := range matches {
				item := map[string]any{"id": m.ID, "name": m.Name, "type": m.Type, "label": m.Label, "score": m.Score}
				urlPtr := renderResult.Images[m.ID]
				if urlPtr == nil || *urlPtr == "" {
					if renderErr := renderResult.Errors[m.ID]; renderErr != "" {
						item["render_error"] = renderErr
					} else {
						item["render_error"] = "no image returned"
					}
					images = append(images, item)
					continue
				}
				item["url"] = *urlPtr
				if !noDownload {
					dest := filepath.Join(outDir, sanitizeFilename(m.Label)+"-"+sanitizeFilename(m.ID)+"."+ext)
					bytesWritten, err := downloadToFile(dlClient, *urlPtr, dest)
					if err != nil {
						item["download_error"] = err.Error()
					} else {
						item["path"] = dest
						item["bytes"] = bytesWritten
					}
				}
				images = append(images, item)
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"file_key": ref.FileKey, "query": query, "count": len(images), "ambiguous": false,
				"out_dir": outDir, "images": images,
				"next_steps": []string{
					"Send a screenshot in Slack by attaching the local file at images[].path.",
					"If path is missing, the render CDN was unreachable; use images[].url instead (it expires).",
				},
			}, flags)
		},
	}
	cmd.Flags().IntVar(&max, "max", 3, "Max screenshots to render")
	cmd.Flags().Float64Var(&scale, "scale", 2, "Image render scale")
	cmd.Flags().StringVar(&format, "format", "png", "Image format")
	cmd.Flags().IntVar(&depth, "depth", 3, "Max tree depth to request from the Figma file API")
	cmd.Flags().StringVar(&types, "types", "SECTION,FRAME,COMPONENT,INSTANCE", "Comma-separated node types to include")
	cmd.Flags().StringVar(&outDir, "out-dir", filepath.Join(os.TempDir(), "figma-pp-cli"), "Directory for downloaded screenshots")
	cmd.Flags().BoolVar(&noDownload, "no-download", false, "Return render URLs without downloading image bytes")
	cmd.Flags().IntVar(&batch, "batch", 2, "Node ids per render request; lower avoids Figma render timeouts")
	cmd.Flags().BoolVar(&children, "children", false, "Render screen-like child frames when the top match is a page or section")
	cmd.Flags().StringVar(&root, "root", "", "Node id to scope the search to a page/section subtree (skips the whole-file fetch)")
	return cmd
}

func resolveAgentShotMatches(flags *rootFlags, c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, ref figmaRef, query, types string, depth, max int, children bool, root string) ([]agentMatch, bool, error) {
	if max <= 0 {
		max = 1
	}
	if query == "" && ref.NodeID != "" {
		m, err := agentResolveDirectMatch(flags, c, ref)
		if err != nil {
			return nil, false, err
		}
		return []agentMatch{m}, false, nil
	}
	if query == "" {
		return nil, false, usageErr(fmt.Errorf("a query is required (or supply a Figma URL with a node-id)"))
	}
	effectiveRoot := normalizeNodeID(strings.TrimSpace(root))
	if effectiveRoot == "" && ref.NodeID != "" {
		effectiveRoot = ref.NodeID
	}
	var nodes []agentNodeSummary
	if effectiveRoot != "" {
		doc, err := fetchSubtreeDocument(c, ref.FileKey, effectiveRoot, depth)
		if err != nil {
			return nil, false, classifyAPIError(err, flags)
		}
		if doc == nil {
			return nil, false, fmt.Errorf("root node %q not found in %s", effectiveRoot, ref.FileKey)
		}
		nodes = buildAgentNodeIndex(doc, parseTypeSet(types), 0)
	} else {
		raw, err := c.Get("/v1/files/"+ref.FileKey, map[string]string{"depth": fmt.Sprintf("%d", depth)})
		if err != nil {
			return nil, false, classifyAPIError(err, flags)
		}
		var env struct {
			Document map[string]any `json:"document"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, false, fmt.Errorf("decoding file response: %w", err)
		}
		nodes = buildAgentNodeIndex(env.Document, parseTypeSet(types), 0)
	}
	matches := []agentMatch{}
	for _, n := range nodes {
		s := scoreAgentNode(query, n)
		if s > 0 && isLikelyScreenNode(n) {
			matches = append(matches, agentMatch{ID: n.ID, Name: n.Name, Type: n.Type, Label: n.Label, Score: s})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		if len(matches[i].Label) != len(matches[j].Label) {
			return len(matches[i].Label) < len(matches[j].Label)
		}
		return matches[i].Label < matches[j].Label
	})
	if max == 1 && len(matches) >= 2 && matches[0].Score == matches[1].Score {
		return matches[:2], true, nil
	}
	if children && len(matches) > 0 {
		top := matches[0]
		topType := strings.ToUpper(strings.TrimSpace(top.Type))
		if topType == "CANVAS" || topType == "SECTION" {
			childMatches, err := agentShotChildMatchesFromSubtree(c, ref.FileKey, top, depth)
			if err != nil || len(childMatches) == 0 {
				childMatches = agentShotChildMatchesFromIndex(nodes, top)
			}
			if len(childMatches) > 0 {
				matches = childMatches
			}
		}
	}
	seen := map[string]bool{}
	out := []agentMatch{}
	for _, m := range matches {
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		out = append(out, m)
		if len(out) >= max {
			break
		}
	}
	return out, false, nil
}

func agentShotNextSteps(fileKey, query string, depth int) []string {
	return []string{fmt.Sprintf("No screenshots rendered for %q. Run 'figma-pp-cli agent outline %s --depth %d --agent' to list available labels.", query, fileKey, depth)}
}

func newAgentFindNodeCmd(flags *rootFlags) *cobra.Command {
	var depth int
	var types string
	var limit int

	cmd := &cobra.Command{
		Use:   "find-node <figma-url-or-key> [query]",
		Short: "Resolve a human label into a Figma node id.",
		Long: `Search a shallow Figma file tree for nodes matching a label query (e.g.
"Prototype") and return ranked candidates with node ids. When a query is
omitted but the Figma URL carries a node-id, that node is resolved as a
direct hit. Ambiguous ties are reported as candidates, never guessed.`,
		Example: strings.Trim(`
  # Find by label
  figma-pp-cli agent find-node abc123XyZ "Prototype" --depth 3 --agent

  # Resolve a node-id embedded in a URL (no query)
  figma-pp-cli agent find-node "https://www.figma.com/design/abc123XyZ/T?node-id=123-456" --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			ref, err := parseFigmaRef(args[0])
			if err != nil {
				return usageErr(err)
			}
			query := ""
			if len(args) >= 2 {
				query = strings.Join(args[1:], " ")
			}

			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":  true,
					"command":  "agent find-node",
					"file_key": ref.FileKey,
					"node_id":  ref.NodeID,
					"query":    query,
					"method":   "GET",
					"path":     "/v1/files/" + ref.FileKey,
					"params":   map[string]string{"depth": fmt.Sprintf("%d", depth)},
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Direct-hit path: URL carries a node-id and no query was given.
			if query == "" && ref.NodeID != "" {
				return agentFindNodeDirect(cmd, flags, c, ref)
			}
			if query == "" {
				return usageErr(fmt.Errorf("a query is required (or supply a Figma URL with a node-id)"))
			}

			allowedTypes := parseTypeSet(types)
			raw, err := c.Get("/v1/files/"+ref.FileKey, map[string]string{"depth": fmt.Sprintf("%d", depth)})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var env struct {
				Name     string         `json:"name"`
				Document map[string]any `json:"document"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				return fmt.Errorf("decoding file response: %w", err)
			}

			nodes := buildAgentNodeIndex(env.Document, allowedTypes, 0)

			var matches []agentMatch
			for _, n := range nodes {
				s := scoreAgentNode(query, n)
				if s > 0 {
					matches = append(matches, agentMatch{
						ID:    n.ID,
						Name:  n.Name,
						Type:  n.Type,
						Label: n.Label,
						Score: s,
					})
				}
			}

			sort.SliceStable(matches, func(i, j int) bool {
				if matches[i].Score != matches[j].Score {
					return matches[i].Score > matches[j].Score
				}
				if len(matches[i].Label) != len(matches[j].Label) {
					return len(matches[i].Label) < len(matches[j].Label)
				}
				return matches[i].Label < matches[j].Label
			})

			ambiguous := false
			if len(matches) >= 2 && matches[0].Score == matches[1].Score {
				ambiguous = true
			}

			total := len(matches)
			if limit > 0 && total > limit {
				matches = matches[:limit]
			}

			out := map[string]any{
				"file_key":    ref.FileKey,
				"query":       query,
				"match_count": len(matches),
				"ambiguous":   ambiguous,
				"matches":     matches,
				"next_steps":  agentNextSteps(ref.FileKey),
			}
			if len(matches) > 0 {
				out["best"] = matches[0]
			} else {
				out["best"] = nil
				out["next_steps"] = append([]string{
					fmt.Sprintf("No nodes matched %q. Run 'figma-pp-cli agent outline %s --depth %d --agent' to list available labels.", query, ref.FileKey, depth),
				}, agentNextSteps(ref.FileKey)...)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().IntVar(&depth, "depth", 3, "Max tree depth to request from the Figma file API")
	cmd.Flags().StringVar(&types, "types", "SECTION,FRAME,INSTANCE,COMPONENT", "Comma-separated node types to include")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max matches to return (0 = unlimited)")
	return cmd
}

func decodeNodesDocument(raw json.RawMessage) (map[string]any, error) {
	var env struct {
		Nodes map[string]json.RawMessage `json:"nodes"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	for _, entryRaw := range env.Nodes {
		if len(entryRaw) == 0 || string(entryRaw) == "null" {
			continue
		}
		var entry map[string]any
		if json.Unmarshal(entryRaw, &entry) != nil {
			continue
		}
		if d, ok := entry["document"].(map[string]any); ok {
			return d, nil
		}
	}
	return nil, nil
}

// fetchSubtreeDocument fetches a single node's subtree via the nodes endpoint
// and returns its document map. It returns (nil, nil) when the id is absent or
// the node was deleted; callers decide whether that is an error.
func fetchSubtreeDocument(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, fileKey, nodeID string, depth int) (map[string]any, error) {
	raw, err := c.Get("/v1/files/"+fileKey+"/nodes", map[string]string{"ids": normalizeNodeID(nodeID), "depth": fmt.Sprintf("%d", depth)})
	if err != nil {
		return nil, err
	}
	doc, err := decodeNodesDocument(raw)
	if err != nil {
		return nil, fmt.Errorf("decoding nodes response: %w", err)
	}
	return doc, nil
}

func agentShotChildMatchesFromSubtree(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, fileKey string, top agentMatch, depth int) ([]agentMatch, error) {
	doc, err := fetchSubtreeDocument(c, fileKey, top.ID, depth)
	if err != nil || doc == nil {
		return nil, err
	}
	return agentShotChildMatchesFromNodes(buildAgentNodeIndex(doc, nil, 0), top, false), nil
}

func agentShotChildMatchesFromIndex(nodes []agentNodeSummary, top agentMatch) []agentMatch {
	return agentShotChildMatchesFromNodes(nodes, top, true)
}

func agentShotChildMatchesFromNodes(nodes []agentNodeSummary, top agentMatch, requirePrefix bool) []agentMatch {
	childMatches := []agentMatch{}
	for _, n := range nodes {
		if n.ID == top.ID {
			continue
		}
		childType := strings.ToUpper(strings.TrimSpace(n.Type))
		if childType != "FRAME" && childType != "INSTANCE" && childType != "COMPONENT" {
			continue
		}
		if requirePrefix && !strings.HasPrefix(n.Label, top.Label+" / ") {
			continue
		}
		if !isLikelyScreenNode(n) {
			continue
		}
		childMatches = append(childMatches, agentMatch{ID: n.ID, Name: n.Name, Type: n.Type, Label: n.Label, Score: top.Score})
	}
	sort.SliceStable(childMatches, func(i, j int) bool {
		labelI := childMatches[i].Label
		labelJ := childMatches[j].Label
		if requirePrefix {
			labelI = strings.TrimPrefix(labelI, top.Label+" / ")
			labelJ = strings.TrimPrefix(labelJ, top.Label+" / ")
		}
		depthI := strings.Count(labelI, " / ")
		depthJ := strings.Count(labelJ, " / ")
		if depthI != depthJ {
			return depthI < depthJ
		}
		return childMatches[i].Label < childMatches[j].Label
	})
	return childMatches
}

func agentResolveDirectMatch(flags *rootFlags, c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, ref figmaRef) (agentMatch, error) {
	raw, err := c.Get("/v1/files/"+ref.FileKey+"/nodes", map[string]string{"ids": ref.NodeID, "depth": "1"})
	if err != nil {
		return agentMatch{}, classifyAPIError(err, flags)
	}
	doc, err := decodeNodesDocument(raw)
	if err != nil {
		return agentMatch{}, fmt.Errorf("decoding nodes response: %w", err)
	}
	if doc == nil {
		return agentMatch{}, fmt.Errorf("node %q not found in Figma file %s", ref.NodeID, ref.FileKey)
	}
	name, _ := doc["name"].(string)
	nodeType, _ := doc["type"].(string)
	return agentMatch{ID: ref.NodeID, Name: name, Type: nodeType, Label: name, Score: 100}, nil
}

// agentFindNodeDirect resolves a node-id embedded in a Figma URL as a single
// direct hit. It never invents a name: if the fetch fails or returns no
// document, the caller surfaces the error normally.
func agentFindNodeDirect(cmd *cobra.Command, flags *rootFlags, c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, ref figmaRef) error {
	hit, err := agentResolveDirectMatch(flags, c, ref)
	if err != nil {
		return err
	}
	matches := []agentMatch{hit}
	out := map[string]any{
		"file_key":    ref.FileKey,
		"query":       "",
		"node_id":     ref.NodeID,
		"match_count": 1,
		"ambiguous":   false,
		"best":        hit,
		"matches":     matches,
		"next_steps":  agentNextSteps(ref.FileKey),
	}
	return printJSONFiltered(cmd.OutOrStdout(), out, flags)
}

// agentNextSteps returns stable, copy-pasteable follow-up commands for a
// resolved node id.
func agentNextSteps(fileKey string) []string {
	return []string{
		fmt.Sprintf("Use the chosen id with: figma-pp-cli files nodes get-file %s --ids <id> --depth 2 --agent", fileKey),
		fmt.Sprintf("Render it with: figma-pp-cli images %s --ids <id> --format png --scale 1 --agent", fileKey),
	}
}
