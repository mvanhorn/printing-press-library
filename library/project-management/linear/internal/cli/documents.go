package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

func newDocumentsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "documents [document-ref]",
		Short: "View, list, create, and edit Linear documents",
		Long: `View a Linear document, or manage documents with the subcommands.

The positional document reference accepts every form Linear surfaces:
  - document UUID:  4a09c2e6-3a25-4cb8-ab63-9c9f6754b24e
  - bare slugId:    f7f48ab36080
  - full URL slug:  my-runbook-f7f48ab36080
  - document URL:   https://linear.app/<org>/document/my-runbook-f7f48ab36080`,
		Example: `  linear-pp-cli documents my-runbook-f7f48ab36080 --agent --select title,updatedAt,content
  linear-pp-cli documents f7f48ab36080 --agent`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3,4,5,7"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			doc, err := fetchDocumentLive(c, args[0])
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			return renderLiveObject(cmd, flags, doc, "documents")
		},
	}
	cmd.AddCommand(newDocumentsListCmd(flags))
	cmd.AddCommand(newDocumentsCreateCmd(flags))
	cmd.AddCommand(newDocumentsEditCmd(flags))
	return cmd
}

func newDocumentsListCmd(flags *rootFlags) *cobra.Command {
	var issue, project, team string
	var after string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Linear documents",
		Example: `  linear-pp-cli documents list --issue ENG-123 --agent
  linear-pp-cli documents list --project <project-uuid> --limit 50 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			filter := map[string]any{}
			if issue != "" {
				issueID, err := resolveIssueID(c, issue)
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				filter["issue"] = map[string]any{"id": map[string]any{"eq": issueID}}
			}
			if project != "" {
				filter["project"] = map[string]any{"id": map[string]any{"eq": project}}
			}
			if team != "" {
				if store.IsUUID(team) {
					filter["team"] = map[string]any{"id": map[string]any{"eq": team}}
				} else {
					filter["team"] = map[string]any{"key": map[string]any{"eqIgnoreCase": team}}
				}
			}
			if limit <= 0 {
				limit = 50
			}
			const query = `query($first: Int!, $filter: DocumentFilter, $after: String) {
				documents(first: $first, filter: $filter, after: $after) {
					nodes {
						id title slugId url createdAt updatedAt summary
						creator { id name displayName email }
						issue { id identifier title }
						project { id name }
						team { id key name }
						documentContentId
					}
					pageInfo { hasNextPage endCursor }
				}
			}`
			var resp struct {
				Documents struct {
					Nodes    []json.RawMessage `json:"nodes"`
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
				} `json:"documents"`
			}
			vars := map[string]any{"first": limit, "filter": filter, "after": nil}
			if after != "" {
				vars["after"] = after
			}
			if err := c.QueryInto(query, vars, &resp); err != nil {
				return classifyAPIError(err, flags)
			}
			out, err := json.Marshal(map[string]any{
				"documents": resp.Documents.Nodes,
				"pageInfo":  resp.Documents.PageInfo,
			})
			if err != nil {
				return err
			}
			return renderLivePayload(cmd, flags, out, "documents", true)
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "Filter by issue identifier or UUID")
	cmd.Flags().StringVar(&project, "project", "", "Filter by project UUID")
	cmd.Flags().StringVar(&team, "team", "", "Filter by team key or UUID")
	cmd.Flags().StringVar(&after, "after", "", "Cursor from pageInfo.endCursor for the next page")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum documents to return")
	return cmd
}

func newDocumentsCreateCmd(flags *rootFlags) *cobra.Command {
	var title, contentFlag, contentFile string
	var contentStdin bool
	var issue, project, team, initiative, cycle, release, folder string
	var mediaFlag []string
	var mediaPublic bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Linear document",
		Example: `  linear-pp-cli documents create --title "Runbook" --issue ENG-123 --content-file /tmp/runbook.md --agent
  linear-pp-cli documents create --title "Project brief" --project <project-uuid> --content-stdin --agent < /tmp/brief.md`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				return usageErr(fmt.Errorf("--title is required"))
			}
			body, bodySet, err := readMarkdownBody(cmd, markdownBodySpec{
				InlineFlag: "content",
				Inline:     contentFlag,
				FileFlag:   "content-file",
				File:       contentFile,
				StdinFlag:  "content-stdin",
				Stdin:      contentStdin,
				Label:      "content",
			})
			if err != nil {
				return err
			}
			if !bodySet && len(mediaFlag) == 0 {
				return usageErr(fmt.Errorf("document content is required; pass --content-file, --content-stdin, --content, or --media"))
			}
			if err := validateDocumentCreateParents(issue, project, team, initiative, cycle, release, folder); err != nil {
				return err
			}
			input := map[string]any{"title": title, "content": body}
			applyDocumentParentInputs(input, issue, project, team, initiative, cycle, release, folder)
			if flags.dryRun {
				out := map[string]any{"input": input}
				if len(mediaFlag) > 0 {
					out["media"] = mediaFlag
					out["media_public"] = mediaPublic
				}
				return renderMutationDryRun(cmd, flags, "would_create_document", "documentCreate", out)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			input = map[string]any{"title": title}
			if err := applyDocumentParents(c, input, issue, project, team, initiative, cycle, release, folder); err != nil {
				return classifyLiveReadError(err, flags)
			}
			body, uploaded, err := uploadMediaAndAppend(c, body, mediaFlag, mediaPublic)
			if err != nil {
				return mediaUploadFailure(err, uploaded)
			}
			input["content"] = body
			const mutation = `mutation($input: DocumentCreateInput!) {
				documentCreate(input: $input) {
					success
					document {
						id title slugId url content createdAt updatedAt documentContentId
						creator { id name displayName email }
						issue { id identifier title }
						project { id name }
						team { id key name }
					}
				}
			}`
			resp, err := c.Mutate(mutation, map[string]any{"input": input})
			if err != nil {
				return classifyMutationError("documentCreate", err, flags, uploaded)
			}
			doc, err := extractMutationObject(resp, "documentCreate", "document")
			if err != nil {
				return mediaUploadFailure(err, uploaded)
			}
			return renderLiveObject(cmd, flags, doc, "documents")
		},
	}
	bindDocumentParentFlags(cmd, &issue, &project, &team, &initiative, &cycle, &release, &folder)
	cmd.Flags().StringVar(&title, "title", "", "Document title")
	cmd.Flags().StringVar(&contentFlag, "content", "", "Document content markdown")
	cmd.Flags().StringVar(&contentFile, "content-file", "", "Read document content markdown from file")
	cmd.Flags().BoolVar(&contentStdin, "content-stdin", false, "Read document content markdown from stdin")
	cmd.Flags().StringSliceVar(&mediaFlag, "media", nil, "Upload file and append it to the document markdown (repeatable)")
	cmd.Flags().BoolVar(&mediaPublic, "media-public", false, "Request public Linear asset URLs for uploaded media")
	return cmd
}

func newDocumentsEditCmd(flags *rootFlags) *cobra.Command {
	var title, contentFlag, contentFile string
	var contentStdin bool
	var issue, project, team, initiative, cycle, release, folder string
	var mediaFlag []string
	var mediaPublic bool
	cmd := &cobra.Command{
		Use:   "edit <document-id-or-slug>",
		Short: "Edit a Linear document",
		Example: `  linear-pp-cli documents edit <document-id> --content-file /tmp/updated.md --agent
  linear-pp-cli documents edit <document-id> --media /tmp/screenshot.png --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := map[string]any{}
			if cmd.Flags().Changed("title") {
				input["title"] = title
			}
			body, bodySet, err := readMarkdownBody(cmd, markdownBodySpec{
				InlineFlag: "content",
				Inline:     contentFlag,
				FileFlag:   "content-file",
				File:       contentFile,
				StdinFlag:  "content-stdin",
				Stdin:      contentStdin,
				Label:      "content",
			})
			if err != nil {
				return err
			}
			if bodySet {
				input["content"] = body
			}
			applyDocumentParentInputs(input, issue, project, team, initiative, cycle, release, folder)
			if len(input) == 0 && len(mediaFlag) == 0 {
				return usageErr(fmt.Errorf("no document fields supplied; pass --title, --content-file, --content-stdin, --content, --media, or a parent flag"))
			}
			if flags.dryRun {
				out := map[string]any{"document": args[0], "input": input}
				if len(mediaFlag) > 0 {
					out["media"] = mediaFlag
					out["media_public"] = mediaPublic
				}
				return renderMutationDryRun(cmd, flags, "would_update_document", "documentUpdate", out)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			docID := args[0]
			var existingContent string
			if !store.IsUUID(args[0]) || (len(mediaFlag) > 0 && !bodySet) {
				existing, err := fetchDocumentLive(c, args[0])
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				var doc struct {
					ID      string `json:"id"`
					Content string `json:"content"`
				}
				if err := json.Unmarshal(existing, &doc); err != nil {
					return fmt.Errorf("parsing existing document: %w", err)
				}
				if doc.ID == "" {
					return fmt.Errorf("document %q did not include an id", args[0])
				}
				docID = doc.ID
				existingContent = doc.Content
			}
			if len(mediaFlag) > 0 && !bodySet {
				body = existingContent
				bodySet = true
			}
			input = map[string]any{}
			if cmd.Flags().Changed("title") {
				input["title"] = title
			}
			if err := applyDocumentParents(c, input, issue, project, team, initiative, cycle, release, folder); err != nil {
				return classifyLiveReadError(err, flags)
			}
			body, uploaded, err := uploadMediaAndAppend(c, body, mediaFlag, mediaPublic)
			if err != nil {
				return mediaUploadFailure(err, uploaded)
			}
			if bodySet {
				input["content"] = body
			}
			if len(input) == 0 {
				return usageErr(fmt.Errorf("no document fields supplied; pass --title, --content-file, --content-stdin, --content, --media, or a parent flag"))
			}
			const mutation = `mutation($id: String!, $input: DocumentUpdateInput!) {
				documentUpdate(id: $id, input: $input) {
					success
					document {
						id title slugId url content createdAt updatedAt documentContentId
						creator { id name displayName email }
						issue { id identifier title }
						project { id name }
						team { id key name }
					}
				}
			}`
			resp, err := c.Mutate(mutation, map[string]any{"id": docID, "input": input})
			if err != nil {
				return classifyMutationError("documentUpdate", err, flags, uploaded)
			}
			docRaw, err := extractMutationObject(resp, "documentUpdate", "document")
			if err != nil {
				return mediaUploadFailure(err, uploaded)
			}
			return renderLiveObject(cmd, flags, docRaw, "documents")
		},
	}
	bindDocumentParentFlags(cmd, &issue, &project, &team, &initiative, &cycle, &release, &folder)
	cmd.Flags().StringVar(&title, "title", "", "Document title")
	cmd.Flags().StringVar(&contentFlag, "content", "", "Document content markdown")
	cmd.Flags().StringVar(&contentFile, "content-file", "", "Read document content markdown from file")
	cmd.Flags().BoolVar(&contentStdin, "content-stdin", false, "Read document content markdown from stdin")
	cmd.Flags().StringSliceVar(&mediaFlag, "media", nil, "Upload file and append it to the document markdown (repeatable)")
	cmd.Flags().BoolVar(&mediaPublic, "media-public", false, "Request public Linear asset URLs for uploaded media")
	return cmd
}

