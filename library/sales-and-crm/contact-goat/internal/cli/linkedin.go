// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/linkedin"

	"github.com/spf13/cobra"
)

// newLinkedInCmd is the parent cobra command for all `linkedin <subcommand>`
// invocations. Each subcommand spawns a short-lived MCP stdio subprocess
// (`uvx linkedin-scraper-mcp@latest`), initializes it, runs a single tool
// call, and tears it down.
//
// Because the underlying Python server uses Selenium (real Chrome), startup
// is not cheap. We make no attempt to pool or keep the subprocess alive
// across commands -- that's a future optimization.
func newLinkedInCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "linkedin",
		Short: "LinkedIn scraper powered by stickerdaniel/linkedin-mcp-server",
		Long: "LinkedIn subcommands wrap the upstream linkedin-scraper-mcp Python MCP server.\n" +
			"First-time setup: run `uvx linkedin-scraper-mcp@latest --login` to cache your\n" +
			"signed-in Chrome profile under ~/.linkedin-mcp/profile. After that, every\n" +
			"`contact-goat-pp-cli linkedin ...` invocation reuses that profile.",
		Example: "  contact-goat-pp-cli linkedin search-people \"VP engineering\" --location \"San Francisco\" --limit 25\n" +
			"  contact-goat-pp-cli linkedin get-person https://www.linkedin.com/in/williamhgates/\n" +
			"  contact-goat-pp-cli linkedin inbox --limit 10 --json",
	}

	cmd.AddCommand(newLISearchPeopleCmd(flags))
	cmd.AddCommand(newLISearchJobsCmd(flags))
	cmd.AddCommand(newLIGetPersonCmd(flags))
	cmd.AddCommand(newLIGetCompanyCmd(flags))
	cmd.AddCommand(newLIInboxCmd(flags))
	cmd.AddCommand(newLIConversationCmd(flags))
	cmd.AddCommand(newLISearchMessagesCmd(flags))
	cmd.AddCommand(newLISendMessageCmd(flags))
	cmd.AddCommand(newLICompanyPostsCmd(flags))
	cmd.AddCommand(newLIJobCmd(flags))
	cmd.AddCommand(newLISidebarCmd(flags))
	cmd.AddCommand(newLIConnectCmd(flags))
	cmd.AddCommand(newLIListToolsCmd(flags))

	return cmd
}

// ---------------------------------------------------------------------------
// read commands
// ---------------------------------------------------------------------------

func newLISearchPeopleCmd(flags *rootFlags) *cobra.Command {
	var location string
	var limit int
	cmd := &cobra.Command{
		Use:   "search-people <keywords>",
		Short: "Search LinkedIn people by keyword",
		Example: "  contact-goat-pp-cli linkedin search-people \"VP engineering fintech\"\n" +
			"  contact-goat-pp-cli linkedin search-people \"Staff eng\" --location \"New York\" --limit 50 --json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// MCP tool search_people only accepts keywords + location; --limit
			// is applied client-side after the call returns.
			args0 := map[string]any{"keywords": args[0]}
			if location != "" {
				args0["location"] = location
			}
			return runLIToolWithLimit(cmd, flags, linkedin.ToolNames.SearchPeople, args0, limit)
		},
	}
	cmd.Flags().StringVar(&location, "location", "", "Geographic filter (e.g. \"San Francisco Bay Area\")")
	cmd.Flags().IntVar(&limit, "limit", 25, "Client-side cap on results returned")
	return cmd
}

