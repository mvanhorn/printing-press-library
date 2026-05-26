package cli

import (
	"errors"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/categorize"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/output"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/store"
)

func newRabbitholesCmd(opts *RootOptions) *cobra.Command {
	var since, gap string
	cmd := &cobra.Command{
		Use:         "rabbitholes",
		Short:       "Find sessions that started productive and drifted into distracting pages over --since (not available for Safari, which omits navigation transition types from History.db)",
		Example:     strings.Trim("safari-history-pp-cli rabbitholes --since 30d", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Drift detection keys off the productive→distracting transition type;
			// Safari's History.db has no transition data, so without this gate the
			// command would always return zero sessions — indistinguishable from a
			// genuinely quiet period. Report unavailable honestly instead.
			if !opts.Source.Capabilities().Transitions {
				return renderNotAvailable(opts, "rabbitholes", "Safari does not record navigation transition types in History.db")
			}
			start, end, err := sourceTimeWindow(since, "", 30*24*time.Hour)
			if err != nil {
				return errors.Join(ErrUsage, err)
			}
			gd, err := time.ParseDuration(gap)
			if err != nil {
				return errors.Join(ErrUsage, err)
			}
			snapshot, err := snapshotPath()
			if err != nil {
				return err
			}
			st, err := store.OpenExisting(snapshot)
			if err != nil {
				if errors.Is(err, store.ErrNoSnapshot) {
					return ErrNoSnapshot
				}
				return err
			}
			defer st.Close()
			events, err := opts.Source.RecentVisits(st.DB(), source.VisitFilter{Since: start, Until: end, Limit: 10000, Device: opts.Device})
			if err != nil {
				return err
			}
			sessions := splitSessions(events, gd)
			out := []map[string]any{}
			hours := map[int]int64{}
			for i, s := range sessions {
				if len(s.Items) == 0 {
					continue
				}
				first := s.Items[0]
				if first.Transition != "typed" {
					continue
				}
				fd := source.DomainFromURL(first.URL)
				_, fp := categorize.Classify(fd)
				if fp != "productive" {
					continue
				}
				drift := false
				for _, e := range s.Items[1:] {
					if e.Transition != "link" {
						continue
					}
					_, p := categorize.Classify(source.DomainFromURL(e.URL))
					if p == "distracting" {
						drift = true
						break
					}
				}
				if drift {
					h := s.Start.In(time.Local).Hour()
					hours[h]++
					out = append(out, map[string]any{"session": i + 1, "start": s.Start.Format(time.RFC3339), "end": s.End.Format(time.RFC3339), "visits": len(s.Items)})
				}
			}
			patterns := []map[string]any{}
			for h, c := range hours {
				patterns = append(patterns, map[string]any{"hour": h, "drift_sessions": c})
			}
			result := map[string]any{"sessions": out, "time_of_day_patterns": patterns}
			maybePrintEmptyWindowHint(st.DB(), since, len(out) == 0)
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, result)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "start time")
	cmd.Flags().StringVar(&gap, "gap", "30m", "session gap")
	return cmd
}
