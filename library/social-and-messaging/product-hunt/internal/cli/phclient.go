// Package cli - Product Hunt GraphQL client helpers.
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/product-hunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/product-hunt/internal/phgraphql"
)

// newPHClient creates a phgraphql.Client from rootFlags config.
func (f *rootFlags) newPHClient() (*phgraphql.Client, error) {
	cfg, err := config.Load(f.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	token := cfg.ProductHuntToken
	if token == "" && cfg.AccessToken != "" {
		token = cfg.AccessToken
	}
	if f.dryRun {
		return phgraphql.NewDryRun(token), nil
	}
	return phgraphql.New(token, f.timeout, f.rateLimit), nil
}
