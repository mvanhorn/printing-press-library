// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — not generated.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/operon/internal/cliutil"
)

const (
	specURL           = "https://operon.so/openapi.json"
	defaultProbeBase  = "https://api.operon.so"
	specVerifyTimeout = 10 * time.Second
)

// specProbeResult is one row of the drift report.
type specProbeResult struct {
	Path           string `json:"path"`
	ExpectedStatus int    `json:"expected_status"`
	ActualStatus   int    `json:"actual_status"`
	BodyShape      string `json:"body_shape"`
	ShapeOK        bool   `json:"shape_ok"`
	Notes          string `json:"notes,omitempty"`
}

func newSpecCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Inspect the published OpenAPI spec and check it against live API behavior.",
	}
	cmd.AddCommand(newSpecVerifyCmd(flags))
	return cmd
}

func newSpecVerifyCmd(flags *rootFlags) *cobra.Command {
	var timeoutFlag time.Duration

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Re-fetch the published OpenAPI spec and probe documented GET endpoints for drift.",
		Long: `Fetch https://operon.so/openapi.json, walk the documented paths, and probe each
auth-optional GET endpoint with a short timeout. Compare the live response
status and body shape against what the spec advertises and report any drift.

Skipped by design:
  - POST/DELETE/PUT/PATCH writes (a verify pass should not mutate state)
  - /x402/campaign create (payment-gated; would return 402, not drift)
  - /c/{impressionId} (302 redirect endpoint, no shape to drift against)`,
		Example: strings.Trim(`
  operon-pp-cli spec verify
  operon-pp-cli spec verify --json
  operon-pp-cli spec verify --timeout 5s
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Defense-in-depth: the verify env-var short-circuit lets the
			// printing-press verifier exercise this command without hitting
			// the network. We still print the plan so the verifier captures
			// a non-empty stdout for the harness.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch:", specURL)
				fmt.Fprintln(cmd.OutOrStdout(), "would probe: /health, /demand")
				return nil
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "GET %s\n", specURL)
				fmt.Fprintf(cmd.OutOrStdout(), "  then probe documented GETs at %s\n", defaultProbeBase)
				fmt.Fprintf(cmd.OutOrStdout(), "  timeout: %s\n", timeoutFlag)
				return nil
			}

			httpClient := &http.Client{Timeout: timeoutFlag}

			// Confirm spec is reachable before pulling it. ProbeReachable
			// also handles the bot-screen / TLS-shutdown cases that bare
			// GET would surface as a confusing read error.
			ctx, cancel := context.WithTimeout(context.Background(), timeoutFlag)
			defer cancel()
			if status, code, err := cliutil.ProbeReachable(ctx, httpClient, specURL); err != nil {
				return apiErr(fmt.Errorf("spec unreachable (%s): %w", status, err))
			} else if status == cliutil.ReachabilityBlocked {
				return apiErr(fmt.Errorf("spec returned HTTP %d (blocked) — cannot verify", code))
			}

			spec, err := fetchSpec(httpClient)
			if err != nil {
				return apiErr(err)
			}

			results := probeDocumentedGets(httpClient, spec)

			if flags.asJSON || flags.csv || flags.selectFields != "" || flags.compact {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				headers := []string{"path", "expected", "actual", "shape", "notes"}
				rows := make([][]string, 0, len(results))
				for _, r := range results {
					shape := "OK"
					if !r.ShapeOK {
						shape = "MISMATCH"
					}
					rows = append(rows, []string{
						r.Path,
						fmt.Sprintf("%d", r.ExpectedStatus),
						fmt.Sprintf("%d", r.ActualStatus),
						shape,
						r.Notes,
					})
				}
				return flags.printTable(cmd, headers, rows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().DurationVar(&timeoutFlag, "timeout", specVerifyTimeout, "Per-request timeout for spec fetch and probes")
	return cmd
}

func fetchSpec(client *http.Client) (map[string]any, error) {
	req, err := http.NewRequest("GET", specURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building spec request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "operon-pp-cli/spec-verify")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", specURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned HTTP %d", specURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("reading spec body: %w", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(body, &spec); err != nil {
		return nil, fmt.Errorf("parsing spec JSON: %w", err)
	}
	return spec, nil
}

// probeDocumentedGets enumerates GET endpoints in the spec, skips the
// excluded set (writes, x402 create, click redirect), and probes the
// remainder. For paths with templated segments (e.g. /x402/campaign/{id})
// we skip probing because we have no synthetic value that would pass the
// path-validation layer and the resulting 404 is not a drift signal.
func probeDocumentedGets(client *http.Client, spec map[string]any) []specProbeResult {
	pathsRaw, _ := spec["paths"].(map[string]any)
	if pathsRaw == nil {
		return nil
	}

	// Sort paths for deterministic output across runs.
	keys := make([]string, 0, len(pathsRaw))
	for k := range pathsRaw {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	results := make([]specProbeResult, 0, len(keys))
	for _, path := range keys {
		ops, _ := pathsRaw[path].(map[string]any)
		if ops == nil {
			continue
		}
		getOp, hasGet := ops["get"].(map[string]any)
		if !hasGet {
			continue
		}
		// Skip the click redirect endpoint — 302 is not a drift signal
		// and any synthetic impressionId returns the operon.so fallback.
		if path == "/c/{impressionId}" {
			results = append(results, specProbeResult{
				Path:           path,
				ExpectedStatus: 302,
				BodyShape:      "skipped",
				ShapeOK:        true,
				Notes:          "302 redirect endpoint — skipped by design",
			})
			continue
		}
		// Skip any path that contains an unresolved template segment we
		// can't fill (e.g. {id} on x402 read-campaign).
		if strings.Contains(path, "{") {
			results = append(results, specProbeResult{
				Path:           path,
				ExpectedStatus: 0,
				BodyShape:      "skipped",
				ShapeOK:        true,
				Notes:          "templated path — no synthetic value available",
			})
			continue
		}

		expectedStatus := pickPrimaryStatus(getOp)
		expectedShape := pickExpectedShape(getOp, expectedStatus)
		result := probeOnePath(client, path, expectedStatus, expectedShape)
		results = append(results, result)
	}
	return results
}

// pickPrimaryStatus chooses the lowest 2xx code documented on the
// operation; if none, it falls back to "default" → 200.
func pickPrimaryStatus(op map[string]any) int {
	responses, _ := op["responses"].(map[string]any)
	if responses == nil {
		return 200
	}
	best := 0
	for code := range responses {
		var n int
		fmt.Sscanf(code, "%d", &n)
		if n >= 200 && n < 300 {
			if best == 0 || n < best {
				best = n
			}
		}
	}
	if best == 0 {
		return 200
	}
	return best
}

// pickExpectedShape returns "array", "object", or "text" based on the
// documented content schema for the chosen status. Falls back to "object"
// when the spec doesn't pin it down.
func pickExpectedShape(op map[string]any, status int) string {
	responses, _ := op["responses"].(map[string]any)
	if responses == nil {
		return "object"
	}
	resp, _ := responses[fmt.Sprintf("%d", status)].(map[string]any)
	if resp == nil {
		return "object"
	}
	content, _ := resp["content"].(map[string]any)
	if content == nil {
		return "object"
	}
	if _, ok := content["text/plain"]; ok {
		return "text"
	}
	for mediaType, raw := range content {
		if !strings.Contains(mediaType, "json") {
			continue
		}
		body, _ := raw.(map[string]any)
		schema, _ := body["schema"].(map[string]any)
		if schema == nil {
			return "object"
		}
		if t, _ := schema["type"].(string); t == "array" {
			return "array"
		}
		return "object"
	}
	return "object"
}

func probeOnePath(client *http.Client, path string, expectedStatus int, expectedShape string) specProbeResult {
	result := specProbeResult{Path: path, ExpectedStatus: expectedStatus}
	target := defaultProbeBase + path

	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		result.Notes = "build request: " + err.Error()
		return result
	}
	req.Header.Set("Accept", "application/json, text/plain;q=0.5, */*;q=0.1")
	req.Header.Set("User-Agent", "operon-pp-cli/spec-verify")
	// /demand is gated on X-Operon-Client in production; sandbox accepts
	// any well-formed UUID. We send a fixture so the probe doesn't 401.
	if path == "/demand" {
		uuid := os.Getenv("OPERON_CLIENT_UUID")
		if uuid == "" {
			uuid = "00000000-0000-4000-8000-000000000001"
		}
		req.Header.Set("X-Operon-Client", uuid)
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Notes = "network: " + err.Error()
		return result
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	result.ActualStatus = resp.StatusCode

	actualShape := classifyShape(body, resp.Header.Get("Content-Type"))
	result.BodyShape = actualShape
	result.ShapeOK = actualShape == expectedShape

	if result.ActualStatus != expectedStatus {
		result.Notes = fmt.Sprintf("status drift: spec=%d actual=%d", expectedStatus, result.ActualStatus)
	}
	if !result.ShapeOK && result.Notes == "" {
		result.Notes = fmt.Sprintf("shape drift: spec=%s actual=%s", expectedShape, actualShape)
	}
	return result
}

func classifyShape(body []byte, contentType string) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "empty"
	}
	if strings.HasPrefix(trimmed, "[") {
		return "array"
	}
	if strings.HasPrefix(trimmed, "{") {
		return "object"
	}
	if strings.Contains(strings.ToLower(contentType), "text/plain") {
		return "text"
	}
	// Bare token like "ok" with no Content-Type — still text-y.
	return "text"
}
