// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored, not generator output. Manifest absorbed rows 2, 6, 7, 11.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/client"
	"github.com/spf13/cobra"
)

var rawMethods = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}

var rawFirstSegment = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// normalizeRawPath returns the /proxy/... path for a user-supplied console path.
// The prefix is what makes this a console escape hatch and not a generic proxy: a
// traversal would let the connector or gateway serve something outside the app
// proxies. Percent-decoding runs repeatedly because an upstream that decodes once
// more would turn %252e into a live dot segment.
func normalizeRawPath(raw string) (string, url.Values, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil, errors.New("path is empty")
	}
	if strings.Contains(raw, "://") || strings.HasPrefix(raw, "//") {
		return "", nil, fmt.Errorf("path %q must be relative to the console, not a URL", raw)
	}
	if strings.ContainsAny(raw, "\\#\x00") {
		return "", nil, fmt.Errorf("path %q contains a forbidden character", raw)
	}
	pathPart, queryPart, _ := strings.Cut(raw, "?")
	query, err := url.ParseQuery(queryPart)
	if err != nil {
		return "", nil, fmt.Errorf("path %q has an invalid query: %w", raw, err)
	}
	decoded := pathPart
	for i := 0; i < 4; i++ {
		next, err := url.PathUnescape(decoded)
		if err != nil {
			return "", nil, fmt.Errorf("path %q has invalid escaping", raw)
		}
		if next == decoded {
			break
		}
		decoded = next
	}
	if strings.ContainsAny(decoded, "\\\x00") {
		return "", nil, fmt.Errorf("path %q decodes to a forbidden character", raw)
	}
	// Shared with the client's own refusal so this cannot accept what it would reject,
	// and so the error here can name the raw argument.
	if client.PathHasTraversal(pathPart) {
		return "", nil, fmt.Errorf("path %q escapes the console prefix", raw)
	}
	for _, segment := range strings.Split(strings.Trim(decoded, "/"), "/") {
		if segment == ".." || segment == "." {
			return "", nil, fmt.Errorf("path %q escapes the console prefix", raw)
		}
	}
	if strings.Contains(strings.Trim(decoded, "/"), "//") {
		return "", nil, fmt.Errorf("path %q has an empty segment", raw)
	}
	clean := strings.TrimPrefix(strings.TrimPrefix(pathPart, "/"), "proxy/")
	first, _, _ := strings.Cut(clean, "/")
	if !rawFirstSegment.MatchString(first) || !strings.Contains(clean, "/") {
		return "", nil, fmt.Errorf("path %q must name a console application, e.g. network/api/s/default/stat/health", raw)
	}
	if first == "v1" {
		return "", nil, fmt.Errorf("path %q is a Site Manager path; raw only reaches console applications under /proxy", raw)
	}
	return "/proxy/" + clean, query, nil
}

// parseRawArgs accepts "raw <path>" (GET) and "raw <METHOD> <path>".
func parseRawArgs(args []string) (method, path string, err error) {
	switch len(args) {
	case 1:
		return "GET", args[0], nil
	case 2:
		method = strings.ToUpper(args[0])
		if !rawMethods[method] {
			return "", "", fmt.Errorf("method %q must be one of GET, POST, PUT, PATCH, DELETE", args[0])
		}
		return method, args[1], nil
	}
	return "", "", errors.New("expected [METHOD] <path>")
}

func newRawCmd(flags *rootFlags) *cobra.Command {
	var confirm bool
	var bodyJSON, bodyFile string
	var queryPairs []string
	cmd := &cobra.Command{
		Use:   "raw [method] [path]",
		Short: "Call any console path under /proxy, reads by default, writes only with --confirm",
		Long: `Call a path on the selected console through the connector or the local gateway.

This is the home for unofficial controller endpoints the published specs do not
cover. They are unverified: whether a key can reach them depends on the console.

Reads run directly. Any other method prints a preview and sends nothing until you
pass --confirm. Paths that leave the console's /proxy prefix are refused.`,
		Example: strings.Trim(`
  unifi-pp-cli raw network/api/s/default/stat/health --console EXAMPLE-CONSOLE-ID --json
  unifi-pp-cli raw GET network/v2/api/site/default/trafficroutes --gateway 192.168.1.1
  unifi-pp-cli raw POST network/api/s/default/cmd/stamgr --body '{"cmd":"block-sta","mac":"02:00:00:00:00:01"}'
  unifi-pp-cli raw POST network/api/s/default/cmd/stamgr --body '{"cmd":"block-sta","mac":"02:00:00:00:00:01"}' --confirm`, "\n"),
		Annotations: map[string]string{
			"pp:data-source":             "live",
			"mcp:read-only-unless-flags": "confirm",
			"mcp:read-flags":             "body-file",
			"pp:preview-happy-path":      "true",
			"pp:happy-args":              "path=network/integration/v1/info",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && len(args) == 0 {
				return writeDryRun(cmd.OutOrStdout(), flags, "raw")
			}
			method, rawPath, err := parseRawArgs(args)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			path, query, err := normalizeRawPath(rawPath)
			if err != nil {
				return usageErr(err)
			}
			for _, pair := range queryPairs {
				key, value, ok := strings.Cut(pair, "=")
				if !ok || key == "" {
					return usageErr(fmt.Errorf("--query %q must be key=value", pair))
				}
				query.Add(key, value)
			}
			if bodyJSON != "" && bodyFile != "" {
				return usageErr(errors.New("use --body or --body-file, not both"))
			}
			var body any
			if bodyFile != "" {
				data, err := os.ReadFile(bodyFile) // #nosec G304 -- operator-named --body-file; mcp:read-flags refuses it from MCP callers
				if err != nil {
					return usageErr(fmt.Errorf("--body-file: %w", err))
				}
				bodyJSON = string(data)
			}
			if bodyJSON != "" {
				if method == "GET" {
					return usageErr(errors.New("--body needs a non-GET method"))
				}
				if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
					return usageErr(fmt.Errorf("--body is not valid JSON: %w", err))
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			params := map[string]string{}
			for key, values := range query {
				if len(values) > 0 {
					params[key] = values[len(values)-1]
				}
			}
			if flags.dryRun || (method != "GET" && !confirm) {
				target, kind, err := c.PreviewTarget(ctx, path)
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				preview := map[string]any{"sent": false, "method": method, "url": target, "transport": string(kind)}
				if len(params) > 0 {
					preview["query"] = params
				}
				if body != nil {
					// Previews land in transcripts; change apply already scrubs its own.
					preview["body"] = client.ScrubSecrets(body)
				}
				if flags.dryRun {
					preview["dry_run"] = true
				} else {
					preview["preview"] = true
					preview["confirmation_required"] = true
					fmt.Fprintln(cmd.ErrOrStderr(), "preview only, nothing sent: pass --confirm to send this request")
				}
				return printJSONFiltered(cmd.OutOrStdout(), preview, flags)
			}
			data, _, err := c.Do(ctx, method, path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			var decoded any
			if json.Unmarshal(data, &decoded) != nil {
				_, err := cmd.OutOrStdout().Write(append(data, '\n'))
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), decoded, flags)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually send a non-GET request; without it a preview is printed")
	cmd.Flags().StringVar(&bodyJSON, "body", "", "JSON request body for non-GET methods")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Path to a JSON request body for non-GET methods")
	cmd.Flags().StringArrayVar(&queryPairs, "query", nil, "Query parameter as key=value; repeatable")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newRawCmd(flags))
	})
}
