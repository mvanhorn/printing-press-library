package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

const version = "0.1.0"

type config struct {
	MailBase     string            `json:"mail_base,omitempty"`
	AccountsBase string            `json:"accounts_base,omitempty"`
	AccessToken  string            `json:"-"`
	RefreshToken string            `json:"refresh_token,omitempty"`
	ClientID     string            `json:"client_id,omitempty"`
	ClientSecret string            `json:"client_secret,omitempty"`
	AccountID    string            `json:"account_id,omitempty"`
	Folders      map[string]string `json:"folders,omitempty"`
	Output       string            `json:"output,omitempty"`
	ConfigPath   string            `json:"-"`
	HTTPClient   *http.Client      `json:"-"`
}

type client struct {
	cfg config
}

type apiEnvelope struct {
	Status any             `json:"status,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "zohomail-pp-cli:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		usage(stdout)
		return nil
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		usage(stdout)
		return nil
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Fprintln(stdout, version)
		return nil
	}

	cfg := configFromEnv()
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	addGlobalFlags(fs, &cfg)

	switch args[0] {
	case "doctor":
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		printDoctor(stdout, cfg)
		return nil
	case "configure":
		accountID := fs.String("account-id", "", "Zoho Mail account ID; default auto-selects first/default account")
		inboxID := fs.String("inbox-folder-id", "", "Inbox folder ID")
		sentID := fs.String("sent-folder-id", "", "Sent folder ID")
		spamID := fs.String("spam-folder-id", "", "Spam folder ID")
		trashID := fs.String("trash-folder-id", "", "Trash folder ID")
		archiveID := fs.String("archive-folder-id", "", "Archive folder ID")
		saveToken := fs.Bool("save-token", false, "persist refresh token and client credentials from env/config")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		next := cfg
		offlineFolders := map[string]string{
			"inbox":   *inboxID,
			"sent":    *sentID,
			"spam":    *spamID,
			"trash":   *trashID,
			"archive": *archiveID,
		}
		if *accountID != "" && hasFolderFlag(offlineFolders) {
			next.AccountID = *accountID
			if next.Folders == nil {
				next.Folders = map[string]string{}
			}
			for name, id := range offlineFolders {
				if id != "" {
					next.Folders[name] = id
				}
			}
		} else if *saveToken && next.AccountID != "" && hasFolderFlag(next.Folders) {
			// Save auth into an already-configured account without another API round trip.
		} else {
			c := client{cfg: cfg.withHTTPClient()}
			discovered, err := c.discoverConfig(*accountID)
			if err != nil {
				return err
			}
			next = discovered
		}
		next.ConfigPath = cfg.ConfigPath
		next.AccessToken = ""
		if !*saveToken {
			next.RefreshToken = ""
			next.ClientSecret = ""
		}
		if err := saveConfig(next); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "saved\t%s\n", next.ConfigPath)
		fmt.Fprintf(stdout, "account_id\t%s\n", next.AccountID)
		printFolderDefaults(stdout, next.Folders)
		if !*saveToken {
			fmt.Fprintln(stdout, "auth\tkept in environment; rerun with --save-token to persist refresh auth")
		}
		return nil
	case "login":
		var scopes string
		var redirect string
		var noOpen bool
		var timeoutText string
		var clientID string
		var clientSecret string
		fs.StringVar(&scopes, "scopes", "ZohoMail.accounts.READ,ZohoMail.folders.READ,ZohoMail.messages.READ,ZohoMail.messages.CREATE", "comma-separated OAuth scopes")
		fs.StringVar(&redirect, "redirect-uri", env("ZOHO_REDIRECT_URI", "http://localhost:53682/callback"), "OAuth redirect URI configured in Zoho API Console")
		fs.BoolVar(&noOpen, "no-open", false, "print URL instead of opening browser")
		fs.StringVar(&timeoutText, "timeout", "5m", "how long to wait for browser callback")
		fs.StringVar(&clientID, "client-id", "", "Zoho OAuth client ID; saved after successful login")
		fs.StringVar(&clientSecret, "client-secret", "", "Zoho OAuth client secret; saved after successful login")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if clientID != "" {
			cfg.ClientID = clientID
		}
		if clientSecret != "" {
			cfg.ClientSecret = clientSecret
		}
		timeout, err := time.ParseDuration(timeoutText)
		if err != nil {
			return err
		}
		return login(stdout, stderr, cfg.withHTTPClient(), redirect, scopes, timeout, !noOpen)
	case "client-setup":
		var noOpen bool
		fs.BoolVar(&noOpen, "no-open", false, "print URL instead of opening browser")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		u := strings.TrimRight(cfg.AccountsBase, "/") + "/developerconsole"
		fmt.Fprintf(stdout, "open\t%s\n", u)
		fmt.Fprintln(stdout, "redirect_uri\thttp://localhost:53682/callback")
		fmt.Fprintln(stdout, "scopes\tZohoMail.accounts.READ ZohoMail.folders.READ ZohoMail.messages.READ ZohoMail.messages.CREATE")
		if !noOpen {
			if err := openURL(u); err != nil {
				fmt.Fprintf(stderr, "open failed\t%s\n", err)
			}
		}
		return nil
	case "auth-save":
		var clientID string
		var clientSecret string
		var refreshToken string
		var noDiscover bool
		fs.StringVar(&clientID, "client-id", "", "Zoho OAuth client ID")
		fs.StringVar(&clientSecret, "client-secret", "", "Zoho OAuth client secret")
		fs.StringVar(&refreshToken, "refresh-token", "", "Zoho OAuth refresh token")
		fs.BoolVar(&noDiscover, "no-discover", false, "save auth without calling Zoho Mail account/folder APIs")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if clientID != "" {
			cfg.ClientID = clientID
		}
		if clientSecret != "" {
			cfg.ClientSecret = clientSecret
		}
		if refreshToken != "" {
			cfg.RefreshToken = refreshToken
		}
		if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RefreshToken == "" {
			return errors.New("missing --client-id, --client-secret, or --refresh-token")
		}
		cfg.AccessToken = ""
		if noDiscover {
			if err := saveConfig(cfg); err != nil {
				return err
			}
		} else if err := discoverAndSaveConfig(cfg); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "saved\t%s\n", cfg.ConfigPath)
		fmt.Fprintf(stdout, "client_id\t%s\n", presentID(cfg.ClientID))
		fmt.Fprintf(stdout, "client_secret\t%s\n", presentSecret(cfg.ClientSecret))
		fmt.Fprintf(stdout, "refresh_token\t%s\n", presentSecret(cfg.RefreshToken))
		fmt.Fprintf(stdout, "account_id\t%s\n", presentID(cfg.AccountID))
		printFolderDefaults(stdout, cfg.Folders)
		return nil
	case "auth-rbw":
		var item string
		var rbwBin string
		var clientIDField string
		var clientSecretField string
		var refreshTokenField string
		var noDiscover bool
		fs.StringVar(&item, "item", "", "rbw item name, URI, or UUID containing Zoho OAuth fields")
		fs.StringVar(&rbwBin, "rbw-bin", "rbw", "rbw executable path")
		fs.StringVar(&clientIDField, "client-id-field", "ZOHO_CLIENT_ID", "rbw custom field for Zoho OAuth client ID")
		fs.StringVar(&clientSecretField, "client-secret-field", "ZOHO_CLIENT_SECRET", "rbw custom field for Zoho OAuth client secret")
		fs.StringVar(&refreshTokenField, "refresh-token-field", "ZOHO_REFRESH_TOKEN", "rbw custom field for Zoho OAuth refresh token")
		fs.BoolVar(&noDiscover, "no-discover", false, "save auth without calling Zoho Mail account/folder APIs")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if item == "" {
			return errors.New("missing --item")
		}
		saved, err := authFromRBW(cfg, rbwBin, item, clientIDField, clientSecretField, refreshTokenField, noDiscover)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "saved\t%s\n", saved.ConfigPath)
		fmt.Fprintf(stdout, "client_id\t%s\n", presentID(saved.ClientID))
		fmt.Fprintf(stdout, "client_secret\t%s\n", presentSecret(saved.ClientSecret))
		fmt.Fprintf(stdout, "refresh_token\t%s\n", presentSecret(saved.RefreshToken))
		fmt.Fprintf(stdout, "account_id\t%s\n", presentID(saved.AccountID))
		printFolderDefaults(stdout, saved.Folders)
		return nil
	case "auth-url":
		var scopes string
		var redirect string
		var clientID string
		fs.StringVar(&scopes, "scopes", "ZohoMail.accounts.READ,ZohoMail.folders.READ,ZohoMail.messages.READ,ZohoMail.messages.CREATE", "comma-separated OAuth scopes")
		fs.StringVar(&redirect, "redirect-uri", env("ZOHO_REDIRECT_URI", "http://localhost"), "OAuth redirect URI")
		fs.StringVar(&clientID, "client-id", "", "Zoho OAuth client ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if clientID != "" {
			cfg.ClientID = clientID
		}
		u, err := authURL(cfg.AccountsBase, cfg.ClientID, redirect, scopes)
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, u)
		return nil
	case "token":
		var code string
		var redirect string
		var selfClient bool
		var saveToken bool
		var clientID string
		var clientSecret string
		fs.StringVar(&code, "code", "", "authorization code from Zoho")
		fs.StringVar(&redirect, "redirect-uri", env("ZOHO_REDIRECT_URI", "http://localhost"), "OAuth redirect URI")
		fs.BoolVar(&selfClient, "self-client", false, "exchange a Zoho API Console Self Client code without redirect_uri")
		fs.BoolVar(&saveToken, "save", false, "persist refresh token and client credentials to local config")
		fs.StringVar(&clientID, "client-id", "", "Zoho OAuth client ID")
		fs.StringVar(&clientSecret, "client-secret", "", "Zoho OAuth client secret")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if clientID != "" {
			cfg.ClientID = clientID
		}
		if clientSecret != "" {
			cfg.ClientSecret = clientSecret
		}
		if code == "" {
			return errors.New("missing --code")
		}
		c := client{cfg: cfg.withHTTPClient()}
		values := url.Values{
			"grant_type":    {"authorization_code"},
			"client_id":     {cfg.ClientID},
			"client_secret": {cfg.ClientSecret},
			"code":          {code},
		}
		if !selfClient {
			values.Set("redirect_uri", redirect)
		}
		body, err := c.tokenRequest(values)
		if err != nil {
			return err
		}
		if saveToken {
			if err := saveTokenConfig(cfg, body); err != nil {
				return err
			}
			fmt.Fprintf(stderr, "saved auth\t%s\n", cfg.ConfigPath)
		}
		return writeJSON(stdout, body)
	case "auth-device":
		var scopes string
		var save bool
		var noOpen bool
		var clientID string
		var clientSecret string
		fs.StringVar(&scopes, "scopes", "ZohoMail.accounts.READ,ZohoMail.folders.READ,ZohoMail.messages.READ,ZohoMail.messages.CREATE", "comma-separated OAuth scopes")
		fs.BoolVar(&save, "save", true, "save refresh token to config and run account/folder discovery")
		fs.BoolVar(&noOpen, "no-open", false, "print URL only, don't open browser")
		fs.StringVar(&clientID, "client-id", "", "Zoho OAuth client ID")
		fs.StringVar(&clientSecret, "client-secret", "", "Zoho OAuth client secret")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if clientID != "" {
			cfg.ClientID = clientID
		}
		if clientSecret != "" {
			cfg.ClientSecret = clientSecret
		}
		return deviceAuth(stdout, stderr, cfg.withHTTPClient(), scopes, !noOpen, save)
	case "auth-client-credentials":
		var scopes string
		var clientID string
		var clientSecret string
		fs.StringVar(&scopes, "scopes", "ZohoMail.accounts.READ,ZohoMail.folders.READ,ZohoMail.messages.READ,ZohoMail.messages.CREATE", "comma-separated OAuth scopes")
		fs.StringVar(&clientID, "client-id", "", "Zoho OAuth client ID")
		fs.StringVar(&clientSecret, "client-secret", "", "Zoho OAuth client secret")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if clientID != "" {
			cfg.ClientID = clientID
		}
		if clientSecret != "" {
			cfg.ClientSecret = clientSecret
		}
		c := client{cfg: cfg.withHTTPClient()}
		body, err := c.tokenRequest(url.Values{
			"grant_type":    {"client_credentials"},
			"client_id":     {cfg.ClientID},
			"client_secret": {cfg.ClientSecret},
			"scope":         {strings.ReplaceAll(scopes, ",", " ")},
		})
		if err != nil {
			return err
		}
		return writeJSON(stdout, body)
	case "accounts":
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		c := client{cfg: cfg.withHTTPClient()}
		return c.getAndWrite(stdout, "/api/accounts", nil, cfg.Output)
	case "folders":
		accountID := fs.String("account-id", "", "Zoho Mail account ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		id := firstNonEmpty(*accountID, cfg.AccountID)
		if id == "" {
			return errors.New("missing --account-id; run zohomail-pp-cli configure")
		}
		c := client{cfg: cfg.withHTTPClient()}
		return c.getAndWrite(stdout, "/api/accounts/"+pathEscape(id)+"/folders", nil, cfg.Output)
	case "inbox", "sent", "spam", "trash", "archive":
		folderName := args[0]
		start := fs.Int("start", 1, "start offset")
		limit := fs.Int("limit", 20, "max rows")
		sortBy := fs.String("sort-by", "", "Zoho sort key, for example date")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if cfg.Folders[folderName] == "" {
			return fmt.Errorf("missing configured %s folder; run zohomail-pp-cli configure", folderName)
		}
		return listMessages(stdout, cfg, cfg.AccountID, cfg.Folders[folderName], *start, *limit, *sortBy)
	case "list":
		accountID := fs.String("account-id", "", "Zoho Mail account ID")
		folderID := fs.String("folder-id", "", "folder ID")
		folder := fs.String("folder", "", "named configured folder: inbox, sent, spam, trash, archive")
		start := fs.Int("start", 1, "start offset")
		limit := fs.Int("limit", 20, "max rows")
		sortBy := fs.String("sort-by", "", "Zoho sort key, for example date")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		id := firstNonEmpty(*folderID, cfg.Folders[strings.ToLower(*folder)])
		return listMessages(stdout, cfg, firstNonEmpty(*accountID, cfg.AccountID), id, *start, *limit, *sortBy)
	case "search":
		accountID := fs.String("account-id", "", "Zoho Mail account ID")
		searchKey := fs.String("search-key", "", "Zoho search syntax, for example from:person@example.com")
		start := fs.Int("start", 1, "start offset")
		limit := fs.Int("limit", 20, "max rows")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		id := firstNonEmpty(*accountID, cfg.AccountID)
		if id == "" || *searchKey == "" {
			return errors.New("missing --account-id or --search-key; run zohomail-pp-cli configure")
		}
		q := url.Values{"searchKey": {*searchKey}, "start": {fmt.Sprint(*start)}, "limit": {fmt.Sprint(*limit)}}
		c := client{cfg: cfg.withHTTPClient()}
		return c.getAndWrite(stdout, "/api/accounts/"+pathEscape(id)+"/messages/search", q, cfg.Output)
	case "read":
		accountID := fs.String("account-id", "", "Zoho Mail account ID")
		folderID := fs.String("folder-id", "", "folder ID")
		folder := fs.String("folder", "", "named configured folder: inbox, sent, spam, trash, archive")
		messageID := fs.String("message-id", "", "message ID")
		mode := fs.String("mode", "content", "content or details")
		includeBlock := fs.Bool("include-block-content", false, "include quoted block content")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		id := firstNonEmpty(*accountID, cfg.AccountID)
		fid := firstNonEmpty(*folderID, cfg.Folders[strings.ToLower(*folder)])
		if id == "" || fid == "" || *messageID == "" {
			return errors.New("missing --account-id, --folder-id, or --message-id; run zohomail-pp-cli configure")
		}
		suffix := "content"
		q := url.Values{}
		if *mode == "details" {
			suffix = "details"
		} else if *includeBlock {
			q.Set("includeBlockContent", "true")
		}
		p := "/api/accounts/" + pathEscape(id) + "/folders/" + pathEscape(fid) + "/messages/" + pathEscape(*messageID) + "/" + suffix
		c := client{cfg: cfg.withHTTPClient()}
		return c.getAndWrite(stdout, p, q, cfg.Output)
	case "send":
		accountID := fs.String("account-id", "", "Zoho Mail account ID")
		from := fs.String("from", "", "from email address")
		to := fs.String("to", "", "comma-separated recipient addresses")
		cc := fs.String("cc", "", "comma-separated CC addresses")
		bcc := fs.String("bcc", "", "comma-separated BCC addresses")
		subject := fs.String("subject", "", "subject")
		content := fs.String("content", "", "plain text or HTML body")
		contentFile := fs.String("content-file", "", "read body from file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		id := firstNonEmpty(*accountID, cfg.AccountID)
		if id == "" || *from == "" || *to == "" || *subject == "" {
			return errors.New("missing --account-id, --from, --to, or --subject; run zohomail-pp-cli configure")
		}
		body := *content
		if *contentFile != "" {
			b, err := os.ReadFile(*contentFile)
			if err != nil {
				return err
			}
			body = string(b)
		}
		payload := map[string]string{
			"fromAddress": *from,
			"toAddress":   *to,
			"subject":     *subject,
			"content":     body,
		}
		if *cc != "" {
			payload["ccAddress"] = *cc
		}
		if *bcc != "" {
			payload["bccAddress"] = *bcc
		}
		c := client{cfg: cfg.withHTTPClient()}
		resp, err := c.request("POST", "/api/accounts/"+pathEscape(id)+"/messages", nil, payload)
		if err != nil {
			return err
		}
		return writeFormatted(stdout, resp, cfg.Output)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `zohomail-pp-cli - Zoho Mail CLI for Printing Press

Usage:
  zohomail-pp-cli <command> [options]

Commands:
  login      Open browser OAuth flow, save auth, discover defaults
  client-setup Open Zoho API Console and print required redirect/scopes
  auth-save  Save existing client/refresh credentials to local config
  auth-rbw   Save existing Zoho OAuth credentials from rbw/Bitwarden
  auth-url   Print OAuth consent URL
  token      Exchange OAuth code for tokens
  auth-device   Device authorization flow: prints code, polls until approved, saves refresh token
  auth-client-credentials  Headless: exchange client credentials for a short-lived access token (no refresh token)
  configure  Discover account/folder defaults and save local config
  doctor     Show auth/base configuration without printing secrets
  accounts   List authenticated user's Zoho Mail accounts
  folders    List folders for an account
  inbox      List configured Inbox
  sent       List configured Sent
  spam       List configured Spam
  trash      List configured Trash
  archive    List configured Archive
  list       List messages from a folder or account view
  search     Search messages with Zoho searchKey syntax
  read       Read message content or metadata
  send       Send email through Zoho Mail
  version    Print version

Auth env:
  ZOHO_MAIL_ACCESS_TOKEN, or
  ZOHO_REFRESH_TOKEN + ZOHO_CLIENT_ID + ZOHO_CLIENT_SECRET
  Or run: auth-device (interactive approval, saves refresh token)
           auth-client-credentials (headless, access token only, 1h TTL)

Base URL env:
  ZOHO_MAIL_BASE_URL      default https://mail.zoho.com
  ZOHO_ACCOUNTS_BASE_URL  default https://accounts.zoho.com

Global options:
  --output json|pretty
  --mail-base URL
  --accounts-base URL
`)
}

func addGlobalFlags(fs *flag.FlagSet, cfg *config) {
	fs.StringVar(&cfg.Output, "output", cfg.Output, "json or pretty")
	fs.StringVar(&cfg.MailBase, "mail-base", cfg.MailBase, "Zoho Mail base URL")
	fs.StringVar(&cfg.AccountsBase, "accounts-base", cfg.AccountsBase, "Zoho Accounts base URL")
}

func configFromEnv() config {
	path := configPath()
	cfg, _ := loadConfig(path)
	cfg.ConfigPath = path
	if cfg.Folders == nil {
		cfg.Folders = map[string]string{}
	}
	applyEnv(&cfg)
	if cfg.MailBase == "" {
		cfg.MailBase = "https://mail.zoho.com"
	}
	if cfg.AccountsBase == "" {
		cfg.AccountsBase = "https://accounts.zoho.com"
	}
	if cfg.Output == "" {
		cfg.Output = "pretty"
	}
	return cfg
}

func applyEnv(cfg *config) {
	if v := os.Getenv("ZOHO_MAIL_BASE_URL"); v != "" {
		cfg.MailBase = v
	}
	if v := os.Getenv("ZOHO_ACCOUNTS_BASE_URL"); v != "" {
		cfg.AccountsBase = v
	}
	if v := os.Getenv("ZOHO_MAIL_ACCESS_TOKEN"); v != "" {
		cfg.AccessToken = v
	}
	if v := os.Getenv("ZOHO_REFRESH_TOKEN"); v != "" {
		cfg.RefreshToken = v
	}
	if v := os.Getenv("ZOHO_CLIENT_ID"); v != "" {
		cfg.ClientID = v
	}
	if v := os.Getenv("ZOHO_CLIENT_SECRET"); v != "" {
		cfg.ClientSecret = v
	}
	if v := os.Getenv("ZOHO_ACCOUNT_ID"); v != "" {
		cfg.AccountID = v
	}
	if v := os.Getenv("ZOHO_MAIL_OUTPUT"); v != "" {
		cfg.Output = v
	}
	folderEnv := map[string]string{
		"inbox":   os.Getenv("ZOHO_INBOX_FOLDER_ID"),
		"sent":    os.Getenv("ZOHO_SENT_FOLDER_ID"),
		"spam":    os.Getenv("ZOHO_SPAM_FOLDER_ID"),
		"trash":   os.Getenv("ZOHO_TRASH_FOLDER_ID"),
		"archive": os.Getenv("ZOHO_ARCHIVE_FOLDER_ID"),
	}
	for name, id := range folderEnv {
		if id != "" {
			cfg.Folders[name] = id
		}
	}
}

func printDoctor(w io.Writer, cfg config) {
	fmt.Fprintf(w, "config\t%s\n", cfg.ConfigPath)
	fmt.Fprintf(w, "mail_base\t%s\n", cfg.MailBase)
	fmt.Fprintf(w, "accounts_base\t%s\n", cfg.AccountsBase)
	fmt.Fprintf(w, "account_id\t%s\n", presentID(cfg.AccountID))
	printFolderDefaults(w, cfg.Folders)
	fmt.Fprintf(w, "client_id\t%s\n", presentID(cfg.ClientID))
	fmt.Fprintf(w, "client_secret\t%s\n", presentSecret(cfg.ClientSecret))
	fmt.Fprintf(w, "refresh_token\t%s\n", presentSecret(cfg.RefreshToken))
	fmt.Fprintf(w, "access_token\t%s\n", presentSecret(cfg.AccessToken))
}

func presentID(v string) string {
	if v == "" {
		return "missing"
	}
	if len(v) <= 12 {
		return "set"
	}
	return v[:8] + "..." + v[len(v)-4:]
}

func presentSecret(v string) string {
	if v == "" {
		return "missing"
	}
	return "set"
}

func (cfg config) withHTTPClient() config {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	return cfg
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func configPath() string {
	if v := os.Getenv("PP_ZOHOMAIL_CONFIG"); v != "" {
		return v
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return ".zohomail-pp-cli.json"
	}
	return dir + "/zohomail-pp-cli/config.json"
}

func loadConfig(path string) (config, error) {
	if path == "" {
		return config{}, errors.New("missing config path")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return config{}, err
	}
	var cfg config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return config{}, err
	}
	if cfg.Folders == nil {
		cfg.Folders = map[string]string{}
	}
	return cfg, nil
}

func saveConfig(cfg config) error {
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = configPath()
	}
	if cfg.Folders == nil {
		cfg.Folders = map[string]string{}
	}
	if idx := strings.LastIndex(cfg.ConfigPath, "/"); idx > 0 {
		if err := os.MkdirAll(cfg.ConfigPath[:idx], 0700); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(cfg.ConfigPath, b, 0600)
}

func saveTokenConfig(cfg config, body []byte) error {
	var parsed struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return err
	}
	if parsed.RefreshToken == "" {
		return errors.New("token response missing refresh_token")
	}
	cfg.RefreshToken = parsed.RefreshToken
	cfg.AccessToken = ""
	return saveConfig(cfg)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func printFolderDefaults(w io.Writer, folders map[string]string) {
	for _, name := range []string{"inbox", "sent", "spam", "trash", "archive"} {
		fmt.Fprintf(w, "folder_%s\t%s\n", name, presentID(folders[name]))
	}
}

func hasFolderFlag(folders map[string]string) bool {
	for _, id := range folders {
		if id != "" {
			return true
		}
	}
	return false
}

func authURL(accountsBase, clientID, redirect, scopes string) (string, error) {
	return authURLWithState(accountsBase, clientID, redirect, scopes, "")
}

func authURLWithState(accountsBase, clientID, redirect, scopes, state string) (string, error) {
	if clientID == "" {
		return "", errors.New("missing ZOHO_CLIENT_ID")
	}
	u, err := url.Parse(strings.TrimRight(accountsBase, "/") + "/oauth/v2/auth")
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("scope", strings.ReplaceAll(scopes, ",", " "))
	q.Set("client_id", clientID)
	q.Set("response_type", "code")
	q.Set("access_type", "offline")
	q.Set("redirect_uri", redirect)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func login(stdout, stderr io.Writer, cfg config, redirect, scopes string, timeout time.Duration, openBrowser bool) error {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return errors.New("missing ZOHO_CLIENT_ID or ZOHO_CLIENT_SECRET")
	}
	listener, actualRedirect, err := listenForRedirect(redirect)
	if err != nil {
		return err
	}
	defer listener.Close()

	state, err := randomState()
	if err != nil {
		return err
	}
	u, err := authURLWithState(cfg.AccountsBase, cfg.ClientID, actualRedirect, scopes, state)
	if err != nil {
		return err
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	server := &http.Server{Handler: callbackHandler(state, codeCh, errCh)}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	defer server.Close()

	fmt.Fprintf(stdout, "open\t%s\n", u)
	if openBrowser {
		if err := openURL(u); err != nil {
			fmt.Fprintf(stderr, "open failed\t%s\n", err)
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return err
	case <-timer.C:
		return errors.New("timed out waiting for OAuth callback")
	}

	c := client{cfg: cfg}
	body, err := c.tokenRequest(url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"code":          {code},
		"redirect_uri":  {actualRedirect},
	})
	if err != nil {
		return err
	}
	if err := saveTokenConfig(cfg, body); err != nil {
		return err
	}
	next, err := loadConfig(cfg.ConfigPath)
	if err != nil {
		return err
	}
	next.ConfigPath = cfg.ConfigPath
	next.HTTPClient = cfg.HTTPClient
	if err := discoverAndSaveConfig(next); err != nil {
		return err
	}
	finalCfg, _ := loadConfig(cfg.ConfigPath)
	fmt.Fprintf(stdout, "saved\t%s\n", cfg.ConfigPath)
	fmt.Fprintf(stdout, "account_id\t%s\n", presentID(finalCfg.AccountID))
	printFolderDefaults(stdout, finalCfg.Folders)
	return nil
}

func deviceAuth(stdout, stderr io.Writer, cfg config, scopes string, openBrowser, save bool) error {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return errors.New("missing ZOHO_CLIENT_ID or ZOHO_CLIENT_SECRET")
	}
	codeURL := strings.TrimRight(cfg.AccountsBase, "/") + "/oauth/v3/device/code"
	req, err := http.NewRequest("POST", codeURL, strings.NewReader(url.Values{
		"client_id":  {cfg.ClientID},
		"scope":      {strings.ReplaceAll(scopes, ",", " ")},
		"grant_type": {"device_request"},
	}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	codeBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("device code request failed: %s: %s", resp.Status, string(codeBody))
	}
	var codeResp struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURL string `json:"verification_url"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
		Error           string `json:"error"`
	}
	if err := json.Unmarshal(codeBody, &codeResp); err != nil {
		return err
	}
	if codeResp.Error != "" {
		if codeResp.Error == "invalid_client" {
			return errors.New("device auth requires a Non-browser Application client in Zoho API Console; Self Client does not support this grant — use auth-client-credentials instead")
		}
		return fmt.Errorf("device code error: %s", codeResp.Error)
	}
	fmt.Fprintf(stdout, "verification_url\t%s\n", codeResp.VerificationURL)
	fmt.Fprintf(stdout, "user_code\t%s\n", codeResp.UserCode)
	if openBrowser {
		if err := openURL(codeResp.VerificationURL); err != nil {
			fmt.Fprintf(stderr, "open failed\t%s\n", err)
		}
	}
	interval := codeResp.Interval
	if interval <= 0 {
		interval = 5
	}
	expiresIn := codeResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 180
	}
	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
	tokenURL := strings.TrimRight(cfg.AccountsBase, "/") + "/oauth/v3/device/token"
	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(interval) * time.Second)
		fmt.Fprint(stderr, ".")
		pollReq, err := http.NewRequest("POST", tokenURL, strings.NewReader(url.Values{
			"client_id":     {cfg.ClientID},
			"client_secret": {cfg.ClientSecret},
			"device_code":   {codeResp.DeviceCode},
			"grant_type":    {"device_token"},
		}.Encode()))
		if err != nil {
			return err
		}
		pollReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		pollResp, err := cfg.HTTPClient.Do(pollReq)
		if err != nil {
			return err
		}
		pollBody, _ := io.ReadAll(pollResp.Body)
		pollResp.Body.Close()
		var tokenResp struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			Error        string `json:"error"`
		}
		if err := json.Unmarshal(pollBody, &tokenResp); err != nil {
			return err
		}
		if tokenResp.Error == "authorization_pending" {
			continue
		}
		if tokenResp.AccessToken != "" {
			fmt.Fprintln(stderr, " approved")
			if err := writeJSON(stdout, pollBody); err != nil {
				return err
			}
			if save {
				if err := saveTokenConfig(cfg, pollBody); err != nil {
					return err
				}
				next, err := loadConfig(cfg.ConfigPath)
				if err != nil {
					return err
				}
				next.ConfigPath = cfg.ConfigPath
				next.HTTPClient = cfg.HTTPClient
				if err := discoverAndSaveConfig(next); err != nil {
					return err
				}
				fmt.Fprintf(stdout, "saved\t%s\n", cfg.ConfigPath)
			}
			return nil
		}
		if tokenResp.Error != "" {
			fmt.Fprintln(stderr, "")
			return fmt.Errorf("device token error: %s", tokenResp.Error)
		}
	}
	fmt.Fprintln(stderr, "")
	return errors.New("device authorization timed out")
}

