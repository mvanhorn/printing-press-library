// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/auth/1password/internal/cliutil"
	"github.com/spf13/cobra"
)

type opItemSummary struct {
	ID        string         `json:"id,omitempty"`
	Title     string         `json:"title,omitempty"`
	Category  string         `json:"category,omitempty"`
	Vault     opVaultSummary `json:"vault,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
	URLs      []opURL        `json:"urls,omitempty"`
	CreatedAt string         `json:"created_at,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
}

type opVaultSummary struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type opURL struct {
	Label string `json:"label,omitempty"`
	Href  string `json:"href,omitempty"`
}

type opItemDetail struct {
	opItemSummary
	Fields []opField `json:"fields,omitempty"`
}

type opField struct {
	ID        string `json:"id,omitempty"`
	Label     string `json:"label,omitempty"`
	Type      string `json:"type,omitempty"`
	Purpose   string `json:"purpose,omitempty"`
	Reference string `json:"reference,omitempty"`
	Value     any    `json:"-"`
}

type opDocumentSummary struct {
	ID        string         `json:"id,omitempty"`
	Title     string         `json:"title,omitempty"`
	Vault     opVaultSummary `json:"vault,omitempty"`
	CreatedAt string         `json:"created_at,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
	Size      int64          `json:"size,omitempty"`
}

type refParts struct {
	Raw     string `json:"ref"`
	Vault   string `json:"vault"`
	Item    string `json:"item"`
	Section string `json:"section,omitempty"`
	Field   string `json:"field"`
	Query   string `json:"query,omitempty"`
}

type finding struct {
	Severity string `json:"severity"`
	Kind     string `json:"kind"`
	ItemID   string `json:"item_id,omitempty"`
	Title    string `json:"title,omitempty"`
	Vault    string `json:"vault,omitempty"`
	Category string `json:"category,omitempty"`
	Field    string `json:"field,omitempty"`
	Reason   string `json:"reason"`
	Ref      string `json:"ref,omitempty"`
}

type opRunner struct {
	verify bool
}

func newOpRunner() opRunner {
	return opRunner{verify: cliutil.IsVerifyEnv()}
}

func (r opRunner) command(ctx context.Context, args ...string) ([]byte, []byte, error) {
	if r.verify {
		return []byte("{}"), nil, nil
	}
	path, err := exec.LookPath("op")
	if err != nil {
		return nil, nil, fmt.Errorf("op CLI not found on PATH: install 1Password CLI v2.18.0 or newer")
	}
	cmd := exec.CommandContext(ctx, path, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.Bytes(), stderr.Bytes(), fmt.Errorf("op %s failed: %s", strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

func (r opRunner) json(ctx context.Context, args []string, v any) error {
	args = append(args, "--format", "json")
	if r.verify {
		return decodeVerifyJSON(args, v)
	}
	out, _, err := r.command(ctx, args...)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return nil
	}
	return json.Unmarshal(out, v)
}

func decodeVerifyJSON(args []string, v any) error {
	switch {
	case hasOpCommand(args, "vault", "list"):
		return remarshal([]opVaultSummary{{ID: "verifyvault00000000000001", Name: "Verify Vault"}}, v)
	case hasOpCommand(args, "document", "list"):
		return remarshal([]opDocumentSummary{{ID: "verifydoc0000000000000001", Title: "deploy-cert.pem", Vault: opVaultSummary{ID: "verifyvault00000000000001", Name: "Verify Vault"}, Size: 2048}}, v)
	case hasOpCommand(args, "item", "get"):
		return remarshal(opItemDetail{
			opItemSummary: opItemSummary{ID: "verifyitem000000000000001", Title: "GitHub Token", Category: "API Credential", Vault: opVaultSummary{ID: "verifyvault00000000000001", Name: "Verify Vault"}, Tags: []string{"owner:platform", "purpose:ci", "env:staging"}},
			Fields:        []opField{{ID: "token", Label: "token", Type: "CONCEALED", Purpose: "PASSWORD", Reference: "op://Verify Vault/GitHub Token/token"}, {ID: "username", Label: "username", Type: "STRING", Reference: "op://Verify Vault/GitHub Token/username"}},
		}, v)
	case hasOpCommand(args, "item", "list"):
		return remarshal([]opItemSummary{
			{ID: "verifyitem000000000000001", Title: "GitHub Token", Category: "API Credential", Vault: opVaultSummary{ID: "verifyvault00000000000001", Name: "Verify Vault"}, Tags: []string{"owner:platform", "purpose:ci", "env:staging"}},
			{ID: "verifynote000000000000001", Title: "AWS keys note", Category: "Secure Note", Vault: opVaultSummary{ID: "verifyvault00000000000001", Name: "Verify Vault"}},
		}, v)
	default:
		return remarshal(map[string]any{"verify_noop": true}, v)
	}
}

func hasOpCommand(args []string, parts ...string) bool {
	if len(args) < len(parts) {
		return false
	}
	for i, part := range parts {
		if args[i] != part {
			return false
		}
	}
	return true
}

func remarshal(src, dst any) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func connectEnvConflicts() []string {
	var out []string
	for _, name := range []string{"OP_CONNECT_HOST", "OP_CONNECT_TOKEN"} {
		if os.Getenv(name) != "" {
			out = append(out, name)
		}
	}
	return out
}

func authMode() string {
	switch {
	case os.Getenv("OP_SERVICE_ACCOUNT_TOKEN") != "":
		return "service-account"
	case os.Getenv("OP_ACCOUNT") != "":
		return "desktop-or-session"
	default:
		return "op-default"
	}
}

func listItems(ctx context.Context, vault, categories string) ([]opItemSummary, error) {
	args := []string{"item", "list"}
	if vault != "" {
		args = append(args, "--vault", vault)
	}
	if categories != "" {
		args = append(args, "--categories", categories)
	}
	var items []opItemSummary
	return items, newOpRunner().json(ctx, args, &items)
}

func getItem(ctx context.Context, id, vault string) (opItemDetail, error) {
	args := []string{"item", "get", id}
	if vault != "" {
		args = append(args, "--vault", vault)
	}
	var item opItemDetail
	return item, newOpRunner().json(ctx, args, &item)
}

func listDocuments(ctx context.Context, vault string) ([]opDocumentSummary, error) {
	args := []string{"document", "list"}
	if vault != "" {
		args = append(args, "--vault", vault)
	}
	var docs []opDocumentSummary
	return docs, newOpRunner().json(ctx, args, &docs)
}

func parseRef(raw string) (refParts, error) {
	if !strings.HasPrefix(raw, "op://") {
		return refParts{}, fmt.Errorf("reference must start with op://")
	}
	without := strings.TrimPrefix(raw, "op://")
	query := ""
	if i := strings.Index(without, "?"); i >= 0 {
		query = without[i+1:]
		without = without[:i]
	}
	parts := strings.Split(without, "/")
	if len(parts) < 3 {
		return refParts{}, fmt.Errorf("reference must be op://vault/item/field or op://vault/item/section/field")
	}
	for i := range parts {
		decoded, err := url.PathUnescape(parts[i])
		if err == nil {
			parts[i] = decoded
		}
	}
	r := refParts{Raw: raw, Vault: parts[0], Item: parts[1], Field: parts[len(parts)-1], Query: query}
	if len(parts) > 3 {
		r.Section = strings.Join(parts[2:len(parts)-1], "/")
	}
	return r, nil
}

func refsInText(text string) []string {
	re := regexp.MustCompile(`op://[^\s"'<>)}\]]+`)
	seen := map[string]bool{}
	var refs []string
	for _, ref := range re.FindAllString(text, -1) {
		ref = strings.TrimRight(ref, ".,;:")
		if !seen[ref] {
			seen[ref] = true
			refs = append(refs, ref)
		}
	}
	return refs
}

