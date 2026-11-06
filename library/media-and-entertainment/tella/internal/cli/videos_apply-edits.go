// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newVideosApplyEditsCmd(flags *rootFlags) *cobra.Command {
	var operationsJSON string
	var stdin bool
	var apply bool
	cmd := &cobra.Command{
		Use:         "apply-edits <id>",
		Short:       "Preview or apply one atomic batch of timeline resource edits",
		Long:        "Add, update, or remove sound effects, text overlays, zooms, blurs, highlights, media overlays, shape overlays, or layouts in batches of 1 to 200 operations. Tella's endpoint does not support cuts.",
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
			data, status, err := api.Post(cmd.Context(), fmt.Sprintf("/v1/videos/%s/edits", args[0]), body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			result["dry_run"] = false
			result["applied"] = true
			result["status"] = status
			result["data"] = jsonRawToAny(data)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&operationsJSON, "operations", "", "JSON array of official add_*, update_*, or remove_* video edit operations")
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
	// Source: live spec paths./v1/videos/{id}/edits.post requestBody operations.items discriminator.mapping (sha256 62c1e72a1a47).
	allowed := map[string]bool{
		"add_blur":             true,
		"add_highlight":        true,
		"add_layout":           true,
		"add_media_overlay":    true,
		"add_shape_overlay":    true,
		"add_sound_effect":     true,
		"add_text_overlay":     true,
		"add_zoom":             true,
		"remove_blur":          true,
		"remove_highlight":     true,
		"remove_layout":        true,
		"remove_media_overlay": true,
		"remove_shape_overlay": true,
		"remove_sound_effect":  true,
		"remove_text_overlay":  true,
		"remove_zoom":          true,
		"update_blur":          true,
		"update_highlight":     true,
		"update_layout":        true,
		"update_media_overlay": true,
		"update_shape_overlay": true,
		"update_sound_effect":  true,
		"update_text_overlay":  true,
		"update_zoom":          true,
	}
	ids := map[string]bool{}
	for i, operation := range operations {
		typeName, _ := operation["type"].(string)
		if !allowed[typeName] {
			types := make([]string, 0, len(allowed))
			for name := range allowed {
				types = append(types, name)
			}
			sort.Strings(types)
			return nil, fmt.Errorf("operation %d has unsupported type %q; allowed types: %s; apply_video_edits cannot cut clips", i, typeName, strings.Join(types, ", "))
		}
		operationID, _ := operation["operationId"].(string)
		if operationID == "" || ids[operationID] {
			return nil, fmt.Errorf("operation %d needs a unique non-empty operationId", i)
		}
		ids[operationID] = true
	}
	return operations, nil
}
