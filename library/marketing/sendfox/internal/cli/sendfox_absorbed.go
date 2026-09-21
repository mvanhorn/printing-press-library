// SendFox compatibility workflows, reconciled against API v1.4.0.
// pp:novel-static-reference: API gap policy comes from the public contract, not account data.
package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/contract"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/evidence"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
	"time"
)

var sendfoxGaps = []string{"webhook management and event delivery", "purchases/orders", "reusable snippets/templates", "public posts/feed controls", "account growth/aggregate analytics parity", "richer behavioral filters", "automation enrollment/progress inspection and sequence-progress transfer"}

func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		addNovelCommandIfAbsent(root, newCapabilitiesCmd(f))
		contacts, _, _ := root.Find([]string{"contacts"})
		for _, name := range []string{"audit-csv", "reconcile-csv", "import-csv"} {
			contacts.AddCommand(newSendfoxCSV(f, name))
		}
		contacts.AddCommand(newContactsOnboardCmd(f))
		forms, _, _ := root.Find([]string{"forms"})
		forms.AddCommand(newSendfoxFormPlan(f))
	})
}
func newCapabilitiesCmd(f *rootFlags) *cobra.Command {
	var resource string
	cmd := &cobra.Command{Use: "capabilities", Short: "Inspect the complete documented API and explicit parity gaps", Example: "  sendfox-pp-cli capabilities --resource campaigns --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"}, RunE: func(cmd *cobra.Command, args []string) error {
		ops := []contract.Operation{}
		for _, op := range contract.Operations {
			if resource == "" || strings.Split(strings.Trim(op.Path, "/"), "/")[0] == resource {
				ops = append(ops, op)
			}
		}
		if resource != "" && len(ops) == 0 {
			return usageErr(fmt.Errorf("unknown --resource %q; use contacts, lists, campaigns, forms, automations, domains, contact-tags or contact-fields", resource))
		}
		return f.printJSON(cmd, map[string]any{"api_version": "1.4.0", "operation_count": len(ops), "operations": ops, "api_gaps": sendfoxGaps, "rate_limit_per_minute": 60, "live_verified": false, "write_policy": "preview with --dry-run; --yes to apply; --approve-sensitive also required for delivery, activation, forms/domains and destruction"})
	}}
	cmd.Flags().StringVar(&resource, "resource", "", "Limit discovery to one API resource family")
	return cmd
}

type sendfoxCSVOptions struct {
	file, input, lists string
	execute            bool
}