func readTextInputs(files []string, inline string) (string, error) {
	var b strings.Builder
	if inline != "" {
		b.WriteString(inline)
		b.WriteByte('\n')
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		b.WriteString(string(data))
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func scoreItem(query string, item opItemSummary) int {
	q := strings.ToLower(query)
	score := 0
	fields := []string{item.Title, item.Category, item.Vault.Name, strings.Join(item.Tags, " ")}
	for _, u := range item.URLs {
		fields = append(fields, u.Href, u.Label)
	}
	hay := strings.ToLower(strings.Join(fields, " "))
	for _, tok := range strings.Fields(q) {
		if strings.Contains(hay, tok) {
			score += 10
		}
	}
	if strings.Contains(strings.ToLower(item.Title), q) {
		score += 50
	}
	return score
}

func resolveRefs(ctx context.Context, query, vault, categories string, limit int) ([]map[string]any, error) {
	items, err := listItems(ctx, vault, categories)
	if err != nil {
		return nil, err
	}
	type scored struct {
		item  opItemSummary
		score int
	}
	var scoredItems []scored
	for _, item := range items {
		score := scoreItem(query, item)
		if query == "" || score > 0 {
			scoredItems = append(scoredItems, scored{item: item, score: score})
		}
	}
	sort.Slice(scoredItems, func(i, j int) bool { return scoredItems[i].score > scoredItems[j].score })
	if limit <= 0 || limit > len(scoredItems) {
		limit = len(scoredItems)
	}
	var out []map[string]any
	for _, s := range scoredItems[:limit] {
		detail, err := getItem(ctx, s.item.ID, firstNonEmpty(s.item.Vault.ID, s.item.Vault.Name, vault))
		if err != nil {
			detail = opItemDetail{opItemSummary: s.item}
		}
		var fields []map[string]string
		for _, f := range detail.Fields {
			ref := f.Reference
			if ref == "" && detail.Vault.Name != "" && detail.Title != "" && f.Label != "" {
				ref = fmt.Sprintf("op://%s/%s/%s", detail.Vault.Name, detail.Title, f.Label)
			}
			fields = append(fields, map[string]string{"id": f.ID, "label": f.Label, "type": f.Type, "purpose": f.Purpose, "reference": ref})
		}
		out = append(out, map[string]any{
			"score":    s.score,
			"id":       firstNonEmpty(detail.ID, s.item.ID),
			"title":    firstNonEmpty(detail.Title, s.item.Title),
			"category": firstNonEmpty(detail.Category, s.item.Category),
			"vault":    firstNonEmpty(detail.Vault.Name, s.item.Vault.Name),
			"vault_id": firstNonEmpty(detail.Vault.ID, s.item.Vault.ID),
			"fields":   fields,
			"values":   "redacted",
		})
	}
	return out, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func hasTagPrefix(tags []string, prefixes ...string) bool {
	for _, tag := range tags {
		lower := strings.ToLower(tag)
		for _, p := range prefixes {
			if strings.HasPrefix(lower, p) {
				return true
			}
		}
	}
	return false
}

func riskCategory(cat string) string {
	c := strings.ToLower(strings.ReplaceAll(cat, " ", "_"))
	if strings.Contains(c, "credit") {
		return "card"
	}
	if strings.Contains(c, "document") {
		return "document"
	}
	if strings.Contains(c, "ssh") {
		return "ssh"
	}
	if strings.Contains(c, "api") || strings.Contains(c, "password") || strings.Contains(c, "login") {
		return "secret"
	}
	return "metadata"
}

func newOpPromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "op",
		Short:       "Show op installation and authentication status without reading secret values",
		Annotations: map[string]string{"pp:endpoint": "op.status", "pp:method": "GET", "pp:path": "/status", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			status := map[string]any{
				"auth_mode":             authMode(),
				"connect_env_conflicts": connectEnvConflicts(),
				"authenticated":         false,
				"verify_noop":           cliutil.IsVerifyEnv(),
			}
			if path, err := exec.LookPath("op"); err == nil {
				status["op_path"] = path
			} else {
				status["op_missing"] = true
			}
			if cliutil.IsVerifyEnv() {
				status["authenticated"] = true
				status["op_version"] = "verify"
				return flags.printJSON(cmd, status)
			}
			if out, _, err := newOpRunner().command(cmd.Context(), "--version"); err == nil {
				status["op_version"] = strings.TrimSpace(string(out))
			}
			if len(connectEnvConflicts()) == 0 {
				if _, _, err := newOpRunner().command(cmd.Context(), "user", "get", "--me", "--format", "json"); err == nil {
					status["authenticated"] = true
				} else {
					status["auth_error"] = err.Error()
				}
			} else {
				status["warning"] = "OP_CONNECT_HOST/OP_CONNECT_TOKEN take precedence over OP_SERVICE_ACCOUNT_TOKEN; clear them for this non-Connect CLI"
			}
			return flags.printJSON(cmd, status)
		},
	}
	return cmd
}

func newNovelSecretsResolveCmd(flags *rootFlags) *cobra.Command {
	var query, vault, category string
	var limit int
	cmd := &cobra.Command{
		Use:         "resolve",
		Short:       "Resolve fuzzy requests to exact op:// references without values",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" {
				return usageErr(fmt.Errorf("--query is required"))
			}
			matches, err := resolveRefs(cmd.Context(), query, vault, category, limit)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"query": query, "matches": matches, "values": "redacted"})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Fuzzy request to resolve")
	cmd.Flags().StringVar(&vault, "vault", "", "Vault name or ID to search")
	cmd.Flags().StringVar(&category, "category", "", "1Password item category filter")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum candidate items")
	return cmd
}

