// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/client"
	"github.com/spf13/cobra"
)

func newVideosClipsInsertFileCmd(flags *rootFlags) *cobra.Command {
	var width, height int
	var duration float64
	var name, contentType string
	cmd := &cobra.Command{
		Use:   "insert-file <videoId> <path>",
		Short: "Create a source, upload a local video file, and add it as a clip",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoID, filePath := args[0], args[1]
			steps := []map[string]any{
				{"method": "POST", "path": "/v1/sources", "body": sourceCreateBody(width, height, duration)},
				{"method": "PUT", "target": "<uploadUrl>", "file": filePath, "content_type": contentType},
				{"method": "POST", "path": fmt.Sprintf("/v1/videos/%s/clips", videoID), "body": map[string]any{"sourceId": "<sourceId>", "name": name}},
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "workflow": "videos clips insert-file", "steps": steps}, flags)
			}
			if width == 0 || height == 0 || duration == 0 {
				return usageErr(fmt.Errorf("--width, --height, and --duration are required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			created, uploadStatus, bytesUploaded, err := createAndUploadSource(c, filePath, width, height, duration, contentType)
			if err != nil {
				return err
			}
			body := map[string]any{"sourceId": created.SourceID}
			if name != "" {
				body["name"] = name
			}
			data, status, err := c.Post(fmt.Sprintf("/v1/videos/%s/clips", videoID), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"success": status >= 200 && status < 300, "status": status, "upload_status": uploadStatus, "bytes": bytesUploaded, "sourceId": created.SourceID, "clip": jsonRawToAny(data)}, flags)
		},
	}
	cmd.Flags().IntVar(&width, "width", 0, "Source width in pixels")
	cmd.Flags().IntVar(&height, "height", 0, "Source height in pixels")
	cmd.Flags().Float64Var(&duration, "duration", 0, "Source duration in seconds")
	cmd.Flags().StringVar(&name, "name", "", "Clip name")
	cmd.Flags().StringVar(&contentType, "content-type", "video/mp4", "Upload content type")
	return cmd
}

func newVideosClipsAddBrollCmd(flags *rootFlags) *cobra.Command {
	var width, height, startMs, durationMs int
	var duration float64
	var layout, transition, contentType string
	cmd := &cobra.Command{
		Use:   "add-broll <videoId> <clipId> <path>",
		Short: "Upload a local video file and attach it as B-roll layout media",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoID, clipID, filePath := args[0], args[1], args[2]
			if layout == "" {
				layout = `{"kind":"fullscreen"}`
			}
			var layoutObj any
			if err := json.Unmarshal([]byte(layout), &layoutObj); err != nil {
				return fmt.Errorf("parsing --layout JSON: %w", err)
			}
			if durationMs == 0 {
				return usageErr(fmt.Errorf("--duration-ms is required for B-roll media layouts"))
			}
			body := map[string]any{"layout": layoutObj, "media": map[string]any{"type": "video", "sourceId": "<sourceId>"}, "startTimeMs": startMs, "durationMs": durationMs}
			if transition != "" {
				body["transitionStyle"] = transition
			}
			steps := []map[string]any{{"method": "POST", "path": "/v1/sources", "body": sourceCreateBody(width, height, duration)}, {"method": "PUT", "target": "<uploadUrl>", "file": filePath}, {"method": "POST", "path": fmt.Sprintf("/v1/videos/%s/clips/%s/layouts", videoID, clipID), "body": body}}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "workflow": "videos clips add-broll", "steps": steps}, flags)
			}
			if width == 0 || height == 0 || duration == 0 {
				return usageErr(fmt.Errorf("--width, --height, and --duration are required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			created, uploadStatus, bytesUploaded, err := createAndUploadSource(c, filePath, width, height, duration, contentType)
			if err != nil {
				return err
			}
			body["media"] = map[string]any{"type": "video", "sourceId": created.SourceID}
			data, status, err := c.Post(fmt.Sprintf("/v1/videos/%s/clips/%s/layouts", videoID, clipID), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"success": status >= 200 && status < 300, "status": status, "upload_status": uploadStatus, "bytes": bytesUploaded, "sourceId": created.SourceID, "layout": jsonRawToAny(data)}, flags)
		},
	}
	cmd.Flags().IntVar(&width, "width", 0, "Source width in pixels")
	cmd.Flags().IntVar(&height, "height", 0, "Source height in pixels")
	cmd.Flags().Float64Var(&duration, "duration", 0, "Source duration in seconds")
	cmd.Flags().IntVar(&startMs, "start-ms", 0, "Layout start time in ms")
	cmd.Flags().IntVar(&durationMs, "duration-ms", 0, "Layout duration in ms")
	cmd.Flags().StringVar(&layout, "layout", `{"kind":"fullscreen"}`, "Layout JSON")
	cmd.Flags().StringVar(&transition, "transition-style", "", "Optional transition style")
	cmd.Flags().StringVar(&contentType, "content-type", "video/mp4", "Upload content type")
	return cmd
}

func newVideosClipsSilenceMapCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "silence-map <videoId> <clipId>", Short: "Combine silences, waveform, and transcript into an edit timeline", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		vid, cid := args[0], args[1]
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "reads": []string{"silences", "source waveform", "uncut transcript"}}, flags)
		}
		sil, err := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/silences", vid, cid), nil)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		tr, err := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/transcript/uncut", vid, cid), nil)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		sources, err := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/sources", vid, cid), nil)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"video_id": vid, "clip_id": cid, "silences": jsonRawToAny(sil), "transcript": jsonRawToAny(tr), "sources": jsonRawToAny(sources)}, flags)
	}}
	return cmd
}

func newVideosClipsCutWordsCmd(flags *rootFlags) *cobra.Command {
	var term string
	var apply bool
	cmd := &cobra.Command{Use: "cut-words <videoId> <clipId>", Short: "Find transcript words matching a term and cut their word ranges", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if term == "" {
			return usageErr(fmt.Errorf("--term is required"))
		}
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "apply": false, "term": term, "steps": []map[string]any{{"method": "GET", "path": fmt.Sprintf("/v1/videos/%s/clips/%s/transcript/uncut", args[0], args[1])}, {"method": "POST", "path": fmt.Sprintf("/v1/videos/%s/clips/%s/cut-by-transcript", args[0], args[1]), "body": "<wordRanges matching term>"}}}, flags)
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		ranges, err := transcriptTermRanges(c, args[0], args[1], term)
		if err != nil {
			return err
		}
		body := map[string]any{"wordRanges": ranges}
		if !apply {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "apply": false, "term": term, "body": body}, flags)
		}
		data, status, err := c.Post(fmt.Sprintf("/v1/videos/%s/clips/%s/cut-by-transcript", args[0], args[1]), body)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"success": status >= 200 && status < 300, "status": status, "data": jsonRawToAny(data)}, flags)
	}}
	cmd.Flags().StringVar(&term, "term", "", "Case-insensitive word/text to cut")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually apply cuts; default prints a plan")
	return cmd
}

