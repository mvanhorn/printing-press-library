package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface/internal/hfx"
)

// ----------------- watch-poll -----------------

func newHFWatchPollCmd(flags *rootFlags) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "watch-poll",
		Short: "Cron-callable: check watch entries for new matches and emit events.",
		Long: `watch-poll iterates the watch list, queries HF per entry filtered to
lastModified > last_poll (or entry.since on first run), and emits structured
events to each entry's notify sink (stdout / file:<path> / jarvis-via-MC-API).

Designed for cron / launchd:

  */15 * * * * huggingface-pp-cli watch-poll --json >> /tmp/hf-watch-poll.log 2>&1`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			stateDir, err := hfx.EnsureStateDir(flags.stateDir, flags.noWrite)
			if err != nil {
				return hfConfigMissing("resolving state dir: %v", err)
			}
			entries, err := loadWatchList(stateDir)
			if err != nil {
				if os.IsNotExist(err) {
					resp := watchPollResponse{Envelope: hfx.NewEnvelope("watch-poll"), Cursor: filepath.Join(stateDir, watchCursorFn), WatchListPath: filepath.Join(stateDir, watchFile)}
					if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
						return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
					}
					fmt.Fprintln(cmd.OutOrStdout(), "(no watch entries; add one with 'watch add')")
					return nil
				}
				return hfConfigMissing("loading watch list: %v", err)
			}

			cursor := loadWatchCursor(stateDir)
			events := []watchEvent{}
			for _, e := range entries {
				since := e.Since
				if last, ok := cursor.LastPoll[e.ID]; ok && last != "" {
					since = last
				}
				matches, err := pollWatchEntry(ctx, e, since)
				if err != nil {
					// Log + continue; one bad entry shouldn't tank the whole poll.
					fmt.Fprintf(os.Stderr, "watch-poll: entry %s (%s/%s): %v\n", e.ID[:8], e.Kind, e.Target, err)
					continue
				}
				for _, m := range matches {
					ev := watchEvent{
						EntryID:    e.ID,
						Kind:       e.Kind,
						Target:     e.Target,
						Match:      m,
						NotifiedTo: e.Notify,
						EmittedAt:  time.Now().UTC().Format(time.RFC3339),
					}
					if !dryRun {
						_ = emitWatchEvent(e.Notify, ev, flags.noWrite)
					}
					events = append(events, ev)
				}
				cursor.LastPoll[e.ID] = time.Now().UTC().Format(time.RFC3339)
			}

			if !dryRun && !flags.noWrite {
				_ = saveWatchCursor(stateDir, cursor, flags.noWrite)
			}

			// Sort events by emitted_at desc
			sort.Slice(events, func(i, j int) bool { return events[i].EmittedAt > events[j].EmittedAt })

			resp := watchPollResponse{
				Envelope:      hfx.NewEnvelope("watch-poll"),
				Polled:        len(entries),
				NewMatches:    len(events),
				Events:        events,
				Cursor:        filepath.Join(stateDir, watchCursorFn),
				WatchListPath: filepath.Join(stateDir, watchFile),
			}
			if flags.explain {
				resp.Explain = fmt.Sprintf("explain: polled %d entries, %d new matches. Cursor updated at %s. Notify sinks: stdout (logged), file:<path> (appended), jarvis (data/alerts/<id>.json — MC API alert pipeline 5-field schema).",
					len(entries), len(events), cursor.UpdatedAt.Format(time.RFC3339))
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "watch-poll: %d entries, %d new matches\n", len(entries), len(events))
			for _, ev := range events {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s/%s → %s (notified=%s)\n", ev.EmittedAt, ev.Kind, ev.Target, ev.Match.ID, ev.NotifiedTo)
			}
			if resp.Explain != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", resp.Explain)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run-emit", false, "Compute matches but do not write to notify sinks (still updates cursor unless --no-write)")
	return cmd
}