func newNovelSecretsReadCmd(flags *rootFlags) *cobra.Command {
	var reveal bool
	cmd := &cobra.Command{
		Use:         "read <op://vault/item/field>",
		Short:       "Read one exact op:// reference after an explicit reveal gate",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseRef(args[0])
			if err != nil {
				return usageErr(err)
			}
			plan := map[string]any{"ref": ref, "policy": policyDecision(ref), "would_reveal": reveal}
			if flags.dryRun || !reveal || cliutil.IsVerifyEnv() {
				plan["value"] = "redacted"
				plan["hint"] = "pass --reveal to print the value"
				return flags.printJSON(cmd, plan)
			}
			if deny, reason := denyRef(ref); deny {
				return fmt.Errorf("policy denied reveal: %s", reason)
			}
			out, _, err := newOpRunner().command(cmd.Context(), "read", args[0])
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(out)
			return err
		},
	}
	cmd.Flags().BoolVar(&reveal, "reveal", false, "Print the secret value after policy checks")
	return cmd
}

func newNovelSecretsExplainCmd(flags *rootFlags) *cobra.Command {
	var query, vault string
	cmd := &cobra.Command{
		Use:         "explain",
		Short:       "Explain why an item/field was selected without values",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			matches, err := resolveRefs(cmd.Context(), query, vault, "", 3)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"query": query, "selection_factors": []string{"title/category/tag match", "vault scope", "field reference availability"}, "candidates": matches})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Fuzzy request to explain")
	cmd.Flags().StringVar(&vault, "vault", "", "Vault name or ID to search")
	return cmd
}

