package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newTEDLinkCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var noCache bool
	cmd := &cobra.Command{
		Use:   "ted-link [publicatieId]",
		Short: "Extract the canonical TED (OJ-S) publication number from a TenderNed notice's eForms XML",
		Long: `Bridges TenderNed publication IDs to EU TED (Tenders Electronic Daily) publication numbers
(format NNNNNN-YYYY). Use the returned number with the sibling eu-tenders CLI:

  eu-tenders notices --query "publication-number=$(tenderned-pp-cli ted-link 425283 -q)"

Requires TENDERNED_USERNAME and TENDERNED_PASSWORD (the XML endpoint uses Basic auth).

Caches the TED number per publicatieId in the local SQLite store so repeated lookups
don't re-hit the authenticated XML endpoint. Pass --no-cache to force a live fetch.`,
		Example: "  tenderned-pp-cli ted-link 425283",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			pubID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("publicatieId must be an integer (got %q)", args[0])
			}

			// Cache: SQLite-backed lookup by publicatieId.
			s, openErr := tnOpenStore(cmd.Context(), dbPath)
			var ted string
			if openErr == nil {
				defer s.Close()
				if !noCache {
					ted = tnTEDCacheLookup(cmd.Context(), s, pubID)
				}
			}

			if ted == "" {
				xml, err := tnPublicationXML(cmd.Context(), pubID)
				if err != nil {
					return err
				}
				ted = tnExtractTEDPublicationNumber(xml)
				if ted == "" {
					return fmt.Errorf("no TED publication number found in eForms XML for publication %d", pubID)
				}
				if s != nil {
					tnTEDCacheStore(cmd.Context(), s, pubID, ted)
				}
			}

			result := map[string]any{
				"publicatieId":         pubID,
				"tedPublicationNumber": ted,
				"tedURL":               "https://ted.europa.eu/nl/notice/" + ted + "/html",
				"euTendersQuery":       "publication-number=" + ted,
			}
			if flags.quiet {
				fmt.Fprintln(cmd.OutOrStdout(), ted)
				return nil
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "TenderNed publication %d → TED %s\n  %s\n", pubID, ted, result["tedURL"])
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Skip the local cache lookup and fetch live XML")
	return cmd
}
