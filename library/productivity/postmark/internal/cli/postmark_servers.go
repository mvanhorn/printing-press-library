// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/config"
)

// Postmark splits auth by endpoint family: account-scoped paths accept only
// X-Postmark-Account-Token and server-scoped paths accept only
// X-Postmark-Server-Token. Sending both to GET /server returns 401 ErrorCode 10,
// so the transport strips whichever header the path does not accept.
const (
	postmarkServerTokenHeader  = "X-Postmark-Server-Token"
	postmarkAccountTokenHeader = "X-Postmark-Account-Token" // #nosec G101 -- HTTP header name, not a credential.
	postmarkSandboxToken       = "POSTMARK_API_TEST"        // #nosec G101 -- Postmark's public sandbox token; it accepts sends without delivering them.
	postmarkServerEnv          = "POSTMARK_SERVER"
)

var postmarkAccountPathPrefixes = []string{"/servers", "/domains", "/senders", "/templates/push", "/data-removals"}

func postmarkIsAccountPath(path string) bool {
	path = "/" + strings.TrimLeft(strings.ToLower(path), "/")
	for _, prefix := range postmarkAccountPathPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") || strings.HasPrefix(path, prefix+"?") {
			return true
		}
	}
	return false
}

type postmarkTokenRoutingTransport struct {
	base http.RoundTripper
}

func (t *postmarkTokenRoutingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	drop := postmarkAccountTokenHeader
	if postmarkIsAccountPath(req.URL.Path) {
		drop = postmarkServerTokenHeader
	}
	if postmarkIsSendPath(req) && !postmarkSendAllowed.Load() && req.Header.Get(postmarkServerTokenHeader) != postmarkSandboxToken {
		return nil, errPostmarkSendNotConfirmed
	}
	internal := req.Context().Value(postmarkInternalTokensKey{}) != nil
	needsOffset := postmarkNeedsFirstPageOffset(req)
	if req.Header.Get(drop) != "" || needsOffset {
		req = req.Clone(req.Context())
		req.Header.Del(drop)
		if needsOffset {
			q := req.URL.Query()
			q.Set("offset", "0")
			req.URL.RawQuery = q.Encode()
		}
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil || resp == nil || internal || !postmarkReturnsServerTokens(req.URL.Path) {
		return resp, err
	}
	return redactPostmarkTokens(resp)
}

// postmarkInternalTokensKey marks a request context whose response needs real
// server tokens. It lives in the context, not a header, so no config file or
// user input can switch masking off.
type postmarkInternalTokensKey struct{}

func withInternalTokens(ctx context.Context) context.Context {
	return context.WithValue(ctx, postmarkInternalTokensKey{}, true)
}

// withSendAllowed runs one delivering request with the send block lifted and
// restores it on every exit path.
func withSendAllowed[T any](fn func() (T, error)) (T, error) {
	postmarkSendAllowed.Store(true)
	defer postmarkSendAllowed.Store(false)
	return fn()
}

// postmarkSendDefaultsServer returns the server whose saved send defaults
// apply: an explicit --server or POSTMARK_SERVER always, a saved default only
// when no server token is configured (its sender must not ride along on
// another server's token).
func postmarkSendDefaultsServer(flags *rootFlags) string {
	name, explicit := postmarkSelectedServer()
	if name == "" || explicit {
		return name
	}
	if cfg, err := config.Load(flags.configPath); err == nil && hasServerToken(cfg) {
		return ""
	}
	return name
}

// postmarkStaleDefaults remembers saved default servers that failed to
// resolve so the warning prints once per process.
var postmarkStaleDefaults sync.Map

// postmarkSendAllowed is set only by commands whose caller explicitly asked to
// deliver (--send). Every other path, including MCP tools that call endpoints
// directly, cannot deliver mail; the sandbox token is always allowed because it
// never delivers.
var postmarkSendAllowed atomic.Bool

var errPostmarkSendNotConfirmed = errors.New("sending is blocked: this CLI delivers email only from a command run with --send (email send --send, email send-once --send) or with --sandbox")

func postmarkIsSendPath(req *http.Request) bool {
	if req.Method != http.MethodPost {
		return false
	}
	switch strings.TrimRight(strings.ToLower(req.URL.Path), "/") {
	case "/email", "/email/batch", "/email/withtemplate", "/email/batchwithtemplates", "/email/bulk":
		return true
	}
	return false
}

// postmarkNeedsFirstPageOffset reports paged list requests that carry a page
// size but no offset. Postmark rejects those with "Parameter 'offset' is
// required", and offset-style sync omits the cursor on its first page.
func postmarkNeedsFirstPageOffset(req *http.Request) bool {
	if req.Method != http.MethodGet {
		return false
	}
	hasCount, hasOffset := false, false
	for key := range req.URL.Query() {
		switch strings.ToLower(key) {
		case "count":
			hasCount = true
		case "offset":
			hasOffset = true
		}
	}
	return hasCount && !hasOffset && !strings.HasPrefix(strings.ToLower(req.URL.Path), "/email/bulk")
}

// postmarkReturnsServerTokens reports paths whose responses carry ApiTokens.
func postmarkReturnsServerTokens(path string) bool {
	path = "/" + strings.TrimLeft(strings.ToLower(path), "/")
	return path == "/server" || path == "/servers" || strings.HasPrefix(path, "/servers/")
}

// redactPostmarkTokens masks every ApiTokens entry so server tokens never
// reach stdout, the response cache, or the local store.
func redactPostmarkTokens(resp *http.Response) (*http.Response, error) {
	if resp.Body == nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, nil
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	var doc any
	if json.Unmarshal(body, &doc) == nil {
		maskPostmarkTokens(doc)
		if redacted, err := json.Marshal(doc); err == nil {
			body = redacted
		}
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return resp, nil
}

func maskPostmarkTokens(node any) {
	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			if strings.EqualFold(k, "ApiTokens") {
				if list, ok := child.([]any); ok {
					for i, tok := range list {
						if s, ok := tok.(string); ok {
							list[i] = maskPostmarkToken(s)
						}
					}
				}
				continue
			}
			maskPostmarkTokens(child)
		}
	case []any:
		for _, child := range v {
			maskPostmarkTokens(child)
		}
	}
}

func maskPostmarkToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}

// postmarkSelection holds the global --server / --sandbox values. The client
// hook reads them because hooks receive only the client.
var postmarkSelection struct {
	server  string
	sandbox bool
}

var (
	postmarkTokenCacheMu sync.Mutex
	postmarkTokenCache   = map[string]postmarkServerRef{}
)

type postmarkServerRef struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	token        string
	deliveryType string
}

func (r postmarkServerRef) target(c *client.Client) postmarkTarget {
	return postmarkTarget{Name: r.Name, ID: r.ID, deliveryType: r.deliveryType, client: c}
}

// postmarkServerDefaults is persisted by `servers use`.
type postmarkServerDefaults struct {
	DefaultServer string                         `json:"default_server,omitempty"`
	Servers       map[string]postmarkSendDefault `json:"servers,omitempty"`
}

type postmarkSendDefault struct {
	From          string `json:"from,omitempty"`
	MessageStream string `json:"message_stream,omitempty"`
}

// forServer returns the saved send defaults for name. Server names are
// matched case-insensitively, as --server matches them.
func (d postmarkServerDefaults) forServer(name string) (postmarkSendDefault, bool) {
	for k, v := range d.Servers {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return postmarkSendDefault{}, false
}

func postmarkDefaultsPath() (string, error) {
	dir, err := cliutil.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "servers.json"), nil
}