func newNovelSecretsPreflightCmd(flags *rootFlags) *cobra.Command {
	var task string
	var files []string
	cmd := &cobra.Command{
		Use:         "preflight",
		Short:       "Check whether a task needs secrets, documents, cards, or writes",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := readTextInputs(files, strings.Join(append([]string{task}, args...), " "))
			if err != nil {
				return err
			}
			lower := strings.ToLower(text)
			refs := refsInText(text)
			return flags.printJSON(cmd, map[string]any{
				"refs":               refs,
				"requires_secrets":   len(refs) > 0 || strings.Contains(lower, "password") || strings.Contains(lower, "token") || strings.Contains(lower, "op run") || strings.Contains(lower, "--env-file"),
				"requires_cards":     strings.Contains(lower, "card") || strings.Contains(lower, "cvv"),
				"requires_documents": strings.Contains(lower, "document") || strings.Contains(lower, ".pem") || strings.Contains(lower, "certificate"),
				"requires_write":     regexp.MustCompile(`\b(create|edit|delete|share|inject|write|run)\b`).MatchString(lower),
			})
		},
	}
	cmd.Flags().StringVar(&task, "task", "", "Planned task text")
	cmd.Flags().StringSliceVar(&files, "file", nil, "Files to inspect")
	return cmd
}

func newNovelEnvPlanCmd(flags *rootFlags) *cobra.Command {
	var files []string
	var vault string
	cmd := &cobra.Command{
		Use:         "plan [KEY=VALUE...]",
		Short:       "Map env/config variables to safe op:// references",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := readTextInputs(files, strings.Join(args, " "))
			if err != nil {
				return err
			}
			var vars []map[string]any
			re := regexp.MustCompile(`(?m)^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*("?[^"\n]*"?)`)
			for _, m := range re.FindAllStringSubmatch(text, -1) {
				value := strings.Trim(m[2], `"`)
				entry := map[string]any{"name": m[1], "has_value": value != "", "refs": refsInText(value)}
				if value == "" || len(entry["refs"].([]string)) == 0 {
					entry["suggested_query"] = strings.ToLower(strings.ReplaceAll(m[1], "_", " "))
					entry["vault"] = vault
				}
				vars = append(vars, entry)
			}
			return flags.printJSON(cmd, map[string]any{"files": files, "variables": vars, "values": "redacted"})
		},
	}
	cmd.Flags().StringSliceVar(&files, "file", nil, "Env/config/shell files to inspect")
	cmd.Flags().StringVar(&vault, "vault", "", "Vault to prefer in suggested references")
	return cmd
}

func newNovelEnvInjectCmd(flags *rootFlags) *cobra.Command {
	var inFile, outFile string
	var write bool
	cmd := &cobra.Command{
		Use:   "inject",
		Short: "Plan or run op inject with an explicit --write gate",
		RunE: func(cmd *cobra.Command, args []string) error {
			if inFile == "" {
				return usageErr(fmt.Errorf("--in-file is required"))
			}
			data, err := os.ReadFile(inFile)
			if err != nil {
				return err
			}
			plan := map[string]any{"in_file": inFile, "out_file": outFile, "refs": refsInText(string(data)), "will_write": write, "values": "redacted"}
			if flags.dryRun || !write || cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, plan)
			}
			if outFile == "" {
				return usageErr(fmt.Errorf("--out-file is required with --write"))
			}
			_, _, err = newOpRunner().command(cmd.Context(), "inject", "-i", inFile, "-o", outFile)
			if err != nil {
				return err
			}
			plan["written"] = outFile
			return flags.printJSON(cmd, plan)
		},
	}
	cmd.Flags().StringVar(&inFile, "in-file", "", "Template file containing op:// references")
	cmd.Flags().StringVar(&outFile, "out-file", "", "Output file for injected content")
	cmd.Flags().BoolVar(&write, "write", false, "Actually run op inject and write --out-file")
	return cmd
}

