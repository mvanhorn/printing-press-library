// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/approval"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/client"
	"github.com/spf13/cobra"
)

const cliPendingOperationTTL = 10 * time.Minute

// executeProtectedMutation implements the CLI's preview -> bound confirmation
// workflow. A successful preview returns executed=false and never calls FedEx.
func executeProtectedMutation(cmd *cobra.Command, flags *rootFlags, fedexClient *client.Client, action, method, path string, body map[string]any) ([]byte, int, bool, error) {
	origin, err := approval.NormalizeOrigin(fedexClient.BaseURL)
	if err != nil {
		return nil, 0, false, err
	}
	pendingDir, err := approval.DefaultStoreDir()
	if err != nil {
		return nil, 0, false, err
	}
	mutation := approval.Mutation{Action: action, Origin: origin, Method: method, Path: path, Request: body}
	store := approval.NewStore(pendingDir, cliPendingOperationTTL)

	if !flags.yes {
		if strings.TrimSpace(flags.operationID) != "" || strings.TrimSpace(flags.confirmationDigest) != "" {
			return nil, 0, false, errors.New("--operation-id and --confirmation-digest require --yes")
		}
		record, err := store.Create(mutation, approval.Summarize(action, body))
		if err != nil {
			return nil, 0, false, err
		}
		preview := map[string]any{
			"status":              approval.StatusPending,
			"action":              action,
			"origin":              origin,
			"operation_id":        record.ID,
			"confirmation_digest": record.ConfirmationDigest,
			"expires_at":          record.ExpiresAt,
			"review":              record.Review,
			"instruction":         "Review this summary, then repeat the exact command with --yes, --operation-id, and --confirmation-digest.",
		}
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(preview); err != nil {
			return nil, 0, false, fmt.Errorf("write mutation preview: %w", err)
		}
		return nil, 0, false, nil
	}

	if flags.dryRun {
		return nil, 0, false, errors.New("--dry-run cannot be combined with --yes for a protected mutation")
	}
	operationID := strings.TrimSpace(flags.operationID)
	digest := strings.TrimSpace(flags.confirmationDigest)
	if operationID == "" || digest == "" {
		return nil, 0, false, errors.New("--yes requires --operation-id and --confirmation-digest from the matching preview")
	}
	_, permit, err := store.Consume(operationID, digest, mutation)
	if err != nil {
		return nil, 0, false, fmt.Errorf("consume mutation confirmation: %w", err)
	}
	fedexClient.MutationPermit = permit

	var data []byte
	var status int
	switch method {
	case "POST":
		data, status, err = fedexClient.Post(path, body)
	case "PUT":
		data, status, err = fedexClient.Put(path, body)
	case "PATCH":
		data, status, err = fedexClient.Patch(path, body)
	case "DELETE":
		data, status, err = fedexClient.Delete(path)
	default:
		err = fmt.Errorf("unsupported protected mutation method %q", method)
	}
	completionStatus := approval.StatusSucceeded
	errorClass := ""
	if err != nil {
		completionStatus = approval.StatusRejected
		errorClass = "mutation_rejected"
		var outcomeUnknown *client.OutcomeUnknownError
		if errors.As(err, &outcomeUnknown) {
			completionStatus = approval.StatusOutcomeUnknown
			errorClass = "outcome_unknown"
		}
	}
	if completeErr := store.Complete(operationID, completionStatus, errorClass); completeErr != nil {
		if err != nil {
			return nil, status, true, errors.Join(err, fmt.Errorf("persist completion status: %w", completeErr))
		}
		return data, status, true, fmt.Errorf("remote mutation succeeded but completion status could not be persisted: %w", completeErr)
	}
	return data, status, true, err
}
