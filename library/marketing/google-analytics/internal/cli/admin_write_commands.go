// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-analytics/internal/ga4"
	"github.com/spf13/cobra"
)

// hintAdminError surfaces actionable guidance (in English) when an Admin call
// fails with a role or version problem. The hint lives here, on the CLI side,
// not baked into ga4.APIError.
func hintAdminError(err error) error {
	var ae ga4.APIError
	if errors.As(err, &ae) {
		switch ae.Status {
		case 403:
			return fmt.Errorf("%v\nhint: token carries the scope but the account lacks the required role (Editor or Administrator) on that property/account", err)
		case 404:
			return fmt.Errorf("%v\nhint: check the path, or the resource may only exist in v1alpha (retry with --api v1alpha)", err)
		}
	}
	return err
}

func listRows(raw map[string]any, key string) []map[string]any {
	rows := []map[string]any{}
	if arr, ok := raw[key].([]any); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]any); ok {
				rows = append(rows, m)
			}
		}
	}
	return rows
}
func listTable(raw map[string]any, key string) string { return table(listRows(raw, key)) }

func parseBody(s string) (any, error) {
	var data []byte
	if strings.HasPrefix(s, "@") {
		b, err := os.ReadFile(strings.TrimPrefix(s, "@"))
		if err != nil {
			return nil, err
		}
		data = b
	} else {
		data = []byte(s)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("invalid --body JSON: %w", err)
	}
	return v, nil
}

// ---- key-events ----

func newKeyEventsCmd(flags *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "key-events", Short: "List, create, patch, and delete GA4 key events"}
	c.AddCommand(newKeyEventsListCmd(flags), newKeyEventsCreateCmd(flags), newKeyEventsDeleteCmd(flags), newKeyEventsPatchCmd(flags))
	return c
}
func newKeyEventsListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List key events for a property", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		cl, _, err := flags.newClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.KeyEvents(context.Background(), p)
		if err != nil {
			return hintAdminError(err)
		}
		return output(cmd, flags, raw, listTable(raw, "keyEvents"))
	}}
}
func newKeyEventsCreateCmd(flags *rootFlags) *cobra.Command {
	var event, counting string
	c := &cobra.Command{Use: "create", Short: "Create a key event", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		if event == "" {
			return fmt.Errorf("--event is required")
		}
		body := map[string]any{"eventName": event}
		if counting != "" {
			body["countingMethod"] = counting
		}
		cl, _, err := flags.newWriteClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.CreateKeyEvent(context.Background(), p, body)
		if err != nil {
			return hintAdminError(err)
		}
		return output(cmd, flags, raw, "created key event")
	}}
	c.Flags().StringVar(&event, "event", "", "Event name to mark as a key event")
	c.Flags().StringVar(&counting, "counting", "", "Counting method: ONCE_PER_EVENT|ONCE_PER_SESSION")
	return c
}
func newKeyEventsDeleteCmd(flags *rootFlags) *cobra.Command {
	var name string
	c := &cobra.Command{Use: "delete", Short: "Delete a key event", RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireExplicitYes(cmd, "key-events delete"); err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		cl, _, err := flags.newWriteClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.DeleteKeyEvent(context.Background(), name)
		if err != nil {
			return hintAdminError(err)
		}
		return output(cmd, flags, raw, "deleted "+name)
	}}
	c.Flags().StringVar(&name, "name", "", "Key event resource name, e.g. properties/123/keyEvents/<name>")
	return c
}
func newKeyEventsPatchCmd(flags *rootFlags) *cobra.Command {
	var name, counting string
	c := &cobra.Command{Use: "patch", Short: "Update a key event's counting method", RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		if counting == "" {
			return fmt.Errorf("--counting is required")
		}
		body := map[string]any{"countingMethod": counting}
		cl, _, err := flags.newWriteClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.PatchKeyEvent(context.Background(), name, body, "countingMethod")
		if err != nil {
			return hintAdminError(err)
		}
		return output(cmd, flags, raw, "patched "+name)
	}}
	c.Flags().StringVar(&name, "name", "", "Key event resource name, e.g. properties/123/keyEvents/<name>")
	c.Flags().StringVar(&counting, "counting", "", "Counting method: ONCE_PER_EVENT|ONCE_PER_SESSION")
	return c
}

