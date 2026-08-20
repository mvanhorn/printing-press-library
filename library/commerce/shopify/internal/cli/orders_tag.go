package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const tagsAddMutation = `mutation($id: ID!, $tags: [String!]!) {
  tagsAdd(id: $id, tags: $tags) {
    node { id }
    userErrors { field message }
  }
}`

const tagsRemoveMutation = `mutation($id: ID!, $tags: [String!]!) {
  tagsRemove(id: $id, tags: $tags) {
    node { id }
    userErrors { field message }
  }
}`

// resourceGID prefixes a bare numeric id with the canonical Shopify Admin GID
// for the given resource (e.g. "1234" + "Order" -> "gid://shopify/Order/1234").
// Full GIDs are passed through unchanged.
func resourceGID(id, resource string) string {
	if strings.HasPrefix(id, "gid://shopify/") {
		return id
	}
	return fmt.Sprintf("gid://shopify/%s/%s", resource, id)
}

func parseTagList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func newOrdersTagCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Add or remove tags on a Shopify order.",
	}
	cmd.AddCommand(newTagsMutationCmd(flags, "Order", "orders", "add", tagsAddMutation, "tagsAdd"))
	cmd.AddCommand(newTagsMutationCmd(flags, "Order", "orders", "remove", tagsRemoveMutation, "tagsRemove"))
	return cmd
}

func newCustomersTagCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Add or remove tags on a Shopify customer.",
	}
	cmd.AddCommand(newTagsMutationCmd(flags, "Customer", "customers", "add", tagsAddMutation, "tagsAdd"))
	cmd.AddCommand(newTagsMutationCmd(flags, "Customer", "customers", "remove", tagsRemoveMutation, "tagsRemove"))
	return cmd
}

func newTagsMutationCmd(flags *rootFlags, resource, parent, verb, mutation, payloadField string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     verb + " [id] [tags]",
		Short:   fmt.Sprintf("%s tags on a Shopify %s. Pass the %s id (numeric or full GID) and a comma-separated tag list.", capitalizeASCII(verb), strings.ToLower(resource), strings.ToLower(resource)),
		Example: fmt.Sprintf("  shopify-pp-cli %s tag %s 1234567890 VIP,wholesale --json", parent, verb),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				if flags.dryRun {
					args = append(args, "example-tag")
				} else {
					return usageErr(fmt.Errorf("%s tag %s requires <id> and <tags>", parent, verb))
				}
			}
			id := args[0]
			if id == "" && flags.dryRun {
				id = "1234567890"
			}
			tags := parseTagList(args[1])
			if len(tags) == 0 && !flags.dryRun {
				return usageErr(fmt.Errorf("at least one tag is required"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			vars := map[string]any{
				"id":   resourceGID(id, resource),
				"tags": tags,
			}
			data, err := c.Mutate(cmd.Context(), mutation, vars)
			if err == nil && !flags.dryRun {
				data, err = extractGraphQLObject(data, payloadField)
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}

// capitalizeASCII uppercases the first byte of s when it's ASCII a-z.
// Replaces deprecated strings.Title for the verb values ("add" / "remove")
// used in this file's command short descriptions; ASCII-only is correct
// because these verbs are compile-time constants.
func capitalizeASCII(s string) string {
	if s == "" {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}
