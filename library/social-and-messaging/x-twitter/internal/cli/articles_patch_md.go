// PATCH: articles patch-md — surgical edits to an existing X Article.
//
// update-md rebuilds the whole body from markdown, so it can only keep what
// the markdown says. That is wrong for an article that already has uploaded
// images, composer-made embeds and hand formatting: an edit that should add
// two paragraphs would re-upload every image and drop anything the markdown
// file forgot. patch-md instead reads the article's current content_state,
// converts it to the write shape untouched, and applies a short list of
// anchored operations from a patch file:
//
//	<!-- after: <start of an existing block's text> -->
//	markdown to insert (embedded posts are bare status URLs on their own line)
//
//	<!-- before: <start of an existing block's text> -->
//	markdown to insert
//
//	<!-- replace: <exact old text> => <new text> -->
//
// Every anchor must match exactly one block and every replace target must
// occur exactly once, or nothing is written. The write reuses update-md's
// sequence (preflight, optional Unpublish -> UpdateContent -> Publish with a
// restore publish on failure).

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type articlePatchOp struct {
	Kind     string // "after", "before", "replace"
	Anchor   string // after/before: block-text prefix; replace: old text
	New      string // replace: new text
	Markdown string // after/before: markdown to insert
	Line     int    // 1-based line of the directive, for errors
}

// readContentState is the camelCase shape ArticleEntitiesSlice returns.
type readContentState struct {
	Blocks []struct {
		Key               string           `json:"key"`
		Text              string           `json:"text"`
		Type              string           `json:"type"`
		Data              map[string]any   `json:"data"`
		EntityRanges      []map[string]any `json:"entityRanges"`
		InlineStyleRanges []inlineStyle    `json:"inlineStyleRanges"`
	} `json:"blocks"`
	EntityMap []struct {
		Key   string `json:"key"`
		Value struct {
			Type       string         `json:"type"`
			Mutability string         `json:"mutability"`
			Data       map[string]any `json:"data"`
		} `json:"value"`
	} `json:"entityMap"`
}

type articlePatchSummary struct {
	ArticleID      string         `json:"article_id"`
	Lifecycle      string         `json:"lifecycle"`
	BlocksBefore   int            `json:"blocks_before"`
	BlocksAfter    int            `json:"blocks_after"`
	EntitiesBefore int            `json:"entities_before"`
	EntitiesAfter  int            `json:"entities_after"`
	Operations     []string       `json:"operations"`
	AddedEntities  map[string]int `json:"added_entities"`
	Fingerprint    string         `json:"base_fingerprint"`
}

