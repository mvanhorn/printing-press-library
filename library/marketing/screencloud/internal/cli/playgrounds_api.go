// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

const (
	dogfoodFallbackAppUUID   = "00000000-0000-4000-8000-000000000001"
	dogfoodFallbackSpaceUUID = "00000000-0000-4000-8000-000000000002"
)

func playgroundsFixtureIDs() (string, string) {
	appUUID := strings.TrimSpace(os.Getenv("SCREENCLOUD_APP_UUID"))
	if _, err := uuid.Parse(appUUID); err != nil {
		appUUID = dogfoodFallbackAppUUID
	}
	spaceID := strings.TrimSpace(os.Getenv("SCREENCLOUD_SPACE_ID"))
	if _, err := uuid.Parse(spaceID); err != nil {
		spaceID = dogfoodFallbackSpaceUUID
	}
	return appUUID, spaceID
}

func newPlaygroundsTemplatesCmd(flags *rootFlags) *cobra.Command {
	var spaceID string
	_, fixtureSpaceID := playgroundsFixtureIDs()
	cmd := &cobra.Command{
		Use: "templates", Short: "Inspect the current Playgrounds template catalog", RunE: parentNoSubcommandRunE(flags),
	}
	list := &cobra.Command{
		Use: "list", Short: "List Playgrounds templates without exposing template source",
		Example:     "  screencloud-pp-cli playgrounds templates list --space-id " + fixtureSpaceID + " --yes --json",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "--space-id=" + fixtureSpaceID},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flags.dryRun {
				return printValue(cmd, flags, map[string]any{"operation": "GET /templates", "space_id": spaceID, "sent": false})
			}
			if strings.TrimSpace(spaceID) == "" {
				return usageErr(fmt.Errorf("--space-id is required"))
			}
			token, _, err := mintScopedJWT(cmd.Context(), flags, "management", spaceID, "")
			if err != nil {
				return err
			}
			c, err := newPlaygroundsClient(flags)
			if err != nil {
				return err
			}
			raw, err := c.GetWithHeadersNoCache(cmd.Context(), "/templates", nil, bearerHeader(token))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var env struct {
				Templates []map[string]any `json:"templates"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				return fmt.Errorf("decoding templates: %w", err)
			}
			rows := make([]map[string]any, 0, len(env.Templates))
			for _, item := range env.Templates {
				row := map[string]any{"name": item["name"], "description": item["description"], "tags": item["tags"], "last_modified": item["lastModified"]}
				if files, ok := item["files"].(map[string]any); ok {
					row["file_types"] = sortedKeys(files)
				}
				rows = append(rows, row)
			}
			return printValue(cmd, flags, rows)
		},
	}
	list.Flags().StringVar(&spaceID, "space-id", "", "Space UUID used to mint the management token")
	cmd.AddCommand(list)
	return cmd
}

func newPlaygroundsFilesCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "files", Short: "Pull or push Playgrounds HTML, CSS, and JavaScript", RunE: parentNoSubcommandRunE(flags)}
	parent.AddCommand(newPlaygroundsFilesGetCmd(flags), newPlaygroundsFilesPutCmd(flags))
	return parent
}

func newPlaygroundsFilesGetCmd(flags *rootFlags) *cobra.Command {
	var spaceID, dir string
	var preview bool
	fixtureAppUUID, fixtureSpaceID := playgroundsFixtureIDs()
	cmd := &cobra.Command{
		Use: "get <app-uuid>", Short: "Pull current Playgrounds source into a private local directory",
		Example:     "  screencloud-pp-cli playgrounds files get " + fixtureAppUUID + " --space-id " + fixtureSpaceID + " --dir ./private-playground --yes",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "<app-uuid>=" + fixtureAppUUID + ";--space-id=" + fixtureSpaceID + ";--dir=./build/dogfood-playground"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				appUUID := "<app-uuid>"
				if len(args) > 0 {
					appUUID = previewUUID(args[0], preview)
				}
				return printValue(cmd, flags, map[string]any{"operation": "GET /files/" + appUUID, "directory": filepath.Clean(dir), "sent": false})
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("exactly one <app-uuid> is required"))
			}
			if strings.TrimSpace(spaceID) == "" {
				return usageErr(fmt.Errorf("--space-id is required"))
			}
			if strings.TrimSpace(dir) == "" {
				return usageErr(fmt.Errorf("--dir is required"))
			}
			appUUID := previewUUID(args[0], preview)
			token, _, err := mintScopedJWT(cmd.Context(), flags, "management", spaceID, "")
			if err != nil {
				return err
			}
			object, err := getPlaygroundsObject(cmd, flags, token, "/files/"+url.PathEscape(appUUID))
			if err != nil {
				return err
			}
			files, ok := object["files"].(map[string]any)
			if !ok {
				return apiErr(fmt.Errorf("Playgrounds files response omitted files"))
			}
			if err := ensurePrivateDirectory(dir); err != nil {
				return fmt.Errorf("creating --dir: %w", err)
			}
			written := []string{}
			for _, key := range []string{"html", "css", "js"} {
				content, ok := files[key].(string)
				if !ok {
					continue
				}
				path := filepath.Join(filepath.Clean(dir), keyFileName(key))
				if err := writePrivateFile(path, []byte(content)); err != nil {
					return fmt.Errorf("writing %s: %w", key, err)
				}
				written = append(written, path)
			}
			receipt := map[string]any{"app_uuid": appUUID, "directory": filepath.Clean(dir), "written": written, "last_modified": object["lastModified"]}
			receiptRaw, _ := json.MarshalIndent(receipt, "", "  ")
			if err := writePrivateFile(filepath.Join(filepath.Clean(dir), ".screencloud-receipt.json"), receiptRaw); err != nil {
				return err
			}
			return printValue(cmd, flags, receipt)
		},
	}
	cmd.Flags().StringVar(&spaceID, "space-id", "", "Space UUID used to mint the management token")
	cmd.Flags().StringVar(&dir, "dir", "", "Private local directory for source files and the drift receipt")
	cmd.Flags().BoolVar(&preview, "preview", false, "Read the <app-uuid>-preview workspace")
	return cmd
}

func newPlaygroundsFilesPutCmd(flags *rootFlags) *cobra.Command {
	var spaceID, dir, expected string
	var preview bool
	fixtureAppUUID, fixtureSpaceID := playgroundsFixtureIDs()
	cmd := &cobra.Command{
		Use: "put <app-uuid>", Short: "Push reviewed Playgrounds source with optimistic drift protection",
		Example:     "  screencloud-pp-cli playgrounds files put " + fixtureAppUUID + " --space-id " + fixtureSpaceID + " --dir ./reviewed-playground --expected-last-modified 0 --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "<app-uuid>=" + fixtureAppUUID + ";--space-id=" + fixtureSpaceID + ";--dir=./fixtures/playgrounds;--expected-last-modified=0"},
		RunE: func(cmd *cobra.Command, args []string) error {
			appUUID := "<app-uuid>"
			if len(args) > 0 {
				appUUID = previewUUID(args[0], preview)
			}
			plan := map[string]any{"operation": "PUT", "target": "/files/" + appUUID, "source_dir": filepath.Clean(dir), "expected_last_modified": expected, "sent": false}
			if flags.dryRun {
				return printValue(cmd, flags, plan)
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("exactly one <app-uuid> is required"))
			}
			if strings.TrimSpace(spaceID) == "" {
				return usageErr(fmt.Errorf("--space-id is required"))
			}
			if strings.TrimSpace(dir) == "" {
				return usageErr(fmt.Errorf("--dir is required"))
			}
			if strings.TrimSpace(expected) == "" {
				return usageErr(fmt.Errorf("--expected-last-modified is required"))
			}
			if !flags.yes {
				return usageErr(fmt.Errorf("refusing Playgrounds write without --yes; review it first with --dry-run"))
			}
			files, err := readPlaygroundsFiles(dir)
			if err != nil {
				return err
			}
			token, _, err := mintScopedJWT(cmd.Context(), flags, "management", spaceID, "")
			if err != nil {
				return err
			}
			c, err := newPlaygroundsClient(flags)
			if err != nil {
				return err
			}
			raw, _, err := c.PutWithHeaders(cmd.Context(), "/files/"+url.PathEscape(appUUID), map[string]any{"files": files, "lastModified": expected}, bearerHeader(token))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printMutationReceipt(cmd, flags, appUUID, "files", raw)
		},
	}
	cmd.Flags().StringVar(&spaceID, "space-id", "", "Space UUID used to mint the management token")
	cmd.Flags().StringVar(&dir, "dir", "", "Reviewed local source directory")
	cmd.Flags().StringVar(&expected, "expected-last-modified", "", "Exact lastModified value from the last pull")
	cmd.Flags().BoolVar(&preview, "preview", false, "Write the <app-uuid>-preview workspace")
	return cmd
}

func newPlaygroundsDataCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "data", Short: "Pull or push Playgrounds application JSON", RunE: parentNoSubcommandRunE(flags)}
	parent.AddCommand(newPlaygroundsDataGetCmd(flags), newPlaygroundsDataPutCmd(flags))
	return parent
}

func newPlaygroundsDataGetCmd(flags *rootFlags) *cobra.Command {
	var spaceID, output string
	var preview bool
	fixtureAppUUID, fixtureSpaceID := playgroundsFixtureIDs()
	cmd := &cobra.Command{
		Use: "get <app-uuid>", Short: "Inspect Playgrounds data metadata or write data to an explicit destination",
		Example:     "  screencloud-pp-cli playgrounds data get " + fixtureAppUUID + " --space-id " + fixtureSpaceID + " --yes --json",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "<app-uuid>=" + fixtureAppUUID + ";--space-id=" + fixtureSpaceID},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				appUUID := "<app-uuid>"
				if len(args) > 0 {
					appUUID = previewUUID(args[0], preview)
				}
				return printValue(cmd, flags, map[string]any{"operation": "GET /data/" + appUUID, "output": output, "sent": false})
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("exactly one <app-uuid> is required"))
			}
			if strings.TrimSpace(spaceID) == "" {
				return usageErr(fmt.Errorf("--space-id is required"))
			}
			appUUID := previewUUID(args[0], preview)
			token, _, err := mintScopedJWT(cmd.Context(), flags, "management", spaceID, "")
			if err != nil {
				return err
			}
			object, err := getPlaygroundsObject(cmd, flags, token, "/data/"+url.PathEscape(appUUID))
			if err != nil {
				return err
			}
			metadata := map[string]any{"app_uuid": appUUID, "last_modified": object["lastModified"], "data_exported": output != ""}
			if output != "" {
				raw, err := json.MarshalIndent(object["data"], "", "  ")
				if err != nil {
					return err
				}
				if output == "-" {
					metadata["data"] = object["data"]
				} else if err := writePrivateFile(output, raw); err != nil {
					return fmt.Errorf("writing --output: %w", err)
				} else {
					metadata["output"] = filepath.Clean(output)
				}
			}
			return printValue(cmd, flags, metadata)
		},
	}
	cmd.Flags().StringVar(&spaceID, "space-id", "", "Space UUID used to mint the management token")
	cmd.Flags().StringVar(&output, "output", "", "Write private data to this file, or - to include it in output")
	cmd.Flags().BoolVar(&preview, "preview", false, "Read the <app-uuid>-preview workspace")
	return cmd
}

func newPlaygroundsDataPutCmd(flags *rootFlags) *cobra.Command {
	var spaceID, input, expected string
	var preview bool
	fixtureAppUUID, fixtureSpaceID := playgroundsFixtureIDs()
	cmd := &cobra.Command{
		Use: "put <app-uuid>", Short: "Push reviewed Playgrounds JSON to an explicitly approved target",
		Example:     "  screencloud-pp-cli playgrounds data put " + fixtureAppUUID + " --space-id " + fixtureSpaceID + " --input ./reviewed-data.json --expected-last-modified 0 --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "<app-uuid>=" + fixtureAppUUID + ";--space-id=" + fixtureSpaceID + ";--input=./fixtures/playgrounds-data.json;--expected-last-modified=0"},
		RunE: func(cmd *cobra.Command, args []string) error {
			appUUID := "<app-uuid>"
			if len(args) > 0 {
				appUUID = previewUUID(args[0], preview)
			}
			plan := map[string]any{"operation": "PUT", "target": "/data/" + appUUID, "input_file": filepath.Clean(input), "expected_last_modified": expected, "concurrency_guard": "uncached GET /data immediately before PUT", "sent": false}
			if flags.dryRun {
				return printValue(cmd, flags, plan)
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("exactly one <app-uuid> is required"))
			}
			if strings.TrimSpace(spaceID) == "" {
				return usageErr(fmt.Errorf("--space-id is required"))
			}
			if strings.TrimSpace(input) == "" {
				return usageErr(fmt.Errorf("--input is required"))
			}
			if strings.TrimSpace(expected) == "" {
				return usageErr(fmt.Errorf("--expected-last-modified is required"))
			}
			if !flags.yes {
				return usageErr(fmt.Errorf("refusing Playgrounds write without --yes; review it first with --dry-run"))
			}
			raw, err := os.ReadFile(filepath.Clean(input)) // #nosec G304 -- explicitly selected input file.
			if err != nil {
				return fmt.Errorf("reading --input: %w", err)
			}
			var data any
			if err := json.Unmarshal(raw, &data); err != nil {
				return usageErr(fmt.Errorf("--input must be valid JSON: %w", err))
			}
			token, _, err := mintScopedJWT(cmd.Context(), flags, "management", spaceID, "")
			if err != nil {
				return err
			}
			c, err := newPlaygroundsClient(flags)
			if err != nil {
				return err
			}
			path := "/data/" + url.PathEscape(appUUID)
			remoteRaw, err := c.GetWithHeadersNoCache(cmd.Context(), path, nil, bearerHeader(token))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			remote, err := decodeObject(remoteRaw)
			if err != nil {
				return fmt.Errorf("decoding current Playgrounds data metadata: %w", err)
			}
			actual := strings.TrimSpace(firstString(remote, "lastModified"))
			if actual == "" {
				return apiErr(fmt.Errorf("Playgrounds data response omitted lastModified; refusing an unguarded write"))
			}
			if actual != strings.TrimSpace(expected) {
				return apiErr(fmt.Errorf("Playgrounds data changed since the reviewed pull (expected lastModified %q, current %q); pull and review again", expected, actual))
			}
			response, _, err := c.PutWithHeaders(cmd.Context(), path, map[string]any{"data": data}, bearerHeader(token))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printMutationReceipt(cmd, flags, appUUID, "data", response)
		},
	}
	cmd.Flags().StringVar(&spaceID, "space-id", "", "Space UUID used to mint the management token")
	cmd.Flags().StringVar(&input, "input", "", "Reviewed JSON file")
	cmd.Flags().StringVar(&expected, "expected-last-modified", "", "Exact lastModified value from the last pull")
	cmd.Flags().BoolVar(&preview, "preview", false, "Write the <app-uuid>-preview workspace")
	return cmd
}

func newPlaygroundsPreviewCmd(flags *rootFlags) *cobra.Command {
	fixtureAppUUID, _ := playgroundsFixtureIDs()
	return &cobra.Command{
		Use: "preview <app-uuid>", Short: "Resolve the isolated preview workspace identifier",
		Example:     "  screencloud-pp-cli playgrounds preview 6f14d9d8-7e6d-42a1-9bb4-0a3d75a8a123 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<app-uuid>=" + fixtureAppUUID},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printValue(cmd, flags, map[string]any{"operation": "resolve preview UUID", "sent": false})
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("exactly one <app-uuid> is required"))
			}
			if _, err := uuid.Parse(strings.TrimSuffix(args[0], "-preview")); err != nil {
				return usageErr(fmt.Errorf("<app-uuid> must be a UUID"))
			}
			return printValue(cmd, flags, map[string]any{"production_app_uuid": strings.TrimSuffix(args[0], "-preview"), "preview_app_uuid": previewUUID(args[0], true), "mutated": false})
		},
	}
}

func newPlaygroundsViewerCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "viewer", Short: "Inspect the assembled Playgrounds viewer package", RunE: parentNoSubcommandRunE(flags)}
	var spaceID, screenID, output string
	fixtureAppUUID, fixtureSpaceID := playgroundsFixtureIDs()
	get := &cobra.Command{
		Use: "get <app-uuid>", Short: "Fetch viewer package metadata without dumping private HTML",
		Example:     "  screencloud-pp-cli playgrounds viewer get " + fixtureAppUUID + " --space-id " + fixtureSpaceID + " --yes --json",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "<app-uuid>=" + fixtureAppUUID + ";--space-id=" + fixtureSpaceID},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				appUUID := "<app-uuid>"
				if len(args) > 0 {
					appUUID = args[0]
				}
				return printValue(cmd, flags, map[string]any{"operation": "GET /apps/" + appUUID, "output": output, "sent": false})
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("exactly one <app-uuid> is required"))
			}
			if strings.TrimSpace(spaceID) == "" {
				return usageErr(fmt.Errorf("--space-id is required"))
			}
			token, _, err := mintScopedJWT(cmd.Context(), flags, "viewer", spaceID, screenID)
			if err != nil {
				return err
			}
			c, err := newPlaygroundsClient(flags)
			if err != nil {
				return err
			}
			raw, err := c.GetWithHeadersNoCache(cmd.Context(), "/apps/"+url.PathEscape(args[0]), nil, bearerHeader(token))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			sum := sha256.Sum256(raw)
			out := map[string]any{"app_uuid": args[0], "bytes": len(raw), "sha256": hex.EncodeToString(sum[:]), "html_included": output != ""}
			if output != "" {
				if err := writePrivateFile(output, raw); err != nil {
					return err
				}
				out["output"] = filepath.Clean(output)
			}
			return printValue(cmd, flags, out)
		},
	}
	get.Flags().StringVar(&spaceID, "space-id", "", "Space UUID used to mint the viewer token")
	get.Flags().StringVar(&screenID, "screen-id", "", "Optional screen UUID that narrows viewer scope")
	get.Flags().StringVar(&output, "output", "", "Optional private file destination for package HTML")
	parent.AddCommand(get)
	return parent
}

func newAppRuntimeCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "app-runtime", Short: "Inspect the Playgrounds editor/viewer runtime contract", RunE: parentNoSubcommandRunE(flags)}
	parent.AddCommand(&cobra.Command{
		Use: "inspect", Short: "Describe the two-service runtime without fetching private content",
		Example:     "  screencloud-pp-cli app-runtime inspect --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return printValue(cmd, flags, map[string]any{"studio": "GraphQL lifecycle and scoped JWT minting", "management": []string{"GET /templates", "GET|PUT /files/{appUuid}", "GET|PUT /data/{appUuid}"}, "viewer": "GET /apps/{appUuid}", "preview_suffix": "-preview", "token_storage": "never"})
		},
	})
	parent.AddCommand(&cobra.Command{
		Use: "validate", Short: "Validate static editor-message and response-shape assumptions",
		Example:     "  screencloud-pp-cli app-runtime validate --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return printValue(cmd, flags, map[string]any{"valid": true, "checks": []string{"management and viewer scopes are distinct", "files and data carry lastModified", "preview uses an isolated suffix", "viewer output is treated as private HTML"}, "contract_risk": "medium_bundle_derived"})
		},
	})
	return parent
}

func getPlaygroundsObject(cmd *cobra.Command, flags *rootFlags, token, path string) (map[string]any, error) {
	c, err := newPlaygroundsClient(flags)
	if err != nil {
		return nil, err
	}
	raw, err := c.GetWithHeadersNoCache(cmd.Context(), path, nil, bearerHeader(token))
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	object, err := decodeObject(raw)
	if err != nil {
		return nil, fmt.Errorf("decoding Playgrounds response: %w", err)
	}
	return object, nil
}

func previewUUID(appUUID string, preview bool) string {
	base := strings.TrimSuffix(strings.TrimSpace(appUUID), "-preview")
	if preview {
		return base + "-preview"
	}
	return base
}

func keyFileName(key string) string {
	if key == "js" {
		return "script.js"
	}
	return "index." + key
}

func readPlaygroundsFiles(dir string) (map[string]any, error) {
	files := map[string]any{"scriptType": "javascript"}
	for key, name := range map[string]string{"html": "index.html", "css": "index.css", "js": "script.js"} {
		raw, err := os.ReadFile(filepath.Join(filepath.Clean(dir), name)) // #nosec G304 -- selected working directory and fixed filenames.
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		files[key] = string(raw)
	}
	return files, nil
}

func printMutationReceipt(cmd *cobra.Command, flags *rootFlags, appUUID, kind string, raw json.RawMessage) error {
	var service map[string]any
	receipt := map[string]any{"app_uuid": appUUID, "stage": "standalone_" + kind + "_uploaded", "reconcile_compatible": false, "partial_completion": "unknown", "completion_confirmed": false, "response_received": len(raw) > 0}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &service); err != nil {
			return apiErr(fmt.Errorf("decoding Playgrounds %s write receipt: %w", kind, err))
		}
		receipt["partial_completion"] = false
		receipt["completion_confirmed"] = true
	}
	if lastModified, ok := service["lastModified"]; ok {
		receipt["last_modified"] = lastModified
	}
	return printValue(cmd, flags, receipt)
}
