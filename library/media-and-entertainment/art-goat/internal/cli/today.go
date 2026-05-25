// Copyright 2026 justinwfu. Licensed under Apache-2.0.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

func newTodayCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var windowDays int
	var jsonOnly bool
	var mode string
	var reroll bool

	cmd := &cobra.Command{
		Use:   "today",
		Short: "Today's curated piece, chosen against your practice",
		Long: `Today's curated piece, chosen using anti-repeat against your recent sits
and journal-aware diversity against your recent medium and culture choices.
Includes a one-line 'why this today' derived from the local data — no
hand-authored weekly themes, no calendar themes, just the data.

Optional --mode shapes the pick policy:
  (none)              Default rotation — anti-repeat + source/medium/region diversity.
  bridge-from-last    Read your last sit's mood (1-5) and pick toward the opposite half
                      of the scale. Heavy (1-2) bridges to calmer (target 4); calm (4-5)
                      bridges to energizing (target 2); neutral (3) stays neutral.
                      Falls back to default rotation if no prior mood is recorded.

Idempotent within a calendar day per --mode: the first invocation picks and
caches; subsequent invocations on the same local date return the cached
work, why, and prompt verbatim. Pass --reroll to discard the cached pick
and re-roll. Switching --mode picks fresh for the new mode without
disturbing the prior mode's cache.

Use --json to emit a structured envelope without entering the interactive
sit flow. Use art-goat-pp-cli sit <id> after running today --json to start
a sit on the chosen piece.`,
		Example: `  # Show today's pick with the 'why this today' line
  art-goat-pp-cli today

  # Bridge across moods: opposite-half pick informed by your last sit
  art-goat-pp-cli today --mode bridge-from-last

  # Re-roll if the cached pick doesn't fit your mood
  art-goat-pp-cli today --reroll

  # Structured envelope for agent use
  art-goat-pp-cli today --json --select work_id,why,prompt

  # Avoid pieces sat with in the last 60 days (default 30)
  art-goat-pp-cli today --window 60`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() && !flags.asJSON && !jsonOnly {
				return emitTodayVerifyEnvelope(cmd, flags)
			}

			if mode != "" && mode != "bridge-from-last" {
				return usageErr(fmt.Errorf("invalid --mode %q: supported values: bridge-from-last", mode))
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

			pick, why, prompt, chosenAt, cached, err := resolveTodayPick(
				cmd.Context(), db, windowDays, mode, reroll,
			)
			if err != nil {
				return err
			}
			if pick == nil {
				return fmt.Errorf("no works available — run `art-goat-pp-cli sync` first")
			}

			envelope := map[string]any{
				"work_id":     pick.ID,
				"source":      pick.Source,
				"title":       pick.Title,
				"creator":     pick.Creator,
				"date":        pick.DateText,
				"medium":      pick.Medium,
				"region":      pick.CultureRegion,
				"image_url":   pick.ImageURL,
				"description": pick.Description,
				"why":         why,
				"prompt":      prompt,
				"chosen_at":   chosenAt.UTC().Format(time.RFC3339),
				"cached":      cached,
				"pick_date":   time.Now().Format("2006-01-02"),
			}
			if mode != "" {
				envelope["mode"] = mode
			}

			if flags.asJSON || jsonOnly {
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}
			renderToday(cmd, pick, why, prompt, cached, chosenAt)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/art-goat-pp-cli/data.db)")
	cmd.Flags().IntVar(&windowDays, "window", 30, "Anti-repeat window in days — avoid pieces sat with in this window")
	cmd.Flags().BoolVar(&jsonOnly, "json-only", false, "Emit JSON only (alias for --json scoped to this command)")
	cmd.Flags().StringVar(&mode, "mode", "", "Pick policy: (none) for default rotation, or 'bridge-from-last' for mood-aware rotation")
	cmd.Flags().BoolVar(&reroll, "reroll", false, "Discard the cached daily pick (if any) and re-roll")
	return cmd
}

