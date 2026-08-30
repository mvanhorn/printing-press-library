// Copyright 2026 neal-kyle and contributors. Licensed under Apache-2.0. See LICENSE.
// Flagship hand-written command: generate an image via POST /images, save the
// output to disk, and record the generation in the local ledger. This is the
// primary workflow: human or agent picks a model, describes the image, gets a
// file (or structured output) back.

package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/openrouter-image/internal/store"
)

// pp:data-source live

type generateResult struct {
	Action   string         `json:"action"`
	Model    string         `json:"model"`
	Prompt   string         `json:"prompt"`
	Images   []generatedImg `json:"images"`
	Usage    *genUsage      `json:"usage,omitempty"`
	LedgerID string         `json:"ledger_id,omitempty"`
}

// imagesResponse mirrors the POST /images success payload. It is shared by
// generate, batch, and regenerate so streaming (SSE) and plain-JSON response
// bodies parse identically everywhere.
type imagesResponse struct {
	Created int64 `json:"created"`
	Data    []struct {
		Index     int    `json:"index"`
		B64JSON   string `json:"b64_json"`
		MediaType string `json:"media_type"`
	} `json:"data"`
	Usage *genUsage `json:"usage"`
}

// sseImageEvent is the union of the streaming image event shapes documented
// by the OpenRouter API. Image data arrives in several places depending on
// event type:
//
//   - image_generation.partial_image / image_generation.completed carry
//     b64_json and partial_image_index at the top level;
//   - response.image_generation_call.partial_image carries partial_image_b64;
//   - ImageStreamingResponse wraps the same payload in a nested data object.
//
// Every shape is captured here so fragments merge correctly regardless of
// which variant the upstream emits.
type sseImageEvent struct {
	Type            string    `json:"type"`
	B64JSON         string    `json:"b64_json"`
	PartialImageB64 string    `json:"partial_image_b64"`
	PartialImageIdx *int      `json:"partial_image_index"`
	Index           *int      `json:"index"`
	MediaType       string    `json:"media_type"`
	Created         int64     `json:"created"`
	Usage           *genUsage `json:"usage"`
	Error           *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
	Data json.RawMessage `json:"data"`
}

// parseImagesResponse decodes a POST /images response body. When stream is
// true the API returns text/event-stream: each `data:` line carries a JSON
// event. b64_json fragments for the same image index may arrive across
// several events, so they are concatenated in arrival order; an
// image_generation.completed event carries the final image and supersedes
// earlier fragments for its index. Non-streaming responses are plain JSON and
// decode directly.
func parseImagesResponse(body []byte, stream bool) (imagesResponse, error) {
	var resp imagesResponse
	if !stream {
		err := json.Unmarshal(body, &resp)
		return resp, err
	}
	frags := map[int][]string{}
	complete := map[int]string{}
	mediaTypes := map[int]string{}
	var usage *genUsage
	for _, payload := range ssePayloads(body) {
		if payload == "[DONE]" {
			continue
		}
		var ev sseImageEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue // keepalive/comment events that are not JSON
		}
		if ev.Error != nil {
			return resp, fmt.Errorf("stream error: %s (%s)", ev.Error.Message, ev.Error.Code)
		}
		if ev.Usage != nil {
			usage = ev.Usage
		}
		if ev.Created != 0 {
			resp.Created = ev.Created
		}
		idx := 0
		if ev.PartialImageIdx != nil {
			idx = *ev.PartialImageIdx
		} else if ev.Index != nil {
			idx = *ev.Index
		}
		if ev.MediaType != "" {
			mediaTypes[idx] = ev.MediaType
		}
		switch {
		case ev.B64JSON != "":
			if ev.Type == "image_generation.completed" {
				complete[idx] = ev.B64JSON
			} else {
				frags[idx] = append(frags[idx], ev.B64JSON)
			}
		case ev.PartialImageB64 != "":
			frags[idx] = append(frags[idx], ev.PartialImageB64)
		case len(ev.Data) > 0 && ev.Data[0] == '[':
			// Nested data array shape (data[].b64_json), tolerated for
			// compatibility with wrapper-style stream payloads.
			var arr []struct {
				Index     int    `json:"index"`
				B64JSON   string `json:"b64_json"`
				MediaType string `json:"media_type"`
			}
			if err := json.Unmarshal(ev.Data, &arr); err == nil {
				for _, img := range arr {
					if img.B64JSON != "" {
						frags[img.Index] = append(frags[img.Index], img.B64JSON)
					}
					if img.MediaType != "" {
						mediaTypes[img.Index] = img.MediaType
					}
				}
			}
		case len(ev.Data) > 0:
			// Nested data object shape (ImageStreamingResponse).
			var obj struct {
				B64JSON         string `json:"b64_json"`
				PartialImageB64 string `json:"partial_image_b64"`
				PartialImageIdx *int   `json:"partial_image_index"`
				Index           *int   `json:"index"`
				MediaType       string `json:"media_type"`
			}
			if err := json.Unmarshal(ev.Data, &obj); err == nil {
				nidx := 0
				if obj.PartialImageIdx != nil {
					nidx = *obj.PartialImageIdx
				} else if obj.Index != nil {
					nidx = *obj.Index
				}
				if obj.B64JSON != "" {
					frags[nidx] = append(frags[nidx], obj.B64JSON)
				}
				if obj.PartialImageB64 != "" {
					frags[nidx] = append(frags[nidx], obj.PartialImageB64)
				}
				if obj.MediaType != "" {
					mediaTypes[nidx] = obj.MediaType
				}
			}
		}
	}
	if len(frags) == 0 && len(complete) == 0 {
		return resp, fmt.Errorf("streamed response contained no image data")
	}
	indexes := make([]int, 0, len(frags)+len(complete))
	seen := map[int]bool{}
	addIdx := func(i int) {
		if !seen[i] {
			seen[i] = true
			indexes = append(indexes, i)
		}
	}
	for i := range frags {
		addIdx(i)
	}
	for i := range complete {
		addIdx(i)
	}
	sort.Ints(indexes)
	for _, i := range indexes {
		b64 := strings.Join(frags[i], "")
		if final := complete[i]; final != "" {
			b64 = final
		}
		resp.Data = append(resp.Data, struct {
			Index     int    `json:"index"`
			B64JSON   string `json:"b64_json"`
			MediaType string `json:"media_type"`
		}{
			Index:     i,
			B64JSON:   b64,
			MediaType: mediaTypes[i],
		})
	}
	resp.Usage = usage
	return resp, nil
}

