// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Hand-written: novel feature (frozen research bundle). See research.json novel_features.

package cli

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"polymarket-pp-cli/internal/cliutil"
	"polymarket-pp-cli/internal/store"
)

func newBundleCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bundle",
		Short: "Reproducible research bundles — export/import markets+events+tags+price-history as portable zip.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newBundleExportCmd(flags))
	cmd.AddCommand(newBundleImportCmd(flags))
	return cmd
}

func newBundleExportCmd(flags *rootFlags) *cobra.Command {
	var tag, event, idsCSV, outPath string

	cmd := &cobra.Command{
		Use:     "export",
		Short:   "Export every market matching a tag, event, or id list as a portable zip: markets + events + tags + full price history + book snapshot + holders.",
		Example: `  polymarket-pp-cli bundle export --tag 2026-election --out ./election-snapshot.zip`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:novel":      "bundle.export",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Selector mutex
			selectorsSet := 0
			if cmd.Flags().Changed("tag") {
				selectorsSet++
			}
			if cmd.Flags().Changed("event") {
				selectorsSet++
			}
			if cmd.Flags().Changed("ids") {
				selectorsSet++
			}
			if selectorsSet == 0 && !flags.dryRun {
				return usageErr(fmt.Errorf("exactly one of --tag, --event, --ids must be set"))
			}
			if selectorsSet > 1 {
				return usageErr(fmt.Errorf("only one of --tag, --event, --ids may be set (got %d)", selectorsSet))
			}
			if outPath == "" && !flags.dryRun {
				return usageErr(fmt.Errorf("required flag \"out\" not set"))
			}
			if dryRunOK(flags) {
				return nil
			}

			dbPath := defaultDBPath("polymarket-pp-cli")
			s, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return apiErr(fmt.Errorf("opening local store: %w (run sync first)", err))
			}
			defer s.Close()

			// 1. Collect candidate market IDs from selector.
			var marketIDs []string
			switch {
			case idsCSV != "":
				for _, id := range strings.Split(idsCSV, ",") {
					if id = strings.TrimSpace(id); id != "" {
						marketIDs = append(marketIDs, id)
					}
				}
			case event != "":
				marketIDs = pickMarketIDsByEvent(s, event)
			case tag != "":
				marketIDs = pickMarketIDsByTag(s, tag)
			}

			// 2. Open output zip.
			f, err := os.Create(outPath)
			if err != nil {
				return apiErr(fmt.Errorf("creating output: %w", err))
			}
			defer f.Close()
			h := sha256.New()
			zw := zip.NewWriter(io.MultiWriter(f, h))

			// 3. Write manifest first.
			manifest := map[string]any{
				"bundle_version": "1",
				"created_at":     time.Now().UTC().Format(time.RFC3339),
				"selector": map[string]string{
					"tag":   tag,
					"event": event,
					"ids":   idsCSV,
				},
				"market_ids": marketIDs,
			}
			if err := writeZipJSON(zw, "manifest.json", manifest); err != nil {
				return apiErr(err)
			}

			// 4. Write markets.jsonl.
			marketsCount, err := writeResourcesByIDs(zw, s, "markets", marketIDs, "markets.jsonl")
			if err != nil {
				return apiErr(err)
			}
			// 5. Write events.jsonl (best-effort by event ID enumeration).
			eventsCount, _ := writeAllResources(zw, s, "events", "events.jsonl", marketIDs, "events")
			// 6. Write tags.jsonl.
			tagsCount, _ := writeAllResources(zw, s, "tags", "tags.jsonl", nil, "")
			// 7. Pull price-history per token (live, capped at 50).
			c, _ := flags.newClient()
			tokenIDs := extractTokensForMarkets(s, marketIDs)
			if len(tokenIDs) > 50 {
				tokenIDs = tokenIDs[:50]
			}
			historyPoints := 0
			if c != nil && len(tokenIDs) > 0 {
				type src struct{ tokenID string }
				sources := make([]src, len(tokenIDs))
				for i, t := range tokenIDs {
					sources[i] = src{tokenID: t}
				}
				results, ferrs := cliutil.FanoutRun(
					cmd.Context(),
					sources,
					func(s src) string { return s.tokenID },
					func(ctx context.Context, s src) (json.RawMessage, error) {
						return c.GetWithHeaders(ctx,
							"https://clob.polymarket.com/prices-history",
							map[string]string{"market": s.tokenID, "interval": "1d"}, nil)
					},
					cliutil.WithConcurrency(4),
				)
				if len(ferrs) > 0 {
					cliutil.FanoutReportErrors(cmd.ErrOrStderr(), ferrs)
				}
				hw, _ := zw.Create("price-history.jsonl")
				for _, r := range results {
					if hw == nil {
						break
					}
					line, _ := json.Marshal(map[string]any{
						"token_id": r.Source,
						"history":  json.RawMessage(r.Value),
					})
					_, _ = hw.Write(line)
					_, _ = hw.Write([]byte("\n"))
					historyPoints++
				}
			}

			if err := zw.Close(); err != nil {
				return apiErr(fmt.Errorf("closing zip: %w", err))
			}
			sum := hex.EncodeToString(h.Sum(nil))
			report := map[string]any{
				"output_path":    outPath,
				"sha256":         sum,
				"markets":        marketsCount,
				"events":         eventsCount,
				"tags":           tagsCount,
				"price_history":  historyPoints,
				"tokens_sampled": len(tokenIDs),
			}
			// When the selector matched no markets, surface an explicit warning
			// on stderr so callers don't get a silent "successful" empty bundle.
			// Common cause: nothing synced yet, or the --tag/--event/--ids
			// argument matched zero rows in the local store.
			if marketsCount == 0 {
				selectorHint := ""
				switch {
				case tag != "":
					selectorHint = fmt.Sprintf("--tag %q", tag)
				case event != "":
					selectorHint = fmt.Sprintf("--event %q", event)
				case idsCSV != "":
					selectorHint = fmt.Sprintf("--ids %q", idsCSV)
				}
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: bundle export wrote 0 markets to %s. "+
						"Selector %s matched no rows in the local store. "+
						"Run `polymarket-pp-cli sync --resources markets,events,tags` first, or try a different selector.\n",
					outPath, selectorHint)
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "Select markets by tag slug")
	cmd.Flags().StringVar(&event, "event", "", "Select markets by event ID")
	cmd.Flags().StringVar(&idsCSV, "ids", "", "Comma-separated market IDs")
	cmd.Flags().StringVar(&outPath, "out", "", "Output zip path (required)")
	return cmd
}

func newBundleImportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "import <path>",
		Short:   "Rehydrate a research bundle into the local SQLite store.",
		Example: `  polymarket-pp-cli bundle import ./election-snapshot.zip`,
		Annotations: map[string]string{
			"pp:novel": "bundle.import",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return usageErr(fmt.Errorf("bundle path required (e.g. ./snapshot.zip)"))
			}
			if dryRunOK(flags) {
				return nil
			}
			path := args[0]
			zr, err := zip.OpenReader(path)
			if err != nil {
				return apiErr(fmt.Errorf("opening zip: %w", err))
			}
			defer zr.Close()

			dbPath := defaultDBPath("polymarket-pp-cli")
			s, err := store.Open(dbPath)
			if err != nil {
				return apiErr(fmt.Errorf("opening local store: %w", err))
			}
			defer s.Close()

			inserted := map[string]int{}
			for _, zf := range zr.File {
				if !strings.HasSuffix(zf.Name, ".jsonl") {
					continue
				}
				resType := strings.TrimSuffix(zf.Name, ".jsonl")
				rc, err := zf.Open()
				if err != nil {
					continue
				}
				dec := json.NewDecoder(rc)
				for dec.More() {
					var raw json.RawMessage
					if err := dec.Decode(&raw); err != nil {
						break
					}
					var obj map[string]any
					if err := json.Unmarshal(raw, &obj); err != nil {
						continue
					}
					id := store.ExtractResourceID(resType, obj)
					if id == "" {
						id = fmt.Sprintf("imported-%d", inserted[resType])
					}
					if err := s.Upsert(resType, id, raw); err == nil {
						inserted[resType]++
					}
				}
				rc.Close()
			}
			out := map[string]any{
				"path":     path,
				"imported": inserted,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

// -- helpers --

func writeZipJSON(zw *zip.Writer, name string, v any) error {
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// writeResourcesByIDs writes each resource matching the given IDs as a
// JSONL line; returns the number of rows written.
func writeResourcesByIDs(zw *zip.Writer, s *store.Store, resType string, ids []string, name string) (int, error) {
	w, err := zw.Create(name)
	if err != nil {
		return 0, fmt.Errorf("create %s: %w", name, err)
	}
	count := 0
	for _, id := range ids {
		raw, err := s.Get(resType, id)
		if err != nil || raw == nil {
			continue
		}
		_, _ = w.Write(raw)
		_, _ = w.Write([]byte("\n"))
		count++
	}
	return count, nil
}

// writeAllResources writes every row of resType matching an optional
// filter. Returns number written.
func writeAllResources(zw *zip.Writer, s *store.Store, resType, name string, filterIDs []string, _ string) (int, error) {
	w, err := zw.Create(name)
	if err != nil {
		return 0, fmt.Errorf("create %s: %w", name, err)
	}
	raws, err := s.List(resType, 10000)
	if err != nil {
		return 0, nil // tolerate missing tables
	}
	filterSet := map[string]struct{}{}
	for _, id := range filterIDs {
		filterSet[id] = struct{}{}
	}
	count := 0
	for _, r := range raws {
		if len(filterIDs) > 0 {
			var obj map[string]any
			if err := json.Unmarshal(r, &obj); err != nil {
				continue
			}
			id := store.ExtractResourceID(resType, obj)
			if _, ok := filterSet[id]; !ok {
				// Allow if the event has a `markets` list that intersects
				if arr, ok := obj["markets"].([]any); ok {
					match := false
					for _, m := range arr {
						if mid, ok := m.(string); ok {
							if _, ok := filterSet[mid]; ok {
								match = true
								break
							}
						}
					}
					if !match {
						continue
					}
				} else {
					continue
				}
			}
		}
		_, _ = w.Write(r)
		_, _ = w.Write([]byte("\n"))
		count++
	}
	return count, nil
}

func pickMarketIDsByTag(s *store.Store, tag string) []string {
	raws, err := s.List("markets", 10000)
	if err != nil {
		return nil
	}
	tagLower := strings.ToLower(tag)
	var ids []string
	for _, r := range raws {
		var m map[string]any
		if err := json.Unmarshal(r, &m); err != nil {
			continue
		}
		// tags may be: tags: ["a","b"] or events[].tags
		if arr, ok := m["tags"].([]any); ok {
			for _, t := range arr {
				if ts, ok := t.(string); ok && strings.Contains(strings.ToLower(ts), tagLower) {
					if id := store.ExtractResourceID("markets", m); id != "" {
						ids = append(ids, id)
					}
					break
				}
			}
		}
	}
	return ids
}

func pickMarketIDsByEvent(s *store.Store, eventID string) []string {
	raws, err := s.List("markets", 10000)
	if err != nil {
		return nil
	}
	var ids []string
	for _, r := range raws {
		var m map[string]any
		if err := json.Unmarshal(r, &m); err != nil {
			continue
		}
		var evID string
		if v, ok := m["eventId"].(string); ok {
			evID = v
		} else if v, ok := m["event_id"].(string); ok {
			evID = v
		}
		if evID == eventID {
			if id := store.ExtractResourceID("markets", m); id != "" {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func extractTokensForMarkets(s *store.Store, marketIDs []string) []string {
	out := []string{}
	for _, mid := range marketIDs {
		raw, err := s.Get("markets", mid)
		if err != nil || raw == nil {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		if rawTokens, ok := m["clobTokenIds"].(string); ok {
			var tokens []string
			_ = json.Unmarshal([]byte(rawTokens), &tokens)
			out = append(out, tokens...)
		} else if arr, ok := m["clobTokenIds"].([]any); ok {
			for _, t := range arr {
				if ts, ok := t.(string); ok {
					out = append(out, ts)
				}
			}
		}
	}
	return out
}
