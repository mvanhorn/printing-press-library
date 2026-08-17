// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Phase 3 foundation: structured Vagaro marketplace commands
// backed by the internal/vagaro sibling client. generate --force preserves
// implemented bodies.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/vagaro"
	"github.com/spf13/cobra"
)

// newVagaroClient builds the sibling client from the shared root flags so
// --timeout and --rate-limit apply the same way they do for generated
// endpoint commands.
func newVagaroClient(flags *rootFlags) *vagaro.Client {
	return vagaro.New(flags.timeout, flags.rateLimit)
}

// emitVagaro renders a Go value through the standard output pipeline
// (--json/--csv/--plain/--quiet/table, --select/--compact) with a live
// data-source meta tag.
func emitVagaro(cmd *cobra.Command, flags *rootFlags, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encoding output: %w", err)
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), json.RawMessage(data), flags, map[string]any{"source": "live"})
}

// classifyVagaroError maps sibling-client errors to structured exit codes.
// A rate-limit exhaustion becomes exit 7; everything else routes through the
// shared HTTP-status classifier.
func classifyVagaroError(err error, flags *rootFlags) error {
	var rl *cliutil.RateLimitError
	if errors.As(err, &rl) {
		return rateLimitErr(err)
	}
	return classifyAPIError(err, flags)
}

// resolveBusinessID resolves a slug to its businessID, preferring the local
// store cache when present and falling back to a live HTML fetch. The
// resolved id is written back to the store so repeat lookups skip the fetch.
func resolveBusinessID(ctx context.Context, c *vagaro.Client, flags *rootFlags, slug string) (string, error) {
	if id := cachedBusinessID(ctx, slug); id != "" {
		return id, nil
	}
	id, err := c.ResolveBusinessID(ctx, slug)
	if err != nil {
		return "", err
	}
	return id, nil
}
