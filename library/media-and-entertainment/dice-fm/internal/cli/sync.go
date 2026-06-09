// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-modified from the generated sync command: the per-resource fetch is
// rewired to the DICE viewer GraphQL connections (see dice_query.go) because
// the generator's root-level `nodes` query shape does not match DICE's
// `viewer { conn { edges { node } } }` API. The command framework (worker
// pool, --json events, access-denied warnings, summary) is preserved.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
	"github.com/spf13/cobra"
)

// syncResult holds the outcome of syncing a single resource.
type syncResult struct {
	Resource string
	Count    int
	Err      error
	Warn     error
	Duration time.Duration
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var resources []string
	var full bool
	var since string
	var concurrency int
	var dbPath string
	var maxPages int
	var latestOnly bool
	var orderTickets bool
	var eventsFrom string
	var eventsTo string
	var eventsDateField string
	var eventIDs string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync your DICE data to local SQLite for offline search and analysis",
		Long: `Sync your events, tickets, orders, returns, transfers, extras, and genres
from the DICE Partners API into a local SQLite database. Fans are derived from
order and ticket holders so the fans commands work offline. Supports resumable
incremental sync (--since) and full resync (--full). Once synced, use 'search'
and 'sql' for instant offline queries, and the analytics commands (door,
revenue, fans, velocity, returns) for cross-event reporting.

Scoping:
  --since gives an incremental floor (events by updatedAt, orders by
  purchasedAt, returns/transfers by their date field). To scope events by a
  show-date window use --events-from / --events-to (RFC3339 or YYYY-MM-DD;
  --events-from is inclusive, --events-to exclusive) and pick the field with
  --events-date-field (startDatetime|onSaleDatetime|updatedAt). Tickets and
  orders have no usable date field for backfill, so scope them by event with
  --event-ids: pass 'auto' to use the events already synced into the local
  store, or a comma-separated list of event IDs.

Exit codes & warnings:
  Resources the API denies access to (GraphQL errors carrying FORBIDDEN /
  UNAUTHENTICATED / PERMISSION_DENIED extensions, or HTTP 401/403) are
  reported as warnings rather than failing the run. In --json mode each is
  emitted as a {"event":"sync_warning",...} line. The command exits non-zero
  only when every selected resource was access-denied or any resource hit a
  hard error.`,
		Example: `  # Sync all resources
  dice-fm-pp-cli sync

  # Sync specific resources only
  dice-fm-pp-cli sync --resources events,orders,tickets

  # Full resync (ignore previous checkpoint)
  dice-fm-pp-cli sync --full

  # Incremental: only data updated in the last 7 days
  dice-fm-pp-cli sync --since 7d

  # Latest-only: refresh head of each resource, no historical backfill
  dice-fm-pp-cli sync --latest-only

  # Scope events to a show-date window, then backfill their tickets & orders
  dice-fm-pp-cli sync --resources events --events-from 2026-01-01 --events-to 2027-01-01
  dice-fm-pp-cli sync --resources tickets,orders --event-ids auto --order-tickets`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			if dbPath == "" {
				dbPath = defaultDBPath("dice-fm-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			if len(resources) == 0 {
				resources = defaultSyncResources()
			}

			// --dry-run: report the sync plan without hitting the network or
			// writing the store. Keeps `sync --dry-run` verify-green.
			if dryRunOK(flags) {
				if humanFriendlyEnabled() {
					fmt.Fprintf(os.Stderr, "dry-run: would sync resources: %s\n", strings.Join(resources, ", "))
				} else {
					// json.Marshal escapes quotes/backslashes/newlines in resource
					// names and emits the array form, so a crafted --resources value
					// can't produce malformed JSON (mirrors sync_warning/sync_error).
					resJSON, _ := json.Marshal(resources)
					fmt.Fprintf(os.Stdout, `{"event":"sync_dry_run","resources":%s}`+"\n", resJSON)
				}
				return nil
			}

			if full {
				for _, resource := range resources {
					_ = db.SaveSyncState(resource, "", 0)
				}
			}

			// --latest-only fetches the single newest page via backward pagination
			// (see latestFetch below), so cap at one page. The forward-resume
			// cursor is intentionally left untouched: the backward fetch does not
			// use it, and clearing it would force the next default sync to re-walk
			// the whole connection from the oldest record. --since takes
			// precedence when both are set.
			if latestOnly {
				if since == "" {
					maxPages = 1
				} else if humanFriendlyEnabled() {
					fmt.Fprintln(os.Stderr, "warning: --latest-only ignored because --since is set; --since takes precedence")
				}
			}

			sinceTS := ""
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return fmt.Errorf("invalid --since value %q: %w", since, err)
				}
				sinceTS = ts.Format(time.RFC3339)
			}
			// --latest-only fetches the newest page via backward pagination; it is
			// suppressed when --since is set (--since wins, handled above).
			latestFetch := latestOnly && since == ""

			// Resolve scoping flags into a syncFilters once, then precompute the
			// per-resource where-input. The command owns where-building so
			// --event-ids auto (which reads the store) is resolved a single time
			// before the worker pool fans out.
			if err := validateEventsDateField(eventsDateField); err != nil {
				return err
			}
			eventsFromTS, err := parseFlexibleDate(eventsFrom)
			if err != nil {
				return fmt.Errorf("invalid --events-from value: %w", err)
			}
			eventsToTS, err := parseFlexibleDate(eventsTo)
			if err != nil {
				return fmt.Errorf("invalid --events-to value: %w", err)
			}
			filters := syncFilters{
				SinceTS:         sinceTS,
				EventsFrom:      eventsFromTS,
				EventsTo:        eventsToTS,
				EventsDateField: eventsDateField,
			}
			if trimmed := strings.TrimSpace(eventIDs); trimmed != "" {
				if trimmed == "auto" {
					ids, err := db.DistinctEventIDs()
					if err != nil {
						return fmt.Errorf("resolving --event-ids auto from local store: %w", err)
					}
					filters.EventIDs = ids
					if len(ids) == 0 && (containsResource(resources, "tickets") || containsResource(resources, "orders")) {
						return fmt.Errorf("--event-ids auto found no events in the local store; sync events first (e.g. 'dice-fm-pp-cli sync --resources events')")
					}
				} else {
					for _, id := range strings.Split(trimmed, ",") {
						if id = strings.TrimSpace(id); id != "" {
							filters.EventIDs = append(filters.EventIDs, id)
						}
					}
				}
			}

			wheres := make(map[string]map[string]any, len(resources))
			for _, resource := range resources {
				wheres[resource] = buildWhere(resource, filters)
			}

			if concurrency < 1 {
				concurrency = 4
			}
			// Under PRINTING_PRESS_VERIFY=1, serialize to avoid SQLITE_BUSY on
			// the writer (no network latency to space out the goroutines).
			if cliutil.IsVerifyEnv() {
				concurrency = 1
			}

			started := time.Now()
			work := make(chan string, len(resources))
			results := make(chan syncResult, len(resources))

			var wg sync.WaitGroup
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for resource := range work {
						results <- syncResource(cmd.Context(), c, db, resource, wheres[resource], full, maxPages, latestFetch, orderTickets)
					}
				}()
			}

			for _, resource := range resources {
				work <- resource
			}
			close(work)

			go func() {
				wg.Wait()
				close(results)
			}()

			var totalSynced, errCount, warnCount, successCount int
			var firstErr, firstPlaceholderErr error
			for res := range results {
				switch {
				case res.Err != nil:
					if humanFriendlyEnabled() {
						fmt.Fprintf(os.Stderr, "  %s: error: %v\n", res.Resource, res.Err)
					}
					errCount++
					if firstErr == nil {
						firstErr = res.Err
					}
					if firstPlaceholderErr == nil && errors.Is(res.Err, client.ErrPlaceholderCredential) {
						firstPlaceholderErr = res.Err
					}
				case res.Warn != nil:
					if humanFriendlyEnabled() {
						fmt.Fprintf(os.Stderr, "  %s: warning: %v\n", res.Resource, res.Warn)
					}
					warnCount++
				default:
					if humanFriendlyEnabled() {
						fmt.Fprintf(os.Stderr, "  %s: %d synced (done)\n", res.Resource, res.Count)
					}
					totalSynced += res.Count
					successCount++
				}
			}

			elapsed := time.Since(started)
			totalResources := successCount + warnCount + errCount
			if humanFriendlyEnabled() {
				if warnCount > 0 {
					fmt.Fprintf(os.Stderr, "Sync complete: %d records across %d resources (%d warned, %.1fs)\n",
						totalSynced, totalResources, warnCount, elapsed.Seconds())
				} else {
					fmt.Fprintf(os.Stderr, "Sync complete: %d records across %d resources (%.1fs)\n",
						totalSynced, totalResources, elapsed.Seconds())
				}
			} else {
				// The summary is the command's result, so it goes to stdout where
				// `sync --json | jq` and agents can read it; per-resource progress
				// events stream to stderr. Keeping progress off stdout also lets
				// `workflow archive --json` emit its own single summary object
				// cleanly when it composes syncResource.
				fmt.Fprintf(os.Stdout, `{"event":"sync_summary","total_records":%d,"resources":%d,"success":%d,"warned":%d,"errored":%d,"duration_ms":%d}`+"\n",
					totalSynced, totalResources, successCount, warnCount, errCount, elapsed.Milliseconds())
			}

			if errCount > 0 {
				if firstPlaceholderErr != nil {
					return classifyAPIError(firstPlaceholderErr, flags)
				}
				return fmt.Errorf("%d resource(s) failed to sync", errCount)
			}
			if warnCount > 0 && successCount == 0 {
				return fmt.Errorf("%d resource(s) skipped due to insufficient access", warnCount)
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&resources, "resources", nil, "Comma-separated resource types to sync (events,tickets,orders,returns,transfers,extras,genres)")
	cmd.Flags().BoolVar(&full, "full", false, "Full resync (ignore previous checkpoint)")
	cmd.Flags().StringVar(&since, "since", "", "Incremental sync duration (e.g. 7d, 24h, 1w, 30m) — applies to events, orders, returns, transfers")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "Number of parallel sync workers")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/dice-fm-pp-cli/data.db)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 0, "Maximum pages to fetch per resource (0 = unlimited; default fetches all)")
	cmd.Flags().BoolVar(&latestOnly, "latest-only", false, "Refresh head of each resource only; clears resume cursor and caps pages at 1. Mutually exclusive with --since (--since wins).")
	cmd.Flags().BoolVar(&orderTickets, "order-tickets", false, "Also fetch each order's tickets (enables date/event-scoped `revenue --by-axis`; slower — heavier payload, opt-in).")
	cmd.Flags().StringVar(&eventsFrom, "events-from", "", "Inclusive lower bound of the event date window (RFC3339 or YYYY-MM-DD); scopes the events resource.")
	cmd.Flags().StringVar(&eventsTo, "events-to", "", "Exclusive upper bound of the event date window (RFC3339 or YYYY-MM-DD); scopes the events resource. NOTE: exclusive here, unlike the inclusive --to on analytics commands (revenue/capacity/etc.).")
	cmd.Flags().StringVar(&eventsDateField, "events-date-field", defaultEventsDateField, "Event date field the window scopes: startDatetime|onSaleDatetime|updatedAt.")
	cmd.Flags().StringVar(&eventIDs, "event-ids", "", "Scope tickets & orders by eventId. Either 'auto' (use the events already in the local store) or a comma-separated list of event IDs.")

	return cmd
}

