// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil"
)

const (
	bootstrapActionCreate  = "create"
	bootstrapActionCreated = "created"
	bootstrapActionExists  = "exists"
	bootstrapActionUpdate  = "update"
	bootstrapActionUpdated = "updated"
	bootstrapActionPush    = "push"
	bootstrapActionPushed  = "pushed"
	bootstrapActionWrite   = "write"
	bootstrapActionWritten = "written"
	bootstrapActionError   = "error"

	bootstrapStepServer    = "server"
	bootstrapStepBroadcast = "broadcast-stream"
	bootstrapStepWebhook   = "webhook"
	bootstrapStepDomain    = "domain"
	bootstrapStepTemplates = "templates"
	bootstrapStepEnvFile   = "env-file"

	bootstrapBroadcastID      = "broadcast"
	bootstrapReturnPathTarget = "pm.mtasv.net"

	// Postmark's DeliveryType values for a new server.
	postmarkDeliveryLive    = "Live"
	postmarkDeliverySandbox = "Sandbox"
)

type bootstrapStep struct {
	Step   string `json:"step"`
	Action string `json:"action"`
	Detail string `json:"detail"`
}

type bootstrapDNSRecord struct {
	Type  string `json:"type"`
	Host  string `json:"host"`
	Value string `json:"value"`
}

type bootstrapTemplate struct {
	Action string `json:"action"`
	Alias  string `json:"alias"`
	Name   string `json:"name"`
}

type bootstrapResult struct {
	Name         string               `json:"name"`
	Applied      bool                 `json:"applied"`
	ServerExists bool                 `json:"server_exists"`
	ServerID     int64                `json:"server_id,omitempty"`
	DeliveryType string               `json:"delivery_type"`
	Steps        []bootstrapStep      `json:"steps"`
	DNSRecords   []bootstrapDNSRecord `json:"dns_records"`
	Templates    []bootstrapTemplate  `json:"templates,omitempty"`
	EnvFile      string               `json:"env_file,omitempty"`
	Next         string               `json:"next,omitempty"`
}

type bootstrapDomain struct {
	ID                         int64  `json:"ID"`
	Name                       string `json:"Name"`
	DKIMVerified               bool   `json:"DKIMVerified"`
	DKIMHost                   string `json:"DKIMHost"`
	DKIMTextValue              string `json:"DKIMTextValue"`
	DKIMPendingHost            string `json:"DKIMPendingHost"`
	DKIMPendingTextValue       string `json:"DKIMPendingTextValue"`
	ReturnPathDomain           string `json:"ReturnPathDomain"`
	ReturnPathDomainVerified   bool   `json:"ReturnPathDomainVerified"`
	ReturnPathDomainCNAMEValue string `json:"ReturnPathDomainCNAMEValue"`
	DKIMRevokedHost            string `json:"DKIMRevokedHost"`
	DKIMUpdateStatus           string `json:"DKIMUpdateStatus"`
	SafeToRemoveRevokedKey     bool   `json:"SafeToRemoveRevokedKeyFromDNS"`
	WeakDKIM                   bool   `json:"WeakDKIM"`
}

type bootstrapWebhook struct {
	ID            int64  `json:"ID"`
	URL           string `json:"Url"`
	MessageStream string `json:"MessageStream"`
	Triggers      struct {
		Bounce        bootstrapTrigger `json:"Bounce"`
		SpamComplaint bootstrapTrigger `json:"SpamComplaint"`
		Delivery      bootstrapTrigger `json:"Delivery"`
	} `json:"Triggers"`
}

type bootstrapTrigger struct {
	Enabled        bool  `json:"Enabled"`
	IncludeContent *bool `json:"IncludeContent,omitempty"`
}

// bootstrapWebhookTriggers are the events a bootstrapped webhook must receive,
// in the order they are reported.
var bootstrapWebhookTriggers = []string{"Bounce", "SpamComplaint", "Delivery"}

func (w bootstrapWebhook) trigger(name string) bootstrapTrigger {
	switch name {
	case "Bounce":
		return w.Triggers.Bounce
	case "SpamComplaint":
		return w.Triggers.SpamComplaint
	default:
		return w.Triggers.Delivery
	}
}