func newNovelItemsClassifyCmd(flags *rootFlags) *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:         "classify",
		Short:       "Find secure notes that likely belong in stronger categories",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runItemAudit(cmd, flags, vault, classifyFindings)
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "Vault name or ID to inspect")
	return cmd
}

func newNovelItemsDuplicatesCmd(flags *rootFlags) *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:         "duplicates",
		Short:       "Detect duplicate titles and likely ambiguous credentials",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := listItems(cmd.Context(), vault, "")
			if err != nil {
				return err
			}
			byTitle := map[string][]opItemSummary{}
			for _, item := range items {
				byTitle[strings.ToLower(item.Title)] = append(byTitle[strings.ToLower(item.Title)], item)
			}
			var findings []finding
			for _, group := range byTitle {
				if len(group) > 1 {
					for _, item := range group {
						findings = append(findings, finding{Severity: "medium", Kind: "duplicate_title", ItemID: item.ID, Title: item.Title, Vault: item.Vault.Name, Category: item.Category, Reason: "same item title appears multiple times"})
					}
				}
			}
			return flags.printJSON(cmd, map[string]any{"findings": findings})
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "Vault name or ID to inspect")
	return cmd
}

func newNovelItemsOwnershipCmd(flags *rootFlags) *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:         "ownership",
		Short:       "Flag items missing owner, purpose, rotation, or environment tags",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runItemAudit(cmd, flags, vault, ownershipFindings)
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "Vault name or ID to inspect")
	return cmd
}

func runItemAudit(cmd *cobra.Command, flags *rootFlags, vault string, fn func([]opItemSummary) []finding) error {
	items, err := listItems(cmd.Context(), vault, "")
	if err != nil {
		return err
	}
	return flags.printJSON(cmd, map[string]any{"findings": fn(items)})
}

func itemAuditCmd(use, short string, flags *rootFlags, fn func([]opItemSummary) []finding) *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:         use,
		Short:       short,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runItemAudit(cmd, flags, vault, fn)
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "Vault name or ID to inspect")
	return cmd
}

func classifyFindings(items []opItemSummary) []finding {
	var out []finding
	for _, item := range items {
		hay := strings.ToLower(item.Title + " " + strings.Join(item.Tags, " "))
		if strings.EqualFold(item.Category, "Secure Note") {
			switch {
			case regexp.MustCompile(`api|token|secret|key`).MatchString(hay):
				out = append(out, finding{Severity: "medium", Kind: "misclassified_api_credential", ItemID: item.ID, Title: item.Title, Vault: item.Vault.Name, Category: item.Category, Reason: "secure note title/tags look like API credential material"})
			case regexp.MustCompile(`card|cvv|visa|amex|mastercard`).MatchString(hay):
				out = append(out, finding{Severity: "high", Kind: "misclassified_card", ItemID: item.ID, Title: item.Title, Vault: item.Vault.Name, Category: item.Category, Reason: "secure note looks like payment-card material"})
			case regexp.MustCompile(`ssh|private key|pem`).MatchString(hay):
				out = append(out, finding{Severity: "high", Kind: "misclassified_ssh_key", ItemID: item.ID, Title: item.Title, Vault: item.Vault.Name, Category: item.Category, Reason: "secure note looks like SSH or private-key material"})
			}
		}
	}
	return out
}

func ownershipFindings(items []opItemSummary) []finding {
	var out []finding
	for _, item := range items {
		if riskCategory(item.Category) == "metadata" {
			continue
		}
		if !hasTagPrefix(item.Tags, "owner:") {
			out = append(out, finding{Severity: "medium", Kind: "missing_owner", ItemID: item.ID, Title: item.Title, Vault: item.Vault.Name, Category: item.Category, Reason: "missing owner: tag"})
		}
		if !hasTagPrefix(item.Tags, "purpose:") {
			out = append(out, finding{Severity: "low", Kind: "missing_purpose", ItemID: item.ID, Title: item.Title, Vault: item.Vault.Name, Category: item.Category, Reason: "missing purpose: tag"})
		}
		if !hasTagPrefix(item.Tags, "env:", "environment:") {
			out = append(out, finding{Severity: "low", Kind: "missing_environment", ItemID: item.ID, Title: item.Title, Vault: item.Vault.Name, Category: item.Category, Reason: "missing env: tag"})
		}
	}
	return out
}

