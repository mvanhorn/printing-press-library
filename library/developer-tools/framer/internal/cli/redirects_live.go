package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/client"

	"github.com/spf13/cobra"
)

func newRedirectsLiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "redirects",
		Short: "Manage redirects on the live Framer project via the Server API bridge",
		Example: `  framer-pp-cli redirects list
  framer-pp-cli redirects list --json
  framer-pp-cli redirects add --from /old --to /new`,
	}

	cmd.AddCommand(newRedirectsLiveListCmd(flags))
	cmd.AddCommand(newRedirectsLiveAddCmd(flags))

	return cmd
}

func newRedirectsLiveListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List redirects from the live Framer project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list redirects from live Framer project")
				return nil
			}

			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}

			raw, err := bc.Call("redirects-list")
			if err != nil {
				return fmt.Errorf("redirects-list failed: %w", err)
			}

			var redirects []struct {
				From       string `json:"from"`
				To         string `json:"to"`
				StatusCode int    `json:"statusCode"`
			}
			if err := json.Unmarshal(raw, &redirects); err != nil {
				return fmt.Errorf("parsing redirects response: %w", err)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, redirects)
			}

			headers := []string{"FROM", "TO", "STATUS"}
			var rows [][]string
			for _, r := range redirects {
				status := fmt.Sprintf("%d", r.StatusCode)
				if r.StatusCode == 0 {
					status = "301"
				}
				rows = append(rows, []string{r.From, r.To, status})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	return cmd
}

func newRedirectsLiveAddCmd(flags *rootFlags) *cobra.Command {
	var from, to string
	var statusCode int

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a redirect to the live Framer project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if from == "" || to == "" {
				return usageErr(fmt.Errorf("--from and --to are required"))
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would add redirect: %s → %s (%d)\n", from, to, statusCode)
				return nil
			}

			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}

			payload := map[string]any{
				"from":       from,
				"to":         to,
				"statusCode": statusCode,
			}
			payloadJSON, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("marshalling redirect: %w", err)
			}

			raw, err := bc.Call("redirects-add", string(payloadJSON))
			if err != nil {
				return fmt.Errorf("redirects-add failed: %w", err)
			}

			if flags.asJSON {
				var result any
				_ = json.Unmarshal(raw, &result)
				return flags.printJSON(cmd, result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added redirect: %s → %s (%d)\n", from, to, statusCode)
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Source path (e.g. /old-page)")
	cmd.Flags().StringVar(&to, "to", "", "Destination path (e.g. /new-page)")
	cmd.Flags().IntVar(&statusCode, "status", 301, "HTTP status code (301 or 302)")

	return cmd
}