func newVideosClipsReplaceWordRangesCmd(flags *rootFlags) *cobra.Command {
	var wordRanges, insertFile, name, contentType string
	var width, height int
	var duration float64
	var apply bool
	cmd := &cobra.Command{Use: "replace-word-ranges <videoId> <clipId>", Short: "Cut transcript word ranges and optionally insert a replacement clip", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if wordRanges == "" {
			return usageErr(fmt.Errorf("required flag %q not set", "word-ranges"))
		}
		body, err := cutByTranscriptBody(false, wordRanges, nil)
		if err != nil {
			return err
		}
		if apply {
			if err := validateReplacementSourceInput(insertFile, width, height, duration); err != nil {
				return err
			}
		}
		steps := []map[string]any{{"method": "POST", "path": fmt.Sprintf("/v1/videos/%s/clips/%s/cut-by-transcript", args[0], args[1]), "body": body}}
		if insertFile != "" {
			steps = append(steps, map[string]any{"workflow": "insert-file", "file": insertFile, "name": name})
		}
		if dryRunOK(flags) || !apply {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "apply": false, "steps": steps}, flags)
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		data, status, err := c.Post(fmt.Sprintf("/v1/videos/%s/clips/%s/cut-by-transcript", args[0], args[1]), body)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		out := map[string]any{"cut_status": status, "cut": jsonRawToAny(data)}
		if insertFile != "" {
			created, up, b, err := createAndUploadSource(c, insertFile, width, height, duration, contentType)
			if err != nil {
				return err
			}
			clipBody := map[string]any{"sourceId": created.SourceID}
			if name != "" {
				clipBody["name"] = name
			}
			cd, cs, err := c.Post(fmt.Sprintf("/v1/videos/%s/clips", args[0]), clipBody)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			out["replacement"] = map[string]any{"status": cs, "upload_status": up, "bytes": b, "sourceId": created.SourceID, "clip": jsonRawToAny(cd)}
		}
		return printJSONFiltered(cmd.OutOrStdout(), out, flags)
	}}
	cmd.Flags().StringVar(&wordRanges, "word-ranges", "", "JSON array of {fromWordIndex,toWordIndex}")
	cmd.Flags().StringVar(&insertFile, "insert-file", "", "Optional replacement video file to upload and add as a new clip")
	cmd.Flags().StringVar(&name, "name", "", "Replacement clip name")
	cmd.Flags().IntVar(&width, "width", 0, "Replacement source width")
	cmd.Flags().IntVar(&height, "height", 0, "Replacement source height")
	cmd.Flags().Float64Var(&duration, "duration", 0, "Replacement source duration seconds")
	cmd.Flags().StringVar(&contentType, "content-type", "video/mp4", "Upload content type")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually apply replacement; default prints a plan")
	return cmd
}

func newVideosStoryboardCmd(flags *rootFlags) *cobra.Command {
	var includeTranscript bool
	cmd := &cobra.Command{Use: "storyboard <videoId>", Short: "Dump video, clips, cuts, layouts, effects, and transcript snippets into one JSON timeline", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		vid := args[0]
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "reads": []string{"video", "clips", "layouts", "zooms", "blurs", "highlights", "transcripts"}}, flags)
		}
		video, _ := c.Get("/v1/videos/"+vid, nil)
		clipsRaw, err := c.Get("/v1/videos/"+vid+"/clips", nil)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		clipIDs := extractAnyIDs(clipsRaw)
		clips := []map[string]any{}
		for _, cid := range clipIDs {
			item := map[string]any{"id": cid}
			for _, res := range []string{"layouts", "zooms", "blurs", "highlights", "sources"} {
				raw, _ := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/%s", vid, cid, res), nil)
				item[res] = jsonRawToAny(raw)
			}
			if includeTranscript {
				raw, _ := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/transcript/uncut", vid, cid), nil)
				item["uncutTranscript"] = jsonRawToAny(raw)
			}
			clips = append(clips, item)
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"video_id": vid, "video": jsonRawToAny(video), "clips": clips}, flags)
	}}
	cmd.Flags().BoolVar(&includeTranscript, "include-transcript", false, "Include uncut transcript for each clip")
	return cmd
}

func newVideosApplyStoryboardCmd(flags *rootFlags) *cobra.Command {
	var file string
	var apply bool
	cmd := &cobra.Command{Use: "apply-storyboard <videoId>", Short: "Apply a declarative JSON edit plan to video/clips/effects", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if file == "" {
			return usageErr(fmt.Errorf("--file is required"))
		}
		plan, err := readJSONFile(file)
		if err != nil {
			return err
		}
		steps := storyboardSteps(args[0], plan)
		if dryRunOK(flags) || !apply {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "apply": false, "steps": steps}, flags)
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		results := []map[string]any{}
		failedCount := 0
		for _, step := range steps {
			data, status, err := applyStoryboardStep(c, step)
			r := map[string]any{"step": step, "status": status}
			if err != nil {
				failedCount++
				r["error"] = err.Error()
			} else {
				r["data"] = jsonRawToAny(data)
			}
			results = append(results, r)
		}
		out := map[string]any{"applied": failedCount == 0, "failed_steps": failedCount, "results": results}
		if failedCount > 0 {
			if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
				return err
			}
			return fmt.Errorf("apply-storyboard failed %d step(s)", failedCount)
		}
		return printJSONFiltered(cmd.OutOrStdout(), out, flags)
	}}
	cmd.Flags().StringVar(&file, "file", "", "Storyboard JSON file")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually apply; default prints a plan")
	return cmd
}

