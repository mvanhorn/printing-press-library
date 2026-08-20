// Hand-authored absorbed read commands for StackAdapt (advertisers, campaigns,
// campaign-groups, ads, segments, account). No generated header: preserved
// across regen. Read-only GraphQL queries.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// saListView is the JSON envelope for a list command.
type saListView struct {
	Resource string            `json:"resource"`
	Count    int               `json:"count"`
	Items    []json.RawMessage `json:"items"`
}

// runConnectionList executes a connection query, extracts nodes, and emits
// them. It honors --data-source: "local" reads only the synced store, "live"
// queries the API only, and "auto" (the default) queries live and falls back
// to the store when the API is unreachable.
func runConnectionList(cmd *cobra.Command, flags *rootFlags, resource, root, query string, vars map[string]any) error {
	storeType := resourceStoreType(resource)
	limit := 100
	if n, ok := vars["n"].(int); ok && n > 0 {
		limit = n
	}

	if flags.dataSource == "local" {
		if storeType == "" {
			return fmt.Errorf("--data-source local is not available for %s", resource)
		}
		return listFromLocalStore(cmd, flags, resource, storeType, limit)
	}

	data, err := runQuery(cmd.Context(), flags, query, vars)
	if err != nil {
		if flags.dataSource == "auto" && storeType != "" && tryLocalFallback(cmd, flags, resource, storeType, limit, err) {
			return nil
		}
		return err
	}
	nodes, _, err := nodesAt(data, root)
	if err != nil {
		return err
	}
	return emitView(cmd, flags, saListView{Resource: resource, Count: len(nodes), Items: nodes})
}

// findNodeByID lists a connection and returns the first node whose "id" matches.
func findNodeByID(cmd *cobra.Command, flags *rootFlags, root, query string, vars map[string]any, id string) (json.RawMessage, error) {
	data, err := runQuery(cmd.Context(), flags, query, vars)
	if err != nil {
		return nil, err
	}
	nodes, _, err := nodesAt(data, root)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		var obj struct {
			ID json.RawMessage `json:"id"`
		}
		if json.Unmarshal(n, &obj) == nil {
			if strings.Trim(string(obj.ID), `"`) == id {
				return n, nil
			}
		}
	}
	return nil, notFoundErr(fmt.Errorf("no %s with id %q", root, id))
}

const advertiserFields = `id name description isArchived`
const campaignFields = `id name channelType goalType isArchived isDraft createdAt budgetRollover campaignStatus { state status } campaignGroup { id name } advertiser { id name }`
const campaignGroupFields = `id name budgetType revenueType revenuePricing timezone createdAt isArchived budgetRollover advertiser { id name }`
const adFields = `id name brandname channelType clickUrl creativeSize paused isArchived isDraft isRejected campaign { id name }`
const segmentFields = `id name description active size createdAt`

func newAdvertisersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "advertisers", Short: "List and inspect StackAdapt advertisers", Annotations: map[string]string{"mcp:read-only": "true"}}
	cmd.RunE = parentNoSubcommandRunE(flags)

	var limit int
	list := &cobra.Command{
		Use: "list", Short: "List advertisers under this token, with total count (use --limit to cap)", Example: "  stackadapt-pp-cli advertisers list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "advertisers list", "would list advertisers")
			}
			q := fmt.Sprintf(`query($n:Int){ advertisers(first:$n){ totalCount nodes { %s } } }`, advertiserFields)
			return runConnectionList(cmd, flags, "advertisers", "advertisers", q, map[string]any{"n": limit})
		},
	}
	list.Flags().IntVar(&limit, "limit", 100, "Max advertisers to return")
	cmd.AddCommand(list)

	get := &cobra.Command{
		Use: "get <advertiser-id>", Short: "Get one advertiser by ID", Example: "  stackadapt-pp-cli advertisers get 123 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "advertisers get", "would get one advertiser")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("advertiser id is required"))
			}
			q := fmt.Sprintf(`query{ advertisers(first:500){ nodes { %s } } }`, advertiserFields)
			node, err := findNodeByID(cmd, flags, "advertisers", q, nil, args[0])
			if err != nil {
				return err
			}
			return emitView(cmd, flags, node)
		},
	}
	cmd.AddCommand(get)
	return cmd
}

func newCampaignsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "campaigns", Short: "List and inspect StackAdapt campaigns", Annotations: map[string]string{"mcp:read-only": "true"}}
	cmd.RunE = parentNoSubcommandRunE(flags)

	var limit int
	list := &cobra.Command{
		Use: "list", Short: "List campaigns with status and budget, total count included (use --limit to cap)", Example: "  stackadapt-pp-cli campaigns list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "campaigns list", "would list campaigns")
			}
			q := fmt.Sprintf(`query($n:Int){ campaigns(first:$n){ totalCount nodes { %s } } }`, campaignFields)
			return runConnectionList(cmd, flags, "campaigns", "campaigns", q, map[string]any{"n": limit})
		},
	}
	list.Flags().IntVar(&limit, "limit", 100, "Max campaigns to return")
	cmd.AddCommand(list)

	get := &cobra.Command{
		Use: "get <campaign-id>", Short: "Get one campaign by ID", Example: "  stackadapt-pp-cli campaigns get 456 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "campaigns get", "would get one campaign")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("campaign id is required"))
			}
			q := fmt.Sprintf(`query{ campaigns(first:500){ nodes { %s } } }`, campaignFields)
			node, err := findNodeByID(cmd, flags, "campaigns", q, nil, args[0])
			if err != nil {
				return err
			}
			return emitView(cmd, flags, node)
		},
	}
	cmd.AddCommand(get)
	return cmd
}

func newCampaignGroupsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "campaign-groups", Short: "List and inspect StackAdapt campaign groups", Annotations: map[string]string{"mcp:read-only": "true"}}
	cmd.RunE = parentNoSubcommandRunE(flags)

	var limit int
	list := &cobra.Command{
		Use: "list", Short: "List campaign groups with total count (use --limit to cap)", Example: "  stackadapt-pp-cli campaign-groups list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "campaign-groups list", "would list campaign groups")
			}
			q := fmt.Sprintf(`query($n:Int){ campaignGroups(first:$n){ totalCount nodes { %s } } }`, campaignGroupFields)
			return runConnectionList(cmd, flags, "campaign-groups", "campaignGroups", q, map[string]any{"n": limit})
		},
	}
	list.Flags().IntVar(&limit, "limit", 100, "Max campaign groups to return")
	cmd.AddCommand(list)

	get := &cobra.Command{
		Use: "get <group-id>", Short: "Get one campaign group by ID", Example: "  stackadapt-pp-cli campaign-groups get 789 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "campaign-groups get", "would get one campaign group")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("campaign group id is required"))
			}
			q := fmt.Sprintf(`query{ campaignGroups(first:500){ nodes { %s } } }`, campaignGroupFields)
			node, err := findNodeByID(cmd, flags, "campaignGroups", q, nil, args[0])
			if err != nil {
				return err
			}
			return emitView(cmd, flags, node)
		},
	}
	cmd.AddCommand(get)
	return cmd
}

func newAdsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "ads", Short: "List StackAdapt ads", Annotations: map[string]string{"mcp:read-only": "true"}}
	cmd.RunE = parentNoSubcommandRunE(flags)

	var limit int
	list := &cobra.Command{
		Use: "list", Short: "List ads with total count (use --limit to cap)", Example: "  stackadapt-pp-cli ads list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "ads list", "would list ads")
			}
			q := fmt.Sprintf(`query($n:Int){ ads(first:$n){ totalCount nodes { %s } } }`, adFields)
			return runConnectionList(cmd, flags, "ads", "ads", q, map[string]any{"n": limit})
		},
	}
	list.Flags().IntVar(&limit, "limit", 100, "Max ads to return")
	cmd.AddCommand(list)
	return cmd
}

func newSegmentsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "segments", Short: "List StackAdapt custom audience segments", Annotations: map[string]string{"mcp:read-only": "true"}}
	cmd.RunE = parentNoSubcommandRunE(flags)

	var limit int
	list := &cobra.Command{
		Use: "list", Short: "List custom audience segments with total count (use --limit to cap)", Example: "  stackadapt-pp-cli segments list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "segments list", "would list custom segments")
			}
			q := fmt.Sprintf(`query($n:Int){ customSegments(first:$n){ totalCount nodes { %s } } }`, segmentFields)
			return runConnectionList(cmd, flags, "segments", "customSegments", q, map[string]any{"n": limit})
		},
	}
	list.Flags().IntVar(&limit, "limit", 100, "Max segments to return")
	cmd.AddCommand(list)
	return cmd
}

func newAccountCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use: "account", Short: "Show the StackAdapt account this token can access", Example: "  stackadapt-pp-cli account --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "account", "would show account info")
			}
			data, err := runQuery(cmd.Context(), flags, `query{ account { id currency } tokenInfo { __typename } }`, nil)
			if err != nil {
				return err
			}
			return emitView(cmd, flags, json.RawMessage(data))
		},
	}
}