func newNovelCardsAuditCmd(flags *rootFlags) *cobra.Command {
	return itemAuditCmd("audit", "Audit card metadata without values", flags, func(items []opItemSummary) []finding {
		var out []finding
		for _, item := range items {
			hay := strings.ToLower(item.Title + " " + strings.Join(item.Tags, " "))
			if strings.Contains(strings.ToLower(item.Category), "credit") {
				if !hasTagPrefix(item.Tags, "owner:") {
					out = append(out, finding{Severity: "high", Kind: "card_missing_owner", ItemID: item.ID, Title: item.Title, Vault: item.Vault.Name, Category: item.Category, Reason: "credit card item missing owner tag"})
				}
				continue
			}
			if regexp.MustCompile(`card|cvv|visa|amex|mastercard`).MatchString(hay) {
				out = append(out, finding{Severity: "high", Kind: "card_misfiled", ItemID: item.ID, Title: item.Title, Vault: item.Vault.Name, Category: item.Category, Reason: "non-card item looks like card material"})
			}
		}
		return out
	})
}

func newNovelCardsResolveCmd(flags *rootFlags) *cobra.Command {
	var query, vault string
	cmd := &cobra.Command{
		Use:         "resolve",
		Short:       "Resolve card items to references without values",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			matches, err := resolveRefs(cmd.Context(), query, vault, "Credit Card", 5)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"matches": matches, "values": "redacted", "policy": "card values are never printed by resolve"})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Card search text")
	cmd.Flags().StringVar(&vault, "vault", "", "Vault name or ID")
	return cmd
}

func newNovelDocumentsInventoryCmd(flags *rootFlags) *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:         "inventory",
		Short:       "List document metadata without contents",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			docs, err := listDocuments(cmd.Context(), vault)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"documents": docs, "contents": "not_downloaded"})
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "Vault name or ID")
	return cmd
}

func newNovelDocumentsAuditCmd(flags *rootFlags) *cobra.Command {
	var vault string
	var maxMB int
	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "Audit document names, sizes, and placement without contents",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			docs, err := listDocuments(cmd.Context(), vault)
			if err != nil {
				return err
			}
			var findings []finding
			for _, doc := range docs {
				lower := strings.ToLower(doc.Title)
				if regexp.MustCompile(`private|secret|token|key|cert|\.pem|\.p12|\.pfx`).MatchString(lower) {
					findings = append(findings, finding{Severity: "high", Kind: "sensitive_filename", ItemID: doc.ID, Title: doc.Title, Vault: doc.Vault.Name, Category: "Document", Reason: "document filename looks secret-bearing"})
				}
				if maxMB > 0 && doc.Size > int64(maxMB)*1024*1024 {
					findings = append(findings, finding{Severity: "medium", Kind: "oversized_document", ItemID: doc.ID, Title: doc.Title, Vault: doc.Vault.Name, Category: "Document", Reason: "document exceeds configured size threshold"})
				}
			}
			return flags.printJSON(cmd, map[string]any{"findings": findings, "contents": "not_downloaded"})
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "Vault name or ID")
	cmd.Flags().IntVar(&maxMB, "max-mb", 50, "Flag documents larger than this size")
	return cmd
}

func newNovelSharePreflightCmd(flags *rootFlags) *cobra.Command {
	var ref, recipient, expires string
	cmd := &cobra.Command{
		Use:   "preflight",
		Short: "Plan item sharing without creating a share link",
		RunE: func(cmd *cobra.Command, args []string) error {
			parts, err := parseRef(ref)
			if err != nil {
				return usageErr(err)
			}
			return flags.printJSON(cmd, map[string]any{"ref": parts, "recipient": recipient, "expires_in": expires, "will_share": false, "required_permission": "share_items", "risk": policyDecision(parts)})
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "Exact op:// reference to the item/field considered for sharing")
	cmd.Flags().StringVar(&recipient, "recipient", "", "Recipient email or domain")
	cmd.Flags().StringVar(&expires, "expires-in", "7d", "Planned expiry")
	_ = cmd.MarkFlagRequired("ref")
	return cmd
}

func newNovelShareAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "Report current support for inspecting existing share links",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return flags.printJSON(cmd, map[string]any{
				"supported": false,
				"reason":    "op and the current public SDK document share-link creation and policy validation, but do not expose a reliable existing-share-link inventory command for this CLI to inspect without creating or fetching links",
				"safe_next": "use share preflight before creating any new share",
			})
		},
	}
	return cmd
}

func newNovelPolicyCheckCmd(flags *rootFlags) *cobra.Command {
	var ref string
	var requireExact bool
	cmd := &cobra.Command{
		Use:         "check",
		Short:       "Check agent policy before reveal, inject, run, or share",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var parsed refParts
			var err error
			if ref != "" {
				parsed, err = parseRef(ref)
				if err != nil {
					return usageErr(err)
				}
			} else if requireExact {
				return usageErr(fmt.Errorf("--ref is required with --require-exact"))
			}
			deny, reason := denyRef(parsed)
			return flags.printJSON(cmd, map[string]any{"ref": parsed, "allowed": !deny, "reason": firstNonEmpty(reason, "policy passed"), "require_exact": requireExact})
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "Exact op:// reference to evaluate")
	cmd.Flags().BoolVar(&requireExact, "require-exact", false, "Require an exact op:// reference")
	return cmd
}

