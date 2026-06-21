// Copyright 2026 Anchal Sharma and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newContentsPipeCmd(flags *rootFlags) *cobra.Command {
	var fromStdin bool
	var maxURLs int
	var wait bool

	cmd := &cobra.Command{
		Use:   "pipe",
		Short: "Pipe search result URLs into content extraction.",
		Long: `Read a search result JSON from stdin, extract URLs, and submit them for
content extraction via POST /v1/contents.

Prints the job ID to stdout. With --wait, polls until the job completes
and prints the full results.`,
		Example: `  valyu-pp-cli semantic-search --query "CRISPR therapy" --json | valyu-pp-cli contents pipe --from-stdin
  valyu-pp-cli semantic-search --query "RNA folding" --json | valyu-pp-cli contents pipe --from-stdin --wait`,
		Annotations: map[string]string{"pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !fromStdin {
				return fmt.Errorf("--from-stdin is required (pipe a search result JSON on stdin)")
			}

			input, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}

			urls := extractURLsFromAnyJSON(json.RawMessage(input))
			if len(urls) == 0 {
				return fmt.Errorf("no URLs found in stdin JSON")
			}
			if maxURLs > 0 && len(urls) > maxURLs {
				urls = urls[:maxURLs]
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			body := map[string]any{"urls": urls}
			data, _, apiErr := c.PostWithParams(cmd.Context(), "/v1/contents", map[string]string{}, body)
			if apiErr != nil {
				return classifyAPIError(apiErr, flags)
			}

			jobID := extractJobID(data)
			if !wait || jobID == "" {
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "job %s submitted, waiting...\n", jobID)
			for {
				select {
				case <-cmd.Context().Done():
					return cmd.Context().Err()
				default:
				}
				statusData, statusErr := c.GetNoCache(cmd.Context(),
					"/v1/contents/jobs/"+jobID, map[string]string{},
				)
				if statusErr != nil {
					return classifyAPIError(statusErr, flags)
				}
				state := extractStringField(statusData, "status")
				if state == "completed" || state == "done" || state == "failed" || state == "error" {
					return printOutputWithFlags(cmd.OutOrStdout(), statusData, flags)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "job %s: %s...\n", jobID, state)
				select {
				case <-cmd.Context().Done():
					return cmd.Context().Err()
				case <-time.After(3 * time.Second):
				}
			}
		},
	}
	cmd.Flags().BoolVar(&fromStdin, "from-stdin", false, "Read search result JSON from stdin")
	cmd.Flags().IntVar(&maxURLs, "max-urls", 20, "Maximum number of URLs to send for extraction")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for job completion and print results")
	return cmd
}

// extractURLsFromAnyJSON tries to extract URL strings from various JSON shapes.
func extractURLsFromAnyJSON(data json.RawMessage) []string {
	// Try search result shape: {results: [{url: "..."}]}
	urls := extractURLsFromSearchResponse(data)
	if len(urls) > 0 {
		return urls
	}

	// Try {meta: ..., results: [...]} provenance envelope
	var envelope struct {
		Results json.RawMessage `json:"results"`
	}
	if json.Unmarshal(data, &envelope) == nil && len(envelope.Results) > 0 {
		urls = extractURLsFromSearchResponse(envelope.Results)
		if len(urls) > 0 {
			return urls
		}
	}

	// Try flat string array
	var strArr []string
	if json.Unmarshal(data, &strArr) == nil {
		var result []string
		for _, s := range strArr {
			if len(s) > 7 && s[:4] == "http" {
				result = append(result, s)
			}
		}
		return result
	}

	return nil
}

// extractJobID pulls the job_id or id field from a JSON response.
func extractJobID(data json.RawMessage) string {
	var resp struct {
		JobID string `json:"job_id"`
		ID    string `json:"id"`
	}
	json.Unmarshal(data, &resp)
	if resp.JobID != "" {
		return resp.JobID
	}
	return resp.ID
}

// extractStringField extracts a string field by name from a JSON object.
func extractStringField(data json.RawMessage, field string) string {
	var obj map[string]any
	if json.Unmarshal(data, &obj) != nil {
		return ""
	}
	v, _ := obj[field].(string)
	return v
}