func listenForRedirect(redirect string) (net.Listener, string, error) {
	u, err := url.Parse(redirect)
	if err != nil {
		return nil, "", err
	}
	if u.Scheme != "http" {
		return nil, "", errors.New("login redirect URI must use http localhost")
	}
	host := u.Hostname()
	if host != "localhost" && host != "127.0.0.1" {
		return nil, "", errors.New("login redirect URI must use localhost or 127.0.0.1")
	}
	port := u.Port()
	if port == "" {
		port = "80"
	}
	listener, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		return nil, "", err
	}
	if u.Port() == "" || u.Port() == "0" {
		addr := listener.Addr().(*net.TCPAddr)
		u.Host = u.Hostname() + fmt.Sprintf(":%d", addr.Port)
	}
	return listener, u.String(), nil
}

func callbackHandler(wantState string, codeCh chan<- string, errCh chan<- error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != wantState {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			errCh <- errors.New("OAuth state mismatch")
			return
		}
		if e := r.URL.Query().Get("error"); e != "" {
			http.Error(w, e, http.StatusBadRequest)
			errCh <- fmt.Errorf("OAuth error: %s", e)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- errors.New("OAuth callback missing code")
			return
		}
		fmt.Fprintln(w, "zohomail-pp-cli login complete. You can close this tab.")
		codeCh <- code
	})
}

