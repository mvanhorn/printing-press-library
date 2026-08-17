// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/v0/internal/store"
	"github.com/spf13/cobra"
)

type v0UsageSnapshot struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
	Total      float64 `json:"total"`
}

type v0SpendRow struct {
	Group    string          `json:"group"`
	Messages int             `json:"messages"`
	Credits  float64         `json:"credits"`
	Tokens   v0UsageSnapshot `json:"tokens"`
}

type v0SpendView struct {
	Since  string       `json:"since,omitempty"`
	By     string       `json:"grouped_by"`
	Rows   []v0SpendRow `json:"rows"`
	Totals v0SpendRow   `json:"totals"`
	Note   string       `json:"note,omitempty"`
}

func newNovelSpendCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagBy string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "spend",
		Short: "Aggregate v0 credit cost and token usage from the synced message mirror, grouped by chat, day, or model.",
		Long:  `Aggregate v0 credit cost and token usage from the local message mirror. Run 'v0-pp-cli sync --resources chats,messages' first to populate usage data. Every v0 generation bills credits; this command shows where they went.`,
		Example: `  v0-pp-cli spend
  v0-pp-cli spend --since 7d --by chat
  v0-pp-cli spend --by day --json
  v0-pp-cli spend --since 30d --by chat --json --select credits,tokens.total`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "spend")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if flagBy == "" {
				flagBy = "chat"
			}
			switch flagBy {
			case "chat", "day", "model":
			default:
				return usageErr(fmt.Errorf("--by must be one of: chat, day, model (got %q)", flagBy))
			}

			if flagDB == "" {
				flagDB = defaultDBPath("v0-pp-cli")
			}
			if _, statErr := os.Stat(flagDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: v0-pp-cli sync --resources chats,messages --db %s\n", flagDB, flagDB)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), v0SpendView{By: flagBy, Rows: make([]v0SpendRow, 0)}, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, flagDB)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			var sinceDur time.Duration
			if flagSince != "" {
				sinceDur, err = parseV0Duration(flagSince)
				if err != nil {
					return usageErr(fmt.Errorf("--since must be a duration like 7d or 24h (got %q)", flagSince))
				}
			}

			rows, err := db.Query(`SELECT chat_id, created_at, data FROM messages`)
			if err != nil {
				return fmt.Errorf("querying messages: %w", err)
			}
			type rawMsg struct {
				chatID    string
				createdAt string
				credits   float64
				tokens    v0UsageSnapshot
			}
			var msgs []rawMsg
			cutoff := time.Now().Add(-sinceDur)
			for rows.Next() {
				var chatID, createdAt string
				var data []byte
				if err := rows.Scan(&chatID, &createdAt, &data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan message: %w", err)
				}
				var m struct {
					CreatedAt string `json:"createdAt"`
					ChatID    string `json:"chatId"`
					Usage     struct {
						Tokens struct {
							Input      float64 `json:"input"`
							Output     float64 `json:"output"`
							CacheRead  float64 `json:"cacheRead"`
							CacheWrite float64 `json:"cacheWrite"`
							Total      float64 `json:"total"`
						} `json:"tokens"`
						CreditsCost struct {
							Total float64 `json:"total"`
						} `json:"creditsCost"`
					} `json:"usage"`
				}
				if err := json.Unmarshal(data, &m); err != nil {
					continue
				}
				if m.CreatedAt != "" {
					createdAt = m.CreatedAt
				}
				if m.ChatID != "" {
					chatID = m.ChatID
				}
				if sinceDur > 0 && createdAt != "" {
					ts, perr := parseV0Time(createdAt)
					if perr == nil && ts.Before(cutoff) {
						continue
					}
				}
				if m.Usage.CreditsCost.Total == 0 && m.Usage.Tokens.Total == 0 {
					continue
				}
				msgs = append(msgs, rawMsg{
					chatID:    chatID,
					createdAt: createdAt,
					credits:   m.Usage.CreditsCost.Total,
					tokens: v0UsageSnapshot{
						Input:      m.Usage.Tokens.Input,
						Output:     m.Usage.Tokens.Output,
						CacheRead:  m.Usage.Tokens.CacheRead,
						CacheWrite: m.Usage.Tokens.CacheWrite,
						Total:      m.Usage.Tokens.Total,
					},
				})
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate messages: %w", err)
			}
			_ = rows.Close()

			modelByMsg := map[string]string{}
			if flagBy == "model" {
				modelRows, qerr := db.Query(`SELECT chat_id, model FROM model_usage`)
				if qerr == nil {
					for modelRows.Next() {
						var cid, model string
						if err := modelRows.Scan(&cid, &model); err == nil {
							modelByMsg[cid] = model
						}
					}
					_ = modelRows.Close()
				}
			}

			groups := map[string]*v0SpendRow{}
			var order []string
			for _, m := range msgs {
				var key string
				switch flagBy {
				case "chat":
					key = m.chatID
				case "day":
					key = "unknown"
					if ts, err := parseV0Time(m.createdAt); err == nil {
						key = ts.Format("2006-01-02")
					}
				case "model":
					key = modelByMsg[m.chatID]
					if key == "" {
						key = "unknown"
					}
				}
				if key == "" {
					key = "unknown"
				}
				if _, ok := groups[key]; !ok {
					groups[key] = &v0SpendRow{Group: key}
					order = append(order, key)
				}
				r := groups[key]
				r.Messages++
				r.Credits += m.credits
				r.Tokens.Input += m.tokens.Input
				r.Tokens.Output += m.tokens.Output
				r.Tokens.CacheRead += m.tokens.CacheRead
				r.Tokens.CacheWrite += m.tokens.CacheWrite
				r.Tokens.Total += m.tokens.Total
			}
			sort.Slice(order, func(i, j int) bool {
				return groups[order[i]].Credits > groups[order[j]].Credits
			})

			rowsOut := make([]v0SpendRow, 0, len(order))
			var totals v0SpendRow
			for _, key := range order {
				r := groups[key]
				rowsOut = append(rowsOut, *r)
				totals.Messages += r.Messages
				totals.Credits += r.Credits
				totals.Tokens.Input += r.Tokens.Input
				totals.Tokens.Output += r.Tokens.Output
				totals.Tokens.CacheRead += r.Tokens.CacheRead
				totals.Tokens.CacheWrite += r.Tokens.CacheWrite
				totals.Tokens.Total += r.Tokens.Total
			}
			totals.Group = "total"
			view := v0SpendView{Since: flagSince, By: flagBy, Rows: rowsOut, Totals: totals}
			if flagBy == "model" {
				view.Note = "model attribution comes from CLI-originated generations (chats stream --model); synced-only messages report as unknown"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rowsOut) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No usage found. Run: v0-pp-cli sync --resources chats,messages")
				return nil
			}
			for _, r := range rowsOut {
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %6d msgs  %10.5f credits  %12.0f tokens\n", truncateV0Label(r.Group, 24), r.Messages, r.Credits, r.Tokens.Total)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-24s %6d msgs  %10.5f credits  %12.0f tokens\n", "total", totals.Messages, totals.Credits, totals.Tokens.Total)
			if view.Note != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only count messages newer than this duration (e.g. 7d, 24h)")
	cmd.Flags().StringVar(&flagBy, "by", "chat", "Grouping dimension: chat, day, or model")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (defaults to the standard v0-pp-cli mirror)")
	return cmd
}

func truncateV0Label(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

func parseV0Time(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if ts, err := time.Parse(layout, s); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp %q", s)
}

func parseV0Duration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
