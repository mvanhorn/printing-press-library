// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
)

func parseCampaignObjectOrNull(flagName, value string) (any, error) {
	var parsed any
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return nil, fmt.Errorf("parsing --%s JSON: %w", flagName, err)
	}
	if parsed == nil {
		return nil, nil
	}
	if _, ok := parsed.(map[string]any); !ok {
		return nil, fmt.Errorf("--%s must be a JSON object or null, got JSON %T", flagName, parsed)
	}
	return parsed, nil
}
