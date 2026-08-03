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

			var resp struct {
				Created int64 `json:"created"`
				Data    []struct {
					B64JSON   string `json:"b64_json"`
					MediaType string `json:"media_type"`
				} `json:"data"`
				Usage *genUsage `json:"usage"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
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
						name := safeName(flagModel) + "-" + time.Now().Format("20060102-150405") + fmt.Sprintf("-%d", i) + ext
						outPath = filepath.Join(orDefault(flagOutput, "."), name)
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

			// Record in the local generation ledger.
			dbPath := defaultDBPath("openrouter-image-pp-cli")
			if db, err := store.OpenWithContext(ctx, dbPath); err == nil {
				_ = db.EnsureOpenRouterImageTables(ctx)
				paramsJSON, _ := json.Marshal(body)
				ledgerID := fmt.Sprintf("gen-%d-%s", time.Now().Unix(), safeName(flagModel))
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
				_ = db.LedgerGeneration(ctx, entry)
				res.LedgerID = ledgerID
				_ = db.Close()
			}

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
