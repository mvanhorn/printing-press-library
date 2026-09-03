// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/approval"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/workflow"
)

func preparePickupPreflight(flags *rootFlags, fedexClient *client.Client, scheduleRequest map[string]any, availabilityJSON, overrideReason string) (protectedMutationOptions, error) {
	availabilityJSON = strings.TrimSpace(availabilityJSON)
	var request map[string]any
	if availabilityJSON != "" {
		if err := json.Unmarshal([]byte(availabilityJSON), &request); err != nil {
			return protectedMutationOptions{}, fmt.Errorf("parsing --availability-request JSON: %w", err)
		}
		if err := workflow.ValidatePickupAvailabilityBinding(scheduleRequest, request); err != nil {
			return protectedMutationOptions{}, fmt.Errorf("pickup availability does not match scheduling request: %w", err)
		}
	}
	preflight, err := workflow.PreparePickupPreflight(fedexClient, flags.yes, request, overrideReason)
	if err != nil {
		return protectedMutationOptions{}, err
	}
	return protectedMutationOptions{
		Context: preflight.Context,
		Persist: workflow.PersistOptions{PickupPreflight: preflight.Status, PickupOverrideReason: preflight.OverrideReason, PickupPreflightCutoff: preflight.CutoffTime, PickupPreflightAccessStart: preflight.AccessStart},
		ReviewUpdate: func(summary *approval.ReviewSummary) {
			summary.PickupPreflight = preflight.Status
			summary.PreflightOverride = preflight.OverrideReason
			summary.PickupWindow = preflight.Window
			summary.PickupCutoffTime = preflight.CutoffTime
			summary.PickupAccessStart = preflight.AccessStart
		},
	}, nil
}
