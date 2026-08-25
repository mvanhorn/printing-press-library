// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/source/magic"
	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/source/taskrabbit"
	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/store"
)

func newNovelStatusCmd(flags *rootFlags) *cobra.Command {
	var flagOpen bool

	cmd := &cobra.Command{
		Use:         "status",
		Short:       "One list of every in-flight task across TaskRabbit bookings and Magic requests",
		Example:     "  human-goat-pp-cli status --open --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// status takes no required flags or args — a bare `status` is its
			// primary use, so it must execute rather than print help by default.
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list in-flight tasks across sources")
				return nil
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("status does not accept positional arguments"))
			}

			out := statusOutput{Tasks: make([]statusRow, 0)}

			// TaskRabbit: page through all current bookings rather than the first page.
			c, err := flags.newClient()
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "taskrabbit: %v\n", err)
			} else {
				tr := taskrabbit.New(c)
				// --open scopes the TaskRabbit listing to active bookings too, not
				// just the Magic side; without this filter status --open returns all
				// historical bookings.
				trFilter := map[string]any{}
				if flagOpen {
					trFilter["status"] = "active"
				}
				bookings, err := listAllBookings(cmd.Context(), tr, trFilter)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "taskrabbit: %v\n", err)
				} else {
					for _, booking := range bookings {
						out.Tasks = append(out.Tasks, statusRow{
							Source: "taskrabbit",
							ID:     booking.ID,
							Status: booking.Status,
							Title:  "",
						})
					}
				}
			}

			// Magic: the API has no list endpoint, so the local store is the inbox.
			// Read the request IDs recorded by send/call/dispatch, then refresh each
			// one live so the reported status reflects current progress.
			magicRows, magicNote := magicStatusRows(cmd, flagOpen)
			out.Tasks = append(out.Tasks, magicRows...)
			out.MagicNote = magicNote

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			tableRows := make([][]string, 0, len(out.Tasks))
			for _, row := range out.Tasks {
				tableRows = append(tableRows, []string{row.Source, row.ID, row.Status, row.Title})
			}
			if err := flags.printTable(cmd, []string{"SOURCE", "ID", "STATUS", "TITLE"}, tableRows); err != nil {
				return err
			}
			if out.MagicNote != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), out.MagicNote)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagOpen, "open", false, "Show only in-flight work: narrows Magic requests to in-progress ones (TaskRabbit's task list is active-only by API design, so its rows are always in-flight)")
	return cmd
}

// listAllBookings pages through TaskRabbit bookings so status is not capped at
// the first page. Bounded so a paging bug can't loop forever.
func listAllBookings(ctx context.Context, tr *taskrabbit.Client, filter map[string]any) ([]taskrabbit.Booking, error) {
	const perPage = 50
	const maxPages = 40
	var all []taskrabbit.Booking
	for page := 1; page <= maxPages; page++ {
		batch, err := tr.ListTasks(ctx, page, perPage, filter, "en-US")
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < perPage {
			break
		}
	}
	return all, nil
}

// magicStatusRows loads the locally-recorded Magic requests and refreshes each
// one live. It returns the rows plus a human-facing note describing why the
// list may be empty (no store, no API key, or nothing tracked yet).
func magicStatusRows(cmd *cobra.Command, openOnly bool) ([]statusRow, string) {
	dbPath := defaultDBPath("human-goat-pp-cli")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, "Magic: no locally tracked requests yet (send/call/dispatch record them here)."
	}
	db, err := store.OpenReadOnlyContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Sprintf("Magic: could not open local store: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = 'magic'`)
	if err != nil {
		return nil, fmt.Sprintf("Magic: could not read local store: %v", err)
	}
	var ids []string
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			_ = rows.Close()
			return nil, fmt.Sprintf("Magic: could not scan local store: %v", err)
		}
		var stored magic.Request
		if err := json.Unmarshal(data, &stored); err == nil && stored.ID != "" {
			ids = append(ids, stored.ID)
		}
	}
	_ = rows.Close()
	if len(ids) == 0 {
		return nil, "Magic: no locally tracked requests yet (send/call/dispatch record them here)."
	}

	client, err := magic.NewClient()
	if err != nil {
		return nil, fmt.Sprintf("Magic: %d tracked request(s), but live refresh unavailable: %v", len(ids), err)
	}

	out := make([]statusRow, 0, len(ids))
	for _, id := range ids {
		req, err := client.GetRequest(cmd.Context(), id)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "magic %s: %v\n", id, err)
			continue
		}
		if openOnly && !magic.IsInProgress(req.Status) {
			continue
		}
		out = append(out, statusRow{
			Source: "magic",
			ID:     req.ID,
			Status: req.Status,
			Title:  req.Title,
		})
	}
	return out, ""
}

type statusOutput struct {
	Tasks     []statusRow `json:"tasks"`
	MagicNote string      `json:"magic_note,omitempty"`
}

type statusRow struct {
	Source string `json:"source"`
	ID     string `json:"id"`
	Status string `json:"status"`
	Title  string `json:"title"`
}