func newArticlesPatchMdCmd(flags *rootFlags) *cobra.Command {
	var articleID string
	var republish bool
	cmd := &cobra.Command{
		Use:   "patch-md <patch-file>",
		Short: "Insert markdown (including embedded posts) into an existing X Article without rewriting the rest",
		Long: "Reads the article's current body and applies anchored edits from a patch file, keeping every existing " +
			"block, image and embed exactly as it is. Directives (HTML comment lines):\n\n" +
			"  <!-- after: <start of an existing paragraph or heading> -->   insert the markdown that follows after that block\n" +
			"  <!-- before: <start of an existing paragraph or heading> -->  insert it before that block\n" +
			"  <!-- replace: <exact old text> => <new text> -->              change text inside one block\n\n" +
			"Markdown under after/before uses the articles-publish-md converter: a post URL alone on a line " +
			"(https://x.com/<user>/status/<id>) becomes an embedded post, ![alt](path) uploads an image, and fenced " +
			"code, lists, links, bold and italic work as usual. Each anchor must match exactly one block and each " +
			"replace target must appear exactly once, or nothing is written. --dry-run prints the plan and the full " +
			"content_state with no write. Published articles need --republish (Unpublish -> UpdateContent -> Publish, " +
			"the same sequence X's composer uses for an edit; the article is briefly unpublished).",
		Example: "  x-twitter-pp-cli articles patch-md edits.md --article-id 1750000000000000000 --dry-run\n" +
			"  x-twitter-pp-cli articles patch-md edits.md --article-id 1750000000000000000 --republish",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			if strings.TrimSpace(articleID) == "" {
				return usageErr(fmt.Errorf("--article-id is required (find it via `articles list --lifecycle Published --agent`)"))
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read %s: %w", args[0], err)
			}
			ops, err := parseArticlePatch(string(data))
			if err != nil {
				return usageErr(err)
			}
			if os.Getenv("PRINTING_PRESS_VERIFY") == "1" {
				fmt.Fprintln(cmd.OutOrStdout(), "verify-env: skipping article patch")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// The plan needs the article's real body, so the read always goes
			// out. --dry-run still writes nothing: the command returns before
			// updateMarkdownArticle, the only path that POSTs.
			c.DryRun = false
			deps := articleUpdateDeps{
				post: c,
				fetchSlice: func(ctx context.Context, lifecycle string) (json.RawMessage, error) {
					return fetchArticleSlice(ctx, c, lifecycle)
				},
				uploadImage: func(p string) (string, error) {
					return c.UploadArticleImage(cmd.Context(), p)
				},
			}
			current, lifecycle, fingerprint, err := loadArticleContentState(cmd.Context(), deps, strings.TrimSpace(articleID))
			if err != nil {
				return err
			}
			patched, summary, err := applyArticlePatch(current, ops)
			if err != nil {
				return err
			}
			summary.ArticleID = strings.TrimSpace(articleID)
			summary.Lifecycle = lifecycle
			summary.Fingerprint = fingerprint
			if flags.dryRun {
				enc := json.NewEncoder(cmd.OutOrStdout())
				if !flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "── Article patch plan (dry-run, no API call) ──")
					enc.SetIndent("", "  ")
				}
				return enc.Encode(map[string]any{"plan": summary, "content_state": articleContentStateRequest(patched)})
			}
			result, err := updateMarkdownArticle(cmd.Context(), deps, articleUpdateOptions{
				articleID:          strings.TrimSpace(articleID),
				contentState:       patched,
				republish:          republish,
				allowedEntityTypes: patchCarrySet(),
				baseFingerprint:    fingerprint,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			out := map[string]any{"result": result, "plan": summary}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "patched article %s (%d -> %d blocks)\n%s\n", result.ArticleID, summary.BlocksBefore, summary.BlocksAfter, result.URL)
			return nil
		},
	}
	cmd.Flags().StringVar(&articleID, "article-id", "", "rest_id of the article to patch (required; find it via articles list)")
	cmd.Flags().BoolVar(&republish, "republish", false, "Allow patching a published article via Unpublish -> UpdateContent -> Publish (briefly unpublishes it)")
	return cmd
}

// parseArticlePatch splits a patch file into operations. Text before the
// first directive must be blank or comments.
func parseArticlePatch(src string) ([]articlePatchOp, error) {
	lines := strings.Split(src, "\n")
	var ops []articlePatchOp
	var cur *articlePatchOp
	var body []string
	flush := func() {
		if cur != nil && cur.Kind != "replace" {
			cur.Markdown = strings.TrimSpace(strings.Join(body, "\n"))
		}
		if cur != nil {
			ops = append(ops, *cur)
		}
		cur = nil
		body = nil
	}
	inFence := false
	for i, raw := range lines {
		trim := strings.TrimSpace(raw)
		if strings.HasPrefix(trim, "```") {
			inFence = !inFence
		}
		// Directive-shaped lines inside a fenced code sample are content.
		if kind, arg, ok := parsePatchDirective(trim); ok && !inFence {
			flush()
			op := articlePatchOp{Kind: kind, Line: i + 1}
			switch kind {
			case "replace":
				parts := strings.SplitN(arg, "=>", 2)
				if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
					return nil, fmt.Errorf("line %d: replace needs `<!-- replace: old text => new text -->`", i+1)
				}
				op.Anchor = strings.TrimSpace(parts[0])
				op.New = strings.TrimSpace(parts[1])
			default:
				if arg == "" {
					return nil, fmt.Errorf("line %d: %s needs the start of an existing block's text", i+1, kind)
				}
				op.Anchor = arg
			}
			cur = &op
			continue
		}
		if cur == nil {
			if trim != "" && !strings.HasPrefix(trim, "<!--") && trim != "---" {
				return nil, fmt.Errorf("line %d: text before the first directive; start with <!-- after: ... -->, <!-- before: ... --> or <!-- replace: ... -->", i+1)
			}
			continue
		}
		if cur.Kind == "replace" {
			if trim != "" {
				return nil, fmt.Errorf("line %d: a replace directive takes no body; put text in an after/before block", i+1)
			}
			continue
		}
		body = append(body, raw)
	}
	flush()
	if len(ops) == 0 {
		return nil, fmt.Errorf("patch file has no directives")
	}
	for _, op := range ops {
		if op.Kind != "replace" && op.Markdown == "" {
			return nil, fmt.Errorf("line %d: %s directive has no markdown under it", op.Line, op.Kind)
		}
	}
	return ops, nil
}

