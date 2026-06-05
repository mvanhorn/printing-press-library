package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/twelvelabs/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/twelvelabs/internal/cliutil"
	"github.com/spf13/cobra"
)

type uploadVideoOptions struct {
	indexID           string
	file              string
	url               string
	metadata          []string
	enableVideoStream bool
	wait              bool
	waitTimeout       time.Duration
	pollInterval      time.Duration
}

type embedWorkflowOptions struct {
	videoFile    string
	videoURL     string
	model        string
	wait         bool
	waitTimeout  time.Duration
	pollInterval time.Duration
}

type briefOptions struct {
	videoID string
	format  string
	out     string
}

type clipsOptions struct {
	input string
	plan  string
	out   string
}

type editPlan struct {
	VideoID         string           `json:"video_id"`
	Title           string           `json:"title"`
	Topics          []string         `json:"topics"`
	Hashtags        []string         `json:"hashtags"`
	Chapters        []briefChapter   `json:"chapters"`
	Highlights      []briefHighlight `json:"highlights"`
	RecommendedCuts []recommendedCut `json:"recommended_cuts"`
}

type briefChapter struct {
	StartSec float64 `json:"start_sec"`
	EndSec   float64 `json:"end_sec"`
	Title    string  `json:"title"`
	Summary  string  `json:"summary"`
}

type briefHighlight struct {
	StartSec float64 `json:"start_sec"`
	EndSec   float64 `json:"end_sec"`
	Title    string  `json:"title"`
	Summary  string  `json:"summary"`
}

type recommendedCut struct {
	StartSec     float64 `json:"start_sec"`
	EndSec       float64 `json:"end_sec"`
	ClipTitle    string  `json:"clip_title"`
	Hook         string  `json:"hook"`
	WhyItMatters string  `json:"why_it_matters"`
	EditingNotes string  `json:"editing_notes"`
	CaptionSeed  string  `json:"caption_seed"`
}

func newUploadVideoCmd(flags *rootFlags) *cobra.Command {
	opts := uploadVideoOptions{enableVideoStream: true, waitTimeout: 30 * time.Minute, pollInterval: 5 * time.Second}
	cmd := &cobra.Command{
		Use:     "upload-video",
		Short:   "Upload or register a video and optionally wait for indexing",
		Example: "  twelvelabs-pp-cli upload-video --index-id IDX --file ./long-video.mp4 --wait --json --dry-run\n  twelvelabs-pp-cli upload-video --index-id IDX --url https://example.com/video.mp4 --wait --json --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 && !flags.dryRun {
				return cmd.Help()
			}
			return runUploadVideo(cmd, flags, opts)
		},
	}
	cmd.Flags().StringVar(&opts.indexID, "index-id", "", "Index ID that should receive the video")
	cmd.Flags().StringVar(&opts.file, "file", "", "Local video file to upload")
	cmd.Flags().StringVar(&opts.url, "url", "", "Public video URL to register")
	cmd.Flags().StringArrayVar(&opts.metadata, "metadata", nil, "Metadata key=value pair; repeat for multiple values")
	cmd.Flags().BoolVar(&opts.enableVideoStream, "enable-video-stream", true, "Store the video for streaming")
	cmd.Flags().BoolVar(&opts.wait, "wait", false, "Poll until indexing reaches a terminal status")
	cmd.Flags().DurationVar(&opts.waitTimeout, "wait-timeout", opts.waitTimeout, "Maximum time to wait for indexing")
	cmd.Flags().DurationVar(&opts.pollInterval, "poll-interval", opts.pollInterval, "Delay between indexing status checks")
	return cmd
}

