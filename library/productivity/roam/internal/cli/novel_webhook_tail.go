package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/roam/internal/store"
	"time"

	"github.com/spf13/cobra"
)

func newWebhookTailCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int

	cmd := &cobra.Command{
		Use:   "webhook-tail",
		Short: "Tail recent webhook deliveries from the local registry",
		Long: `Reads from the local webhook_deliveries table populated when this CLI is run as a
webhook listener (or by manual /webhook.subscribe + delivery capture).

Run 'roam-pp-cli webhook subscribe' to register a subscription first.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var _ store.Store
			var db *sql.DB
			db, closeDB, err := openNovelDB(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer closeDB()
			if err := ensureMessagesTables(cmd.Context(), db); err != nil {
				return apiErr(err)
			}

			cutoff := ""
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(err)
				}
				cutoff = ts.UTC().Format(time.RFC3339)
			}
			if limit <= 0 {
				limit = 50
			}

			sql := `SELECT id, subscription_id, event, received_at, http_status, payload FROM webhook_deliveries WHERE 1=1`
			a := []any{}
			if cutoff != "" {
				sql += ` AND received_at >= ?`
				a = append(a, cutoff)
			}
			sql += fmt.Sprintf(` ORDER BY received_at DESC LIMIT %d`, limit)
			rows, err := db.QueryContext(cmd.Context(), sql, a...)
			if err != nil {
				return apiErr(fmt.Errorf("query deliveries: %w", err))
			}
			defer rows.Close()

			type delivery struct {
				ID             string          `json:"id"`
				SubscriptionID string          `json:"subscription_id"`
				Event          string          `json:"event"`
				ReceivedAt     string          `json:"received_at"`
				HTTPStatus     int             `json:"http_status"`
				Payload        json.RawMessage `json:"payload,omitempty"`
			}
			out := []delivery{}
			for rows.Next() {
				var d delivery
				var payload string
				_ = rows.Scan(&d.ID, &d.SubscriptionID, &d.Event, &d.ReceivedAt, &d.HTTPStatus, &payload)
				if payload != "" {
					d.Payload = json.RawMessage(payload)
				}
				out = append(out, d)
			}

			w := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(w) {
				body, _ := json.Marshal(map[string]any{"deliveries": out})
				fmt.Fprintln(w, string(body))
				return nil
			}
			if len(out) == 0 {
				fmt.Fprintln(w, "(no deliveries logged — register a subscription with 'roam-pp-cli webhook subscribe')")
				return nil
			}
			fmt.Fprintf(w, "%-25s  %-20s  %-6s  %s\n", "RECEIVED", "EVENT", "STATUS", "ID")
			for _, d := range out {
				fmt.Fprintf(w, "%-25s  %-20s  %-6d  %s\n", d.ReceivedAt, d.Event, d.HTTPStatus, d.ID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only show deliveries since (e.g. 1h, 24h)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max deliveries to surface")
	return cmd
}