func configureSendfoxCSV(cmd *cobra.Command, f *rootFlags, name string, o *sendfoxCSVOptions) *cobra.Command {
	cmd.Example = "  sendfox-pp-cli contacts " + name + " --file examples/contacts.csv --input examples/snapshot.json --agent"
	cmd.Annotations = map[string]string{"pp:data-source": "local", "pp:happy-args": "--file=examples/contacts.csv;--input=examples/snapshot.json"}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		file, input, lists, execute := o.file, o.input, o.lists, o.execute
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "contacts "+name)
		}
		if file == "" {
			return usageErr(fmt.Errorf("--file is required; CSV headers: email, first_name, last_name, status, unsubscribed_at"))
		}
		rows, err := evidence.ReadCSV(file)
		if err != nil {
			return usageErr(err)
		}
		s := evidence.Snapshot{SchemaVersion: 1, Resources: map[string]evidence.Resource{}}
		if input != "" {
			s, err = evidence.Load(input)
			if err != nil {
				return usageErr(err)
			}
		}
		r := evidence.ReconcileCSV(rows, s)
		if name != "audit-csv" {
			r.Require(s, time.Now(), f.maxAge, "contacts", "unsubscribed")
		}
		r.Finish()
		ids := []int{}
		if lists != "" {
			for _, value := range strings.Split(lists, ",") {
				id, e := strconv.Atoi(strings.TrimSpace(value))
				if e != nil || id < 1 {
					return usageErr(fmt.Errorf("--lists must contain positive integer IDs"))
				}
				ids = append(ids, id)
			}
		}
		creates := r.Data["to_create"].([]evidence.Row)
		for _, row := range creates {
			if len(ids) > 0 {
				row["lists"] = ids
			}
		}
		r.ProposedActions = append(r.ProposedActions, evidence.Row{"method": "POST", "path": "/contacts/batch", "contacts": creates, "execute": false, "max_batch": 1000})
		if !execute || name != "import-csv" {
			return printSendfoxEvidence(cmd, f, r)
		}
		if !f.yes {
			return usageErr(fmt.Errorf("--execute requires --yes after reviewing the plan"))
		}
		if !r.Ready {
			return usageErr(fmt.Errorf("import blocked by incomplete/stale contact or suppression evidence; review without --execute"))
		}
		c, err := f.newClient()
		if err != nil {
			return err
		}
		outcomes := []evidence.Row{}
		for start := 0; start < len(creates); start += 1000 {
			end := start + 1000
			if end > len(creates) {
				end = len(creates)
			}
			raw, code, e := c.Post(cmd.Context(), "/contacts/batch", map[string]any{"contacts": creates[start:end]})
			var result any
			if len(raw) > 0 {
				if parseErr := json.Unmarshal(raw, &result); parseErr != nil && e == nil {
					e = fmt.Errorf("batch response could not be decoded; reconcile before retry: %w", parseErr)
				}
			}
			if e == nil {
				e = validateSendfoxBatchAcknowledgement(result, end-start)
			}
			outcomes = append(outcomes, evidence.Row{"offset": start, "submitted": end - start, "status": code, "response": result, "ambiguous": e != nil})
			r.Data["outcomes"] = outcomes
			if e != nil {
				r.Blockers = append(r.Blockers, "Stopped after request error; reconcile server state before retrying. No automatic replay.")
				r.Finish()
				if printErr := printSendfoxEvidence(cmd, f, r); printErr != nil {
					return printErr
				}
				return e
			}
		}
		r.Data["executed"] = true
		return printSendfoxEvidence(cmd, f, r)
	}
	if name != "import-csv" {
		cmd.Annotations["mcp:read-only"] = "true"
	}
	return cmd
}
func validateSendfoxBatchAcknowledgement(result any, submitted int) error {
	m, ok := result.(map[string]any)
	created, cOK := m["created"].(float64)
	updated, uOK := m["updated"].(float64)
	if !ok || !cOK || !uOK || created < 0 || updated < 0 ||
		created != float64(int(created)) || updated != float64(int(updated)) ||
		created+updated != float64(submitted) {
		return fmt.Errorf("batch acknowledgement counts do not match submitted contacts; reconcile before retry")
	}
	return nil
}
func newContactsOnboardCmd(f *rootFlags) *cobra.Command {
	var email, first, last, lists string
	var execute bool
	cmd := &cobra.Command{Use: "onboard", Short: "Preview contact onboarding; explicit execution uses the documented create endpoint", Example: "  sendfox-pp-cli contacts onboard --email reader@example.com --lists 12 --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "contacts onboard")
		}
		if !evidence.ValidEmail(email) {
			return usageErr(fmt.Errorf("--email must be a bare valid email address"))
		}
		body := map[string]any{"email": email, "first_name": first, "last_name": last}
		ids := []int{}
		if lists != "" {
			for _, v := range strings.Split(lists, ",") {
				id, e := strconv.Atoi(v)
				if e != nil || id < 1 {
					return usageErr(fmt.Errorf("--lists expects positive integers"))
				}
				ids = append(ids, id)
			}
			body["lists"] = ids
		}
		if !execute {
			return f.printJSON(cmd, map[string]any{"execute": false, "method": "POST", "path": "/contacts", "body": body, "note": "Existing contact/list semantics and suppression must be reviewed before execution."})
		}
		if !f.yes {
			return usageErr(fmt.Errorf("--execute requires --yes"))
		}
		c, e := f.newClient()
		if e != nil {
			return e
		}
		raw, _, e := c.Post(cmd.Context(), "/contacts", body)
		if e != nil {
			return e
		}
		return f.printJSON(cmd, json.RawMessage(raw))
	}}
	cmd.Flags().StringVar(&email, "email", "", "Bare subscriber email, for example reader@example.com")
	cmd.Flags().StringVar(&first, "first-name", "", "Subscriber given name to include in the request")
	cmd.Flags().StringVar(&last, "last-name", "", "Subscriber family name to include in the request")
	cmd.Flags().StringVar(&lists, "lists", "", "Comma-separated existing list IDs to attach")
	cmd.Flags().BoolVar(&execute, "execute", false, "Create or update the contact after review; requires --yes")
	return cmd
}
func newSendfoxFormPlan(f *rootFlags) *cobra.Command {
	var title, lists, redirect string
	var consent bool
	cmd := &cobra.Command{Use: "generate", Short: "Generate a local form creation payload for review", Example: "  sendfox-pp-cli forms generate --title Newsletter --lists 12 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "forms generate")
		}
		if title == "" || lists == "" {
			return usageErr(fmt.Errorf("--title and --lists are required"))
		}
		ids := []int{}
		for _, v := range strings.Split(lists, ",") {
			id, e := strconv.Atoi(v)
			if e != nil || id < 1 {
				return usageErr(fmt.Errorf("--lists expects positive integers"))
			}
			ids = append(ids, id)
		}
		body := map[string]any{"title": title, "lists": ids, "gdpr_required": consent}
		if redirect != "" {
			body["redirect_url"] = redirect
		}
		op, _ := contract.Match("POST", "/forms")
		if err := contract.Validate(op.BodySchema, body); err != nil {
			return usageErr(err)
		}
		return f.printJSON(cmd, map[string]any{"method": "POST", "path": "/forms", "body": body, "execute": false, "approval_required": true, "note": "Use forms create with explicit approvals after review. Never put the API token in browser HTML."})
	}}
	cmd.Flags().StringVar(&title, "title", "", "Internal title for the proposed signup form")
	cmd.Flags().StringVar(&lists, "lists", "", "Comma-separated list IDs receiving form signups")
	cmd.Flags().StringVar(&redirect, "redirect-url", "", "Absolute URL shown after a successful subscription")
	cmd.Flags().BoolVar(&consent, "gdpr-required", true, "Require an explicit consent checkbox in the proposed form")
	return cmd
}

func newSendfoxCSV(f *rootFlags, name string) *cobra.Command {
	switch name {
	case "audit-csv":
		return newContactsAuditCSVCmd(f)
	case "reconcile-csv":
		return newContactsReconcileCSVCmd(f)
	default:
		return newContactsImportCSVCmd(f)
	}
}
