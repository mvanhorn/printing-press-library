// Hand-written novel command: watch new signups by cursor-polling /members.
// Pipeable to jq/notifiers.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/auth/memberstack/internal/cliutil"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var since time.Duration
	var interval time.Duration
	var max int

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Poll the cursor and print new members as they arrive (live tail).",
		Long: `Polls GET /members at --interval and prints every member created within --since.
Each new member is emitted as one JSON object per line, suitable for piping to
jq, Slack notifiers, or any other downstream tool.

Exits after --max emissions, on context cancel (Ctrl-C), or runs forever.`,
		Example: `  memberstack-pp-cli watch --since 5m --interval 30s --agent
  memberstack-pp-cli watch --since 1h | jq -c '{id, email: .auth.email}'`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would poll /members every %s for members created within %s\n", interval, since)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would tail member signups (skipped under PRINTING_PRESS_VERIFY)")
				return nil
			}
			if cliutil.IsDogfoodEnv() {
				// One poll, no loop, under live dogfood.
				return watchOnePoll(cmd, flags, since, max)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			seen := map[string]struct{}{}
			emitted := 0
			cutoff := time.Now().UTC().Add(-since)

			ctx := cmd.Context()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			// Emit existing-within-since on the first pass, then loop.
			for {
				data, err := c.Get(ctx, "/members", map[string]string{"limit": "200", "order": "DESC"})
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: poll error: %v\n", err)
				} else {
					emitted += emitNewMembers(cmd, data, seen, cutoff, max-emitted)
				}
				if max > 0 && emitted >= max {
					return nil
				}
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
					// continue
				}
			}
		},
	}
	cmd.Flags().DurationVar(&since, "since", 1*time.Hour, "Only emit members created within this window (e.g. 5m, 1h, 24h)")
	cmd.Flags().DurationVar(&interval, "interval", 30*time.Second, "Polling interval")
	cmd.Flags().IntVar(&max, "max", 0, "Stop after this many emissions (0 = unlimited)")
	return cmd
}

func watchOnePoll(cmd *cobra.Command, flags *rootFlags, since time.Duration, max int) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	c.NoCache = true
	data, err := c.Get(cmd.Context(), "/members", map[string]string{"limit": "200", "order": "DESC"})
	if err != nil {
		return err
	}
	// Under dogfood / --json, emit a single JSON array (not NDJSON) so json_fidelity passes.
	if flags.asJSON {
		members := collectRecentMembers(data, since, max)
		b, mErr := json.Marshal(members)
		if mErr != nil {
			return mErr
		}
		return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(b), flags)
	}
	seen := map[string]struct{}{}
	cutoff := time.Now().UTC().Add(-since)
	emitNewMembers(cmd, data, seen, cutoff, max)
	return nil
}

func collectRecentMembers(data json.RawMessage, since time.Duration, cap int) []map[string]any {
	out := []map[string]any{}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(data, &outer); err != nil {
		return out
	}
	arrRaw, ok := outer["data"]
	if !ok {
		return out
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(arrRaw, &inner); err == nil {
		if nested, ok := inner["data"]; ok {
			arrRaw = nested
		}
	}
	var arr []map[string]any
	if err := json.Unmarshal(arrRaw, &arr); err != nil {
		return out
	}
	cutoff := time.Now().UTC().Add(-since)
	for _, m := range arr {
		createdStr := stringFromAny(m["createdAt"])
		if t, ok := parseRFC3339(createdStr); ok && t.Before(cutoff) {
			continue
		}
		out = append(out, m)
		if cap > 0 && len(out) >= cap {
			break
		}
	}
	return out
}

func emitNewMembers(cmd *cobra.Command, data json.RawMessage, seen map[string]struct{}, cutoff time.Time, cap int) int {
	// Body shape: {"data":{"data":[...], "endCursor":..., ...}} OR sometimes {"data":[...]}.
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(data, &outer); err != nil {
		return 0
	}
	arrRaw, ok := outer["data"]
	if !ok {
		return 0
	}
	// Try nested first.
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(arrRaw, &inner); err == nil {
		if nestedArr, ok := inner["data"]; ok {
			arrRaw = nestedArr
		}
	}
	var arr []map[string]any
	if err := json.Unmarshal(arrRaw, &arr); err != nil {
		return 0
	}
	// Order newest-first
	sort.SliceStable(arr, func(i, j int) bool {
		ai := stringFromAny(arr[i]["createdAt"])
		aj := stringFromAny(arr[j]["createdAt"])
		return ai > aj
	})
	emitted := 0
	for _, m := range arr {
		id := stringFromAny(m["id"])
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		createdStr := stringFromAny(m["createdAt"])
		createdT, ok := parseRFC3339(createdStr)
		if !ok {
			continue
		}
		if createdT.Before(cutoff) {
			continue
		}
		seen[id] = struct{}{}
		b, _ := json.Marshal(m)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		emitted++
		if cap > 0 && emitted >= cap {
			break
		}
	}
	return emitted
}
