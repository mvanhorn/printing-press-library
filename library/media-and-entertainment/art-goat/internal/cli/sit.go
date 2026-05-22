// Copyright 2026 justinwfu. Licensed under Apache-2.0.

package cli

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

func newSitCmd(flags *rootFlags) *cobra.Command {
	var durationStr string
	var noImage bool
	var dryRunFlag bool
	var dbPath string
	var modeFlag string
	var sourceFilter string
	var launch bool
	var inline bool

	cmd := &cobra.Command{
		Use:   "sit [id]",
		Short: "Sit with one piece of art for a fixed period",
		Long: `Sit with one piece of art for a fixed period of extended attention.

By default, picks a random work from the local corpus and runs a quiet
timer (10 minutes). After the timer, you can write a reflection that's
saved to the local journal.

Pass an explicit work ID to sit with a specific piece, or no argument to
let art-goat pick. Set --source aic or --source apod to constrain to one
source. Use --dry-run to inspect what would be shown without starting the
timer or writing to the journal.`,
		Example: `  # Sit with a random piece for 10 minutes (default)
  art-goat-pp-cli sit

  # Shorten the timer
  art-goat-pp-cli sit aic:24645 --duration 5m

  # Constrain to a single source
  art-goat-pp-cli sit --source apod

  # Inspect without starting the timer or writing to the journal
  art-goat-pp-cli sit --dry-run --json`,
		Annotations: map[string]string{
			"mcp:hidden": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().Changed("help") {
				return cmd.Help()
			}
			// Verify-mode floor: never block on stdin or open a browser
			// during a verify pass.
			if cliutil.IsVerifyEnv() {
				return emitSitVerifyEnvelope(cmd, args, flags)
			}

			duration, err := parseSitDuration(durationStr)
			if err != nil {
				return fmt.Errorf("invalid --duration: %w", err)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("art-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureArtGoatTables(cmd.Context()); err != nil {
				return err
			}

			work, err := pickSitWork(cmd, db, args, sourceFilter)
			if err != nil {
				return err
			}
			if work == nil {
				return fmt.Errorf("no works available — run `art-goat-pp-cli sync` first")
			}

			prompt := pickPrompt(promptSeed(work.ID))

			// --dry-run / --json short-circuit: emit a structured envelope.
			if dryRunFlag || flags.dryRun || flags.asJSON {
				envelope := map[string]any{
					"work_id":     work.ID,
					"source":      work.Source,
					"title":       work.Title,
					"creator":     work.Creator,
					"date":        work.DateText,
					"medium":      work.Medium,
					"region":      work.CultureRegion,
					"image_url":   work.ImageURL,
					"description": work.Description,
					"prompt":      prompt,
					"duration_s":  int(duration.Seconds()),
					"mode":        modeFlag,
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
				}
				renderSitPreview(cmd, work, prompt, duration, noImage)
				return nil
			}

			// Optional per-sit streak greeting (off by default; opt-in via
			// `journal opt-in --show-streak`).
			if prefs, _ := loadJournalPrefs(); prefs.ShowStreak {
				if streak, _ := db.CurrentStreak(cmd.Context()); streak > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Streak: %d day%s.\n", streak, pluralS(streak))
				}
			}

			// Render the piece. Default: description-mode in terminal. With
			// --launch, also write an HTML page and open it in the browser.
			// With --inline, attempt terminal-graphics emit for iTerm2 /
			// Kitty / imgcat.
			renderSitView(cmd, work, prompt, noImage)
			if launch && !noImage {
				if path, err := writeSitHTML(work, prompt); err == nil {
					if oerr := openInBrowser(path); oerr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "note: --launch wrote %s but could not open it: %v\n", path, oerr)
					}
				}
			}
			if inline && !noImage && work.ImageURL != "" {
				_ = emitInlineImage(cmd.OutOrStdout(), work.ImageURL)
			}

			// Run the timer. Capture wall-clock start BEFORE the timer so
			// StartedAt is accurate regardless of how long the post-sit
			// reflection dialogue takes. Ctrl-C is an advertised "stop
			// early" path — when it fires we fall through to capture a
			// partial reflection so the user's practice record isn't lost
			// (the sit happened, it just ended early).
			startedAt := time.Now().UTC()
			timerErr := runSitTimer(cmd, duration)
			endedAt := time.Now().UTC()
			endedEarly := false
			if timerErr != nil {
				if errors.Is(timerErr, context.Canceled) {
					endedEarly = true
					fmt.Fprintln(cmd.OutOrStdout(), "\nSit ended early — capturing what you have.")
				} else {
					return timerErr
				}
			}
			actualDuration := endedAt.Sub(startedAt)

			// Capture reflection. Stdin is independent of cmd.Context()
			// so this still works after Ctrl-C cancellation.
			reflection, mood, tags := captureReflection(cmd)

			sit := store.Sit{
				StartedAt:       startedAt,
				EndedAt:         sql.NullTime{Time: endedAt, Valid: true},
				WorkID:          work.ID,
				DurationSeconds: int(actualDuration.Seconds()),
				Prompt:          prompt,
				Reflection:      reflection,
				Tags:            tags,
				Mode:            modeFlag,
			}
			if mood > 0 {
				sit.Mood = sql.NullInt64{Int64: int64(mood), Valid: true}
			}
			// Use a fresh context for the save — if Ctrl-C canceled
			// cmd.Context(), passing it through would kill InsertSit and
			// drop the very entry we worked to capture.
			saveCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			id, err := db.InsertSit(saveCtx, sit)
			if err != nil {
				return fmt.Errorf("save reflection: %w", err)
			}
			if endedEarly {
				fmt.Fprintf(cmd.OutOrStdout(), "\nSaved as journal entry #%d (sit ended early after %s).\n", id, actualDuration.Round(time.Second))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "\nSaved as journal entry #%d.\n", id)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&durationStr, "duration", "10m", "Sit duration (default 10m): bare integer is minutes (e.g. 5), or use Go units (5m, 20m, 30s)")
	cmd.Flags().BoolVar(&noImage, "no-image", false, "Skip image display; use description-only mode")
	cmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Show the chosen piece + prompt without starting the timer or writing the journal")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/art-goat-pp-cli/data.db)")
	cmd.Flags().StringVar(&modeFlag, "mode", "atomic", "Sit mode: atomic | phased | bare")
	cmd.Flags().StringVar(&sourceFilter, "source", "", "Constrain to one source (e.g. aic, apod). Empty = all sources.")
	cmd.Flags().BoolVar(&launch, "launch", false, "Also write an HTML page for this work and open it in the browser")
	cmd.Flags().BoolVar(&inline, "inline", false, "Attempt to render the image inline in supported terminals (iTerm2, Kitty, imgcat)")
	return cmd
}

