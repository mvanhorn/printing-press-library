// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `delta`: everything new since the last checkpoint. The
// checkpoint (mail_checkpoints, kind 'delta') is a history-ish watermark —
// the max internal_date plus row count the store held at the last run.
// Reporting reads the local store only; the checkpoint ADVANCES after a
// successful report unless --peek. First run sets the baseline and says so
// honestly.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// deltaSpikeFactor: a sender "spikes" when its since-checkpoint count
// exceeds this multiple of its prior daily average.
const deltaSpikeFactor = 3.0

// deltaSenderRow is one sender's new-message count.
type deltaSenderRow struct {
	FromEmail string `json:"from_email"`
	Count     int    `json:"count"`
}

// deltaSpikeRow is one volume spike.
type deltaSpikeRow struct {
	FromEmail      string  `json:"from_email"`
	SinceCount     int     `json:"since_count"`
	PriorDailyAvg  float64 `json:"prior_daily_avg"`
	SpikeThreshold float64 `json:"spike_threshold"`
}

// deltaOutput is the delta JSON envelope.
type deltaOutput struct {
	Account      string           `json:"account"`
	BaselineSet  bool             `json:"baseline_set"`
	CheckpointAt string           `json:"checkpoint_at,omitempty"`
	WatermarkMs  int64            `json:"watermark_ms,omitempty"`
	NewTotal     int              `json:"new_total"`
	PerCategory  map[string]int   `json:"per_category"`
	PerSender    []deltaSenderRow `json:"per_sender"`
	NewSenders   []string         `json:"new_senders"`
	Spikes       []deltaSpikeRow  `json:"spikes"`
	Advanced     bool             `json:"advanced"`
	NewWatermark int64            `json:"new_watermark_ms,omitempty"`
	Note         string           `json:"note,omitempty"`
}

func newNovelDeltaCmd(flags *rootFlags) *cobra.Command {
	var peek bool

	cmd := &cobra.Command{
		Use:   "delta",
		Short: "Everything new since your last check: new messages per category and sender, never-seen senders, volume spikes — then the checkpoint advances",
		Long: `Report what changed since the last delta checkpoint, then advance it.

The checkpoint is a local watermark (max internal_date + message count at
the last run). Against it this reports: new messages per category and per
sender, senders never seen before the checkpoint, and volume spikes — any
sender whose since-checkpoint count exceeds 3x its prior daily average
(prior count divided by the days between its first appearance and the
checkpoint).

The very first run has nothing to diff against: it sets the baseline,
says so honestly, and reports no changes. --peek reports without advancing
the checkpoint (including not setting a first baseline).

Reads only the local store — run 'sync' first so "new" means new.`,
		Example: `  # What's new since I last looked?
  gmail-pp-cli delta --account personal

  # Look without consuming the delta
  gmail-pp-cli delta --account personal --peek --agent`,
		Annotations: map[string]string{
			"mcp:read-only":   "true",
			"mcp:local-write": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := resolveGauthAccount(flags)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			maxMs, count, err := db.MailWatermark(account)
			if err != nil {
				return fmt.Errorf("reading store watermark: %w", err)
			}
			cp, exists, err := db.GetMailCheckpoint(account, "delta")
			if err != nil {
				return fmt.Errorf("reading checkpoint: %w", err)
			}

			out := deltaOutput{Account: account, PerCategory: map[string]int{}, PerSender: []deltaSenderRow{}, NewSenders: []string{}, Spikes: []deltaSpikeRow{}}
			if !exists {
				out.BaselineSet = true
				out.Note = "first run: baseline set — nothing to diff against yet; the next run reports changes since now"
				if peek {
					out.Advanced = false
					out.Note = "first run with --peek: no baseline exists and none was set; run without --peek to set one"
				} else {
					if err := db.SaveMailCheckpoint(store.MailCheckpoint{Account: account, Kind: "delta", WatermarkMs: maxMs, MsgCount: count}); err != nil {
						return fmt.Errorf("setting baseline checkpoint: %w", err)
					}
					out.Advanced = true
					out.NewWatermark = maxMs
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			out.CheckpointAt = cp.TakenAt
			out.WatermarkMs = cp.WatermarkMs
			cats, err := db.DeltaCategoryCounts(account, cp.WatermarkMs)
			if err != nil {
				return fmt.Errorf("counting new messages per category: %w", err)
			}
			out.PerCategory = cats
			stats, err := db.DeltaSenderStats(account, cp.WatermarkMs)
			if err != nil {
				return fmt.Errorf("aggregating new messages per sender: %w", err)
			}
			for _, st := range stats {
				out.NewTotal += st.SinceCount
				if len(out.PerSender) < 25 {
					out.PerSender = append(out.PerSender, deltaSenderRow{FromEmail: st.FromEmail, Count: st.SinceCount})
				}
				if st.PriorCount == 0 {
					out.NewSenders = append(out.NewSenders, st.FromEmail)
					continue
				}
				days := float64(cp.WatermarkMs-st.FirstSeenMs) / float64(24*60*60*1000)
				if days < 1 {
					days = 1
				}
				avg := float64(st.PriorCount) / days
				if float64(st.SinceCount) > deltaSpikeFactor*avg {
					out.Spikes = append(out.Spikes, deltaSpikeRow{
						FromEmail:      st.FromEmail,
						SinceCount:     st.SinceCount,
						PriorDailyAvg:  avg,
						SpikeThreshold: deltaSpikeFactor * avg,
					})
				}
			}
			sort.Strings(out.NewSenders)

			if peek {
				out.Note = "--peek: checkpoint not advanced; the next delta reports these changes again"
			} else {
				if err := db.SaveMailCheckpoint(store.MailCheckpoint{Account: account, Kind: "delta", WatermarkMs: maxMs, MsgCount: count}); err != nil {
					return fmt.Errorf("advancing checkpoint: %w", err)
				}
				out.Advanced = true
				out.NewWatermark = maxMs
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&peek, "peek", false, "Report without advancing the checkpoint (the next delta sees the same changes)")
	return cmd
}