// syncResource fetches one DICE viewer connection (paginated, resumable) and
// upserts its nodes into the local store page by page. For orders and tickets
// it also derives the fans table from embedded holder/fan objects, since DICE
// exposes no top-level fan connection. Each page is upserted immediately and
// the resume cursor is advanced per page so an interrupted sync can resume from
// the last completed page rather than restarting from scratch. enrichOrders
// selects the heavier per-ticket nested selection for orders; see --order-tickets.
// where is the prebuilt GraphQL where-input value for this resource (nil for
// none); the command owns where-building via buildWhere so it can resolve
// cross-resource scoping (e.g. --event-ids auto) once before dispatch.
func syncResource(ctx context.Context, c *client.Client, db *store.Store, resource string, where map[string]any, full bool, maxPages int, latest bool, enrichOrders bool) syncResult {
	started := time.Now()
	if !humanFriendlyEnabled() {
		// json.Marshal escapes the resource name so a value containing a quote,
		// backslash, or newline can't produce a malformed sync_start event.
		resJSON, _ := json.Marshal(resource)
		fmt.Fprintf(os.Stderr, `{"event":"sync_start","resource":%s}`+"\n", resJSON)
	}
	if _, ok := diceConnections[resource]; !ok {
		return syncResult{Resource: resource, Err: fmt.Errorf("unknown resource %q", resource), Duration: time.Since(started)}
	}

	// Under live-dogfood, curtail to a single page so the matrix's flat 30s
	// per-command timeout is not tripped by a full historical backfill. Emit a
	// one-line stderr notice so the truncation is never silent (an observer must
	// not mistake a 1-page dogfood sync for a complete backfill).
	if cliutil.IsDogfoodEnv() && (maxPages == 0 || maxPages > 1) {
		maxPages = 1
		if !humanFriendlyEnabled() {
			resJSON, _ := json.Marshal(resource)
			fmt.Fprintf(os.Stderr, `{"event":"sync_dogfood_cap","resource":%s,"max_pages":1,"reason":"dogfood env caps sync to 1 page"}`+"\n", resJSON)
		} else {
			fmt.Fprintf(os.Stderr, "note: dogfood mode caps %s sync to 1 page (not a full backfill)\n", resource)
		}
	}

	startCursor := ""
	if !full {
		startCursor, _, _, _ = db.GetSyncState(resource)
	}

	max := 0
	if maxPages > 0 {
		max = maxPages * dicePerPage
	}

	// Accumulators updated per page inside the streaming callback.
	var stored, fanCount int
	lastHeartbeat := time.Now()

	_, err := fetchConnectionStream(ctx, c, resource, where, dicePerPage, max, startCursor, latest, enrichOrders,
		func(pageNodes []json.RawMessage, endCursor string, totalFetched int) error {
			pageStored, pageFans, persistErr := persistSyncPage(db, resource, pageNodes, endCursor, latest, stored)
			if persistErr != nil {
				return persistErr
			}
			stored += pageStored
			fanCount += pageFans

			// Emit a time-throttled heartbeat (~every 5s) so observers can
			// distinguish a long fetch from a stuck one.
			if time.Since(lastHeartbeat) >= 5*time.Second {
				lastHeartbeat = time.Now()
				if !humanFriendlyEnabled() {
					resJSON, _ := json.Marshal(resource)
					fmt.Fprintf(os.Stderr, `{"event":"sync_progress","resource":%s,"fetched":%d}`+"\n", resJSON, totalFetched)
				} else {
					fmt.Fprintf(os.Stderr, "  %s: %d fetched…\n", resource, totalFetched)
				}
			}
			return nil
		},
	)
	if err != nil {
		if w, ok := isSyncAccessWarning(err); ok {
			if !humanFriendlyEnabled() {
				// json.Marshal escapes backslashes, newlines, and control bytes
				// that raw API bodies carry; a bare quote-only replace would
				// emit invalid JSON and break `sync --json 2>&1 | jq`.
				msgJSON, _ := json.Marshal(w.Message)
				fmt.Fprintf(os.Stderr, `{"event":"sync_warning","resource":"%s","status":%d,"reason":"%s","message":%s}`+"\n",
					resource, w.Status, w.Reason, msgJSON)
			}
			return syncResult{Resource: resource, Warn: fmt.Errorf("skipped %s: %s", resource, w.Reason), Duration: time.Since(started)}
		}
		if !humanFriendlyEnabled() {
			errJSON, _ := json.Marshal(err.Error())
			fmt.Fprintf(os.Stderr, `{"event":"sync_error","resource":"%s","error":%s}`+"\n", resource, errJSON)
		}
		return syncResult{Resource: resource, Err: fmt.Errorf("fetching %s: %w", resource, err), Duration: time.Since(started)}
	}

	if !humanFriendlyEnabled() {
		fmt.Fprintf(os.Stderr, `{"event":"sync_complete","resource":"%s","total":%d,"fans":%d,"duration_ms":%d}`+"\n",
			resource, stored, fanCount, time.Since(started).Milliseconds())
	} else {
		fmt.Fprintf(os.Stderr, "  %s: %d fetched\n", resource, stored)
	}

	return syncResult{Resource: resource, Count: stored, Duration: time.Since(started)}
}