func pickSitWork(cmd *cobra.Command, db *store.Store, args []string, sourceFilter string) (*store.Work, error) {
	if len(args) > 0 && args[0] != "" {
		w, err := db.GetWork(cmd.Context(), args[0])
		if err != nil {
			return nil, err
		}
		if w == nil {
			return nil, fmt.Errorf("no work found with id %q (try `sync` first or check the id format <source>:<id>)", args[0])
		}
		return w, nil
	}
	var sources []string
	if sourceFilter != "" {
		sources = []string{sourceFilter}
	}
	return db.RandomWork(cmd.Context(), sources, nil)
}

func renderSitPreview(cmd *cobra.Command, w *store.Work, prompt string, duration time.Duration, noImage bool) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "── art-goat sit (dry run) ──")
	fmt.Fprintf(out, "%s\n", coalesce(w.Title, "(untitled)"))
	if w.Creator != "" {
		fmt.Fprintf(out, "  by %s\n", w.Creator)
	}
	if w.DateText != "" {
		fmt.Fprintf(out, "  %s\n", w.DateText)
	}
	fmt.Fprintf(out, "  %s · %s\n", w.Source, w.ID)
	fmt.Fprintf(out, "Duration: %s\n", duration)
	fmt.Fprintf(out, "Prompt: %s\n", prompt)
	if !noImage && w.ImageURL != "" {
		fmt.Fprintf(out, "Image: %s\n", w.ImageURL)
	}
	if w.Description != "" {
		fmt.Fprintf(out, "\n%s\n", truncateForPreview(w.Description, 600))
	}
}

