// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/square/internal/config"
	"github.com/spf13/cobra"
)

func newNovelRequestCheckCmd(flags *rootFlags) *cobra.Command {
	var flagMethod string
	var flagPath string
	var flagBody string
	var flagApproveMutation bool

	cmd := &cobra.Command{
		Use:         "check",
		Short:       "Validate a planned Square request, environment, API version, mutation policy, and idempotency before anything is sent.",
		Example:     "  square-pp-cli request check --method POST --path /v2/payments --body payment-request.json --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(flagMethod) == "" {
				return fmt.Errorf("required flag(s) \"method\" not set")
			}
			if strings.TrimSpace(flagPath) == "" {
				return fmt.Errorf("required flag(s) \"path\" not set")
			}
			method := strings.ToUpper(strings.TrimSpace(flagMethod))
			allowed := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "HEAD": true}
			if !allowed[method] {
				return fmt.Errorf("invalid value %q for --method: must be GET, POST, PUT, PATCH, DELETE, or HEAD", flagMethod)
			}
			parsed, err := url.ParseRequestURI(flagPath)
			if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") {
				return fmt.Errorf("invalid value %q for --path: must be a relative API path such as /v2/payments", flagPath)
			}
			if !(strings.HasPrefix(parsed.Path, "/v2/") || strings.HasPrefix(parsed.Path, "/v1/") || strings.HasPrefix(parsed.Path, "/reporting/v1/")) {
				return fmt.Errorf("invalid value %q for --path: must start with /v2/, /v1/, or /reporting/v1/", flagPath)
			}
			mutation := method != "GET" && method != "HEAD"
			var body map[string]any
			bodyValid := flagBody == ""
			if flagBody != "" {
				body, err = readRequestBodyObject(flagBody)
				if err != nil {
					return err
				}
				bodyValid = true
			}
			operation, operationFound := findRequestOperation(method, parsed.Path)
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			baseURL, err := inspectSquareBaseURL(cfg.BaseURL)
			if err != nil {
				return fmt.Errorf("invalid configured base URL: %w", err)
			}
			environment := baseURL.Environment
			apiVersion := ""
			for key, value := range cfg.Headers {
				if strings.EqualFold(key, "Square-Version") {
					apiVersion = strings.TrimSpace(value)
					break
				}
			}
			versionSource := "config"
			if apiVersion == "" {
				versionSource = "not configured"
			}
			idempotency := containsKey(body, "idempotency_key")
			type check struct {
				Name   string `json:"name"`
				Status string `json:"status"`
				Detail string `json:"detail"`
			}
			checks := []check{{"method", "pass", method + " is a recognized HTTP method"}}
			if operationFound {
				checks = append(checks, check{"operation", "pass", method + " " + operation.Path + " is present in this CLI's generated operation surface"})
			} else {
				checks = append(checks, check{"operation", "fail", method + " " + parsed.Path + " was not found in this CLI's generated operation surface"})
			}
			readyForManualReview := operationFound && bodyValid
			if environment == "custom/unknown" {
				checks = append(checks, check{"environment", "warning", "Configured base URL is valid but is not an exact Square production or sandbox host"})
				readyForManualReview = false
			} else {
				checks = append(checks, check{"environment", "pass", environment + " base URL selected"})
			}
			if apiVersion == "" {
				checks = append(checks, check{"api_version", "warning", "Square-Version is not configured, so this client will not send an API version header"})
				readyForManualReview = false
			} else {
				checks = append(checks, check{"api_version", "pass", apiVersion + " (" + versionSource + ")"})
			}
			checks = append(checks, check{"request_schema", "unavailable", "This build can verify the operation exists, but does not expose a runtime request schema; field names, types, and required fields were not schema-validated"})
			if mutation && flagBody == "" {
				checks = append(checks, check{"body", "warning", "Mutation has no JSON body to inspect"})
				readyForManualReview = false
			} else if flagBody != "" && bodyValid {
				checks = append(checks, check{"body", "pass", "JSON body is readable and valid"})
			} else {
				checks = append(checks, check{"body", "pass", "No request body supplied"})
			}
			if mutation && !idempotency {
				checks = append(checks, check{"idempotency", "warning", "No idempotency_key found; confirm whether this endpoint requires one"})
				readyForManualReview = false
			} else if mutation {
				checks = append(checks, check{"idempotency", "pass", "idempotency_key is present"})
			}
			if mutation && !flagApproveMutation {
				checks = append(checks, check{"mutation_approval", "fail", "Mutation was not explicitly approved; re-run with --approve-mutation after reviewing the request"})
				readyForManualReview = false
			} else if mutation {
				checks = append(checks, check{"mutation_approval", "pass", "Mutation was explicitly approved for readiness evaluation; this command still did not send it"})
			}
			// A syntactic/contract preflight cannot prove a request is safe to
			// send without runtime request-schema validation. Keep the strong
			// safety claim false and expose the narrower readiness result.
			result, err := json.Marshal(map[string]any{
				"valid":                   readyForManualReview,
				"safe_to_send":            false,
				"ready_for_manual_review": readyForManualReview,
				"method":                  method,
				"path":                    flagPath,
				"matched_operation_path":  operation.Path,
				"operation":               operation.Endpoint,
				"environment":             environment,
				"base_url":                baseURL.Origin,
				"api_version":             apiVersion,
				"api_version_source":      versionSource,
				"mutation":                mutation,
				"mutation_approved":       flagApproveMutation,
				"idempotency_present":     idempotency,
				"body_valid":              bodyValid,
				"schema_validation": map[string]any{
					"available": false,
					"detail":    "Operation existence is checked against generated CLI metadata; request fields are not schema-validated by this command.",
				},
				"checks": checks,
				"note":   "This command did not send a network request.",
			})
			if err != nil {
				return err
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), result, flags, map[string]any{"source": "computed"})
		},
	}
	cmd.Flags().StringVar(&flagMethod, "method", "", "HTTP method to validate (for example POST)")
	cmd.Flags().StringVar(&flagPath, "path", "", "Relative Square API path (for example /v2/payments)")
	cmd.Flags().StringVar(&flagBody, "body", "", "Path to an optional JSON request body")
	cmd.Flags().BoolVar(&flagApproveMutation, "approve-mutation", false, "Explicitly approve a mutation for readiness evaluation (does not send it)")
	return cmd
}