func policyDecision(ref refParts) string {
	if deny, reason := denyRef(ref); deny {
		return "deny: " + reason
	}
	return "allow metadata; reveal requires explicit command gate"
}

func denyRef(ref refParts) (bool, string) {
	if hasProductionComponent(ref) {
		return true, "production values require a human-approved workflow"
	}
	lower := strings.ToLower(strings.Join(refComponents(ref), " "))
	if regexp.MustCompile(`\b(credit|card|cvv|cvc|csc|pan|payment|payments|bank|security[_ -]?code|expiry?|expiration)\b`).MatchString(lower) {
		return true, "payment-card values are blocked"
	}
	return false, ""
}

func hasProductionComponent(ref refParts) bool {
	for _, part := range refComponents(ref) {
		if isProductionComponent(part) {
			return true
		}
	}
	return false
}

func isProductionComponent(part string) bool {
	for _, token := range regexp.MustCompile(`[^a-z0-9]+`).Split(strings.ToLower(part), -1) {
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "production") {
			return true
		}
		if strings.HasPrefix(token, "prod") && !strings.HasPrefix(token, "product") && !strings.HasPrefix(token, "producer") {
			return true
		}
	}
	return false
}

func refComponents(ref refParts) []string {
	return []string{ref.Vault, ref.Item, ref.Section, ref.Field}
}

func newNovelAccessScopeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "scope",
		Short:       "Summarize accessible vaults and item categories without values",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var vaults []opVaultSummary
			if err := newOpRunner().json(cmd.Context(), []string{"vault", "list"}, &vaults); err != nil {
				return err
			}
			summary := map[string]any{"auth_mode": authMode(), "connect_env_conflicts": connectEnvConflicts(), "vault_count": len(vaults), "vaults": []any{}}
			var vaultOut []map[string]any
			for _, vault := range vaults {
				items, _ := listItems(cmd.Context(), firstNonEmpty(vault.ID, vault.Name), "")
				counts := map[string]int{}
				for _, item := range items {
					counts[item.Category]++
				}
				vaultOut = append(vaultOut, map[string]any{"id": vault.ID, "name": vault.Name, "item_count": len(items), "categories": counts})
			}
			summary["vaults"] = vaultOut
			return flags.printJSON(cmd, summary)
		},
	}
	return cmd
}

func newNovelRateLimitStatusCmd(flags *rootFlags) *cobra.Command {
	var serviceAccount string
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show service-account rate-limit usage",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var data any
			opArgs := []string{"service-account", "ratelimit"}
			if serviceAccount != "" {
				opArgs = append(opArgs, serviceAccount)
			}
			if err := newOpRunner().json(cmd.Context(), opArgs, &data); err != nil {
				if os.Getenv("OP_SERVICE_ACCOUNT_TOKEN") == "" && serviceAccount == "" {
					return flags.printJSON(cmd, map[string]any{
						"supported": false,
						"auth_mode": authMode(),
						"reason":    "op service-account ratelimit needs OP_SERVICE_ACCOUNT_TOKEN or an explicit --service-account when using desktop/session auth",
					})
				}
				return err
			}
			return flags.printJSON(cmd, map[string]any{"rate_limit": data, "auth_mode": authMode()})
		},
	}
	cmd.Flags().StringVar(&serviceAccount, "service-account", "", "Service account name or ID for desktop/session-authenticated op")
	return cmd
}

func newNovelAgentGrantPlanCmd(flags *rootFlags) *cobra.Command {
	var task, vault string
	cmd := &cobra.Command{
		Use:         "grant-plan",
		Short:       "Suggest minimum service-account permissions for a task",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			text := strings.ToLower(task + " " + strings.Join(args, " "))
			perms := []string{"read_items"}
			if regexp.MustCompile(`\b(write|create|edit|delete|archive|inject)\b`).MatchString(text) {
				perms = append(perms, "write_items")
			}
			if strings.Contains(text, "share") {
				perms = append(perms, "share_items")
			}
			return flags.printJSON(cmd, map[string]any{"vault": vault, "permissions": perms, "reason": "minimum inferred from task verbs; service-account vault permissions are immutable after creation"})
		},
	}
	cmd.Flags().StringVar(&task, "task", "", "Task to plan for")
	cmd.Flags().StringVar(&vault, "vault", "", "Vault name for grant syntax")
	return cmd
}

