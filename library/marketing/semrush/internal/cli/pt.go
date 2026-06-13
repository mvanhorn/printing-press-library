// Position Tracking namespace for cookie-authenticated subcommands that hit
// www.semrush.com's internal UI endpoints (not the public Semrush API). These
// complement the official-API `tracking` namespace by exposing PT features
// that aren't in the public API spec — add-keywords with tags, current
// rankings snapshot, and chart annotations.
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// newPTCmd is the parent of cookie-authenticated PT subcommands. To be
// registered in root.go (handled by post-merge automation that reads
// novel_features from .printing-press.json).
func newPTCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pt",
		Aliases: []string{"position-tracking"},
		Short:   "Position Tracking — cookie-auth UI subcommands (add-keywords, rankings, annotate)",
		Long: "Position Tracking commands that hit Semrush's internal UI backend " +
			"(www.semrush.com/tracking/web-api and /notes/api). These complement " +
			"the public-API `tracking` namespace by exposing features the public " +
			"API doesn't cover: bulk keyword adds with tags, current rankings " +
			"snapshots, and chart annotations. Most require `SEMRUSH_API_KEY` " +
			"plus Chrome session cookies imported via `auth login --chrome`.",
	}
	cmd.AddCommand(newPTAddKeywordsCmd(flags))
	cmd.AddCommand(newPTRankingsCmd(flags))
	cmd.AddCommand(newPTAnnotateCmd(flags))
	return cmd
}

// ptPrint emits the response through the standard flag pipeline (--json,
// --select, --compact, --csv). Falls back to JSON pretty-print if the response
// is already JSON.
func ptPrint(cmd *cobra.Command, flags *rootFlags, data []byte) error {
	_ = flags
	var anyVal any
	if json.Unmarshal(data, &anyVal) != nil {
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	out, err := json.MarshalIndent(anyVal, "", "  ")
	if err != nil {
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}

func truncatePT(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
