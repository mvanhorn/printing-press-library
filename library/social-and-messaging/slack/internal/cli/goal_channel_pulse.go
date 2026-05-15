// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// This file is hand-built (NOT generator-emitted). It implements
// `goal-channel-pulse` — per active Asana Rock in the current quarter,
// the mapped Slack channel's 7-day message count, unique participants,
// total reactions and a stalled flag. The Rock-to-channel mapping comes
// from an explicit slack_channel: key in rocks.yml; the Asana side is
// read from the sibling pp-asana mirror via ATTACH DATABASE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// rockMapping is one entry of rocks.yml: a Rock and the Slack channel it
// is discussed in.
type rockMapping struct {
	Rock         string `json:"rock"`
	SlackChannel string `json:"slack_channel"`
}

// parseRocksYAML extracts rock/slack_channel pairs from a minimal YAML
// doc. The supported shape is a top-level `rocks:` list of mappings:
//
//	rocks:
//	  - rock: "Ship CSM v2"
//	    slack_channel: "#csm-signals"
//	  - rock: "Attio migration"
//	    slack_channel: "#attio-migration"
//
// A hand-rolled scanner is used to avoid pulling a YAML dependency into
// go.mod, mirroring parseSkipYAML's approach in sync_mirror.go. Comments
// (#, except inside quotes) and surrounding quotes are stripped.
func parseRocksYAML(doc string) []rockMapping {
	var out []rockMapping
	var cur *rockMapping
	for _, raw := range strings.Split(doc, "\n") {
		line := stripYAMLComment(raw)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == "rocks:" {
			continue
		}
		// New list item.
		if strings.HasPrefix(trimmed, "- ") {
			if cur != nil && (cur.Rock != "" || cur.SlackChannel != "") {
				out = append(out, *cur)
			}
			cur = &rockMapping{}
			trimmed = strings.TrimSpace(trimmed[2:])
			if trimmed == "" {
				continue
			}
		}
		if cur == nil {
			cur = &rockMapping{}
		}
		key, val, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = cleanYAMLValue(val)
		switch key {
		case "rock", "name", "goal":
			cur.Rock = val
		case "slack_channel", "channel":
			cur.SlackChannel = val
		}
	}
	if cur != nil && (cur.Rock != "" || cur.SlackChannel != "") {
		out = append(out, *cur)
	}
	return out
}

// stripYAMLComment removes a trailing # comment from a line, but only
// when the # is not inside a quoted string.
func stripYAMLComment(line string) string {
	inS, inD := false, false
	for i, r := range line {
		switch r {
		case '\'':
			if !inD {
				inS = !inS
			}
		case '"':
			if !inS {
				inD = !inD
			}
		case '#':
			if !inS && !inD {
				return line[:i]
			}
		}
	}
	return line
}

// cleanYAMLValue trims whitespace and surrounding quotes from a value.
func cleanYAMLValue(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	return strings.TrimSpace(s)
}

// loadRocksFile reads the rocks.yml mapping file. The default location
// is ~/.config/slack-pp-cli/rocks.yml; an explicit path overrides it. An
// absent default file is not an error — it yields an empty mapping so
// the verb can emit an honest empty result with a note.
func loadRocksFile(explicit string) ([]rockMapping, string, bool, error) {
	path := explicit
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, "", false, fmt.Errorf("resolving home directory: %w", err)
		}
		path = filepath.Join(home, ".config", "slack-pp-cli", "rocks.yml")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, path, false, nil
		}
		return nil, path, false, fmt.Errorf("reading rocks file %s: %w", path, err)
	}
	return parseRocksYAML(string(data)), path, true, nil
}

// channelPulse is the computed pulse for one Rock's Slack channel.
type channelPulse struct {
	Rock              string `json:"rock"`
	SlackChannel      string `json:"slack_channel"`
	ChannelID         string `json:"channel_id,omitempty"`
	ChannelResolved   bool   `json:"channel_resolved"`
	AsanaRockActive   *bool  `json:"asana_rock_active,omitempty"`
	MessageCount      int    `json:"message_count"`
	UniqueParticipants int   `json:"unique_participants"`
	TotalReactions    int    `json:"total_reactions"`
	Stalled           bool   `json:"stalled"`
}

// goalChannelPulseReport is the full JSON shape of the verb.
type goalChannelPulseReport struct {
	Quarter        string         `json:"quarter"`
	RocksFile      string         `json:"rocks_file"`
	Note           string         `json:"note,omitempty"`
	Pulses         []channelPulse `json:"pulses"`
	MissingSources []string       `json:"missing_sources"`
}

// computeChannelPulse computes message count, unique participants and
// total reactions for one channel's messages over the window. Pure
// aggregation kept testable.
func computeChannelPulse(msgs []store.Message, reactions []store.Reaction, since, until string) (msgCount, participants, totalReactions int) {
	inWindow := map[string]bool{}
	users := map[string]bool{}
	for _, m := range msgs {
		if since != "" && m.TS < since {
			continue
		}
		if until != "" && m.TS > until {
			continue
		}
		inWindow[m.TS] = true
		msgCount++
		if m.UserID != "" {
			users[m.UserID] = true
		}
	}
	for _, r := range reactions {
		if inWindow[r.MessageTS] {
			totalReactions += r.Count
		}
	}
	return msgCount, len(users), totalReactions
}