func bindDocumentParentFlags(cmd *cobra.Command, issue, project, team, initiative, cycle, release, folder *string) {
	cmd.Flags().StringVar(issue, "issue", "", "Attach document to issue identifier or UUID")
	cmd.Flags().StringVar(project, "project", "", "Attach document to project UUID")
	cmd.Flags().StringVar(team, "team", "", "Attach document to team key or UUID")
	cmd.Flags().StringVar(initiative, "initiative", "", "Attach document to initiative UUID")
	cmd.Flags().StringVar(cycle, "cycle", "", "Attach document to cycle UUID")
	cmd.Flags().StringVar(release, "release", "", "Attach document to release UUID")
	cmd.Flags().StringVar(folder, "folder", "", "Attach document to resource folder UUID")
}

func validateDocumentCreateParents(issue, project, team, initiative, cycle, release, folder string) error {
	count := 0
	for _, value := range []string{issue, project, team, initiative, cycle, release, folder} {
		if value != "" {
			count++
		}
	}
	if count != 1 {
		return usageErr(fmt.Errorf("document create requires exactly one parent; pass one of --issue, --project, --team, --initiative, --cycle, --release, or --folder"))
	}
	return nil
}

func applyDocumentParents(c graphqlQueryer, input map[string]any, issue, project, team, initiative, cycle, release, folder string) error {
	applyDocumentParentInputs(input, issue, project, team, initiative, cycle, release, folder)
	if issue != "" {
		issueID, err := resolveIssueID(c, issue)
		if err != nil {
			return err
		}
		input["issueId"] = issueID
	}
	if team != "" && !store.IsUUID(team) {
		teamID, err := resolveTeamIDLive(c, team)
		if err != nil {
			return err
		}
		input["teamId"] = teamID
	}
	return nil
}

