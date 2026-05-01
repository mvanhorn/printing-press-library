// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// tail: poll LinkedIn + Happenstance (and optionally Deepline) every interval,
// diff against the previous snapshot in the local store, and emit NEW items
// only as NDJSON. With --watch it runs until SIGTERM/SIGINT; without --watch
// it does a single poll and exits.
//
// The original generated `tail <resource>` shape is preserved: passing a
// resource falls back to the legacy single-endpoint poll so scripts that used
// it keep working.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/linkedin"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/store"

	"github.com/spf13/cobra"
)

func newTailCmd(flags *rootFlags) *cobra.Command {
	var resource string
	var interval time.Duration
	var follow bool
	var watch bool
	var sourcesCSV string
	var includeDeepline bool

	cmd := &cobra.Command{
		Use:   "tail [resource]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Stream NEW items across LinkedIn + Happenstance (and optionally Deepline)",
		Long: `Tail polls each configured source at --interval and emits only items that
weren't in the previous snapshot, as NDJSON on stdout. Gracefully shuts down
on SIGTERM / SIGINT.

Sources:
  - li   LinkedIn inbox (new messages)
  - hp   Happenstance feed + recent research

Deepline is off by default because every call costs credits. Pass
--include-deepline to also log deepline_log inserts.

When called with a single positional resource (legacy mode) the command
falls back to polling just that one Happenstance endpoint.`,
		Example: `  contact-goat-pp-cli tail --watch --interval 5m
  contact-goat-pp-cli tail --sources hp --watch
  contact-goat-pp-cli tail friends --interval 30s   # legacy single-resource mode
  contact-goat-pp-cli tail | jq 'select(.source=="hp")'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				resource = args[0]
			}

			// Legacy single-resource mode kicks in when --sources wasn't set
			// and a positional resource was provided.
			if resource != "" && !cmd.Flags().Changed("sources") {
				return runLegacyTail(cmd, flags, resource, interval, follow)
			}

			sources := parseSourcesCSV(sourcesCSV)
			if len(sources) == 0 {
				return usageErr(fmt.Errorf("--sources must include at least one of: li, hp"))
			}

			enc := json.NewEncoder(cmd.OutOrStdout())

			// A fresh poll + exit when --watch is false.
			if !watch {
				_, _ = pollAndEmitOnce(cmd, flags, enc, sources, includeDeepline)
				return nil
			}

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

			fmt.Fprintf(os.Stderr, "tail watching %v every %s (Ctrl+C to stop)\n",
				sourceLabels(sources), interval)

			// Emit once right away so the first poll is snappy.
			_, _ = pollAndEmitOnce(cmd, flags, enc, sources, includeDeepline)

			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-sig:
					fmt.Fprintln(os.Stderr, "\nshutting down gracefully...")
					return nil
				case <-ticker.C:
					if _, err := pollAndEmitOnce(cmd, flags, enc, sources, includeDeepline); err != nil {
						fmt.Fprintf(os.Stderr, "warning: poll failed: %v\n", err)
					}
				}
			}
		},
	}

	cmd.Flags().StringVar(&resource, "resource", "", "(legacy) Resource type to tail as a single Happenstance endpoint")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Minute, "Poll interval")
	cmd.Flags().BoolVar(&follow, "follow", true, "Keep running (legacy flag, kept for compat)")
	cmd.Flags().BoolVar(&watch, "watch", false, "Keep polling until SIGINT/SIGTERM; without this, poll once and exit")
	cmd.Flags().StringVar(&sourcesCSV, "sources", "li,hp", "Comma-separated sources: li, hp")
	cmd.Flags().BoolVar(&includeDeepline, "include-deepline", false, "Also log deepline_log inserts (burns credits if any sync runs; default off)")

	return cmd
}

// sourceLabels returns a stable, printable set of source names for stderr.
func sourceLabels(sources map[string]bool) []string {
	out := make([]string, 0, len(sources))
	for k := range sources {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// pollAndEmitOnce runs a single diff-and-emit across all configured sources.
// Returns the number of new items emitted and any error from the last failing
// source (errors on individual sources are logged but don't abort the poll).
func pollAndEmitOnce(cmd *cobra.Command, flags *rootFlags, enc *json.Encoder, sources map[string]bool, includeDeepline bool) (int, error) {
	s, err := openP2Store()
	if err != nil {
		return 0, fmt.Errorf("opening local store: %w", err)
	}
	defer s.Close()

	total := 0
	var lastErr error

	if sources["hp"] {
		n, err := pollHappenstance(cmd, flags, enc, s)
		if err != nil {
			lastErr = err
			fmt.Fprintf(os.Stderr, "warning: hp poll failed: %v\n", err)
		}
		total += n
	}
	if sources["li"] {
		n, err := pollLinkedIn(cmd, flags, enc, s)
		if err != nil {
			lastErr = err
			fmt.Fprintf(os.Stderr, "warning: li poll failed: %v\n", err)
		}
		total += n
	}
	if includeDeepline {
		n, err := pollDeepline(cmd, enc, s)
		if err != nil {
			lastErr = err
		}
		total += n
	}
	return total, lastErr
}

// pollHappenstance diffs the current unseen feed + research list against the
// prior snapshot. Emits one NDJSON line per new item.
func pollHappenstance(cmd *cobra.Command, flags *rootFlags, enc *json.Encoder, s *store.Store) (int, error) {
	c, err := flags.newClientRequireCookies("happenstance")
	if err != nil {
		return 0, err
	}

	total := 0
	for _, endpoint := range []struct {
		name     string
		path     string
		params   map[string]string
		resource string
	}{
		{"hp_feed", "/api/feed", map[string]string{"unseen": "true", "limit": "50"}, "feed"},
		{"hp_research", "/api/research/recent", nil, "research"},
	} {
		raw, err := c.Get(endpoint.path, endpoint.params)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s fetch failed: %v\n", endpoint.name, err)
			continue
		}
		raw = extractResponseData(raw)
		items := extractArray(raw)
		if len(items) == 0 {
			continue
		}

		prev, _, _ := s.TailSnapshotGet(endpoint.name)
		ids := make([]string, 0, len(items))
		for _, item := range items {
			id := extractItemID(item)
			if id == "" {
				continue
			}
			ids = append(ids, id)
			if prev[id] {
				continue
			}
			// New item — emit.
			out := map[string]any{
				"event":     "new",
				"source":    "hp",
				"resource":  endpoint.resource,
				"id":        id,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"data":      json.RawMessage(item),
			}
			_ = enc.Encode(out)
			total++
		}
		if err := s.TailSnapshotSet(endpoint.name, ids); err != nil {
			fmt.Fprintf(os.Stderr, "warning: snapshot write failed for %s: %v\n", endpoint.name, err)
		}
	}
	return total, nil
}

// pollLinkedIn diffs the LinkedIn inbox. We don't spawn the scraper if the
// user isn't logged in — we emit a single stderr warning and skip.
func pollLinkedIn(cmd *cobra.Command, flags *rootFlags, enc *json.Encoder, s *store.Store) (int, error) {
	// Avoid burning subprocess startup when users explicitly didn't log in.
	if ok, _ := linkedinLoggedIn(); !ok {
		fmt.Fprintln(os.Stderr, "warning: skipping li poll — linkedin-mcp not logged in (run `uvx linkedin-scraper-mcp@latest --login`)")
		return 0, nil
	}

	items, err := fetchLinkedInInboxSnapshot(cmd.Context(), flags)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}

	prev, _, _ := s.TailSnapshotGet("li_inbox")
	ids := make([]string, 0, len(items))
	total := 0
	for _, item := range items {
		var m map[string]any
		if err := json.Unmarshal(item, &m); err != nil {
			continue
		}
		id := str(firstNonNil(m, "thread_id", "id", "conversation_id", "urn"))
		if id == "" {
			continue
		}
		ids = append(ids, id)
		if prev[id] {
			continue
		}
		out := map[string]any{
			"event":     "new",
			"source":    "li",
			"resource":  "inbox",
			"id":        id,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"data":      json.RawMessage(item),
		}
		_ = enc.Encode(out)
		total++
	}
	if err := s.TailSnapshotSet("li_inbox", ids); err != nil {
		fmt.Fprintf(os.Stderr, "warning: li_inbox snapshot write failed: %v\n", err)
	}
	return total, nil
}

// pollDeepline tails the local deepline_log table — emits any new rows since
// last snapshot. This intentionally uses local state only: it does not call
// Deepline itself (which would cost credits).
func pollDeepline(cmd *cobra.Command, enc *json.Encoder, s *store.Store) (int, error) {
	rows, err := s.Query(`SELECT id, tool_id, cost_credits, status, timestamp FROM deepline_log ORDER BY id DESC LIMIT 100`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type dlRow struct {
		ID        int64  `json:"id"`
		ToolID    string `json:"tool_id"`
		Cost      int    `json:"cost_credits"`
		Status    string `json:"status"`
		Timestamp string `json:"timestamp"`
	}
	var items []dlRow
	for rows.Next() {
		var r dlRow
		if err := rows.Scan(&r.ID, &r.ToolID, &r.Cost, &r.Status, &r.Timestamp); err != nil {
			continue
		}
		items = append(items, r)
	}
	if len(items) == 0 {
		return 0, nil
	}
	prev, _, _ := s.TailSnapshotGet("deepline_log")
	ids := make([]string, 0, len(items))
	total := 0
	for _, it := range items {
		id := fmt.Sprintf("%d", it.ID)
		ids = append(ids, id)
		if prev[id] {
			continue
		}
		out := map[string]any{
			"event":     "new",
			"source":    "dl",
			"resource":  "deepline_log",
			"id":        id,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"data":      it,
		}
		_ = enc.Encode(out)
		total++
	}
	if err := s.TailSnapshotSet("deepline_log", ids); err != nil {
		fmt.Fprintf(os.Stderr, "warning: deepline_log snapshot write failed: %v\n", err)
	}
	return total, nil
}

// extractItemID uses a handful of common field names — it must be stable so
// the snapshot dedupe works across polls.
func extractItemID(raw json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	for _, k := range []string{"id", "uuid", "urn", "slug", "_id", "event_id"} {
		if v, ok := m[k]; ok {
			s := str(v)
			if s != "" {
				return s
			}
		}
	}
	return ""
}

// runLegacyTail is the behavior of the original generated `tail <resource>`
// command. Kept so pre-existing scripts continue to work verbatim.
func runLegacyTail(cmd *cobra.Command, flags *rootFlags, resource string, interval time.Duration, follow bool) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	c.NoCache = true

	path := "/" + strings.TrimLeft(resource, "/")
	enc := json.NewEncoder(cmd.OutOrStdout())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	fmt.Fprintf(os.Stderr, "tailing %s every %s (Ctrl+C to stop)\n", resource, interval)

	if err := legacyFetchAndEmit(c, path, enc); err != nil {
		fmt.Fprintf(os.Stderr, "warning: initial fetch failed: %v\n", err)
	}
	if !follow {
		return nil
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-sig:
			fmt.Fprintln(os.Stderr, "\nshutting down gracefully...")
			return nil
		case <-ticker.C:
			if err := legacyFetchAndEmit(c, path, enc); err != nil {
				fmt.Fprintf(os.Stderr, "warning: poll failed: %v\n", err)
			}
		}
	}
}

func legacyFetchAndEmit(c *client.Client, path string, enc *json.Encoder) error {
	data, err := c.Get(path, nil)
	if err != nil {
		return err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return enc.Encode(map[string]any{
			"event":     "data",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"data":      json.RawMessage(data),
		})
	}
	for _, item := range items {
		if err := enc.Encode(map[string]any{
			"event":     "data",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"data":      item,
		}); err != nil {
			return err
		}
	}
	return nil
}

// linkedinLoggedIn is a thin wrapper around the linkedin package's IsLoggedIn
// to keep package imports tidy at the call site.
func linkedinLoggedIn() (bool, error) {
	return linkedin.IsLoggedIn()
}
