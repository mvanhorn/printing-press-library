package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/client"
	"github.com/spf13/cobra"
)

func newNodesLiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "Canvas node operations via the Framer Server API",
	}
	cmd.AddCommand(newNodesGetLiveCmd(flags))
	cmd.AddCommand(newNodesChildrenLiveCmd(flags))
	cmd.AddCommand(newNodesRemoveLiveCmd(flags))
	cmd.AddCommand(newNodesMoveLiveCmd(flags))
	cmd.AddCommand(newNodesSetLiveCmd(flags))
	cmd.AddCommand(newNodesCloneLiveCmd(flags))
	cmd.AddCommand(newNodesCreateFrameLiveCmd(flags))
	return cmd
}

func newNodesGetLiveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "get <node-id>",
		Short:       "Get a node by ID from the live Framer project",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}
			raw, err := bc.Call("nodes-get", args[0])
			if err != nil {
				return err
			}
			if flags.asJSON {
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			var node struct{ ID, Name, Type string }
			json.Unmarshal(raw, &node)
			fmt.Fprintf(cmd.OutOrStdout(), "ID: %s  Name: %s  Type: %s\n", node.ID, node.Name, node.Type)
			return nil
		},
	}
}

func newNodesChildrenLiveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "children <node-id>",
		Short:       "List children of a node",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}
			raw, err := bc.Call("nodes-children", args[0])
			if err != nil {
				return err
			}
			if flags.asJSON {
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			var children []struct{ ID, Name, Type string }
			json.Unmarshal(raw, &children)
			for _, c := range children {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-30s %s\n", c.ID, c.Name, c.Type)
			}
			return nil
		},
	}
}

func newNodesRemoveLiveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <node-id>",
		Short: "Remove a node from the canvas",
		Example: strings.Trim(`
  # Remove a node
  framer-pp-cli nodes remove abc123

  # Dry-run to preview
  framer-pp-cli nodes remove abc123 --dry-run`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would remove node %s\n", args[0])
				return nil
			}
			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}
			_, err = bc.Call("nodes-remove", args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed node %s from Framer\n", args[0])
			return nil
		},
	}
}

func newNodesMoveLiveCmd(flags *rootFlags) *cobra.Command {
	var parentID string
	var index int
	cmd := &cobra.Command{
		Use:   "move <node-id> --parent <parent-id> [--index N]",
		Short: "Move a node to a new parent (reorder by index)",
		Example: strings.Trim(`
  # Move node under a new parent
  framer-pp-cli nodes move abc123 --parent def456

  # Move and set position (0 = first child)
  framer-pp-cli nodes move abc123 --parent def456 --index 0`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if parentID == "" {
				return usageErr(fmt.Errorf("--parent is required"))
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would move node %s to parent %s\n", args[0], parentID)
				return nil
			}
			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}
			payload := map[string]interface{}{"id": args[0], "parentId": parentID}
			if cmd.Flags().Changed("index") {
				payload["index"] = index
			}
			arg, _ := json.Marshal(payload)
			_, err = bc.Call("nodes-set-parent", string(arg))
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Moved node %s to parent %s in Framer\n", args[0], parentID)
			return nil
		},
	}
	cmd.Flags().StringVar(&parentID, "parent", "", "Target parent node ID (required)")
	cmd.Flags().IntVar(&index, "index", -1, "Position among siblings (0 = first)")
	return cmd
}

func newNodesSetLiveCmd(flags *rootFlags) *cobra.Command {
	var attrs []string
	cmd := &cobra.Command{
		Use:   "set <node-id> --attr key=value [--attr key=value ...]",
		Short: "Set attributes on a node",
		Example: strings.Trim(`
  # Set width on a node
  framer-pp-cli nodes set abc123 --attr width=400

  # Set multiple attributes
  framer-pp-cli nodes set abc123 --attr width=400 --attr height=300 --attr opacity=0.5`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(attrs) == 0 {
				return usageErr(fmt.Errorf("at least one --attr key=value is required"))
			}
			attrMap := make(map[string]interface{})
			for _, a := range attrs {
				parts := strings.SplitN(a, "=", 2)
				if len(parts) != 2 {
					return usageErr(fmt.Errorf("invalid --attr format: %q (expected key=value)", a))
				}
				attrMap[parts[0]] = parts[1]
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would set %d attributes on node %s\n", len(attrMap), args[0])
				return nil
			}
			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}
			payload, _ := json.Marshal(map[string]interface{}{"id": args[0], "attributes": attrMap})
			_, err = bc.Call("nodes-set-attributes", string(payload))
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated %d attributes on node %s in Framer\n", len(attrMap), args[0])
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&attrs, "attr", nil, "Attribute to set (key=value, repeatable)")
	return cmd
}

func newNodesCloneLiveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "clone <node-id>",
		Short: "Clone a node",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would clone node %s\n", args[0])
				return nil
			}
			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}
			raw, err := bc.Call("nodes-clone", args[0])
			if err != nil {
				return err
			}
			if flags.asJSON {
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			var node struct{ ID, Name, Type string }
			json.Unmarshal(raw, &node)
			fmt.Fprintf(cmd.OutOrStdout(), "Cloned node %s → new node %s in Framer\n", args[0], node.ID)
			return nil
		},
	}
}

func newNodesCreateFrameLiveCmd(flags *rootFlags) *cobra.Command {
	var parentID string
	var width, height int
	var name string
	cmd := &cobra.Command{
		Use:   "create-frame",
		Short: "Create a new frame node on the canvas",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would create a frame node")
				return nil
			}
			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}
			payload := map[string]interface{}{}
			if parentID != "" {
				payload["parentId"] = parentID
			}
			if name != "" {
				payload["name"] = name
			}
			if width > 0 {
				payload["width"] = width
			}
			if height > 0 {
				payload["height"] = height
			}
			arg, _ := json.Marshal(payload)
			raw, err := bc.Call("nodes-create-frame", string(arg))
			if err != nil {
				return err
			}
			if flags.asJSON {
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			var node struct{ ID string }
			json.Unmarshal(raw, &node)
			fmt.Fprintf(cmd.OutOrStdout(), "Created frame %s in Framer\n", node.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&parentID, "parent", "", "Parent node ID")
	cmd.Flags().StringVar(&name, "name", "", "Frame name")
	cmd.Flags().IntVar(&width, "width", 0, "Frame width")
	cmd.Flags().IntVar(&height, "height", 0, "Frame height")
	return cmd
}
