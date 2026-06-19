// Copyright 2026 Anchal Sharma and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// generationStatusPollResponse is the minimal shape we need from GET /v1.0/generations/{id}.
type generationStatusPollResponse struct {
	GenerationID string `json:"generationId"`
	Status       string `json:"status"`
	GammaID      string `json:"gammaId"`
	GammaURL     string `json:"gammaUrl"`
	ExportURL    string `json:"exportUrl"`
	Error        *struct {
		Message    string `json:"message"`
		StatusCode int    `json:"statusCode"`
	} `json:"error"`
	Credits *struct {
		Deducted  float64 `json:"deducted"`
		Remaining float64 `json:"remaining"`
	} `json:"credits"`
}

// gammaClientGetter is satisfied by the generated client (used for type-safe polling without
// importing the client package directly from a novel command file).
type gammaClientGetter interface {
	GetNoCache(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

// watchGenerationStatus polls GET /v1.0/generations/{id} every 5 seconds until
// status is "completed" or "failed". It prints a spinner to stderr and returns
// the raw JSON of the final poll response.
func watchGenerationStatus(ctx context.Context, c gammaClientGetter, flags *rootFlags, generationID string, errW io.Writer) (json.RawMessage, error) {
	const pollInterval = 5 * time.Second
	const maxPolls = 120 // 10 minutes ceiling

	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinIdx := 0

	path := "/v1.0/generations/" + generationID
	params := map[string]string{}

	isTTY := isTerminal(errW)

	if isTTY {
		fmt.Fprintf(errW, "Generation started: %s\n", generationID)
	} else if flags.asJSON {
		enc := json.NewEncoder(errW)
		_ = enc.Encode(map[string]any{"event": "generation_started", "generationId": generationID})
	}

	for attempt := 0; attempt < maxPolls; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}

		data, err := c.GetNoCache(ctx, path, params)
		if err != nil {
			return nil, classifyAPIError(err, flags)
		}

		var resp generationStatusPollResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("parsing generation status: %w", err)
		}

		status := resp.Status
		if isTTY {
			sp := spinner[spinIdx%len(spinner)]
			spinIdx++
			// Clear line and reprint
			fmt.Fprintf(errW, "\r%s  %s (%ds elapsed)   ", sp, status, (attempt+1)*5)
		} else if flags.asJSON {
			enc := json.NewEncoder(errW)
			_ = enc.Encode(map[string]any{
				"event":        "generation_poll",
				"generationId": generationID,
				"status":       status,
				"attempt":      attempt + 1,
			})
		}

		switch status {
		case "completed":
			if isTTY {
				fmt.Fprintln(errW) // newline after spinner
				fmt.Fprintf(errW, "✓ Generation completed!\n")
				fmt.Fprintf(errW, "  gammaId:  %s\n", resp.GammaID)
				fmt.Fprintf(errW, "  gammaUrl: %s\n", resp.GammaURL)
				if resp.ExportURL != "" {
					fmt.Fprintf(errW, "  export:   %s\n", resp.ExportURL)
				}
				if resp.Credits != nil {
					fmt.Fprintf(errW, "  credits:  %.0f deducted, %.0f remaining\n", resp.Credits.Deducted, resp.Credits.Remaining)
				}
			} else if flags.asJSON {
				enc := json.NewEncoder(errW)
				_ = enc.Encode(map[string]any{
					"event":        "generation_completed",
					"generationId": generationID,
					"gammaId":      resp.GammaID,
					"gammaUrl":     resp.GammaURL,
					"exportUrl":    resp.ExportURL,
				})
			}
			return data, nil

		case "failed":
			if isTTY {
				fmt.Fprintln(errW)
			}
			errMsg := "generation failed"
			if resp.Error != nil && resp.Error.Message != "" {
				errMsg = resp.Error.Message
			}
			return data, fmt.Errorf("generation %s failed: %s", generationID, errMsg)
		}
		// status == "pending" — keep polling
	}

	if isTTY {
		fmt.Fprintln(errW)
	}
	return nil, fmt.Errorf("timed out waiting for generation %s after %d minutes", generationID, maxPolls*5/60)
}

// downloadExportFile downloads the exportUrl from a completed generation status response
// to the given directory. The filename is derived from the gammaId and the URL extension.
func downloadExportFile(ctx context.Context, statusData json.RawMessage, dir string, errW io.Writer) error {
	var resp generationStatusPollResponse
	if err := json.Unmarshal(statusData, &resp); err != nil {
		return fmt.Errorf("parsing status for download: %w", err)
	}
	if resp.ExportURL == "" {
		return fmt.Errorf("no exportUrl in generation response (was --export-as set during generation?)")
	}

	// Derive filename from gammaId + URL extension
	ext := ".bin"
	u := resp.ExportURL
	if idx := strings.LastIndex(u, "."); idx > strings.LastIndex(u, "/") {
		candidate := u[idx:]
		if i := strings.IndexAny(candidate, "?#"); i > 0 {
			candidate = candidate[:i]
		}
		if candidate != "" {
			ext = candidate
		}
	}
	gammaID := resp.GammaID
	if gammaID == "" {
		gammaID = resp.GenerationID
	}
	filename := gammaID + ext
	destPath := filepath.Join(dir, filename)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resp.ExportURL, nil)
	if err != nil {
		return fmt.Errorf("building download request: %w", err)
	}
	httpClient := &http.Client{Timeout: 5 * time.Minute}
	httpResp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading export: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 300 {
		return fmt.Errorf("download returned HTTP %d", httpResp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", destPath, err)
	}
	defer f.Close()

	written, err := io.Copy(f, httpResp.Body)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	if isTerminal(errW) {
		fmt.Fprintf(errW, "  saved:    %s (%.0f KB)\n", destPath, float64(written)/1024)
	} else {
		enc := json.NewEncoder(errW)
		_ = enc.Encode(map[string]any{
			"event": "export_downloaded",
			"path":  destPath,
			"bytes": written,
		})
	}
	return nil
}

// applySharingPreset maps convenience preset names to workspaceAccess / externalAccess values.
// private       → workspaceAccess=noAccess,   externalAccess=noAccess
// view-link     → workspaceAccess=noAccess,   externalAccess=view
// edit-link     → workspaceAccess=noAccess,   externalAccess=edit
// workspace-view → workspaceAccess=view,      externalAccess=noAccess
func applySharingPreset(preset string, workspaceAccess, externalAccess *string) {
	switch strings.ToLower(preset) {
	case "private":
		*workspaceAccess = "noAccess"
		*externalAccess = "noAccess"
	case "view-link":
		*workspaceAccess = "noAccess"
		*externalAccess = "view"
	case "edit-link":
		*workspaceAccess = "noAccess"
		*externalAccess = "edit"
	case "workspace-view":
		*workspaceAccess = "view"
		*externalAccess = "noAccess"
	default:
		fmt.Fprintf(os.Stderr, "warning: unknown --sharing preset %q; expected: private, view-link, edit-link, workspace-view\n", preset)
	}
}
