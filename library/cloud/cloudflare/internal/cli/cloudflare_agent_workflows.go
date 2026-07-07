package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/cloud/cloudflare/internal/config"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newProjectWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "project", Short: "Launch and preview Cloudflare app projects", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newProjectLaunchCmd(flags))
	cmd.AddCommand(newProjectPreviewCmd(flags))
	return cmd
}

func newProjectLaunchCmd(flags *rootFlags) *cobra.Command {
	var accountID, project, domain, mode string
	var confirm, noOpen bool
	cmd := &cobra.Command{
		Use:   "launch <dir>",
		Short: "Plan or launch a Cloudflare Pages/Workers project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" || project == "" {
				return fmt.Errorf("--account and --project are required")
			}
			dir := detectSiteDir(args[0])
			files, err := collectFiles(dir)
			if err != nil {
				return err
			}
			if mode == "" || mode == "auto" {
				mode = detectDeployMode(dir)
			}
			plan := map[string]any{
				"account":       accountID,
				"project":       project,
				"domain":        domain,
				"dir":           dir,
				"files":         len(files),
				"mode":          mode,
				"no_open":       noOpen,
				"recipe":        recipeForDeployMode(mode),
				"steps":         projectLaunchSteps(mode, domain),
				"requires_flag": "--confirm",
			}
			if flags.dryRun || !confirmWrite(flags, confirm) {
				return printWorkflowResult(cmd, flags, plan)
			}
			if mode == "workers" {
				return fmt.Errorf("project launch for workers requires worker deploy; run cloudflare-pp-cli worker deploy with --dry-run first")
			}
			return deployPagesDirect(cmd, flags, accountID, project, dir, "production", domain)
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&project, "project", "", "Pages project or Worker script name")
	cmd.Flags().StringVar(&domain, "domain", "", "Custom domain to attach")
	cmd.Flags().StringVar(&mode, "mode", "auto", "Deployment mode: auto, pages, or workers")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Launch the project")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Do not open dashboard URLs")
	return cmd
}

func newProjectPreviewCmd(flags *rootFlags) *cobra.Command {
	var accountID, project, branch, domain string
	var confirm, noOpen bool
	cmd := &cobra.Command{
		Use:   "preview <dir>",
		Short: "Create a Cloudflare Pages preview deployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" || project == "" {
				return fmt.Errorf("--account and --project are required")
			}
			if branch == "" {
				branch = "preview"
			}
			dir := detectSiteDir(args[0])
			files, err := collectFiles(dir)
			if err != nil {
				return err
			}
			plan := map[string]any{
				"account":       accountID,
				"project":       project,
				"branch":        branch,
				"domain":        domain,
				"dir":           dir,
				"files":         len(files),
				"no_open":       noOpen,
				"method":        "POST multipart/form-data",
				"path":          "/accounts/{account_id}/pages/projects/{project_name}/deployments",
				"requires_flag": "--confirm",
			}
			if flags.dryRun || !confirmWrite(flags, confirm) {
				return printWorkflowResult(cmd, flags, plan)
			}
			return deployPagesDirect(cmd, flags, accountID, project, dir, branch, domain)
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&project, "project", "", "Pages project name")
	cmd.Flags().StringVar(&branch, "branch", "preview", "Preview branch name")
	cmd.Flags().StringVar(&domain, "domain", "", "Optional preview hostname to connect")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Create the preview deployment")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Do not open dashboard URLs")
	return cmd
}

func newZoneWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "zone", Short: "Diagnose Cloudflare zones", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newZoneDoctorCmd(flags))
	return cmd
}

