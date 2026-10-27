// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/mutationguard"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/workflow"
)

func newWorkflowOperationID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte("fedex-workflow-op"))
	}
	return hex.EncodeToString(buf)
}

// runWorkflowMutation validates a label or pickup write, records local ledger
// state, sends it once through the shared --yes and duplicate guard, and
// returns a typed receipt instead of the raw FedEx body.
func runWorkflowMutation(ctx context.Context, fedexClient *client.Client, action, method, path string, body map[string]any, options workflow.PersistOptions) ([]byte, int, error) {
	if options.OperationID == "" {
		options.OperationID = newWorkflowOperationID()
	}
	if fedexClient.DryRun {
		if err := workflow.ValidateRequest(action, body); err != nil {
			return nil, 0, err
		}
		return callWorkflowMethod(fedexClient, method, path, body)
	}
	already, err := workflow.Prepare(ctx, action, body, options)
	if err != nil || already != nil {
		return already, 200, err
	}
	data, status, err := callWorkflowMethod(fedexClient, method, path, body)
	if preSendWorkflowError(err) {
		workflow.AbortInFlight(ctx, action, body, options)
		return nil, status, err
	}
	var unknown *client.OutcomeUnknownError
	isUnknown := errors.As(err, &unknown)
	finished, finishErr := workflow.Finish(ctx, action, body, data, err, isUnknown, options)
	return finished, status, finishErr
}

func callWorkflowMethod(fedexClient *client.Client, method, path string, body map[string]any) ([]byte, int, error) {
	switch method {
	case "POST":
		return fedexClient.Post(path, body)
	case "PUT":
		return fedexClient.Put(path, body)
	default:
		return nil, 0, errors.New("unsupported workflow method " + method)
	}
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
