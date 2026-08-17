// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelPlaygroundsCreateReconcileCmd(flags *rootFlags) *cobra.Command {
	// pp:data-source computed
	var flagReceipt string
	var verifyLive bool

	cmd := &cobra.Command{
		Use:         "create-reconcile",
		Short:       "Turn a partial create receipt into a resume or cleanup plan; a no-op requires live verification.",
		Example:     "  screencloud-pp-cli playgrounds create-reconcile --receipt ./path/to/reviewed-receipt.json --verify-live --yes --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printValue(cmd, flags, map[string]any{"operation": "read receipt and compute an idempotent plan", "receipt": filepath.Clean(flagReceipt), "verify_live": verifyLive, "external_mutation": false})
			}
			if strings.TrimSpace(flagReceipt) == "" {
				return usageErr(fmt.Errorf("--receipt is required"))
			}
			raw, err := os.ReadFile(filepath.Clean(flagReceipt)) // #nosec G304 -- explicitly selected receipt.
			if err != nil {
				return fmt.Errorf("reading --receipt: %w", err)
			}
			var receipt map[string]any
			if err := json.Unmarshal(raw, &receipt); err != nil {
				return usageErr(fmt.Errorf("receipt must be a JSON object: %w", err))
			}
			if nested, ok := receipt["receipt"].(map[string]any); ok {
				receipt = nested
			}
			stage := strings.ToLower(firstString(receipt, "stage"))
			studioCreated := boolAt(receipt, "studio_instance_created")
			filesUploaded := boolAt(receipt, "files_uploaded")
			dataUploaded := boolAt(receipt, "data_uploaded")
			switch stage {
			case "", "planned", "validated":
			case "studio_instance_created":
				studioCreated = true
			case "files_uploaded":
				studioCreated, filesUploaded = true, true
			case "data_uploaded", "complete":
				studioCreated, filesUploaded, dataUploaded = true, true, true
			case "failed", "files_failed", "data_failed":
				if !hasReceiptStateBooleans(receipt) {
					return usageErr(fmt.Errorf("ambiguous failure stage %q requires explicit studio_instance_created, files_uploaded, and data_uploaded booleans", stage))
				}
			default:
				return usageErr(fmt.Errorf("unsupported receipt stage %q", stage))
			}
			cleanupRequested := boolAt(receipt, "cleanup_requested") || strings.EqualFold(firstString(receipt, "intent", "desired_action"), "cleanup")
			liveState := map[string]any{"checked": false}
			if verifyLive {
				appInstanceID := firstString(receipt, "app_instance_id", "appInstanceId")
				appUUID := firstString(receipt, "app_uuid", "appUuid")
				spaceID := firstString(receipt, "space_id", "spaceId")
				if appInstanceID == "" || appUUID == "" || spaceID == "" {
					return usageErr(fmt.Errorf("--verify-live requires app_instance_id, app_uuid, and space_id in the receipt"))
				}
				data, _, err := runGraphQL(cmd.Context(), flags, `query ReconcileInstance($id: UUID!) { allAppInstances(first: 1, filter: {id: {equalTo: $id}}) { nodes { id } } }`, map[string]any{"id": appInstanceID})
				if err != nil {
					return err
				}
				var studio struct {
					AllAppInstances struct {
						Nodes []map[string]any `json:"nodes"`
					} `json:"allAppInstances"`
				}
				if err := json.Unmarshal(data, &studio); err != nil {
					return err
				}
				studioCreated = len(studio.AllAppInstances.Nodes) > 0
				token, _, err := mintScopedJWT(cmd.Context(), flags, "management", spaceID, "")
				if err != nil {
					return err
				}
				playgroundsClient, err := newPlaygroundsClient(flags)
				if err != nil {
					return err
				}
				_, filesErr := playgroundsClient.GetWithHeadersNoCache(cmd.Context(), "/files/"+url.PathEscape(appUUID), nil, bearerHeader(token))
				if filesErr != nil && !isHTTPStatus(filesErr, 404) {
					return classifyAPIError(filesErr, flags)
				}
				_, dataErr := playgroundsClient.GetWithHeadersNoCache(cmd.Context(), "/data/"+url.PathEscape(appUUID), nil, bearerHeader(token))
				if dataErr != nil && !isHTTPStatus(dataErr, 404) {
					return classifyAPIError(dataErr, flags)
				}
				filesUploaded = filesErr == nil
				dataUploaded = dataErr == nil
				liveState = map[string]any{"checked": true, "studio_instance_exists": studioCreated, "files_exist": filesUploaded, "data_exists": dataUploaded}
			}
			actions := []map[string]any{}
			state := "resume"
			if cleanupRequested {
				state = "cleanup"
				if dataUploaded {
					actions = append(actions, map[string]any{"order": len(actions) + 1, "action": "remove_playgrounds_data", "guard": "verify receipt target and lastModified before deletion"})
				}
				if filesUploaded {
					actions = append(actions, map[string]any{"order": len(actions) + 1, "action": "remove_playgrounds_files", "guard": "verify receipt target and lastModified before deletion"})
				}
				if studioCreated {
					actions = append(actions, map[string]any{"order": len(actions) + 1, "action": "delete_studio_instance", "guard": "re-query by receipt identity and require a separately approved mutation"})
				}
			} else if !studioCreated {
				actions = append(actions, map[string]any{"order": 1, "action": "create_studio_instance", "guard": "re-query by receipt identity before create"})
			}
			if !cleanupRequested && !filesUploaded {
				actions = append(actions, map[string]any{"order": len(actions) + 1, "action": "upload_files", "guard": "compare expected lastModified"})
			}
			if !cleanupRequested && !dataUploaded {
				actions = append(actions, map[string]any{"order": len(actions) + 1, "action": "upload_data", "guard": "compare expected lastModified"})
			}
			if len(actions) == 0 {
				if verifyLive {
					state = "noop"
				} else {
					state = "verification_required"
					actions = append(actions, map[string]any{"order": 1, "action": "verify_live_state", "guard": "rerun with --verify-live --yes and receipt target identifiers before accepting a no-op"})
				}
			}
			out := map[string]any{"state": state, "actions": actions, "external_mutation": false, "receipt_stage": stage, "app_uuid": firstString(receipt, "app_uuid", "appUuid"), "space_id": firstString(receipt, "space_id", "spaceId"), "live_state": liveState, "sensitive_fields_echoed": false}
			return printValue(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagReceipt, "receipt", "", "Redacted partial-operation receipt JSON")
	cmd.Flags().BoolVar(&verifyLive, "verify-live", false, "Verify receipt stages with content-read-only calls; this mints a scoped JWT and requires --yes")
	return cmd
}

func boolAt(object map[string]any, key string) bool {
	value, _ := object[key].(bool)
	return value
}

func hasReceiptStateBooleans(receipt map[string]any) bool {
	for _, key := range []string{"studio_instance_created", "files_uploaded", "data_uploaded"} {
		if _, ok := receipt[key].(bool); !ok {
			return false
		}
	}
	return true
}
