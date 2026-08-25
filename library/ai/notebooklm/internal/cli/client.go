// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/config"
	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/nlm"
)

func newAPIClient(ctx context.Context, flags *rootFlags) (*nlm.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	if cfg.AuthHeader() == "" {
		return nil, authErr(fmt.Errorf("not authenticated — try: run notebooklm-pp-cli auth login --chrome, then run notebooklm-pp-cli doctor"))
	}
	hc, err := cfg.HTTPClient()
	if err != nil {
		return nil, configErr(err)
	}
	client, err := nlm.NewClient(ctx, hc)
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return client, nil
}
