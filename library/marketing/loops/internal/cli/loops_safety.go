package cli

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/loops/internal/client"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// pp:data-source live
// This hook wraps every generated API mutation, including future endpoints.
// The shared client supplies a second fail-closed guard for non-CLI callers.
func init() {
	registerNovelCommand(installLoopsSafety)
}

func installLoopsSafety(root *cobra.Command, flags *rootFlags) {
	var team string
	var execute, confirmSend, confirmPublish, confirmSuppression, confirmDestructive bool
	root.PersistentFlags().StringVar(&team, "team", "", "Exact Loops team name required for mutations")
	root.PersistentFlags().BoolVar(&execute, "execute", false, "Execute a mutation after the safe preview")
	root.PersistentFlags().BoolVar(&confirmSend, "confirm-send", false, "Explicitly approve one email or event send")
	root.PersistentFlags().BoolVar(&confirmPublish, "confirm-publish", false, "Explicitly approve campaign or transactional publication")
	root.PersistentFlags().BoolVar(&confirmSuppression, "confirm-suppression-removal", false, "Explicitly approve suppression removal")
	root.PersistentFlags().BoolVar(&confirmDestructive, "confirm-destructive", false, "Explicitly approve a delete")

	var walk func(*cobra.Command)
	walk = func(command *cobra.Command) {
		for _, child := range command.Commands() {
			walk(child)
		}
		method := strings.ToUpper(command.Annotations["pp:method"])
		if method == "GET" && strings.Contains(command.Annotations["pp:path"], "{") {
			// Spec example IDs are illustrative, not live account fixtures. Let
			// dogfood resolve an ID from a list or skip when none exists.
			delete(command.Annotations, "pp:happy-args")
		}
		if command.Annotations["pp:endpoint"] == "contacts.find" {
			command.Annotations["pp:happy-args"] = "--email=synthetic-person@example.test"
		}
		if command.Annotations["pp:endpoint"] == "contacts.get-suppression" {
			command.Annotations["pp:happy-args"] = "--email=synthetic-person@example.test"
			command.Annotations["pp:typed-exit-codes"] = "0,3" // An absent synthetic contact is expected.
		}
		if command.Example == "" && command.Annotations["pp:endpoint"] != "" {
			command.Example = loopsMissingExample(command.Annotations["pp:endpoint"])
		}
		if (command.Name() == "teach" || command.Name() == "teach-playbook" || command.CommandPath() == root.CommandPath()+" playbook amend") && command.RunE != nil {
			original := command.RunE
			command.RunE = func(cmd *cobra.Command, args []string) error {
				if noLearnActive(flags) && flags.asJSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"ok": true, "skipped": "no-learn", "dry_run": true, "action": cmd.CommandPath(),
					})
				}
				return original(cmd, args)
			}
		}
		bulkImport := command.Name() == "import" && command.Parent() == root
		if (method == "" && !bulkImport) || method == "GET" || command.RunE == nil {
			if command.Annotations["pp:path"] == "/v1/contacts/find" {
				original := command.RunE
				command.RunE = func(cmd *cobra.Command, args []string) error {
					if flags.agent && flags.selectFields == "" {
						flags.selectFields = "subscribed,optInStatus"
					}
					return original(cmd, args)
				}
			}
			return
		}
		original := command.RunE
		path := command.Annotations["pp:path"]
		if bulkImport {
			method, path = "POST", "/v1/{resource}"
		}
		command.Long += "\n\nThis command previews by default without an API call. Use --execute --team with the required confirmation flag to change Loops data. Previews omit argument values and private contact data."
		command.RunE = func(cmd *cobra.Command, args []string) error {
			if len(args) < strings.Count(cmd.Use, "<") {
				return errors.New("required resource argument is missing; run --help")
			}
			if command.Annotations["pp:requires-input"] == "true" && len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return errors.New("required input is missing; run --help")
			}
			actualPath := path
			if bulkImport {
				if len(args) != 1 {
					return errors.New("import requires one resource")
				}
				if args[0] == "events" || args[0] == "transactional" {
					return errors.New("bulk sends are unavailable; use one keyed send command per recipient")
				}
				resolved, err := resourceWritePath(args[0])
				if err != nil {
					return err
				}
				actualPath = resolved
			}
			effect, confirmation := loopsEffect(actualPath, method)
			provided := []string{}
			cmd.Flags().Visit(func(f *pflag.Flag) { provided = append(provided, f.Name) })
			sort.Strings(provided)
			view := map[string]any{
				"command": cmd.CommandPath(), "effect": effect, "method": method,
				"pathTemplate": actualPath, "argumentCount": len(args), "providedFlags": provided,
				"teamSelected": strings.TrimSpace(team) != "", "requires": confirmation,
				"privateValues": "redacted",
			}
			if !execute {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"ok": true, "executed": false, "dry_run": true,
					"action": cmd.CommandPath(), "preview": view,
				})
			}
			if flags.dryRun {
				return errors.New("use the default preview without --execute; generated dry runs may expose private input values")
			}
			if bulkImport {
				if localDryRun, _ := cmd.Flags().GetBool("dry-run"); localDryRun {
					return errors.New("use the default import preview without --execute --dry-run")
				}
			}
			if strings.TrimSpace(team) == "" {
				return errors.New("--team is required before any Loops mutation")
			}
			switch confirmation {
			case "--confirm-send":
				if !confirmSend {
					return errors.New("--confirm-send is required")
				}
			case "--confirm-publish":
				if !confirmPublish {
					return errors.New("--confirm-publish is required")
				}
			case "--confirm-suppression-removal":
				if !confirmSuppression {
					return errors.New("--confirm-suppression-removal is required")
				}
			case "--confirm-destructive":
				if !confirmDestructive {
					return errors.New("--confirm-destructive is required")
				}
			}
			if actualPath == "/v1/events/send" || actualPath == "/v1/transactional" {
				key, err := cmd.Flags().GetString("idempotency-key")
				if err != nil || strings.TrimSpace(key) == "" {
					return errors.New("--idempotency-key is required for event and transactional sends")
				}
			}
			if err := json.NewEncoder(cmd.ErrOrStderr()).Encode(map[string]any{"preview": view}); err != nil {
				return err
			}
			cmd.SetContext(client.WithLoopsMutationIntent(cmd.Context(), team))
			return original(cmd, args)
		}
	}
	walk(root)
}