func newZoneDoctorCmd(flags *rootFlags) *cobra.Command {
	var zoneID, hostname string
	cmd := &cobra.Command{
		Use:         "doctor",
		Short:       "Check zone, DNS, SSL, Pages, tunnel, and email readiness",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if zoneID == "" {
				return fmt.Errorf("--zone is required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			out := map[string]any{"zone": zoneID, "hostname": hostname}
			addProbe(cmd, c, out, "zone", "/zones/"+url.PathEscape(zoneID), nil)
			params := map[string]string{"per_page": "100"}
			if hostname != "" {
				params["name"] = hostname
			}
			addProbe(cmd, c, out, "dns_records", "/zones/"+url.PathEscape(zoneID)+"/dns_records", params)
			addProbe(cmd, c, out, "zone_settings", "/zones/"+url.PathEscape(zoneID)+"/settings", nil)
			addProbe(cmd, c, out, "email_routing", "/zones/"+url.PathEscape(zoneID)+"/email/routing", nil)
			addProbe(cmd, c, out, "worker_routes", "/zones/"+url.PathEscape(zoneID)+"/workers/routes", nil)
			return printWorkflowResult(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&zoneID, "zone", "", "Cloudflare zone ID")
	cmd.Flags().StringVar(&hostname, "hostname", "", "Optional hostname to inspect")
	return cmd
}

func newDomainWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "domain", Short: "Connect hostnames to Cloudflare products", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newDomainConnectCmd(flags))
	return cmd
}

func newDomainConnectCmd(flags *rootFlags) *cobra.Command {
	var accountID, zoneID, target, project, script, tunnelID, bucket, service string
	var confirm, noOpen bool
	cmd := &cobra.Command{
		Use:   "connect <hostname>",
		Short: "Connect a domain to Pages, Workers, tunnels, or R2",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hostname := args[0]
			if accountID == "" || zoneID == "" || target == "" {
				return fmt.Errorf("--account, --zone, and --target are required")
			}
			plan := domainConnectPlan(hostname, target, project, script, tunnelID, bucket, service)
			plan["account"] = accountID
			plan["zone"] = zoneID
			plan["no_open"] = noOpen
			plan["requires_flag"] = "--confirm"
			if flags.dryRun || !confirmWrite(flags, confirm) {
				return printWorkflowResult(cmd, flags, plan)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			applied := map[string]any{"hostname": hostname, "target": target}
			switch target {
			case "pages":
				if project == "" {
					return fmt.Errorf("--project is required for --target pages")
				}
				data, status, err := c.Post(cmd.Context(), "/accounts/"+url.PathEscape(accountID)+"/pages/projects/"+url.PathEscape(project)+"/domains", map[string]any{"name": hostname})
				if err != nil {
					return err
				}
				applied["pages_domain_status"] = status
				applied["pages_domain_response"] = json.RawMessage(data)
			case "worker":
				if script == "" {
					return fmt.Errorf("--script is required for --target worker")
				}
				body := map[string]any{"pattern": hostname + "/*", "script": script}
				data, status, err := c.Post(cmd.Context(), "/zones/"+url.PathEscape(zoneID)+"/workers/routes", body)
				if err != nil {
					return err
				}
				applied["worker_route_status"] = status
				applied["worker_route_response"] = json.RawMessage(data)
			case "tunnel":
				if tunnelID == "" {
					return fmt.Errorf("--tunnel is required for --target tunnel")
				}
				body := map[string]any{"type": "CNAME", "name": hostname, "content": tunnelID + ".cfargotunnel.com", "proxied": true}
				data, status, err := c.Post(cmd.Context(), "/zones/"+url.PathEscape(zoneID)+"/dns_records", body)
				if err != nil {
					return err
				}
				applied["dns_status"] = status
				applied["dns_response"] = json.RawMessage(data)
			case "r2":
				if bucket == "" {
					return fmt.Errorf("--bucket is required for --target r2")
				}
				body := map[string]any{"domain": hostname, "enabled": true}
				data, status, err := c.Post(cmd.Context(), "/accounts/"+url.PathEscape(accountID)+"/r2/buckets/"+url.PathEscape(bucket)+"/domains/custom", body)
				if err != nil {
					return err
				}
				applied["r2_domain_status"] = status
				applied["r2_domain_response"] = json.RawMessage(data)
			default:
				return fmt.Errorf("unknown --target %q; use pages, worker, tunnel, or r2", target)
			}
			return printWorkflowResult(cmd, flags, applied)
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&zoneID, "zone", "", "Cloudflare zone ID")
	cmd.Flags().StringVar(&target, "target", "", "Target type: pages, worker, tunnel, or r2")
	cmd.Flags().StringVar(&project, "project", "", "Pages project name")
	cmd.Flags().StringVar(&script, "script", "", "Worker script name")
	cmd.Flags().StringVar(&tunnelID, "tunnel", "", "Tunnel ID")
	cmd.Flags().StringVar(&bucket, "bucket", "", "R2 bucket name")
	cmd.Flags().StringVar(&service, "service", "", "Origin service URL for tunnel planning")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Apply the domain connection")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Do not open dashboard URLs")
	return cmd
}

func newWorkerWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "worker", Short: "Deploy Cloudflare Workers", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newWorkerDeployCmd(flags))
	cmd.AddCommand(newWorkerSecretCmd(flags))
	return cmd
}

func newWorkerSecretCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "secret", Short: "Manage Worker script secrets", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newWorkerSecretPutCmd(flags))
	cmd.AddCommand(newWorkerSecretListCmd(flags))
	cmd.AddCommand(newWorkerSecretDeleteCmd(flags))
	return cmd
}

func newWorkerSecretPutCmd(flags *rootFlags) *cobra.Command {
	var accountID, scriptName, value, fromEnv, fromFile string
	var fromStdin, confirm bool
	cmd := &cobra.Command{
		Use:   "put <name>",
		Short: "Set a secret on a Worker script",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" || scriptName == "" {
				return fmt.Errorf("--account and --script are required")
			}
			secretName := args[0]
			secretValue, source, err := readWorkerSecretValue(value, fromEnv, fromFile, fromStdin)
			if err != nil {
				return err
			}
			body := map[string]any{"name": secretName, "text": secretValue, "type": "secret_text"}
			plan := map[string]any{
				"account":       accountID,
				"script":        scriptName,
				"secret":        secretName,
				"value":         redacted(secretValue),
				"value_source":  source,
				"method":        "PUT",
				"path":          "/accounts/{account_id}/workers/scripts/{script_name}/secrets",
				"requires_flag": "--confirm",
			}
			if flags.dryRun || !confirmWrite(flags, confirm) {
				return printWorkflowResult(cmd, flags, plan)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := "/accounts/" + url.PathEscape(accountID) + "/workers/scripts/" + url.PathEscape(scriptName) + "/secrets"
			data, status, err := c.Put(cmd.Context(), path, body)
			if err != nil {
				return err
			}
			return printWorkflowResult(cmd, flags, map[string]any{
				"status": status,
				"script": scriptName,
				"secret": secretName,
				"value":  redacted(secretValue),
				"result": json.RawMessage(data),
			})
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&scriptName, "script", "", "Worker script name")
	cmd.Flags().StringVar(&value, "value", "", "Secret value (prefer --from-env, --from-file, or --from-stdin)")
	cmd.Flags().StringVar(&fromEnv, "from-env", "", "Read secret value from an environment variable")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Read secret value from a local file")
	cmd.Flags().BoolVar(&fromStdin, "from-stdin", false, "Read secret value from stdin")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Set the Worker secret")
	return cmd
}

func newWorkerSecretListCmd(flags *rootFlags) *cobra.Command {
	var accountID, scriptName string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List Worker script secrets without values",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" || scriptName == "" {
				return fmt.Errorf("--account and --script are required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := "/accounts/" + url.PathEscape(accountID) + "/workers/scripts/" + url.PathEscape(scriptName) + "/secrets"
			data, err := c.GetNoCache(cmd.Context(), path, nil)
			if err != nil {
				return err
			}
			return printWorkflowResult(cmd, flags, map[string]any{"script": scriptName, "secrets": json.RawMessage(data)})
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&scriptName, "script", "", "Worker script name")
	return cmd
}

func newWorkerSecretDeleteCmd(flags *rootFlags) *cobra.Command {
	var accountID, scriptName string
	var confirm bool
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a Worker script secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" || scriptName == "" {
				return fmt.Errorf("--account and --script are required")
			}
			secretName := args[0]
			plan := map[string]any{
				"account":       accountID,
				"script":        scriptName,
				"secret":        secretName,
				"method":        "DELETE",
				"path":          "/accounts/{account_id}/workers/scripts/{script_name}/secrets/{secret_name}",
				"requires_flag": "--confirm",
			}
			if flags.dryRun || !confirmWrite(flags, confirm) {
				return printWorkflowResult(cmd, flags, plan)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := "/accounts/" + url.PathEscape(accountID) + "/workers/scripts/" + url.PathEscape(scriptName) + "/secrets/" + url.PathEscape(secretName)
			data, status, err := c.Delete(cmd.Context(), path)
			if err != nil {
				return err
			}
			return printWorkflowResult(cmd, flags, map[string]any{"status": status, "script": scriptName, "secret": secretName, "result": json.RawMessage(data)})
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&scriptName, "script", "", "Worker script name")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Delete the Worker secret")
	return cmd
}

