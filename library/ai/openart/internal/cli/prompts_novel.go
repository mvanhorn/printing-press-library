package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/openart/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/openart/internal/openartmodels"
)

func newPromptsNovelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompts",
		Short: "Recover, replay, and rank past OpenArt prompts (FTS over local store)",
		Long: `Distinct from 'prompt' (singular, the spec-derived enhance/from-image
utilities), 'prompts' (plural) operates on your local mirror of past
generations: full-text search, replay-with-changes, spend leaderboard.

All subcommands run against the local SQLite store. Run
'openart-pp-cli sync' to refresh.`,
	}
	cmd.AddCommand(newPromptsFindCmd(flags))
	cmd.AddCommand(newPromptsReplayCmd(flags))
	cmd.AddCommand(newPromptsTopCmd(flags))
	return cmd
}

func newPromptsFindCmd(flags *rootFlags) *cobra.Command {
	var (
		modelInput  string
		hasAudio    bool
		minDuration int
		maxDuration int
		since       string
		limit       int
	)
	cmd := &cobra.Command{
		Use:   "find <query>",
		Short: "Full-text search past generations by prompt text, with OpenArt-specific filters",
		Example: `  openart-pp-cli prompts find "molten dragon"
  openart-pp-cli prompts find "neon city" --model seedance2 --has-audio --since 30d`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.Join(args, " ")

			db, err := openLocalStore()
			if err != nil {
				return fmt.Errorf("open local store: %w (run 'openart-pp-cli sync' first)", err)
			}
			defer db.Close()

			cutoff := parseSince(since)
			modelFilter := ""
			if modelInput != "" {
				m := openartmodels.Resolve(modelInput)
				if m == nil {
					return fmt.Errorf("unknown model %q", modelInput)
				}
				modelFilter = m.Slug
			}

			// FTS5 over resources_fts (id, resource_type, content)
			rows, err := db.QueryContext(cmd.Context(), `
				SELECT m.id, m.data, m.created_at, m.synced_at, m.url, m.thumbnail_url, m.resource_type
				FROM resources_fts ft
				JOIN media m ON m.id = ft.id
				WHERE resources_fts MATCH ?
				ORDER BY rank
				LIMIT ?`, query, ftsLimit(limit))
			if err != nil {
				return fmt.Errorf("FTS query: %w", err)
			}
			defer rows.Close()

			type hit struct {
				ResourceID   string  `json:"resource_id"`
				Prompt       string  `json:"prompt"`
				Model        string  `json:"model"`
				Capability   string  `json:"capability_id"`
				Tool         string  `json:"tool"`
				URL          string  `json:"url"`
				ThumbnailURL string  `json:"thumbnail_url,omitempty"`
				ResourceType string  `json:"resource_type"`
				DurationSec  float64 `json:"duration_sec,omitempty"`
				HasAudio     bool    `json:"has_audio,omitempty"`
				CreatedAt    string  `json:"created_at"`
			}
			out := []hit{}
			for rows.Next() {
				var id, syncedAt, urlStr, thumb, rType string
				var createdAt sql.NullInt64
				var data []byte
				if err := rows.Scan(&id, &data, &createdAt, &syncedAt, &urlStr, &thumb, &rType); err != nil {
					continue
				}
				h := hit{ResourceID: id, URL: urlStr, ThumbnailURL: thumb, ResourceType: rType, CreatedAt: syncedAt}
				if createdAt.Valid {
					h.CreatedAt = time.UnixMilli(createdAt.Int64).UTC().Format("2006-01-02 15:04:05")
				}
				var blob map[string]any
				if json.Unmarshal(data, &blob) == nil {
					if input, ok := blob["input"].(map[string]any); ok {
						if p, ok := input["prompt"].(string); ok {
							h.Prompt = p
						}
						if m, ok := input["model"].(string); ok {
							h.Model = m
						}
					}
					if gen, ok := blob["generation"].(map[string]any); ok {
						if cap, ok := gen["capabilityId"].(string); ok {
							h.Capability = cap
							if h.Model == "" {
								if i := strings.IndexByte(cap, ':'); i > 0 {
									h.Model = cap[:i]
								}
							}
						}
						if t, ok := gen["tool"].(string); ok {
							h.Tool = t
						}
					}
					if meta, ok := blob["metadata"].(map[string]any); ok {
						if d, ok := meta["duration"].(float64); ok {
							h.DurationSec = d
						}
						if hau, ok := meta["has_audio"].(bool); ok {
							h.HasAudio = hau
						}
					}
				}

				if modelFilter != "" && h.Model != modelFilter {
					continue
				}
				if hasAudio && !h.HasAudio {
					continue
				}
				if minDuration > 0 && h.DurationSec < float64(minDuration) {
					continue
				}
				if maxDuration > 0 && h.DurationSec > float64(maxDuration) {
					continue
				}
				// PATCH: --since windows on created_at (generation time)
				// rather than synced_at, which is rewritten on every resync —
				// after a first-time full sync every historical row would
				// land inside the window. synced_at remains the fallback for
				// rows whose payload had no createdAt. Greptile P1 on PR #554.
				if !cutoff.IsZero() {
					if createdAt.Valid {
						if time.UnixMilli(createdAt.Int64).Before(cutoff) {
							continue
						}
					} else {
						ts, _ := time.Parse(time.RFC3339, syncedAt)
						if ts.IsZero() {
							ts, _ = time.Parse("2006-01-02 15:04:05", syncedAt)
						}
						if !ts.IsZero() && ts.Before(cutoff) {
							continue
						}
					}
				}
				out = append(out, h)
				if limit > 0 && len(out) >= limit {
					break
				}
			}
			result := map[string]any{
				"query":   query,
				"matches": out,
				"count":   len(out),
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&modelInput, "model", "", "Filter by model slug or shorthand")
	cmd.Flags().BoolVar(&hasAudio, "has-audio", false, "Only return generations that produced audio")
	cmd.Flags().IntVar(&minDuration, "duration-min", 0, "Minimum duration in seconds")
	cmd.Flags().IntVar(&maxDuration, "duration-max", 0, "Maximum duration in seconds")
	cmd.Flags().StringVar(&since, "since", "", "Time window: 24h, 7d, 30d, 90d")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max hits to return")
	return cmd
}

func newPromptsReplayCmd(flags *rootFlags) *cobra.Command {
	var (
		modelOverride string
		bumpFlags     []string
		wait          bool
		downloadDir   string
	)
	cmd := &cobra.Command{
		Use:   "replay <resourceId>",
		Short: "Re-issue a past generation, optionally on a different model with parameter bumps",
		Long: `Looks up a past generation in the local store, remaps its parameters
across model schemas if --model is provided, and re-issues the submit.

Use --bump key=value to override individual params (e.g. --bump duration=10
--bump count=2). Pass --wait and --download to behave like 'video gen'.`,
		Example: `  # Re-run on the same model
  openart-pp-cli prompts replay 3dVHEhDjyq82gLwBudaG --wait

  # Re-run the prompt on a cheaper model
  openart-pp-cli prompts replay 3dVHEhDjyq82gLwBudaG --model grok-imagine

  # Tweak duration and count
  openart-pp-cli prompts replay <id> --bump duration=10 --bump count=2`,
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			resourceID := args[0]

			db, err := openLocalStore()
			if err != nil {
				return fmt.Errorf("open local store: %w (run 'openart-pp-cli sync' first, or pass the resource through 'media get')", err)
			}
			defer db.Close()
			row := db.QueryRowContext(cmd.Context(), `SELECT data FROM media WHERE id = ?`, resourceID)
			var raw []byte
			if err := row.Scan(&raw); err != nil {
				return fmt.Errorf("resource %s not found in local store: %w", resourceID, err)
			}
			var blob map[string]any
			if err := json.Unmarshal(raw, &blob); err != nil {
				return fmt.Errorf("parse stored resource: %w", err)
			}
			input, _ := blob["input"].(map[string]any)
			if input == nil {
				input = map[string]any{}
			}
			gen, _ := blob["generation"].(map[string]any)
			origCap, _ := gen["capabilityId"].(string)

			origModelSlug := ""
			if origCap != "" {
				if i := strings.IndexByte(origCap, ':'); i > 0 {
					origModelSlug = origCap[:i]
				}
			}
			if s, ok := input["model"].(string); ok && s != "" {
				origModelSlug = s
			}

			newModel := openartmodels.Resolve(origModelSlug)
			if modelOverride != "" {
				newModel = openartmodels.Resolve(modelOverride)
				if newModel == nil {
					return fmt.Errorf("unknown model %q", modelOverride)
				}
			}
			if newModel == nil {
				return fmt.Errorf("could not resolve a model to replay against")
			}

			// PATCH: pick the right submit shape for the target model.
			// Image models take {prompt, imageCount, aspectRatio,
			// visualReferences, resolution?} against
			// create-image:reference:<slug>; video models take the
			// {prompt, videoCount, duration, aspectRatio, resolution, ...}
			// body against <slug>:text2video. Sending video fields to
			// the image endpoint silently 4xx's at submit time (greptile
			// P1 follow-up on PR #554). The earlier resolution-panic
			// guard is now folded into the video branch where it
			// applies; image bodies don't need it.
			isImage := newModel.Family == openartmodels.FamilyImage
			formType := openartmodels.FormText2Video
			if isImage {
				formType = openartmodels.FormText2Image
			}

			var body map[string]any
			if isImage {
				count := intOr(input["imageCount"], intOr(input["videoCount"], 1))
				body = map[string]any{
					"prompt":           stringOr(input["prompt"], ""),
					"model":            newModel.Slug,
					"projectId":        "",
					"folderId":         nil,
					"imageCount":       count,
					"aspectRatio":      stringOr(input["aspectRatio"], "1:1"),
					"visualReferences": []string{},
				}
				if vr, ok := input["visualReferences"].([]any); ok && len(vr) > 0 {
					body["visualReferences"] = vr
				}
				if r, ok := input["resolution"].(string); ok && r != "" {
					body["resolution"] = r
				}
			} else {
				body = map[string]any{
					"prompt":            stringOr(input["prompt"], ""),
					"model":             newModel.Slug,
					"projectId":         "",
					"folderId":          nil,
					"videoCount":        intOr(input["videoCount"], 1),
					"duration":          intOr(input["duration"], (newModel.DurationMinSec+newModel.DurationMaxSec)/2),
					"aspectRatio":       stringOr(input["aspectRatio"], "16:9"),
					"resolution":        stringOr(input["resolution"], "720p"),
					"autoEnhancePrompt": false,
					"enableUnlimited":   true,
				}
				candidates := newModel.Resolutions
				if len(candidates) == 0 {
					candidates = newModel.PixelResolutions
				}
				currentRes, _ := body["resolution"].(string)
				if !modelSupports(candidates, currentRes) {
					if len(candidates) > 0 {
						body["resolution"] = candidates[0]
					}
				}
				d := body["duration"].(int)
				if d < newModel.DurationMinSec {
					body["duration"] = newModel.DurationMinSec
				} else if d > newModel.DurationMaxSec {
					body["duration"] = newModel.DurationMaxSec
				}
			}

			// Apply --bump overrides. "count" routes to imageCount on
			// image models / videoCount on video; "duration" is
			// video-only and errors with a helpful message on image
			// models rather than silently dropping.
			for _, b := range bumpFlags {
				k, v, ok := strings.Cut(b, "=")
				if !ok {
					return fmt.Errorf("invalid --bump %q (expected key=value)", b)
				}
				switch k {
				case "duration":
					if isImage {
						return fmt.Errorf("--bump duration is video-only; %s is an image model", newModel.Slug)
					}
					body["duration"] = mustAtoi(v)
				case "count", "videoCount", "imageCount":
					if isImage {
						body["imageCount"] = mustAtoi(v)
					} else {
						body["videoCount"] = mustAtoi(v)
					}
				case "resolution":
					body["resolution"] = v
				case "aspectRatio", "aspect-ratio":
					body["aspectRatio"] = v
				case "prompt":
					body["prompt"] = v
				default:
					return fmt.Errorf("unsupported --bump key %q", k)
				}
			}

			if cliutil.IsVerifyEnv() || flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"would_replay":  true,
					"model":         newModel.Slug,
					"capability_id": newModel.Capability(formType),
					"body":          body,
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			projectID, perr := resolveDefaultProject(c)
			if perr != nil {
				return fmt.Errorf("resolve project: %w", perr)
			}
			body["projectId"] = projectID

			capability := newModel.Capability(formType)
			path := "/forms/creations/" + url.PathEscape(capability)
			rawResp, status, err := c.Post(path, body)
			if err != nil {
				return err
			}
			if status >= 400 {
				return fmt.Errorf("submit HTTP %d: %s", status, summariseBody(rawResp))
			}
			var sub struct {
				HistoryID   string   `json:"historyId"`
				ResourceIDs []string `json:"resourceIds"`
			}
			_ = json.Unmarshal(rawResp, &sub)

			result := map[string]any{
				"original_resource":  resourceID,
				"replay_history_id":  sub.HistoryID,
				"replay_resources":   sub.ResourceIDs,
				"original_model":     origModelSlug,
				"replay_model":       newModel.Slug,
				"replay_capability":  capability,
				"submit_body":        body,
			}

			if !wait || len(sub.ResourceIDs) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 25*time.Minute)
			defer cancel()
			completions, _ := waitForResources(ctx, c, sub.ResourceIDs, 5*time.Second, cmd.ErrOrStderr())
			result["completions"] = completions
			if downloadDir != "" {
				downloads, _ := downloadCompletions(ctx, downloadDir, completions, cmd.ErrOrStderr())
				result["downloads"] = downloads
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&modelOverride, "model", "", "Replay on a different model (slug or shorthand)")
	cmd.Flags().StringArrayVar(&bumpFlags, "bump", nil, "Override params: --bump duration=10 --bump count=2")
	cmd.Flags().BoolVar(&wait, "wait", false, "Poll until completion")
	cmd.Flags().StringVar(&downloadDir, "download", "", "Download dir (implies --wait)")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if downloadDir != "" {
			wait = true
		}
		return nil
	}
	return cmd
}

