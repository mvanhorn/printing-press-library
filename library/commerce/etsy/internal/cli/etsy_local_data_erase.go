// Hand-written Etsy buyer erasure command; preserved across reprints.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local
func newEtsyLocalDataEraseBuyerCmd(flags *rootFlags) *cobra.Command {
	var buyers, emails, receipts, unsuppress []string
	var path string
	var vacuum, noVacuum bool
	cmd := &cobra.Command{Use: "erase-buyer", Short: "Erase a buyer's local data (dry run unless --yes)", Args: func(cmd *cobra.Command, args []string) error {
		pending := 0
		for _, id := range unsuppress {
			if id == "receipt" {
				pending++
			}
		}
		if len(args) > 0 && len(args) == pending {
			return nil
		}
		return cobra.NoArgs(cmd, args)
	}, Annotations: map[string]string{"mcp:hidden": "true", "pp:typed-exit-codes": "0,2", "pp:data-source": "local"}, Example: `  # Preview erasure by buyer id
  etsy-pp-cli local-data erase-buyer --buyer-user-id 1000000002

  # Preview erasure with an email from stdin
  echo buyer@example.invalid | etsy-pp-cli local-data erase-buyer --buyer-email -

  # Preview removing buyer suppression
  etsy-pp-cli local-data erase-buyer --unsuppress 1000000002`, RunE: func(cmd *cobra.Command, args []string) error {
		// etsy-ext: retain space-separated buyer ids alongside valueless receipt unsuppression.
		if len(args) > 0 {
			next := 0
			for i, id := range unsuppress {
				if id == "receipt" {
					unsuppress[i] = args[next]
					next++
				}
			}
		}
		// etsy-ext: a buyer-valued unsuppress cannot also select receipts.
		if len(receipts) > 0 {
			for _, id := range unsuppress {
				if id != "receipt" {
					return usageErr(fmt.Errorf("--unsuppress <buyer> cannot be combined with --receipt-id"))
				}
			}
		}
		if vacuum && noVacuum {
			return usageErr(fmt.Errorf("--vacuum and --no-vacuum are mutually exclusive"))
		}
		if cliutil.IsMCPChildEnv() {
			return usageErr(fmt.Errorf("local-data erase-buyer is unavailable over MCP"))
		}
		if len(buyers)+len(emails)+len(receipts)+len(unsuppress) == 0 {
			return usageErr(fmt.Errorf("provide a buyer, email, receipt, or --unsuppress selector"))
		}
		if len(unsuppress) > 0 && len(buyers)+len(emails) > 0 {
			return usageErr(fmt.Errorf("--unsuppress cannot be combined with erasure selectors"))
		}
		// etsy-ext: a valueless --unsuppress requires --receipt-id.
		if len(unsuppress) == 1 && unsuppress[0] == "receipt" {
			if len(receipts) == 0 {
				return usageErr(fmt.Errorf("--unsuppress requires a buyer id or --receipt-id"))
			}
			unsuppress = []string{"1"}
		}
		for _, list := range [][]string{buyers, receipts, unsuppress} {
			for _, id := range list {
				if err := store.EtsyValidateErasureID(id); err != nil {
					return usageErr(err)
				}
			}
		}
		var stdin []byte
		read := false
		for i, email := range emails {
			if email != "-" {
				fmt.Fprintln(cmd.ErrOrStderr(), "Warning: use --buyer-email - with stdin to keep email out of shell history")
			}
			if email == "-" {
				if !read {
					var err error
					stdin, err = io.ReadAll(io.LimitReader(cmd.InOrStdin(), 65537))
					if err != nil || len(stdin) > 65536 {
						return usageErr(fmt.Errorf("cannot read buyer email from stdin"))
					}
					read = true
				}
				emails[i] = strings.TrimSpace(string(stdin))
			}
			if strings.TrimSpace(emails[i]) == "" {
				return usageErr(fmt.Errorf("buyer email must not be empty"))
			}
		}
		var expanded []string
		for _, email := range emails {
			for _, line := range strings.Split(email, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					expanded = append(expanded, line)
				}
			}
		}
		emails = expanded
		o := &store.EtsyEraseOptions{BuyerIDs: buyers, Emails: emails, ReceiptIDs: receipts, Unsuppress: unsuppress}
		// etsy-ext: --unsuppress --receipt-id removes receipt suppression.
		if len(unsuppress) > 0 && len(receipts) > 0 {
			o.UnsuppressReceipts = receipts
			o.ReceiptIDs = nil
			o.Unsuppress = nil
		}
		// Etsy OAuth access tokens prefix the token owner's numeric user id. Never
		// print tokens; config has no separate persisted owner-id field.
		cfg, err := config.Load(flags.configPath)
		if err != nil {
			return err
		}
		o.ShopID = cfg.TemplateVars["shop_id"]
		for _, token := range []string{cfg.AccessToken, cfg.EtsyOauth2} {
			prefix, _, found := strings.Cut(token, ".")
			if found && store.EtsyValidateErasureID(prefix) == nil {
				o.OwnerIDs = append(o.OwnerIDs, prefix)
			}
		}
		return etsyRunSelective(cmd, flags, path, store.EtsyPruneOptions{Receipts: true, Erase: o, Vacuum: !noVacuum, ExplicitVacuum: vacuum}, "local-data erase-buyer", nil)
	}}
	cmd.Flags().StringArrayVar(&buyers, "buyer-user-id", nil, "Buyer user id (repeatable)")
	cmd.Flags().StringArrayVar(&emails, "buyer-email", nil, "Buyer email, or - for stdin (repeatable; never echoed)")
	cmd.Flags().StringArrayVar(&receipts, "receipt-id", nil, "Receipt id; erases and suppresses its whole buyer when known (repeatable); combine with --unsuppress to remove receipt suppression")
	cmd.Flags().StringArrayVar(&unsuppress, "unsuppress", nil, "Remove suppression for a buyer id (repeatable)")
	cmd.Flags().Lookup("unsuppress").NoOptDefVal = "receipt"
	cmd.Flags().StringVar(&path, "db", "", "Additional store file to process (DataDir stores are always processed)")
	cmd.Flags().BoolVar(&vacuum, "vacuum", false, "Accepted compatibility alias; erasure vacuums by default")
	cmd.Flags().BoolVar(&noVacuum, "no-vacuum", false, "Skip reclaiming SQLite free pages after erasure")
	return cmd
}