// syncFilters carries the resolved scoping inputs from the sync flags. The
// command parses the flags once and hands a syncFilters to buildWhere per
// resource, so where-building stays a pure function the tests can exercise
// without a Cobra command.
type syncFilters struct {
	// SinceTS is the RFC3339 incremental floor from --since (back-compat).
	SinceTS string
	// EventsFrom / EventsTo are RFC3339 bounds for the event date window
	// (--events-from inclusive lower, --events-to exclusive upper).
	EventsFrom string
	EventsTo   string
	// EventsDateField is which event date field the window scopes
	// (startDatetime | onSaleDatetime | updatedAt).
	EventsDateField string
	// EventIDs scopes tickets and orders to {eventId: {in: [...]}}.
	EventIDs []string
}

// eventsDateFields enumerates the event date fields --events-date-field accepts.
// Each is an OperatorsDateInput on EventWhereInput in the DICE schema.
var eventsDateFields = []string{"startDatetime", "onSaleDatetime", "updatedAt"}

const defaultEventsDateField = "startDatetime"

// validateEventsDateField rejects any --events-date-field value outside the
// schema's date inputs on EventWhereInput.
func validateEventsDateField(field string) error {
	for _, f := range eventsDateFields {
		if field == f {
			return nil
		}
	}
	return fmt.Errorf("--events-date-field must be one of %s (got %q)", strings.Join(eventsDateFields, "|"), field)
}