// attachAsanaRockActive queries the attached Asana mirror for whether a
// Rock (goal) is active in the current quarter. Defensive against
// unknown sibling schema — returns nil when the source is unusable.
func attachAsanaRockActive(ctx context.Context, cs *crossSource, rock string) *bool {
	q := `SELECT COUNT(*) FROM x_asana.goals
	      WHERE name LIKE '%' || ? || '%' COLLATE NOCASE
	        AND (is_complete = 0 OR is_complete IS NULL)`
	rows, ok := cs.probeQuery(ctx, "asana", q, rock)
	if !ok {
		return nil
	}
	defer rows.Close()
	var n int
	if rows.Next() {
		_ = rows.Scan(&n)
	}
	active := n > 0
	return &active
}

func newGoalChannelPulseCmd(flags *rootFlags) *cobra.Command {
	var quarter, rocksFile, window, dbPath string
	var skipMissing bool

	cmd := &cobra.Command{
		Use:   "goal-channel-pulse",
		Short: "Per Asana Rock, the discussion pulse of its mapped Slack channel",
		Long: `goal-channel-pulse reports, for each Rock listed in rocks.yml, the
7-day discussion pulse of the Slack channel mapped to it: message
count, unique participants, total reactions, and a 'stalled' flag set
when discussion is zero.

The Rock-to-channel mapping is read from rocks.yml (default
~/.config/slack-pp-cli/rocks.yml, override with --rocks-file). Each
entry needs a 'rock:' and a 'slack_channel:' key. The Slack pulse is
computed from this CLI's local mirror. Whether each Rock is still active
is read from the sibling pp-asana CLI's SQLite mirror via ATTACH
DATABASE — when that mirror is absent the asana_rock_active field is
omitted and "asana" is listed in 'missing_sources'.

If rocks.yml does not exist the verb emits an honest empty result with
an explanatory note. No live Slack calls are made.`,
		Example: stringTrimNL(`
  # Pulse for every Rock this quarter
  slack-pp-cli goal-channel-pulse --quarter current --agent

  # Use an explicit rocks.yml
  slack-pp-cli goal-channel-pulse --rocks-file ./rocks.yml --json

  # Preview without touching any database
  slack-pp-cli goal-channel-pulse --dry-run`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			since, until, err := windowBounds(window)
			if err != nil {
				return usageErr(err)
			}
			mappings, rocksPath, found, err := loadRocksFile(rocksFile)
			if err != nil {
				return configErr(err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'slack-pp-cli sync mirror' first.", err)
			}
			defer db.Close()

			report := goalChannelPulseReport{
				Quarter:   quarter,
				RocksFile: rocksPath,
				Pulses:    []channelPulse{},
			}
			if !found {
				report.Note = "rocks.yml not found at " + rocksPath +
					" — create it with rock/slack_channel mappings to populate this report"
				report.MissingSources = []string{}
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			if len(mappings) == 0 {
				report.Note = "rocks.yml at " + rocksPath + " contains no rock/slack_channel entries"
			}

			cs, _ := newCrossSource(cmd.Context(), db.DB(), map[string]string{
				"asana": "asana-pp-cli",
			})
			defer cs.detach(cmd.Context())

			for _, mp := range mappings {
				p := channelPulse{Rock: mp.Rock, SlackChannel: mp.SlackChannel}
				ch, rerr := db.ResolveChannel(cmd.Context(), mp.SlackChannel)
				if rerr == nil {
					p.ChannelResolved = true
					p.ChannelID = ch.ID
					msgs, merr := db.MessagesInWindow(cmd.Context(), []string{ch.ID}, since, until)
					if merr != nil {
						return fmt.Errorf("reading messages for %s: %w", mp.SlackChannel, merr)
					}
					reactions, rxerr := db.ReactionsForChannel(cmd.Context(), ch.ID)
					if rxerr != nil {
						return fmt.Errorf("reading reactions for %s: %w", mp.SlackChannel, rxerr)
					}
					p.MessageCount, p.UniqueParticipants, p.TotalReactions =
						computeChannelPulse(msgs, reactions, since, until)
					p.Stalled = p.MessageCount == 0
				} else if rerr == sql.ErrNoRows {
					// Channel not in mirror — count as stalled, unresolved.
					p.Stalled = true
				} else {
					return usageErr(rerr)
				}
				p.AsanaRockActive = attachAsanaRockActive(cmd.Context(), cs, mp.Rock)
				report.Pulses = append(report.Pulses, p)
			}

			missing := cs.missing()
			if missing == nil {
				missing = []string{}
			}
			report.MissingSources = missing
			_ = skipMissing // flag is informational; degradation is unconditional
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&quarter, "quarter", "current", "Quarter label (informational; e.g. current, Q2-2026)")
	cmd.Flags().StringVar(&rocksFile, "rocks-file", "", "Path to rocks.yml (default: ~/.config/slack-pp-cli/rocks.yml)")
	cmd.Flags().StringVar(&window, "window", "7d", "Discussion window (e.g. 7d, 14d)")
	cmd.Flags().BoolVar(&skipMissing, "skip-missing", false, "Degrade gracefully when the Asana mirror is absent (default behaviour)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/slack-pp-cli/data.db)")
	return cmd
}
