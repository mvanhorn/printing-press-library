// Copyright 2026 jrimmer. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// newRepoCmd creates the top-level 'repo' command group, mirroring gh's UX.
// This is the primary entry point for repository operations — create, list,
// view, and delete. For the full generated API surface see 'fj user', 'fj repos'.
func newRepoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Create, list, view, and delete repositories",
		Long: `Manage Forgejo repositories. Common operations at your fingertips.

  fj repo create --name myproject
  fj repo list
  fj repo view owner/repo
  fj repo delete owner/repo

For the full generated API surface see 'fj user' and 'fj repos'.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelRepoCreateCmd(flags))
	cmd.AddCommand(newNovelRepoListCmd(flags))
	cmd.AddCommand(newNovelRepoViewCmd(flags))
	cmd.AddCommand(newNovelRepoDeleteCmd(flags))
	return cmd
}

// newNovelRepoCreateCmd creates a repository for the authenticated user.
// Proxies POST /user/repos (fj user create-current-repo).
func newNovelRepoCreateCmd(flags *rootFlags) *cobra.Command {
	var name, description, defaultBranch, gitignores, license, readme, trustModel string
	var private, autoInit, template bool
	var stdinBody bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new repository",
		Long:  "Create a new repository for the authenticated user.",
		Example: `  # Create a public repository
  fj repo create --name myproject --description "My new project"

  # Create a private repository with a README and MIT license
  fj repo create --name myproject --private --auto-init --readme Default --license mit

  # Create from JSON on stdin
  echo '{"name":"myproject","private":true}' | fj repo create --stdin`,
		Annotations: map[string]string{
			"pp:endpoint": "user.create-current-repo",
			"pp:method":   "POST",
			"pp:path":     "/user/repos",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody && name == "" && !flags.dryRun {
				return fmt.Errorf("--name is required\nUsage: fj repo create --name <name>")
			}

			path := "/user/repos"
			var body map[string]any
			if stdinBody {
				stdinData, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				if err := json.Unmarshal(stdinData, &body); err != nil {
					return fmt.Errorf("parsing stdin JSON: %w", err)
				}
			} else {
				body = map[string]any{}
				if name != "" {
					body["name"] = name
				}
				if description != "" {
					body["description"] = description
				}
				if cmd.Flags().Changed("private") {
					body["private"] = private
				}
				if cmd.Flags().Changed("auto-init") {
					body["auto_init"] = autoInit
				}
				if defaultBranch != "" {
					body["default_branch"] = defaultBranch
				}
				if gitignores != "" {
					body["gitignores"] = gitignores
				}
				if license != "" {
					body["license"] = license
				}
				if readme != "" {
					body["readme"] = readme
				}
				if trustModel != "" {
					body["trust_model"] = trustModel
				}
				if cmd.Flags().Changed("template") {
					body["template"] = template
				}
			}

			if flags.dryRun {
				bodyJSON, _ := json.MarshalIndent(body, "", "  ")
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: POST %s\n%s\n", path, string(bodyJSON))
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, statusCode, err := c.PostWithParams(cmd.Context(), path, map[string]string{}, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var partialFailure *partialFailureReport
			if statusCode >= 200 && statusCode < 300 {
				partialFailure = detectPartialFailure(data)
				if partialFailure != nil {
					fmt.Fprintf(os.Stderr, "warning: partial failure detected: %s\n", partialFailure.Message)
				}
			}

			if statusCode >= 200 && statusCode < 300 && (partialFailure == nil || flags.allowPartialFailure) {
				writeMutationResponseToStore(cmd.Context(), "user", data, "")
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				envelope := map[string]any{
					"action":   "post",
					"resource": "user",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300 && (partialFailure == nil || flags.allowPartialFailure),
				}
				if len(filtered) > 0 {
					var parsed any
					if err := json.Unmarshal(filtered, &parsed); err == nil {
						envelope["data"] = parsed
					}
				}
				envelopeJSON, _ := json.Marshal(envelope)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
			}

			if perr := printOutputWithFlags(cmd.OutOrStdout(), data, flags); perr != nil {
				return perr
			}
			if partialFailure != nil && !flags.allowPartialFailure {
				return partialFailureErr(fmt.Errorf("partial failure: %s", partialFailure.Message))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Repository name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Repository description")
	cmd.Flags().BoolVar(&private, "private", false, "Make the repository private")
	cmd.Flags().BoolVar(&autoInit, "auto-init", false, "Initialize the repository with a README commit")
	cmd.Flags().StringVar(&defaultBranch, "default-branch", "", "Default branch name (default: main or server default)")
	cmd.Flags().StringVar(&gitignores, "gitignore", "", ".gitignore template name (e.g. Go, Python, Node)")
	cmd.Flags().StringVar(&license, "license", "", "License template name (e.g. mit, apache-2.0)")
	cmd.Flags().StringVar(&readme, "readme", "", "README template (Default, or a custom template name)")
	cmd.Flags().StringVar(&trustModel, "trust-model", "", "Signature trust model (collaborator, committer, collaboratorcommitter)")
	cmd.Flags().BoolVar(&template, "template", false, "Make this repository a template")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}

// newNovelRepoListCmd lists repositories owned by the authenticated user.
// Proxies GET /user/repos (fj user current-list-repos).
func newNovelRepoListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var orderBy string
	var all bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your repositories",
		Long:  "List repositories owned by the authenticated user.",
		Example: `  # List your repositories
  fj repo list

  # List all (paginate through everything)
  fj repo list --all

  # Output as JSON
  fj repo list --json

  # Sort by most recently updated
  fj repo list --order-by recentupdate --limit 10`,
		Annotations: map[string]string{
			"pp:endpoint":   "user.current-list-repos",
			"pp:method":     "GET",
			"pp:path":       "/user/repos",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeNoop(flags, "dry-run", "dry-run: would list your repositories")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			params := map[string]string{
				"limit": fmt.Sprintf("%d", limit),
			}
			if orderBy != "" {
				params["order_by"] = orderBy
			}

			data, prov, err := resolvePaginatedRead(cmd.Context(), c, flags, "user", "/user/repos", params, nil, all, "page", "", "", cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var countItems []json.RawMessage
				_ = json.Unmarshal(data, &countItems)
				printProvenance(cmd, len(countItems), prov)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= limit && !all {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To see more: add --limit N or --all.\n", len(items))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum number of results to return")
	cmd.Flags().StringVar(&orderBy, "order-by", "", "Sort order: newest, oldest, recentupdate, leastupdate, alphabetically, reversealphabetically, moststars, feweststars")
	cmd.Flags().BoolVar(&all, "all", false, "Paginate through all results")

	return cmd
}

// newNovelRepoViewCmd shows details about a repository.
// Proxies GET /repos/{owner}/{repo} (fj repos get).
func newNovelRepoViewCmd(flags *rootFlags) *cobra.Command {
	var owner, repo string

	cmd := &cobra.Command{
		Use:   "view [owner/repo]",
		Short: "View repository details",
		Long:  "Show details about a repository. Accepts owner/repo as a positional argument or --owner/--repo flags.",
		Example: `  fj repo view jrimmer/myproject
  fj repo view --owner jrimmer --repo myproject`,
		Annotations: map[string]string{
			"pp:endpoint":   "repos.get",
			"pp:method":     "GET",
			"pp:path":       "/repos/{owner}/{repo}",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeNoop(flags, "dry-run", "dry-run: would fetch repository details")
			}

			// Accept owner/repo as a single positional arg
			if len(args) == 1 {
				parts := splitOwnerRepo(args[0])
				if len(parts) == 2 {
					owner, repo = parts[0], parts[1]
				} else {
					return fmt.Errorf("expected owner/repo, got %q\nUsage: fj repo view <owner>/<repo>", args[0])
				}
			}
			if owner == "" || repo == "" {
				return fmt.Errorf("owner and repo are required\nUsage: fj repo view <owner>/<repo>")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/repos/%s/%s", owner, repo)
			data, prov, err := resolveRead(cmd.Context(), c, flags, "repos", false, path, map[string]string{}, nil, cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			data = extractResponseData(data)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var countItems []json.RawMessage
				if json.Unmarshal(data, &countItems) != nil {
					countItems = []json.RawMessage{data}
				}
				printProvenance(cmd, len(countItems), prov)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					return printAutoTable(cmd.OutOrStdout(), items)
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().StringVar(&owner, "owner", "", "Repository owner (user or org)")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository name")

	return cmd
}

// newNovelRepoDeleteCmd deletes a repository.
// Proxies DELETE /repos/{owner}/{repo} (fj repos delete).
func newNovelRepoDeleteCmd(flags *rootFlags) *cobra.Command {
	var owner, repo string

	cmd := &cobra.Command{
		Use:   "delete [owner/repo]",
		Short: "Delete a repository",
		Long:  "Permanently delete a repository. This cannot be undone. Requires confirmation unless --yes is set.",
		Example: `  fj repo delete jrimmer/myproject
  fj repo delete --owner jrimmer --repo myproject --yes`,
		Annotations: map[string]string{
			"pp:endpoint": "repos.delete",
			"pp:method":   "DELETE",
			"pp:path":     "/repos/{owner}/{repo}",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Accept owner/repo as a single positional arg
			if len(args) == 1 {
				parts := splitOwnerRepo(args[0])
				if len(parts) == 2 {
					owner, repo = parts[0], parts[1]
				} else {
					return fmt.Errorf("expected owner/repo, got %q\nUsage: fj repo delete <owner>/<repo>", args[0])
				}
			}
			if owner == "" || repo == "" {
				return fmt.Errorf("owner and repo are required\nUsage: fj repo delete <owner>/<repo>")
			}

			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: DELETE /repos/%s/%s\n", owner, repo)
				return nil
			}

			// Confirm unless --yes
			if !flags.yes && !flags.noInput && isTerminal(os.Stdin) {
				fmt.Fprintf(os.Stderr, "Delete repository %s/%s? This cannot be undone. [y/N]: ", owner, repo)
				reader := bufio.NewReader(os.Stdin)
				line, _ := reader.ReadString('\n')
				confirm := strings.TrimSpace(line)
				if confirm != "y" && confirm != "Y" {
					fmt.Fprintln(os.Stderr, "Aborted.")
					return nil
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/repos/%s/%s", owner, repo)
			data, statusCode, err := c.DeleteWithParams(cmd.Context(), path, map[string]string{})
			if err != nil {
				return classifyDeleteError(err, flags)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				envelope := map[string]any{
					"action":   "delete",
					"resource": "repos",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300,
				}
				envelopeJSON, _ := json.Marshal(envelope)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
			}

			if statusCode == 204 || (statusCode >= 200 && statusCode < 300 && len(data) == 0) {
				fmt.Fprintf(cmd.OutOrStdout(), "Repository %s/%s deleted.\n", owner, repo)
				return nil
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().StringVar(&owner, "owner", "", "Repository owner (user or org)")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository name")

	return cmd
}

// splitOwnerRepo splits "owner/repo" into ["owner", "repo"].
// Returns a single-element slice if the input has zero or more than one slash.
func splitOwnerRepo(s string) []string {
	for i, c := range s {
		if c == '/' {
			rest := s[i+1:]
			for _, rc := range rest {
				if rc == '/' {
					return []string{s}
				}
			}
			return []string{s[:i], rest}
		}
	}
	return []string{s}
}
