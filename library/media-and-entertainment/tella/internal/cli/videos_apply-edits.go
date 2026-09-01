// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newVideosApplyEditsCmd(flags *rootFlags) *cobra.Command {
	var operationsJSON string
	var stdin bool
	var apply bool
	cmd := &cobra.Command{
		Use:         "apply-edits <id>",
		Short:       "Preview or apply one atomic batch of timeline resource additions",
		Long:        "Adds up to 200 sound effects, text overlays, zooms, blurs, highlights, or media overlays. Tella's endpoint does not support cuts.",
		Example:     `  tella-pp-cli videos apply-edits vid_abc --operations '[{"type":"add_zoom","operationId":"z1","clipId":"cl_xyz","zoomType":"manualZoom","startTimeMs":1000,"durationMs":1500,"scale":2}]' --dry-run`,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"pp:endpoint": "videos.apply-edits", "pp:method": "POST", "pp:path": "/v1/videos/{id}/edits"},
		RunE: func(cmd *cobra.Command, args []string) error {
			operations, err := videoEditOperations(operationsJSON, stdin)
			if err != nil {
				return usageErr(err)
			}
			body := map[string]any{"operations": operations}
			result := map[string]any{"video_id": args[0], "body": body, "dry_run": true, "applied": false}
			if flags.dryRun || !apply {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			api, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := api.Post(fmt.Sprintf("/v1/videos/%s/edits", args[0]), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result["dry_run"] = false
			result["applied"] = true
			result["status"] = status
			result["data"] = jsonRawToAny(data)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&operationsJSON, "operations", "", "JSON array of official add_* video edit operations")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "Read the operation array or {operations:[...]} from stdin")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the atomic batch; default previews the request")
	return cmd
}

func videoEditOperations(raw string, fromStdin bool) ([]map[string]any, error) {
	if fromStdin {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		raw = string(data)
	}
	if raw == "" {
		return nil, fmt.Errorf("pass --operations JSON or --stdin")
	}
	var operations []map[string]any
	if err := json.Unmarshal([]byte(raw), &operations); err != nil {
		var envelope struct {
			Operations []map[string]any `json:"operations"`
		}
		if envelopeErr := json.Unmarshal([]byte(raw), &envelope); envelopeErr != nil {
			return nil, fmt.Errorf("parsing operations: %w", err)
		}
		operations = envelope.Operations
	}
	if len(operations) == 0 || len(operations) > 200 {
		return nil, fmt.Errorf("operations must contain 1 to 200 items")
	}
	allowed := map[string]bool{
		"add_sound_effect": true, "add_text_overlay": true, "add_zoom": true,
		"add_blur": true, "add_highlight": true, "add_media_overlay": true,
	}
	ids := map[string]bool{}
	for i, operation := range operations {
		typeName, _ := operation["type"].(string)
		if !allowed[typeName] {
			return nil, fmt.Errorf("operation %d has unsupported type %q; apply_video_edits cannot cut clips", i, typeName)
		}
		operationID, _ := operation["operationId"].(string)
		if operationID == "" || ids[operationID] {
			return nil, fmt.Errorf("operation %d needs a unique non-empty operationId", i)
		}
		ids[operationID] = true
	}
	return operations, nil
}
