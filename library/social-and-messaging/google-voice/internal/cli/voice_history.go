// Local Google Voice evidence queries; no remote service calls.
// pp:data-source local
package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/google-voice/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/google-voice/internal/voice"
	"github.com/spf13/cobra"
	"os"
	"time"
)

func voiceWindow(after, before string) error {
	var a, b time.Time
	var e error
	if after != "" {
		a, e = time.Parse(time.RFC3339Nano, after)
		if e != nil {
			return usageErr(fmt.Errorf("--after must be an RFC3339 timestamp"))
		}
	}
	if before != "" {
		b, e = time.Parse(time.RFC3339Nano, before)
		if e != nil {
			return usageErr(fmt.Errorf("--before must be an RFC3339 timestamp"))
		}
	}
	if !a.IsZero() && !b.IsZero() && !a.Before(b) {
		return usageErr(fmt.Errorf("--after must be earlier than --before"))
	}
	return nil
}
func newVoiceQuery(f *rootFlags) *cobra.Command {
	var q voice.Query
	var dbPath string
	c := voiceCommand(&cobra.Command{Use: "query", Short: "Find locally observed messages using compound literal evidence filters"}, "local", f, func(c *cobra.Command, _ []string) error {
		if e := voiceWindow(q.After, q.Before); e != nil {
			return e
		}
		if q.Limit < 1 || q.Limit > 10000 {
			return usageErr(fmt.Errorf("--limit must be 1 through 10000"))
		}
		if q.Direction != "" && q.Direction != "inbound" && q.Direction != "outbound" && q.Direction != "unknown" {
			return usageErr(fmt.Errorf("--direction must be inbound, outbound or unknown"))
		}
		m, e := openVoiceMirror(c, f, dbPath)
		if e != nil {
			return e
		}
		messages := []voice.Record{}
		if m != nil {
			defer m.Close()
			ctx, cancel := boundCtx(c.Context(), f)
			defer cancel()
			messages, e = m.Messages(ctx, q)
			if e != nil {
				return e
			}
		}
		return f.printJSON(c, map[string]any{"messages": messages, "potentially_clipped": len(messages) == q.Limit, "note": "Only locally observed records match these literal constraints; inspect mirror coverage before interpreting absence."})
	})
	c.Long = "Use this command to find message records matching participant, message-time, direction, attachment and literal-text constraints. Do NOT use this command to expand a known message into neighboring conversation; use 'history context' instead."
	c.Example = "  google-voice-pp-cli history query --text appointment --direction inbound --limit 20 --agent"
	c.Flags().StringVar(&q.Text, "text", "", "Literal case-insensitive substring required in each message body")
	c.Flags().StringVar(&q.Participant, "participant", "", "Exact participant phone identifier; display names never merge identities")
	c.Flags().StringVar(&q.Direction, "direction", "", "Message direction: inbound, outbound or unknown")
	c.Flags().StringVar(&q.After, "after", "", "Inclusive source-message timestamp bound in RFC3339 format")
	c.Flags().StringVar(&q.Before, "before", "", "Exclusive source-message timestamp bound in RFC3339 format")
	c.Flags().BoolVar(&q.HasAttachment, "has-attachment", false, "Require returned attachment metadata on matching messages")
	c.Flags().IntVar(&q.Limit, "limit", 20, "Maximum matching locally observed message records to return")
	c.Flags().StringVar(&dbPath, "db", "", "Private existing Voice SQLite mirror path")
	return c
}
func newVoiceContext(f *rootFlags) *cobra.Command {
	var id, dbPath string
	var before, after int
	c := voiceCommand(&cobra.Command{Use: "context", Short: "Retrieve bounded same-thread evidence around an exact observed message ID"}, "local", f, func(c *cobra.Command, _ []string) error {
		if id == "" || before < 0 || before > 100 || after < 0 || after > 100 {
			return usageErr(fmt.Errorf("--message-id is required; --before and --after must be 0 through 100"))
		}
		m, e := openVoiceMirror(c, f, dbPath)
		if e != nil {
			return e
		}
		if m == nil {
			return f.printJSON(c, map[string]any{"messages": []voice.Record{}, "note": "No local mirror; run mirror sync before selecting a message anchor."})
		}
		defer m.Close()
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		out, e := m.Context(ctx, id, before, after)
		if errors.Is(e, sql.ErrNoRows) {
			return notFoundErr(fmt.Errorf("--message-id was not observed in this profile; find an anchor with history query"))
		}
		if e != nil {
			return e
		}
		return f.printJSON(c, out)
	})
	c.Long = "Use this command to retrieve neighboring messages around an exact message ID. Do NOT use this command to discover which messages match a question; use 'history query' instead."
	c.Example = "  google-voice-pp-cli history context --message-id sms-example --before 4 --after 4 --agent"
	if fixture := os.Getenv("GOOGLE_VOICE_FIXTURE_MESSAGE_ID"); fixture != "" {
		c.Annotations["pp:happy-args"] = "--message-id=" + fixture
	}
	c.Flags().StringVar(&id, "message-id", "", "Exact source message ID returned by search or history query")
	c.Flags().IntVar(&before, "before", 4, "Maximum preceding locally observed messages in this thread")
	c.Flags().IntVar(&after, "after", 4, "Maximum following locally observed messages in this thread")
	c.Flags().StringVar(&dbPath, "db", "", "Private existing Voice SQLite mirror path")
	return c
}
func newVoiceCoverage(f *rootFlags) *cobra.Command {
	var participant, after, before, dbPath string
	var limit int
	c := voiceCommand(&cobra.Command{Use: "coverage", Short: "Inspect collection observations, unknown areas, failures and pending cursors"}, "local", f, func(c *cobra.Command, _ []string) error {
		if e := voiceWindow(after, before); e != nil {
			return e
		}
		if limit < 1 || limit > 10000 {
			return usageErr(fmt.Errorf("--limit must be 1 through 10000"))
		}
		m, e := openVoiceMirror(c, f, dbPath)
		if e != nil {
			return e
		}
		out := map[string]any{"requested_window": map[string]string{"after_inclusive": after, "before_exclusive": before}, "exact_participant": participant, "coverage": nil, "note": "Unobserved inventory and unfinished pagination prevent absence claims. This does not infer continuous time coverage from observed message bounds."}
		if m != nil {
			defer m.Close()
			ctx, cancel := boundCtx(c.Context(), f)
			defer cancel()
			coverage, e := m.Coverage(ctx, participant, limit)
			if e != nil {
				return e
			}
			out["coverage"] = coverage
			out["potentially_clipped"] = len(coverage.Threads) == limit
		}
		return f.printJSON(c, out)
	})
	c.Long = "Use this command to inspect collection freshness, inventory discovery, pagination and failed-work evidence for a proposed lookup. Do NOT use this command to infer recent communication from import health; use 'history activity' instead."
	c.Example = "  google-voice-pp-cli mirror coverage --agent"
	c.Flags().StringVar(&participant, "participant", "", "Exact participant phone identifier to scope thread observations")
	c.Flags().StringVar(&after, "after", "", "Inclusive RFC3339 bound of the question being investigated")
	c.Flags().StringVar(&before, "before", "", "Exclusive RFC3339 bound of the question being investigated")
	c.Flags().IntVar(&limit, "limit", 100, "Maximum matching thread observation records to return")
	c.Flags().StringVar(&dbPath, "db", "", "Private existing Voice SQLite mirror path")
	return c
}
func newVoiceArrivals(f *rootFlags) *cobra.Command {
	var afterRun int64
	var cursor, dbPath string
	var limit int
	c := voiceCommand(&cobra.Command{Use: "arrivals", Short: "Inspect newly mirrored source IDs separately from source communication times"}, "local", f, func(c *cobra.Command, _ []string) error {
		if limit < 1 || limit > 10000 || afterRun < 0 {
			return usageErr(fmt.Errorf("--limit must be 1 through 10000; --after-run must be nonnegative"))
		}
		m, e := openVoiceMirror(c, f, dbPath)
		if e != nil {
			return e
		}
		if m == nil {
			return f.printJSON(c, map[string]any{"messages": []voice.Record{}, "next_cursor": ""})
		}
		defer m.Close()
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		out, e := m.Arrivals(ctx, afterRun, cursor, limit)
		if e != nil {
			return usageErr(e)
		}
		return f.printJSON(c, out)
	})
	c.Long = "Use this command to inspect records newly added to the local mirror after a collection-run watermark, including old history found by backfill. Do NOT use this command to ask what communication happened recently; use 'history query' or 'history activity' instead."
	c.Example = "  google-voice-pp-cli history arrivals --limit 100 --agent"
	c.Flags().Int64Var(&afterRun, "after-run", 0, "Return IDs first observed after this saved terminal collection run")
	c.Flags().StringVar(&cursor, "cursor", "", "Opaque account-scoped arrival continuation cursor from the previous result")
	c.Flags().IntVar(&limit, "limit", 100, "Maximum newly mirrored source message IDs to return")
	c.Flags().StringVar(&dbPath, "db", "", "Private existing Voice SQLite mirror path")
	return c
}
func newVoiceActivity(f *rootFlags) *cobra.Command {
	var since, after, before, dbPath string
	var limit int
	c := voiceCommand(&cobra.Command{Use: "activity", Short: "Report observed directional text and MMS counts for each separate thread"}, "local", f, func(c *cobra.Command, _ []string) error {
		if limit < 1 || limit > 10000 {
			return usageErr(fmt.Errorf("--limit must be 1 through 10000"))
		}
		end := time.Now().UTC()
		if before != "" {
			v, e := time.Parse(time.RFC3339Nano, before)
			if e != nil {
				return usageErr(fmt.Errorf("--before must be an RFC3339 timestamp"))
			}
			end = v
		}
		duration, e := cliutil.ParseDurationLoose(since)
		if e != nil || duration <= 0 {
			return usageErr(fmt.Errorf("--since must be a positive duration such as 30d or 4w"))
		}
		start := end.Add(-duration)
		if after != "" {
			if c.Flags().Changed("since") {
				return usageErr(fmt.Errorf("choose --after or --since, rather than supplying both"))
			}
			v, e := time.Parse(time.RFC3339Nano, after)
			if e != nil {
				return usageErr(fmt.Errorf("--after must be an RFC3339 timestamp"))
			}
			start = v
		}
		if !start.Before(end) {
			return usageErr(fmt.Errorf("activity requires --after earlier than --before"))
		}
		m, e := openVoiceMirror(c, f, dbPath)
		if e != nil {
			return e
		}
		rows := []voice.Activity{}
		if m != nil {
			defer m.Close()
			ctx, cancel := boundCtx(c.Context(), f)
			defer cancel()
			rows, e = m.Activity(ctx, start, end, limit)
			if e != nil {
				return e
			}
		}
		return f.printJSON(c, map[string]any{"threads": rows, "after_inclusive": start.Format(time.RFC3339Nano), "before_exclusive": end.Format(time.RFC3339Nano), "potentially_clipped": len(rows) == limit, "note": "Counts describe observed text/MMS only. Threads and exact participants remain separate; zero counts do not establish complete history or relationship meaning."})
	})
	c.Long = "Use this command for deterministic communication counts and last-observed inbound/outbound message times within a window. Do NOT use this command to retrieve exact message evidence; use 'history query' instead."
	c.Example = "  google-voice-pp-cli history activity --since 30d --limit 50 --agent"
	c.Flags().StringVar(&since, "since", "30d", "Observed activity lookback duration; accepts day and week shorthand")
	c.Flags().StringVar(&after, "after", "", "Inclusive RFC3339 source-message time, instead of --since")
	c.Flags().StringVar(&before, "before", "", "Exclusive RFC3339 source-message time; defaults to now")
	c.Flags().IntVar(&limit, "limit", 50, "Maximum separate conversation activity rows to return")
	c.Flags().StringVar(&dbPath, "db", "", "Private existing Voice SQLite mirror path")
	return c
}
