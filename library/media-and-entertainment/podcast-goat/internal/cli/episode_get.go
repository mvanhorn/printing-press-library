// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 episode get powered by the cookie -> free -> paid dispatcher.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/dispatch"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

func newEpisodeGetCmd(flags *rootFlags) *cobra.Command {
	var (
		flagText     bool
		flagJSON     bool
		flagJSONL    bool
		flagMD       bool
		flagOut      string
		flagPaid     bool
		flagProvider string
		flagAutoPaid bool
		flagExplain  bool
		flagLang     string
		flagNoCache  bool
	)

	cmd := &cobra.Command{
		Use:   "get [url]",
		Short: "Fetch one transcript by URL via the cookie -> free -> paid dispatch chain",
		Example: `  podcast-goat-pp-cli episode get https://www.dwarkesh.com/p/karpathy
  podcast-goat-pp-cli episode get https://youtu.be/abc --md
  podcast-goat-pp-cli episode get <url> --explain --dry-run
  podcast-goat-pp-cli episode get <url> --paid --provider spoken`,
		Annotations: map[string]string{"pp:endpoint": "episode.get", "pp:method": "GET", "pp:path": "/episode/{url}", "mcp:read-only": "true"},
		Args:        cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			url := args[0]
			if err := validateLangFlag(flagLang); err != nil {
				return err
			}
			providers := []string{}
			if flagProvider != "" {
				for _, p := range strings.Split(flagProvider, ",") {
					if t := strings.TrimSpace(p); t != "" {
						providers = append(providers, t)
					}
				}
			}
			allowPaid := flagPaid || flagAutoPaid
			opts := dispatch.Options{
				AllowPaid:        allowPaid,
				AllowedProviders: providers,
				DryRun:           flags.dryRun,
				Explain:          flagExplain || flags.dryRun,
				Lang:             flagLang,
			}

			if cliutil.IsVerifyEnv() && !flags.dryRun {
				// Side-effect-safe stub for verify runs.
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch transcript: %s (verify mode short-circuit)\n", url)
				return nil
			}

			res, err := dispatch.Dispatch(cmd.Context(), url, opts)
			if err != nil {
				return classifyDispatchErr(cmd.Context(), url, err)
			}

			// Dry-run / explain-only path: print trace and return.
			if flags.dryRun {
				return emitTrace(cmd, flags, res)
			}

			tr := res.Transcript
			if tr == nil {
				return fmt.Errorf("dispatcher returned no transcript and no error")
			}

			// Persist to local store unless --no-cache. Non-default --lang
			// fetches are NOT cached: cache identity is per-URL and currently
			// language-blind, so an it fetch would silently overwrite the en
			// row (and vice versa), and search/quote would serve whichever
			// language landed last. Skipping is declared, not silent.
			if !cacheableLang(flagLang) {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: --lang %s transcript not written to the local cache (cache identity is per-URL and language-blind in v0.1); use --out to keep it\n", flagLang)
			}
			if !flagNoCache && !flags.noCache && cacheableLang(flagLang) {
				if ps, perr := openPodcastStore(cmd.Context()); perr == nil {
					_ = ps.UpsertTranscript(cmd.Context(), tr)
					if tr.Tier == transcript.TierPaid && tr.CostCredits > 0 {
						_ = ps.RecordSpend(cmd.Context(), tr.Provider, tr.URL, tr.CostCredits, estimateUSD(tr.Provider, tr.CostCredits))
					}
				}
			}

			// Format + emit.
			body := renderTranscript(tr, flagText, flagJSON, flagJSONL, flagMD)
			if flagOut != "" {
				if err := writeFile(flagOut, body); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), flagOut)
			} else if dest := defaultCachePath(tr, flagText, flagJSON, flagJSONL); dest != "" && cacheableLang(flagLang) && !isTerminal(cmd.OutOrStdout()) {
				// When stdout is piped and no explicit --out, also persist a cache copy.
				_ = writeFile(dest, body)
				fmt.Fprint(cmd.OutOrStdout(), body)
			} else {
				fmt.Fprint(cmd.OutOrStdout(), body)
			}

			if flagExplain {
				_ = emitTrace(cmd, flags, res)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagMD, "md", true, "Emit canonical speaker-labeled markdown (default)")
	cmd.Flags().BoolVar(&flagText, "text", false, "Emit plain text (no markdown)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "Emit full JSON document")
	cmd.Flags().BoolVar(&flagJSONL, "jsonl", false, "Emit one JSON segment per line")
	cmd.Flags().StringVar(&flagOut, "out", "", "Write to this file path (default: stdout + cache)")
	cmd.Flags().BoolVar(&flagPaid, "paid", false, "Allow paid-tier adapters (spoken, taddy, whisperapi)")
	cmd.Flags().BoolVar(&flagAutoPaid, "auto-paid", false, "Same as --paid; auto-confirm cost preview")
	cmd.Flags().StringVar(&flagProvider, "provider", "", "Comma-separated allowed adapters (e.g. spoken,youtube)")
	cmd.Flags().BoolVar(&flagExplain, "explain", false, "Print dispatcher trace alongside the transcript")
	cmd.Flags().StringVar(&flagLang, "lang", "", "Subtitle language for the YouTube source — one code per fetch, e.g. it (default: en). Rolling-cue de-dup applies to space-tokenized languages only; non-default languages are not written to the cache")
	cmd.Flags().BoolVar(&flagNoCache, "fetch-only", false, "Skip writing the result to the local cache")
	return cmd
}