// missingTriggers lists the required triggers the webhook has disabled.
func (w bootstrapWebhook) missingTriggers() []string {
	var missing []string
	for _, name := range bootstrapWebhookTriggers {
		if !w.trigger(name).Enabled {
			missing = append(missing, name)
		}
	}
	return missing
}

// triggerUpdate enables only the named triggers and keeps each one's existing
// IncludeContent setting; Postmark leaves triggers omitted from an edit
// unchanged.
func (w bootstrapWebhook) triggerUpdate(names []string) map[string]any {
	body := make(map[string]any, len(names))
	for _, name := range names {
		t := map[string]any{"Enabled": true}
		if ic := w.trigger(name).IncludeContent; ic != nil {
			t["IncludeContent"] = *ic
		}
		body[name] = t
	}
	return body
}

// sameWebhookURL compares scheme and host case-insensitively and everything
// else exactly (userinfo, path including any trailing slash, query including
// an empty one, and fragment), since
// servers may route those differently.
func sameWebhookURL(a, b string) bool {
	ua, errA := url.Parse(strings.TrimSpace(a))
	ub, errB := url.Parse(strings.TrimSpace(b))
	if errA != nil || errB != nil {
		return strings.TrimSpace(a) == strings.TrimSpace(b)
	}
	return strings.EqualFold(ua.Scheme, ub.Scheme) &&
		strings.EqualFold(ua.Host, ub.Host) &&
		ua.User.String() == ub.User.String() &&
		ua.EscapedPath() == ub.EscapedPath() &&
		ua.RawQuery == ub.RawQuery &&
		ua.ForceQuery == ub.ForceQuery &&
		ua.EscapedFragment() == ub.EscapedFragment()
}

type bootstrapOptions struct {
	name, deliveryType, color, domain, returnPath, webhookURL, copyFrom, envFile string
	broadcast, noVerifyWebhook, apply                                            bool
}

// bootstrapDNSRecords lists the DNS records the sending domain needs: the
// DKIM TXT (pending value first, since a new or rotating key verifies against
// it) and the Return-Path CNAME.
func bootstrapDNSRecords(d bootstrapDomain, returnPath string) []bootstrapDNSRecord {
	records := make([]bootstrapDNSRecord, 0, 2)
	host, value := d.DKIMPendingHost, d.DKIMPendingTextValue
	if host == "" {
		host, value = d.DKIMHost, d.DKIMTextValue
	}
	if host != "" {
		records = append(records, bootstrapDNSRecord{Type: "TXT", Host: host, Value: value})
	}
	rp := d.ReturnPathDomain
	if rp == "" {
		rp = returnPath
	}
	cname := d.ReturnPathDomainCNAMEValue
	if cname == "" {
		cname = bootstrapReturnPathTarget
	}
	if rp != "" {
		records = append(records, bootstrapDNSRecord{Type: "CNAME", Host: rp, Value: cname})
	}
	return records
}

// bootstrapEnvFileContent replaces (or appends) the POSTMARK_SERVER_TOKEN
// line and keeps every other line.
func bootstrapEnvFileContent(existing, token string) string {
	line := postmarkServerTokenEnv + "=" + token
	out := make([]string, 0)
	replaced := false
	if strings.TrimSpace(existing) != "" {
		for _, l := range strings.Split(strings.TrimRight(existing, "\n"), "\n") {
			trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "export "))
			if strings.HasPrefix(trimmed, postmarkServerTokenEnv+"=") {
				if !replaced {
					out = append(out, line)
					replaced = true
				}
				continue
			}
			out = append(out, l)
		}
	}
	if !replaced {
		out = append(out, line)
	}
	return strings.Join(out, "\n") + "\n"
}