func runUploadVideo(cmd *cobra.Command, flags *rootFlags, opts uploadVideoOptions) error {
	if strings.TrimSpace(opts.indexID) == "" && !flags.dryRun {
		return usageErr(fmt.Errorf("required flag %q not set", "index-id"))
	}
	if err := requireExactlyOne("video input", map[string]string{"file": opts.file, "url": opts.url}); err != nil && !flags.dryRun {
		return usageErr(err)
	}
	metadata, err := metadataJSON(opts.metadata)
	if err != nil {
		return usageErr(err)
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	fields := map[string]string{
		"enable_video_stream": strconv.FormatBool(opts.enableVideoStream),
		"index_id":            opts.indexID,
	}
	if metadata != "" {
		fields["user_metadata"] = metadata
	}
	fileFields := map[string]string{}
	if opts.file != "" {
		fileFields["video_file"] = opts.file
	}
	if opts.url != "" {
		fields["video_url"] = opts.url
	}
	data, status, err := c.PostMultipartWithParams(cmd.Context(), "/tasks", nil, fields, fileFields)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	out := map[string]any{
		"task_id":     extractString(data, "_id", "id", "task_id"),
		"index_id":    opts.indexID,
		"status":      extractString(data, "status"),
		"video_id":    extractString(data, "video_id"),
		"http_status": status,
	}
	if verifySynthetic(data) {
		out["verify_noop"] = true
	}
	if opts.wait && out["task_id"] != "" && !verifySynthetic(data) && !flags.dryRun {
		final, err := pollJSON(cmd.Context(), c, "/tasks/"+urlPathEscape(out["task_id"].(string)), opts.pollInterval, opts.waitTimeout, terminalTaskStatus)
		if err != nil {
			return err
		}
		out["status"] = extractString(final, "status")
		out["video_id"] = extractString(final, "video_id")
		out["task"] = decodeJSONMap(final)
	}
	return printJSONFiltered(cmd.OutOrStdout(), out, flags)
}

func configureEmbedWorkflowCmd(cmd *cobra.Command, flags *rootFlags) {
	opts := embedWorkflowOptions{model: "marengo3.0", waitTimeout: 30 * time.Minute, pollInterval: 5 * time.Second}
	cmd.Hidden = false
	cmd.Short = "Create or retrieve embeddings for video workflows"
	cmd.Example = "  twelvelabs-pp-cli embed --video-file ./long-video.mp4 --model marengo3.0 --wait --json --dry-run\n  twelvelabs-pp-cli embed --video-url https://example.com/video.mp4 --model marengo3.0 --wait --json --dry-run"
	cmd.Annotations = nil
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if workflowFlagsChanged(cmd, "video-file", "video-url", "model", "wait", "wait-timeout", "poll-interval") {
			return runEmbedWorkflow(cmd, flags, opts)
		}
		return parentNoSubcommandRunE(flags)(cmd, args)
	}
	cmd.Flags().StringVar(&opts.videoFile, "video-file", "", "Local video file to embed")
	cmd.Flags().StringVar(&opts.videoURL, "video-url", "", "Public video URL to embed")
	cmd.Flags().StringVar(&opts.model, "model", opts.model, "Embedding model name")
	cmd.Flags().BoolVar(&opts.wait, "wait", false, "Poll until embeddings are ready or failed")
	cmd.Flags().DurationVar(&opts.waitTimeout, "wait-timeout", opts.waitTimeout, "Maximum time to wait for embeddings")
	cmd.Flags().DurationVar(&opts.pollInterval, "poll-interval", opts.pollInterval, "Delay between embedding status checks")
}