func newWorkerDeployCmd(flags *rootFlags) *cobra.Command {
	var accountID, scriptName, entry string
	var confirm bool
	cmd := &cobra.Command{
		Use:   "deploy <path>",
		Short: "Plan or deploy a Worker script",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" || scriptName == "" {
				return fmt.Errorf("--account and --script are required")
			}
			path := args[0]
			files, err := collectFiles(path)
			if err != nil {
				return err
			}
			if entry == "" && len(files) == 1 {
				entry = files[0]
			}
			plan := map[string]any{
				"account":       accountID,
				"script":        scriptName,
				"path":          path,
				"files":         len(files),
				"entry":         entry,
				"method":        "PUT",
				"path_template": "/accounts/{account_id}/workers/scripts/{script_name}",
				"requires_flag": "--confirm",
			}
			if flags.dryRun || !confirmWrite(flags, confirm) {
				return printWorkflowResult(cmd, flags, plan)
			}
			if entry == "" || len(files) != 1 {
				return fmt.Errorf("confirmed worker deploy currently supports one script file; pass a single file or --entry, and use dry-run for bundled projects")
			}
			status, body, err := putWorkerScript(cmd, flags, accountID, scriptName, entry)
			if err != nil {
				return err
			}
			return printWorkflowResult(cmd, flags, map[string]any{"status": status, "response": json.RawMessage(body)})
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&scriptName, "script", "", "Worker script name")
	cmd.Flags().StringVar(&entry, "entry", "", "Entry script file for bundled projects")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Deploy the Worker")
	return cmd
}

func newAIWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "ai", Short: "Configure Cloudflare AI workflows", RunE: parentNoSubcommandRunE(flags)}
	gateway := &cobra.Command{Use: "gateway", Short: "Manage AI Gateway workflows", RunE: parentNoSubcommandRunE(flags)}
	gateway.AddCommand(newAIGatewaySetupCmd(flags))
	cmd.AddCommand(gateway)
	return cmd
}

func newAIGatewaySetupCmd(flags *rootFlags) *cobra.Command {
	var accountID, gatewayID, name string
	var confirm bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Plan or create an AI Gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" || gatewayID == "" {
				return fmt.Errorf("--account and --gateway are required")
			}
			if name == "" {
				name = gatewayID
			}
			body := map[string]any{"id": gatewayID, "name": name}
			plan := map[string]any{
				"account":       accountID,
				"gateway":       gatewayID,
				"body":          body,
				"method":        "POST",
				"path":          "/accounts/{account_id}/ai-gateway/gateways",
				"usage_path":    "/accounts/{account_id}/ai-gateway/billing/usage-history",
				"requires_flag": "--confirm",
			}
			if flags.dryRun || !confirmWrite(flags, confirm) {
				return printWorkflowResult(cmd, flags, plan)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := c.Post(cmd.Context(), "/accounts/"+url.PathEscape(accountID)+"/ai-gateway/gateways", body)
			if err != nil {
				return err
			}
			return printWorkflowResult(cmd, flags, map[string]any{"status": status, "response": json.RawMessage(data)})
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&gatewayID, "gateway", "", "AI Gateway ID")
	cmd.Flags().StringVar(&name, "name", "", "AI Gateway display name")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Create the AI Gateway")
	return cmd
}

func newCostWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "cost", Short: "Inspect Cloudflare cost and usage", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newCostScanCmd(flags, "scan", "Read billable usage and AI Gateway usage signals"))
	cmd.AddCommand(newCostScanCmd(flags, "gun", "Alias for cost scan"))
	return cmd
}

func newCostScanCmd(flags *rootFlags, use, short string) *cobra.Command {
	var accountID, since, until string
	cmd := &cobra.Command{
		Use:         use,
		Short:       short,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" {
				return fmt.Errorf("--account is required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			if since != "" {
				params["since"] = since
			}
			if until != "" {
				params["until"] = until
			}
			out := map[string]any{"account": accountID, "since": since, "until": until}
			addProbe(cmd, c, out, "billable_usage", "/accounts/"+url.PathEscape(accountID)+"/billable/usage", params)
			addProbe(cmd, c, out, "paygo_usage", "/accounts/"+url.PathEscape(accountID)+"/paygo-usage", params)
			addProbe(cmd, c, out, "ai_gateway_credit_balance", "/accounts/"+url.PathEscape(accountID)+"/ai-gateway/billing/credit-balance", nil)
			addProbe(cmd, c, out, "ai_gateway_usage", "/accounts/"+url.PathEscape(accountID)+"/ai-gateway/billing/usage-history", params)
			return printWorkflowResult(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&since, "since", "", "Start time/date for usage APIs")
	cmd.Flags().StringVar(&until, "until", "", "End time/date for usage APIs")
	return cmd
}

func newDeployWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "deploy", Short: "Inspect and roll back Cloudflare deployments", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newDeployRollbackCmd(flags))
	return cmd
}