const maxRequestCheckBodyBytes int64 = 10 << 20

func readRequestBodyObject(path string) (map[string]any, error) {
	// #nosec G304 -- --body is an intentional user-selected local file; it is
	// opened read-only, required to be regular, and capped at 10 MiB below.
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("invalid value %q for --body: %v", path, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("invalid value %q for --body: %v", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("invalid value %q for --body: must be a regular file", path)
	}
	if info.Size() > maxRequestCheckBodyBytes {
		return nil, fmt.Errorf("invalid value %q for --body: file exceeds 10 MiB", path)
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxRequestCheckBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("invalid value %q for --body: %v", path, err)
	}
	if int64(len(raw)) > maxRequestCheckBodyBytes {
		return nil, fmt.Errorf("invalid value %q for --body: file exceeds 10 MiB", path)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("invalid value %q for --body: must contain a JSON object: %v", path, err)
	}
	if body == nil {
		return nil, fmt.Errorf("invalid value %q for --body: must contain a JSON object, not null", path)
	}
	return body, nil
}

type squareBaseURLInspection struct {
	Environment string
	Origin      string
}

func inspectSquareBaseURL(raw string) (squareBaseURLInspection, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if err == nil {
			err = fmt.Errorf("must be an absolute URL with a scheme and host")
		}
		return squareBaseURLInspection{}, err
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return squareBaseURLInspection{}, fmt.Errorf("unsupported URL scheme %q; expected http or https", parsed.Scheme)
	}
	// Only emit an origin reconstructed from parsed URL components. User info,
	// paths, queries, and fragments can carry credentials and never belong in
	// readiness output.
	origin := (&url.URL{Scheme: scheme, Host: parsed.Host}).String()
	inspection := squareBaseURLInspection{Environment: "custom/unknown", Origin: origin}
	canonicalOrigin := scheme == "https" && parsed.User == nil &&
		parsed.RawQuery == "" && !parsed.ForceQuery && parsed.Fragment == "" &&
		(parsed.EscapedPath() == "" || parsed.EscapedPath() == "/") &&
		(parsed.Port() == "" || parsed.Port() == "443")
	if !canonicalOrigin {
		return inspection, nil
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "connect.squareupsandbox.com":
		inspection.Environment = "sandbox"
	case "connect.squareup.com":
		inspection.Environment = "production"
	}
	return inspection, nil
}

func classifySquareBaseURL(raw string) (string, error) {
	inspection, err := inspectSquareBaseURL(raw)
	return inspection.Environment, err
}

type requestOperation struct {
	Endpoint string
	Path     string
}

// findRequestOperation checks the same generated Cobra metadata that powers the
// shipped CLI. This keeps the check offline and avoids a second operation list
// that could drift away from the commands users can actually run.
func findRequestOperation(method, requestPath string) (requestOperation, bool) {
	root := RootCmd()
	var best requestOperation
	bestScore := -1
	var visit func(*cobra.Command)
	visit = func(parent *cobra.Command) {
		for _, child := range parent.Commands() {
			annotations := child.Annotations
			if strings.EqualFold(annotations["pp:method"], method) {
				template := annotations["pp:path"]
				if matched, score := matchOperationPath(template, requestPath); matched && score > bestScore {
					best = requestOperation{Endpoint: annotations["pp:endpoint"], Path: template}
					bestScore = score
				}
			}
			visit(child)
		}
	}
	visit(root)
	return best, bestScore >= 0
}

func matchOperationPath(template, requestPath string) (bool, int) {
	if template == "" {
		return false, 0
	}
	templateParts := strings.Split(strings.Trim(template, "/"), "/")
	requestParts := strings.Split(strings.Trim(requestPath, "/"), "/")
	if len(templateParts) != len(requestParts) {
		return false, 0
	}
	score := 0
	for i := range templateParts {
		part := templateParts[i]
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			if requestParts[i] == "" {
				return false, 0
			}
			continue
		}
		if part != requestParts[i] {
			return false, 0
		}
		score++
	}
	return true, score
}

func containsKey(value any, wanted string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if strings.EqualFold(key, wanted) {
				return true
			}
			if containsKey(child, wanted) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsKey(child, wanted) {
				return true
			}
		}
	}
	return false
}
