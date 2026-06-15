package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/cloud/cloudflare/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/cloudflare/internal/config"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type cfPermissionNeed struct {
	Scope    string `json:"scope"`
	Name     string `json:"name"`
	Optional bool   `json:"optional,omitempty"`
	ID       string `json:"id,omitempty"`
	Missing  bool   `json:"missing,omitempty"`
}

type cfRecipe struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Needs       []cfPermissionNeed `json:"permissions"`
	Notes       []string           `json:"notes,omitempty"`
}

var cfRecipes = map[string]cfRecipe{
	"pages-static": {
		Name:        "pages-static",
		Description: "Deploy a static site to Cloudflare Pages and attach DNS.",
		Needs: []cfPermissionNeed{
			{Scope: "account", Name: "Cloudflare Pages Edit"},
			{Scope: "zone", Name: "DNS Edit", Optional: true},
		},
	},
	"workers-static": {
		Name:        "workers-static",
		Description: "Deploy a Worker-backed static asset site and attach routes.",
		Needs: []cfPermissionNeed{
			{Scope: "account", Name: "Workers Scripts Edit"},
			{Scope: "account", Name: "Workers Routes Edit"},
			{Scope: "zone", Name: "DNS Edit", Optional: true},
		},
	},
	"site-launch": {
		Name:        "site-launch",
		Description: "Deploy a Pages or Workers static site and attach DNS.",
		Needs: []cfPermissionNeed{
			{Scope: "account", Name: "Cloudflare Pages Edit"},
			{Scope: "account", Name: "Workers Scripts Edit", Optional: true},
			{Scope: "account", Name: "Workers Routes Edit", Optional: true},
			{Scope: "zone", Name: "DNS Edit"},
			{Scope: "account", Name: "Cloudflare Tunnel Edit", Optional: true},
		},
	},
	"tunnels": {
		Name:        "tunnels",
		Description: "Create and operate Cloudflare tunnels and public hostname routes.",
		Needs: []cfPermissionNeed{
			{Scope: "account", Name: "Cloudflare Tunnel Edit"},
			{Scope: "account", Name: "Cloudflare One Connectors Edit", Optional: true},
			{Scope: "zone", Name: "DNS Edit", Optional: true},
		},
	},
	"tunnel-launch": {
		Name:        "tunnel-launch",
		Description: "Create a remotely managed tunnel, route a hostname, and prepare cloudflared.",
		Needs: []cfPermissionNeed{
			{Scope: "account", Name: "Cloudflare Tunnel Edit"},
			{Scope: "account", Name: "Cloudflare One Connectors Edit"},
			{Scope: "zone", Name: "DNS Edit"},
		},
	},
	"r2-assets": {
		Name:        "r2-assets",
		Description: "Create buckets and upload static assets to R2.",
		Needs: []cfPermissionNeed{
			{Scope: "account", Name: "Workers R2 Storage Edit"},
			{Scope: "account", Name: "Workers Scripts Edit", Optional: true},
		},
	},
	"email-routing": {
		Name:        "email-routing",
		Description: "Configure Email Routing destinations, routing rules, and DNS readiness.",
		Needs: []cfPermissionNeed{
			{Scope: "zone", Name: "Email Routing Edit"},
			{Scope: "zone", Name: "DNS Edit"},
		},
	},
	"agent-admin": {
		Name:        "agent-admin",
		Description: "Bootstrap an agent operator token that can mint scoped tokens for other agents.",
		Needs: []cfPermissionNeed{
			{Scope: "user", Name: "API Tokens Read"},
			{Scope: "user", Name: "API Tokens Write"},
		},
		Notes: []string{
			"Use this token only for token doctor/create/rotate workflows; give workload agents recipe-specific tokens.",
			"Store the token in 1Password, an auth store, or a 0600 env file, not in application source.",
			"Cloudflare may require creating the first token-management token in the dashboard before API token creation is available.",
		},
	},
}

type cfPermissionGroup struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
	Scope  string   `json:"scope"`
}

func newTokenWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "token", Short: "Plan and create scoped Cloudflare API tokens", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newTokenRecipeCmd(flags))
	cmd.AddCommand(newTokenCreateCmd(flags))
	cmd.AddCommand(newTokenDoctorCmd(flags))
	cmd.AddCommand(newTokenRotateCmd(flags))
	return cmd
}

func newTokenRecipeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "recipe <name>",
		Short:       "Print least-privilege permission groups for a workflow",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			recipe, ok := cfRecipes[args[0]]
			if !ok {
				return fmt.Errorf("unknown recipe %q; available: %s", args[0], strings.Join(sortedRecipeNames(), ", "))
			}
			resolved, err := resolveRecipePermissions(cmd, flags, recipe)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: permission IDs unresolved: %v\n", err)
				resolved = recipe
			}
			return printWorkflowResult(cmd, flags, resolved)
		},
	}
	return cmd
}

func newTokenCreateCmd(flags *rootFlags) *cobra.Command {
	var recipeName, accountID, tokenName, writeEnv string
	var confirm, showToken bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create or guide creation of a scoped Cloudflare API token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if recipeName == "" {
				return fmt.Errorf("required flag \"recipe\" not set")
			}
			recipe, ok := cfRecipes[recipeName]
			if !ok {
				return fmt.Errorf("unknown recipe %q; available: %s", recipeName, strings.Join(sortedRecipeNames(), ", "))
			}
			if tokenName == "" {
				tokenName = "cloudflare " + recipeName
			}
			return createTokenFromRecipe(cmd, flags, recipeName, accountID, tokenName, writeEnv, confirm, showToken, recipe)
		},
	}
	cmd.Flags().StringVar(&recipeName, "recipe", "", "Token recipe name")
	cmd.Flags().StringVar(&accountID, "account", "", "Account ID to scope account resources")
	cmd.Flags().StringVar(&tokenName, "name", "", "Token name")
	cmd.Flags().StringVar(&writeEnv, "write-env", "", "Write the created token to a 0600 env file")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Create the token")
	cmd.Flags().BoolVar(&showToken, "show-token", false, "Print the created token value once")
	return cmd
}

func newTokenDoctorCmd(flags *rootFlags) *cobra.Command {
	var recipeName, accountID string
	cmd := &cobra.Command{
		Use:         "doctor",
		Short:       "Compare the current Cloudflare token with a workflow recipe",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			report := map[string]any{
				"account": accountID,
			}
			verify, verifyErr := c.GetNoCache(cmd.Context(), "/user/tokens/verify", nil)
			report["verify_ok"] = verifyErr == nil
			if verifyErr != nil {
				report["verify_error"] = verifyErr.Error()
			} else {
				report["verify"] = json.RawMessage(verify)
			}
			groups, groupsErr := cachedPermissionGroups(cmd, c)
			report["permission_groups_ok"] = groupsErr == nil
			if groupsErr != nil {
				report["permission_groups_error"] = groupsErr.Error()
			} else {
				report["permission_groups_count"] = len(groups)
			}
			if recipeName != "" {
				recipe, ok := cfRecipes[recipeName]
				if !ok {
					return fmt.Errorf("unknown recipe %q; available: %s", recipeName, strings.Join(sortedRecipeNames(), ", "))
				}
				resolved, err := resolveRecipePermissions(cmd, flags, recipe)
				report["recipe"] = resolved
				report["recipe_ready"] = err == nil && !hasMissingPermission(resolved)
				if err != nil {
					report["recipe_error"] = err.Error()
				}
			}
			return printWorkflowResult(cmd, flags, report)
		},
	}
	cmd.Flags().StringVar(&recipeName, "recipe", "", "Recipe name to check")
	cmd.Flags().StringVar(&accountID, "account", "", "Account ID to include in the report")
	return cmd
}

