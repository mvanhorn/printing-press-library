// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature command (Phase 3).

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

func newChaptersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "chapters",
		Short:       "Auto-generate video chapters from transcripts",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newChaptersAutoCmd(flags))
	return cmd
}

func newChaptersAutoCmd(flags *rootFlags) *cobra.Command {
	var provider, ytdlpPath string
	var apply bool
	cmd := &cobra.Command{
		Use:   "auto [video-id]",
		Short: "Pull transcript via yt-dlp, generate chapter timestamps via LLM, write back to description",
		Long: `Pipeline:
1. yt-dlp pulls the auto-generated or manual captions for the target video
2. The captions are sent to an LLM provider (--provider claude|openai|none) to
   propose chapter timestamps in YouTube's '0:00 Title' format
3. With --apply, the chapters are prepended to the video's description via
   videos.update; without --apply, the proposed chapter block is printed

Providers:
  claude  Anthropic Messages API. Default model: claude-sonnet-4-6.
          Requires YT_PP_CLI_CLAUDE_API_KEY (sk-ant-...).
          Override model via YT_PP_CLI_CLAUDE_MODEL.
  openai  OpenAI Chat Completions API. Default model: gpt-4o-mini.
          Requires YT_PP_CLI_OPENAI_API_KEY (sk-...).
          Override model via YT_PP_CLI_OPENAI_MODEL.
  none    Heuristic preview only (no API call) — useful for testing the
          transcript-pull + write-back path without spending tokens.`,
		Example: "  youtube-pp-cli chapters auto dQw4w9WgXcQ --provider claude\n" +
			"  youtube-pp-cli chapters auto dQw4w9WgXcQ --provider claude --apply",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if flags.dryRun {
					return nil
				}
				return cmd.Help()
			}
			videoID := args[0]
			if dryRunOK(flags) {
				return nil
			}

			// Resolve yt-dlp
			if ytdlpPath == "" {
				p, err := exec.LookPath("yt-dlp")
				if err != nil {
					return configErr(fmt.Errorf("yt-dlp not on PATH; pass --yt-dlp or install with: pip install -U yt-dlp"))
				}
				ytdlpPath = p
			}

			// 1) Pull transcript (auto-subs)
			tmpDir, err := os.MkdirTemp("", "yt-chapters-*")
			if err != nil {
				return fmt.Errorf("creating temp dir: %w", err)
			}
			defer os.RemoveAll(tmpDir)
			ytArgs := []string{
				"--write-auto-subs", "--sub-langs", "en", "--skip-download",
				"--sub-format", "vtt",
				"-o", fmt.Sprintf("%s/%%(id)s.%%(ext)s", tmpDir),
				"https://www.youtube.com/watch?v=" + videoID,
			}
			run := exec.Command(ytdlpPath, ytArgs...)
			if out, err := run.CombinedOutput(); err != nil {
				return apiErr(fmt.Errorf("yt-dlp failed: %w: %s", err, string(out)))
			}
			// Find the VTT file
			matches, _ := os.ReadDir(tmpDir)
			var vttPath string
			for _, m := range matches {
				if strings.HasSuffix(m.Name(), ".vtt") {
					vttPath = tmpDir + "/" + m.Name()
					break
				}
			}
			if vttPath == "" {
				return apiErr(fmt.Errorf("yt-dlp produced no .vtt captions for %s (video may have captions disabled)", videoID))
			}
			vttData, err := os.ReadFile(vttPath)
			if err != nil {
				return err
			}

			// 2) Parse VTT into [timestamp, text] sequence
			segs := parseVTT(string(vttData))

			// 3) Send to LLM provider (or fall back to instructional message)
			var chapters string
			if provider == "" || provider == "none" {
				chapters = fmt.Sprintf("# Chapters (provider=none — set --provider claude or openai with the right env var)\n0:00 Intro\n# Transcript has %d segments — configure an LLM provider for real chapter suggestions\n", len(segs))
			} else {
				chapters, err = generateChaptersWithLLM(provider, segs)
				if err != nil {
					return apiErr(err)
				}
			}

			if !apply {
				return flags.printJSON(cmd, map[string]any{
					"video_id": videoID,
					"provider": provider,
					"segments": len(segs),
					"chapters": chapters,
					"note":     "Pass --apply to prepend these chapters to the video's description.",
				})
			}

			// 4) Apply: fetch current snippet, prepend chapters
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			quotaLogCost("videos-list", 1)
			vdata, err := c.Get("/youtube/v3/videos", map[string]string{
				"part": "snippet",
				"id":   videoID,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var vresp struct {
				Items []struct {
					Snippet map[string]any `json:"snippet"`
				} `json:"items"`
			}
			_ = json.Unmarshal(vdata, &vresp)
			if len(vresp.Items) == 0 {
				return notFoundErr(fmt.Errorf("video %s not found", videoID))
			}
			snip := vresp.Items[0].Snippet
			oldDesc, _ := snip["description"].(string)
			snip["description"] = chapters + "\n\n" + oldDesc
			body := map[string]any{
				"id":      videoID,
				"snippet": snip,
			}
			quotaLogCost("videos-update", 50)
			_, _, err = c.PostWithParams("/youtube/v3/videos", map[string]string{"part": "snippet"}, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, map[string]any{
				"video_id":      videoID,
				"applied":       true,
				"chapter_lines": strings.Count(chapters, "\n"),
			})
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "none", "LLM provider: none, claude, openai")
	cmd.Flags().StringVar(&ytdlpPath, "yt-dlp", "", "Path to yt-dlp binary")
	cmd.Flags().BoolVar(&apply, "apply", false, "Write chapters back to the video description")
	return cmd
}

type vttSeg struct {
	StartSec int
	Text     string
}

func parseVTT(s string) []vttSeg {
	// Very lean VTT parser: finds timestamp lines like "00:01:23.000 --> ..." and the following text line.
	re := regexp.MustCompile(`(?m)^(\d{2}):(\d{2}):(\d{2})\.\d+ --> .*$`)
	lines := strings.Split(s, "\n")
	var segs []vttSeg
	for i := 0; i < len(lines); i++ {
		m := re.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		h, mn, sec := 0, 0, 0
		fmt.Sscanf(m[1], "%d", &h)
		fmt.Sscanf(m[2], "%d", &mn)
		fmt.Sscanf(m[3], "%d", &sec)
		// Combine subsequent non-empty text lines
		var text []string
		for j := i + 1; j < len(lines); j++ {
			line := strings.TrimSpace(lines[j])
			if line == "" {
				break
			}
			text = append(text, line)
		}
		segs = append(segs, vttSeg{StartSec: h*3600 + mn*60 + sec, Text: strings.Join(text, " ")})
	}
	return segs
}

// generateChaptersWithLLM dispatches to the configured provider and returns
// the chapter block as YouTube expects it ("0:00 Title" lines), or an error.
//
// Providers:
//   - claude: Anthropic Messages API (api.anthropic.com/v1/messages)
//   - openai: OpenAI Chat Completions API (api.openai.com/v1/chat/completions)
//   - none:   returns a heuristic preview (no API call) so the flow can be
//             tested end-to-end without credentials.
//
// Env vars:
//   YT_PP_CLI_CLAUDE_API_KEY  (sk-ant-...)
//   YT_PP_CLI_OPENAI_API_KEY  (sk-...)
//   YT_PP_CLI_CLAUDE_MODEL    (optional override, default claude-sonnet-4-6)
//   YT_PP_CLI_OPENAI_MODEL    (optional override, default gpt-4o-mini)
func generateChaptersWithLLM(provider string, segs []vttSeg) (string, error) {
	prompt := buildChapterPrompt(segs)
	switch provider {
	case "claude":
		key := os.Getenv("YT_PP_CLI_CLAUDE_API_KEY")
		if key == "" {
			return "", fmt.Errorf("YT_PP_CLI_CLAUDE_API_KEY not set")
		}
		return callClaude(prompt, key)
	case "openai":
		key := os.Getenv("YT_PP_CLI_OPENAI_API_KEY")
		if key == "" {
			return "", fmt.Errorf("YT_PP_CLI_OPENAI_API_KEY not set")
		}
		return callOpenAI(prompt, key)
	}
	return "", fmt.Errorf("unknown provider %q (use claude, openai, or none)", provider)
}

// buildChapterPrompt assembles the prompt sent to the LLM. The transcript is
// truncated to ~30K characters so even 4-hour streams fit in one request.
func buildChapterPrompt(segs []vttSeg) string {
	var b strings.Builder
	b.WriteString("You are generating YouTube chapter timestamps for a video.\n\n")
	b.WriteString("Rules:\n")
	b.WriteString("- Output 5-12 chapter lines in YouTube's required format: `M:SS Title` or `H:MM:SS Title` for videos over an hour.\n")
	b.WriteString("- The FIRST chapter MUST start at exactly 0:00.\n")
	b.WriteString("- Titles: 2-6 words, descriptive, Title Case. No emoji. No leading dashes.\n")
	b.WriteString("- One chapter per major topic shift (not every minute).\n")
	b.WriteString("- Respond with ONLY the chapter lines — no preamble, no explanation, no markdown fencing.\n\n")
	b.WriteString("Transcript (timestamped):\n")
	const maxTranscriptChars = 30000
	totalLen := 0
	for _, s := range segs {
		h := s.StartSec / 3600
		m := (s.StartSec % 3600) / 60
		sec := s.StartSec % 60
		var line string
		if h > 0 {
			line = fmt.Sprintf("[%02d:%02d:%02d] %s\n", h, m, sec, s.Text)
		} else {
			line = fmt.Sprintf("[%02d:%02d] %s\n", m, sec, s.Text)
		}
		if totalLen+len(line) > maxTranscriptChars {
			b.WriteString("[transcript truncated]\n")
			break
		}
		b.WriteString(line)
		totalLen += len(line)
	}
	return b.String()
}

// callClaude POSTs the prompt to Anthropic's Messages API and returns the
// first text content block. Default model: claude-sonnet-4-6.
func callClaude(prompt, apiKey string) (string, error) {
	model := os.Getenv("YT_PP_CLI_CLAUDE_MODEL")
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	body := map[string]any{
		"model":      model,
		"max_tokens": 1000,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling Anthropic API: %w", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("anthropic API returned %d: %s", resp.StatusCode, truncate(string(respBytes), 200))
	}
	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return "", fmt.Errorf("parsing Anthropic response: %w", err)
	}
	for _, c := range apiResp.Content {
		if c.Type == "text" {
			return strings.TrimSpace(c.Text), nil
		}
	}
	return "", fmt.Errorf("anthropic response had no text content block")
}

// callOpenAI POSTs the prompt to OpenAI's Chat Completions API and returns
// the first choice's message content. Default model: gpt-4o-mini.
func callOpenAI(prompt, apiKey string) (string, error) {
	model := os.Getenv("YT_PP_CLI_OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	body := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  1000,
		"temperature": 0.2,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling OpenAI API: %w", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("openai API returned %d: %s", resp.StatusCode, truncate(string(respBytes), 200))
	}
	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return "", fmt.Errorf("parsing OpenAI response: %w", err)
	}
	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("openai response had no choices")
	}
	return strings.TrimSpace(apiResp.Choices[0].Message.Content), nil
}