func newLISearchJobsCmd(flags *rootFlags) *cobra.Command {
	var location string
	var maxPages int
	cmd := &cobra.Command{
		Use:   "search-jobs <keywords>",
		Short: "Search LinkedIn job postings by keyword",
		Example: "  contact-goat-pp-cli linkedin search-jobs \"senior backend engineer\"\n" +
			"  contact-goat-pp-cli linkedin search-jobs \"product designer\" --location Remote --max-pages 5",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// MCP tool search_jobs accepts keywords, location, max_pages (1-10).
			// Client-side --limit would be applied on the returned list.
			args0 := map[string]any{"keywords": args[0]}
			if location != "" {
				args0["location"] = location
			}
			if maxPages > 0 {
				args0["max_pages"] = maxPages
			}
			return runLITool(cmd, flags, linkedin.ToolNames.SearchJobs, args0)
		},
	}
	cmd.Flags().StringVar(&location, "location", "", "Geographic filter (e.g. Remote, \"Seattle, WA\")")
	cmd.Flags().IntVar(&maxPages, "max-pages", 3, "Pages of results to load (1-10)")
	return cmd
}

func newLIGetPersonCmd(flags *rootFlags) *cobra.Command {
	var sections []string
	cmd := &cobra.Command{
		Use:   "get-person <url-or-slug>",
		Short: "Fetch a LinkedIn profile",
		Example: "  contact-goat-pp-cli linkedin get-person williamhgates\n" +
			"  contact-goat-pp-cli linkedin get-person https://www.linkedin.com/in/satyanadella/ \\\n" +
			"      --sections experience,education,skills",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := map[string]any{
				"linkedin_username": normalizePersonInput(args[0]),
			}
			if joined := normalizeSections(sections); joined != "" {
				payload["sections"] = joined
			}
			return runLITool(cmd, flags, linkedin.ToolNames.GetPerson, payload)
		},
	}
	cmd.Flags().StringSliceVar(&sections, "sections", nil,
		"Comma-separated sections to include: experience,education,skills,accomplishments,contacts,recommendations")
	return cmd
}

func newLIGetCompanyCmd(flags *rootFlags) *cobra.Command {
	var sections []string
	cmd := &cobra.Command{
		Use:   "get-company <slug>",
		Short: "Fetch a LinkedIn company profile",
		Example: "  contact-goat-pp-cli linkedin get-company openai\n" +
			"  contact-goat-pp-cli linkedin get-company stripe --sections posts,jobs",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := map[string]any{
				"company_name": args[0],
			}
			if joined := normalizeSections(sections); joined != "" {
				payload["sections"] = joined
			}
			return runLITool(cmd, flags, linkedin.ToolNames.GetCompany, payload)
		},
	}
	cmd.Flags().StringSliceVar(&sections, "sections", nil, "Comma-separated sections: posts,jobs,employees,about")
	return cmd
}

func newLIInboxCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "List recent LinkedIn conversations",
		Example: "  contact-goat-pp-cli linkedin inbox\n" +
			"  contact-goat-pp-cli linkedin inbox --limit 25 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLITool(cmd, flags, linkedin.ToolNames.Inbox, map[string]any{
				"limit": limit,
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Number of threads to return")
	return cmd
}

func newLIConversationCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conversation <user-or-thread-id>",
		Short: "Read a single LinkedIn conversation",
		Long: "Read a single LinkedIn conversation.\n\n" +
			"WARNING: upstream issue stickerdaniel/linkedin-mcp-server#307 reports\n" +
			"intermittent failures here (the MCP occasionally returns empty results\n" +
			"even when messages exist). If you see empty output, retry or fall back\n" +
			"to `linkedin inbox` + manual thread selection.",
		Example: "  contact-goat-pp-cli linkedin conversation 2-NzBjZjM4NDktNjRkMy00\n" +
			"  contact-goat-pp-cli linkedin conversation williamhgates --json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "warning: upstream issue #307 — conversation may return empty; retry on failure.")
			return runLITool(cmd, flags, linkedin.ToolNames.Conversation, map[string]any{
				"user_or_thread_id": args[0],
			})
		},
	}
	return cmd
}

func newLISearchMessagesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search-messages <query>",
		Short: "Full-text search your LinkedIn messages",
		Example: "  contact-goat-pp-cli linkedin search-messages \"series A\"\n" +
			"  contact-goat-pp-cli linkedin search-messages \"coffee chat\" --json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLITool(cmd, flags, linkedin.ToolNames.SearchMessages, map[string]any{
				"query": args[0],
			})
		},
	}
	return cmd
}

func newLICompanyPostsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "company-posts <slug>",
		Short: "List recent posts from a company page",
		Example: "  contact-goat-pp-cli linkedin company-posts anthropic\n" +
			"  contact-goat-pp-cli linkedin company-posts openai --limit 20",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLITool(cmd, flags, linkedin.ToolNames.CompanyPosts, map[string]any{
				"company_slug": args[0],
				"limit":        limit,
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Number of posts to return")
	return cmd
}

func newLIJobCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job <job-id>",
		Short: "Fetch full detail for a single LinkedIn job posting",
		Example: "  contact-goat-pp-cli linkedin job 3712345678\n" +
			"  contact-goat-pp-cli linkedin job 3712345678 --json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLITool(cmd, flags, linkedin.ToolNames.Job, map[string]any{
				"job_id": args[0],
			})
		},
	}
	return cmd
}

func newLISidebarCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sidebar <person-url>",
		Short: "Fetch the \"People also viewed\" sidebar for a profile",
		Example: "  contact-goat-pp-cli linkedin sidebar https://www.linkedin.com/in/satyanadella/\n" +
			"  contact-goat-pp-cli linkedin sidebar williamhgates",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLITool(cmd, flags, linkedin.ToolNames.Sidebar, map[string]any{
				"person_url": normalizePersonInput(args[0]),
			})
		},
	}
	return cmd
}

func newLIListToolsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list-tools",
		Short: "List tools exposed by the underlying linkedin-scraper-mcp server",
		Example: "  contact-goat-pp-cli linkedin list-tools\n" +
			"  contact-goat-pp-cli linkedin list-tools --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx(cmd.Context())
			defer cancel()
			client, err := spawnLIClient(ctx)
			if err != nil {
				return err
			}
			defer client.Close()
			if _, err := client.Initialize(ctx, linkedin.Implementation{Name: "contact-goat-pp-cli", Version: version}); err != nil {
				return fmt.Errorf("initialize: %w", err)
			}
			res, err := client.ListTools(ctx)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
			}
			for _, t := range res.Tools {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", t.Name, t.Description)
			}
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// write commands (require --confirm or --dry-run)
// ---------------------------------------------------------------------------

func newLISendMessageCmd(flags *rootFlags) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:   "send-message <recipient> <body>",
		Short: "Send a LinkedIn direct message (write)",
		Long: "Send a LinkedIn direct message.\n\n" +
			"This is a write operation and defaults to NOT sending. You must pass\n" +
			"either --dry-run (print the request and exit) or --confirm (actually\n" +
			"send). If neither is set, the command errors out with guidance.",
		Example: "  # dry-run (safe):\n" +
			"  contact-goat-pp-cli linkedin send-message williamhgates \"Hi Bill — loved the letter\" --dry-run\n\n" +
			"  # actually send (agents must pass --confirm):\n" +
			"  contact-goat-pp-cli linkedin send-message satyanadella \"Thanks for the intro\" --confirm",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			recipient, body := args[0], args[1]
			payload := map[string]any{"recipient": recipient, "body": body}
			if flags.dryRun {
				fmt.Fprintln(os.Stderr, "dry-run: would call send_message with:")
				return json.NewEncoder(cmd.OutOrStdout()).Encode(payload)
			}
			if !confirm {
				return usageErr(errors.New(
					"refusing to send: pass --confirm to actually send, or --dry-run to preview"))
			}
			return runLITool(cmd, flags, linkedin.ToolNames.SendMessage, payload)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually send the message (required for non-dry-run sends)")
	return cmd
}

