// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/mutationguard"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/workflow"
)

func workflowActionFor(ep endpointTool) (string, bool) {
	switch {
	case ep.Method == "POST" && ep.Path == "/ship/v1/shipments":
		return workflow.ActionCreateLabel, true
	case ep.Method == "PUT" && ep.Path == "/ship/v1/shipments/cancel":
		return workflow.ActionCancelShipment, true
	case ep.Method == "POST" && ep.Path == "/pickup/v1/pickups":
		return workflow.ActionSchedulePickup, true
	case ep.Method == "PUT" && ep.Path == "/pickup/v1/pickups/cancel":
		return workflow.ActionCancelPickup, true
	default:
		return "", false
	}
}

func readActionFor(ep endpointTool) (string, bool) {
	switch ep.Path {
	case "/rate/v1/rates/quotes":
		return "get_rates", true
	case "/address/v1/addresses/resolve":
		return "validate_address", true
	case "/ship/v1/shipments/packages/validate":
		return "validate_shipment", true
	case "/pickup/v1/pickups/availabilities":
		return "pickup_availability", true
	default:
		return "", false
	}
}

func newWorkflowOperationID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "fedex-workflow-operation"
	}
	return hex.EncodeToString(buf)
}

func preSendWorkflowError(err error) bool {
	if err == nil {
		return false
	}
	var duplicate *mutationguard.DuplicateError
	return errors.As(err, &duplicate) ||
		errors.Is(err, mutationguard.ErrBusy) ||
		errors.Is(err, client.ErrWriteNotApproved) ||
		errors.Is(err, client.ErrCompanionWriteRefused)
}

func pickupPreflightOptions(fedexClient *client.Client, schedule map[string]any, arguments map[string]any) (workflow.PersistOptions, error) {
	override, _ := arguments["availability_override_reason"].(string)
	var request map[string]any
	if raw, ok := arguments["availability_request"]; ok && raw != nil {
		encoded, err := json.Marshal(raw)
		if err != nil {
			return workflow.PersistOptions{}, fmt.Errorf("availability_request must be a JSON object")
		}
		if err := json.Unmarshal(encoded, &request); err != nil {
			return workflow.PersistOptions{}, fmt.Errorf("availability_request must be a JSON object")
		}
		if err := workflow.ValidatePickupAvailabilityBinding(schedule, request); err != nil {
			return workflow.PersistOptions{}, fmt.Errorf("pickup availability does not match scheduling request: %w", err)
		}
	}
	preflight, err := workflow.PreparePickupPreflight(fedexClient, false, request, override)
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

func runMCPWorkflow(ctx context.Context, fedexClient *client.Client, ep endpointTool, body map[string]any, options workflow.PersistOptions) (json.RawMessage, int, error) {
	if options.OperationID == "" {
		options.OperationID = newWorkflowOperationID()
	}
	action, _ := workflowActionFor(ep)
	already, err := workflow.Prepare(ctx, action, body, options)
	if err != nil || already != nil {
		if already != nil {
			return already, 200, nil
		}
		return nil, 0, err
	}
	data, status, err := executeEndpoint(fedexClient, ep.Method, ep.Path, body)
	if preSendWorkflowError(err) {
		workflow.AbortInFlight(ctx, action, body, options)
		return nil, status, err
	}
	var unknown *client.OutcomeUnknownError
	isUnknown := errors.As(err, &unknown)
	finished, finishErr := workflow.Finish(ctx, action, body, data, err, isUnknown, options)
	if finishErr != nil {
		return nil, status, finishErr
	}
	return finished, status, nil
}

func workflowErrorClass(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	switch {
	case strings.HasPrefix(message, "invalid "):
		return "invalid_request"
	case strings.Contains(message, "availability"):
		return "pickup_preflight_failed"
	case strings.Contains(message, "legacy"):
		return "pickup_resolution_failed"
	default:
		return ""
	}
}
