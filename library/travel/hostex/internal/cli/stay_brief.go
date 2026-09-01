// Hand-authored transcendence command. generate --force preserves this file.
// pp:data-source local
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelStayBriefCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	cmd := &cobra.Command{
		Use:   "stay-brief <stay_code>",
		Short: "One dossier for a single stay: reservation, guest, thread, tasks, transactions, review.",
		Long: "Fan-joins every locally synced entity keyed on a stay_code — reservation,\n" +
			"linked conversation, tasks, transactions, and review — into one object, so an\n" +
			"agent gets the full picture of a stay without five separate lookups.\n\n" +
			"Reads the local mirror; run `hostex-pp-cli sync` first. To scan many stays for\n" +
			"problems use `ops-gaps` instead.",
		Example:     "  hostex-pp-cli stay-brief HMABC123 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if err := rejectLiveDataSource(flags); err != nil {
				return err
			}
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a stay_code argument is required"))
			}
			stayCode := strings.TrimSpace(args[0])

			db, done, err := openLocalMirror(cmd, flags, flagDB)
			if done {
				return err
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "reservations") {
				hintIfStale(cmd, db, "reservations", flags.maxAge)
			}

			reservations, err := listObjs(db, "reservations")
			if err != nil {
				return err
			}
			var reservation map[string]any
			for _, r := range reservations {
				if novStr(r, "stay_code") == stayCode {
					reservation = r
					break
				}
			}
			if reservation == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "no reservation with stay_code %q in the local mirror\n", stayCode)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "null")
				}
				return nil
			}

			matchStay := func(objs []map[string]any) []map[string]any {
				out := make([]map[string]any, 0)
				for _, o := range objs {
					if novStr(o, "stay_code") == stayCode {
						out = append(out, o)
					}
				}
				return out
			}

			tasksAll, err := listObjs(db, "tasks")
			if err != nil {
				return fmt.Errorf("reading tasks: %w", err)
			}
			txAll, err := listObjs(db, "transactions")
			if err != nil {
				return fmt.Errorf("reading transactions: %w", err)
			}
			reviewsAll, err := listObjs(db, "reviews")
			if err != nil {
				return fmt.Errorf("reading reviews: %w", err)
			}
			tasks := matchStay(tasksAll)
			transactions := matchStay(txAll)

			reservationCode := novStr(reservation, "reservation_code")
			reviews := make([]map[string]any, 0)
			for _, rv := range reviewsAll {
				if novStr(rv, "stay_code") == stayCode || (reservationCode != "" && novStr(rv, "reservation_code") == reservationCode) {
					reviews = append(reviews, rv)
				}
			}

			var conversation map[string]any
			convID := novStr(reservation, "conversation_id")
			if convID != "" {
				convAll, err := listObjs(db, "conversations")
				if err != nil {
					return fmt.Errorf("reading conversations: %w", err)
				}
				for _, c := range convAll {
					if novStr(c, "id") == convID {
						conversation = c
						break
					}
				}
			}

			// Summary metrics.
			var income, expense float64
			for _, t := range transactions {
				amt, ok := novNum(t, "amount")
				if !ok {
					continue
				}
				if strings.EqualFold(novStr(t, "direction"), "expense") {
					expense += amt
				} else {
					income += amt
				}
			}
			openTasks := 0
			for _, t := range tasks {
				if !strings.EqualFold(novStr(t, "status"), "completed") {
					openTasks++
				}
			}

			view := struct {
				StayCode     string           `json:"stay_code"`
				Reservation  map[string]any   `json:"reservation"`
				Conversation map[string]any   `json:"conversation"`
				Tasks        []map[string]any `json:"tasks"`
				Transactions []map[string]any `json:"transactions"`
				Reviews      []map[string]any `json:"reviews"`
				Summary      map[string]any   `json:"summary"`
			}{
				StayCode:     stayCode,
				Reservation:  reservation,
				Conversation: conversation,
				Tasks:        tasks,
				Transactions: transactions,
				Reviews:      reviews,
				Summary: map[string]any{
					"net_revenue":  income - expense,
					"income":       income,
					"expense":      expense,
					"open_tasks":   openTasks,
					"task_count":   len(tasks),
					"has_thread":   conversation != nil,
					"review_count": len(reviews),
				},
			}
			return novEmit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite mirror (default: per-user data dir)")
	return cmd
}