func loadPostmarkDefaults() (postmarkServerDefaults, error) {
	var d postmarkServerDefaults
	path, err := postmarkDefaultsPath()
	if err != nil {
		return d, err
	}
	data, err := os.ReadFile(path) // #nosec G304 -- app-owned defaults file under cliutil.ConfigDir.
	if errors.Is(err, os.ErrNotExist) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return d, fmt.Errorf("parsing %s: %w", path, err)
	}
	return d, nil
}

func savePostmarkDefaults(d postmarkServerDefaults) (string, error) {
	path, err := postmarkDefaultsPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(data, '\n'), 0o600)
}

// postmarkSelectedServer returns the requested server name and whether it was
// explicit (flag or env). A saved default only applies when no server token is
// configured, so an exported POSTMARK_SERVER_TOKEN keeps working unchanged.
func postmarkSelectedServer() (string, bool) {
	if s := strings.TrimSpace(postmarkSelection.server); s != "" {
		return s, true
	}
	if s := strings.TrimSpace(os.Getenv(postmarkServerEnv)); s != "" {
		return s, true
	}
	d, err := loadPostmarkDefaults()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(d.DefaultServer), false
}

type postmarkServerListing struct {
	TotalCount int `json:"TotalCount"`
	Servers    []struct {
		ID           int64    `json:"ID"`
		Name         string   `json:"Name"`
		DeliveryType string   `json:"DeliveryType"`
		APITokens    []string `json:"ApiTokens"`
	} `json:"Servers"`
}

// refs returns the listed servers, each with its first API token.
func (l postmarkServerListing) refs() []postmarkServerRef {
	refs := make([]postmarkServerRef, 0, len(l.Servers))
	for _, s := range l.Servers {
		ref := postmarkServerRef{ID: s.ID, Name: s.Name, deliveryType: s.DeliveryType}
		if len(s.APITokens) > 0 {
			ref.token = s.APITokens[0]
		}
		refs = append(refs, ref)
	}
	return refs
}