func newTokenRotateCmd(flags *rootFlags) *cobra.Command {
	var writeEnv string
	var confirm, showToken bool
	cmd := &cobra.Command{
		Use:   "rotate <token-id>",
		Short: "Roll an existing Cloudflare API token secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tokenID := args[0]
			plan := map[string]any{
				"method":        "PUT",
				"path":          "/user/tokens/{token_id}/value",
				"token_id":      tokenID,
				"write_env":     writeEnv,
				"secret_output": "redacted unless --show-token is set",
			}
			if flags.dryRun || !confirmWrite(flags, confirm) {
				plan["requires_flag"] = "--confirm"
				return printWorkflowResult(cmd, flags, plan)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := c.Put(cmd.Context(), "/user/tokens/"+url.PathEscape(tokenID)+"/value", map[string]any{})
			if err != nil {
				return err
			}
			tokenValue := firstString(data, "value", "token")
			out := map[string]any{
				"status":        status,
				"token_id":      tokenID,
				"token":         redacted(tokenValue),
				"token_present": tokenValue != "",
			}
			if writeEnv != "" {
				if tokenValue == "" {
					return fmt.Errorf("token rotation succeeded but response did not contain a token value to write")
				}
				if err := os.WriteFile(writeEnv, []byte("CLOUDFLARE_API_TOKEN="+tokenValue+"\n"), 0o600); err != nil {
					return err
				}
				out["wrote_env"] = writeEnv
			}
			if showToken {
				out["token"] = tokenValue
				out["token_warning"] = "printed once because --show-token was set"
			}
			return printWorkflowResult(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&writeEnv, "write-env", "", "Write the rotated token to a 0600 env file")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Roll the token secret")
	cmd.Flags().BoolVar(&showToken, "show-token", false, "Print the rotated token value once")
	return cmd
}

func newTunnelWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "tunnel", Short: "Launch Cloudflare tunnels", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newTunnelLaunchCmd(flags))
	cmd.AddCommand(newTunnelStatusCmd(flags))
	return cmd
}

func newTunnelLaunchCmd(flags *rootFlags) *cobra.Command {
	var accountID, zoneID, hostname, service, name string
	var confirm, noOpen bool
	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Create a remotely managed tunnel and route a hostname",
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" || hostname == "" || service == "" {
				return fmt.Errorf("--account, --hostname, and --service are required")
			}
			if name == "" {
				name = strings.ReplaceAll(hostname, ".", "-")
			}
			plan := []string{
				"POST /accounts/{account_id}/cfd_tunnel",
				"PUT /accounts/{account_id}/cfd_tunnel/{tunnel_id}/configurations",
				"GET /accounts/{account_id}/cfd_tunnel/{tunnel_id}/token",
				"optional POST /zones/{zone_id}/dns_records",
			}
			if flags.dryRun || !confirmWrite(flags, confirm) {
				return printWorkflowResult(cmd, flags, map[string]any{"plan": plan, "requires_flag": "--confirm", "no_open": noOpen})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			createBody := map[string]any{"name": name, "config_src": "cloudflare"}
			created, _, err := c.Post(cmd.Context(), "/accounts/"+url.PathEscape(accountID)+"/cfd_tunnel", createBody)
			if err != nil {
				return err
			}
			tunnelID := firstString(created, "id")
			if tunnelID == "" {
				return fmt.Errorf("tunnel created but response did not contain result.id")
			}
			configBody := map[string]any{"config": map[string]any{"ingress": []map[string]string{{"hostname": hostname, "service": service}, {"service": "http_status:404"}}}}
			if _, _, err := c.Put(cmd.Context(), "/accounts/"+url.PathEscape(accountID)+"/cfd_tunnel/"+url.PathEscape(tunnelID)+"/configurations", configBody); err != nil {
				return err
			}
			if zoneID != "" {
				dnsBody := map[string]any{"type": "CNAME", "name": hostname, "content": tunnelID + ".cfargotunnel.com", "proxied": true}
				if _, _, err := c.Post(cmd.Context(), "/zones/"+url.PathEscape(zoneID)+"/dns_records", dnsBody); err != nil {
					return err
				}
			}
			token, err := c.GetNoCache(cmd.Context(), "/accounts/"+url.PathEscape(accountID)+"/cfd_tunnel/"+url.PathEscape(tunnelID)+"/token", nil)
			if err != nil {
				return err
			}
			return printWorkflowResult(cmd, flags, map[string]any{
				"tunnel_id": tunnelID,
				"hostname":  hostname,
				"service":   service,
				"run":       "cloudflared tunnel run --token " + redacted(firstString(token, "token", "value")),
			})
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&zoneID, "zone", "", "Zone ID for DNS record creation")
	cmd.Flags().StringVar(&hostname, "hostname", "", "Public hostname to route")
	cmd.Flags().StringVar(&service, "service", "", "Origin service URL, for example http://localhost:3000")
	cmd.Flags().StringVar(&name, "name", "", "Tunnel name")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Create and configure the tunnel")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Do not open dashboard URLs")
	return cmd
}

