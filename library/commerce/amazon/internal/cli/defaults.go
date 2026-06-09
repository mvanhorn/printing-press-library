package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// defaultsFile is the per-profile JSON blob storing manual config:
//
//	{ "card_last4": "1234", "card_label": "Visa", "address_label": "Home" }
//
// Lives at <ProfileDir>/defaults.json. Read by `cart show` and `checkout`
// to surface payment info Amazon's cart page doesn't reliably expose.
type profileDefaults struct {
	CardLast4    string `json:"card_last4,omitempty"`
	CardLabel    string `json:"card_label,omitempty"`
	AddressLabel string `json:"address_label,omitempty"`
}

func defaultsPath(profileDir string) string {
	return filepath.Join(profileDir, "defaults.json")
}

// LoadProfileDefaults returns the stored defaults, or an empty struct if the
// file doesn't exist. Errors only on read/parse failures of an existing file.
func LoadProfileDefaults(profileDir string) (profileDefaults, error) {
	var d profileDefaults
	b, err := os.ReadFile(defaultsPath(profileDir))
	if err != nil {
		if os.IsNotExist(err) {
			return d, nil
		}
		return d, err
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return d, fmt.Errorf("parse defaults.json: %w", err)
	}
	return d, nil
}

func saveProfileDefaults(profileDir string, d profileDefaults) error {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(defaultsPath(profileDir), b, 0o600)
}

func newDefaultsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "defaults",
		Short: "Read or write per-profile default payment / address labels",
		Long: `Stores manual config that Amazon's static cart parser can't reliably surface
— specifically the default card's last-4 digits and a human label. Read by
the cart and checkout flows so the agent can confirm "charging Visa ····1234"
before the user commits.`,
	}
	cmd.AddCommand(newDefaultsShowCmd())
	cmd.AddCommand(newDefaultsSetCmd())
	return cmd
}

func newDefaultsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "show",
		Short:       "Print the stored defaults for the active profile",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			d, err := LoadProfileDefaults(app.Cfg.ProfileDir(app.Profile.Name))
			if err != nil {
				return coded(ExitTransient, "%v", err)
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(d)
			}
			if d.CardLast4 == "" && d.AddressLabel == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "(no defaults set — run `defaults set --card-last4 XXXX [--card-label \"Visa\"] [--address-label \"Home\"]`)")
				return nil
			}
			if d.CardLabel != "" || d.CardLast4 != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Card:    %s ····%s\n", d.CardLabel, d.CardLast4)
			}
			if d.AddressLabel != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Address: %s\n", d.AddressLabel)
			}
			return nil
		},
	}
}

func newDefaultsSetCmd() *cobra.Command {
	var (
		cardLast4    string
		cardLabel    string
		addressLabel string
		clear        bool
	)
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set the per-profile defaults",
		Long:  "Update the stored default-card last-4 / label and default-address label. Pass --clear to wipe.",
		Example: "  amazon-pp-cli --profile personal defaults set --card-last4 1234 --card-label Visa\n" +
			"  amazon-pp-cli --profile work defaults set --card-last4 1234 --card-label Amex --address-label 'Office'",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			d, err := LoadProfileDefaults(app.Cfg.ProfileDir(app.Profile.Name))
			if err != nil {
				return coded(ExitTransient, "%v", err)
			}
			if clear {
				d = profileDefaults{}
			} else {
				if cardLast4 != "" {
					if len(cardLast4) != 4 {
						return coded(ExitUsage, "--card-last4 must be exactly 4 digits")
					}
					d.CardLast4 = cardLast4
				}
				if cardLabel != "" {
					d.CardLabel = cardLabel
				}
				if addressLabel != "" {
					d.AddressLabel = addressLabel
				}
			}
			if err := saveProfileDefaults(app.Cfg.ProfileDir(app.Profile.Name), d); err != nil {
				return coded(ExitTransient, "save: %v", err)
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(d)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved: card=%s ····%s, address=%s\n", d.CardLabel, d.CardLast4, d.AddressLabel)
			return nil
		},
	}
	cmd.Flags().StringVar(&cardLast4, "card-last4", "", "Last 4 digits of the default card")
	cmd.Flags().StringVar(&cardLabel, "card-label", "", "Human label, e.g. 'Visa' or 'Personal Amex'")
	cmd.Flags().StringVar(&addressLabel, "address-label", "", "Human label for the default ship-to, e.g. 'Home' or 'Office'")
	cmd.Flags().BoolVar(&clear, "clear", false, "Wipe all defaults")
	return cmd
}