func newVideosFormatCmd(flags *rootFlags) *cobra.Command {
	var preset string
	var apply bool
	cmd := &cobra.Command{Use: "format <videoId>", Short: "Apply preset dimensions plus viewer/caption/download settings", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		body := formatPresetBody(preset)
		if dryRunOK(flags) || !apply {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "body": body}, flags)
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		data, status, err := c.Patch("/v1/videos/"+args[0], body)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"status": status, "data": jsonRawToAny(data)}, flags)
	}}
	cmd.Flags().StringVar(&preset, "preset", "youtube", "Preset: youtube, shorts, square, linkedin")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually apply; default prints plan")
	return cmd
}

func newVideosAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "audit <videoId>", Short: "Detect likely pre-publish issues in a Tella video", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		vid := args[0]
		clipsRaw, err := c.Get("/v1/videos/"+vid+"/clips", nil)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		issues := []map[string]any{}
		for _, cid := range extractAnyIDs(clipsRaw) {
			sil, silErr := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/silences", vid, cid), nil)
			if silErr != nil {
				issues = append(issues, map[string]any{"clip_id": cid, "type": "silences_unavailable", "error": truncate(silErr.Error(), 200)})
			} else {
				for _, r := range extractSilenceRanges(sil) {
					if r.End-r.Start > 3000 {
						issues = append(issues, map[string]any{"clip_id": cid, "type": "long_silence", "fromMs": r.Start, "toMs": r.End})
					}
				}
			}
			tr, trErr := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/transcript/uncut", vid, cid), nil)
			if trErr != nil {
				issues = append(issues, map[string]any{"clip_id": cid, "type": "transcript_unavailable", "error": truncate(trErr.Error(), 200)})
			} else if transcriptLooksEmpty(tr) {
				issues = append(issues, map[string]any{"clip_id": cid, "type": "empty_transcript"})
			}
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"video_id": vid, "issues": issues, "issue_count": len(issues)}, flags)
	}}
	return cmd
}

func newPlaylistsEditPassCmd(flags *rootFlags) *cobra.Command {
	var removeFillers, removeBuffers, trimEdges, export bool
	cmd := &cobra.Command{Use: "edit-pass <playlistId>", Short: "Plan/apply a standard edit recipe across every video in a playlist", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		vids, err := listPlaylistVideoIDs(c, args[0])
		if err != nil {
			return classifyAPIError(err, flags)
		}
		steps := []map[string]any{}
		for _, vid := range vids {
			clips, err := listClipIDs(c, vid)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			for _, cid := range clips {
				if removeFillers {
					steps = append(steps, map[string]any{"command": "videos clips remove-fillers", "video_id": vid, "clip_id": cid})
				}
				if removeBuffers {
					steps = append(steps, map[string]any{"command": "videos clips remove-buffers", "video_id": vid, "clip_id": cid})
				}
				if trimEdges {
					steps = append(steps, map[string]any{"command": "videos clips trim-edges", "video_id": vid, "clip_id": cid})
				}
			}
			if export {
				steps = append(steps, map[string]any{"command": "videos exports video", "video_id": vid})
			}
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"playlist_id": args[0], "dry_run": true, "planned": steps}, flags)
	}}
	cmd.Flags().BoolVar(&removeFillers, "remove-fillers", false, "Plan filler removal")
	cmd.Flags().BoolVar(&removeBuffers, "remove-buffers", false, "Plan buffer removal")
	cmd.Flags().BoolVar(&trimEdges, "trim-edges", false, "Plan edge trimming")
	cmd.Flags().BoolVar(&export, "export", false, "Plan export after edits")
	return cmd
}

