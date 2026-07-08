package cli

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
	sfmock "github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/testdata/salesforce-mock"

	"github.com/spf13/cobra"
)

func newTrustCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Manage bundle signing keys",
		Long: `Generate, register, list, and retire bundle-signing keys.

Bundles produced by 'agent context' are signed by an Ed25519 keypair per
(org, host, user). The private key lives on disk at
~/.config/pp/salesforce-headless-360/keys/<org>/<host>_<user>/private.pem with 0600 perms.
The public key is registered with the Salesforce org as a Certificate
record (preferred) or a Custom Metadata record (fallback). Verifiers look
up the public key by the JWS 'kid' header.

Register once per device per org before generating bundles.`,
		Example: `  # First-time setup
  salesforce-headless-360-pp-cli trust register --org prod

  # See registered keys
  salesforce-headless-360-pp-cli trust list-keys --json

  # Rotate to a new keypair
  salesforce-headless-360-pp-cli trust rotate --org prod

  # Retire a compromised key
  salesforce-headless-360-pp-cli trust revoke-key <kid>`,
	}
	cmd.AddCommand(newTrustRegisterCmd(flags))
	cmd.AddCommand(newTrustRotateCmd(flags))
	cmd.AddCommand(newTrustListKeysCmd(flags))
	cmd.AddCommand(newTrustRevokeKeyCmd(flags))
	cmd.AddCommand(newTrustInstallApexCmd(flags))
	return cmd
}

func newTrustRegisterCmd(flags *rootFlags) *cobra.Command {
	var orgAlias string
	var useMock bool
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Generate a local keypair and register the public key with the org",
		Long: `Generates an Ed25519 keypair if one does not exist for this org+host+user,
then registers the public key with the org as a Salesforce Certificate.
If Certificate is unavailable for the org edition, falls back to a
SF360_Bundle_Key__mdt record with a hash-chained possession receipt.`,
		Example: `  salesforce-headless-360-pp-cli trust register --org prod
  salesforce-headless-360-pp-cli trust register --mock --org prod`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if orgAlias == "" {
				return fmt.Errorf("--org is required")
			}
			cleanup, err := configureTrustMock(useMock)
			if err != nil {
				return err
			}
			defer cleanup()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			result, err := trust.RegisterKey(c, trust.RegisterOptions{OrgAlias: orgAlias})
			if err != nil {
				return err
			}
			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, result)
			}
			if result.Idempotent {
				fmt.Fprintf(cmd.OutOrStdout(), "key already registered for org=%s\n  kid:    %s\n  source: %s\n", orgAlias, result.KID, result.Source)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"registered key for org=%s\n  kid:    %s\n  source: %s\n",
				orgAlias, result.KID, result.Source,
			)
			return nil
		},
	}
	cmd.Flags().StringVar(&orgAlias, "org", "", "Org alias (required)")
	cmd.Flags().BoolVar(&useMock, "mock", false, "Run against the in-process Salesforce mock server.")
	_ = cmd.MarkFlagRequired("org")
	return cmd
}

