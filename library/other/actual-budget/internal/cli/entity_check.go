// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/client"
)

// requireEntity confirms that the entity a sub-resource read hangs off exists.
// actual-http-api answers 200 with 0, "" or [] for unknown ids on endpoints
// such as account balance, notes and payee rules, which would otherwise pass
// as a real (empty) answer. A missing entity surfaces as the GET's 404.
func requireEntity(ctx context.Context, c *client.Client, flags *rootFlags, entityPath string, headers map[string]string) error {
	if flags.dryRun {
		return nil
	}
	_, err := c.GetWithHeaders(ctx, entityPath, nil, headers)
	return err
}
