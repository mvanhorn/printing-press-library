package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const baseURL = "https://api.cloudflare.com/client/v4"

var version = "dev"

type cfEnvelope struct {
	Success  bool            `json:"success"`
	Errors   []cfMessage     `json:"errors,omitempty"`
	Messages []cfMessage     `json:"messages,omitempty"`
	Result   json.RawMessage `json:"result,omitempty"`
}

type cfMessage struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type config struct {
	Token     string
	AccountID string
	HTTP      *http.Client
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUsage(stdout)
		return nil
	}
	if args[0] == "--version" || args[0] == "version" {
		fmt.Fprintln(stdout, version)
		return nil
	}

	cfg := config{
		Token:     firstNonEmpty(os.Getenv("CLOUDFLARE_API_TOKEN"), os.Getenv("CF_API_TOKEN")),
		AccountID: firstNonEmpty(os.Getenv("CLOUDFLARE_ACCOUNT_ID"), os.Getenv("ACCOUNT_ID")),
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}

	switch args[0] {
	case "doctor":
		return doctor(cfg, stdout)
	case "domain-search":
		return domainSearch(cfg, args[1:], stdout)
	case "domain-check":
		return domainCheck(cfg, args[1:], stdout)
	case "domain-register":
		return domainRegister(cfg, args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q\nrun: cf-domain-pp-cli help", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `cf-domain-pp-cli - Cloudflare Registrar domain CLI

Commands:
  doctor                         Validate required env without printing secrets
  domain-search --query NAME     Search available domain ideas
  domain-check --domain NAME     Live check availability/pricing for exact domain
  domain-register --domain NAME --confirm-domain NAME
                                 Register exact domain; confirmation is required
  version                        Print version

Env:
  CLOUDFLARE_API_TOKEN or CF_API_TOKEN
  CLOUDFLARE_ACCOUNT_ID or ACCOUNT_ID

Safety:
  domain-register refuses to run unless --confirm-domain exactly matches --domain.
  Always run domain-check immediately before registering and confirm returned price.
`)
}

func doctor(cfg config, w io.Writer) error {
	missing := missingEnv(cfg)
	out := map[string]any{
		"ok":                  len(missing) == 0,
		"missing":             missing,
		"has_api_token":       cfg.Token != "",
		"has_account_id":      cfg.AccountID != "",
		"required_permission": "Account:Registrar:Edit scoped to the specific account",
	}
	return writeJSON(w, out)
}

func domainSearch(cfg config, args []string, w io.Writer) error {
	fs := flag.NewFlagSet("domain-search", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	query := fs.String("query", "", "search query")
	limit := fs.String("limit", "20", "result limit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*query) == "" {
		return errors.New("domain-search requires --query")
	}
	if err := requireAuth(cfg); err != nil {
		return err
	}
	u := fmt.Sprintf("%s/accounts/%s/registrar/domain-search?q=%s&limit=%s", baseURL, url.PathEscape(cfg.AccountID), url.QueryEscape(*query), url.QueryEscape(*limit))
	return callAndWrite(cfg, http.MethodGet, u, nil, w)
}

func domainCheck(cfg config, args []string, w io.Writer) error {
	fs := flag.NewFlagSet("domain-check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	domain := fs.String("domain", "", "domain name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	name := normalizeDomain(*domain)
	if name == "" {
		return errors.New("domain-check requires --domain")
	}
	if err := requireAuth(cfg); err != nil {
		return err
	}
	body := map[string][]string{"domains": {name}}
	u := fmt.Sprintf("%s/accounts/%s/registrar/domain-check", baseURL, url.PathEscape(cfg.AccountID))
	return callAndWrite(cfg, http.MethodPost, u, body, w)
}

func domainRegister(cfg config, args []string, w io.Writer) error {
	fs := flag.NewFlagSet("domain-register", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	domain := fs.String("domain", "", "domain name")
	confirm := fs.String("confirm-domain", "", "must exactly match --domain")
	if err := fs.Parse(args); err != nil {
		return err
	}
	name := normalizeDomain(*domain)
	if name == "" {
		return errors.New("domain-register requires --domain")
	}
	if normalizeDomain(*confirm) != name {
		return fmt.Errorf("refusing paid registration: --confirm-domain must exactly match %q", name)
	}
	if err := requireAuth(cfg); err != nil {
		return err
	}
	body := map[string]string{"domain_name": name}
	u := fmt.Sprintf("%s/accounts/%s/registrar/registrations", baseURL, url.PathEscape(cfg.AccountID))
	return callAndWrite(cfg, http.MethodPost, u, body, w)
}

func callAndWrite(cfg config, method, endpoint string, body any, w io.Writer) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := cfg.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cloudflare api failed: HTTP %d: %s", resp.StatusCode, sanitize(string(payload)))
	}
	var env cfEnvelope
	if err := json.Unmarshal(payload, &env); err == nil && !env.Success && len(env.Errors) > 0 {
		return fmt.Errorf("cloudflare api error: %s", summarizeMessages(env.Errors))
	}
	_, err = w.Write(append(payload, '\n'))
	return err
}

func requireAuth(cfg config) error {
	if missing := missingEnv(cfg); len(missing) > 0 {
		return fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}
	return nil
}

func missingEnv(cfg config) []string {
	var missing []string
	if cfg.Token == "" {
		missing = append(missing, "CLOUDFLARE_API_TOKEN")
	}
	if cfg.AccountID == "" {
		missing = append(missing, "CLOUDFLARE_ACCOUNT_ID")
	}
	return missing
}

func writeJSON(w io.Writer, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

func normalizeDomain(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func summarizeMessages(messages []cfMessage) string {
	parts := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg.Code != 0 {
			parts = append(parts, fmt.Sprintf("%d %s", msg.Code, msg.Message))
		} else {
			parts = append(parts, msg.Message)
		}
	}
	return strings.Join(parts, "; ")
}

func sanitize(s string) string {
	for _, env := range []string{"CLOUDFLARE_API_TOKEN", "CF_API_TOKEN"} {
		if token := os.Getenv(env); token != "" {
			s = strings.ReplaceAll(s, token, "[redacted]")
		}
	}
	return s
}