func validateReplacementSourceInput(filePath string, width, height int, duration float64) error {
	if filePath == "" {
		return nil
	}
	if width == 0 || height == 0 || duration == 0 {
		return usageErr(fmt.Errorf("--width, --height, and --duration are required with --insert-file"))
	}
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening --insert-file before cutting transcript: %w", err)
	}
	return f.Close()
}

func sourceCreateBody(width, height int, duration float64) map[string]any {
	b := map[string]any{"kind": "video"}
	if width != 0 {
		b["width"] = width
	}
	if height != 0 {
		b["height"] = height
	}
	if duration != 0 {
		b["duration"] = duration
	}
	return b
}
func createAndUploadSource(c *client.Client, filePath string, width, height int, duration float64, contentType string) (sourceUploadResponse, int, int, error) {
	data, _, err := c.Post("/v1/sources", sourceCreateBody(width, height, duration))
	if err != nil {
		return sourceUploadResponse{}, 0, 0, err
	}
	var created sourceUploadResponse
	if err := json.Unmarshal(data, &created); err != nil {
		return created, 0, 0, err
	}
	payload, err := os.ReadFile(filePath)
	if err != nil {
		return created, 0, 0, err
	}
	req, err := http.NewRequest(http.MethodPut, created.UploadURL, bytes.NewReader(payload))
	if err != nil {
		return created, 0, 0, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	uploadClient := c.HTTPClient
	if uploadClient == nil {
		uploadClient = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := uploadClient.Do(req)
	if err != nil {
		return created, 0, 0, err
	}
	respBody, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return created, 0, 0, readErr
	}
	if resp.StatusCode >= 400 {
		return created, resp.StatusCode, len(payload), fmt.Errorf("source upload returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return created, resp.StatusCode, len(payload), nil
}
func jsonRawToAny(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if json.Unmarshal(raw, &v) == nil {
		return v
	}
	return string(raw)
}
func extractAnyIDs(raw json.RawMessage) []string {
	var v any
	_ = json.Unmarshal(raw, &v)
	ids := []string{}
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			if id, ok := t["id"].(string); ok {
				ids = append(ids, id)
			}
			for _, key := range sortedMapKeys(t) {
				walk(t[key])
			}
		case []any:
			for _, vv := range t {
				walk(vv)
			}
		}
	}
	walk(v)
	return uniqueStrings(ids)
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
func transcriptTermRanges(c *client.Client, vid, cid, term string) ([]map[string]int, error) {
	raw, err := c.Get(fmt.Sprintf("/v1/videos/%s/clips/%s/transcript/uncut", vid, cid), nil)
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("parsing transcript JSON: %w", err)
	}
	return transcriptTermRangesFromValue(v, term), nil
}

func transcriptTermRangesFromValue(v any, term string) []map[string]int {
	ranges := []map[string]int{}
	needle := strings.ToLower(term)
	var scanWords func([]any)
	scanWords = func(words []any) {
		for pos, item := range words {
			wordMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			text, _ := wordMap["text"].(string)
			if text == "" {
				text, _ = wordMap["word"].(string)
			}
			if text == "" || !strings.Contains(strings.ToLower(text), needle) {
				continue
			}
			wordIndex := pos
			switch idx := wordMap["index"].(type) {
			case float64:
				wordIndex = int(idx)
			case int:
				wordIndex = idx
			}
			ranges = append(ranges, map[string]int{"fromWordIndex": wordIndex, "toWordIndex": wordIndex})
		}
	}
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			if words, ok := t["words"].([]any); ok {
				scanWords(words)
			}
			for _, key := range sortedMapKeys(t) {
				if key == "words" {
					continue
				}
				walk(t[key])
			}
		case []any:
			for _, vv := range t {
				walk(vv)
			}
		}
	}
	walk(v)
	return ranges
}

