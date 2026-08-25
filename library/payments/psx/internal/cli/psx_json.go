// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/psx"
)

// jsonUnmarshalStrict is a thin wrapper so novel feed code has one obvious
// decode path and callers do not each import encoding/json separately.
func jsonUnmarshalStrict(raw json.RawMessage, v any) error {
	return json.Unmarshal(raw, v)
}

// symbolIsListed reports whether a code appears in the instrument master. It
// exists so commands can tell "this symbol does not exist" (an error) apart
// from "this symbol has no history yet" (a legitimate empty result).
func symbolIsListed(ctx context.Context, c *psx.Client, sym string) (bool, error) {
	raw, err := c.Get(ctx, "/symbols")
	if err != nil {
		return false, err
	}
	var rows []struct {
		Symbol string `json:"symbol"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return false, err
	}
	for _, r := range rows {
		if strings.EqualFold(strings.TrimSpace(r.Symbol), sym) {
			return true, nil
		}
	}
	return false, nil
}