func newTrustRotateCmd(flags *rootFlags) *cobra.Command {
	var orgAlias string
	var useMock bool
	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "Retire the current key and generate a new one",
		Long: `Marks the current key for this org retired and generates a fresh
keypair. Bundles signed by the retired key continue to verify (against
the cached public key) until their 'exp' claim lapses.`,
		Example: `  salesforce-headless-360-pp-cli trust rotate --org prod --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if orgAlias == "" {
				return fmt.Errorf("--org is required")
			}
			if err := confirmTrustAction(cmd, flags, "rotate active key for org "+orgAlias); err != nil {
				return err
			}
			cleanup, err := configureTrustMock(useMock)
			if err != nil {
				return err
			}
			defer cleanup()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			result, err := trust.RotateKey(c, trust.RotateOptions{OrgAlias: orgAlias})
			if err != nil {
				return err
			}
			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "rotated key for org=%s\n  old_kid: %s\n  new_kid: %s\n", orgAlias, result.OldKID, result.New.KID)
			return nil
		},
	}
	cmd.Flags().StringVar(&orgAlias, "org", "", "Org alias (required)")
	cmd.Flags().BoolVar(&useMock, "mock", false, "Run against the in-process Salesforce mock server.")
	_ = cmd.MarkFlagRequired("org")
	return cmd
}

func newTrustListKeysCmd(flags *rootFlags) *cobra.Command {
	var orgAlias string
	var live bool
	var useMock bool
	cmd := &cobra.Command{
		Use:   "list-keys",
		Short: "List all registered keys in the local keystore",
		Example: `  salesforce-headless-360-pp-cli trust list-keys
  salesforce-headless-360-pp-cli trust list-keys --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var orgClient trust.OrgClient
			if live {
				cleanup, err := configureTrustMock(useMock)
				if err != nil {
					return err
				}
				defer cleanup()
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				orgClient = c
			}
			keyRows, err := trust.ListKeys(orgClient, trust.KeyListOptions{OrgAlias: orgAlias, Live: live})
			if err != nil {
				return err
			}
			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, keyRows)
			}
			if len(keyRows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no keys registered. run 'trust register --org <alias>' first.")
				return nil
			}
			headers := []string{"KID", "ORG", "ALGORITHM", "STATUS", "REGISTERED", "LAST_USED"}
			var tableRows [][]string
			for _, r := range keyRows {
				lastUsed := ""
				if r.LastUsedAt != nil {
					lastUsed = r.LastUsedAt.Format(time.RFC3339)
				}
				tableRows = append(tableRows, []string{r.KID, r.OrgAlias, r.Algorithm, r.Status, r.RegisteredAt.Format(time.RFC3339), lastUsed})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
	cmd.Flags().StringVar(&orgAlias, "org", "", "Filter by org alias")
	cmd.Flags().BoolVar(&live, "live", false, "Merge local keys with keys queried from the org")
	cmd.Flags().BoolVar(&useMock, "mock", false, "Run live lookup against the in-process Salesforce mock server.")
	return cmd
}

func newTrustRevokeKeyCmd(flags *rootFlags) *cobra.Command {
	var kid string
	var reason string
	var useMock bool
	cmd := &cobra.Command{
		Use:     "revoke-key [kid]",
		Short:   "Mark a registered key as retired",
		Args:    cobra.MaximumNArgs(1),
		Example: `  salesforce-headless-360-pp-cli trust revoke-key --kid 4tK9abc... --reason "compromised laptop" --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if kid == "" && len(args) == 1 {
				kid = args[0]
			}
			if kid == "" {
				return fmt.Errorf("--kid is required")
			}
			if reason == "" {
				return fmt.Errorf("--reason is required")
			}
			if err := confirmTrustAction(cmd, flags, "revoke key "+kid); err != nil {
				return err
			}
			cleanup, err := configureTrustMock(useMock)
			if err != nil {
				return err
			}
			defer cleanup()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			result, err := trust.RevokeKey(c, trust.RevokeOptions{KID: kid, Reason: reason})
			if err != nil {
				return fmt.Errorf("revoke key %s: %w", kid, err)
			}
			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "retired key %s\n", kid)
			return nil
		},
	}
	cmd.Flags().StringVar(&kid, "kid", "", "KID to revoke")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason to store with the retirement")
	cmd.Flags().BoolVar(&useMock, "mock", false, "Run against the in-process Salesforce mock server.")
	return cmd
}

func configureTrustMock(useMock bool) (func(), error) {
	if !useMock {
		return func() {}, nil
	}
	mockServer, err := sfmock.StartBackground()
	if err != nil {
		return nil, fmt.Errorf("starting mock Salesforce server: %w", err)
	}
	restore := []func(){
		doctorSetenv("SALESFORCE_INSTANCE_URL", mockServer.URL),
		doctorSetenv("SALESFORCE_HEADLESS_360_BASE_URL", mockServer.URL),
		doctorSetenv("SALESFORCE_ACCESS_TOKEN", "mock-access-token"),
	}
	return func() {
		for i := len(restore) - 1; i >= 0; i-- {
			restore[i]()
		}
		mockServer.Close()
	}, nil
}

func confirmTrustAction(cmd *cobra.Command, flags *rootFlags, action string) error {
	if flags.yes || flags.agent {
		return nil
	}
	if flags.noInput {
		return fmt.Errorf("%s requires confirmation; pass --yes", action)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s? type yes to continue: ", action)
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(strings.ToLower(line)) != "yes" {
		return fmt.Errorf("cancelled")
	}
	return nil
}