// ---- custom-dimensions ----

func newCustomDimensionsCmd(flags *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "custom-dimensions", Short: "List, create, and archive GA4 custom dimensions"}
	c.AddCommand(newCustomDimensionsListCmd(flags), newCustomDimensionsCreateCmd(flags), newCustomDimensionsArchiveCmd(flags))
	return c
}
func newCustomDimensionsListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List custom dimensions for a property", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		cl, _, err := flags.newClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.CustomDimensions(context.Background(), p)
		if err != nil {
			return hintAdminError(err)
		}
		return output(cmd, flags, raw, listTable(raw, "customDimensions"))
	}}
}
func newCustomDimensionsCreateCmd(flags *rootFlags) *cobra.Command {
	var parameter, displayName, scope, description string
	var disallowAds bool
	c := &cobra.Command{Use: "create", Short: "Create a custom dimension", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		if parameter == "" {
			return fmt.Errorf("--parameter is required")
		}
		if displayName == "" {
			return fmt.Errorf("--display-name is required")
		}
		body := map[string]any{"parameterName": parameter, "displayName": displayName}
		if scope != "" {
			body["scope"] = scope
		}
		if description != "" {
			body["description"] = description
		}
		if disallowAds {
			body["disallowAdsPersonalization"] = true
		}
		cl, _, err := flags.newWriteClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.CreateCustomDimension(context.Background(), p, body)
		if err != nil {
			return hintAdminError(err)
		}
		return output(cmd, flags, raw, "created custom dimension")
	}}
	c.Flags().StringVar(&parameter, "parameter", "", "Parameter name (without trailing _name)")
	c.Flags().StringVar(&displayName, "display-name", "", "Display name")
	c.Flags().StringVar(&scope, "scope", "", "Scope: EVENT|USER|ITEM")
	c.Flags().StringVar(&description, "description", "", "Optional description")
	c.Flags().BoolVar(&disallowAds, "disallow-ads-personalization", false, "Disallow ads personalization")
	return c
}
func newCustomDimensionsArchiveCmd(flags *rootFlags) *cobra.Command {
	var name string
	c := &cobra.Command{Use: "archive", Short: "Archive a custom dimension (irreversible)", RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireExplicitYes(cmd, "custom-dimensions archive"); err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		cl, _, err := flags.newWriteClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.ArchiveCustomDimension(context.Background(), name)
		if err != nil {
			return hintAdminError(err)
		}
		return output(cmd, flags, raw, "archived "+name)
	}}
	c.Flags().StringVar(&name, "name", "", "Custom dimension resource name, e.g. properties/123/customDimensions/<name>")
	return c
}

// ---- custom-metrics ----

func newCustomMetricsCmd(flags *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "custom-metrics", Short: "List, create, and archive GA4 custom metrics"}
	c.AddCommand(newCustomMetricsListCmd(flags), newCustomMetricsCreateCmd(flags), newCustomMetricsArchiveCmd(flags))
	return c
}
func newCustomMetricsListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List custom metrics for a property", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		cl, _, err := flags.newClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.CustomMetrics(context.Background(), p)
		if err != nil {
			return hintAdminError(err)
		}
		return output(cmd, flags, raw, listTable(raw, "customMetrics"))
	}}
}
func newCustomMetricsCreateCmd(flags *rootFlags) *cobra.Command {
	var parameter, displayName, unit, scope, description string
	c := &cobra.Command{Use: "create", Short: "Create a custom metric", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		if parameter == "" {
			return fmt.Errorf("--parameter is required")
		}
		if displayName == "" {
			return fmt.Errorf("--display-name is required")
		}
		body := map[string]any{"parameterName": parameter, "displayName": displayName}
		if unit != "" {
			body["measurementUnit"] = unit
		}
		if scope != "" {
			body["scope"] = scope
		}
		if description != "" {
			body["description"] = description
		}
		cl, _, err := flags.newWriteClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.CreateCustomMetric(context.Background(), p, body)
		if err != nil {
			return hintAdminError(err)
		}
		return output(cmd, flags, raw, "created custom metric")
	}}
	c.Flags().StringVar(&parameter, "parameter", "", "Parameter name (without trailing _sum/_count suffix)")
	c.Flags().StringVar(&displayName, "display-name", "", "Display name")
	c.Flags().StringVar(&unit, "measurement-unit", "", "STANDARD|CURRENCY|FEET|METERS|KILOMETERS|MILES|MILLISECONDS|SECONDS|MINUTES|HOURS")
	c.Flags().StringVar(&scope, "scope", "", "Scope: EVENT")
	c.Flags().StringVar(&description, "description", "", "Optional description")
	return c
}
func newCustomMetricsArchiveCmd(flags *rootFlags) *cobra.Command {
	var name string
	c := &cobra.Command{Use: "archive", Short: "Archive a custom metric (irreversible)", RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireExplicitYes(cmd, "custom-metrics archive"); err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		cl, _, err := flags.newWriteClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.ArchiveCustomMetric(context.Background(), name)
		if err != nil {
			return hintAdminError(err)
		}
		return output(cmd, flags, raw, "archived "+name)
	}}
	c.Flags().StringVar(&name, "name", "", "Custom metric resource name, e.g. properties/123/customMetrics/<name>")
	return c
}