func runEmbedWorkflow(cmd *cobra.Command, flags *rootFlags, opts embedWorkflowOptions) error {
	if err := requireExactlyOne("video input", map[string]string{"video-file": opts.videoFile, "video-url": opts.videoURL}); err != nil && !flags.dryRun {
		return usageErr(err)
	}
	if strings.TrimSpace(opts.model) == "" && !flags.dryRun {
		return usageErr(fmt.Errorf("required flag %q not set", "model"))
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	fields := map[string]string{"model_name": opts.model}
	fileFields := map[string]string{}
	if opts.videoFile != "" {
		fileFields["video_file"] = opts.videoFile
	}
	if opts.videoURL != "" {
		fields["video_url"] = opts.videoURL
	}
	data, status, err := c.PostMultipartWithParams(cmd.Context(), "/embed/tasks", nil, fields, fileFields)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	taskID := extractString(data, "_id", "id", "task_id")
	out := map[string]any{
		"task_id":     taskID,
		"status":      extractString(data, "status"),
		"model":       opts.model,
		"http_status": status,
	}
	if verifySynthetic(data) {
		out["verify_noop"] = true
	}
	if opts.wait && taskID != "" && !verifySynthetic(data) && !flags.dryRun {
		statusBody, err := pollJSON(cmd.Context(), c, "/embed/tasks/"+urlPathEscape(taskID)+"/status", opts.pollInterval, opts.waitTimeout, terminalTaskStatus)
		if err != nil {
			return err
		}
		out["status"] = extractString(statusBody, "status")
		out["status_response"] = decodeJSONMap(statusBody)
		if extractString(statusBody, "status") == "ready" {
			result, err := c.GetNoCache(cmd.Context(), "/embed/tasks/"+urlPathEscape(taskID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			out["result"] = decodeJSONMap(result)
		}
	}
	return printJSONFiltered(cmd.OutOrStdout(), out, flags)
}

func newVideoBriefCmd(flags *rootFlags) *cobra.Command {
	opts := briefOptions{format: "json"}
	cmd := &cobra.Command{
		Use:         "video-brief",
		Short:       "Create an editor-ready chapter, highlight, and cut plan",
		Example:     "  twelvelabs-pp-cli video-brief --video-id VID --format json --json --dry-run\n  twelvelabs-pp-cli video-brief --video-id VID --format markdown --dry-run",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 && !flags.dryRun {
				return cmd.Help()
			}
			return runVideoBrief(cmd, flags, opts)
		},
	}
	cmd.Flags().StringVar(&opts.videoID, "video-id", "", "Video ID to analyze")
	cmd.Flags().StringVar(&opts.format, "format", opts.format, "Output format: json or markdown")
	cmd.Flags().StringVar(&opts.out, "out", "", "Write the brief to this file instead of stdout")
	return cmd
}

func runVideoBrief(cmd *cobra.Command, flags *rootFlags, opts briefOptions) error {
	if strings.TrimSpace(opts.videoID) == "" && !flags.dryRun {
		return usageErr(fmt.Errorf("required flag %q not set", "video-id"))
	}
	switch opts.format {
	case "json", "markdown":
	default:
		return usageErr(fmt.Errorf("invalid --format %q: must be json or markdown", opts.format))
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	plan := editPlan{VideoID: opts.videoID, Topics: []string{}, Hashtags: []string{}, Chapters: []briefChapter{}, Highlights: []briefHighlight{}, RecommendedCuts: []recommendedCut{}}
	if !flags.dryRun {
		built, err := buildLegacyBriefPlan(cmd.Context(), c, opts.videoID)
		if err != nil && isDeprecatedEndpointError(err) {
			built, err = buildAnalyzeBriefPlan(cmd.Context(), c, opts.videoID)
		}
		if err != nil {
			return classifyAPIError(err, flags)
		}
		plan = built
	}
	if opts.format == "markdown" {
		return writeBriefOutput(cmd, opts.out, []byte(renderBriefMarkdown(plan)))
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return writeBriefOutput(cmd, opts.out, append(data, '\n'))
}

func newClipsCmd(flags *rootFlags) *cobra.Command {
	opts := clipsOptions{}
	cmd := &cobra.Command{
		Use:     "clips",
		Short:   "Cut local clips from a video-brief JSON plan",
		Example: "  twelvelabs-pp-cli clips --dry-run --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 && !flags.dryRun {
				return cmd.Help()
			}
			return runClips(cmd, flags, opts)
		},
	}
	cmd.Flags().StringVar(&opts.input, "input", "", "Source video file")
	cmd.Flags().StringVar(&opts.plan, "plan", "", "video-brief JSON plan")
	cmd.Flags().StringVar(&opts.out, "out", "", "Directory for generated clips")
	return cmd
}

func runClips(cmd *cobra.Command, flags *rootFlags, opts clipsOptions) error {
	if flags.dryRun {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "clips": 0}, flags)
	}
	if opts.input == "" || opts.plan == "" || opts.out == "" {
		return usageErr(fmt.Errorf("--input, --plan, and --out are required"))
	}
	data, err := os.ReadFile(opts.plan)
	if err != nil {
		return fmt.Errorf("reading plan: %w", err)
	}
	var plan editPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return fmt.Errorf("parsing plan JSON: %w", err)
	}
	if len(plan.RecommendedCuts) == 0 {
		return fmt.Errorf("plan contains no recommended_cuts")
	}
	if cliutil.IsVerifyEnv() {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"clips": len(plan.RecommendedCuts), "verify_noop": true}, flags)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg is required to cut clips; install it and retry")
	}
	if err := os.MkdirAll(opts.out, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	written := []string{}
	for i, cut := range plan.RecommendedCuts {
		if cut.StartSec < 0 || cut.EndSec <= cut.StartSec {
			return fmt.Errorf("recommended_cuts[%d] has invalid timestamps", i)
		}
		name := fmt.Sprintf("%02d-%s.mp4", i+1, sanitizeClipName(cut.ClipTitle))
		outPath := filepath.Join(opts.out, name)
		args := []string{"-y", "-ss", formatSeconds(cut.StartSec), "-to", formatSeconds(cut.EndSec), "-i", opts.input, "-c", "copy", outPath}
		if output, err := exec.CommandContext(cmd.Context(), "ffmpeg", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("ffmpeg failed for %s: %w\n%s", name, err, strings.TrimSpace(string(output)))
		}
		written = append(written, outPath)
	}
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"clips": written}, flags)
}

