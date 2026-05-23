package cli

import (
	"context"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/vault"
)

// vaultctx bundles the open vault and store. Returned by openVaultAndStore so
// commands don't need to know which package owns which lifecycle.
type vaultctx struct {
	V   *vault.Vault
	S   *store.Store
	Cfg *config.Config
}

// Close closes the store; the vault itself owns no resources.
func (vc *vaultctx) Close() error {
	if vc.S != nil {
		return vc.S.Close()
	}
	return nil
}

// openVaultAndStore resolves the vault path from the global config (env var
// OBSIDIAN_VAULT_PATH or config file), opens the vault, and opens the store.
//
// Returns a configErr (exit 10) when the vault path is unset, which is the
// canonical configuration failure for this CLI.
func openVaultAndStore(ctx context.Context, flags *rootFlags) (*vaultctx, error) {
	_ = ctx
	cfg, err := config.Load("")
	if err != nil {
		return nil, configErr(fmt.Errorf("load config: %w", err))
	}
	if cfg.VaultPath == "" {
		return nil, configErr(fmt.Errorf("vault path not set — export OBSIDIAN_VAULT_PATH=/path/to/vault or set vault_path in %s", cfg.Path))
	}
	v, err := vault.New(cfg.VaultPath)
	if err != nil {
		return nil, configErr(err)
	}
	s, err := store.Open(cfg.StorePath)
	if err != nil {
		return nil, configErr(fmt.Errorf("open store at %s: %w", cfg.StorePath, err))
	}
	return &vaultctx{V: v, S: s, Cfg: cfg}, nil
}

// openVaultOnly opens the vault without touching the store (for commands that
// don't need the index, e.g. lint --no-store).
func openVaultOnly() (*vault.Vault, *config.Config, error) {
	cfg, err := config.Load("")
	if err != nil {
		return nil, nil, configErr(err)
	}
	if cfg.VaultPath == "" {
		return nil, nil, configErr(fmt.Errorf("vault path not set — export OBSIDIAN_VAULT_PATH=/path/to/vault or set vault_path in %s", cfg.Path))
	}
	v, err := vault.New(cfg.VaultPath)
	if err != nil {
		return nil, nil, configErr(err)
	}
	return v, cfg, nil
}