// ---- data-streams ----

func newDataStreamsCmd(flags *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "data-streams", Short: "List, get, create, patch, and delete GA4 data streams"}
	c.AddCommand(newDataStreamsListCmd(flags), newDataStreamsGetCmd(flags), newDataStreamsCreateCmd(flags), newDataStreamsPatchCmd(flags), newDataStreamsDeleteCmd(flags))
	return c
}
func newDataStreamsListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List data streams for a property", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		cl, _, err := flags.newClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.AdminList(context.Background(), "v1beta", fmt.Sprintf("properties/%s/dataStreams", url.PathEscape(p)))
		if err != nil {
			return hintAdminError(err)
		}
		return output(cmd, flags, raw, listTable(raw, "dataStreams"))
	}}
}
func newDataStreamsGetCmd(flags *rootFlags) *cobra.Command {
	var name string
	c := &cobra.Command{Use: "get", Short: "Get a data stream by resource name", RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		cl, _, err := flags.newClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.AdminCall(context.Background(), "v1beta", "GET", name, nil, "")
		if err != nil {
			return hintAdminError(err)
		}
		return output(cmd, flags, raw, "")
	}}
	c.Flags().StringVar(&name, "name", "", "Data stream resource name, e.g. properties/123/dataStreams/<id>")
	return c
}
func newDataStreamsCreateCmd(flags *rootFlags) *cobra.Command {
	var displayName, typ, uri, packageName, bundleID string
	c := &cobra.Command{Use: "create", Short: "Create a data stream", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		if displayName == "" {
			return fmt.Errorf("--display-name is required")
		}
		if typ == "" {
			return fmt.Errorf("--type is required: WEB_DATA_STREAM|ANDROID_APP_DATA_STREAM|IOS_APP_DATA_STREAM")
		}
		body := map[string]any{"displayName": displayName, "type": typ}
		switch typ {
		case "WEB_DATA_STREAM":
			if uri != "" {
				body["webStreamData"] = map[string]any{"defaultUri": uri}
			}
		case "ANDROID_APP_DATA_STREAM":
			if packageName != "" {
				body["androidAppStreamData"] = map[string]any{"packageName": packageName}
			}
		case "IOS_APP_DATA_STREAM":
			if bundleID != "" {
				body["iosAppStreamData"] = map[string]any{"bundleId": bundleID}
			}
		}
		cl, _, err := flags.newWriteClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.CreateDataStream(context.Background(), p, body)
		if err != nil {
			return hintAdminError(err)
		}
		return output(cmd, flags, raw, "created data stream")
	}}
	c.Flags().StringVar(&displayName, "display-name", "", "Display name")
	c.Flags().StringVar(&typ, "type", "", "WEB_DATA_STREAM|ANDROID_APP_DATA_STREAM|IOS_APP_DATA_STREAM")
	c.Flags().StringVar(&uri, "uri", "", "Web stream default URI")
	c.Flags().StringVar(&packageName, "package-name", "", "Android app package name")
	c.Flags().StringVar(&bundleID, "bundle-id", "", "iOS app bundle id")
	return c
}
func newDataStreamsPatchCmd(flags *rootFlags) *cobra.Command {
	var name, displayName, uri string
	c := &cobra.Command{Use: "patch", Short: "Update a data stream's display name or web URI", RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		body := map[string]any{}
		mask := []string{}
		if cmd.Flags().Changed("display-name") {
			body["displayName"] = displayName
			mask = append(mask, "displayName")
		}
		if cmd.Flags().Changed("uri") {
			body["webStreamData"] = map[string]any{"defaultUri": uri}
			mask = append(mask, "webStreamData.defaultUri")
		}
		if len(mask) == 0 {
			return fmt.Errorf("nothing to patch: pass at least one of --display-name, --uri")
		}
		cl, _, err := flags.newWriteClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.PatchDataStream(context.Background(), name, body, strings.Join(mask, ","))
		if err != nil {
			return hintAdminError(err)
		}
		return output(cmd, flags, raw, "patched "+name)
	}}
	c.Flags().StringVar(&name, "name", "", "Data stream resource name, e.g. properties/123/dataStreams/<id>")
	c.Flags().StringVar(&displayName, "display-name", "", "New display name")
	c.Flags().StringVar(&uri, "uri", "", "New web stream default URI")
	return c
}
func newDataStreamsDeleteCmd(flags *rootFlags) *cobra.Command {
	var name string
	c := &cobra.Command{Use: "delete", Short: "Delete a data stream", RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireExplicitYes(cmd, "data-streams delete"); err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		cl, _, err := flags.newWriteClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.DeleteDataStream(context.Background(), name)
		if err != nil {
			return hintAdminError(err)
		}
		return output(cmd, flags, raw, "deleted "+name)
	}}
	c.Flags().StringVar(&name, "name", "", "Data stream resource name, e.g. properties/123/dataStreams/<id>")
	return c
}