// ssePayloads extracts the JSON payload of every `data:` line in an SSE body.
// Lines that are not data events (comments, event names, blank lines) are
// skipped.
func ssePayloads(body []byte) []string {
	var payloads []string
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if payload != "" {
			payloads = append(payloads, payload)
		}
	}
	return payloads
}

type generatedImg struct {
	Index     int    `json:"index"`
	MediaType string `json:"media_type,omitempty"`
	SavedTo   string `json:"saved_to,omitempty"`
	B64Len    int    `json:"b64_len,omitempty"`
}

type genUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost"`
}

func newGenerateCmd(flags *rootFlags) *cobra.Command {
	var (
		flagModel        string
		flagPrompt       string
		flagN            int
		flagResolution   string
		flagAspectRatio  string
		flagSize         string
		flagQuality      string
		flagOutputFormat string
		flagBackground   string
		flagOutputComp   int
		flagSeed         int64
		flagOutput       string
		flagProvider     string
		flagReference    []string
		flagStream       bool
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate an image from a text prompt on the model you choose, save it to disk, and log it locally",
		Long: `Generate an image from a text prompt via OpenRouter's dedicated Image API.

The model is always explicit: pass --model with any image-capable slug
(openai/gpt-image-1, google/gemini-2.5-flash-image, bytedance-seed/seedream-4.5,
black-forest-labs/flux.2-pro, ...). Find candidates offline with:
  openrouter-image-pp-cli models rank --max-cost 0.10
  openrouter-image-pp-cli models list

Use this command for a single ad-hoc image.
Do NOT use it to re-run a past generation; use 'regenerate' instead.
Do NOT use it to run a budgeted batch; use 'batch' instead.`,
		Example: strings.Trim(`
  openrouter-image-pp-cli generate --model openai/gpt-image-1 --prompt "a red panda astronaut" --output panda.png
  openrouter-image-pp-cli generate --model google/gemini-2.5-flash-image --prompt "watercolor of a lighthouse" --aspect-ratio 16:9 --quality high
  openrouter-image-pp-cli generate --model bytedance-seed/seedream-4.5 --prompt "a cute cat" --n 4 --output ./out/ --json --agent
  openrouter-image-pp-cli generate --model openai/gpt-image-1 --prompt "edit this" --reference photo.jpg --output edited.png
`, "\n"),
		Annotations: map[string]string{
			"mcp:write-positionals": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would generate %d image(s) with model %s\n", maxInt(flagN, 1), flagModel)
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			if flagModel == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--model is required (e.g. --model openai/gpt-image-1)"))
			}
			if flagPrompt == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--prompt is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			body := map[string]any{
				"model":  flagModel,
				"prompt": flagPrompt,
			}
			if flagN > 0 {
				body["n"] = flagN
			}
			if flagResolution != "" {
				body["resolution"] = flagResolution
			}
			if flagAspectRatio != "" {
				body["aspect_ratio"] = flagAspectRatio
			}
			if flagSize != "" {
				body["size"] = flagSize
			}
			if flagQuality != "" {
				body["quality"] = flagQuality
			}
			if flagOutputFormat != "" {
				body["output_format"] = flagOutputFormat
			}
			if flagBackground != "" {
				body["background"] = flagBackground
			}
			if flagOutputComp > 0 {
				body["output_compression"] = flagOutputComp
			}
			if cmd.Flags().Changed("seed") {
				body["seed"] = flagSeed
			}
			if flagStream {
				body["stream"] = true
			}
			if len(flagReference) > 0 {
				refs := make([]map[string]any, 0, len(flagReference))
				for _, r := range flagReference {
					url := r
					if _, err := os.Stat(r); err == nil {
						// #nosec G304 -- user-named reference image path, explicit CLI input
						data, err := os.ReadFile(r)
						if err != nil {
							return fmt.Errorf("reading reference image %s: %w", r, err)
						}
						mime := "image/png"
						switch strings.ToLower(filepath.Ext(r)) {
						case ".jpg", ".jpeg":
							mime = "image/jpeg"
						case ".webp":
							mime = "image/webp"
						case ".gif":
							mime = "image/gif"
						}
						url = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
					}
					refs = append(refs, map[string]any{
						"type":      "image_url",
						"image_url": map[string]string{"url": url},
					})
				}
				body["input_references"] = refs
			}
			if flagProvider != "" {
				body["provider"] = map[string]any{"only": strings.Split(flagProvider, ",")}
			}

			data, statusCode, err := c.PostWithParams(ctx, "/images", nil, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if statusCode < 200 || statusCode >= 300 {
				return apiErr(fmt.Errorf("images: HTTP %d: %s", statusCode, truncateJSON(string(data), 300)))
			}

			// Streaming requests return text/event-stream; parse SSE events and
			// concatenate b64_json fragments per image index. Non-streaming
			// responses are plain JSON and go through the same parser.
			resp, err := parseImagesResponse(data, flagStream)
			if err != nil {
				return fmt.Errorf("parsing generation response: %w", err)
			}
			if len(resp.Data) == 0 {
				return apiErr(fmt.Errorf("generation returned no images (HTTP %d)", statusCode))
			}

			res := generateResult{
				Action: "generate",
				Model:  flagModel,
				Prompt: flagPrompt,
				Usage:  resp.Usage,
			}

			isDir := flagOutput != "" && (strings.HasSuffix(flagOutput, "/") || strings.HasSuffix(flagOutput, string(os.PathSeparator)))
			if flagOutput == "" {
				isDir = true
			}
			for i, img := range resp.Data {
				ext := extFromMediaType(img.MediaType)
				gi := generatedImg{Index: i, MediaType: img.MediaType, B64Len: len(img.B64JSON)}
				if img.B64JSON != "" {
					raw, err := base64.StdEncoding.DecodeString(img.B64JSON)
					if err != nil {
						return fmt.Errorf("decoding image %d: %w", i, err)
					}
					var outPath string
					if isDir {
						name := safeName(flagModel) + "-" + time.Now().Format("20060102-150405") + fmt.Sprintf("-%d-%d-%d", time.Now().UnixNano(), os.Getpid(), i) + ext
						outPath = filepath.Join(orDefault(flagOutput, "."), name)
					} else if len(resp.Data) > 1 {
						// Multiple images with a file --output: suffix each image
						// so later writes do not overwrite earlier ones.
						fileExt := filepath.Ext(flagOutput)
						base := strings.TrimSuffix(flagOutput, fileExt)
						outPath = fmt.Sprintf("%s-%d%s", base, i, fileExt)
					} else {
						outPath = flagOutput
					}
					// #nosec G301 -- user-owned output directory, 0755 is the standard for mkdir
					if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
						return fmt.Errorf("creating output dir: %w", err)
					}
					// #nosec G306 -- output images need read permission for the user's tools
					if err := os.WriteFile(outPath, raw, 0o644); err != nil {
						return fmt.Errorf("writing image %d: %w", i, err)
					}
					gi.SavedTo = outPath
				}
				res.Images = append(res.Images, gi)
			}

			// Record in the local generation ledger. A billed generation
			// that cannot be recorded must fail the command: cost history
			// and regeneration metadata must not silently disappear.
			dbPath := defaultDBPath("openrouter-image-pp-cli")
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening ledger database: %w", err)
			}
			_ = db.EnsureOpenRouterImageTables(ctx)
			paramsJSON, _ := json.Marshal(body)
			ledgerID := newLedgerID(flagModel)
			entry := store.GenerationEntry{
				ID:         ledgerID,
				Model:      flagModel,
				Prompt:     flagPrompt,
				Params:     string(paramsJSON),
				OutputPath: firstSaved(res.Images),
			}
			if resp.Usage != nil {
				entry.CostUSD = resp.Usage.Cost
			}
			if err := db.LedgerGeneration(ctx, entry); err != nil {
				_ = db.Close()
				return fmt.Errorf("recording generation in ledger: %w", err)
			}
			res.LedgerID = ledgerID
			_ = db.Close()

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			for _, img := range res.Images {
				if img.SavedTo != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "saved: %s\n", img.SavedTo)
				}
			}
			if res.Usage != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "cost: $%.4f (%d tokens)\n", res.Usage.Cost, res.Usage.TotalTokens)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagModel, "model", "", "Image model slug (e.g. openai/gpt-image-1, google/gemini-2.5-flash-image)")
	cmd.Flags().StringVar(&flagPrompt, "prompt", "", "Text description of the desired image")
	cmd.Flags().IntVar(&flagN, "n", 1, "Number of images to generate (1-10)")
	cmd.Flags().StringVar(&flagResolution, "resolution", "", "Resolution tier: 512, 1K, 2K, 4K (provider-dependent)")
	cmd.Flags().StringVar(&flagAspectRatio, "aspect-ratio", "", "Aspect ratio: 1:1, 16:9, 9:16, 4:3, 3:4, etc.")
	cmd.Flags().StringVar(&flagSize, "size", "", "Convenience size shorthand: tier (2K) or pixels (2048x2048)")
	cmd.Flags().StringVar(&flagQuality, "quality", "", "Quality: auto, low, medium, high")
	cmd.Flags().StringVar(&flagOutputFormat, "output-format", "", "Output format: png, jpeg, webp, svg")
	cmd.Flags().StringVar(&flagBackground, "background", "", "Background: auto, transparent, opaque")
	cmd.Flags().IntVar(&flagOutputComp, "output-compression", 0, "Compression 0-100 for webp/jpeg")
	cmd.Flags().Int64Var(&flagSeed, "seed", 0, "Deterministic seed for reproducible generation")
	cmd.Flags().StringVar(&flagOutput, "output", "", "Output file path, or directory (trailing /) for multiple images")
	cmd.Flags().StringVar(&flagProvider, "provider", "", "Comma-separated provider slugs to restrict routing to")
	cmd.Flags().StringSliceVar(&flagReference, "reference", nil, "Reference image for image-to-image (local path, URL, or data URL); repeatable")
	cmd.Flags().BoolVar(&flagStream, "stream", false, "Request SSE streaming of partial images (model must support it)")
	return cmd
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func safeName(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, s)
	return strings.Trim(s, "-")
}