// resolveTodayPick implements the daily-pick cache: a (date, mode) key
// in today_picks is checked first; on hit the cached work, why, and
// prompt are returned verbatim (cached=true). On miss — or when
// --reroll discards the cache — a fresh pick is computed and saved.
//
// The cache key is the writer's LOCAL date so the contemplative
// practice's "today" matches what the user reads on their wall clock,
// not UTC midnight. mode is taken verbatim so distinct policies
// (empty string for default rotation, "bridge-from-last", etc.) cache
// independently.
//
// If the cached work_id no longer resolves in the works table (rare:
// a sync replaced or removed it), we treat the cache as stale and pick
// fresh. This avoids returning a phantom pick that no other command
// can act on.
func resolveTodayPick(
	ctx context.Context, db *store.Store, windowDays int, mode string, reroll bool,
) (pick *store.Work, why, prompt string, chosenAt time.Time, cached bool, err error) {
	today := time.Now().Format("2006-01-02")

	if reroll {
		if _, derr := db.ClearTodayPick(ctx, today, mode); derr != nil {
			return nil, "", "", time.Time{}, false, derr
		}
	} else {
		hit, gerr := db.GetTodayPick(ctx, today, mode)
		if gerr != nil {
			return nil, "", "", time.Time{}, false, gerr
		}
		if hit != nil {
			w, werr := db.GetWork(ctx, hit.WorkID)
			if werr != nil {
				return nil, "", "", time.Time{}, false, werr
			}
			if w != nil {
				return w, hit.Why, hit.Prompt, hit.ChosenAt, true, nil
			}
			// Stale cache (work missing) — wipe and fall through to fresh pick.
			if _, derr := db.ClearTodayPick(ctx, today, mode); derr != nil {
				return nil, "", "", time.Time{}, false, derr
			}
		}
	}

	pick, why, err = pickTodayWorkWithMode(ctx, db, windowDays, mode)
	if err != nil || pick == nil {
		return pick, why, "", time.Time{}, false, err
	}
	prompt = pickPrompt(promptSeed(pick.ID))
	chosenAt = time.Now().UTC()
	saveErr := db.SaveTodayPick(ctx, store.TodayPick{
		Date:     today,
		Mode:     mode,
		WorkID:   pick.ID,
		Why:      why,
		Prompt:   prompt,
		ChosenAt: chosenAt,
	})
	if saveErr != nil {
		// Don't fail the read just because the cache write failed —
		// emit a warning on stderr and return the fresh pick so the
		// user still gets today's piece. Cache-write failure is
		// recoverable on the next invocation.
		fmt.Fprintf(os.Stderr, "warning: failed to cache today's pick: %v\n", saveErr)
	}
	return pick, why, prompt, chosenAt, false, nil
}

// pickTodayWorkWithMode dispatches between the default rotation and
// mode-specific pickers. When mode is empty, behavior is identical to
// pickTodayWork (the historical default).
//
// `bridge-from-last` reads the last sit's mood (1-5) and biases the
// pick toward the opposite half of the scale, using the journal's
// historical mood-by-source / mood-by-medium / mood-by-region averages
// as a proxy for what a candidate work is likely to evoke. With no
// prior mood data the function falls through to the default rotation —
// the bridge policy is additive, not a hard requirement.
func pickTodayWorkWithMode(ctx context.Context, db *store.Store, windowDays int, mode string) (*store.Work, string, error) {
	if mode == "bridge-from-last" {
		return pickBridgeFromLastMood(ctx, db, windowDays)
	}
	return pickTodayWork(ctx, db, windowDays)
}

