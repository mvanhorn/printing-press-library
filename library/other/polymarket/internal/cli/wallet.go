// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Hand-written: wallet management for Polymarket EOA keys.

package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/polymarket/internal/config"
)

func newWalletCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wallet",
		Short: "Manage your Polygon EOA wallet (create, import, show, reset).",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWalletCreateCmd(flags))
	cmd.AddCommand(newWalletImportCmd(flags))
	cmd.AddCommand(newWalletShowCmd(flags))
	cmd.AddCommand(newWalletAddressCmd(flags))
	cmd.AddCommand(newWalletResetCmd(flags))
	return cmd
}

func newWalletCreateCmd(flags *rootFlags) *cobra.Command {
	var force, reveal bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Generate a new random Polygon EOA private key and save to config.",
		Example: `  polymarket-pp-cli wallet create
  polymarket-pp-cli wallet create --reveal --force`,
		Annotations: map[string]string{"pp:novel": "wallet.create"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if cfg.PolymarketPrivateKey != "" && !force {
				return usageErr(fmt.Errorf("wallet already configured (use --force to overwrite, or 'wallet show' to inspect)"))
			}
			// Generate 32 random bytes — this is the raw secp256k1 PK.
			// (Real EOA derivation requires secp256k1 — see honest note below.)
			pkBytes := make([]byte, 32)
			if _, err := rand.Read(pkBytes); err != nil {
				return apiErr(fmt.Errorf("generating random bytes: %w", err))
			}
			pkHex := "0x" + hex.EncodeToString(pkBytes)

			cfg.PolymarketPrivateKey = pkHex
			if err := saveWalletPK(cfg, pkHex); err != nil {
				return apiErr(err)
			}

			out := map[string]any{
				"created":     true,
				"config_path": cfg.Path,
				"address":     "<derive_requires_secp256k1>",
				"note":        "Address derivation from secp256k1 PK requires go-ethereum (not in this build). Use MetaMask or ethers.js to inspect the address; the PK is saved and ready for `auth derive`.",
			}
			if reveal {
				out["private_key"] = pkHex
			} else {
				out["private_key"] = "<hidden — use --reveal to print>"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing wallet without prompt")
	cmd.Flags().BoolVar(&reveal, "reveal", false, "Print the generated private key (default: hidden)")
	return cmd
}

func newWalletImportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "import <pk>",
		Short:       "Import an existing Polygon EOA private key (hex, 0x-prefixed).",
		Example:     `  polymarket-pp-cli wallet import 0x1234abcd...`,
		Annotations: map[string]string{"pp:novel": "wallet.import"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return usageErr(fmt.Errorf("private key required (hex, 0x-prefixed, 64 chars)"))
			}
			if dryRunOK(flags) {
				return nil
			}
			pk := strings.TrimSpace(args[0])
			if !validatePKHex(pk) {
				return usageErr(fmt.Errorf("invalid PK: expected 0x-prefixed 64-character hex string"))
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			cfg.PolymarketPrivateKey = pk
			if err := saveWalletPK(cfg, pk); err != nil {
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"imported":    true,
				"config_path": cfg.Path,
				"address":     "<derive_requires_secp256k1>",
				"note":        "PK saved to config. Run 'polymarket-pp-cli auth derive' to bootstrap L2 credentials.",
			}, flags)
		},
	}
	return cmd
}

func newWalletShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "show",
		Short:       "Show the configured wallet and auth-tier readiness (which env vars are set).",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			tier := "L0 (no auth)"
			if cfg.PolymarketPrivateKey != "" {
				tier = "L1 (private key set)"
			}
			if cfg.PolymarketApiKey != "" && cfg.PolymarketApiSecret != "" && cfg.PolymarketApiPassphrase != "" {
				tier = "L2 (HMAC API trio set)"
			}
			out := map[string]any{
				"config_path":          cfg.Path,
				"auth_source":          cfg.AuthSource,
				"auth_tier":            tier,
				"private_key_set":      cfg.PolymarketPrivateKey != "",
				"api_key_set":          cfg.PolymarketApiKey != "",
				"api_secret_set":       cfg.PolymarketApiSecret != "",
				"api_passphrase_set":   cfg.PolymarketApiPassphrase != "",
				"funder_set":           cfg.PolymarketFunder != "",
				"signature_type":       cfg.PolymarketSignatureType,
				"chain_id":             cfg.PolymarketChainId,
				"env_overrides_active": authEnvActive(),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

func newWalletAddressCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "address",
		Short:       "Print just the wallet address (derived from POLYMARKET_PRIVATE_KEY or config).",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if cfg.PolymarketPrivateKey == "" {
				return authErr(fmt.Errorf("no PK configured (run 'wallet create' or 'wallet import')"))
			}
			// Honest stub: cannot derive EOA address without secp256k1.
			out := map[string]any{
				"address": "<derive_requires_secp256k1>",
				"note":    "Address derivation requires the secp256k1 curve which Go stdlib does not provide. Run `go get github.com/ethereum/go-ethereum/crypto` and wire crypto.PubkeyToAddress(crypto.ToECDSA(pk).PublicKey) in v0.2. For now, derive the address with MetaMask or ethers.js.",
				"funder":  cfg.PolymarketFunder,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

func newWalletResetCmd(flags *rootFlags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:         "reset",
		Short:       "Remove the wallet PK from config (preserves other settings).",
		Example:     `  polymarket-pp-cli wallet reset --force`,
		Annotations: map[string]string{"pp:novel": "wallet.reset"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force && !flags.yes && !flags.dryRun {
				return usageErr(fmt.Errorf("destructive operation: pass --force or --yes to confirm"))
			}
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			cfg.PolymarketPrivateKey = ""
			if err := saveWalletPK(cfg, ""); err != nil {
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"reset":       true,
				"config_path": cfg.Path,
			}, flags)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Confirm destructive reset")
	return cmd
}

// -- helpers --

func validatePKHex(s string) bool {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		ok := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !ok {
			return false
		}
	}
	return true
}

func saveWalletPK(cfg *config.Config, pk string) error {
	// Reuse SaveTokens to persist via config's save() helper.
	// We pass through the existing OAuth fields (or zero) and set PK
	// separately via re-load + re-save.
	cfg.PolymarketPrivateKey = pk
	// Save via the package's exposed save() — use SaveTokens to trigger.
	if err := cfg.SaveTokens(cfg.ClientID, cfg.ClientSecret, cfg.AccessToken, cfg.RefreshToken, cfg.TokenExpiry); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	return nil
}

func authEnvActive() []string {
	envs := []string{
		"POLYMARKET_PRIVATE_KEY",
		"POLYMARKET_API_KEY",
		"POLYMARKET_API_SECRET",
		"POLYMARKET_API_PASSPHRASE",
		"POLYMARKET_FUNDER",
		"POLYMARKET_SIGNATURE_TYPE",
		"POLYMARKET_CHAIN_ID",
	}
	var active []string
	for _, e := range envs {
		if os.Getenv(e) != "" {
			active = append(active, e)
		}
	}
	return active
}