func newTunnelStatusCmd(flags *rootFlags) *cobra.Command {
	var accountID, tunnelID string
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Inspect a Cloudflare tunnel and connector status",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" || tunnelID == "" {
				return fmt.Errorf("--account and --tunnel are required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			basePath := "/accounts/" + url.PathEscape(accountID) + "/cfd_tunnel/" + url.PathEscape(tunnelID)
			tunnel, tunnelErr := c.GetNoCache(cmd.Context(), basePath, nil)
			connectors, connectorErr := c.GetNoCache(cmd.Context(), basePath+"/connections", nil)
			out := map[string]any{
				"account": accountID,
				"tunnel":  tunnelID,
			}
			if tunnelErr != nil {
				out["tunnel_error"] = tunnelErr.Error()
			} else {
				out["tunnel_response"] = json.RawMessage(tunnel)
			}
			if connectorErr != nil {
				out["connections_error"] = connectorErr.Error()
			} else {
				out["connections_response"] = json.RawMessage(connectors)
			}
			return printWorkflowResult(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&tunnelID, "tunnel", "", "Tunnel ID")
	return cmd
}

func newSiteWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "site", Short: "Deploy static sites to Cloudflare", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newSitePlanCmd(flags))
	cmd.AddCommand(newSiteDeployCmd(flags))
	return cmd
}

func newSitePlanCmd(flags *rootFlags) *cobra.Command {
	var accountID, project, domain, mode string
	cmd := &cobra.Command{
		Use:         "plan [dir]",
		Short:       "Inspect a static site output and choose Pages or Workers",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			dir = detectSiteDir(dir)
			files, err := collectFiles(dir)
			if err != nil {
				return err
			}
			detected := detectDeployMode(dir)
			chosen := mode
			if chosen == "" || chosen == "auto" {
				chosen = detected
			}
			return printWorkflowResult(cmd, flags, map[string]any{
				"account":        accountID,
				"project":        project,
				"domain":         domain,
				"dir":            dir,
				"files":          len(files),
				"detected_mode":  detected,
				"selected_mode":  chosen,
				"recipe":         recipeForDeployMode(chosen),
				"next_command":   siteDeployCommand(dir, project, accountID, domain, chosen),
				"mutates":        false,
				"worker_config":  hasWorkerConfig(dir),
				"pages_default":  chosen == "pages",
				"workers_assets": chosen == "workers",
			})
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&project, "project", "", "Pages project or Worker script name")
	cmd.Flags().StringVar(&domain, "domain", "", "Custom domain to attach")
	cmd.Flags().StringVar(&mode, "mode", "auto", "Deployment mode: auto, pages, or workers")
	return cmd
}

func newSiteDeployCmd(flags *rootFlags) *cobra.Command {
	var accountID, project, domain, mode string
	var confirm, noOpen bool
	cmd := &cobra.Command{
		Use:   "deploy <dir>",
		Short: "Plan a Pages Direct Upload or Workers assets deployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := detectSiteDir(args[0])
			if project == "" {
				return fmt.Errorf("--project is required")
			}
			if accountID == "" {
				return fmt.Errorf("--account is required")
			}
			files, err := collectFiles(dir)
			if err != nil {
				return err
			}
			if mode == "" || mode == "auto" {
				mode = detectDeployMode(dir)
			}
			plan := map[string]any{"mode": mode, "dir": dir, "files": len(files), "project": project, "domain": domain, "no_open": noOpen}
			if flags.dryRun || !confirmWrite(flags, confirm) {
				plan["requires_flag"] = "--confirm"
				return printWorkflowResult(cmd, flags, plan)
			}
			if mode == "workers" {
				return fmt.Errorf("workers static asset direct upload needs the Workers asset upload protocol; use --dry-run to inspect the plan")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			fields := map[string]string{"branch": "production"}
			fileFields := map[string]string{}
			for _, file := range files {
				rel, _ := filepath.Rel(dir, file)
				fileFields[filepath.ToSlash(rel)] = file
			}
			data, status, err := c.PostMultipart(cmd.Context(), "/accounts/"+url.PathEscape(accountID)+"/pages/projects/"+url.PathEscape(project)+"/deployments", fields, fileFields)
			if err != nil {
				return err
			}
			if domain != "" {
				_, _, _ = c.Post(cmd.Context(), "/accounts/"+url.PathEscape(accountID)+"/pages/projects/"+url.PathEscape(project)+"/domains", map[string]any{"name": domain})
			}
			return printWorkflowResult(cmd, flags, map[string]any{"status": status, "response": json.RawMessage(data)})
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&project, "project", "", "Pages project name")
	cmd.Flags().StringVar(&domain, "domain", "", "Custom domain to attach after deploy")
	cmd.Flags().StringVar(&mode, "mode", "", "Deployment mode: pages or workers")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Upload the deployment")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Do not open dashboard URLs")
	return cmd
}