func newPromptsTopCmd(flags *rootFlags) *cobra.Command {
	var (
		since string
		limit int
		by    string
	)
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Rank past prompts by total credit spend",
		Long: `Joins media (prompt text) with credits (CONSUME amounts) by history_id
and ranks prompt-hashes by total credits spent in the window.

Reveals "I have spent 8,000 credits iterating this dragon prompt" so you
can stop iterating it without thinking.`,
		Example: `  openart-pp-cli prompts top --since 30d
  openart-pp-cli prompts top --since 7d --limit 5 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openLocalStore()
			if err != nil {
				return fmt.Errorf("open local store: %w", err)
			}
			defer db.Close()

			cutoff := parseSince(since)
			cutoffStr := ""
			if !cutoff.IsZero() {
				cutoffStr = cutoff.Format("2006-01-02 15:04:05")
			}

			// Walk media + credits in-process and join by capabilityId+createdAt
			// proximity (the API doesn't link media→credit ledger entries
			// 1:1 explicitly, but createdAt + capability typically uniquely
			// identifies). Fall back to grouping by prompt text alone.
			mediaQ := `SELECT id, data, created_at, synced_at FROM media`
			rows, err := db.QueryContext(cmd.Context(), mediaQ)
			if err != nil {
				return err
			}
			defer rows.Close()

			type promptKey struct {
				prompt string
				model  string
			}
			counts := map[promptKey]int{} // events
			lastSeen := map[promptKey]string{}
			for rows.Next() {
				var id, syncedAt string
				var createdAt sql.NullInt64
				var data []byte
				if err := rows.Scan(&id, &data, &createdAt, &syncedAt); err != nil {
					continue
				}
				// PATCH: window on created_at (generation time) with a
				// synced_at fallback for rows without a payload createdAt,
				// mirroring stats and prompts find. Greptile P1 on PR #554.
				if cutoffStr != "" {
					if createdAt.Valid {
						if createdAt.Int64 < cutoff.UnixMilli() {
							continue
						}
					} else if syncedAt < cutoffStr {
						continue
					}
				}
				seenAt := syncedAt
				if createdAt.Valid {
					seenAt = time.UnixMilli(createdAt.Int64).UTC().Format("2006-01-02 15:04:05")
				}
				var blob map[string]any
				if json.Unmarshal(data, &blob) != nil {
					continue
				}
				input, _ := blob["input"].(map[string]any)
				if input == nil {
					continue
				}
				prompt, _ := input["prompt"].(string)
				if prompt == "" {
					continue
				}
				model, _ := input["model"].(string)
				k := promptKey{prompt, model}
				counts[k]++
				if seenAt > lastSeen[k] {
					lastSeen[k] = seenAt
				}
			}

			// Fallback spend = events × 200 (rough average) when we cannot
			// join 1:1; the credit-ledger pass below replaces this with the
			// actual aggregate when it can.
			spend := map[promptKey]int{}
			for k, n := range counts {
				if m := openartmodels.Resolve(k.model); m != nil && m.CreditsPerVideoDefault > 0 {
					spend[k] = m.CreditsPerVideoDefault * n
				} else {
					spend[k] = 200 * n
				}
			}

			// Aggregate from credits ledger by businessType=<model:form>.
			// PATCH: filter on first_seen_at — the timestamp the local
			// store first observed the consume event — instead of
			// synced_at, which is bumped on every resync and would
			// inflate the `--since` window to "everything ever synced"
			// after a full historical sync (greptile P1 on PR #554).
			creditQ := `SELECT amount, json_extract(data, '$.reference.businessType')
				FROM credits WHERE type = 'CONSUME'`
			if cutoffStr != "" {
				creditQ += ` AND first_seen_at >= '` + cutoffStr + `'`
			}
			cRows, err := db.QueryContext(cmd.Context(), creditQ)
			if err == nil {
				perModel := map[string]int{}
				for cRows.Next() {
					var amt sql.NullInt64
					var business sql.NullString
					if err := cRows.Scan(&amt, &business); err != nil {
						continue
					}
					if !business.Valid {
						continue
					}
					slug := business.String
					if i := strings.IndexByte(slug, ':'); i > 0 {
						slug = slug[:i]
					}
					perModel[slug] += -int(amt.Int64)
				}
				cRows.Close()
				// Replace the rough estimates with model-prorated actuals
				// when totals are available.
				modelEvents := map[string]int{}
				for k, n := range counts {
					modelEvents[k.model] += n
				}
				for k := range spend {
					if total, ok := perModel[k.model]; ok && modelEvents[k.model] > 0 {
						spend[k] = total * counts[k] / modelEvents[k.model]
					}
				}
			}

			type row struct {
				Prompt    string `json:"prompt"`
				Model     string `json:"model"`
				Events    int    `json:"events"`
				Credits   int    `json:"credits_spent"`
				LastSeen  string `json:"last_seen"`
			}
			out := make([]row, 0, len(counts))
			for k, n := range counts {
				out = append(out, row{
					Prompt:   truncateUtf8(k.prompt, 140),
					Model:    k.model,
					Events:   n,
					Credits:  spend[k],
					LastSeen: lastSeen[k],
				})
			}
			sort.SliceStable(out, func(i, j int) bool {
				switch strings.ToLower(by) {
				case "count", "events":
					return out[i].Events > out[j].Events
				case "recency", "recent":
					return out[i].LastSeen > out[j].LastSeen
				default: // spend
					return out[i].Credits > out[j].Credits
				}
			})
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"since":   since,
				"top":     out,
				"count":   len(out),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Window")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max prompts to return")
	cmd.Flags().StringVar(&by, "by", "spend", "Rank by: spend | count | recency")
	return cmd
}

func stringOr(v any, fallback string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return fallback
}

func intOr(v any, fallback int) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return fallback
}

func mustAtoi(s string) int {
	n, _ := atoiSafe(s)
	return n
}

func ftsLimit(want int) int {
	want *= 4
	if want < 100 {
		return 100
	}
	return want
}

func truncateUtf8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