func parsePatchDirective(line string) (string, string, bool) {
	if !strings.HasPrefix(line, "<!--") || !strings.HasSuffix(line, "-->") {
		return "", "", false
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "<!--"), "-->"))
	for _, kind := range []string{"after", "before", "replace"} {
		if strings.HasPrefix(inner, kind+":") {
			return kind, strings.TrimSpace(inner[len(kind)+1:]), true
		}
	}
	return "", "", false
}

// loadArticleContentState finds the article in the Draft or Published slice
// and returns its body in the write shape, its lifecycle, and a fingerprint
// of the body as read so the write can refuse if it changed in between.
func loadArticleContentState(ctx context.Context, deps articleUpdateDeps, articleID string) (draftContentState, string, string, error) {
	for _, lifecycle := range []string{"Draft", "Published"} {
		raw, err := deps.fetchSlice(ctx, lifecycle)
		if err != nil {
			return draftContentState{}, "", "", fmt.Errorf("read %s articles: %w", lifecycle, err)
		}
		rcs, found, err := findArticleContentState(raw, articleID)
		if err != nil {
			return draftContentState{}, "", "", err
		}
		if found {
			cs, err := readContentStateToWrite(rcs)
			return cs, lifecycle, contentStateFingerprint(rcs), err
		}
	}
	return draftContentState{}, "", "", fmt.Errorf("article %s not found in your Draft or Published articles; check the id via `articles list --agent`", articleID)
}