func loopsMissingExample(endpoint string) string {
	examples := map[string]string{
		"components.create":                 "  loops-pp-cli components create --name Example --lmx '<div>Example</div>'",
		"preview.email-message":             "  loops-pp-cli email-messages preview email-message <emailMessageId> --emails synthetic-person@example.test",
		"event-patterns.get":                "  loops-pp-cli event-patterns get <eventPatternId> --json",
		"event-patterns.get-by-name":        "  loops-pp-cli event-patterns get-by-name <eventName> --json",
		"workflows.create":                  "  loops-pp-cli workflows create --name Example",
		"workflows.delete":                  "  loops-pp-cli workflows delete <workflowId> --expected-revision-id <revisionId>",
		"mailing-list.change-workflow":      "  loops-pp-cli workflows mailing-list change-workflow <workflowId> --expected-revision-id <revisionId> --mailing-list-id <mailingListId>",
		"nodes.create-workflow":             "  loops-pp-cli workflows nodes create-workflow <workflowId> --body-json '{}'",
		"nodes.delete-workflow":             "  loops-pp-cli workflows nodes delete-workflow <workflowId> <nodeId> --expected-revision-id <revisionId>",
		"nodes.delete-workflow-recursively": "  loops-pp-cli workflows nodes delete-workflow-recursively <workflowId> <nodeId> --expected-revision-id <revisionId>",
		"nodes.reroute-connection":          "  loops-pp-cli workflows nodes reroute-connection <workflowId> <nodeId> --expected-revision-id <revisionId> --new-target-node-id <targetNodeId>",
		"nodes.update-workflow":             "  loops-pp-cli workflows nodes update-workflow <workflowId> <nodeId> --expected-revision-id <revisionId> --payload '{}'",
	}
	return examples[endpoint]
}

func loopsEffect(path, method string) (string, string) {
	switch {
	case path == "/v1/events/send":
		return "sends_event_may_send_email", "--confirm-send"
	case path == "/v1/transactional" || strings.HasSuffix(path, "/preview") && strings.HasPrefix(path, "/v1/email-messages/"):
		return "sends_email", "--confirm-send"
	case path == "/v1/contacts/suppression":
		return "removes_suppression", "--confirm-suppression-removal"
	case method == "DELETE" || path == "/v1/contacts/delete":
		return "deletes_data", "--confirm-destructive"
	case path == "/v1/transactional-emails/{transactionalId}/publish" || path == "/v1/campaigns/{campaignId}" || path == "/v1/campaigns":
		return "may_publish_or_schedule_email", "--confirm-publish"
	default:
		return "changes_data", "--execute"
	}
}
