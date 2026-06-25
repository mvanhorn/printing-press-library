package canvaswrite

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/socketio"
	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/yjs"
)

type DocAuditOptions struct {
	WorkspaceID  string
	DocID        string
	SnapshotFile string
	Timestamp    string
	Keywords     []string
	TextLimit    int
}

type DocAuditMatch struct {
	ID      string `json:"id"`
	Flavour string `json:"flavour,omitempty"`
	Type    string `json:"type,omitempty"`
	Text    string `json:"text"`
}

type DocAuditResult struct {
	DocID       string                     `json:"doc_id"`
	Timestamp   string                     `json:"timestamp,omitempty"`
	BlockCount  int                        `json:"block_count"`
	Flavours    map[string]int             `json:"flavours"`
	Notes       DocAuditNoteStats          `json:"notes"`
	TextBlocks  int                        `json:"text_blocks"`
	TextChars   int                        `json:"text_chars"`
	KeywordHits map[string]int             `json:"keyword_hits,omitempty"`
	Matches     map[string][]DocAuditMatch `json:"matches,omitempty"`
}

type DocAuditNoteStats struct {
	Total              int                              `json:"total"`
	DirectPageChildren int                              `json:"direct_page_children"`
	DirectPageNotes    int                              `json:"direct_page_notes"`
	ByDisplayMode      map[string]DocAuditNoteModeStats `json:"by_display_mode"`
}

type DocAuditNoteModeStats struct {
	Notes      int `json:"notes"`
	Hidden     int `json:"hidden"`
	Children   int `json:"children"`
	TextBlocks int `json:"text_blocks"`
	TextChars  int `json:"text_chars"`
}

func AuditDoc(cfg *config.Config, opts DocAuditOptions) (DocAuditResult, error) {
	if opts.SnapshotFile == "" && opts.WorkspaceID == "" {
		return DocAuditResult{}, fmt.Errorf("--workspace is required")
	}
	if opts.SnapshotFile == "" && opts.DocID == "" {
		return DocAuditResult{}, fmt.Errorf("--doc is required")
	}
	if opts.TextLimit <= 0 {
		opts.TextLimit = 240
	}

	engine, err := yjs.NewEngine()
	if err != nil {
		return DocAuditResult{}, err
	}
	blocks, err := auditBlocks(cfg, engine, opts)
	if err != nil {
		return DocAuditResult{}, err
	}

	result := DocAuditResult{
		DocID:      opts.DocID,
		Timestamp:  opts.Timestamp,
		BlockCount: len(blocks),
		Flavours:   map[string]int{},
		Notes: DocAuditNoteStats{
			ByDisplayMode: map[string]DocAuditNoteModeStats{},
		},
	}
	if len(opts.Keywords) > 0 {
		result.KeywordHits = map[string]int{}
		result.Matches = map[string][]DocAuditMatch{}
	}

	ids := make([]string, 0, len(blocks))
	for id := range blocks {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		block := blocks[id]
		flavour := stringField(block, "sys:flavour")
		blockType := stringField(block, "sys:type")
		result.Flavours[flavour]++
		if flavour == "affine:page" {
			children, _ := block["sys:children"].([]any)
			result.Notes.DirectPageChildren += len(children)
			for _, rawID := range children {
				childID, _ := rawID.(string)
				if stringField(blocks[childID], "sys:flavour") == "affine:note" {
					result.Notes.DirectPageNotes++
				}
			}
		}
		if flavour == "affine:note" {
			result.addNoteStats(blocks, block)
		}
		text := stringField(block, "prop:text")
		if text == "" {
			continue
		}
		result.TextBlocks++
		result.TextChars += len([]rune(text))
		lowerText := strings.ToLower(text)
		for _, keyword := range opts.Keywords {
			keyword = strings.TrimSpace(keyword)
			if keyword == "" {
				continue
			}
			hits := strings.Count(lowerText, strings.ToLower(keyword))
			if hits == 0 {
				continue
			}
			result.KeywordHits[keyword] += hits
			if len(result.Matches[keyword]) < 10 {
				result.Matches[keyword] = append(result.Matches[keyword], DocAuditMatch{ID: id, Flavour: flavour, Type: blockType, Text: truncateRunes(text, opts.TextLimit)})
			}
		}
	}
	return result, nil
}

