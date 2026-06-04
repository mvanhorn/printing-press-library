// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

// journal revisit and journal compare — longitudinal journal commands.
// Both read from the local store; neither writes.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

func newJournalRevisitCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var ageStr string
	var windowDays int

	cmd := &cobra.Command{
		Use:   "revisit",
		Short: "Surface the sit closest to a past anchor (e.g. one year ago)",
		Long: `Find the sit whose started_at is closest to (today - --age) within a
±windowDays day window. Useful for the contemplative practice of looking
back: "what was I noticing on this day a year ago?"

--age accepts a positive value with one of the suffixes d (days), w
(weeks), mo (months), y (years). Examples: 7d, 2w, 6mo, 1y.

Errors with exit code 3 (not found) when no sit falls inside the
window — the journal has no anchor at that depth.`,
		Example: `  art-goat-pp-cli journal revisit --age 1y
  art-goat-pp-cli journal revisit --age 6mo
  art-goat-pp-cli journal revisit --age 30d --window 14 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() && !flags.asJSON {
				return emitJournalRevisitVerifyEnvelope(cmd, flags)
			}
			if strings.TrimSpace(ageStr) == "" {
				return usageErr(fmt.Errorf("--age is required (e.g. 1y, 6mo, 30d)"))
			}
			ageDur, err := parseHumanAge(ageStr)
			if err != nil {
				return usageErr(err)
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

			target := time.Now().UTC().Add(-ageDur)
			sit, err := db.SitNearestToDate(cmd.Context(), target, windowDays)
			if err != nil {
				return err
			}
			if sit == nil {
				return notFoundErr(fmt.Errorf(
					"no sit within ±%d days of %s — journal doesn't reach back that far",
					windowDays, target.Format("2006-01-02"),
				))
			}

			var work *store.Work
			if sit.WorkID != "" {
				work, _ = db.GetWork(cmd.Context(), sit.WorkID)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), revisitEnvelope(ageStr, ageDur, target, windowDays, sit, work), flags)
			}
			renderJournalRevisit(cmd, ageStr, target, windowDays, sit, work)
			return nil
		},
	}

	cmd.Flags().StringVar(&ageStr, "age", "", "How far back to look: 7d, 2w, 6mo, 1y")
	cmd.Flags().IntVar(&windowDays, "window", 7, "Match window in days around the target date")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// parseHumanAge accepts "Nd", "Nw", "Nmo", "Ny" with case-insensitive
// suffix. Returns a positive duration; the caller subtracts from now.
//
// "mo" must be checked before "m" or "d" because a naive single-letter
// match on the last byte would alias "6mo" to "6m" (minutes — wrong).
// time.ParseDuration handles seconds/minutes/hours but not days, so we
// roll our own and reject anything outside the contemplative-practice
// vocabulary.
func parseHumanAge(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("--age cannot be empty")
	}
	suffixes := []struct {
		suf string
		dur time.Duration
	}{
		{"mo", 30 * 24 * time.Hour},
		{"y", 365 * 24 * time.Hour},
		{"w", 7 * 24 * time.Hour},
		{"d", 24 * time.Hour},
	}
	for _, sx := range suffixes {
		if strings.HasSuffix(s, sx.suf) {
			numStr := strings.TrimSuffix(s, sx.suf)
			n, err := strconv.Atoi(numStr)
			if err != nil || n <= 0 {
				return 0, fmt.Errorf("invalid --age %q: prefix must be a positive integer", s)
			}
			return time.Duration(n) * sx.dur, nil
		}
	}
	return 0, fmt.Errorf("invalid --age %q: expected a positive integer followed by d, w, mo, or y", s)
}

func revisitEnvelope(ageStr string, ageDur time.Duration, target time.Time, windowDays int, sit *store.Sit, work *store.Work) map[string]any {
	out := map[string]any{
		"age":             ageStr,
		"age_days":        int(ageDur.Hours() / 24),
		"target_date":     target.Format("2006-01-02"),
		"window_days":     windowDays,
		"sit":             sitToEnvelope(*sit),
		"days_off_target": daysBetween(sit.StartedAt, target),
	}
	if work != nil {
		out["work"] = workToEnvelope(*work)
	}
	return out
}

func daysBetween(a, b time.Time) int {
	d := a.Sub(b)
	if d < 0 {
		d = -d
	}
	return int(d.Hours() / 24)
}

func renderJournalRevisit(cmd *cobra.Command, ageStr string, target time.Time, windowDays int, sit *store.Sit, work *store.Work) {
	out := cmd.OutOrStdout()
	delta := daysBetween(sit.StartedAt, target)
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Revisit: %s ago (anchor %s, window ±%dd)\n", ageStr, target.Format("2006-01-02"), windowDays)
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Sit #%d  ·  %s  ·  %d day%s off anchor\n",
		sit.ID, sit.StartedAt.Format("2006-01-02 15:04 MST"), delta, pluralS(delta))
	if work != nil {
		fmt.Fprintf(out, "Work: %s — %s\n",
			coalesce(work.Title, "(untitled)"),
			coalesce(work.Creator, "(unknown)"),
		)
		fmt.Fprintf(out, "Source: %s · %s\n", work.Source, work.ID)
	} else if sit.WorkID != "" {
		fmt.Fprintf(out, "Work: %s (not in local store — try `sync`)\n", sit.WorkID)
	}
	if sit.Prompt != "" {
		fmt.Fprintln(out, "")
		fmt.Fprintf(out, "Prompt: %s\n", sit.Prompt)
	}
	if sit.Reflection != "" {
		fmt.Fprintln(out, "")
		fmt.Fprintf(out, "Reflection: %s\n", sit.Reflection)
	}
	if sit.Mood.Valid {
		fmt.Fprintln(out, "")
		fmt.Fprintf(out, "Mood: %d/5\n", sit.Mood.Int64)
	}
	if sit.Tags != "" {
		fmt.Fprintln(out, "")
		fmt.Fprintf(out, "Tags: %s\n", sit.Tags)
	}
	fmt.Fprintln(out, "")
}

func emitJournalRevisitVerifyEnvelope(cmd *cobra.Command, flags *rootFlags) error {
	envelope := map[string]any{
		"command":                 "journal revisit",
		"verify_noop":             true,
		"success":                 true,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "journal revisit reads the local store; PRINTING_PRESS_VERIFY=1 short-circuits the rendering. Pass --json to get the data envelope.",
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

// newJournalCompareCmd implements `journal compare <id-a> <id-b>` — see
// journal_compare.go for the renderer.
func newJournalCompareCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "compare <sit-id-a> <sit-id-b>",
		Short: "Side-by-side comparison of two journal sits",
		Long: `Render two past sits side-by-side: each sit's work, prompt,
reflection, mood, and tags. Turns the journal from a flat list into a
longitudinal log — useful when you want to read your own response to
two pieces against each other (across days, sources, or moods).

Sit IDs are the numeric row IDs from 'journal search' (or the value
printed by 'journal write' when capturing a reflection).`,
		Example: `  art-goat-pp-cli journal compare 12 47
  art-goat-pp-cli journal compare 1 145 --json`,
		Args: cobra.ExactArgs(2),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() && !flags.asJSON {
				return emitJournalCompareVerifyEnvelope(cmd, flags)
			}
			idA, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("sit-id-a must be an integer, got %q", args[0]))
			}
			idB, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("sit-id-b must be an integer, got %q", args[1]))
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

			a, err := db.GetSit(cmd.Context(), idA)
			if err != nil {
				return err
			}
			if a == nil {
				return notFoundErr(fmt.Errorf("sit #%d not found", idA))
			}
			b, err := db.GetSit(cmd.Context(), idB)
			if err != nil {
				return err
			}
			if b == nil {
				return notFoundErr(fmt.Errorf("sit #%d not found", idB))
			}

			var workA, workB *store.Work
			if a.WorkID != "" {
				workA, _ = db.GetWork(cmd.Context(), a.WorkID)
			}
			if b.WorkID != "" {
				workB, _ = db.GetWork(cmd.Context(), b.WorkID)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), journalCompareEnvelope(a, workA, b, workB), flags)
			}
			renderJournalCompare(cmd, a, workA, b, workB)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func journalCompareEnvelope(a *store.Sit, workA *store.Work, b *store.Sit, workB *store.Work) map[string]any {
	envA := sitToEnvelope(*a)
	envB := sitToEnvelope(*b)
	if workA != nil {
		envA["work"] = workToEnvelope(*workA)
	}
	if workB != nil {
		envB["work"] = workToEnvelope(*workB)
	}
	return map[string]any{"a": envA, "b": envB}
}

func renderJournalCompare(cmd *cobra.Command, a *store.Sit, workA *store.Work, b *store.Sit, workB *store.Work) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "A: sit #%d (%s)\n", a.ID, a.StartedAt.Format("2006-01-02"))
	fmt.Fprintf(out, "B: sit #%d (%s)\n", b.ID, b.StartedAt.Format("2006-01-02"))
	fmt.Fprintln(out, "")

	rows := []compareField{
		{"Date", a.StartedAt.Format("2006-01-02"), b.StartedAt.Format("2006-01-02")},
		{"Mode", a.Mode, b.Mode},
		{"Work", workSummary(workA, a.WorkID), workSummary(workB, b.WorkID)},
		{"Creator", workField(workA, func(w *store.Work) string { return w.Creator }), workField(workB, func(w *store.Work) string { return w.Creator })},
		{"Source", workField(workA, func(w *store.Work) string { return w.Source }), workField(workB, func(w *store.Work) string { return w.Source })},
		{"Medium", workField(workA, func(w *store.Work) string { return w.Medium }), workField(workB, func(w *store.Work) string { return w.Medium })},
		{"Region", workField(workA, func(w *store.Work) string { return w.CultureRegion }), workField(workB, func(w *store.Work) string { return w.CultureRegion })},
		{"Mood", moodField(a), moodField(b)},
		{"Duration", durationField(a), durationField(b)},
		{"Tags", a.Tags, b.Tags},
		{"Prompt", a.Prompt, b.Prompt},
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "FIELD\tA\tB")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\n",
			r.label,
			truncate(coalesce(r.a, "—"), 60),
			truncate(coalesce(r.b, "—"), 60),
		)
	}
	_ = tw.Flush()

	// Reflections rendered as separate blocks so multi-line text is
	// readable; the tabwriter row would truncate at 60 chars.
	fmt.Fprintln(out, "")
	if a.Reflection != "" || b.Reflection != "" {
		fmt.Fprintln(out, "Reflections")
		fmt.Fprintf(out, "  A: %s\n", coalesce(a.Reflection, "—"))
		fmt.Fprintf(out, "  B: %s\n", coalesce(b.Reflection, "—"))
		fmt.Fprintln(out, "")
	}
}

func workSummary(w *store.Work, fallbackID string) string {
	if w == nil {
		if fallbackID != "" {
			return fallbackID + " (not in store)"
		}
		return ""
	}
	return coalesce(w.Title, "(untitled)") + " · " + w.ID
}

func workField(w *store.Work, pick func(*store.Work) string) string {
	if w == nil {
		return ""
	}
	return pick(w)
}

func moodField(sit *store.Sit) string {
	if sit.Mood.Valid {
		return fmt.Sprintf("%d/5", sit.Mood.Int64)
	}
	return ""
}

func durationField(sit *store.Sit) string {
	if sit.DurationSeconds <= 0 {
		return ""
	}
	return formatDuration(sit.DurationSeconds)
}

func emitJournalCompareVerifyEnvelope(cmd *cobra.Command, flags *rootFlags) error {
	envelope := map[string]any{
		"command":                 "journal compare",
		"verify_noop":             true,
		"success":                 true,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "journal compare reads the local store; PRINTING_PRESS_VERIFY=1 short-circuits the rendering. Pass --json to get the data envelope.",
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

// avoid unused import warning if the file later drops its context usage.
var _ = context.Background