// ---- admin escape hatch ----

func adminListHuman(raw map[string]any) string {
	for _, k := range ga4.AdminListKeys {
		if arr, ok := raw[k].([]any); ok {
			rows := []map[string]any{}
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					rows = append(rows, m)
				}
			}
			return table(rows)
		}
	}
	return ""
}

func newAdminCmd(flags *rootFlags) *cobra.Command {
	var body, updateMask, api string
	var noPaginate bool
	c := &cobra.Command{
		Use:   "admin",
		Short: "Escape-hatch Admin API call: admin <GET|POST|PATCH|PUT|DELETE> <path>",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			path := args[1]
			switch method {
			case "GET", "POST", "PATCH", "PUT", "DELETE":
			default:
				return fmt.Errorf("unknown method %q; expected GET|POST|PATCH|PUT|DELETE", args[0])
			}
			if !ga4.ValidAdminAPI(api) {
				return fmt.Errorf("unknown --api %q; expected v1beta or v1alpha", api)
			}
			if method == "GET" {
				cl, _, err := flags.newClient()
				if err != nil {
					return err
				}
				if !noPaginate {
					raw, _, err := cl.AdminList(context.Background(), api, path)
					if err != nil {
						return hintAdminError(err)
					}
					return output(cmd, flags, raw, adminListHuman(raw))
				}
				raw, _, err := cl.AdminCall(context.Background(), api, "GET", path, nil, "")
				if err != nil {
					return hintAdminError(err)
				}
				return output(cmd, flags, raw, "")
			}
			if method == "DELETE" {
				if err := requireExplicitYes(cmd, "admin DELETE "+path); err != nil {
					return err
				}
			}
			cl, _, err := flags.newWriteClient()
			if err != nil {
				return err
			}
			var bd any
			if body != "" {
				bd, err = parseBody(body)
				if err != nil {
					return err
				}
			}
			raw, _, err := cl.AdminCall(context.Background(), api, method, path, bd, updateMask)
			if err != nil {
				return hintAdminError(err)
			}
			return output(cmd, flags, raw, "")
		},
	}
	c.Flags().StringVar(&body, "body", "", "JSON body (inline string or @file.json)")
	c.Flags().StringVar(&updateMask, "update-mask", "", "Comma-separated updateMask fields")
	c.Flags().StringVar(&api, "api", "v1beta", "Admin API version: v1beta|v1alpha")
	c.Flags().BoolVar(&noPaginate, "no-paginate", false, "Disable pagination for GET")
	return c
}