func discoverAndSaveConfig(cfg config) error {
	c := client{cfg: cfg.withHTTPClient()}
	discovered, err := c.discoverConfig(cfg.AccountID)
	if err != nil {
		return err
	}
	discovered.ConfigPath = cfg.ConfigPath
	discovered.ClientID = cfg.ClientID
	discovered.ClientSecret = cfg.ClientSecret
	discovered.RefreshToken = cfg.RefreshToken
	return saveConfig(discovered)
}

func authFromRBW(cfg config, rbwBin, item, clientIDField, clientSecretField, refreshTokenField string, noDiscover bool) (config, error) {
	values := map[string]*string{
		clientIDField:     &cfg.ClientID,
		clientSecretField: &cfg.ClientSecret,
		refreshTokenField: &cfg.RefreshToken,
	}
	for field, target := range values {
		value, err := rbwField(rbwBin, item, field)
		if err != nil {
			return config{}, err
		}
		*target = value
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RefreshToken == "" {
		return config{}, errors.New("rbw item missing Zoho client ID, client secret, or refresh token")
	}
	cfg.AccessToken = ""
	if noDiscover {
		if err := saveConfig(cfg); err != nil {
			return config{}, err
		}
		return cfg, nil
	}
	if err := discoverAndSaveConfig(cfg); err != nil {
		return config{}, err
	}
	saved, err := loadConfig(cfg.ConfigPath)
	if err != nil {
		return config{}, err
	}
	saved.ConfigPath = cfg.ConfigPath
	return saved, nil
}

func rbwField(rbwBin, item, field string) (string, error) {
	cmd := exec.Command(rbwBin, "get", item, "--field", field)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("rbw get field %q failed: %s", field, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func openURL(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	return cmd.Start()
}

func (c client) bearerToken() (string, error) {
	if c.cfg.AccessToken != "" {
		return c.cfg.AccessToken, nil
	}
	if c.cfg.RefreshToken == "" || c.cfg.ClientID == "" || c.cfg.ClientSecret == "" {
		return "", errors.New("missing auth: set ZOHO_MAIL_ACCESS_TOKEN or refresh-token env trio")
	}
	body, err := c.tokenRequest(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {c.cfg.RefreshToken},
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
	})
	if err != nil {
		return "", err
	}
	var parsed struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.AccessToken == "" {
		if parsed.Error != "" {
			return "", fmt.Errorf("token refresh failed: %s", parsed.Error)
		}
		return "", errors.New("token refresh response missing access_token")
	}
	return parsed.AccessToken, nil
}

func (c client) tokenRequest(values url.Values) ([]byte, error) {
	u := strings.TrimRight(c.cfg.AccountsBase, "/") + "/oauth/v2/token"
	req, err := http.NewRequest("POST", u, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token request failed: %s: %s", resp.Status, string(body))
	}
	return body, nil
}

func (c client) getAndWrite(w io.Writer, path string, q url.Values, output string) error {
	body, err := c.request("GET", path, q, nil)
	if err != nil {
		return err
	}
	return writeFormatted(w, body, output)
}

func (c client) getData(path string, q url.Values) (json.RawMessage, error) {
	body, err := c.request("GET", path, q, nil)
	if err != nil {
		return nil, err
	}
	var env apiEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	if len(env.Data) == 0 {
		return nil, errors.New("response missing data")
	}
	return env.Data, nil
}

func (c client) discoverConfig(accountID string) (config, error) {
	next := c.cfg
	next.Folders = map[string]string{}
	raw, err := c.getData("/api/accounts", nil)
	if err != nil {
		return config{}, err
	}
	var accounts []map[string]any
	if err := json.Unmarshal(raw, &accounts); err != nil {
		return config{}, err
	}
	next.AccountID = firstNonEmpty(accountID, pickAccountID(accounts))
	if next.AccountID == "" {
		return config{}, errors.New("no Zoho Mail account found")
	}
	raw, err = c.getData("/api/accounts/"+pathEscape(next.AccountID)+"/folders", nil)
	if err != nil {
		return config{}, err
	}
	var folders []map[string]any
	if err := json.Unmarshal(raw, &folders); err != nil {
		return config{}, err
	}
	next.Folders = pickFolderIDs(folders)
	return next, nil
}

func pickAccountID(accounts []map[string]any) string {
	for _, row := range accounts {
		if truthy(row["isDefaultAccount"]) {
			return fmt.Sprint(row["accountId"])
		}
	}
	for _, row := range accounts {
		if id := fmt.Sprint(row["accountId"]); id != "" && id != "<nil>" {
			return id
		}
	}
	return ""
}

func pickFolderIDs(rows []map[string]any) map[string]string {
	out := map[string]string{}
	targets := map[string][]string{
		"inbox":   {"Inbox", "/Inbox"},
		"sent":    {"Sent", "/Sent"},
		"spam":    {"Spam", "/Spam"},
		"trash":   {"Trash", "/Trash"},
		"archive": {"Archive", "/Archive"},
	}
	for key, names := range targets {
		for _, row := range rows {
			folderID := fmt.Sprint(row["folderId"])
			folderName := fmt.Sprint(row["folderName"])
			folderPath := fmt.Sprint(row["path"])
			for _, name := range names {
				if strings.EqualFold(folderName, name) || strings.EqualFold(folderPath, name) {
					out[key] = folderID
					break
				}
			}
			if out[key] != "" {
				break
			}
		}
	}
	return out
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true")
	default:
		return fmt.Sprint(v) == "true"
	}
}

func listMessages(w io.Writer, cfg config, accountID, folderID string, start, limit int, sortBy string) error {
	if accountID == "" {
		return errors.New("missing --account-id; run zohomail-pp-cli configure")
	}
	q := url.Values{"start": {fmt.Sprint(start)}, "limit": {fmt.Sprint(limit)}}
	if folderID != "" {
		q.Set("folderId", folderID)
	}
	if sortBy != "" {
		q.Set("sortBy", sortBy)
	}
	c := client{cfg: cfg.withHTTPClient()}
	return c.getAndWrite(w, "/api/accounts/"+pathEscape(accountID)+"/messages/view", q, cfg.Output)
}

func (c client) request(method, path string, q url.Values, payload any) ([]byte, error) {
	token, err := c.bearerToken()
	if err != nil {
		return nil, err
	}
	u := strings.TrimRight(c.cfg.MailBase, "/") + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s failed: %s: %s", method, path, resp.Status, string(respBody))
	}
	return respBody, nil
}

