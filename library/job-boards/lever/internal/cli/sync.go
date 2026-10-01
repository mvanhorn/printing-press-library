// Copyright 2026 Hunter Veltri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/job-boards/lever/internal/client"
	"github.com/mvanhorn/printing-press-library/library/job-boards/lever/internal/store"
)

// fetchOpenPostingsSnapshot rejects incomplete or malformed feeds before a
// caller may replace the company's local snapshot.
func fetchOpenPostingsSnapshot(ctx context.Context, c *client.Client, flags *rootFlags, path string) ([]json.RawMessage, error) {
	if err := validateDataSourceStrategy(flags, "live"); err != nil {
		return nil, err
	}
	first, err := fetchOpenPostingsScan(ctx, c, path)
	if err != nil {
		return nil, err
	}
	second, err := fetchOpenPostingsScan(ctx, c, path)
	if err != nil {
		return nil, err
	}
	if err := comparePostingIDs(first, second); err != nil {
		return nil, fmt.Errorf("postings changed during pagination; local snapshot was not changed: %w", err)
	}
	return second, nil
}

func fetchOpenPostingsScan(ctx context.Context, c *client.Client, path string) ([]json.RawMessage, error) {
	const pageSize = 100
	items := make([]json.RawMessage, 0)
	for page := 0; page < paginatedGetMaxPages; page++ {
		params := map[string]string{
			"mode":   "json",
			"limit":  strconv.Itoa(pageSize),
			"offset": strconv.Itoa(page * pageSize),
		}
		data, err := c.GetWithHeadersNoCache(ctx, path, params, nil)
		if err != nil {
			return nil, err
		}
		var pageItems []json.RawMessage
		if err := json.Unmarshal(data, &pageItems); err != nil || pageItems == nil {
			return nil, fmt.Errorf("postings page %d is not a JSON array", page+1)
		}
		if len(pageItems) > pageSize {
			return nil, fmt.Errorf("postings page %d exceeds the requested limit", page+1)
		}
		items = append(items, pageItems...)
		if len(pageItems) < pageSize {
			return items, nil
		}
	}
	return nil, fmt.Errorf("postings exceed the %d-page safety limit; local snapshot was not changed", paginatedGetMaxPages)
}

func comparePostingIDs(first, second []json.RawMessage) error {
	collect := func(items []json.RawMessage) (map[string]struct{}, error) {
		ids := make(map[string]struct{}, len(items))
		for i, raw := range items {
			var item struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &item); err != nil || strings.TrimSpace(item.ID) == "" {
				return nil, fmt.Errorf("posting %d has no valid ID", i+1)
			}
			if _, exists := ids[item.ID]; exists {
				return nil, fmt.Errorf("posting ID appears twice")
			}
			ids[item.ID] = struct{}{}
		}
		return ids, nil
	}
	firstIDs, err := collect(first)
	if err != nil {
		return err
	}
	secondIDs, err := collect(second)
	if err != nil {
		return err
	}
	if len(firstIDs) != len(secondIDs) {
		return fmt.Errorf("the two scans have different posting counts")
	}
	for id := range firstIDs {
		if _, ok := secondIDs[id]; !ok {
			return fmt.Errorf("the two scans have different posting IDs")
		}
	}
	return nil
}

// newSyncCmd fetches every open posting for a company and persists it to
// the local store under a company-scoped key (postings:leverdemo), so
// local reads stay per-company and work offline. The scaffold referenced
// `sync` in missing-store guidance, MCP error text, and docs, but
// generated no command; this wires the real one.
// pp:data-source live
func newSyncCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "sync <company>",
		Short:        "Fetch all open postings for a company and store them locally",
		Example:      "  lever-pp-cli sync leverdemo",
		Annotations:  map[string]string{"mcp:local-write": "true"},
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sync")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			company := args[0]
			path := replacePathParam("/postings/{company}", "company", company)
			// Fetch every page before replacing the company snapshot.
			items, err := fetchOpenPostingsSnapshot(cmd.Context(), c, flags, path)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Persist under a company-scoped key and verify durability, so
			// a failed store write never masquerades as a successful sync.
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("lever-pp-cli"))
			if err != nil {
				return fmt.Errorf("sync: open local store: %w", err)
			}
			defer db.Close()

			scoped := "postings:" + company
			stored, err := db.UpsertAuthoritativeSnapshot(scoped, items)
			if err != nil {
				return fmt.Errorf("sync postings: persist: %w", err)
			}
			skipped := 0
			// The replacement count describes the complete open-postings
			// snapshot, including removals of jobs Lever no longer lists.
			if len(items) > 0 && stored == 0 {
				return fmt.Errorf("sync postings: fetched %d records but none persisted (%d skipped); the sync did not persist", len(items), skipped)
			}
			readback, err := db.List(scoped, 0)
			if err != nil {
				return fmt.Errorf("sync postings: verify local write: %w", err)
			}
			if len(items) > 0 && len(readback) == 0 {
				return fmt.Errorf("sync postings: fetched %d records but the local store holds none; the sync did not persist", len(items))
			}
			// Record the sync marker only after the write and the
			// verification both succeeded, using the persisted count.
			if err := db.SaveSyncState(scoped, "", stored); err != nil {
				return fmt.Errorf("sync postings: record sync state: %w", err)
			}
			if flags != nil && flags.asJSON {
				data, err := wrapAgentOutput(json.RawMessage("null"), map[string]any{
					"source":        "live",
					"resource_type": "postings",
					"company":       company,
					"stored":        stored,
					"skipped":       skipped,
					"synced_at":     time.Now().UTC().Format(time.RFC3339),
				})
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d records for company %q\n", stored, company)
			return nil
		},
	}
	return cmd
}