func postJSON(ctx context.Context, c *client.Client, path string, body map[string]any) (json.RawMessage, error) {
	data, _, err := c.PostWithParams(ctx, path, nil, body)
	return data, err
}

func buildLegacyBriefPlan(ctx context.Context, c *client.Client, videoID string) (editPlan, error) {
	plan := emptyEditPlan(videoID)
	chapterBody, err := postJSON(ctx, c, "/summarize", map[string]any{"video_id": videoID, "type": "chapter", "prompt": "Return a chronological chapter breakdown with concise editor-facing titles and summaries.", "temperature": 0.2})
	if err != nil {
		return plan, err
	}
	plan.Chapters = extractChapters(chapterBody)
	highlightBody, err := postJSON(ctx, c, "/summarize", map[string]any{"video_id": videoID, "type": "highlight", "prompt": "Identify the strongest moments for short-form or editorial reuse. Keep summaries practical for an editor.", "temperature": 0.2})
	if err != nil {
		return plan, err
	}
	plan.Highlights = extractHighlights(highlightBody)
	gistBody, err := postJSON(ctx, c, "/gist", map[string]any{"video_id": videoID, "types": []string{"title", "topic", "hashtag"}})
	if err != nil {
		return plan, err
	}
	plan.Title = extractString(gistBody, "title")
	plan.Topics = extractStringSlice(gistBody, "topics")
	plan.Hashtags = extractStringSlice(gistBody, "hashtags")
	analyzeBody, err := postJSON(ctx, c, "/analyze", map[string]any{
		"video_id":        videoID,
		"prompt":          analyzePrompt(),
		"temperature":     0.2,
		"stream":          false,
		"max_tokens":      3000,
		"response_format": map[string]any{"type": "json_schema", "json_schema": recommendedCutsSchema()},
	})
	if err != nil {
		return plan, err
	}
	plan.RecommendedCuts = extractRecommendedCuts(analyzeBody)
	return plan, nil
}

func buildAnalyzeBriefPlan(ctx context.Context, c *client.Client, videoID string) (editPlan, error) {
	body, err := postJSON(ctx, c, "/analyze", map[string]any{
		"video_id":        videoID,
		"prompt":          fullBriefPrompt(),
		"temperature":     0.2,
		"stream":          false,
		"max_tokens":      4096,
		"response_format": map[string]any{"type": "json_schema", "json_schema": fullBriefSchema()},
	})
	if err != nil {
		return emptyEditPlan(videoID), err
	}
	plan := extractAnalyzePlan(videoID, body)
	if plan.VideoID == "" {
		plan.VideoID = videoID
	}
	return plan, nil
}