// validateLangFlag enforces the v0.1 --lang contract: one language code per
// fetch. Comma lists would make yt-dlp write several VTT files and the
// adapter's file picker would grab an arbitrary one — silently returning the
// wrong language.
func validateLangFlag(lang string) error {
	if strings.ContainsAny(lang, ", \t") {
		return fmt.Errorf("--lang accepts one language code per fetch in v0.1 (got %q); run the command once per language", lang)
	}
	return nil
}

// cacheableLang reports whether a fetch with this --lang value may be written
// to the local store. Only the default language is cached: cache identity is
// per-URL with no language column, so other languages would overwrite it.
func cacheableLang(lang string) bool {
	return lang == "" || lang == "en"
}

func classifyDispatchErr(ctx context.Context, url string, err error) error {
	var cm *source.CookieMissingError
	if errors.As(err, &cm) {
		if hint := alternateSourceHint(ctx, url); hint != "" {
			return authErr(fmt.Errorf("%w\n%s", err, hint))
		}
		return authErr(err)
	}
	var km *source.KeyMissingError
	if errors.As(err, &km) {
		return authErr(err)
	}
	return err
}

func emitTrace(cmd *cobra.Command, flags *rootFlags, res *dispatch.DispatchResult) error {
	if flags != nil && flags.asJSON {
		out, _ := json.MarshalIndent(res, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Dispatch trace:")
	for _, e := range res.Trace {
		fmt.Fprintf(cmd.OutOrStdout(), "  [%s/%s] %s  %s\n", e.Tier, e.Source, e.Verdict, e.Reason)
	}
	if res.FiredBy != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "fired_by: %s\n", res.FiredBy)
	}
	return nil
}

func renderTranscript(t *transcript.Transcript, text, asJSON, jsonl, md bool) string {
	switch {
	case asJSON:
		return t.JSON() + "\n"
	case jsonl:
		return t.JSONL()
	case text:
		return t.PlainText()
	default:
		_ = md
		return t.CanonicalMarkdown()
	}
}

func defaultCachePath(t *transcript.Transcript, text, asJSON, jsonl bool) string {
	ext := "md"
	switch {
	case asJSON:
		ext = "json"
	case jsonl:
		ext = "jsonl"
	case text:
		ext = "txt"
	}
	if t == nil {
		return ""
	}
	dir := filepath.Join(podcastCacheDir(), t.Source)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	slug := t.ID
	if len(slug) > 16 {
		slug = slug[:16]
	}
	return filepath.Join(dir, slug+"."+ext)
}

func writeFile(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

// estimateUSD returns a coarse $ estimate from credit count, by provider.
func estimateUSD(provider string, credits float64) float64 {
	switch provider {
	case "spoken":
		return credits * 0.08
	case "taddy":
		return credits * 0.40 // $40/100 transcripts ~ $0.40 each
	case "whisperapi":
		return credits * 0.004 // ElevenLabs Scribe per-minute
	}
	return 0
}
