// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: RSA keypair generation for SNAP onboarding.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/snap"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelSnapKeygenCmd(flags *rootFlags) *cobra.Command {
	var outDir string
	var force bool

	cmd := &cobra.Command{
		Use:   "keygen",
		Short: "Generate the RSA-2048 keypair SNAP onboarding requires (PEM, dashboard-ready)",
		Long: strings.Trim(`
Generates an RSA-2048 private/public key pair in PEM format. Upload the public
key in Dashboard > Settings > API Keys (separate keys for sandbox and live),
then point DURIANPAY_SNAP_PRIVATE_KEY at the private key file.
`, "\n"),
		Example: strings.Trim(`
  durianpay-pp-cli snap keygen --out ./keys --force
  durianpay-pp-cli snap keygen --out ./keys --force --json
`, "\n"),
		Annotations: map[string]string{
			"pp:happy-args": "--out=/tmp/durianpay-keys-example;--force",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would generate an RSA-2048 keypair under", outDir)
				return nil
			}
			if outDir == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--out directory is required"))
			}
			privPath := filepath.Join(outDir, "rsa_private_key.pem")
			pubPath := filepath.Join(outDir, "rsa_public_key.pem")
			if !force {
				if _, err := os.Stat(privPath); err == nil {
					return usageErr(fmt.Errorf("%s already exists; use --force to overwrite", privPath))
				}
			}
			priv, pub, err := snap.GenerateKeypair()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(outDir, 0o700); err != nil {
				return fmt.Errorf("creating %s: %w", outDir, err)
			}
			if err := os.WriteFile(privPath, []byte(priv), 0o600); err != nil {
				return fmt.Errorf("writing private key: %w", err)
			}
			if err := os.WriteFile(pubPath, []byte(pub), 0o600); err != nil {
				return fmt.Errorf("writing public key: %w", err)
			}
			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, map[string]any{
					"private_key_path": privPath,
					"public_key_path":  pubPath,
					"next_steps": []string{
						"Upload " + pubPath + " in Dashboard > Settings > API Keys",
						"export DURIANPAY_SNAP_PRIVATE_KEY=" + privPath,
					},
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Private key: %s (0600 — keep it secret)\nPublic key:  %s\n\n%s\nNext steps:\n  1. Upload the public key in Dashboard > Settings > API Keys\n  2. export DURIANPAY_SNAP_PRIVATE_KEY=%s\n", privPath, pubPath, pub, privPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&outDir, "out", "", "Directory to write rsa_private_key.pem and rsa_public_key.pem")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing key files")
	return cmd
}