// parseFlexibleDate accepts a full RFC3339 timestamp (returned unchanged) or a
// date-only YYYY-MM-DD value (expanded to YYYY-MM-DDT00:00:00Z). An empty
// string is passed through (open bound). Any other shape is an error.
func parseFlexibleDate(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", nil
	}
	if _, err := time.Parse(time.RFC3339, v); err == nil {
		// Preserve the caller's timestamp verbatim, including its offset.
		return v, nil
	}
	if _, err := time.Parse("2006-01-02", v); err == nil {
		return v + "T00:00:00Z", nil
	}
	return "", fmt.Errorf("must be an RFC3339 timestamp or a YYYY-MM-DD date (got %q)", v)
}

// buildWhere returns the GraphQL where-input value for a resource given the
// resolved filters, or nil when no filter applies. The single-field --since
// behavior remains reachable so a --since-only sync is unchanged.
func buildWhere(resource string, f syncFilters) map[string]any {
	switch resource {
	case "events":
		field := f.EventsDateField
		if field == "" {
			field = defaultEventsDateField
		}
		if f.EventsFrom != "" || f.EventsTo != "" {
			ops := map[string]any{}
			if f.EventsFrom != "" {
				ops["gte"] = f.EventsFrom
			}
			if f.EventsTo != "" {
				ops["lt"] = f.EventsTo
			}
			return map[string]any{field: ops}
		}
		if f.SinceTS != "" {
			return map[string]any{"updatedAt": map[string]any{"gte": f.SinceTS}}
		}
		return nil
	case "orders":
		// Explicit by-event scoping wins over the incremental floor: that is
		// the deliberate "fetch every order for these events" intent.
		if len(f.EventIDs) > 0 {
			return map[string]any{"eventId": map[string]any{"in": f.EventIDs}}
		}
		if f.SinceTS != "" {
			return map[string]any{"purchasedAt": map[string]any{"gte": f.SinceTS}}
		}
		return nil
	case "tickets":
		// TicketWhereInput has eventId but no date field, so --since does not
		// apply here.
		if len(f.EventIDs) > 0 {
			return map[string]any{"eventId": map[string]any{"in": f.EventIDs}}
		}
		return nil
	case "returns":
		if f.SinceTS != "" {
			return map[string]any{"returnedAt": map[string]any{"gte": f.SinceTS}}
		}
		return nil
	case "transfers":
		if f.SinceTS != "" {
			return map[string]any{"transferredAt": map[string]any{"gte": f.SinceTS}}
		}
		return nil
	default:
		return nil
	}
}