func newLIConnectCmd(flags *rootFlags) *cobra.Command {
	var note string
	var confirm bool
	cmd := &cobra.Command{
		Use:   "connect <person-url>",
		Short: "Send a LinkedIn connection request (write)",
		Long: "Send a LinkedIn connection request with an optional note.\n\n" +
			"WARNING: upstream issue stickerdaniel/linkedin-mcp-server#365 reports\n" +
			"that notes are sometimes dropped. Verify via the LinkedIn UI if the\n" +
			"note is critical.\n\n" +
			"This is a write operation and defaults to NOT sending. You must pass\n" +
			"either --dry-run (print the request and exit) or --confirm (send).",
		Example: "  # preview (safe):\n" +
			"  contact-goat-pp-cli linkedin connect https://www.linkedin.com/in/satyanadella/ --note \"Met at KubeCon\" --dry-run\n\n" +
			"  # actually send:\n" +
			"  contact-goat-pp-cli linkedin connect williamhgates --note \"Hi Bill — fan of the foundation\" --confirm",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "warning: upstream issue #365 — the --note may not reach the recipient.")
			payload := map[string]any{
				"person_url": normalizePersonInput(args[0]),
				"note":       note,
			}
			if flags.dryRun {
				fmt.Fprintln(os.Stderr, "dry-run: would call send_connection_request with:")
				return json.NewEncoder(cmd.OutOrStdout()).Encode(payload)
			}
			if !confirm {
				return usageErr(errors.New(
					"refusing to send: pass --confirm to actually send, or --dry-run to preview"))
			}
			return runLITool(cmd, flags, linkedin.ToolNames.Connect, payload)
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "Optional personal note (max 300 chars per LinkedIn)")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually send the request (required for non-dry-run)")
	return cmd
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// runLITool is the common path for every read-style linkedin subcommand.
// It spawns the MCP subprocess, initializes it, calls one tool, prints the
// result, and tears everything down.
// runLIToolWithLimit calls runLITool then applies a client-side --limit to
// the result's JSON array (if it is one). MCP search_people has no server
// limit arg, so this is the only way to cap output without downloading
// everything through the terminal.
func runLIToolWithLimit(cmd *cobra.Command, flags *rootFlags, toolName string, rawArgs map[string]any, limit int) error {
	return runLIToolInternal(cmd, flags, toolName, rawArgs, limit)
}

func runLITool(cmd *cobra.Command, flags *rootFlags, toolName string, rawArgs map[string]any) error {
	return runLIToolInternal(cmd, flags, toolName, rawArgs, 0)
}

func runLIToolInternal(cmd *cobra.Command, flags *rootFlags, toolName string, rawArgs map[string]any, clientLimit int) error {
	args := pruneEmpty(rawArgs)

	if flags.dryRun {
		fmt.Fprintf(os.Stderr, "dry-run: would call MCP tool %q with:\n", toolName)
		return json.NewEncoder(cmd.OutOrStdout()).Encode(args)
	}

	// Surface a friendly warning if the user hasn't logged in yet. The MCP
	// itself will also error, but we can give clearer guidance earlier.
	if ok, _ := linkedin.IsLoggedIn(); !ok {
		fmt.Fprintln(os.Stderr, "warning: no cached LinkedIn profile detected at ~/.linkedin-mcp/profile.")
		fmt.Fprintln(os.Stderr, linkedin.LoginHint())
	}

	ctx, cancel := signalCtx(cmd.Context())
	defer cancel()

	// Respect --timeout from root flags, but clamp to >= 10s so Selenium has
	// a chance to start.
	timeout := flags.timeout
	if timeout < 10*time.Second {
		timeout = 30 * time.Second
	}
	callCtx, callCancel := context.WithTimeout(ctx, timeout)
	defer callCancel()

	client, err := spawnLIClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	if _, err := client.Initialize(ctx, linkedin.Implementation{Name: "contact-goat-pp-cli", Version: version}); err != nil {
		return fmt.Errorf("initialize linkedin-mcp: %w\nstderr: %s", err, client.StderrTail())
	}

	result, err := client.CallTool(callCtx, toolName, args)
	if err != nil {
		return fmt.Errorf("call %s: %w", toolName, err)
	}

	if clientLimit > 0 {
		result = truncateLIArray(result, clientLimit)
	}
	return printLIResult(cmd, flags, result)
}