// ledgerSeq disambiguates ledger ids and directory output names created
// within the same clock tick. Second-resolution ids collide when two
// generations for the same model land in the same second, and the ledger's
// INSERT OR REPLACE would silently overwrite the first row, losing its
// parameters, output path, and cost.
var ledgerSeq uint64

// nextLedgerSeq atomically advances the per-process sequence. Every call
// consumes a unique value, so directory output names derived from it never
// collide even when the ledger write that would normally advance the counter
// fails.
func nextLedgerSeq() uint64 {
	ledgerSeq++
	return ledgerSeq
}

// newLedgerID builds a collision-resistant generation id: unix nanoseconds
// plus a per-process sequence suffix, so ids are unique even for back-to-back
// generations of the same model in the same instant.
func newLedgerID(model string) string {
	return fmt.Sprintf("gen-%d-%d-%s", time.Now().UnixNano(), nextLedgerSeq(), safeName(model))
}

func extFromMediaType(mt string) string {
	switch {
	case strings.Contains(mt, "jpeg"), strings.Contains(mt, "jpg"):
		return ".jpg"
	case strings.Contains(mt, "webp"):
		return ".webp"
	case strings.Contains(mt, "svg"):
		return ".svg"
	case strings.Contains(mt, "gif"):
		return ".gif"
	default:
		return ".png"
	}
}

func firstSaved(imgs []generatedImg) string {
	for _, i := range imgs {
		if i.SavedTo != "" {
			return i.SavedTo
		}
	}
	return ""
}

func truncateJSON(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