func newDeployRollbackCmd(flags *rootFlags) *cobra.Command {
	var accountID, project, deploymentID string
	var confirm bool
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Roll back a Cloudflare Pages production deployment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" || project == "" || deploymentID == "" {
				return fmt.Errorf("--account, --project, and --deployment are required")
			}
			plan := map[string]any{
				"account":       accountID,
				"project":       project,
				"deployment":    deploymentID,
				"method":        "POST",
				"path":          "/accounts/{account_id}/pages/projects/{project_name}/deployments/{deployment_id}/rollback",
				"requires_flag": "--confirm",
			}
			if flags.dryRun || !confirmWrite(flags, confirm) {
				return printWorkflowResult(cmd, flags, plan)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := "/accounts/" + url.PathEscape(accountID) + "/pages/projects/" + url.PathEscape(project) + "/deployments/" + url.PathEscape(deploymentID) + "/rollback"
			data, status, err := c.Post(cmd.Context(), path, map[string]any{})
			if err != nil {
				return err
			}
			return printWorkflowResult(cmd, flags, map[string]any{"status": status, "response": json.RawMessage(data)})
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&project, "project", "", "Pages project name")
	cmd.Flags().StringVar(&deploymentID, "deployment", "", "Pages deployment ID")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Roll back the deployment")
	return cmd
}

func newRAGWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "rag", Short: "Bootstrap Cloudflare RAG infrastructure", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newRAGBootstrapCmd(flags))
	return cmd
}

func newRAGBootstrapCmd(flags *rootFlags) *cobra.Command {
	var accountID, name string
	var dimensions int
	var confirm bool
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Create R2, D1, Vectorize, and AI Gateway primitives for RAG",
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" || name == "" {
				return fmt.Errorf("--account and --name are required")
			}
			if dimensions == 0 {
				dimensions = 768
			}
			plan := ragPlan(accountID, name, dimensions)
			plan["requires_flag"] = "--confirm"
			if flags.dryRun || !confirmWrite(flags, confirm) {
				return printWorkflowResult(cmd, flags, plan)
			}
			return applyAgentDataStack(cmd, flags, accountID, name, dimensions, true)
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&name, "name", "", "Base name for created resources")
	cmd.Flags().IntVar(&dimensions, "dimensions", 768, "Vector dimensions")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Create the RAG resources")
	return cmd
}

func newMCPWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "mcp", Short: "Deploy MCP servers to Cloudflare", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newMCPDeployCmd(flags))
	return cmd
}

func newMCPDeployCmd(flags *rootFlags) *cobra.Command {
	var accountID, scriptName, route string
	var confirm bool
	cmd := &cobra.Command{
		Use:   "deploy <worker-file>",
		Short: "Deploy a Worker-hosted MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" || scriptName == "" {
				return fmt.Errorf("--account and --script are required")
			}
			plan := map[string]any{
				"account":       accountID,
				"script":        scriptName,
				"route":         route,
				"worker_file":   args[0],
				"steps":         []string{"PUT /accounts/{account_id}/workers/scripts/{script_name}", "optional domain connect --target worker"},
				"requires_flag": "--confirm",
			}
			if flags.dryRun || !confirmWrite(flags, confirm) {
				return printWorkflowResult(cmd, flags, plan)
			}
			status, body, err := putWorkerScript(cmd, flags, accountID, scriptName, args[0])
			if err != nil {
				return err
			}
			return printWorkflowResult(cmd, flags, map[string]any{"status": status, "response": json.RawMessage(body), "route": route})
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&scriptName, "script", "", "Worker script name")
	cmd.Flags().StringVar(&route, "route", "", "Optional route hostname/path")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Deploy the MCP Worker")
	return cmd
}

func newAgentMemoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "memory", Short: "Manage agent memory infrastructure", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newAgentMemoryBootstrapCmd(flags))
	return cmd
}

