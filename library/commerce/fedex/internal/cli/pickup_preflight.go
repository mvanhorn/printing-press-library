// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/workflow"
)

func preparePickupPreflight(fedexClient *client.Client, scheduleRequest map[string]any, availabilityJSON, overrideReason string) (workflow.PersistOptions, error) {
	availabilityJSON = strings.TrimSpace(availabilityJSON)
	var request map[string]any
	if availabilityJSON != "" {
		if err := json.Unmarshal([]byte(availabilityJSON), &request); err != nil {
			return workflow.PersistOptions{}, fmt.Errorf("parsing availability request JSON: %w", err)
		}
		if err := workflow.ValidatePickupAvailabilityBinding(scheduleRequest, request); err != nil {
			return workflow.PersistOptions{}, fmt.Errorf("pickup availability does not match scheduling request: %w", err)
		}
	}
	// Dry-run must not call FedEx. Binding and the override rule are still enforced.
	if fedexClient.DryRun {
		if len(request) > 0 && strings.TrimSpace(overrideReason) != "" {
			return workflow.PersistOptions{}, fmt.Errorf("use either an availability request or an override reason, not both")
		}
		if len(request) == 0 && strings.TrimSpace(overrideReason) == "" {
			return workflow.PersistOptions{}, fmt.Errorf("pickup scheduling requires an availability request or a documented override reason")
		}
		if strings.TrimSpace(overrideReason) != "" {
			if len(strings.TrimSpace(overrideReason)) < 10 {
				return workflow.PersistOptions{}, fmt.Errorf("availability override reason must contain at least 10 characters")
			}
			return workflow.PersistOptions{PickupPreflight: "overridden", PickupOverrideReason: strings.TrimSpace(overrideReason)}, nil
		}
		return workflow.PersistOptions{PickupPreflight: "verified"}, nil
	}
	preflight, err := workflow.PreparePickupPreflight(fedexClient, false, request, overrideReason)
	if err != nil {
		return workflow.PersistOptions{}, err
	}
	return workflow.PersistOptions{
		PickupPreflight:            preflight.Status,
		PickupOverrideReason:       preflight.OverrideReason,
		PickupPreflightCutoff:      preflight.CutoffTime,
		PickupPreflightAccessStart: preflight.AccessStart,
	}, nil
}