// extractFans pulls unique fan objects from order/ticket nodes (orders carry
// `fan`, tickets carry `holder`) and upserts them as resource_type='fans'.
// Returns the number of fans stored.
// persistSyncPage stores one fetched page of a resource, derives + persists its
// fans (for orders/tickets), and advances the resume cursor — IN THAT ORDER.
//
// Ordering is load-bearing: the cursor is advanced only after BOTH the resource
// page and its derived fans persist. A fan-write failure returns an error
// (failing the page) so the cursor is NOT advanced and the next sync re-fetches
// this page and re-derives the fans. Previously the fan-derivation error was
// swallowed and the cursor advanced anyway, silently and permanently losing
// those fan rows.
//
// priorStored is the cumulative resource-row count stored across all earlier
// pages of this sync run; it is summed with this page's count before the cursor
// checkpoint so sync_state.total_count stays cumulative (matching the prior
// behavior). Returns this page's stored count and fan count. Cursor advance is
// skipped when latest is true (the --latest-only backward path carries an empty
// endCursor that would clobber the forward checkpoint).
func persistSyncPage(db *store.Store, resource string, pageNodes []json.RawMessage, endCursor string, latest bool, priorStored int) (stored int, fans int, err error) {
	pageStored, _, upsertErr := db.UpsertBatch(resource, pageNodes)
	if upsertErr != nil {
		return 0, 0, fmt.Errorf("upserting %s: %w", resource, upsertErr)
	}

	if resource == "orders" || resource == "tickets" {
		pageFans, fanErr := extractFans(db, pageNodes)
		if fanErr != nil {
			// Fail the page before advancing the cursor; re-sync re-derives.
			return 0, 0, fmt.Errorf("deriving fans from %s: %w", resource, fanErr)
		}
		fans = pageFans
	}

	if !latest {
		_ = db.SaveSyncState(resource, endCursor, priorStored+pageStored)
	}
	return pageStored, fans, nil
}

