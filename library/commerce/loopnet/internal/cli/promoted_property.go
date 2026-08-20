// LoopNet live property-detail command. Hand-written: replaces the generated
// generic HTML-page command with full RealEstateListing/Product JSON-LD plus
// facts-table extraction.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/loopnet/internal/loopnet"
)

func newPropertyPromotedCmd(flags *rootFlags) *cobra.Command {
	var useLocal bool

	cmd := &cobra.Command{
		Use:   "property <listing-id>",
		Short: "Fetch the full detail fact sheet for one LoopNet listing",
		Long: `Fetch one LoopNet listing's full detail record.

Accepts a numeric LoopNet listing id or a full /Listing/ URL. The record
combines the schema.org RealEstateListing/Product JSON-LD (price, broker,
address, description) with the facts table (cap rate, building class, year
built, zoning, parcel numbers, tax assessments). Pass --local to read a
previously synced copy instead of fetching live.`,
		Example: `  loopnet-pp-cli property 38523625
  loopnet-pp-cli property 38523625 --json --select name,price,cap_rate,total_assessment
  loopnet-pp-cli property https://www.loopnet.com/Listing/2035-W-15th-St/38523625/ --local`,
		Annotations: map[string]string{
			"pp:endpoint": "property.detail", "pp:method": "GET",
			"pp:path": "/Listing/{id}/", "mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			id := strings.TrimSpace(args[0])
			if strings.Contains(id, "loopnet.com") {
				if extracted := loopnet.ListingIDFromURL(id); extracted != "" {
					id = extracted
				}
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch: %s\n", loopnet.BuildDetailURL(id))
				return nil
			}
			if id == "" {
				return usageErr(fmt.Errorf("a LoopNet listing id (or /Listing/ URL) is required"))
			}

			if useLocal {
				st, err := lnOpenStore(flags)
				if err != nil {
					return err
				}
				defer st.Close()
				if p, ok := lnGetProperty(st, id); ok {
					return flags.printJSON(cmd, p)
				}
				return notFoundErr(fmt.Errorf("listing %s not in local store — run 'sync' or drop --local for a live fetch", id))
			}

			prop, err := lnFetchDetail(flags, id)
			if err != nil {
				return lnClassifyFetchError(fmt.Errorf("fetching listing %s: %w", id, err), flags)
			}
			return flags.printJSON(cmd, prop)
		},
	}

	cmd.Flags().BoolVar(&useLocal, "local", false, "Read a previously synced copy from the local store instead of fetching live")
	return cmd
}