// pickTodayWork applies anti-repeat against recent sit work_ids and a
// journal-aware diversity rule against the last 2-3 sits' (source,
// medium, region). Returns the chosen work plus a one-line "why this
// today" derived from the data.
func pickTodayWork(ctx context.Context, db *store.Store, windowDays int) (*store.Work, string, error) {
	recentIDs, err := db.RecentSitWorkIDs(ctx, windowDays)
	if err != nil {
		return nil, "", err
	}
	lastSits, err := db.SearchSits(ctx, "", 3)
	if err != nil {
		return nil, "", err
	}

	// Collect "fingerprint" from recent sits: source, medium, region.
	recentSources := map[string]bool{}
	recentMediums := map[string]bool{}
	recentRegions := map[string]bool{}
	for _, sit := range lastSits {
		if sit.WorkID == "" {
			continue
		}
		w, err := db.GetWork(ctx, sit.WorkID)
		if err != nil || w == nil {
			continue
		}
		recentSources[w.Source] = true
		recentMediums[strings.ToLower(w.Medium)] = true
		recentRegions[strings.ToLower(w.CultureRegion)] = true
	}

	// Try up to 30 random picks, score each against the diversity rule.
	// Pick the first one that differs from the recent fingerprint on at
	// least one dimension (source / medium / region). Fall through to
	// pure random if none of the 30 differ.
	var best *store.Work
	var bestScore int = -1
	for i := 0; i < 30; i++ {
		w, err := db.RandomWork(ctx, nil, recentIDs)
		if err != nil {
			return nil, "", err
		}
		if w == nil {
			break
		}
		score := 0
		if !recentSources[w.Source] {
			score++
		}
		if !recentMediums[strings.ToLower(w.Medium)] {
			score++
		}
		if !recentRegions[strings.ToLower(w.CultureRegion)] {
			score++
		}
		if score > bestScore {
			best = w
			bestScore = score
		}
		if score == 3 {
			break
		}
	}
	if best == nil {
		// Anti-repeat eliminated everything (sat with every work in window) —
		// pick a pure random.
		best, err = db.RandomWork(ctx, nil, nil)
		if err != nil {
			return nil, "", err
		}
	}
	if best == nil {
		return nil, "", nil
	}

	// Compose "why this today" from what's different about the pick.
	why := composeWhy(best, recentSources, recentMediums, recentRegions, len(lastSits))
	return best, why, nil
}

func composeWhy(w *store.Work, recentSources, recentMediums, recentRegions map[string]bool, sitsSoFar int) string {
	if sitsSoFar == 0 {
		return "first sit — anything goes; this one came up randomly."
	}
	var reasons []string
	if w.CultureRegion != "" && !recentRegions[strings.ToLower(w.CultureRegion)] {
		reasons = append(reasons, fmt.Sprintf("you haven't sat with %s recently", w.CultureRegion))
	}
	if w.Medium != "" && !recentMediums[strings.ToLower(w.Medium)] {
		reasons = append(reasons, fmt.Sprintf("different medium (%s)", w.Medium))
	}
	if w.Source != "" && !recentSources[w.Source] {
		reasons = append(reasons, fmt.Sprintf("different source (%s)", w.Source))
	}
	if len(reasons) == 0 {
		return "no recent overlap — anti-repeat brought this one forward."
	}
	return strings.Join(reasons, "; ") + "."
}

func renderToday(cmd *cobra.Command, w *store.Work, why, prompt string, cached bool, chosenAt time.Time) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Today: %s", coalesce(w.Title, "(untitled)"))
	if w.Creator != "" {
		fmt.Fprintf(out, " — %s", w.Creator)
	}
	if w.DateText != "" {
		fmt.Fprintf(out, " (%s)", w.DateText)
	}
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Source: %s · %s\n", w.Source, w.ID)
	if w.SourceURL != "" {
		// Pad the label so the URL stays column-aligned with "Source:" and "Why:".
		label := linkLabelFor(w.Source, w.SourceURL)
		fmt.Fprintf(out, "%-7s %s\n", label+":", w.SourceURL)
	}
	fmt.Fprintf(out, "Why:    %s\n", why)
	if cached && !chosenAt.IsZero() {
		fmt.Fprintf(out, "Cached: chosen %s. Re-roll with --reroll.\n", humanAgo(chosenAt))
	}
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Prompt:")
	fmt.Fprintf(out, "  %s\n", prompt)
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Start the sit:  art-goat-pp-cli sit %s\n", w.ID)
	fmt.Fprintln(out, "")
}

// bridgeTargetMood maps the last sit's mood (1-5) onto a target mood
// for the opposite half of the scale. The mapping is deliberately
// coarse: heavy (1-2) targets calm/light (4), calm (4-5) targets
// energizing (2), neutral (3) stays neutral. Returning 0 means "no
// bridge available" (last mood not in 1..5) and triggers the fallback.
func bridgeTargetMood(lastMood int) int {
	switch {
	case lastMood <= 0 || lastMood > 5:
		return 0
	case lastMood <= 2:
		return 4
	case lastMood >= 4:
		return 2
	default:
		return 3
	}
}