func transcriptLooksEmpty(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return true
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return false
	}
	return !hasTranscriptWords(v)
}

func hasTranscriptWords(v any) bool {
	found := false
	var walk func(any)
	walk = func(x any) {
		if found {
			return
		}
		switch t := x.(type) {
		case map[string]any:
			if words, ok := t["words"].([]any); ok {
				for _, item := range words {
					wordMap, ok := item.(map[string]any)
					if !ok {
						continue
					}
					if text, _ := wordMap["text"].(string); text != "" {
						found = true
						return
					}
					if word, _ := wordMap["word"].(string); word != "" {
						found = true
						return
					}
				}
			}
			for _, key := range sortedMapKeys(t) {
				if key == "words" {
					continue
				}
				walk(t[key])
			}
		case []any:
			for _, vv := range t {
				walk(vv)
			}
		}
	}
	walk(v)
	return found
}
func readJSONFile(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}
func storyboardSteps(videoID string, plan map[string]any) []map[string]any {
	steps := []map[string]any{}
	if v, ok := plan["video"].(map[string]any); ok {
		body := videoPatchBody(v)
		if len(body) > 0 {
			steps = append(steps, map[string]any{"method": "PATCH", "path": "/v1/videos/" + videoID, "body": body})
		}
	}
	if clips, ok := plan["clips"].([]any); ok {
		for _, ci := range clips {
			cmap, ok := ci.(map[string]any)
			if !ok {
				continue
			}
			cid, _ := cmap["id"].(string)
			if cid == "" {
				continue
			}
			body := map[string]any{}
			for _, k := range []string{"name", "order", "cuts", "background"} {
				if val, ok := cmap[k]; ok {
					body[k] = val
				}
			}
			if len(body) > 0 {
				steps = append(steps, map[string]any{"method": "PATCH", "path": fmt.Sprintf("/v1/videos/%s/clips/%s", videoID, cid), "body": body})
			}
			if wr, ok := cmap["wordCuts"]; ok {
				steps = append(steps, map[string]any{"method": "POST", "path": fmt.Sprintf("/v1/videos/%s/clips/%s/cut-by-transcript", videoID, cid), "body": map[string]any{"wordRanges": wr}})
			}
		}
	}
	return steps
}
func videoPatchBody(in map[string]any) map[string]any {
	if nested, ok := in["video"].(map[string]any); ok {
		in = nested
	}
	body := map[string]any{}
	for _, k := range []string{"allowedEmbedDomains", "captionsDefaultEnabled", "commentEmailsEnabled", "commentsEnabled", "customThumbnailURL", "defaultPlaybackRate", "description", "dimensions", "downloadsEnabled", "linkScope", "name", "password", "publishDateEnabled", "rawDownloadsEnabled", "searchEngineIndexingEnabled", "transcriptsEnabled", "viewCountEnabled"} {
		if val, ok := in[k]; ok {
			body[k] = val
		}
	}
	return body
}

func applyStoryboardStep(c *client.Client, step map[string]any) (json.RawMessage, int, error) {
	method, _ := step["method"].(string)
	path, _ := step["path"].(string)
	body := step["body"]
	switch method {
	case "PATCH":
		return c.Patch(path, body)
	case "POST":
		return c.Post(path, body)
	default:
		return nil, 0, fmt.Errorf("unsupported method %q", method)
	}
}
func formatPresetBody(preset string) map[string]any {
	body := map[string]any{"captionsDefaultEnabled": true, "transcriptsEnabled": true}
	switch preset {
	case "shorts", "tiktok", "vertical":
		body["dimensions"] = map[string]any{"width": 1080, "height": 1920}
		body["downloadsEnabled"] = false
	case "square":
		body["dimensions"] = map[string]any{"width": 1080, "height": 1080}
	case "linkedin":
		body["dimensions"] = map[string]any{"width": 1920, "height": 1080}
		body["commentsEnabled"] = true
	default:
		body["dimensions"] = map[string]any{"width": 1920, "height": 1080}
	}
	return body
}