func newR2WorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "r2", Short: "Manage R2 assets", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newR2PutCmd(flags, "put"))
	cmd.AddCommand(newR2PutCmd(flags, "sync"))
	return cmd
}

func newR2PutCmd(flags *rootFlags, use string) *cobra.Command {
	var accountID, bucket, prefix string
	var confirm, noOpen bool
	cmd := &cobra.Command{
		Use:   use + " <path>",
		Short: "Upload a file or directory to R2 object storage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" || bucket == "" {
				return fmt.Errorf("--account and --bucket are required")
			}
			files, err := collectFiles(args[0])
			if err != nil {
				return err
			}
			if flags.dryRun || !confirmWrite(flags, confirm) {
				return printWorkflowResult(cmd, flags, map[string]any{"files": len(files), "bucket": bucket, "prefix": prefix, "requires_flag": "--confirm", "no_open": noOpen})
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			var uploaded []string
			base := strings.TrimRight(cfg.BaseURL, "/")
			for _, file := range files {
				key := objectKey(args[0], file, prefix)
				if err := putR2Object(cmd, cfg, base, accountID, bucket, key, file); err != nil {
					return err
				}
				uploaded = append(uploaded, key)
			}
			return printWorkflowResult(cmd, flags, map[string]any{"uploaded": uploaded, "bucket": bucket})
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().StringVar(&bucket, "bucket", "", "R2 bucket name")
	cmd.Flags().StringVar(&prefix, "prefix", "", "Object key prefix")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Upload objects")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Do not open dashboard URLs")
	return cmd
}

func newEmailWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "email", Short: "Configure Cloudflare email services", RunE: parentNoSubcommandRunE(flags)}
	route := &cobra.Command{Use: "route", Short: "Manage email routes", RunE: parentNoSubcommandRunE(flags)}
	route.AddCommand(newEmailRouteCreateCmd(flags))
	cmd.AddCommand(route)
	return cmd
}

func newEmailRouteCreateCmd(flags *rootFlags) *cobra.Command {
	var zoneID, to string
	var confirm, noOpen bool
	cmd := &cobra.Command{
		Use:   "create <address>",
		Short: "Create an Email Routing forward rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if zoneID == "" || to == "" {
				return fmt.Errorf("--zone and --to are required")
			}
			body := map[string]any{
				"name":    "forward " + args[0],
				"enabled": true,
				"matchers": []map[string]string{{
					"type":  "literal",
					"field": "to",
					"value": args[0],
				}},
				"actions": []map[string]any{{
					"type":  "forward",
					"value": []string{to},
				}},
			}
			if flags.dryRun || !confirmWrite(flags, confirm) {
				return printWorkflowResult(cmd, flags, map[string]any{"method": "POST", "path": "/zones/{zone_id}/email/routing/rules", "body": body, "requires_flag": "--confirm", "no_open": noOpen})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := c.Post(cmd.Context(), "/zones/"+url.PathEscape(zoneID)+"/email/routing/rules", body)
			if err != nil {
				return err
			}
			return printWorkflowResult(cmd, flags, map[string]any{"status": status, "response": json.RawMessage(data)})
		},
	}
	cmd.Flags().StringVar(&zoneID, "zone", "", "Cloudflare zone ID")
	cmd.Flags().StringVar(&to, "to", "", "Destination address")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Create the route")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Do not open dashboard URLs")
	return cmd
}

func newAgentWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "agent", Short: "Prepare agent Cloudflare credentials", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newAgentAdminCmd(flags))
	cmd.AddCommand(newAgentSetupCmd(flags))
	cmd.AddCommand(newAgentMemoryCmd(flags))
	return cmd
}

func newAgentAdminCmd(flags *rootFlags) *cobra.Command {
	var accountID, tokenName, writeEnv string
	var confirm, showToken bool
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Create or guide an agent-admin token for minting scoped agent tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			recipe := cfRecipes["agent-admin"]
			if tokenName == "" {
				tokenName = "cloudflare agent admin"
			}
			return createTokenFromRecipe(cmd, flags, "agent-admin", accountID, tokenName, writeEnv, confirm, showToken, recipe)
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Optional account ID to include in resource scoping")
	cmd.Flags().StringVar(&tokenName, "name", "", "Token name")
	cmd.Flags().StringVar(&writeEnv, "write-env", "", "Write the created token to a 0600 env file")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Create the token")
	cmd.Flags().BoolVar(&showToken, "show-token", false, "Print the created token value once")
	return cmd
}

func newAgentSetupCmd(flags *rootFlags) *cobra.Command {
	var recipeName, writeEnv, accountID string
	var confirm bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Print or write agent setup material for a workflow recipe",
		RunE: func(cmd *cobra.Command, args []string) error {
			if recipeName == "" {
				return fmt.Errorf("--recipe is required")
			}
			recipe, ok := cfRecipes[recipeName]
			if !ok {
				return fmt.Errorf("unknown recipe %q; available: %s", recipeName, strings.Join(sortedRecipeNames(), ", "))
			}
			resolved, _ := resolveRecipePermissions(cmd, flags, recipe)
			out := map[string]any{"recipe": resolved, "account": accountID, "env": map[string]string{"CLOUDFLARE_API_TOKEN": "<scoped-token>"}}
			if writeEnv == "" || flags.dryRun || !confirmWrite(flags, confirm) {
				out["write_env"] = writeEnv
				out["requires_flag"] = "--confirm"
				return printWorkflowResult(cmd, flags, out)
			}
			content := "CLOUDFLARE_API_TOKEN=\n"
			if err := os.WriteFile(writeEnv, []byte(content), 0o600); err != nil {
				return err
			}
			out["wrote"] = writeEnv
			return printWorkflowResult(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&recipeName, "recipe", "", "Recipe name")
	cmd.Flags().StringVar(&writeEnv, "write-env", "", "Path to write a 0600 env file")
	cmd.Flags().StringVar(&accountID, "account", "", "Cloudflare account ID")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Write files")
	return cmd
}

func createTokenFromRecipe(cmd *cobra.Command, flags *rootFlags, recipeName, accountID, tokenName, writeEnv string, confirm, showToken bool, recipe cfRecipe) error {
	resolved, err := resolveRecipePermissions(cmd, flags, recipe)
	if err != nil || hasMissingPermission(resolved) {
		return printTokenDashboardFallback(cmd, flags, recipeName, accountID, resolved, err)
	}
	body := map[string]any{
		"name": tokenName,
		"policies": []map[string]any{{
			"effect":            "allow",
			"permission_groups": permissionGroupRefs(resolved),
			"resources":         tokenResources(accountID),
		}},
	}
	if flags.dryRun || !confirmWrite(flags, confirm) {
		return printWorkflowResult(cmd, flags, map[string]any{
			"dry_run":       flags.dryRun,
			"requires_flag": "--confirm",
			"method":        "POST",
			"path":          "/user/tokens",
			"recipe":        resolved,
			"body":          body,
			"secret_output": "redacted unless --show-token is set or --write-env is used",
			"write_env":     writeEnv,
		})
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	data, status, err := c.Post(cmd.Context(), "/user/tokens", body)
	if err != nil {
		return printTokenDashboardFallback(cmd, flags, recipeName, accountID, resolved, err)
	}
	tokenValue := firstString(data, "value", "token")
	out := map[string]any{
		"status":        status,
		"recipe":        recipeName,
		"token":         redacted(tokenValue),
		"token_present": tokenValue != "",
		"secret_output": "redacted",
	}
	if writeEnv != "" {
		if tokenValue == "" {
			return fmt.Errorf("token created but response did not contain a token value to write")
		}
		if err := os.WriteFile(writeEnv, []byte("CLOUDFLARE_API_TOKEN="+tokenValue+"\n"), 0o600); err != nil {
			return err
		}
		out["wrote_env"] = writeEnv
	}
	if showToken {
		out["token"] = tokenValue
		out["token_warning"] = "printed once because --show-token was set"
	}
	return printWorkflowResult(cmd, flags, out)
}

func resolveRecipePermissions(cmd *cobra.Command, flags *rootFlags, recipe cfRecipe) (cfRecipe, error) {
	c, err := flags.newClient()
	if err != nil {
		return recipe, err
	}
	c.DryRun = false
	data, err := cachedPermissionGroups(cmd, c)
	if err != nil {
		return recipe, err
	}
	byName := map[string]cfPermissionGroup{}
	for _, group := range data {
		byName[strings.ToLower(group.Name)] = group
	}
	for i := range recipe.Needs {
		group, ok := byName[strings.ToLower(recipe.Needs[i].Name)]
		if !ok && strings.HasSuffix(recipe.Needs[i].Name, " Edit") {
			group, ok = byName[strings.ToLower(strings.TrimSuffix(recipe.Needs[i].Name, " Edit")+" Write")]
		}
		if ok {
			recipe.Needs[i].ID = group.ID
		} else {
			recipe.Needs[i].Missing = true
		}
	}
	return recipe, nil
}

func cachedPermissionGroups(cmd *cobra.Command, c *client.Client) ([]cfPermissionGroup, error) {
	cachePath, err := permissionGroupCachePath()
	if err == nil {
		if info, statErr := os.Stat(cachePath); statErr == nil && time.Since(info.ModTime()) < 24*time.Hour {
			if groups, readErr := readPermissionGroupCache(cachePath); readErr == nil {
				return groups, nil
			}
		}
	}
	raw, err := c.GetNoCache(cmd.Context(), "/user/tokens/permission_groups", nil)
	if err != nil {
		return nil, err
	}
	groups, err := parsePermissionGroups(raw)
	if err != nil {
		return nil, err
	}
	if cachePath != "" {
		_ = os.MkdirAll(filepath.Dir(cachePath), 0o700)
		if data, marshalErr := json.MarshalIndent(groups, "", "  "); marshalErr == nil {
			_ = os.WriteFile(cachePath, data, 0o600)
		}
	}
	return groups, nil
}

func permissionGroupCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "cloudflare-pp-cli", "permission-groups.json"), nil
}

