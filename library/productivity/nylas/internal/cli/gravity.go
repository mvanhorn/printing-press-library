// Copyright 2026 nathan-kettles. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/nylas/internal/store"
)

// gravityRow is one ranked counterparty in the cross-grant interaction
// graph. weight = sent_to + received_from + meeting_attended.
type gravityRow struct {
	Email    string `json:"email"`
	Name     string `json:"name,omitempty"`
	Sent     int    `json:"sent"`
	Received int    `json:"received"`
	Meetings int    `json:"meetings"`
	Weight   int    `json:"weight"`
	Grants   int    `json:"grants_seen"`
}

func newGravityCmd(flags *rootFlags) *cobra.Command {
	var top int
	var since string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "gravity",
		Short: "Rank counterparties by cross-grant interaction weight",
		Long: `Compute interaction weight for every email address that has appeared
in this tenant's synced messages and events — unified across every
connected grant.

Weight = messages-sent-to + messages-received-from + meetings-attended.
"grants_seen" is the number of distinct grants in which the counterparty
appears, which is impossible to compute against the live Nylas API in
one call.`,
		Example: strings.Trim(`
  # Top 25 counterparties across the tenant, last 90 days
  nylas-pp-cli gravity --top 25 --since 90d --agent

  # All-time gravity, output as table
  nylas-pp-cli gravity --top 10
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("nylas-pp-cli")
			}
			autoRefreshIfStale(cmd.Context(), dbPath, cmd.ErrOrStderr())
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'nylas-pp-cli sync' first.", err)
			}
			defer db.Close()

			params := []any{}
			where := ""
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return fmt.Errorf("invalid --since: %w", err)
				}
				cutoff := ts.UTC().Format("2006-01-02 15:04:05")
				where = " AND synced_at >= ?"
				params = append(params, cutoff)
			}

			counters := make(map[string]*gravityRow)
			grantSet := make(map[string]map[string]struct{})

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT COALESCE(grants_id,'') AS grants_id,
				        COALESCE(json_extract(data,'$.from[0].email'), '') AS from_email,
				        COALESCE(json_extract(data,'$.from[0].name'), '')  AS from_name,
				        COALESCE(json_extract(data,'$.to'), '[]')           AS to_json
				   FROM grants_messages WHERE 1=1`+where, params...)
			if err != nil {
				return fmt.Errorf("querying grants_messages: %w\nRun 'nylas-pp-cli sync' first.", err)
			}
			for rows.Next() {
				var grant, fromEmail, fromName, toJSON string
				if err := rows.Scan(&grant, &fromEmail, &fromName, &toJSON); err != nil {
					continue
				}
				bumpGravity(counters, grantSet, strings.ToLower(strings.TrimSpace(fromEmail)), fromName, grant, "received")
				for _, ent := range parseEmailArray(toJSON) {
					bumpGravity(counters, grantSet, strings.ToLower(ent.email), ent.name, grant, "sent")
				}
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return fmt.Errorf("iterating grants_messages: %w", err)
			}
			rows.Close()

			rows2, err := db.DB().QueryContext(cmd.Context(),
				`SELECT COALESCE(grants_id,'') AS grants_id,
				        COALESCE(json_extract(data,'$.participants'), '[]') AS parts
				   FROM events WHERE 1=1`+where, params...)
			if err != nil {
				return fmt.Errorf("querying events: %w", err)
			}
			for rows2.Next() {
				var grant, parts string
				if err := rows2.Scan(&grant, &parts); err != nil {
					continue
				}
				for _, ent := range parseEmailArray(parts) {
					bumpGravity(counters, grantSet, strings.ToLower(ent.email), ent.name, grant, "meeting")
				}
			}
			if err := rows2.Err(); err != nil {
				rows2.Close()
				return fmt.Errorf("iterating events: %w", err)
			}
			rows2.Close()

			for email, gset := range grantSet {
				if r := counters[email]; r != nil {
					r.Grants = len(gset)
				}
			}
			out := make([]gravityRow, 0, len(counters))
			for _, r := range counters {
				r.Weight = r.Sent + r.Received + r.Meetings
				out = append(out, *r)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Weight > out[j].Weight })
			if top > 0 && len(out) > top {
				out = out[:top]
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().IntVar(&top, "top", 25, "Show top N counterparties (0 = all)")
	cmd.Flags().StringVar(&since, "since", "", "Restrict to data synced within this duration (e.g. 30d, 90d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite database")
	return cmd
}

type emailEntry struct{ email, name string }

func parseEmailArray(raw string) []emailEntry {
	if raw == "" || raw == "null" || raw == "[]" {
		return nil
	}
	type item struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	var arr []item
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil
	}
	out := make([]emailEntry, 0, len(arr))
	for _, it := range arr {
		out = append(out, emailEntry{email: it.Email, name: it.Name})
	}
	return out
}

func bumpGravity(counters map[string]*gravityRow, grantSet map[string]map[string]struct{}, email, name, grant, kind string) {
	if email == "" {
		return
	}
	r, ok := counters[email]
	if !ok {
		r = &gravityRow{Email: email, Name: name}
		counters[email] = r
	}
	if r.Name == "" && name != "" {
		r.Name = name
	}
	switch kind {
	case "sent":
		r.Sent++
	case "received":
		r.Received++
	case "meeting":
		r.Meetings++
	}
	if grant != "" {
		if _, ok := grantSet[email]; !ok {
			grantSet[email] = map[string]struct{}{}
		}
		grantSet[email][grant] = struct{}{}
	}
}