func newAgentMemoryBootstrapCmd(flags *rootFlags) *cobra.Command {
	var accountID, name string
	var dimensions int
	var confirm bool
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Create D1, Vectorize, KV, R2, and queue resources for agent memory",
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" || name == "" {
				return fmt.Errorf("--account and --name are required")
			}
			if dimensions == 0 {
				dimensions = 768
			}
			plan := agentMemoryPlan(accountID, name, dimensions)
			plan["requires_flag"] = "--confirm"
			if flags.dryRun || !confirmWrite(flags, confirm) {
				return printWorkflowResult(cmd, flags, plan)
			}
			return applyAgentDataStack(cmd, flags, accountID, name, dimensions, false)
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&name, "name", "", "Base name for created resources")
	cmd.Flags().IntVar(&dimensions, "dimensions", 768, "Vector dimensions")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Create agent memory resources")
	return cmd
}

func projectLaunchSteps(mode, domain string) []string {
	steps := []string{"inspect output", "resolve recipe " + recipeForDeployMode(mode)}
	if mode == "workers" {
		steps = append(steps, "worker deploy")
	} else {
		steps = append(steps, "Pages Direct Upload")
	}
	if domain != "" {
		steps = append(steps, "domain connect")
	}
	return steps
}

func deployPagesDirect(cmd *cobra.Command, flags *rootFlags, accountID, project, dir, branch, domain string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	files, err := collectFiles(dir)
	if err != nil {
		return err
	}
	fields := map[string]string{"branch": branch}
	fileFields := map[string]string{}
	for _, file := range files {
		rel, _ := filepath.Rel(dir, file)
		fileFields[filepath.ToSlash(rel)] = file
	}
	data, status, err := c.PostMultipart(cmd.Context(), "/accounts/"+url.PathEscape(accountID)+"/pages/projects/"+url.PathEscape(project)+"/deployments", fields, fileFields)
	if err != nil {
		return err
	}
	out := map[string]any{"status": status, "response": json.RawMessage(data), "branch": branch}
	if domain != "" {
		domainData, domainStatus, domainErr := c.Post(cmd.Context(), "/accounts/"+url.PathEscape(accountID)+"/pages/projects/"+url.PathEscape(project)+"/domains", map[string]any{"name": domain})
		if domainErr != nil {
			out["domain_error"] = domainErr.Error()
		} else {
			out["domain_status"] = domainStatus
			out["domain_response"] = json.RawMessage(domainData)
		}
	}
	return printWorkflowResult(cmd, flags, out)
}

func addProbe(cmd *cobra.Command, c interface {
	GetNoCache(context.Context, string, map[string]string) (json.RawMessage, error)
}, out map[string]any, name, path string, params map[string]string) {
	data, err := c.GetNoCache(cmd.Context(), path, params)
	if err != nil {
		out[name+"_error"] = err.Error()
		return
	}
	out[name] = json.RawMessage(data)
}

func domainConnectPlan(hostname, target, project, script, tunnelID, bucket, service string) map[string]any {
	steps := []string{}
	switch target {
	case "pages":
		steps = append(steps, "POST /accounts/{account_id}/pages/projects/{project_name}/domains")
	case "worker":
		steps = append(steps, "POST /zones/{zone_id}/workers/routes")
	case "tunnel":
		steps = append(steps, "POST /zones/{zone_id}/dns_records CNAME "+hostname+" -> "+tunnelID+".cfargotunnel.com")
	case "r2":
		steps = append(steps, "POST /accounts/{account_id}/r2/buckets/{bucket_name}/domains/custom")
	default:
		steps = append(steps, "choose --target pages|worker|tunnel|r2")
	}
	return map[string]any{"hostname": hostname, "target": target, "project": project, "script": script, "tunnel": tunnelID, "bucket": bucket, "service": service, "steps": steps}
}

func putWorkerScript(cmd *cobra.Command, flags *rootFlags, accountID, scriptName, file string) (int, []byte, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return 0, nil, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPut, strings.TrimRight(cfg.BaseURL, "/")+"/accounts/"+url.PathEscape(accountID)+"/workers/scripts/"+url.PathEscape(scriptName), bytes.NewReader(data))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", cfg.AuthHeader())
	req.Header.Set("Content-Type", contentTypeForFile(file))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return resp.StatusCode, body, fmt.Errorf("PUT Worker script returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	return resp.StatusCode, body, nil
}