func (r *DocAuditResult) addNoteStats(blocks map[string]map[string]any, note map[string]any) {
	mode := stringField(note, "prop:displayMode")
	if mode == "" {
		mode = "(unset)"
	}
	children, _ := note["sys:children"].([]any)
	textBlocks, textChars := countTextDescendants(blocks, children, map[string]bool{})
	stats := r.Notes.ByDisplayMode[mode]
	stats.Notes++
	if hidden, _ := note["prop:hidden"].(bool); hidden {
		stats.Hidden++
	}
	stats.Children += len(children)
	stats.TextBlocks += textBlocks
	stats.TextChars += textChars
	r.Notes.ByDisplayMode[mode] = stats
	r.Notes.Total++
}

func countTextDescendants(blocks map[string]map[string]any, children []any, seen map[string]bool) (int, int) {
	var textBlocks int
	var textChars int
	for _, rawID := range children {
		id, _ := rawID.(string)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		block := blocks[id]
		if block == nil {
			continue
		}
		if text := stringField(block, "prop:text"); text != "" {
			textBlocks++
			textChars += len([]rune(text))
		}
		nested, _ := block["sys:children"].([]any)
		nestedBlocks, nestedChars := countTextDescendants(blocks, nested, seen)
		textBlocks += nestedBlocks
		textChars += nestedChars
	}
	return textBlocks, textChars
}

func auditBlocks(cfg *config.Config, engine *yjs.Engine, opts DocAuditOptions) (map[string]map[string]any, error) {
	if opts.SnapshotFile != "" {
		raw, err := os.ReadFile(opts.SnapshotFile)
		if err != nil {
			return nil, err
		}
		doc, err := engine.ApplyBase64Update(base64.StdEncoding.EncodeToString(raw))
		if err != nil {
			return nil, err
		}
		return engine.ReadBlocks(doc)
	}

	if opts.Timestamp != "" {
		base := strings.TrimSuffix(strings.TrimSuffix(cfg.BaseURL, "/graphql"), "/")
		url := fmt.Sprintf("%s/api/workspaces/%s/docs/%s/histories/%s", base, opts.WorkspaceID, opts.DocID, opts.Timestamp)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		if cfg.AuthHeader() != "" {
			req.Header.Set("Authorization", cfg.AuthHeader())
		}
		limiter := cliutil.NewAdaptiveLimiter(1)
		limiter.Wait()
		hc := &http.Client{Timeout: 60 * time.Second}
		res, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		if res.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("snapshot %s: rate limited: %s", opts.Timestamp, res.Status)
		}
		if res.StatusCode < 200 || res.StatusCode > 299 {
			body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
			return nil, fmt.Errorf("snapshot %s: %s %s", opts.Timestamp, res.Status, strings.TrimSpace(string(body)))
		}
		raw, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		doc, err := engine.ApplyBase64Update(base64.StdEncoding.EncodeToString(raw))
		if err != nil {
			return nil, err
		}
		return engine.ReadBlocks(doc)
	}

	bearer := cfg.AffineToken
	if bearer == "" {
		bearer = strings.TrimPrefix(cfg.AuthHeader(), "Bearer ")
	}
	client, err := socketio.Connect(socketio.WSURLFromGraphQL(strings.TrimRight(cfg.BaseURL, "/")+"/graphql"), "", bearer)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	if err := joinWorkspace(client, opts.WorkspaceID); err != nil {
		return nil, err
	}
	loaded, err := client.LoadDoc(opts.WorkspaceID, opts.DocID)
	if err != nil {
		return nil, err
	}
	doc, err := engine.ApplyBase64Update(loaded.Missing)
	if err != nil {
		return nil, err
	}
	return engine.ReadBlocks(doc)
}

func truncateRunes(s string, limit int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "..."
}
