// Hand-authored. Do not regenerate over this file with `printing-press generate`
// without merging — it owns the ClickUp ergonomic alias surface.
//
// The generator names commands after API operationIds, so v3 docs and chat
// land under `workspaces docs search-public` and `workspaces chat get-channels`.
// That naming preserves a 1:1 mapping back to the spec but is rough on humans.
// This file adds parallel top-level `docs` and `chat` commands with shorter,
// more idiomatic verbs while leaving the originals intact for back-compat.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// renameForAlias rewrites a command's Use/Example to the alias verb while
// keeping the original verb as a Cobra alias so old muscle memory still works.
func renameForAlias(cmd *cobra.Command, newVerb, oldVerb string) {
	cmd.Use = strings.Replace(cmd.Use, oldVerb, newVerb, 1)
	cmd.Example = strings.ReplaceAll(cmd.Example, " "+oldVerb+" ", " "+newVerb+" ")
	cmd.Aliases = append(cmd.Aliases, oldVerb)
}

// newDocsAliasCmd builds a top-level `docs` command tree mirroring
// `workspaces docs <verb>-public` with cleaner verbs.
//
//	clickup-pp-cli docs search   → workspaces docs search-public
//	clickup-pp-cli docs get      → workspaces docs get-public
//	clickup-pp-cli docs pages    → workspaces docs get-pages-public
//	clickup-pp-cli docs page     → workspaces docs get-page-public
//	clickup-pp-cli docs listing  → workspaces docs get-page-listing-public
//	clickup-pp-cli docs create   → workspaces docs create-public
//	clickup-pp-cli docs new-page → workspaces docs create-page-public
//	clickup-pp-cli docs edit     → workspaces docs edit-page-public
func newDocsAliasCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "ClickUp Docs (v3) — shorthand for `workspaces docs ...`",
	}

	search := newWorkspacesDocsSearchPublicCmd(flags)
	renameForAlias(search, "search", "search-public")
	cmd.AddCommand(search)

	get := newWorkspacesDocsGetPublicCmd(flags)
	renameForAlias(get, "get", "get-public")
	cmd.AddCommand(get)

	pages := newWorkspacesDocsGetPagesPublicCmd(flags)
	renameForAlias(pages, "pages", "get-pages-public")
	cmd.AddCommand(pages)

	page := newWorkspacesDocsGetPagePublicCmd(flags)
	renameForAlias(page, "page", "get-page-public")
	cmd.AddCommand(page)

	listing := newWorkspacesDocsGetPageListingPublicCmd(flags)
	renameForAlias(listing, "listing", "get-page-listing-public")
	cmd.AddCommand(listing)

	create := newWorkspacesDocsCreatePublicCmd(flags)
	renameForAlias(create, "create", "create-public")
	cmd.AddCommand(create)

	newPage := newWorkspacesDocsCreatePagePublicCmd(flags)
	renameForAlias(newPage, "new-page", "create-page-public")
	cmd.AddCommand(newPage)

	edit := newWorkspacesDocsEditPagePublicCmd(flags)
	renameForAlias(edit, "edit", "edit-page-public")
	cmd.AddCommand(edit)

	return cmd
}

// newChatAliasCmd builds a top-level `chat` command tree mirroring
// `workspaces chat <verb>` with cleaner verbs.
//
//	clickup-pp-cli chat list           → workspaces chat get-channels
//	clickup-pp-cli chat get            → workspaces chat get-channel
//	clickup-pp-cli chat new            → workspaces chat create-channel
//	clickup-pp-cli chat new-location   → workspaces chat create-location-channel
//	clickup-pp-cli chat new-dm         → workspaces chat create-direct-message-channel
//	clickup-pp-cli chat update         → workspaces chat update-channel
//	clickup-pp-cli chat delete         → workspaces chat delete-channel
//	clickup-pp-cli chat followers      → workspaces chat get-channel-followers
//	clickup-pp-cli chat members        → workspaces chat get-channel-members
//	clickup-pp-cli chat messages       → workspaces chat get-messages
//	clickup-pp-cli chat send           → workspaces chat create-message
//	clickup-pp-cli chat edit           → workspaces chat patch-message
//	clickup-pp-cli chat delete-msg     → workspaces chat delete-message
//	clickup-pp-cli chat reactions      → workspaces chat get-message-reactions
//	clickup-pp-cli chat react          → workspaces chat create-reaction
//	clickup-pp-cli chat unreact        → workspaces chat delete-reaction
//	clickup-pp-cli chat replies        → workspaces chat get-message-replies
//	clickup-pp-cli chat reply          → workspaces chat create-reply-message
//	clickup-pp-cli chat tagged-users   → workspaces chat get-message-tagged-users
func newChatAliasCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "ClickUp Chat (v3) — shorthand for `workspaces chat ...`",
	}

	pairs := []struct {
		newVerb string
		oldVerb string
		build   func(*rootFlags) *cobra.Command
	}{
		{"list", "get-channels", newWorkspacesChatGetChannelsCmd},
		{"get", "get-channel", newWorkspacesChatGetChannelCmd},
		{"new", "create-channel", newWorkspacesChatCreateChannelCmd},
		{"new-location", "create-location-channel", newWorkspacesChatCreateLocationChannelCmd},
		{"new-dm", "create-direct-message-channel", newWorkspacesChatCreateDirectMessageChannelCmd},
		{"update", "update-channel", newWorkspacesChatUpdateChannelCmd},
		{"delete", "delete-channel", newWorkspacesChatDeleteChannelCmd},
		{"followers", "get-channel-followers", newWorkspacesChatGetChannelFollowersCmd},
		{"members", "get-channel-members", newWorkspacesChatGetChannelMembersCmd},
		{"messages", "get-messages", newWorkspacesChatGetMessagesCmd},
		{"send", "create-message", newWorkspacesChatCreateMessageCmd},
		{"edit", "patch-message", newWorkspacesChatPatchMessageCmd},
		{"delete-msg", "delete-message", newWorkspacesChatDeleteMessageCmd},
		{"reactions", "get-message-reactions", newWorkspacesChatGetMessageReactionsCmd},
		{"react", "create-reaction", newWorkspacesChatCreateReactionCmd},
		{"unreact", "delete-reaction", newWorkspacesChatDeleteReactionCmd},
		{"replies", "get-message-replies", newWorkspacesChatGetMessageRepliesCmd},
		{"reply", "create-reply-message", newWorkspacesChatCreateReplyMessageCmd},
		{"tagged-users", "get-message-tagged-users", newWorkspacesChatGetMessageTaggedUsersCmd},
	}

	for _, p := range pairs {
		c := p.build(flags)
		renameForAlias(c, p.newVerb, p.oldVerb)
		cmd.AddCommand(c)
	}

	return cmd
}