func emptyEditPlan(videoID string) editPlan {
	return editPlan{VideoID: videoID, Topics: []string{}, Hashtags: []string{}, Chapters: []briefChapter{}, Highlights: []briefHighlight{}, RecommendedCuts: []recommendedCut{}}
}

func isDeprecatedEndpointError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "HTTP 410") || strings.Contains(msg, "endpoint_deprecated")
}

func pollJSON(ctx context.Context, c *client.Client, path string, interval, timeout time.Duration, terminal func(string) bool) (json.RawMessage, error) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		data, err := c.GetNoCache(pollCtx, path, nil)
		if err != nil {
			return nil, classifyAPIError(err, nil)
		}
		if terminal(extractString(data, "status")) {
			return data, nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-pollCtx.Done():
			timer.Stop()
			return nil, fmt.Errorf("timed out waiting for %s", path)
		case <-timer.C:
		}
	}
}

func terminalTaskStatus(status string) bool {
	switch strings.ToLower(status) {
	case "ready", "failed", "completed", "complete", "done", "error", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func requireExactlyOne(label string, values map[string]string) error {
	set := []string{}
	for key, value := range values {
		if strings.TrimSpace(value) != "" {
			set = append(set, "--"+key)
		}
	}
	sort.Strings(set)
	if len(set) != 1 {
		return fmt.Errorf("provide exactly one %s (%s)", label, strings.Join(sortedFlagNames(values), " or "))
	}
	return nil
}

func sortedFlagNames(values map[string]string) []string {
	names := make([]string, 0, len(values))
	for key := range values {
		names = append(names, "--"+key)
	}
	sort.Strings(names)
	return names
}

func metadataJSON(entries []string) (string, error) {
	if len(entries) == 0 {
		return "", nil
	}
	out := map[string]string{}
	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return "", fmt.Errorf("--metadata must be key=value, got %q", entry)
		}
		out[strings.TrimSpace(key)] = value
	}
	data, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func workflowFlagsChanged(cmd *cobra.Command, names ...string) bool {
	for _, name := range names {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

func decodeJSONMap(data json.RawMessage) map[string]any {
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func verifySynthetic(data json.RawMessage) bool {
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return false
	}
	v, _ := out["__pp_verify_synthetic__"].(bool)
	return v
}

func extractString(data json.RawMessage, keys ...string) string {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return ""
	}
	for _, key := range keys {
		if found, ok := findKey(value, key); ok {
			switch v := found.(type) {
			case string:
				return v
			case float64:
				return strconv.FormatFloat(v, 'f', -1, 64)
			}
		}
	}
	return ""
}

func extractStringSlice(data json.RawMessage, key string) []string {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return []string{}
	}
	found, ok := findKey(value, key)
	if !ok {
		return []string{}
	}
	items, ok := found.([]any)
	if !ok {
		return []string{}
	}
	out := []string{}
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

const maxJSONSearchDepth = 20

func findKey(value any, key string) (any, bool) {
	return findKeyDepth(value, key, 0)
}

func findKeyDepth(value any, key string, depth int) (any, bool) {
	if depth > maxJSONSearchDepth {
		return nil, false
	}
	switch v := value.(type) {
	case map[string]any:
		if found, ok := v[key]; ok {
			return found, true
		}
		for _, child := range v {
			if found, ok := findKeyDepth(child, key, depth+1); ok {
				return found, true
			}
		}
	case []any:
		for _, child := range v {
			if found, ok := findKeyDepth(child, key, depth+1); ok {
				return found, true
			}
		}
	}
	return nil, false
}

func extractChapters(data json.RawMessage) []briefChapter {
	items := extractArray(data, "chapters")
	out := []briefChapter{}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, briefChapter{
			StartSec: numberField(m, "start_sec", "start"),
			EndSec:   numberField(m, "end_sec", "end"),
			Title:    stringField(m, "chapter_title", "title"),
			Summary:  stringField(m, "chapter_summary", "summary"),
		})
	}
	return out
}

func extractHighlights(data json.RawMessage) []briefHighlight {
	items := extractArray(data, "highlights")
	out := []briefHighlight{}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, briefHighlight{
			StartSec: numberField(m, "start_sec", "start"),
			EndSec:   numberField(m, "end_sec", "end"),
			Title:    stringField(m, "highlight", "title"),
			Summary:  stringField(m, "highlight_summary", "summary"),
		})
	}
	return out
}

