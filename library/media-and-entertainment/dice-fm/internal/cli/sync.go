// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.
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

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync your DICE data to local SQLite for offline search and analysis",
		Long: `Sync your events, tickets, orders, returns, transfers, extras, and genres
from the DICE Partners API into a local SQLite database. Fans are derived from
order and ticket holders so the fans commands work offline. Supports resumable
incremental sync (--since) and full resync (--full). Once synced, use 'search'
and 'sql' for instant offline queries, and the analytics commands (door,
revenue, fans, velocity, returns) for cross-event reporting.

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
  dice-fm-pp-cli sync --latest-only`,
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
				if humanFriendly {
					fmt.Fprintf(os.Stderr, "dry-run: would sync resources: %s\n", strings.Join(resources, ", "))
				} else {
					fmt.Fprintf(os.Stderr, `{"event":"sync_dry_run","resources":[%s]}`+"\n", `"`+strings.Join(resources, `","`)+`"`)
				}
				return nil
			}

			if full {
				for _, resource := range resources {
					_ = db.SaveSyncState(resource, "", 0)
				}
			}

			// --latest-only: cap at page 1 and clear the resume cursor so we
			// fetch from the head. --since takes precedence when both are set.
			if latestOnly {
				if since == "" {
					maxPages = 1
					for _, resource := range resources {
						existing, _, _, _ := db.GetSyncState(resource)
						if existing != "" {
							_ = db.SaveSyncState(resource, "", 0)
						}
					}
				} else if humanFriendly {
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
						results <- syncResource(cmd.Context(), c, db, resource, sinceTS, full, maxPages)
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
					if humanFriendly {
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
					if humanFriendly {
						fmt.Fprintf(os.Stderr, "  %s: warning: %v\n", res.Resource, res.Warn)
					}
					warnCount++
				default:
					if humanFriendly {
						fmt.Fprintf(os.Stderr, "  %s: %d synced (done)\n", res.Resource, res.Count)
					}
					totalSynced += res.Count
					successCount++
				}
			}

			elapsed := time.Since(started)
			totalResources := successCount + warnCount + errCount
			if humanFriendly {
				if warnCount > 0 {
					fmt.Fprintf(os.Stderr, "Sync complete: %d records across %d resources (%d warned, %.1fs)\n",
						totalSynced, totalResources, warnCount, elapsed.Seconds())
				} else {
					fmt.Fprintf(os.Stderr, "Sync complete: %d records across %d resources (%.1fs)\n",
						totalSynced, totalResources, elapsed.Seconds())
				}
			} else {
				fmt.Fprintf(os.Stderr, `{"event":"sync_summary","total_records":%d,"resources":%d,"success":%d,"warned":%d,"errored":%d,"duration_ms":%d}`+"\n",
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
	cmd.Flags().IntVar(&maxPages, "max-pages", 10, "Maximum pages to fetch per resource (0 = unlimited)")
	cmd.Flags().BoolVar(&latestOnly, "latest-only", false, "Refresh head of each resource only; clears resume cursor and caps pages at 1. Mutually exclusive with --since (--since wins).")

	return cmd
}

// syncResource fetches one DICE viewer connection (paginated, resumable) and
// upserts its nodes into the local store. For orders and tickets it also
// derives the fans table from embedded holder/fan objects, since DICE exposes
// no top-level fan connection.
func syncResource(ctx context.Context, c *client.Client, db *store.Store, resource, sinceTS string, full bool, maxPages int) syncResult {
	started := time.Now()
	if !humanFriendly {
		fmt.Fprintf(os.Stderr, `{"event":"sync_start","resource":"%s"}`+"\n", resource)
	}
	if _, ok := diceConnections[resource]; !ok {
		return syncResult{Resource: resource, Err: fmt.Errorf("unknown resource %q", resource), Duration: time.Since(started)}
	}

	// Under live-dogfood, curtail to a single page so the matrix's flat 30s
	// per-command timeout is not tripped by a full historical backfill.
	if cliutil.IsDogfoodEnv() && (maxPages == 0 || maxPages > 1) {
		maxPages = 1
	}

	startCursor := ""
	if !full {
		startCursor, _, _, _ = db.GetSyncState(resource)
	}

	where := syncWhere(resource, sinceTS)

	max := 0
	if maxPages > 0 {
		max = maxPages * dicePerPage
	}

	nodes, endCursor, err := fetchConnection(ctx, c, resource, where, dicePerPage, max, startCursor)
	if err != nil {
		if w, ok := isSyncAccessWarning(err); ok {
			if !humanFriendly {
				// json.Marshal escapes backslashes, newlines, and control bytes
				// that raw API bodies carry; a bare quote-only replace would
				// emit invalid JSON and break `sync --json 2>&1 | jq`.
				msgJSON, _ := json.Marshal(w.Message)
				fmt.Fprintf(os.Stderr, `{"event":"sync_warning","resource":"%s","status":%d,"reason":"%s","message":%s}`+"\n",
					resource, w.Status, w.Reason, msgJSON)
			}
			return syncResult{Resource: resource, Warn: fmt.Errorf("skipped %s: %s", resource, w.Reason), Duration: time.Since(started)}
		}
		if !humanFriendly {
			errJSON, _ := json.Marshal(err.Error())
			fmt.Fprintf(os.Stderr, `{"event":"sync_error","resource":"%s","error":%s}`+"\n", resource, errJSON)
		}
		return syncResult{Resource: resource, Err: fmt.Errorf("fetching %s: %w", resource, err), Duration: time.Since(started)}
	}

	stored, _, err := db.UpsertBatch(resource, nodes)
	if err != nil {
		return syncResult{Resource: resource, Err: fmt.Errorf("upserting %s: %w", resource, err), Duration: time.Since(started)}
	}

	fanCount := 0
	if resource == "orders" || resource == "tickets" {
		fanCount = extractFans(db, nodes)
	}

	_ = db.SaveSyncState(resource, endCursor, stored)

	if !humanFriendly {
		fmt.Fprintf(os.Stderr, `{"event":"sync_complete","resource":"%s","total":%d,"fans":%d,"duration_ms":%d}`+"\n",
			resource, stored, fanCount, time.Since(started).Milliseconds())
	} else {
		fmt.Fprintf(os.Stderr, "  %s: %d fetched\n", resource, stored)
	}

	return syncResult{Resource: resource, Count: stored, Duration: time.Since(started)}
}

// syncWhere returns the incremental date floor where-input for a resource, or
// nil when --since is unset or the resource has no date filter in its schema.
func syncWhere(resource, sinceTS string) map[string]any {
	if sinceTS == "" {
		return nil
	}
	var field string
	switch resource {
	case "events":
		field = "updatedAt"
	case "orders":
		field = "purchasedAt"
	case "returns":
		field = "returnedAt"
	case "transfers":
		field = "transferredAt"
	default:
		return nil
	}
	return map[string]any{field: map[string]any{"gte": sinceTS}}
}

// extractFans pulls unique fan objects from order/ticket nodes (orders carry
// `fan`, tickets carry `holder`) and upserts them as resource_type='fans'.
// Returns the number of fans stored.
func extractFans(db *store.Store, nodes []json.RawMessage) int {
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
		return 0
	}
	stored, _, err := db.UpsertBatch("fans", fans)
	if err != nil {
		return 0
	}
	return stored
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