func applyDocumentParentInputs(input map[string]any, issue, project, team, initiative, cycle, release, folder string) {
	if issue != "" {
		input["issueId"] = issue
	}
	if project != "" {
		input["projectId"] = project
	}
	if team != "" {
		input["teamId"] = team
	}
	if initiative != "" {
		input["initiativeId"] = initiative
	}
	if cycle != "" {
		input["cycleId"] = cycle
	}
	if release != "" {
		input["releaseId"] = release
	}
	if folder != "" {
		input["resourceFolderId"] = folder
	}
}

func resolveTeamIDLive(c graphqlQueryer, keyOrName string) (string, error) {
	if id, err := resolveTeamIDByKeyLive(c, keyOrName); err != nil {
		return "", err
	} else if id != "" {
		return id, nil
	}
	if id, err := resolveTeamIDByNameLive(c, keyOrName); err != nil {
		return "", err
	} else if id != "" {
		return id, nil
	}
	return "", notFoundErr(fmt.Errorf("team %q not found", keyOrName))
}

func resolveTeamIDByKeyLive(c graphqlQueryer, key string) (string, error) {
	const query = `query($key: String!) {
		teams(filter: { key: { eq: $key } }, first: 1) {
			nodes { id key name }
		}
	}`
	return resolveTeamIDFromQuery(c, query, map[string]any{"key": strings.ToUpper(key)})
}