// listPostmarkServers fetches every server the account token can see.
func listPostmarkServers(c *client.Client) ([]postmarkServerRef, error) {
	if !hasAccountToken(c) {
		return nil, configErr(errors.New("selecting a server by name needs POSTMARK_ACCOUNT_TOKEN (the account API lists servers and their tokens)"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Unmasked token responses must never land in the on-disk response cache.
	prevNoCache := c.NoCache
	c.NoCache = true
	defer func() { c.NoCache = prevNoCache }()
	return listPostmarkPages(func(offset int) ([]postmarkServerRef, int, error) {
		raw, err := c.Get(withInternalTokens(ctx), "/servers", postmarkPageParams(offset))
		if err != nil {
			return nil, 0, fmt.Errorf("listing servers: %w", err)
		}
		var page postmarkServerListing
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, 0, fmt.Errorf("parsing server list: %w", err)
		}
		return page.refs(), page.TotalCount, nil
	})
}

// matchPostmarkServer resolves a name or numeric ID: exact ID, then exact
// case-insensitive name, then a unique substring match.
func matchPostmarkServer(refs []postmarkServerRef, want string) (postmarkServerRef, error) {
	if id, err := strconv.ParseInt(want, 10, 64); err == nil {
		for _, r := range refs {
			if r.ID == id {
				return r, nil
			}
		}
	}
	for _, r := range refs {
		if strings.EqualFold(r.Name, want) {
			return r, nil
		}
	}
	var partial []postmarkServerRef
	for _, r := range refs {
		if strings.Contains(strings.ToLower(r.Name), strings.ToLower(want)) {
			partial = append(partial, r)
		}
	}
	if len(partial) == 1 {
		return partial[0], nil
	}
	names := make([]string, 0, len(refs))
	for _, r := range refs {
		names = append(names, r.Name)
	}
	sort.Strings(names)
	if len(partial) > 1 {
		return postmarkServerRef{}, usageErr(fmt.Errorf("server %q matches more than one server; use the full name or ID. Servers: %s", want, strings.Join(names, ", ")))
	}
	return postmarkServerRef{}, usageErr(fmt.Errorf("no server named %q; servers on this account: %s", want, strings.Join(names, ", ")))
}

// postmarkTokenCacheKey ties a cached server token to the API base URL and the
// account token that resolved it, so a long-running process (the MCP server)
// never hands out one account's server token after the account credentials
// change.
func postmarkTokenCacheKey(c *client.Client, want string) string {
	account := ""
	if c != nil {
		account = postmarkEffectiveAccountToken(c.Config)
	}
	sum := sha256.Sum256([]byte(account))
	base := ""
	if c != nil {
		base = c.BaseURL
	}
	return base + "|" + hex.EncodeToString(sum[:8]) + "|" + strings.ToLower(strings.TrimSpace(want))
}

func resolvePostmarkServer(c *client.Client, want string) (postmarkServerRef, error) {
	key := postmarkTokenCacheKey(c, want)
	postmarkTokenCacheMu.Lock()
	if ref, ok := postmarkTokenCache[key]; ok {
		postmarkTokenCacheMu.Unlock()
		return ref, nil
	}
	postmarkTokenCacheMu.Unlock()
	refs, err := listPostmarkServers(c)
	if err != nil {
		return postmarkServerRef{}, err
	}
	ref, err := matchPostmarkServer(refs, want)
	if err != nil {
		return postmarkServerRef{}, err
	}
	if ref.token == "" {
		return postmarkServerRef{}, fmt.Errorf("server %q returned no API token for this account token", ref.Name)
	}
	postmarkTokenCacheMu.Lock()
	postmarkTokenCache[key] = ref
	postmarkTokenCacheMu.Unlock()
	return ref, nil
}

func init() {
	registerClientHook(func(c *client.Client) error {
		if c.HTTPClient != nil {
			if _, wrapped := c.HTTPClient.Transport.(*postmarkTokenRoutingTransport); !wrapped {
				c.HTTPClient.Transport = &postmarkTokenRoutingTransport{base: c.HTTPClient.Transport}
			}
		}
		if c.Config == nil {
			return nil
		}
		if err := postmarkCredentialHeaderConflict(c.Config); err != nil {
			return err
		}
		if postmarkSelection.sandbox {
			setPostmarkServerToken(c.Config, postmarkSandboxToken)
			return nil
		}
		name, explicit := postmarkSelectedServer()
		if name == "" || c.DryRun {
			return nil
		}
		if !explicit && hasServerToken(c.Config) {
			return nil
		}
		if !explicit {
			if _, stale := postmarkStaleDefaults.Load(strings.ToLower(name)); stale {
				return nil
			}
		}
		ref, err := resolvePostmarkServer(c, name)
		if err != nil {
			if !explicit {
				postmarkStaleDefaults.Store(strings.ToLower(name), true)
				// A stale saved default must not break unrelated commands.
				fmt.Fprintf(os.Stderr, "warning: saved default server %q could not be resolved (%v); run 'postmark-pp-cli servers use <name>' or 'servers use --clear'\n", name, err)
				return nil
			}
			return err
		}
		setPostmarkServerToken(c.Config, ref.token)
		return nil
	})

	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.PersistentFlags().StringVar(&postmarkSelection.server, "server", "", "Postmark server name or ID to act on; resolves its token through POSTMARK_ACCOUNT_TOKEN (env: POSTMARK_SERVER)")
		root.PersistentFlags().BoolVar(&postmarkSelection.sandbox, "sandbox", false, "Use Postmark's POSTMARK_API_TEST token: sends are validated but never delivered")
		if prev := root.PersistentPreRunE; prev != nil {
			root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
				if err := prev(cmd, args); err != nil {
					return err
				}
				applyPostmarkSendDefaults(cmd, flags)
				if err := keepPostmarkArchiveOnFullSync(cmd); err != nil {
					return err
				}
				if cmd.Name() == "doctor" && !flags.dryRun {
					exportSelectedServerTokenForDoctor(flags)
				}
				return nil
			}
		}
		// Postmark answers an unknown stream with 200 and an empty list, so an
		// invalid stream ID is indistinguishable from a stream with no suppressions.
		if listCmd, _, err := root.Find([]string{"suppressions", "list"}); err == nil && listCmd.Name() == "list" {
			if listCmd.Annotations == nil {
				listCmd.Annotations = map[string]string{}
			}
			listCmd.Annotations["pp:no-error-path-probe"] = "true"
		}
		if serversCmd, _, err := root.Find([]string{"servers"}); err == nil && serversCmd != root {
			addNovelCommandIfAbsent(serversCmd, newServersUseCmd(flags))
			addNovelCommandIfAbsent(serversCmd, newServersTokensCmd(flags))
		}
	})
}