func newNovelServersBootstrapCmd(flags *rootFlags) *cobra.Command {
	var o bootstrapOptions

	cmd := &cobra.Command{
		Use:   "bootstrap [name]",
		Short: "Stand up a new project's Postmark server with streams, webhooks, domain DNS, and templates; prints the plan unless --apply is set, and reruns never duplicate",
		Long: strings.Trim(`
Use this command to stand up a complete Postmark server for a new project in one idempotent step. Do NOT use it to change settings on an existing server; use 'servers update' instead. Do NOT use it to check DNS verification on existing domains; use 'domains health' instead.

Without --apply it prints the plan: what already exists and what would be
created. With --apply it creates only what is missing, in order: the server
(account token), a Broadcasts stream (--broadcast-stream), a bounce, spam, and
delivery webhook (--webhook-url), the sending domain (--domain) with the DNS
records to add, and a template copy from another server (--copy-templates-from).
A rerun finds the existing server by exact name and never creates a second one.

--env-file writes the new server's token as a POSTMARK_SERVER_TOKEN line (file
mode 0600). The token is never printed. The global --sandbox flag selects
Postmark's test token; use --delivery-type sandbox for a sandbox server.

Argument: the new server's name, as the first positional (servers bootstrap <name>).`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli servers bootstrap "Lumen" --broadcast-stream --agent
  postmark-pp-cli servers bootstrap "Lumen" --domain mail.example.com --broadcast-stream --copy-templates-from "Main App" --json
  postmark-pp-cli servers bootstrap "Lumen" --domain mail.example.com --env-file .env --apply`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "live",
			// MCP tools can plan a bootstrap but never apply it or write files:
			// --apply and --env-file are withheld from the MCP schema.
			"mcp:write-flags": "env-file,apply",
			"mcp:read-only":   "true",
			// Any string is a valid name for a new server, so there is no
			// invalid positional to probe.
			"pp:no-error-path-probe": "true",
			"pp:happy-args":          "name=Lumen",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "servers bootstrap")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(errors.New("server name is required: servers bootstrap <name>"))
			}
			if o.apply && cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "create Postmark server resources")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			o.name = strings.TrimSpace(args[0])
			if err := bootstrapValidate(&o); err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			res, err := runServersBootstrap(ctx, flags, o)
			if res != nil {
				if perr := bootstrapOutput(cmd, flags, *res); perr != nil && err == nil {
					err = perr
				}
			}
			return err
		},
	}
	cmd.Flags().StringVar(&o.deliveryType, "delivery-type", "live", "Server type set at creation: live or sandbox")
	cmd.Flags().StringVar(&o.color, "color", "", "Server color: purple, blue, turquoise, green, red, yellow, grey, or orange")
	cmd.Flags().StringVar(&o.domain, "domain", "", "Sending domain to add (or reuse) and print DNS records for")
	cmd.Flags().StringVar(&o.returnPath, "return-path", "", "Custom Return-Path subdomain (default pm-bounces.<domain>)")
	cmd.Flags().BoolVar(&o.broadcast, "broadcast-stream", false, "Ensure a Broadcasts message stream exists")
	cmd.Flags().StringVar(&o.webhookURL, "webhook-url", "", "Ensure a webhook for bounce, spam complaint, and delivery events at this URL")
	cmd.Flags().BoolVar(&o.noVerifyWebhook, "no-verify-webhook", false, "Save the webhook without Postmark calling the URL first")
	cmd.Flags().StringVar(&o.copyFrom, "copy-templates-from", "", "Copy templates from this server (name or ID) to the new server")
	cmd.Flags().StringVar(&o.envFile, "env-file", "", "Write POSTMARK_SERVER_TOKEN=<new server token> into this file (mode 0600)")
	cmd.Flags().BoolVar(&o.apply, "apply", false, "Create the missing pieces (default prints the plan only)")
	return cmd
}

func bootstrapValidate(o *bootstrapOptions) error {
	switch strings.ToLower(strings.TrimSpace(o.deliveryType)) {
	case "live", "":
		o.deliveryType = postmarkDeliveryLive
	case "sandbox":
		o.deliveryType = postmarkDeliverySandbox
	default:
		return fmt.Errorf("--delivery-type must be live or sandbox (got %q)", o.deliveryType)
	}
	o.domain = strings.ToLower(strings.TrimSpace(o.domain))
	o.returnPath = strings.ToLower(strings.TrimSpace(o.returnPath))
	if o.returnPath != "" && o.domain == "" {
		return errors.New("--return-path needs --domain")
	}
	if o.domain != "" && o.returnPath == "" {
		o.returnPath = "pm-bounces." + o.domain
	}
	if o.returnPath != "" && !strings.HasSuffix(o.returnPath, "."+o.domain) {
		return fmt.Errorf("--return-path %q must be a subdomain of %s", o.returnPath, o.domain)
	}
	if o.webhookURL != "" {
		u, err := url.Parse(o.webhookURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return fmt.Errorf("--webhook-url must be an http(s) URL (got %q)", redactURLSecrets(o.webhookURL))
		}
	}
	if o.noVerifyWebhook && o.webhookURL == "" {
		return errors.New("--no-verify-webhook needs --webhook-url")
	}
	return nil
}

// runServersBootstrap plans, and with apply creates, each missing piece. The
// returned result reflects everything done so far even when err is set.
func runServersBootstrap(ctx context.Context, flags *rootFlags, o bootstrapOptions) (*bootstrapResult, error) {
	acct, err := newUncachedClient(flags)
	if err != nil {
		return nil, err
	}
	if !hasAccountToken(acct) {
		return nil, configErr(errors.New("servers bootstrap needs POSTMARK_ACCOUNT_TOKEN (servers, domains, and template push use the account API)"))
	}
	res := &bootstrapResult{Name: o.name, DeliveryType: o.deliveryType, Steps: make([]bootstrapStep, 0), DNSRecords: make([]bootstrapDNSRecord, 0)}

	existing, err := bootstrapFindServer(ctx, acct, o.name)
	if err != nil {
		return nil, err
	}
	var source *postmarkServerRef
	if o.copyFrom != "" {
		refs, err := listPostmarkServers(acct)
		if err != nil {
			return nil, classifyAPIErrorOnly(err)
		}
		ref, err := matchPostmarkServer(refs, o.copyFrom)
		if err != nil {
			return nil, err
		}
		if existing != nil && ref.ID == existing.ID {
			return nil, usageErr(fmt.Errorf("--copy-templates-from %q is the server being bootstrapped", o.copyFrom))
		}
		source = &ref
	}
	var domain *bootstrapDomain
	if o.domain != "" {
		domain, err = bootstrapFindDomain(ctx, acct, o.domain)
		if err != nil {
			return nil, err
		}
	}

	var sc *client.Client
	token := ""
	if existing != nil {
		res.ServerExists, res.ServerID, token = true, existing.ID, existing.token
		if strings.HasPrefix(token, "****") {
			token = ""
		}
		res.Steps = append(res.Steps, bootstrapStep{Step: bootstrapStepServer, Action: bootstrapActionExists, Detail: fmt.Sprintf("server %q (ID %d) already exists; it will not be recreated", existing.Name, existing.ID)})
		if sc, err = bootstrapServerClient(flags, token); err != nil {
			return res, err
		}
	} else {
		res.Steps = append(res.Steps, bootstrapStep{Step: bootstrapStepServer, Action: bootstrapActionCreate, Detail: fmt.Sprintf("create %s server %q", strings.ToLower(o.deliveryType), o.name)})
	}

	if !o.apply {
		if err := bootstrapPlanRest(ctx, sc, o, source, domain, res); err != nil {
			return res, err
		}
		res.Next = "rerun with --apply to create the missing pieces"
		return res, nil
	}

	res.Applied = true
	if existing == nil {
		body := map[string]any{"Name": o.name, "DeliveryType": o.deliveryType}
		if o.color != "" {
			body["Color"] = strings.ToLower(o.color)
		}
		data, _, err := acct.Post(withInternalTokens(ctx), "/servers", body)
		if err != nil {
			return res, fmt.Errorf("creating server: %w", classifyAPIErrorOnly(err))
		}
		var created struct {
			ID        int64    `json:"ID"`
			Name      string   `json:"Name"`
			APITokens []string `json:"ApiTokens"`
		}
		if err := json.Unmarshal(data, &created); err != nil {
			return res, fmt.Errorf("parsing created server: %w", err)
		}
		if len(created.APITokens) == 0 || strings.HasPrefix(created.APITokens[0], "****") {
			return res, fmt.Errorf("server %q (ID %d) was created but the response carried no usable token; rerun to continue with the existing server", created.Name, created.ID)
		}
		res.ServerID, token = created.ID, created.APITokens[0]
		res.Steps[0] = bootstrapStep{Step: bootstrapStepServer, Action: bootstrapActionCreated, Detail: fmt.Sprintf("created %s server %q (ID %d)", strings.ToLower(o.deliveryType), created.Name, created.ID)}
		if sc, err = bootstrapServerClient(flags, token); err != nil {
			return res, err
		}
	}

	if o.broadcast {
		step, err := bootstrapEnsureBroadcast(ctx, sc, true)
		res.Steps = append(res.Steps, step)
		if err != nil {
			return res, err
		}
	}
	if o.webhookURL != "" {
		step, err := bootstrapEnsureWebhook(ctx, sc, o, true)
		res.Steps = append(res.Steps, step)
		if err != nil {
			return res, err
		}
	}
	if o.domain != "" {
		if domain == nil {
			data, _, err := acct.Post(ctx, "/domains", map[string]any{"Name": o.domain, "ReturnPathDomain": o.returnPath})
			if err != nil {
				return res, fmt.Errorf("creating domain %s: %w", o.domain, classifyAPIErrorOnly(err))
			}
			var created bootstrapDomain
			if err := json.Unmarshal(data, &created); err != nil {
				return res, fmt.Errorf("parsing created domain: %w", err)
			}
			domain = &created
			res.Steps = append(res.Steps, bootstrapStep{Step: bootstrapStepDomain, Action: bootstrapActionCreated, Detail: fmt.Sprintf("added %s (ID %d); add the DNS records, then run: postmark-pp-cli domains verify-dkim %d && postmark-pp-cli domains verify-return-path %d", created.Name, created.ID, created.ID, created.ID)})
		} else {
			res.Steps = append(res.Steps, bootstrapDomainExistsStep(*domain))
		}
		res.DNSRecords = bootstrapDNSRecords(*domain, o.returnPath)
	}
	if source != nil {
		data, _, err := acct.Put(ctx, "/templates/push", map[string]any{"SourceServerID": source.ID, "DestinationServerID": res.ServerID, "PerformChanges": true})
		if err != nil {
			return res, fmt.Errorf("copying templates from %q: %w", source.Name, classifyAPIErrorOnly(err))
		}
		var pushed struct {
			TotalCount int `json:"TotalCount"`
			Templates  []struct {
				Action string `json:"Action"`
				Alias  string `json:"Alias"`
				Name   string `json:"Name"`
			} `json:"Templates"`
		}
		if err := json.Unmarshal(data, &pushed); err != nil {
			return res, fmt.Errorf("parsing template push: %w", err)
		}
		for _, t := range pushed.Templates {
			res.Templates = append(res.Templates, bootstrapTemplate{Action: t.Action, Alias: t.Alias, Name: t.Name})
		}
		res.Steps = append(res.Steps, bootstrapStep{Step: bootstrapStepTemplates, Action: bootstrapActionPushed, Detail: fmt.Sprintf("pushed %d templates from %q (ID %d)", len(pushed.Templates), source.Name, source.ID)})
	}
	if o.envFile != "" {
		if strings.TrimSpace(token) == "" {
			return res, fmt.Errorf("not writing %s: the server token is unavailable for this account token", o.envFile)
		}
		existingEnv, err := os.ReadFile(o.envFile)
		if err != nil && !os.IsNotExist(err) {
			return res, fmt.Errorf("reading %s: %w", o.envFile, err)
		}
		if err := cliutil.AtomicWritePrivateFile(o.envFile, []byte(bootstrapEnvFileContent(string(existingEnv), token)), 0o600, 0o700); err != nil {
			return res, fmt.Errorf("writing %s: %w", o.envFile, err)
		}
		res.EnvFile = o.envFile
		res.Steps = append(res.Steps, bootstrapStep{Step: bootstrapStepEnvFile, Action: bootstrapActionWritten, Detail: fmt.Sprintf("wrote %s to %s (mode 0600)", postmarkServerTokenEnv, o.envFile)})
	}
	return res, nil
}

// bootstrapPlanRest adds the non-server plan steps. sc is nil when the server
// does not exist yet, in which case every server-scoped piece is a create.
func bootstrapPlanRest(ctx context.Context, sc *client.Client, o bootstrapOptions, source *postmarkServerRef, domain *bootstrapDomain, res *bootstrapResult) error {
	if o.broadcast {
		if sc == nil {
			res.Steps = append(res.Steps, bootstrapStep{Step: bootstrapStepBroadcast, Action: bootstrapActionCreate, Detail: "ensure a Broadcasts stream (new servers usually include one; created only if missing)"})
		} else {
			step, err := bootstrapEnsureBroadcast(ctx, sc, false)
			res.Steps = append(res.Steps, step)
			if err != nil {
				return err
			}
		}
	}
	if o.webhookURL != "" {
		if sc == nil {
			res.Steps = append(res.Steps, bootstrapStep{Step: bootstrapStepWebhook, Action: bootstrapActionCreate, Detail: "create a bounce, spam complaint, and delivery webhook to " + redactURLSecrets(o.webhookURL)})
		} else {
			step, err := bootstrapEnsureWebhook(ctx, sc, o, false)
			res.Steps = append(res.Steps, step)
			if err != nil {
				return err
			}
		}
	}
	if o.domain != "" {
		if domain == nil {
			res.Steps = append(res.Steps, bootstrapStep{Step: bootstrapStepDomain, Action: bootstrapActionCreate, Detail: fmt.Sprintf("add %s with Return-Path %s; Postmark issues the DKIM record on creation", o.domain, o.returnPath)})
			res.DNSRecords = bootstrapDNSRecords(bootstrapDomain{}, o.returnPath)
		} else {
			res.Steps = append(res.Steps, bootstrapDomainExistsStep(*domain))
			res.DNSRecords = bootstrapDNSRecords(*domain, o.returnPath)
		}
	}
	if source != nil {
		dest := "the new server"
		if res.ServerID != 0 {
			dest = strconv.FormatInt(res.ServerID, 10)
		}
		res.Steps = append(res.Steps, bootstrapStep{Step: bootstrapStepTemplates, Action: bootstrapActionPush, Detail: fmt.Sprintf("push templates from %q (ID %d) to %s", source.Name, source.ID, dest)})
	}
	if o.envFile != "" {
		res.Steps = append(res.Steps, bootstrapStep{Step: bootstrapStepEnvFile, Action: bootstrapActionWrite, Detail: fmt.Sprintf("write %s to %s (mode 0600)", postmarkServerTokenEnv, o.envFile)})
	}
	return nil
}

func bootstrapDomainExistsStep(d bootstrapDomain) bootstrapStep {
	return bootstrapStep{Step: bootstrapStepDomain, Action: bootstrapActionExists, Detail: fmt.Sprintf("%s already added (ID %d; DKIM verified: %t, Return-Path %s verified: %t)", d.Name, d.ID, d.DKIMVerified, d.ReturnPathDomain, d.ReturnPathDomainVerified)}
}

// bootstrapServerClient returns an uncached client that acts on the
// bootstrapped server with its own token.
func bootstrapServerClient(flags *rootFlags, token string) (*client.Client, error) {
	c, err := newUncachedClient(flags)
	if err != nil {
		return nil, err
	}
	setPostmarkServerToken(c.Config, token)
	return c, nil
}

// bootstrapFindServer looks up a server by exact (case-insensitive) name and
// returns it with its token, or nil when none exists.
func bootstrapFindServer(ctx context.Context, c *client.Client, name string) (*postmarkServerRef, error) {
	params := postmarkPageParams(0)
	params["name"] = name
	raw, err := c.Get(withInternalTokens(ctx), "/servers", params)
	if err != nil {
		return nil, fmt.Errorf("looking up server %q: %w", name, classifyAPIErrorOnly(err))
	}
	var page postmarkServerListing
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, fmt.Errorf("parsing server list: %w", err)
	}
	for _, ref := range page.refs() {
		if strings.EqualFold(strings.TrimSpace(ref.Name), name) {
			return &ref, nil
		}
	}
	return nil, nil
}

// bootstrapFindDomain returns the account domain named name with its DNS
// details, or nil.
func bootstrapFindDomain(ctx context.Context, c *client.Client, name string) (*bootstrapDomain, error) {
	var match *postmarkDomainRef
	err := walkPostmarkPages(postmarkDomainPages(ctx, c), func(d postmarkDomainRef) bool {
		if strings.EqualFold(d.Name, name) {
			match = &d
			return true
		}
		return false
	})
	if err != nil || match == nil {
		return nil, err
	}
	detail, err := getPostmarkDomain(ctx, c, match.ID)
	if err != nil {
		return nil, fmt.Errorf("reading domain %s: %w", name, err)
	}
	return &detail, nil
}

// bootstrapEnsureBroadcast reports (and with create, adds) a Broadcasts stream.
func bootstrapEnsureBroadcast(ctx context.Context, sc *client.Client, create bool) (bootstrapStep, error) {
	streams, err := listPostmarkStreams(ctx, sc, true)
	if err != nil {
		return bootstrapStep{Step: bootstrapStepBroadcast, Action: bootstrapActionError, Detail: err.Error()}, err
	}
	for _, s := range streams {
		if strings.EqualFold(s.MessageStreamType, messageStreamTypeBroadcasts) && s.active() {
			return bootstrapStep{Step: bootstrapStepBroadcast, Action: bootstrapActionExists, Detail: fmt.Sprintf("Broadcasts stream %q already exists", s.ID)}, nil
		}
	}
	if !create {
		return bootstrapStep{Step: bootstrapStepBroadcast, Action: bootstrapActionCreate, Detail: fmt.Sprintf("create Broadcasts stream %q", bootstrapBroadcastID)}, nil
	}
	body := map[string]any{
		"ID":                bootstrapBroadcastID,
		"Name":              "Broadcasts",
		"MessageStreamType": messageStreamTypeBroadcasts,
		"SubscriptionManagementConfiguration": map[string]string{
			"UnsubscribeHandlingType": "Postmark",
		},
	}
	if _, _, err := sc.Post(ctx, "/message-streams", body); err != nil {
		err = fmt.Errorf("creating Broadcasts stream: %w", classifyAPIErrorOnly(err))
		return bootstrapStep{Step: bootstrapStepBroadcast, Action: bootstrapActionError, Detail: err.Error()}, err
	}
	return bootstrapStep{Step: bootstrapStepBroadcast, Action: bootstrapActionCreated, Detail: fmt.Sprintf("created Broadcasts stream %q", bootstrapBroadcastID)}, nil
}

// bootstrapEnsureWebhook reports (and with create, adds) a webhook for the URL.
func bootstrapEnsureWebhook(ctx context.Context, sc *client.Client, o bootstrapOptions, create bool) (bootstrapStep, error) {
	var doc struct {
		Webhooks []bootstrapWebhook `json:"Webhooks"`
	}
	if err := postmarkGetJSON(ctx, sc, "/webhooks", nil, &doc); err != nil {
		err = fmt.Errorf("listing webhooks: %w", err)
		return bootstrapStep{Step: bootstrapStepWebhook, Action: bootstrapActionError, Detail: err.Error()}, err
	}
	// A webhook counts only when it posts to the URL from the outbound stream;
	// Postmark cannot move a webhook between streams, so a same-URL webhook on
	// another stream does not satisfy the step and a new one is created.
	for _, w := range doc.Webhooks {
		if !sameWebhookURL(w.URL, o.webhookURL) || w.MessageStream != postmarkDefaultStream {
			continue
		}
		missing := w.missingTriggers()
		if len(missing) == 0 {
			return bootstrapStep{Step: bootstrapStepWebhook, Action: bootstrapActionExists, Detail: fmt.Sprintf("webhook %d already posts bounce, spam complaint, and delivery events to %s", w.ID, redactURLSecrets(w.URL))}, nil
		}
		detail := fmt.Sprintf("enable %s on webhook %d (%s)", strings.Join(missing, ", "), w.ID, redactURLSecrets(w.URL))
		if !create {
			return bootstrapStep{Step: bootstrapStepWebhook, Action: bootstrapActionUpdate, Detail: detail}, nil
		}
		update := map[string]any{"Triggers": w.triggerUpdate(missing)}
		var params map[string]string
		if o.noVerifyWebhook {
			update["Verify"] = false
			params = map[string]string{"verify": "false"}
		}
		if _, _, err := sc.PutWithParams(ctx, fmt.Sprintf("/webhooks/%d", w.ID), params, update); err != nil {
			err = fmt.Errorf("updating webhook %d: %w", w.ID, classifyAPIErrorOnly(err))
			return bootstrapStep{Step: bootstrapStepWebhook, Action: bootstrapActionError, Detail: err.Error()}, err
		}
		return bootstrapStep{Step: bootstrapStepWebhook, Action: bootstrapActionUpdated, Detail: "enabled " + strings.TrimPrefix(detail, "enable ")}, nil
	}
	if !create {
		return bootstrapStep{Step: bootstrapStepWebhook, Action: bootstrapActionCreate, Detail: "create a bounce, spam complaint, and delivery webhook to " + redactURLSecrets(o.webhookURL)}, nil
	}
	body := map[string]any{
		"Url":           o.webhookURL,
		"MessageStream": postmarkDefaultStream,
		"Triggers": map[string]any{
			"Bounce":        map[string]bool{"Enabled": true, "IncludeContent": false},
			"SpamComplaint": map[string]bool{"Enabled": true, "IncludeContent": false},
			"Delivery":      map[string]bool{"Enabled": true},
		},
		"Verify": !o.noVerifyWebhook,
	}
	var params map[string]string
	if o.noVerifyWebhook {
		params = map[string]string{"verify": "false"}
	}
	data, _, err := sc.PostWithParams(ctx, "/webhooks", params, body)
	if err != nil {
		err = fmt.Errorf("creating webhook: %w", classifyAPIErrorOnly(err))
		return bootstrapStep{Step: bootstrapStepWebhook, Action: bootstrapActionError, Detail: err.Error()}, err
	}
	var created bootstrapWebhook
	_ = json.Unmarshal(data, &created)
	return bootstrapStep{Step: bootstrapStepWebhook, Action: bootstrapActionCreated, Detail: fmt.Sprintf("created webhook %d to %s (bounce, spam complaint, delivery)", created.ID, redactURLSecrets(o.webhookURL))}, nil
}

func bootstrapOutput(cmd *cobra.Command, flags *rootFlags, res bootstrapResult) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), res, flags)
	}
	out := cmd.OutOrStdout()
	mode := "Plan"
	if res.Applied {
		mode = "Applied"
	}
	fmt.Fprintf(out, "%s for server %q:\n", mode, res.Name)
	for _, s := range res.Steps {
		fmt.Fprintf(out, "  %-16s %-8s %s\n", s.Step, s.Action, s.Detail)
	}
	if len(res.DNSRecords) > 0 {
		fmt.Fprintln(out, "\nDNS records:")
		for _, r := range res.DNSRecords {
			fmt.Fprintf(out, "  %-5s %s\n        %s\n", r.Type, r.Host, r.Value)
		}
	}
	if len(res.Templates) > 0 {
		counts := map[string]int{}
		for _, t := range res.Templates {
			counts[t.Action]++
		}
		keys := make([]string, 0, len(counts))
		for k := range counts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprint(out, "\nTemplates:")
		for _, k := range keys {
			fmt.Fprintf(out, " %s=%d", k, counts[k])
		}
		fmt.Fprintln(out)
	}
	if res.Next != "" {
		fmt.Fprintf(out, "\nnext: %s\n", res.Next)
	}
	return nil
}
