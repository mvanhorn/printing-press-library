package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/goatstore"
)

func newNearCmd(flags *rootFlags) *cobra.Command {
	var (
		criteria string
		identity string
		minutes  int
	)
	cmd := &cobra.Command{
		Use:   "near <anchor>",
		Short: "Find the 3-5 amazing things within walking distance that match your stated identity and criteria — not the 40 closest things.",
		Long: `Persona-shaped local fanout. Geocodes the anchor (address or "lat,lng"),
then fans out to every source eligible for the location's country. Scores by
trust × (1 + country_match_boost) × intent_match × walking-time-decay against
the local SQLite store + live source results, returning 3-5 ranked picks
with local-language names preserved alongside transliterations.`,
		Example: strings.Trim(`
  # Persona-shaped 15-minute walk near a Tokyo hotel
  wanderlust-goat-pp-cli near "Park Hyatt Tokyo" \
    --criteria "vintage jazz kissaten, no tourists, great pour-over" \
    --identity "coffee snob, into 70s Japanese kissaten culture" \
    --minutes 15

  # By lat,lng with --agent for JSON output
  wanderlust-goat-pp-cli near 35.6895,139.6917 \
    --criteria "viewpoint, blue hour" --identity "photographer" \
    --minutes 20 --agent --select results.name,results.walking_min`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			anchor := strings.Join(args, " ")
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			res, err := resolveAnchor(ctx, anchor)
			if err != nil {
				return err
			}
			store, err := openGoatStore(cmd, flags)
			if err != nil {
				return err
			}
			defer store.Close()
			out := Fanout(ctx, res, criteria, identity, minutes, store)
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&criteria, "criteria", "", "Free-text criteria (e.g. \"vintage jazz kissaten, no tourists\").")
	cmd.Flags().StringVar(&identity, "identity", "", "Free-text identity (e.g. \"coffee snob, into 70s kissaten culture\") — boosts local-language sources.")
	cmd.Flags().IntVar(&minutes, "minutes", 15, "Walking-time radius in minutes (default 15, max 60).")
	return cmd
}

// openGoatStore opens the shared goatstore at the canonical path.
func openGoatStore(cmd *cobra.Command, flags *rootFlags) (*goatstore.Store, error) {
	path := goatstore.DefaultPath("wanderlust-goat-pp-cli")
	return goatstore.Open(cmd.Context(), path)
}

// renderFanoutJSON marshals a fanout result for printJSONFiltered.
// Kept as a method so future commands can share a common envelope.
func renderFanoutJSON(_ context.Context, out FanoutResult) ([]byte, error) {
	return json.Marshal(out)
}

// fanoutEnvelope wraps multi-command output for `--agent`-friendly shape.
// Currently unused outside near; kept here so other compound commands can
// share the same field names without redeclaring.
type fanoutEnvelope struct {
	Anchor   AnchorResolution `json:"anchor"`
	Criteria string           `json:"criteria,omitempty"`
	Minutes  int              `json:"minutes"`
	Picks    []Pick           `json:"results"`
	Notes    []string         `json:"notes,omitempty"`
}

func init() {
	// silence unused warnings if a refactor drops one of the helpers.
	_ = fmt.Sprintf
}