// exportSelectedServerTokenForDoctor lets doctor's config-based auth check see
// a server chosen with --server, POSTMARK_SERVER, or `servers use`. Other
// commands resolve the token per client in the client hook.
func exportSelectedServerTokenForDoctor(flags *rootFlags) {
	if postmarkSelection.sandbox {
		return
	}
	if name, _ := postmarkSelectedServer(); name == "" {
		return
	}
	c, err := flags.newClient()
	if err != nil || c.Config == nil {
		return
	}
	if token := strings.TrimSpace(c.Config.PostmarkServerToken); token != "" {
		_ = os.Setenv(postmarkServerTokenEnv, token)
	}
}

// keepPostmarkArchiveOnFullSync makes `sync --full` keep rows the API no longer
// returns and refuses --no-prune=false. The local store is an archive shared by
// every server synced into it: a prune compares the whole table with one
// server's current listing, so it would delete other servers' rows and the
// messages Postmark has already expired.
func keepPostmarkArchiveOnFullSync(cmd *cobra.Command) error {
	if cmd.Name() != "sync" {
		return nil
	}
	full := cmd.Flags().Lookup("full")
	noPrune := cmd.Flags().Lookup("no-prune")
	if full == nil || noPrune == nil || full.Value.String() != "true" {
		return nil
	}
	if noPrune.Changed {
		if noPrune.Value.String() == "false" {
			return usageErr(errors.New("--no-prune=false is not supported: the local archive holds every synced server's rows and messages Postmark has expired, and pruning against one server's listing would delete them. To start a fresh archive, sync into a new file with --db <path>; the current archive and send ledger stay as they are"))
		}
		return nil
	}
	_ = cmd.Flags().Set("no-prune", "true")
	return nil
}

// applyPostmarkSendDefaults fills --from and --message-stream from the
// selected server's saved defaults when the command has those flags and the
// caller left them unset.
func applyPostmarkSendDefaults(cmd *cobra.Command, flags *rootFlags) {
	if cmd.Parent() == nil || cmd.Parent().Name() != "email" {
		return
	}
	name := postmarkSendDefaultsServer(flags)
	if name == "" {
		return
	}
	d, err := loadPostmarkDefaults()
	if err != nil || len(d.Servers) == 0 {
		return
	}
	def, found := d.forServer(name)
	if !found {
		return
	}
	for flagName, value := range map[string]string{"from": def.From, "message-stream": def.MessageStream} {
		if value == "" {
			continue
		}
		if f := cmd.Flags().Lookup(flagName); f != nil && !f.Changed {
			_ = cmd.Flags().Set(flagName, value)
		}
	}
}

type serversUseResult struct {
	DefaultServer string `json:"default_server"`
	ServerID      int64  `json:"server_id,omitempty"`
	From          string `json:"from,omitempty"`
	MessageStream string `json:"message_stream,omitempty"`
	Path          string `json:"path,omitempty"`
	Verified      bool   `json:"verified"`
}