// truncateLIArray applies a client-side limit when the MCP result is a JSON
// array. Non-array results pass through unchanged.
func truncateLIArray(r *linkedin.CallToolResult, limit int) *linkedin.CallToolResult {
	if r == nil || limit <= 0 {
		return r
	}
	payload := linkedin.TextPayload(r)
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(payload), &arr); err != nil || len(arr) <= limit {
		return r
	}
	arr = arr[:limit]
	buf, err := json.Marshal(arr)
	if err != nil {
		return r
	}
	// Build a new result with the truncated JSON in a single text block.
	return &linkedin.CallToolResult{
		Content: []linkedin.ContentBlock{{Type: "text", Text: string(buf)}},
	}
}

func spawnLIClient(ctx context.Context) (*linkedin.Client, error) {
	bin, args, err := linkedin.ResolveSpawnCommand()
	if err != nil {
		return nil, err
	}
	return linkedin.NewClient(ctx, linkedin.Options{
		Command:    bin,
		Args:       args,
		ClientInfo: linkedin.Implementation{Name: "contact-goat-pp-cli", Version: version},
	})
}

// printLIResult renders an MCP CallToolResult respecting --json.
//
// The MCP server encodes payloads as one or more text content blocks. When
// the blocks contain JSON we pretty-print that JSON; when they don't, we
// emit the concatenated text as-is.
func printLIResult(cmd *cobra.Command, flags *rootFlags, r *linkedin.CallToolResult) error {
	payload := linkedin.TextPayload(r)
	w := cmd.OutOrStdout()

	// Try to parse the text as JSON.
	var parsed json.RawMessage
	parsed = json.RawMessage(payload)
	isJSON := json.Valid([]byte(payload))

	if flags.asJSON || !isTerminal(w) {
		if !isJSON {
			// Wrap non-JSON text into an object for agent-friendly output.
			wrapper := map[string]any{"text": payload}
			return json.NewEncoder(w).Encode(wrapper)
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(parsed)
	}

	if !isJSON {
		_, err := fmt.Fprintln(w, payload)
		return err
	}

	// Pretty-print JSON for terminal users.
	var pretty any
	if err := json.Unmarshal([]byte(payload), &pretty); err != nil {
		_, err := fmt.Fprintln(w, payload)
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(pretty)
}

// pruneEmpty strips nil / empty-string / zero-int / empty-slice values from
// the args map so we don't send `"location": ""` to the MCP server (which
// some upstream tools treat as a literal empty-string filter).
func pruneEmpty(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch x := v.(type) {
		case nil:
			continue
		case string:
			if x == "" {
				continue
			}
		case int:
			if x == 0 {
				continue
			}
		case []string:
			if len(x) == 0 {
				continue
			}
		}
		out[k] = v
	}
	return out
}

// normalizePersonInput accepts either a bare vanity slug ("williamhgates")
// or a full LinkedIn profile URL, and returns the bare username the
// upstream linkedin-scraper-mcp tool requires. The MCP validates its
// argument as `linkedin_username` and rejects full URLs. Known forms:
//
//	williamhgates
//	https://www.linkedin.com/in/williamhgates
//	https://www.linkedin.com/in/williamhgates/
//	http://linkedin.com/in/alonsovelasco
//	/in/alonsovelasco/
func normalizePersonInput(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "/")
	if idx := strings.LastIndex(s, "/in/"); idx >= 0 {
		return strings.TrimSuffix(s[idx+len("/in/"):], "/")
	}
	// Already a bare slug.
	return s
}

// normalizeSections turns a []string list into the comma-joined string
// the upstream MCP expects on `sections` fields. Empty slice returns
// "" so the caller can drop the arg entirely.
func normalizeSections(sections []string) string {
	return strings.Join(sections, ",")
}

// signalCtx wraps a parent context so SIGINT / SIGTERM cancels it.
func signalCtx(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(sigCh)
	}()
	return ctx, cancel
}
