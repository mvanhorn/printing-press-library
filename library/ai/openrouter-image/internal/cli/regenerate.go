// Copyright 2026 neal-kyle and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: deterministic re-generation. Reads the exact stored parameter
// set (model, prompt, seed, resolution, quality, references) from the local
// generation ledger and rebuilds a POST /images request, so a past generation
// can be replayed or tweaked exactly.

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

// pp:data-source auto

func newNovelRegenerateCmd(flags *rootFlags) *cobra.Command {
	var (
		flagOutput string
		flagTweak  string
		dbPath     string
	)

	cmd := &cobra.Command{
		Use:   "regenerate",
		Short: "Re-run a past generation with its exact stored parameters (model, seed, resolution, quality, references).",
		Long: `Re-run a past generation with its exact stored parameters.

The generation id comes from the local ledger (each successful generate or
batch run records one). The stored request body is replayed verbatim, with an
optional --tweak applied to the prompt.

Use this command to re-run a past generation with its exact stored parameters.
Do NOT use it for a brand-new prompt; use 'generate' instead.`,
		Example: strings.Trim(`
  openrouter-image-pp-cli regenerate gen-1234567890 --output winner.png
  openrouter-image-pp-cli regenerate gen-1234567890 --tweak "make it darker" --output v2.png
`, "\n"),
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would regenerate generation", firstArg(args))
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("missing required positional argument: generation id"))
			}
			genID := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("openrouter-image-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureOpenRouterImageTables(ctx); err != nil {
				return err
			}
			entry, err := db.GetGeneration(ctx, genID)
			if err != nil {
				return fmt.Errorf("reading generation %s: %w", genID, err)
			}
			if entry == nil {
				return notFoundErr(fmt.Errorf("no local generation %q in the ledger; run generate first", genID))
			}

			var body map[string]any
			if entry.Params != "" {
				_ = json.Unmarshal([]byte(entry.Params), &body)
			}
			if body == nil {
				body = map[string]any{"model": entry.Model, "prompt": entry.Prompt}
			}
			if flagTweak != "" {
				prompt, _ := body["prompt"].(string)
				body["prompt"] = prompt + " " + flagTweak
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, statusCode, err := c.PostWithParams(ctx, "/images", nil, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if statusCode < 200 || statusCode >= 300 {
				return apiErr(fmt.Errorf("regenerate: HTTP %d: %s", statusCode, truncateJSON(string(data), 300)))
			}

			// The replayed body may include stream:true, so the response can
			// be text/event-stream; parseImagesResponse handles both shapes.
			streamed, _ := body["stream"].(bool)
			resp, err := parseImagesResponse(data, streamed)
			if err != nil {
				return fmt.Errorf("parsing regeneration response: %w", err)
			}

			res := generateResult{
				Action: "regenerate",
				Model:  entry.Model,
				Prompt: entry.Prompt,
				Usage:  resp.Usage,
			}
			if len(resp.Data) > 0 {
				for i, img := range resp.Data {
					gi := generatedImg{Index: i, MediaType: img.MediaType, B64Len: len(img.B64JSON)}
					if img.B64JSON != "" && img.MediaType != "" && flagOutput != "" {
						raw, err := decodeB64(img.B64JSON)
						if err != nil {
							return err
						}
						outPath := flagOutput
						if len(resp.Data) > 1 {
							// Multiple images with a file --output: suffix each
							// image so later writes do not overwrite earlier ones.
							fileExt := filepath.Ext(flagOutput)
							base := strings.TrimSuffix(flagOutput, fileExt)
							outPath = fmt.Sprintf("%s-%d%s", base, i, fileExt)
						}
						// #nosec G306 -- output images need read permission for the user's tools
						if err := os.WriteFile(outPath, raw, 0o644); err != nil {
							return fmt.Errorf("writing regenerated image: %w", err)
						}
						gi.SavedTo = outPath
					}
					res.Images = append(res.Images, gi)
				}
			}

			// Record the re-run in the ledger.
			paramsJSON, _ := json.Marshal(body)
			newID := newLedgerID(entry.Model)
			newEntry := store.GenerationEntry{
				ID:         newID,
				Model:      entry.Model,
				Prompt:     orDefault(flagTweak, entry.Prompt),
				Params:     string(paramsJSON),
				OutputPath: firstSaved(res.Images),
			}
			if resp.Usage != nil {
				newEntry.CostUSD = resp.Usage.Cost
			}
			if err := db.LedgerGeneration(ctx, newEntry); err != nil {
				return fmt.Errorf("recording regenerated generation in ledger: %w", err)
			}
			res.LedgerID = newID

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
	cmd.Flags().StringVar(&flagOutput, "output", "", "Output file path for the regenerated image")
	cmd.Flags().StringVar(&flagTweak, "tweak", "", "Optional prompt edit appended to the original prompt")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: platform data dir)")
	return cmd
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func decodeB64(s string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decoding base64 image: %w", err)
	}
	return raw, nil
}

func nowUnix() int64 {
	return time.Now().Unix()
}