func writeFormatted(w io.Writer, body []byte, output string) error {
	if output == "json" {
		return writeJSON(w, body)
	}
	var env apiEnvelope
	if err := json.Unmarshal(body, &env); err != nil || len(env.Data) == 0 {
		return writeJSON(w, body)
	}
	return writePrettyData(w, env.Data)
}

func writeJSON(w io.Writer, body []byte) error {
	var out bytes.Buffer
	if err := json.Indent(&out, body, "", "  "); err != nil {
		_, err = w.Write(body)
		return err
	}
	out.WriteByte('\n')
	_, err := w.Write(out.Bytes())
	return err
}

func writePrettyData(w io.Writer, raw json.RawMessage) error {
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err == nil {
		return printRows(w, rows)
	}
	var row map[string]any
	if err := json.Unmarshal(raw, &row); err == nil {
		for _, k := range sortedKeys(row) {
			fmt.Fprintf(w, "%s\t%v\n", k, row[k])
		}
		return nil
	}
	return writeJSON(w, raw)
}

func printRows(w io.Writer, rows []map[string]any) error {
	if len(rows) == 0 {
		fmt.Fprintln(w, "No rows.")
		return nil
	}
	keys := preferredKeys(rows)
	fmt.Fprintln(w, strings.Join(keys, "\t"))
	for _, row := range rows {
		vals := make([]string, len(keys))
		for i, k := range keys {
			vals[i] = strings.ReplaceAll(fmt.Sprint(row[k]), "\n", " ")
		}
		fmt.Fprintln(w, strings.Join(vals, "\t"))
	}
	return nil
}

