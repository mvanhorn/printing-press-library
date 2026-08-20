package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newXMLCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xml",
		Short: "Fetch the eForms XML payload for one publication (Basic-auth)",
		Long: strings.TrimSpace(`
Fetch the full eForms XML for a TenderNed publication. This is the only
endpoint that requires authentication; set TENDERNED_USERNAME and
TENDERNED_PASSWORD (request credentials via functioneelbeheer@tenderned.nl).
`),
	}
	cmd.AddCommand(newXMLFetchCmd(flags))
	return cmd
}

func newXMLFetchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fetch [publicatieId]",
		Short:   "Fetch eForms XML for one publication",
		Example: "  tenderned-pp-cli xml fetch 425283",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,1",
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
			body, err := tnPublicationXML(cmd.Context(), pubID)
			if err != nil {
				return err
			}
			if flags.asJSON {
				out, _ := json.Marshal(map[string]any{
					"publicatieId": pubID,
					"xml":          string(body),
					"bytes":        len(body),
				})
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			_, err = cmd.OutOrStdout().Write(body)
			return err
		},
	}
	return cmd
}