// pickBridgeFromLastMood implements the bridge-from-last policy.
//
// Algorithm:
//  1. Read the last sit; if it has no mood, fall back to default rotation.
//  2. Compute the target mood and pull historical mood-by-dimension averages.
//  3. Sample up to 30 random candidates (anti-repeat against the recent window).
//  4. Score each candidate by (a) source/medium/region diversity vs. the
//     last 3 sits (same as default rotation) and (b) closeness of its
//     dimension averages to the target mood. The mood term is folded
//     into a single float score; the candidate maximizing it wins.
//  5. If no mood data exists for the candidate's dimensions, fall back
//     to a pure diversity score so the picker still produces a pick.
//
// The composite score is `diversityScore - moodDistance`, so a closer
// mood match wins among same-diversity candidates and a wider diversity
// gap can outweigh a small mood-mismatch (intentional — we don't want
// the bridge to defeat anti-repeat).
func pickBridgeFromLastMood(ctx context.Context, db *store.Store, windowDays int) (*store.Work, string, error) {
	last, err := db.LastSit(ctx)
	if err != nil {
		return nil, "", err
	}
	if last == nil || !last.Mood.Valid {
		// No prior mood → fall through to default rotation with a why
		// line that notes the fallback so users / agents know why the
		// bridge policy didn't activate.
		w, why, err := pickTodayWork(ctx, db, windowDays)
		if err != nil || w == nil {
			return w, why, err
		}
		return w, "bridge-from-last requested but no prior mood recorded; " + why, nil
	}
	lastMood := int(last.Mood.Int64)
	target := bridgeTargetMood(lastMood)
	if target == 0 {
		w, why, err := pickTodayWork(ctx, db, windowDays)
		if err != nil || w == nil {
			return w, why, err
		}
		return w, "bridge-from-last requested but last mood out of range; " + why, nil
	}

	recentIDs, err := db.RecentSitWorkIDs(ctx, windowDays)
	if err != nil {
		return nil, "", err
	}
	lastSits, err := db.SearchSits(ctx, "", 3)
	if err != nil {
		return nil, "", err
	}
	recentSources, recentMediums, recentRegions := buildRecentFingerprint(ctx, db, lastSits)

	moodBySource, moodByMedium, moodByRegion, err := db.MoodAveragesByDimension(ctx)
	if err != nil {
		return nil, "", err
	}

	var best *store.Work
	var bestScore = math.Inf(-1)
	for i := 0; i < 30; i++ {
		w, err := db.RandomWork(ctx, nil, recentIDs)
		if err != nil {
			return nil, "", err
		}
		if w == nil {
			break
		}
		score := bridgeCandidateScore(w, target, moodBySource, moodByMedium, moodByRegion,
			recentSources, recentMediums, recentRegions)
		if score > bestScore {
			best = w
			bestScore = score
		}
	}
	if best == nil {
		// Anti-repeat eliminated everything — pure random.
		best, err = db.RandomWork(ctx, nil, nil)
		if err != nil {
			return nil, "", err
		}
	}
	if best == nil {
		return nil, "", nil
	}

	why := composeBridgeWhy(lastMood, target, best, moodBySource, moodByMedium, moodByRegion,
		recentSources, recentMediums, recentRegions)
	return best, why, nil
}

// buildRecentFingerprint pulls the (source, medium, region) maps from
// the last few sits' works, mirroring the inline logic in pickTodayWork
// so the bridge picker and default picker share the same anti-overlap
// signal. Errors fetching a missing work are silently skipped — the
// fingerprint is best-effort, not load-bearing.
func buildRecentFingerprint(ctx context.Context, db *store.Store, sits []store.Sit) (
	sources, mediums, regions map[string]bool,
) {
	sources = map[string]bool{}
	mediums = map[string]bool{}
	regions = map[string]bool{}
	for _, sit := range sits {
		if sit.WorkID == "" {
			continue
		}
		w, err := db.GetWork(ctx, sit.WorkID)
		if err != nil || w == nil {
			continue
		}
		sources[w.Source] = true
		mediums[strings.ToLower(w.Medium)] = true
		regions[strings.ToLower(w.CultureRegion)] = true
	}
	return sources, mediums, regions
}