// contentStateFingerprint hashes the body as read. encoding/json sorts map
// keys, so the same body always hashes the same.
func contentStateFingerprint(rcs readContentState) string {
	raw, err := json.Marshal(rcs)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// patchCarrySet lists the entity types readContentStateToWrite can carry over
// unchanged: everything the converter emits plus TWEMOJI.
func patchCarrySet() map[string]bool {
	set := converterEntityEmitSet()
	set["TWEMOJI"] = true
	return set
}

func findArticleContentState(raw json.RawMessage, articleID string) (readContentState, bool, error) {
	var response struct {
		Data struct {
			User struct {
				Result struct {
					Slice struct {
						Items []struct {
							Results struct {
								Result struct {
									RestID       string           `json:"rest_id"`
									ContentState readContentState `json:"content_state"`
								} `json:"result"`
							} `json:"article_entity_results"`
						} `json:"items"`
					} `json:"articles_article_mixer_slice"`
				} `json:"result"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return readContentState{}, false, fmt.Errorf("parse articles slice: %w", err)
	}
	for _, item := range response.Data.User.Result.Slice.Items {
		if item.Results.Result.RestID == articleID {
			return item.Results.Result.ContentState, true, nil
		}
	}
	return readContentState{}, false, nil
}

// readContentStateToWrite converts the camelCase read shape to the snake_case
// write shape UpdateContent accepts. Verified live: entityKey (server-owned)
// must be dropped or the write 422s; block data (mentions, urls) is derived
// by the server from the text, and sending it back duplicates mentions, so
// blocks are written with empty data.
func readContentStateToWrite(rcs readContentState) (draftContentState, error) {
	out := draftContentState{Blocks: []draftBlock{}, EntityMap: []draftEntity{}}
	keyIndex := map[string]int{}
	for _, e := range rcs.EntityMap {
		data, err := entityReadDataToWrite(e.Value.Type, e.Value.Data)
		if err != nil {
			return draftContentState{}, err
		}
		keyIndex[e.Key] = len(out.EntityMap)
		out.EntityMap = append(out.EntityMap, draftEntity{
			Key:   strconv.Itoa(len(out.EntityMap)),
			Value: draftEntityValue{Type: e.Value.Type, Mutability: e.Value.Mutability, Data: data},
		})
	}
	for _, b := range rcs.Blocks {
		ranges := []map[string]any{}
		for _, r := range b.EntityRanges {
			oldKey := fmt.Sprint(r["key"])
			if f, ok := r["key"].(float64); ok {
				oldKey = strconv.Itoa(int(f))
			}
			idx, ok := keyIndex[oldKey]
			if !ok {
				return draftContentState{}, fmt.Errorf("block %s references entity %s that is not in the entity map", b.Key, oldKey)
			}
			ranges = append(ranges, map[string]any{"key": idx, "offset": jsonInt(r["offset"]), "length": jsonInt(r["length"])})
		}
		styles := b.InlineStyleRanges
		if styles == nil {
			styles = []inlineStyle{}
		}
		out.Blocks = append(out.Blocks, draftBlock{
			Data:              map[string]any{},
			Text:              b.Text,
			Key:               b.Key,
			Type:              b.Type,
			EntityRanges:      ranges,
			InlineStyleRanges: styles,
		})
	}
	return out, nil
}

func jsonInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}

func entityReadDataToWrite(entityType string, data map[string]any) (map[string]any, error) {
	str := func(k string) string { s, _ := data[k].(string); return s }
	switch entityType {
	case "LINK", "TWEMOJI":
		return map[string]any{"url": str("url")}, nil
	case "MARKDOWN":
		return map[string]any{"markdown": str("markdown")}, nil
	case "DIVIDER":
		return map[string]any{}, nil
	case "TWEET":
		id := str("tweetId")
		if id == "" {
			id = str("tweet_id")
		}
		return map[string]any{"tweet_id": id}, nil
	case "MEDIA":
		itemsRaw, _ := data["mediaItems"].([]any)
		items := []map[string]any{}
		for _, it := range itemsRaw {
			m, _ := it.(map[string]any)
			s := func(k string) string { v, _ := m[k].(string); return v }
			items = append(items, map[string]any{
				"local_media_id": jsonInt(m["localMediaId"]),
				"media_category": s("mediaCategory"),
				"media_id":       s("mediaId"),
			})
		}
		out := map[string]any{"media_items": items}
		if caption := str("caption"); caption != "" {
			out["caption"] = caption
		}
		return out, nil
	}
	return nil, fmt.Errorf("article contains a %s entity that patch-md cannot carry over safely; edit this article in the composer", entityType)
}

func applyArticlePatch(cs draftContentState, ops []articlePatchOp) (draftContentState, articlePatchSummary, error) {
	summary := articlePatchSummary{
		BlocksBefore:   len(cs.Blocks),
		EntitiesBefore: len(cs.EntityMap),
		AddedEntities:  map[string]int{},
	}
	// Resolve every anchor against the ORIGINAL body first so one insert
	// cannot change what a later anchor matches.
	type resolved struct {
		op    articlePatchOp
		index int
	}
	var inserts []resolved
	for _, op := range ops {
		if op.Kind == "replace" {
			continue
		}
		idx, err := findAnchorBlock(cs.Blocks, op)
		if err != nil {
			return draftContentState{}, summary, err
		}
		inserts = append(inserts, resolved{op, idx})
	}
	// Text replacements first (they never move blocks).
	for _, op := range ops {
		if op.Kind != "replace" {
			continue
		}
		if err := replaceInBlocks(cs.Blocks, op); err != nil {
			return draftContentState{}, summary, err
		}
		summary.Operations = append(summary.Operations, fmt.Sprintf("replace %q => %q", op.Anchor, op.New))
	}
	// Convert each insert in file order, renumbering its entities onto the
	// end of the map, then rebuild the block list in one pass over the
	// original body. Chunks that land at the same position keep file order.
	pending := map[int][]draftBlock{}
	for _, r := range inserts {
		chunk := MarkdownBodyToDraftJS(r.op.Markdown)
		base := len(cs.EntityMap)
		for _, e := range chunk.EntityMap {
			k, _ := strconv.Atoi(e.Key)
			e.Key = strconv.Itoa(base + k)
			cs.EntityMap = append(cs.EntityMap, e)
			summary.AddedEntities[e.Value.Type]++
		}
		for bi := range chunk.Blocks {
			for ri := range chunk.Blocks[bi].EntityRanges {
				chunk.Blocks[bi].EntityRanges[ri]["key"] = base + jsonInt(chunk.Blocks[bi].EntityRanges[ri]["key"])
			}
		}
		pos := insertPosition(r.op, r.index)
		pending[pos] = append(pending[pos], chunk.Blocks...)
		summary.Operations = append(summary.Operations, fmt.Sprintf("%s %q: +%d blocks", r.op.Kind, truncateAnchor(r.op.Anchor), len(chunk.Blocks)))
	}
	rebuilt := make([]draftBlock, 0, len(cs.Blocks)+len(pending))
	for i := 0; i <= len(cs.Blocks); i++ {
		rebuilt = append(rebuilt, pending[i]...)
		if i < len(cs.Blocks) {
			rebuilt = append(rebuilt, cs.Blocks[i])
		}
	}
	cs.Blocks = rebuilt
	// Block keys must stay unique across the merged body.
	seen := map[string]bool{}
	for i := range cs.Blocks {
		for seen[cs.Blocks[i].Key] {
			cs.Blocks[i].Key = randBlockKey()
		}
		seen[cs.Blocks[i].Key] = true
	}
	summary.BlocksAfter = len(cs.Blocks)
	summary.EntitiesAfter = len(cs.EntityMap)
	return cs, summary, nil
}

func insertPosition(op articlePatchOp, index int) int {
	if op.Kind == "after" {
		return index + 1
	}
	return index
}

func normalizeAnchorText(s string) string {
	s = strings.NewReplacer("\u2019", "'", "\u2018", "'", "\u201c", "\"", "\u201d", "\"", "\u00a0", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func findAnchorBlock(blocks []draftBlock, op articlePatchOp) (int, error) {
	want := normalizeAnchorText(op.Anchor)
	var hits []int
	for i, b := range blocks {
		if b.Type == "atomic" {
			continue
		}
		if strings.HasPrefix(normalizeAnchorText(b.Text), want) {
			hits = append(hits, i)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return -1, fmt.Errorf("line %d: no block starts with %q; copy the start of the paragraph or heading exactly", op.Line, op.Anchor)
	default:
		return -1, fmt.Errorf("line %d: %d blocks start with %q; use a longer anchor", op.Line, len(hits), op.Anchor)
	}
}

func truncateAnchor(s string) string {
	if len(s) > 48 {
		return s[:48] + "..."
	}
	return s
}

// replaceInBlocks changes one exact occurrence of op.Anchor across all text
// blocks, shifting inline style and entity ranges by the UTF-16 length change.
// A replacement that cuts across a range boundary is refused.
func replaceInBlocks(blocks []draftBlock, op articlePatchOp) error {
	type hit struct{ block, byteIdx int }
	var hits []hit
	for i, b := range blocks {
		if b.Type == "atomic" {
			continue
		}
		from := 0
		for {
			j := strings.Index(b.Text[from:], op.Anchor)
			if j < 0 {
				break
			}
			hits = append(hits, hit{i, from + j})
			// Advance one byte past the match start so overlapping
			// occurrences ("ana" in "banana") count as ambiguous too.
			from += j + 1
		}
	}
	if len(hits) != 1 {
		return fmt.Errorf("line %d: replace target %q appears %d times; it must appear exactly once", op.Line, op.Anchor, len(hits))
	}
	b := &blocks[hits[0].block]
	start := utf16Len(b.Text[:hits[0].byteIdx])
	oldLen := utf16Len(op.Anchor)
	end := start + oldLen
	delta := utf16Len(op.New) - oldLen
	shift := func(offset, length int) (int, int, error) {
		rEnd := offset + length
		switch {
		case rEnd <= start:
			return offset, length, nil
		case offset >= end:
			return offset + delta, length, nil
		case offset <= start && rEnd >= end:
			return offset, length + delta, nil
		}
		return 0, 0, fmt.Errorf("line %d: replace target %q cuts across a link or style boundary; narrow it so it stays entirely inside or outside each link or styled span", op.Line, op.Anchor)
	}
	for i := range b.InlineStyleRanges {
		o, l, err := shift(b.InlineStyleRanges[i].Offset, b.InlineStyleRanges[i].Length)
		if err != nil {
			return err
		}
		b.InlineStyleRanges[i].Offset, b.InlineStyleRanges[i].Length = o, l
	}
	for i := range b.EntityRanges {
		o, l, err := shift(jsonInt(b.EntityRanges[i]["offset"]), jsonInt(b.EntityRanges[i]["length"]))
		if err != nil {
			return err
		}
		b.EntityRanges[i]["offset"], b.EntityRanges[i]["length"] = o, l
	}
	b.Text = b.Text[:hits[0].byteIdx] + op.New + b.Text[hits[0].byteIdx+len(op.Anchor):]
	return nil
}