func extractRecommendedCuts(data json.RawMessage) []recommendedCut {
	candidate := unwrapAnalyzeJSON(data)
	items := extractArray(candidate, "recommended_cuts")
	out := []recommendedCut{}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		start := numberField(m, "start_sec")
		end := numberField(m, "end_sec")
		if end <= start {
			continue
		}
		out = append(out, recommendedCut{
			StartSec:     start,
			EndSec:       end,
			ClipTitle:    stringField(m, "clip_title", "title"),
			Hook:         stringField(m, "hook"),
			WhyItMatters: stringField(m, "why_it_matters", "why"),
			EditingNotes: stringField(m, "editing_notes", "notes"),
			CaptionSeed:  stringField(m, "caption_seed", "caption"),
		})
	}
	return out
}

func extractAnalyzePlan(videoID string, data json.RawMessage) editPlan {
	candidate := unwrapAnalyzeJSON(data)
	plan := emptyEditPlan(videoID)
	if id := extractString(candidate, "video_id"); id != "" {
		plan.VideoID = id
	}
	plan.Title = extractString(candidate, "title")
	plan.Topics = extractStringSlice(candidate, "topics")
	plan.Hashtags = extractStringSlice(candidate, "hashtags")
	plan.Chapters = extractChapters(candidate)
	plan.Highlights = extractHighlights(candidate)
	plan.RecommendedCuts = extractRecommendedCuts(candidate)
	return plan
}

func unwrapAnalyzeJSON(data json.RawMessage) json.RawMessage {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return data
	}
	if _, ok := value.(map[string]any); ok {
		for _, key := range []string{"data", "text", "answer", "response"} {
			if found, ok := findKey(value, key); ok {
				if s, ok := found.(string); ok && json.Valid([]byte(s)) {
					return json.RawMessage(s)
				}
				if m, ok := found.(map[string]any); ok {
					if encoded, err := json.Marshal(m); err == nil {
						return encoded
					}
				}
			}
		}
	}
	return data
}

func extractArray(data json.RawMessage, key string) []any {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil
	}
	found, ok := findKey(value, key)
	if !ok {
		return nil
	}
	items, _ := found.([]any)
	return items
}

func numberField(m map[string]any, keys ...string) float64 {
	for _, key := range keys {
		switch v := m[key].(type) {
		case float64:
			if !math.IsNaN(v) && !math.IsInf(v, 0) {
				return v
			}
		case string:
			if n, err := strconv.ParseFloat(v, 64); err == nil {
				return n
			}
		}
	}
	return 0
}

func stringField(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s, ok := m[key].(string); ok {
			return s
		}
	}
	return ""
}

func analyzePrompt() string {
	return "Identify the best moments in this long-form video for an editor. Return only JSON that matches the schema. For each recommended cut, use timestamps only when the video supports them, pick concise clip titles, explain why the moment is interesting, suggest pacing or caption notes, provide a hook or first-line idea, and include a caption seed. The guidance should work for interviews, tutorials, demos, talks, podcasts, documentaries, and social clips."
}

func fullBriefPrompt() string {
	return "Create an editor-ready plan for this video. Return only JSON that matches the schema. Include a concise title, topics, hashtags, chronological chapters, highlights, and recommended cuts. Use timestamps only when they are derived from the video. Do not fill unknown timestamps with guesses. The recommendations should be useful for long-form content, interviews, tutorials, demos, talks, podcasts, and social clips."
}

func recommendedCutsSchema() map[string]any {
	cut := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"start_sec":      map[string]any{"type": "number"},
			"end_sec":        map[string]any{"type": "number"},
			"clip_title":     map[string]any{"type": "string"},
			"hook":           map[string]any{"type": "string"},
			"why_it_matters": map[string]any{"type": "string"},
			"editing_notes":  map[string]any{"type": "string"},
			"caption_seed":   map[string]any{"type": "string"},
		},
		"required":             []string{"start_sec", "end_sec", "clip_title", "hook", "why_it_matters", "editing_notes", "caption_seed"},
		"additionalProperties": false,
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"recommended_cuts": map[string]any{"type": "array", "items": cut},
		},
		"required":             []string{"recommended_cuts"},
		"additionalProperties": false,
	}
}