func resolveTeamIDByNameLive(c graphqlQueryer, name string) (string, error) {
	const query = `query($name: String!) {
		teams(filter: { name: { eq: $name } }, first: 1) {
			nodes { id key name }
		}
	}`
	return resolveTeamIDFromQuery(c, query, map[string]any{"name": name})
}

func resolveTeamIDFromQuery(c graphqlQueryer, query string, variables map[string]any) (string, error) {
	var resp struct {
		Teams struct {
			Nodes []struct {
				ID   string `json:"id"`
				Key  string `json:"key"`
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"teams"`
	}
	if err := c.QueryInto(query, variables, &resp); err != nil {
		return "", err
	}
	if len(resp.Teams.Nodes) == 0 || resp.Teams.Nodes[0].ID == "" {
		return "", nil
	}
	return resp.Teams.Nodes[0].ID, nil
}

// normalizeDocumentRef maps the document references humans and Linear surface
// to the identifier shapes the GraphQL lookup accepts. Accepted inputs:
//
//   - UUID document id (passed through)
//   - bare slugId, e.g. "f7f48ab36080" (passed through)
//   - full URL slug, e.g. "symphony-pipeline-restart-runbook-f7f48ab36080"
//     (trailing slugId segment extracted)
//   - full Linear document URL, e.g.
//     "https://linear.app/<org>/document/<title-slug>-<slugId>"
//     (path tail extracted, then the trailing slugId segment)
//
// Linear's documents(filter: {slugId: ...}) only matches the bare slugId, so
// title-slug and URL forms must be reduced to the segment after the last "-".
func normalizeDocumentRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if store.IsUUID(ref) {
		return ref
	}
	// Full URL: keep the last path segment, dropping any query/fragment.
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		if u, err := url.Parse(ref); err == nil {
			ref = path.Base(u.Path)
		}
	}
	if store.IsUUID(ref) {
		return ref
	}
	// Title-slug + slugId: the slugId is the segment after the last hyphen.
	if idx := strings.LastIndex(ref, "-"); idx >= 0 && idx < len(ref)-1 {
		ref = ref[idx+1:]
	}
	return ref
}

func fetchDocumentLive(c graphqlQueryer, ref string) (json.RawMessage, error) {
	idOrSlug := normalizeDocumentRef(ref)
	if idOrSlug == "" || strings.Trim(idOrSlug, "-") == "" {
		return nil, notFoundErr(fmt.Errorf("document %q not found (could not extract a document UUID or slugId); pass a document UUID, bare slugId, full URL slug, or the document URL", ref))
	}
	if store.IsUUID(idOrSlug) {
		const byID = `query($id: String!) {
		document(id: $id) {
			id title slugId url content createdAt updatedAt documentContentId
			creator { id name displayName email }
			issue { id identifier title }
			project { id name }
			team { id key name }
		}
	}`
		var resp struct {
			Document json.RawMessage `json:"document"`
		}
		if err := c.QueryInto(byID, map[string]any{"id": idOrSlug}, &resp); err != nil {
			return nil, err
		}
		if len(resp.Document) == 0 || string(resp.Document) == "null" {
			return nil, notFoundErr(fmt.Errorf("document %q not found", ref))
		}
		return resp.Document, nil
	}
	const bySlug = `query($slug: String!) {
		documents(filter: { slugId: { eq: $slug } }, first: 1) {
			nodes {
				id title slugId url content createdAt updatedAt documentContentId
				creator { id name displayName email }
				issue { id identifier title }
				project { id name }
				team { id key name }
			}
		}
	}`
	var slugResp struct {
		Documents struct {
			Nodes []json.RawMessage `json:"nodes"`
		} `json:"documents"`
	}
	if err := c.QueryInto(bySlug, map[string]any{"slug": idOrSlug}, &slugResp); err != nil {
		return nil, err
	}
	if len(slugResp.Documents.Nodes) == 0 {
		return nil, notFoundErr(fmt.Errorf("document %q not found (looked up slugId %q); pass a document UUID, bare slugId, full URL slug, or the document URL", ref, idOrSlug))
	}
	return slugResp.Documents.Nodes[0], nil
}