func newNovelRunPlanCmd(flags *rootFlags) *cobra.Command {
	var envFiles []string
	var command string
	cmd := &cobra.Command{
		Use:         "plan",
		Short:       "Inspect op run inputs before executing",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := readTextInputs(envFiles, command+" "+strings.Join(args, " "))
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"env_files": envFiles, "command": firstNonEmpty(command, strings.Join(args, " ")), "refs": refsInText(text), "will_execute": false})
		},
	}
	cmd.Flags().StringSliceVar(&envFiles, "env-file", nil, "Environment files op run would parse")
	cmd.Flags().StringVar(&command, "command", "", "Command string to inspect")
	return cmd
}

func newNovelAuditStaleCmd(flags *rootFlags) *cobra.Command {
	var vault string
	var days int
	cmd := &cobra.Command{
		Use:         "stale",
		Short:       "Flag old, untagged, duplicated, or likely unused items",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := listItems(cmd.Context(), vault, "")
			if err != nil {
				return err
			}
			cutoff := time.Now().AddDate(0, 0, -days)
			var findings []finding
			for _, item := range items {
				if len(item.Tags) == 0 {
					findings = append(findings, finding{Severity: "low", Kind: "untagged", ItemID: item.ID, Title: item.Title, Vault: item.Vault.Name, Category: item.Category, Reason: "item has no tags"})
				}
				if t, err := time.Parse(time.RFC3339, firstNonEmpty(item.UpdatedAt, item.CreatedAt)); err == nil && t.Before(cutoff) {
					findings = append(findings, finding{Severity: "medium", Kind: "stale", ItemID: item.ID, Title: item.Title, Vault: item.Vault.Name, Category: item.Category, Reason: "item metadata is older than threshold"})
				}
			}
			return flags.printJSON(cmd, map[string]any{"days": days, "findings": findings})
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "Vault name or ID")
	cmd.Flags().IntVar(&days, "days", 180, "Staleness threshold")
	return cmd
}

func newNovelAuditMisplacedCmd(flags *rootFlags) *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:         "misplaced",
		Short:       "Find API keys, cards, docs, or SSH material in wrong categories",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runItemAudit(cmd, flags, vault, classifyFindings)
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "Vault name or ID to inspect")
	return cmd
}

func formatRefForVar(vault, item, field string) string {
	return fmt.Sprintf("op://%s/%s/%s", vault, item, field)
}

func attachOpAgentExamples(root *cobra.Command) {
	examples := map[string]string{
		"access scope":        "  1password-pp-cli access scope --json",
		"agent grant-plan":    "  1password-pp-cli agent grant-plan --task \"read staging deploy token\" --json",
		"audit misplaced":     "  1password-pp-cli audit misplaced --json",
		"audit stale":         "  1password-pp-cli audit stale --days 180 --json",
		"cards audit":         "  1password-pp-cli cards audit --json",
		"cards resolve":       "  1password-pp-cli cards resolve --query \"card\" --json",
		"documents audit":     "  1password-pp-cli documents audit --json",
		"documents inventory": "  1password-pp-cli documents inventory --json",
		"env inject":          "  1password-pp-cli env inject --in-file README.md --out-file injected.env --json",
		"env plan":            "  1password-pp-cli env plan API_TOKEN= --json",
		"items classify":      "  1password-pp-cli items classify --json",
		"items duplicates":    "  1password-pp-cli items duplicates --json",
		"items ownership":     "  1password-pp-cli items ownership --json",
		"op":                  "  1password-pp-cli op --json",
		"policy check":        "  1password-pp-cli policy check --ref op://Production/API/token --require-exact --json",
		"rate-limit status":   "  1password-pp-cli rate-limit status --json\n  1password-pp-cli rate-limit status --service-account SERVICE_ACCOUNT_UUID --json",
		"run plan":            "  1password-pp-cli run plan --command \"npm test\" --json",
		"secrets explain":     "  1password-pp-cli secrets explain --query \"token\" --json",
		"secrets preflight":   "  1password-pp-cli secrets preflight --task \"deploy using op run --env-file .env\" --json",
		"secrets read":        "  1password-pp-cli secrets read op://Engineering/GitHub/token --dry-run --json",
		"secrets resolve":     "  1password-pp-cli secrets resolve --query \"token\" --json",
		"share audit":         "  1password-pp-cli share audit --json",
		"share preflight":     "  1password-pp-cli share preflight --ref op://Engineering/GitHub/token --recipient recipient --expires-in 1d --json",
	}
	var walk func(*cobra.Command, []string)
	walk = func(cmd *cobra.Command, parts []string) {
		if cmd.Use != "" {
			parts = append(parts, strings.Fields(cmd.Use)[0])
		}
		key := strings.Join(parts, " ")
		if example, ok := examples[key]; ok {
			cmd.Example = example
		}
		for _, child := range cmd.Commands() {
			walk(child, parts)
		}
	}
	for _, child := range root.Commands() {
		walk(child, nil)
	}
}
