// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `sort suggest`: mine the labeling you already do by hand.
// Senders whose labeled messages consistently carry the same user label
// become suggestions, each with a ready 'cleanup plan' invocation to label
// the rest the same way. Suggestions only — nothing is ever applied here.
// Pure local read.

package cli

import (
	"fmt"
	"math"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// sortSuggestRow is one labeling suggestion.
type sortSuggestRow struct {
	Sender         string  `json:"sender"`
	Label          string  `json:"label"`
	Confidence     float64 `json:"confidence"`
	LabeledCount   int     `json:"labeled_count"`
	UnlabeledCount int     `json:"unlabeled_count"`
	PlanInvocation string  `json:"plan_invocation"`
}

// buildSortSuggestions turns per-(sender,label) stats into suggestion rows:
// per sender the dominant user label wins when its share of the sender's
// LABELED messages reaches minConfidence and the sender has at least
// minLabeled labeled messages. Confidence is rounded to 4 decimals for
// stable JSON.
func buildSortSuggestions(account string, stats []store.SenderLabelStat, minConfidence float64, minLabeled int) []sortSuggestRow {
	type best struct {
		label        string
		count        int
		labeledTotal int
		senderTotal  int
	}
	bySender := map[string]best{}
	var order []string
	for _, st := range stats {
		b, seen := bySender[st.FromEmail]
		if !seen {
			order = append(order, st.FromEmail)
		}
		if st.LabelCount > b.count || (st.LabelCount == b.count && (b.label == "" || st.Label < b.label)) {
			bySender[st.FromEmail] = best{label: st.Label, count: st.LabelCount, labeledTotal: st.LabeledTotal, senderTotal: st.SenderTotal}
		}
	}
	var out []sortSuggestRow
	for _, sender := range order {
		b := bySender[sender]
		if b.labeledTotal < minLabeled || b.labeledTotal == 0 {
			continue
		}
		conf := float64(b.count) / float64(b.labeledTotal)
		if conf < minConfidence {
			continue
		}
		out = append(out, sortSuggestRow{
			Sender:         sender,
			Label:          b.label,
			Confidence:     math.Round(conf*10000) / 10000,
			LabeledCount:   b.labeledTotal,
			UnlabeledCount: b.senderTotal - b.labeledTotal,
			PlanInvocation: fmt.Sprintf("cleanup plan --account %s --q \"from:%s -label:%s\" --action label --add %s", account, sender, b.label, b.label),
		})
	}
	return out
}

func newNovelSortSuggestCmd(flags *rootFlags) *cobra.Command {
	var minConfidence float64
	var minLabeled int

	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "Senders whose mail you already label consistently, each with a ready 'cleanup plan' invocation to label the rest the same way",
		Long: `Find the sorting rules you already follow by hand.

For every sender, look at the messages you have labeled (any non-system
label; INBOX/UNREAD/CATEGORY_*/TRASH/SPAM/SENT/DRAFT/IMPORTANT/STARRED are
ignored). When at least --min-labeled of a sender's messages carry user
labels and at least --min-confidence of those share ONE label, emit a
suggestion: the sender, the label, the confidence, how many of the
sender's messages are still unlabeled, and a ready-to-run
'cleanup plan ... --action label' invocation that would label them the
same way — through the full preview-confirm engine, never directly.

Labels are the store's label references (user label IDs like Label_7);
'cleanup plan --add' accepts IDs directly. The -label: operator in the
generated query needs the label NAME in Gmail search — check 'labels list'
if your label IDs and names differ.

Reads only the local store — run 'sync' first.`,
		Example: `  # What sorting do I already do?
  gmail-pp-cli sort suggest --account personal

  # Stricter: 95% agreement over at least 10 labeled messages
  gmail-pp-cli sort suggest --account personal --min-confidence 0.95 --min-labeled 10 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := resolveGauthAccount(flags)
			if err != nil {
				return err
			}
			if minConfidence <= 0 || minConfidence > 1 {
				return usageErr(fmt.Errorf("--min-confidence must be in (0, 1], got %v", minConfidence))
			}
			if minLabeled <= 0 {
				return usageErr(fmt.Errorf("--min-labeled must be positive, got %d", minLabeled))
			}
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			stats, err := db.SenderLabelStats(account)
			if err != nil {
				return fmt.Errorf("aggregating sender labels: %w", err)
			}
			rows := buildSortSuggestions(account, stats, minConfidence, minLabeled)
			if rows == nil {
				rows = []sortSuggestRow{}
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no consistent labeling found for account %q (thresholds: confidence >= %v over >= %d labeled messages)\n",
					account, minConfidence, minLabeled)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().Float64Var(&minConfidence, "min-confidence", 0.8, "Minimum share of a sender's labeled messages that must agree on one label")
	cmd.Flags().IntVar(&minLabeled, "min-labeled", 5, "Minimum labeled messages a sender needs before a suggestion is made")
	return cmd
}