func readPermissionGroupCache(path string) ([]cfPermissionGroup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var groups []cfPermissionGroup
	if err := json.Unmarshal(data, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func parsePermissionGroups(raw json.RawMessage) ([]cfPermissionGroup, error) {
	var envelope struct {
		Result []cfPermissionGroup `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Result != nil {
		return envelope.Result, nil
	}
	var groups []cfPermissionGroup
	if err := json.Unmarshal(raw, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func sortedRecipeNames() []string {
	names := make([]string, 0, len(cfRecipes))
	for name := range cfRecipes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func printWorkflowResult(cmd *cobra.Command, flags *rootFlags, v any) error {
	if flags.asJSON {
		return flags.printJSON(cmd, v)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func confirmWrite(flags *rootFlags, confirm bool) bool {
	return confirm || flags.yes
}

func hasMissingPermission(recipe cfRecipe) bool {
	for _, need := range recipe.Needs {
		if need.Missing && !need.Optional {
			return true
		}
	}
	return false
}

func permissionGroupRefs(recipe cfRecipe) []map[string]string {
	var refs []map[string]string
	for _, need := range recipe.Needs {
		if need.ID != "" && !need.Optional {
			refs = append(refs, map[string]string{"id": need.ID})
		}
	}
	return refs
}

func tokenResources(accountID string) map[string]string {
	resources := map[string]string{"com.cloudflare.api.user.*": "*"}
	if accountID != "" {
		resources["com.cloudflare.api.account."+accountID] = "*"
	}
	return resources
}

func printTokenDashboardFallback(cmd *cobra.Command, flags *rootFlags, recipeName, accountID string, recipe cfRecipe, cause error) error {
	out := map[string]any{
		"created":       false,
		"reason":        "token API unavailable or current token lacks API Tokens Write",
		"dashboard_url": "https://dash.cloudflare.com/profile/api-tokens",
		"recipe":        recipeName,
		"account":       accountID,
		"permissions":   recipe.Needs,
		"notes":         recipe.Notes,
		"checklist":     tokenDashboardChecklist(recipe),
		"next_steps": []string{
			"Create the token in the Cloudflare dashboard using the checklist above.",
			"Store the value in 1Password or write it to a 0600 env file.",
			"Run token doctor --recipe " + recipeName + " with the new token before handing it to an agent.",
		},
	}
	if cause != nil {
		out["error"] = cause.Error()
	}
	return printWorkflowResult(cmd, flags, out)
}

func tokenDashboardChecklist(recipe cfRecipe) []string {
	checklist := make([]string, 0, len(recipe.Needs))
	for _, need := range recipe.Needs {
		prefix := "Required"
		if need.Optional {
			prefix = "Optional"
		}
		checklist = append(checklist, fmt.Sprintf("%s %s permission: %s", prefix, need.Scope, need.Name))
	}
	return checklist
}

func firstString(raw json.RawMessage, keys ...string) string {
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return ""
	}
	cur := envelope["result"]
	if cur == nil {
		cur = envelope
	}
	for _, key := range keys {
		if m, ok := cur.(map[string]any); ok {
			if s, ok := m[key].(string); ok {
				return s
			}
			cur = m[key]
		}
	}
	if s, ok := cur.(string); ok {
		return s
	}
	return ""
}

func redacted(s string) string {
	if s == "" {
		return "<redacted>"
	}
	return "<redacted:" + fmt.Sprint(len(s)) + " chars>"
}

func detectSiteDir(path string) string {
	if path != "." {
		return path
	}
	for _, candidate := range []string{"dist", "build", "out"} {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
	}
	return path
}

func detectDeployMode(dir string) string {
	for _, name := range []string{"wrangler.toml", "wrangler.json", "_worker.js"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return "workers"
		}
	}
	return "pages"
}

func hasWorkerConfig(dir string) bool {
	return detectDeployMode(dir) == "workers"
}

func recipeForDeployMode(mode string) string {
	if mode == "workers" {
		return "workers-static"
	}
	return "pages-static"
}

func siteDeployCommand(dir, project, accountID, domain, mode string) string {
	parts := []string{"cloudflare-pp-cli", "site", "deploy", dir}
	if project != "" {
		parts = append(parts, "--project", project)
	}
	if accountID != "" {
		parts = append(parts, "--account", accountID)
	}
	if domain != "" {
		parts = append(parts, "--domain", domain)
	}
	if mode != "" {
		parts = append(parts, "--mode", mode)
	}
	parts = append(parts, "--confirm")
	return strings.Join(parts, " ")
}

func collectFiles(root string) ([]string, error) {
	st, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return []string{root}, nil
	}
	var files []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func objectKey(root, file, prefix string) string {
	key := filepath.Base(file)
	if st, err := os.Stat(root); err == nil && st.IsDir() {
		if rel, err := filepath.Rel(root, file); err == nil {
			key = filepath.ToSlash(rel)
		}
	}
	return strings.Trim(strings.Trim(prefix, "/")+"/"+key, "/")
}

func putR2Object(cmd *cobra.Command, cfg *config.Config, base, accountID, bucket, key, file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	escapedKey := strings.Join(escapeSegments(strings.Split(key, "/")), "/")
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPut, base+"/accounts/"+url.PathEscape(accountID)+"/r2/buckets/"+url.PathEscape(bucket)+"/objects/"+escapedKey, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", cfg.AuthHeader())
	req.Header.Set("Content-Type", contentTypeForFile(file))
	req.Header.Set("ETag", contentHash(data))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PUT R2 object %s returned HTTP %d: %s", key, resp.StatusCode, string(body))
	}
	return nil
}

func escapeSegments(parts []string) []string {
	out := make([]string, len(parts))
	for i, part := range parts {
		out[i] = url.PathEscape(part)
	}
	return out
}

func contentTypeForFile(path string) string {
	if ct := mime.TypeByExtension(filepath.Ext(path)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

func contentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