func fullBriefSchema() map[string]any {
	chapter := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"start_sec": map[string]any{"type": "number"},
			"end_sec":   map[string]any{"type": "number"},
			"title":     map[string]any{"type": "string"},
			"summary":   map[string]any{"type": "string"},
		},
		"required":             []string{"start_sec", "end_sec", "title", "summary"},
		"additionalProperties": false,
	}
	highlight := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"start_sec": map[string]any{"type": "number"},
			"end_sec":   map[string]any{"type": "number"},
			"title":     map[string]any{"type": "string"},
			"summary":   map[string]any{"type": "string"},
		},
		"required":             []string{"start_sec", "end_sec", "title", "summary"},
		"additionalProperties": false,
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"video_id":         map[string]any{"type": "string"},
			"title":            map[string]any{"type": "string"},
			"topics":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"hashtags":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"chapters":         map[string]any{"type": "array", "items": chapter},
			"highlights":       map[string]any{"type": "array", "items": highlight},
			"recommended_cuts": recommendedCutsSchema()["properties"].(map[string]any)["recommended_cuts"],
		},
		"required":             []string{"video_id", "title", "topics", "hashtags", "chapters", "highlights", "recommended_cuts"},
		"additionalProperties": false,
	}
}

func renderBriefMarkdown(plan editPlan) string {
	var b strings.Builder
	title := plan.Title
	if title == "" {
		title = "Video Brief"
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "Video ID: `%s`\n\n", plan.VideoID)
	if len(plan.Topics) > 0 {
		fmt.Fprintf(&b, "Topics: %s\n\n", strings.Join(plan.Topics, ", "))
	}
	if len(plan.Hashtags) > 0 {
		fmt.Fprintf(&b, "Hashtags: %s\n\n", strings.Join(plan.Hashtags, " "))
	}
	b.WriteString("## Chapters\n\n")
	for _, ch := range plan.Chapters {
		fmt.Fprintf(&b, "- %s-%s: **%s** - %s\n", formatSeconds(ch.StartSec), formatSeconds(ch.EndSec), ch.Title, ch.Summary)
	}
	b.WriteString("\n## Highlights\n\n")
	for _, h := range plan.Highlights {
		fmt.Fprintf(&b, "- %s-%s: **%s** - %s\n", formatSeconds(h.StartSec), formatSeconds(h.EndSec), h.Title, h.Summary)
	}
	b.WriteString("\n## Recommended Cuts\n\n")
	for _, cut := range plan.RecommendedCuts {
		fmt.Fprintf(&b, "### %s (%s-%s)\n\n", cut.ClipTitle, formatSeconds(cut.StartSec), formatSeconds(cut.EndSec))
		fmt.Fprintf(&b, "- Hook: %s\n", cut.Hook)
		fmt.Fprintf(&b, "- Why it matters: %s\n", cut.WhyItMatters)
		fmt.Fprintf(&b, "- Editing notes: %s\n", cut.EditingNotes)
		fmt.Fprintf(&b, "- Caption seed: %s\n\n", cut.CaptionSeed)
	}
	return b.String()
}

func writeBriefOutput(cmd *cobra.Command, outPath string, data []byte) error {
	if outPath == "" {
		_, err := cmd.OutOrStdout().Write(data)
		return err
	}
	return os.WriteFile(outPath, data, 0o644)
}

func sanitizeClipName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		name = "clip"
	}
	re := regexp.MustCompile(`[^a-z0-9]+`)
	name = strings.Trim(re.ReplaceAllString(name, "-"), "-")
	if name == "" {
		return "clip"
	}
	if len(name) > 60 {
		name = strings.Trim(name[:60], "-")
	}
	return name
}

func formatSeconds(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}

func urlPathEscape(value string) string {
	return url.PathEscape(value)
}
