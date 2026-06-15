package mcp

import (
	"context"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/cloud/cloudflare/internal/mcp/cobratree"
)

// RegisterCloudflareAliasTools gives agents stable Cloudflare-specific workflow
// tool names while the generated code-orchestration pair keeps raw endpoints hidden.
func RegisterCloudflareAliasTools(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("cloudflare_search",
			mcplib.WithDescription("Search the Cloudflare API endpoint catalog."),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("Natural-language description of the endpoint to find.")),
			mcplib.WithNumber("limit", mcplib.Description("Max endpoints to return.")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		handleCodeOrchSearch,
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_execute",
			mcplib.WithDescription("Execute a Cloudflare API endpoint returned by cloudflare_search."),
			mcplib.WithString("endpoint_id", mcplib.Required(), mcplib.Description("Endpoint identifier returned by cloudflare_search.")),
			mcplib.WithObject("params", mcplib.Description("Parameters for the endpoint.")),
		),
		handleCodeOrchExecute,
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_token_recipe",
			mcplib.WithDescription("Print least-privilege Cloudflare token permissions for a workflow recipe."),
			mcplib.WithString("recipe", mcplib.Required(), mcplib.Description("Recipe name, for example site-launch.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			return []string{"token", "recipe", stringArg(args, "recipe"), "--dry-run"}, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_token_create",
			mcplib.WithDescription("Create or plan a scoped Cloudflare API token for a workflow recipe."),
			mcplib.WithString("recipe", mcplib.Required(), mcplib.Description("Recipe name, for example site-launch.")),
			mcplib.WithString("account", mcplib.Description("Cloudflare account ID for account-scoped recipes.")),
			mcplib.WithString("name", mcplib.Description("Token display name.")),
			mcplib.WithString("write-env", mcplib.Description("Path to write a 0600 env file with the created token.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without creating the token.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually create the token.")),
			mcplib.WithBoolean("show-token", mcplib.Description("Print the created token value once.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"token", "create", "--recipe", stringArg(args, "recipe")}
			out = appendOptionalString(out, args, "account")
			out = appendOptionalString(out, args, "name", "write-env")
			out = appendOptionalBool(out, args, "dry-run", "confirm", "show-token", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_token_doctor",
			mcplib.WithDescription("Check current Cloudflare token reachability and recipe readiness."),
			mcplib.WithString("recipe", mcplib.Description("Optional recipe name to check.")),
			mcplib.WithString("account", mcplib.Description("Cloudflare account ID.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"token", "doctor"}
			out = appendOptionalString(out, args, "recipe", "account")
			out = appendOptionalBool(out, args, "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_token_rotate",
			mcplib.WithDescription("Roll an existing Cloudflare API token secret."),
			mcplib.WithString("token_id", mcplib.Required(), mcplib.Description("Cloudflare API token ID.")),
			mcplib.WithString("write-env", mcplib.Description("Path to write a 0600 env file.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without rotating the token.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually rotate the token.")),
			mcplib.WithBoolean("show-token", mcplib.Description("Print the rotated token once.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"token", "rotate", stringArg(args, "token_id")}
			out = appendOptionalString(out, args, "write-env")
			out = appendOptionalBool(out, args, "dry-run", "confirm", "show-token", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_tunnel_launch",
			mcplib.WithDescription("Plan or launch a remotely managed Cloudflare Tunnel with hostname routing."),
			mcplib.WithString("hostname", mcplib.Required(), mcplib.Description("Public hostname, for example app.example.com.")),
			mcplib.WithString("service", mcplib.Required(), mcplib.Description("Origin service URL, for example http://localhost:3000.")),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("zone", mcplib.Description("Zone ID or name for DNS setup.")),
			mcplib.WithString("name", mcplib.Description("Tunnel name.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without mutating Cloudflare.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually create/configure the tunnel.")),
			mcplib.WithBoolean("no-open", mcplib.Description("Do not open dashboard or browser links.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"tunnel", "launch", "--hostname", stringArg(args, "hostname"), "--service", stringArg(args, "service"), "--account", stringArg(args, "account")}
			out = appendOptionalString(out, args, "zone", "name")
			out = appendOptionalBool(out, args, "dry-run", "confirm", "no-open", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_tunnel_status",
			mcplib.WithDescription("Inspect a Cloudflare tunnel and its connector status."),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("tunnel", mcplib.Required(), mcplib.Description("Tunnel ID.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"tunnel", "status", "--account", stringArg(args, "account"), "--tunnel", stringArg(args, "tunnel")}
			out = appendOptionalBool(out, args, "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_site_deploy",
			mcplib.WithDescription("Plan or deploy a static site through Cloudflare Pages direct upload."),
			mcplib.WithString("path", mcplib.Required(), mcplib.Description("Build output directory.")),
			mcplib.WithString("project", mcplib.Required(), mcplib.Description("Pages project name.")),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("domain", mcplib.Description("Custom domain to attach.")),
			mcplib.WithString("mode", mcplib.Description("Deployment mode: auto, pages, or workers.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without mutating Cloudflare.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually upload/deploy.")),
			mcplib.WithBoolean("no-open", mcplib.Description("Do not open dashboard or browser links.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"site", "deploy", stringArg(args, "path"), "--project", stringArg(args, "project"), "--account", stringArg(args, "account")}
			out = appendOptionalString(out, args, "domain", "mode")
			out = appendOptionalBool(out, args, "dry-run", "confirm", "no-open", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_site_plan",
			mcplib.WithDescription("Inspect static site output and choose Cloudflare Pages or Workers."),
			mcplib.WithString("path", mcplib.Description("Build output directory.")),
			mcplib.WithString("project", mcplib.Description("Pages project or Worker script name.")),
			mcplib.WithString("account", mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("domain", mcplib.Description("Custom domain to attach.")),
			mcplib.WithString("mode", mcplib.Description("Deployment mode: auto, pages, or workers.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"site", "plan"}
			if path := stringArg(args, "path"); path != "" {
				out = append(out, path)
			}
			out = appendOptionalString(out, args, "project", "account", "domain", "mode")
			out = appendOptionalBool(out, args, "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_r2_put",
			mcplib.WithDescription("Plan or upload a file/directory to Cloudflare R2."),
			mcplib.WithString("path", mcplib.Required(), mcplib.Description("Local file or directory path.")),
			mcplib.WithString("bucket", mcplib.Required(), mcplib.Description("R2 bucket name.")),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("prefix", mcplib.Description("Object key prefix.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without uploading.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually upload objects.")),
			mcplib.WithBoolean("no-open", mcplib.Description("Do not open dashboard or browser links.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"r2", "put", stringArg(args, "path"), "--bucket", stringArg(args, "bucket"), "--account", stringArg(args, "account")}
			out = appendOptionalString(out, args, "prefix")
			out = appendOptionalBool(out, args, "dry-run", "confirm", "no-open", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_email_route_create",
			mcplib.WithDescription("Plan or create a Cloudflare Email Routing forward rule."),
			mcplib.WithString("address", mcplib.Required(), mcplib.Description("Custom address, for example support@example.com.")),
			mcplib.WithString("to", mcplib.Required(), mcplib.Description("Destination email address.")),
			mcplib.WithString("zone", mcplib.Required(), mcplib.Description("Zone ID or name.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without creating the route.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually create the route.")),
			mcplib.WithBoolean("no-open", mcplib.Description("Do not open dashboard or browser links.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"email", "route", "create", stringArg(args, "address"), "--to", stringArg(args, "to"), "--zone", stringArg(args, "zone")}
			out = appendOptionalBool(out, args, "dry-run", "confirm", "no-open", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_agent_setup",
			mcplib.WithDescription("Plan or write an agent environment file for a Cloudflare workflow recipe."),
			mcplib.WithString("recipe", mcplib.Required(), mcplib.Description("Recipe name, for example site-launch.")),
			mcplib.WithString("write-env", mcplib.Required(), mcplib.Description("Path to write, for example .env.cloudflare.")),
			mcplib.WithString("account", mcplib.Description("Cloudflare account ID.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without writing files.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually write the env file.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"agent", "setup", "--recipe", stringArg(args, "recipe"), "--write-env", stringArg(args, "write-env")}
			out = appendOptionalString(out, args, "account")
			out = appendOptionalBool(out, args, "dry-run", "confirm", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_agent_admin",
			mcplib.WithDescription("Create or plan a token-management bootstrap token for agent operators."),
			mcplib.WithString("account", mcplib.Description("Optional Cloudflare account ID to include in resource scoping.")),
			mcplib.WithString("name", mcplib.Description("Token display name.")),
			mcplib.WithString("write-env", mcplib.Description("Path to write a 0600 env file with the created token.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without creating the token.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually create the token.")),
			mcplib.WithBoolean("show-token", mcplib.Description("Print the created token value once.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"agent", "admin"}
			out = appendOptionalString(out, args, "account", "name", "write-env")
			out = appendOptionalBool(out, args, "dry-run", "confirm", "show-token", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_project_launch",
			mcplib.WithDescription("Plan or launch a Cloudflare app project using Pages or Workers."),
			mcplib.WithString("path", mcplib.Required(), mcplib.Description("Build output directory.")),
			mcplib.WithString("project", mcplib.Required(), mcplib.Description("Project or script name.")),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("domain", mcplib.Description("Custom hostname to connect.")),
			mcplib.WithString("mode", mcplib.Description("Deployment mode: auto, pages, or workers.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without launching.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually launch.")),
			mcplib.WithBoolean("no-open", mcplib.Description("Do not open dashboard URLs.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"project", "launch", stringArg(args, "path"), "--project", stringArg(args, "project"), "--account", stringArg(args, "account")}
			out = appendOptionalString(out, args, "domain", "mode")
			out = appendOptionalBool(out, args, "dry-run", "confirm", "no-open", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_project_preview",
			mcplib.WithDescription("Create or plan a Cloudflare Pages preview deployment."),
			mcplib.WithString("path", mcplib.Required(), mcplib.Description("Build output directory.")),
			mcplib.WithString("project", mcplib.Required(), mcplib.Description("Pages project name.")),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("branch", mcplib.Description("Preview branch name.")),
			mcplib.WithString("domain", mcplib.Description("Optional preview hostname.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without deploying.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually create preview.")),
			mcplib.WithBoolean("no-open", mcplib.Description("Do not open dashboard URLs.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"project", "preview", stringArg(args, "path"), "--project", stringArg(args, "project"), "--account", stringArg(args, "account")}
			out = appendOptionalString(out, args, "branch", "domain")
			out = appendOptionalBool(out, args, "dry-run", "confirm", "no-open", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_zone_doctor",
			mcplib.WithDescription("Check zone, DNS, SSL, Workers routes, and Email Routing readiness."),
			mcplib.WithString("zone", mcplib.Required(), mcplib.Description("Cloudflare zone ID.")),
			mcplib.WithString("hostname", mcplib.Description("Optional hostname to inspect.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"zone", "doctor", "--zone", stringArg(args, "zone")}
			out = appendOptionalString(out, args, "hostname")
			out = appendOptionalBool(out, args, "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_domain_connect",
			mcplib.WithDescription("Connect a hostname to Pages, Workers, tunnels, or R2."),
			mcplib.WithString("hostname", mcplib.Required(), mcplib.Description("Hostname to connect.")),
			mcplib.WithString("target", mcplib.Required(), mcplib.Description("Target type: pages, worker, tunnel, or r2.")),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("zone", mcplib.Required(), mcplib.Description("Cloudflare zone ID.")),
			mcplib.WithString("project", mcplib.Description("Pages project name.")),
			mcplib.WithString("script", mcplib.Description("Worker script name.")),
			mcplib.WithString("tunnel", mcplib.Description("Tunnel ID.")),
			mcplib.WithString("bucket", mcplib.Description("R2 bucket name.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without applying.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually apply.")),
			mcplib.WithBoolean("no-open", mcplib.Description("Do not open dashboard URLs.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"domain", "connect", stringArg(args, "hostname"), "--target", stringArg(args, "target"), "--account", stringArg(args, "account"), "--zone", stringArg(args, "zone")}
			out = appendOptionalString(out, args, "project", "script", "tunnel", "bucket")
			out = appendOptionalBool(out, args, "dry-run", "confirm", "no-open", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_worker_deploy",
			mcplib.WithDescription("Plan or deploy a Cloudflare Worker script."),
			mcplib.WithString("path", mcplib.Required(), mcplib.Description("Worker file or bundle path.")),
			mcplib.WithString("script", mcplib.Required(), mcplib.Description("Worker script name.")),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("entry", mcplib.Description("Entry script file.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without deploying.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually deploy.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"worker", "deploy", stringArg(args, "path"), "--script", stringArg(args, "script"), "--account", stringArg(args, "account")}
			out = appendOptionalString(out, args, "entry")
			out = appendOptionalBool(out, args, "dry-run", "confirm", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_worker_secret_put",
			mcplib.WithDescription("Set a secret on a Cloudflare Worker script. Secret values are redacted from output."),
			mcplib.WithString("name", mcplib.Required(), mcplib.Description("Secret binding name.")),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("script", mcplib.Required(), mcplib.Description("Worker script name.")),
			mcplib.WithString("value", mcplib.Description("Secret value. Prefer from_env/from_file/from_stdin when available.")),
			mcplib.WithString("from-env", mcplib.Description("Environment variable containing the secret value.")),
			mcplib.WithString("from-file", mcplib.Description("File containing the secret value.")),
			mcplib.WithBoolean("from-stdin", mcplib.Description("Read the secret value from stdin.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without writing the secret.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually set the secret.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"worker", "secret", "put", stringArg(args, "name"), "--account", stringArg(args, "account"), "--script", stringArg(args, "script")}
			out = appendOptionalString(out, args, "value", "from-env", "from-file")
			out = appendOptionalBool(out, args, "from-stdin", "dry-run", "confirm", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_worker_secret_list",
			mcplib.WithDescription("List Cloudflare Worker script secrets without values."),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("script", mcplib.Required(), mcplib.Description("Worker script name.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"worker", "secret", "list", "--account", stringArg(args, "account"), "--script", stringArg(args, "script")}
			out = appendOptionalBool(out, args, "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_worker_secret_delete",
			mcplib.WithDescription("Delete a secret from a Cloudflare Worker script."),
			mcplib.WithString("name", mcplib.Required(), mcplib.Description("Secret binding name.")),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("script", mcplib.Required(), mcplib.Description("Worker script name.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without deleting.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually delete the secret.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"worker", "secret", "delete", stringArg(args, "name"), "--account", stringArg(args, "account"), "--script", stringArg(args, "script")}
			out = appendOptionalBool(out, args, "dry-run", "confirm", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_ai_gateway_setup",
			mcplib.WithDescription("Plan or create a Cloudflare AI Gateway."),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("gateway", mcplib.Required(), mcplib.Description("AI Gateway ID.")),
			mcplib.WithString("name", mcplib.Description("AI Gateway display name.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without creating.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually create.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"ai", "gateway", "setup", "--account", stringArg(args, "account"), "--gateway", stringArg(args, "gateway")}
			out = appendOptionalString(out, args, "name")
			out = appendOptionalBool(out, args, "dry-run", "confirm", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_cost_scan",
			mcplib.WithDescription("Read Cloudflare billable usage and AI Gateway usage signals."),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("since", mcplib.Description("Start time/date.")),
			mcplib.WithString("until", mcplib.Description("End time/date.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"cost", "scan", "--account", stringArg(args, "account")}
			out = appendOptionalString(out, args, "since", "until")
			out = appendOptionalBool(out, args, "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_deploy_rollback",
			mcplib.WithDescription("Plan or roll back a Cloudflare Pages production deployment."),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("project", mcplib.Required(), mcplib.Description("Pages project name.")),
			mcplib.WithString("deployment", mcplib.Required(), mcplib.Description("Pages deployment ID.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without rolling back.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually roll back.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"deploy", "rollback", "--account", stringArg(args, "account"), "--project", stringArg(args, "project"), "--deployment", stringArg(args, "deployment")}
			out = appendOptionalBool(out, args, "dry-run", "confirm", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_agent_memory_bootstrap",
			mcplib.WithDescription("Plan or create Cloudflare resources for persistent agent memory."),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("name", mcplib.Required(), mcplib.Description("Base name for created resources.")),
			mcplib.WithNumber("dimensions", mcplib.Description("Vector dimensions.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without creating.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually create resources.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"agent", "memory", "bootstrap", "--account", stringArg(args, "account"), "--name", stringArg(args, "name")}
			out = appendOptionalNumber(out, args, "dimensions")
			out = appendOptionalBool(out, args, "dry-run", "confirm", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_rag_bootstrap",
			mcplib.WithDescription("Plan or create Cloudflare RAG resources: R2, D1, Vectorize, and AI Gateway."),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("name", mcplib.Required(), mcplib.Description("Base name for created resources.")),
			mcplib.WithNumber("dimensions", mcplib.Description("Vector dimensions.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without creating.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually create resources.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"rag", "bootstrap", "--account", stringArg(args, "account"), "--name", stringArg(args, "name")}
			out = appendOptionalNumber(out, args, "dimensions")
			out = appendOptionalBool(out, args, "dry-run", "confirm", "json")
			return out, nil
		}),
	)

	s.AddTool(
		mcplib.NewTool("cloudflare_mcp_deploy",
			mcplib.WithDescription("Plan or deploy a Worker-hosted MCP server."),
			mcplib.WithString("path", mcplib.Required(), mcplib.Description("Worker file path.")),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Cloudflare account ID.")),
			mcplib.WithString("script", mcplib.Required(), mcplib.Description("Worker script name.")),
			mcplib.WithString("route", mcplib.Description("Optional route.")),
			mcplib.WithBoolean("dry-run", mcplib.Description("Plan without deploying.")),
			mcplib.WithBoolean("confirm", mcplib.Description("Actually deploy.")),
			mcplib.WithBoolean("json", mcplib.Description("Return JSON output.")),
		),
		cloudflareWorkflowHandler(func(args map[string]any) ([]string, error) {
			out := []string{"mcp", "deploy", stringArg(args, "path"), "--account", stringArg(args, "account"), "--script", stringArg(args, "script")}
			out = appendOptionalString(out, args, "route")
			out = appendOptionalBool(out, args, "dry-run", "confirm", "json")
			return out, nil
		}),
	)
}

func cloudflareWorkflowHandler(build func(map[string]any) ([]string, error)) server.ToolHandlerFunc {
	lookupPath, lookupErr := cobratree.SiblingCLIPath()
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		if lookupErr != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("companion CLI binary not found: %v", lookupErr)), nil
		}
		args := req.GetArguments()
		finalArgs, err := build(args)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		out, err := cobratree.RunCLICommand(ctx, lookupPath, finalArgs)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		return mcplib.NewToolResultText(out), nil
	}
}

func stringArg(args map[string]any, key string) string {
	if value, ok := args[key].(string); ok {
		return value
	}
	return ""
}

func appendOptionalString(out []string, args map[string]any, keys ...string) []string {
	for _, key := range keys {
		if value := stringArg(args, key); value != "" {
			out = append(out, "--"+key, value)
		}
	}
	return out
}

func appendOptionalBool(out []string, args map[string]any, keys ...string) []string {
	for _, key := range keys {
		if value, ok := args[key].(bool); ok && value {
			out = append(out, "--"+key)
		}
	}
	return out
}

func appendOptionalNumber(out []string, args map[string]any, keys ...string) []string {
	for _, key := range keys {
		if value, ok := args[key].(float64); ok && value != 0 {
			out = append(out, "--"+key, fmt.Sprintf("%g", value))
		}
	}
	return out
}