func preferredKeys(rows []map[string]any) []string {
	if hasAny(rows, "accountId") {
		return presentKeys(rows, []string{"accountId", "mailboxAddress", "displayName", "accountName", "isDefaultAccount", "zuid"})
	}
	if hasAny(rows, "messageId") {
		return presentKeys(rows, []string{"messageId", "subject", "fromAddress", "sender", "receivedTime", "sentDateInGMT", "summary", "status", "folderId"})
	}
	if hasAny(rows, "folderId") {
		return presentKeys(rows, []string{"folderId", "folderName", "path", "folderType", "imapAccess"})
	}
	preferred := []string{"accountId", "mailboxAddress", "displayName", "folderId", "folderName", "messageId", "subject", "fromAddress", "sender", "receivedTime", "sentDateInGMT", "status"}
	return presentKeysWithRest(rows, preferred)
}

func presentKeys(rows []map[string]any, preferred []string) []string {
	seen := map[string]bool{}
	for _, row := range rows {
		for k := range row {
			seen[k] = true
		}
	}
	var keys []string
	for _, k := range preferred {
		if seen[k] {
			keys = append(keys, k)
		}
	}
	return keys
}

func presentKeysWithRest(rows []map[string]any, preferred []string) []string {
	keys := presentKeys(rows, preferred)
	seen := map[string]bool{}
	for _, row := range rows {
		for k := range row {
			seen[k] = true
		}
	}
	for _, k := range keys {
		delete(seen, k)
	}
	rest := make([]string, 0, len(seen))
	for k := range seen {
		rest = append(rest, k)
	}
	sort.Strings(rest)
	return append(keys, rest...)
}

func hasAny(rows []map[string]any, key string) bool {
	for _, row := range rows {
		if _, ok := row[key]; ok {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func pathEscape(s string) string {
	return url.PathEscape(strings.TrimSpace(s))
}