// pollWatchEntry returns models matching this entry that were modified after `since`.
func pollWatchEntry(ctx context.Context, e watchEntry, since string) ([]hfModelLite, error) {
	q := url.Values{}
	switch e.Kind {
	case "uploader":
		q.Set("author", e.Target)
	case "base-model":
		q.Set("search", e.Target)
	case "feature":
		// Feature-watch is the most expensive: search a relevant slice and
		// post-filter via classifier. For watch-poll we keep it cheap (no
		// per-model config.json fan-out) and filter on tags/keywords.
		q.Set("search", e.Target)
	default:
		return nil, fmt.Errorf("unknown kind %q", e.Kind)
	}
	q.Set("sort", "lastModified")
	q.Set("direction", "-1")
	q.Set("limit", strconv.Itoa(50))

	ms, _, err := hfListModels(ctx, q, hfTokenForRequests())
	if err != nil {
		return nil, err
	}

	cutoff, _ := time.Parse(time.RFC3339, since)
	out := []hfModelLite{}
	for _, m := range ms {
		// Compare lastModified > cutoff
		t, terr := time.Parse(time.RFC3339Nano, m.LastModified)
		if terr != nil {
			t, terr = time.Parse(time.RFC3339, m.LastModified)
			if terr != nil {
				continue
			}
		}
		if !cutoff.IsZero() && !t.After(cutoff) {
			continue
		}
		// base-model + feature: extra client-side filter
		if e.Kind == "base-model" {
			matched := false
			for _, tag := range m.Tags {
				tl := strings.ToLower(tag)
				if strings.HasPrefix(tl, "base_model:") && strings.Contains(tl, strings.ToLower(e.Target)) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if e.Kind == "feature" {
			matched := false
			for _, tag := range m.Tags {
				if strings.EqualFold(tag, e.Target) || strings.Contains(strings.ToLower(tag), strings.ToLower(e.Target)) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		out = append(out, hfModelLite{
			ID:           m.ID,
			Author:       m.Author,
			LastModified: m.LastModified,
			Downloads:    m.Downloads,
		})
	}
	return out, nil
}

// emitWatchEvent dispatches the event to the configured sink.
//
//	stdout      — print as a JSON line to stdout (caller's stdout)
//	file:<path> — append a JSON line to <path>
//	jarvis      — write data/alerts/<entry-id>.json with MC API alert pipeline shape (5-field schema)
//
// Errors here are non-fatal at the poll level; one sink failure shouldn't stop the others.
func emitWatchEvent(notify string, ev watchEvent, noWrite bool) error {
	if noWrite {
		return nil
	}
	switch {
	case notify == "stdout":
		buf, _ := json.Marshal(ev)
		fmt.Fprintln(os.Stdout, string(buf))
		return nil
	case strings.HasPrefix(notify, "file:"):
		path := strings.TrimPrefix(notify, "file:")
		path = expandHome(path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		buf, _ := json.Marshal(ev)
		_, err = fmt.Fprintln(f, string(buf))
		return err
	case notify == "jarvis":
		return emitJarvisAlert(ev)
	default:
		return fmt.Errorf("unknown notify sink %q", notify)
	}
}

// emitJarvisAlert writes an alert in the MC API 5-field schema to data/alerts/<entry-id>.json.
//
// Schema (per workspace/scripts/lib/alert.mjs and the perceiver pickup loop):
//
//	{ id, severity, type, what, chance?, impact?, ifNothing?, play?, trace? }
//
// Severity = P2 (informational; watch matches are not user-blocking).
func emitJarvisAlert(ev watchEvent) error {
	// Resolve alerts dir: prefer CWD/data/alerts, then HF_OPENCLAW_ROOT/data/alerts
	// if set. No hardcoded user-specific defaults.
	cands := []string{"data/alerts"}
	if root := hfOpenclawRoot(); root != "" {
		cands = append(cands, filepath.Join(root, "data", "alerts"))
	}
	openclawRoot := firstExistingPath(cands)
	if openclawRoot == "" {
		// Best-effort: create under HF_OPENCLAW_ROOT or CWD
		if r := hfOpenclawRoot(); r != "" {
			openclawRoot = filepath.Join(r, "data", "alerts")
		} else {
			openclawRoot = filepath.Join("data", "alerts")
		}
		if err := os.MkdirAll(openclawRoot, 0o755); err != nil {
			return fmt.Errorf("creating alerts dir: %w", err)
		}
	}

	alertID := "hf-watch-" + ev.EntryID[:8] + "-" + ev.Match.ID
	alertID = strings.ReplaceAll(alertID, "/", "-")
	alertPath := filepath.Join(openclawRoot, alertID+".json")

	alert := map[string]any{
		"id":         alertID,
		"severity":   "P2",
		"type":       "hf-watch-match",
		"what":       fmt.Sprintf("HF %s watch on %q matched: %s (modified %s)", ev.Kind, ev.Target, ev.Match.ID, ev.Match.LastModified),
		"trace":      ev,
		"emitted_at": ev.EmittedAt,
	}
	buf, err := json.MarshalIndent(alert, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(alertPath, buf, 0o644)
}