// extractFans derives fan rows from a page of orders/tickets and upserts them
// into the fans table. It returns the count persisted and any write error.
//
// The error MUST be surfaced by the caller: previously this swallowed the
// UpsertBatch error and returned 0, so a fan-write failure was invisible AND
// the resume cursor advanced past the page (sync.go callback), meaning a
// re-sync never re-derived the lost fans. The sync callback now fails the page
// on a non-nil error so the cursor is not advanced and re-sync re-derives.
func extractFans(db *store.Store, nodes []json.RawMessage) (int, error) {
	seen := map[string]bool{}
	var fans []json.RawMessage
	for _, n := range nodes {
		var probe struct {
			Fan    json.RawMessage `json:"fan"`
			Holder json.RawMessage `json:"holder"`
		}
		_ = json.Unmarshal(n, &probe)
		raw := probe.Fan
		if len(raw) == 0 || string(raw) == "null" {
			raw = probe.Holder
		}
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		id := extractID(raw)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		fans = append(fans, raw)
	}
	if len(fans) == 0 {
		return 0, nil
	}
	stored, _, err := db.UpsertBatch("fans", fans)
	if err != nil {
		return 0, fmt.Errorf("upserting derived fans: %w", err)
	}
	return stored, nil
}

// containsResource reports whether resource is present in list.
func containsResource(list []string, resource string) bool {
	for _, r := range list {
		if r == resource {
			return true
		}
	}
	return false
}

func defaultSyncResources() []string {
	return []string{
		"events",
		"tickets",
		"orders",
		"returns",
		"transfers",
		"extras",
		"genres",
	}
}

// parseSinceDuration converts human-friendly duration strings into a time.Time.
func parseSinceDuration(s string) (time.Time, error) {
	re := regexp.MustCompile(`^(\d+)([dhwm])$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(s))
	if matches == nil {
		return time.Time{}, fmt.Errorf("expected format like 7d, 24h, 1w, or 30m")
	}

	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return time.Time{}, err
	}

	now := time.Now()
	switch matches[2] {
	case "d":
		return now.Add(-time.Duration(n) * 24 * time.Hour), nil
	case "h":
		return now.Add(-time.Duration(n) * time.Hour), nil
	case "w":
		return now.Add(-time.Duration(n) * 7 * 24 * time.Hour), nil
	case "m":
		return now.Add(-time.Duration(n) * time.Minute), nil
	default:
		return time.Time{}, fmt.Errorf("unknown unit %q", matches[2])
	}
}