func renderSitView(cmd *cobra.Command, w *store.Work, prompt string, noImage bool) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "──────────────────────────────────────────────────────────")
	fmt.Fprintf(out, "  %s\n", coalesce(w.Title, "(untitled)"))
	if w.Creator != "" {
		fmt.Fprintf(out, "  %s\n", w.Creator)
	}
	if w.DateText != "" {
		fmt.Fprintf(out, "  %s\n", w.DateText)
	}
	if w.Medium != "" {
		fmt.Fprintf(out, "  %s\n", w.Medium)
	}
	if w.CultureRegion != "" {
		fmt.Fprintf(out, "  %s\n", w.CultureRegion)
	}
	fmt.Fprintf(out, "  %s\n", w.SourceURL)
	fmt.Fprintln(out, "──────────────────────────────────────────────────────────")
	if w.Description != "" {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, paragraphWrap(w.Description, 76))
	}
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Prompt: %s\n", prompt)
	if !noImage && w.ImageURL != "" {
		fmt.Fprintf(out, "Image:  %s\n", w.ImageURL)
		fmt.Fprintln(out, "(Open the image URL in your browser if you want to see the piece while you sit.)")
	}
	fmt.Fprintln(out, "")
}

func runSitTimer(cmd *cobra.Command, duration time.Duration) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Timer running: %s. Press Ctrl-C to stop early.\n", duration)
	select {
	case <-time.After(duration):
		fmt.Fprintln(cmd.OutOrStdout(), "Timer finished.")
		return nil
	case <-cmd.Context().Done():
		return cmd.Context().Err()
	}
}

func captureReflection(cmd *cobra.Command) (reflection string, mood int, tags string) {
	out := cmd.OutOrStdout()
	in := bufio.NewReader(os.Stdin)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Write your reflection. Single line; leave blank to skip.")
	fmt.Fprint(out, "> ")
	line, _ := in.ReadString('\n')
	reflection = strings.TrimSpace(line)

	fmt.Fprint(out, "Mood (1-5, blank to skip): ")
	moodLine, _ := in.ReadString('\n')
	if v, err := readSmallInt(strings.TrimSpace(moodLine)); err == nil && v >= 1 && v <= 5 {
		mood = v
	}

	fmt.Fprint(out, "Tags (comma-separated, blank to skip): ")
	tagLine, _ := in.ReadString('\n')
	tags = strings.TrimSpace(tagLine)
	return reflection, mood, tags
}

func emitSitVerifyEnvelope(cmd *cobra.Command, args []string, flags *rootFlags) error {
	envelope := map[string]any{
		"command":                 "sit",
		"args":                    args,
		"verify_noop":             true,
		"success":                 false,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "sit is interactive; PRINTING_PRESS_VERIFY=1 short-circuits before opening browsers, running timers, or writing the journal",
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

func parseSitDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 5 * time.Minute, nil
	}
	// Accept a bare integer as minutes — the natural way a user types
	// `--duration 10`. time.ParseDuration requires a unit suffix.
	if n, err := strconv.Atoi(s); err == nil {
		if n <= 0 {
			return 0, fmt.Errorf("duration must be positive")
		}
		return time.Duration(n) * time.Minute, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return d, nil
}

func promptSeed(workID string) int64 {
	// Deterministic seed: the prompt for a given (workID, date) pair is
	// stable so re-running `sit <id>` on the same day shows the same
	// prompt. Different day yields different prompt.
	day := time.Now().UTC().Format("2006-01-02")
	hash := int64(0)
	for _, r := range workID + ":" + day {
		hash = hash*131 + int64(r)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

func readSmallInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, err
	}
	return n, nil
}

func coalesce(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// truncateForPreview returns the first n runes of s followed by an
// ellipsis. Operates on []rune (not []byte) so a multibyte character
// can't be split mid-rune — NPM Taipei titles contain Chinese
// characters, AIC's free-text descriptions contain typographic
// punctuation, and APOD copyright fields sometimes carry accented
// names. AGENTS.md mandates UTF-8-safe truncation.
func truncateForPreview(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

func paragraphWrap(s string, width int) string {
	var b strings.Builder
	line := 0
	for _, w := range strings.Fields(s) {
		if line == 0 {
			b.WriteString(w)
			line = len(w)
			continue
		}
		if line+1+len(w) > width {
			b.WriteRune('\n')
			b.WriteString(w)
			line = len(w)
			continue
		}
		b.WriteRune(' ')
		b.WriteString(w)
		line += 1 + len(w)
	}
	return b.String()
}