func readWorkerSecretValue(value, fromEnv, fromFile string, fromStdin bool) (string, string, error) {
	sources := 0
	for _, candidate := range []string{value, fromEnv, fromFile} {
		if candidate != "" {
			sources++
		}
	}
	if fromStdin {
		sources++
	}
	if sources == 0 {
		return "", "", fmt.Errorf("one of --value, --from-env, --from-file, or --from-stdin is required")
	}
	if sources > 1 {
		return "", "", fmt.Errorf("use only one of --value, --from-env, --from-file, or --from-stdin")
	}
	switch {
	case value != "":
		return value, "flag", nil
	case fromEnv != "":
		envValue, ok := os.LookupEnv(fromEnv)
		if !ok {
			return "", "", fmt.Errorf("environment variable %s is not set", fromEnv)
		}
		return envValue, "env:" + fromEnv, nil
	case fromFile != "":
		data, err := os.ReadFile(fromFile)
		if err != nil {
			return "", "", err
		}
		return strings.TrimRight(string(data), "\r\n"), "file:" + fromFile, nil
	case fromStdin:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", "", err
		}
		return strings.TrimRight(string(data), "\r\n"), "stdin", nil
	default:
		return "", "", fmt.Errorf("unreachable secret source")
	}
}

func ragPlan(accountID, name string, dimensions int) map[string]any {
	return map[string]any{
		"account":    accountID,
		"name":       name,
		"dimensions": dimensions,
		"resources": map[string]string{
			"r2_bucket":    name + "-assets",
			"d1_database":  name + "-db",
			"vector_index": name + "-vectors",
			"ai_gateway":   name + "-gateway",
		},
		"steps": []string{
			"POST /accounts/{account_id}/r2/buckets",
			"POST /accounts/{account_id}/d1/database",
			"POST /accounts/{account_id}/vectorize/v2/indexes",
			"POST /accounts/{account_id}/ai-gateway/gateways",
			"worker deploy with R2/D1/Vectorize bindings",
		},
	}
}

func agentMemoryPlan(accountID, name string, dimensions int) map[string]any {
	plan := ragPlan(accountID, name, dimensions)
	plan["resources"].(map[string]string)["kv_namespace"] = name + "-kv"
	plan["resources"].(map[string]string)["queue"] = name + "-events"
	plan["steps"] = append(plan["steps"].([]string),
		"POST /accounts/{account_id}/storage/kv/namespaces",
		"POST /accounts/{account_id}/queues",
	)
	return plan
}

func applyAgentDataStack(cmd *cobra.Command, flags *rootFlags, accountID, name string, dimensions int, ragOnly bool) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	out := map[string]any{"account": accountID, "name": name}
	applyPost(cmd, c, out, "r2_bucket", "/accounts/"+url.PathEscape(accountID)+"/r2/buckets", map[string]any{"name": name + "-assets"})
	applyPost(cmd, c, out, "d1_database", "/accounts/"+url.PathEscape(accountID)+"/d1/database", map[string]any{"name": name + "-db"})
	applyPost(cmd, c, out, "vector_index", "/accounts/"+url.PathEscape(accountID)+"/vectorize/v2/indexes", map[string]any{"name": name + "-vectors", "config": map[string]any{"dimensions": dimensions, "metric": "cosine"}})
	applyPost(cmd, c, out, "ai_gateway", "/accounts/"+url.PathEscape(accountID)+"/ai-gateway/gateways", map[string]any{"id": name + "-gateway", "name": name + " Gateway"})
	if !ragOnly {
		applyPost(cmd, c, out, "kv_namespace", "/accounts/"+url.PathEscape(accountID)+"/storage/kv/namespaces", map[string]any{"title": name + "-kv"})
		applyPost(cmd, c, out, "queue", "/accounts/"+url.PathEscape(accountID)+"/queues", map[string]any{"queue_name": name + "-events"})
	}
	return printWorkflowResult(cmd, flags, out)
}

func applyPost(cmd *cobra.Command, c interface {
	Post(context.Context, string, any) (json.RawMessage, int, error)
}, out map[string]any, name, path string, body any) {
	data, status, err := c.Post(cmd.Context(), path, body)
	if err != nil {
		out[name+"_error"] = err.Error()
		return
	}
	out[name+"_status"] = status
	out[name+"_response"] = json.RawMessage(data)
}