// bridgeCandidateScore composes diversity + mood-proximity for one
// candidate work. Diversity contributes 0..3 (one point per axis the
// candidate differs from the recent fingerprint). Mood distance is the
// candidate's average dimensional mood vs. the target, normalized to
// 0..4 (range of mood scale). Final score is diversity - moodDistance;
// higher is better.
func bridgeCandidateScore(
	w *store.Work, target int,
	moodBySource, moodByMedium, moodByRegion map[string]float64,
	recentSources, recentMediums, recentRegions map[string]bool,
) float64 {
	diversity := 0.0
	if !recentSources[w.Source] {
		diversity++
	}
	if !recentMediums[strings.ToLower(w.Medium)] {
		diversity++
	}
	if !recentRegions[strings.ToLower(w.CultureRegion)] {
		diversity++
	}

	dist, ok := candidateMoodDistance(w, target, moodBySource, moodByMedium, moodByRegion)
	if !ok {
		// No mood data → score is just diversity.
		return diversity
	}
	return diversity - dist
}

// candidateMoodDistance returns the absolute distance from the
// candidate's expected mood (the average over whichever of its
// source/medium/region have historical mood data) to the target mood.
// Returns (0, false) when none of the three dimensions have data.
func candidateMoodDistance(
	w *store.Work, target int,
	moodBySource, moodByMedium, moodByRegion map[string]float64,
) (float64, bool) {
	var samples []float64
	if v, ok := moodBySource[w.Source]; ok {
		samples = append(samples, v)
	}
	if v, ok := moodByMedium[strings.ToLower(w.Medium)]; ok {
		samples = append(samples, v)
	}
	if v, ok := moodByRegion[strings.ToLower(w.CultureRegion)]; ok {
		samples = append(samples, v)
	}
	if len(samples) == 0 {
		return 0, false
	}
	var sum float64
	for _, s := range samples {
		sum += s
	}
	avg := sum / float64(len(samples))
	d := avg - float64(target)
	if d < 0 {
		d = -d
	}
	return d, true
}

// composeBridgeWhy renders the one-line "why this today" for the
// bridge-from-last pick. Always opens with the mood→target framing so
// the user / agent can see what policy is in play, then folds in the
// usual diversity reasoning.
func composeBridgeWhy(
	lastMood, target int, w *store.Work,
	moodBySource, moodByMedium, moodByRegion map[string]float64,
	recentSources, recentMediums, recentRegions map[string]bool,
) string {
	parts := []string{fmt.Sprintf("bridging from last sit mood %d → target %d", lastMood, target)}
	if dist, ok := candidateMoodDistance(w, target, moodBySource, moodByMedium, moodByRegion); ok {
		parts = append(parts, fmt.Sprintf("candidate's dimensional avg %.1f off target", dist))
	} else {
		parts = append(parts, "no mood history for this candidate's source/medium/region — picked on diversity")
	}
	if w.CultureRegion != "" && !recentRegions[strings.ToLower(w.CultureRegion)] {
		parts = append(parts, "new region ("+w.CultureRegion+")")
	}
	if w.Medium != "" && !recentMediums[strings.ToLower(w.Medium)] {
		parts = append(parts, "new medium ("+w.Medium+")")
	}
	if w.Source != "" && !recentSources[w.Source] {
		parts = append(parts, "new source ("+w.Source+")")
	}
	return strings.Join(parts, "; ") + "."
}

func emitTodayVerifyEnvelope(cmd *cobra.Command, flags *rootFlags) error {
	envelope := map[string]any{
		"command":                 "today",
		"verify_noop":             true,
		"success":                 true,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "today renders to terminal by default; PRINTING_PRESS_VERIFY=1 short-circuits the rendering. Pass --json to get the data envelope.",
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