func newServersUseCmd(flags *rootFlags) *cobra.Command {
	var from, stream string
	var show, clear bool
	cmd := &cobra.Command{
		Use:   "use [name]",
		Short: "Set the default server for later commands, with optional default sender and stream",
		Long: strings.Trim(`
Set the server that commands act on when --server is not given. The server
token is looked up through POSTMARK_ACCOUNT_TOKEN each run and is never written
to disk. An exported POSTMARK_SERVER_TOKEN still takes precedence over the saved
default; --server and POSTMARK_SERVER override both.

--from and --stream save per-server defaults that fill 'email send' style
commands when those flags are left unset.`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli servers use "Main App"
  postmark-pp-cli servers use "Main App" --from app@example.com --stream outbound
  postmark-pp-cli servers use --show --json`, "\n"),
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "set default Postmark server")
			}
			d, err := loadPostmarkDefaults()
			if err != nil {
				return err
			}
			path, _ := postmarkDefaultsPath()
			if show {
				res := serversUseResult{DefaultServer: d.DefaultServer, Path: path}
				if def, ok := d.Servers[d.DefaultServer]; ok {
					res.From, res.MessageStream = def.From, def.MessageStream
				}
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if clear {
				d.DefaultServer = ""
				if _, err := savePostmarkDefaults(d); err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), serversUseResult{Path: path}, flags)
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(errors.New("server name or ID is required (or pass --show / --clear)"))
			}
			name := args[0]
			res := serversUseResult{DefaultServer: name}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if hasAccountToken(c) {
				ref, err := resolvePostmarkServer(c, name)
				if err != nil {
					return err
				}
				res.DefaultServer, res.ServerID, res.Verified = ref.Name, ref.ID, true
			}
			d.DefaultServer = res.DefaultServer
			if from != "" || stream != "" {
				if d.Servers == nil {
					d.Servers = map[string]postmarkSendDefault{}
				}
				def := d.Servers[res.DefaultServer]
				if from != "" {
					def.From = from
				}
				if stream != "" {
					def.MessageStream = stream
				}
				d.Servers[res.DefaultServer] = def
			}
			if def, ok := d.Servers[res.DefaultServer]; ok {
				res.From, res.MessageStream = def.From, def.MessageStream
			}
			if res.Path, err = savePostmarkDefaults(d); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Default sender address for sends on this server")
	cmd.Flags().StringVar(&stream, "stream", "", "Default message stream ID for sends on this server")
	cmd.Flags().BoolVar(&show, "show", false, "Print the saved default server and send defaults")
	cmd.Flags().BoolVar(&clear, "clear", false, "Remove the saved default server")
	return cmd
}

type serverTokenRow struct {
	Server string `json:"server"`
	ID     int64  `json:"id"`
	Token  string `json:"token"`
}

// newServersTokensCmd is the one place full server tokens are printed. Every
// other response is masked, and this lookup bypasses the response cache and
// never writes the local store, so tokens only reach the terminal that asked.
func newServersTokensCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokens [name]",
		Short: "Print the full API token for one server, or for every server",
		Long: strings.Trim(`
Print full server API tokens using POSTMARK_ACCOUNT_TOKEN. Other commands mask
ApiTokens in their output; this command is the explicit way to reveal them. It
never caches or stores the tokens and is not exposed as an MCP tool.`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli servers tokens "Main App"
  postmark-pp-cli servers tokens --json`, "\n"),
		Annotations: map[string]string{"mcp:hidden": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "print server tokens")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			refs, err := listPostmarkServers(c)
			if err != nil {
				return err
			}
			if len(args) > 0 {
				ref, err := matchPostmarkServer(refs, args[0])
				if err != nil {
					return err
				}
				refs = []postmarkServerRef{ref}
			}
			rows := make([]serverTokenRow, 0, len(refs))
			for _, r := range refs {
				rows = append(rows, serverTokenRow{Server: r.Name, ID: r.ID, Token: r.token})
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d\t%s\n", r.Server, r.ID, r.Token)
			}
			return nil
		},
	}
	return cmd
}
