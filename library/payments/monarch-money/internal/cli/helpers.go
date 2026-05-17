package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

const (
	baseURL       = "https://api.monarch.com"
	loginEndpoint = baseURL + "/auth/login/"
	graphqlURL    = baseURL + "/graphql"
)

type sessionFile struct {
	Token string `json:"token"`
}

type gqlRequest struct {
	OperationName string         `json:"operationName,omitempty"`
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables,omitempty"`
}

type gqlResponse struct {
	Data   map[string]any `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printDryRun(operation string, variables map[string]any) error {
	return printJSON(map[string]any{
		"dryRun":    true,
		"operation": operation,
		"variables": variables,
		"applyHint": "rerun with --yes to apply this write",
	})
}

func printError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

func sessionPath() string {
	if p := os.Getenv("MONARCH_SESSION_FILE"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".monarch-pp-cli-session.json"
	}
	return filepath.Join(home, ".monarch-pp-cli", "session.json")
}

func loadToken() (string, error) {
	if token := strings.TrimSpace(os.Getenv("MONARCH_TOKEN")); token != "" {
		return token, nil
	}
	path := sessionPath()
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("not authenticated: run `monarch-money-pp-cli login` or set MONARCH_TOKEN")
		}
		return "", err
	}
	var sf sessionFile
	if err := json.Unmarshal(b, &sf); err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	if strings.TrimSpace(sf.Token) == "" {
		return "", fmt.Errorf("session file %s has no token", path)
	}
	return sf.Token, nil
}

func saveToken(token string) error {
	path := sessionPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(sessionFile{Token: token}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

func loginRequest(email, password, mfa string) (string, error) {
	payload := map[string]any{
		"username":       email,
		"password":       password,
		"supports_mfa":   true,
		"trusted_device": true,
	}
	if strings.TrimSpace(mfa) != "" {
		payload["totp"] = strings.TrimSpace(mfa)
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, loginEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	setBaseHeaders(req)
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading login response: %w", err)
	}
	if resp.StatusCode == http.StatusForbidden && strings.TrimSpace(mfa) == "" {
		return "", fmt.Errorf("multi-factor authentication required; rerun login with --mfa <code>")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("login failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var parsed struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if parsed.Token == "" {
		return "", fmt.Errorf("login succeeded but response did not include a token")
	}
	if strings.Count(parsed.Token, ".") == 2 {
		return "", fmt.Errorf("refusing to save JWT-style short-lived features token")
	}
	return parsed.Token, nil
}

func graphql(operation, query string, variables map[string]any) (map[string]any, error) {
	token, err := loadToken()
	if err != nil {
		return nil, err
	}
	payload := gqlRequest{OperationName: operation, Query: query, Variables: variables}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, graphqlURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	setBaseHeaders(req)
	req.Header.Set("Authorization", "Token "+token)
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading graphql response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("graphql HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var parsed gqlResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Errors) > 0 {
		msgs := make([]string, 0, len(parsed.Errors))
		for _, e := range parsed.Errors {
			msgs = append(msgs, e.Message)
		}
		return nil, fmt.Errorf("graphql error: %s", strings.Join(msgs, "; "))
	}
	return parsed.Data, nil
}

func payloadErrors(v any) string {
	errs := asSlice(v)
	if len(errs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(errs))
	for _, ev := range errs {
		em := asMap(ev)
		msg := str(em["message"])
		if msg != "" {
			parts = append(parts, msg)
		}
		for _, fv := range asSlice(em["fieldErrors"]) {
			fm := asMap(fv)
			field := str(fm["field"])
			messages := []string{}
			for _, mv := range asSlice(fm["messages"]) {
				messages = append(messages, str(mv))
			}
			if field != "" && len(messages) > 0 {
				parts = append(parts, fmt.Sprintf("%s: %s", field, strings.Join(messages, ", ")))
			}
		}
	}
	return strings.Join(parts, "; ")
}

func setBaseHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Client-Platform", "web")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "monarch-money-pp-cli (https://github.com/mvanhorn/printing-press-library)")
}

func asSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func field(m map[string]any, keys ...string) any {
	var cur any = m
	for _, key := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[key]
	}
	return cur
}

func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	default:
		return 0
	}
}

func money(v any) string {
	amount := num(v)
	if amount < 0 {
		return fmt.Sprintf("-$%.2f", -amount)
	}
	return fmt.Sprintf("$%.2f", amount)
}

func table(headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	fmt.Fprintln(tw, strings.Join(dashes(len(headers)), "\t"))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	return tw.Flush()
}

func dashes(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "---"
	}
	return out
}

func sortRows(rows [][]string, col int) {
	sort.SliceStable(rows, func(i, j int) bool {
		if col >= len(rows[i]) || col >= len(rows[j]) {
			return false
		}
		return strings.ToLower(rows[i][col]) < strings.ToLower(rows[j][col])
	})
}
