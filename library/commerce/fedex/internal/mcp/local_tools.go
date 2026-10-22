// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/mcp/cobratree"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const localCommandTimeout = 2 * time.Minute

type positionalKind int

const (
	noPositional positionalKind = iota
	optionalOne
	requiredOne
	many
)

// localCommand is one fixed companion-CLI command exposed over MCP. Only the
// listed command-local flags are accepted: no global flags (--config,
// --deliver, --yes, --data-source), no raw argument string, and no flag that
// takes a file path. The child process runs with FEDEX_PP_MCP_CHILD=1, which
// makes its HTTP client refuse every FedEx write.
type localCommand struct {
	name        string
	path        []string
	flags       []string
	positional  positionalKind
	argName     string
	argHelp     string
	readOnly    bool
	destructive bool
	openWorld   bool
}

var localCommands = []localCommand{
	{name: "rate_shop", path: []string{"rate", "shop"}, flags: []string{"from", "to", "from-country", "to-country", "weight", "units", "account", "services", "rank-by"}, readOnly: true, openWorld: true},
	{name: "address_validate", path: []string{"address", "validate"}, flags: []string{"street", "city", "state", "zip", "country", "cache"}, readOnly: true, openWorld: true},
	{name: "track_diff", path: []string{"track", "diff"}, flags: []string{"since", "limit"}, positional: many, argName: "tracking_numbers", argHelp: "Tracking numbers to check. Omit to poll recent shipments from the local archive.", readOnly: true, openWorld: true},
	{name: "archive", path: []string{"archive"}, flags: []string{"service", "account", "since", "limit"}, positional: optionalOne, argName: "query", argHelp: "Full-text search over recipient, reference, tracking number, and address.", readOnly: true},
	{name: "spend_report", path: []string{"spend", "report"}, flags: []string{"since", "by", "account"}, readOnly: true},
	{name: "manifest", path: []string{"manifest"}, flags: []string{"date", "account"}, readOnly: true},
	{name: "address_list", path: []string{"address", "list"}, readOnly: true},
	{name: "address_save", path: []string{"address", "save"}, flags: []string{"contact-name", "company", "phone", "email", "street", "street2", "city", "state", "zip", "country", "residential", "notes"}, positional: requiredOne, argName: "name", argHelp: "Name to save the recipient under."},
	{name: "address_delete", path: []string{"address", "delete"}, positional: requiredOne, argName: "name", argHelp: "Saved recipient name to delete.", destructive: true},
}

func registerLocalCommandTools(s *server.MCPServer) {
	root := cli.RootCmd()
	cliPath, lookupErr := cobratree.SiblingCLIPath()
	for _, command := range localCommands {
		lc := command
		cmd, _, err := root.Find(lc.path)
		if err != nil || cmd == nil || cmd.Name() != lc.path[len(lc.path)-1] {
			continue
		}
		options := []mcplib.ToolOption{
			mcplib.WithDescription(localDescription(cmd, lc)),
			mcplib.WithReadOnlyHintAnnotation(lc.readOnly),
			mcplib.WithDestructiveHintAnnotation(lc.destructive),
			mcplib.WithIdempotentHintAnnotation(lc.readOnly),
			mcplib.WithOpenWorldHintAnnotation(lc.openWorld),
		}
		for _, name := range lc.flags {
			if flag := cmd.LocalNonPersistentFlags().Lookup(name); flag != nil {
				options = append(options, localFlagOption(flag))
			}
		}
		switch lc.positional {
		case optionalOne:
			options = append(options, mcplib.WithString(lc.argName, mcplib.Description(lc.argHelp)))
		case requiredOne:
			options = append(options, mcplib.WithString(lc.argName, mcplib.Required(), mcplib.Description(lc.argHelp)))
		case many:
			options = append(options, mcplib.WithArray(lc.argName, mcplib.Description(lc.argHelp), mcplib.Items(map[string]any{"type": "string"})))
		}
		s.AddTool(mcplib.NewTool(lc.name, options...), makeLocalHandler(lc, cmd, cliPath, lookupErr))
	}
}

func localDescription(cmd *cobra.Command, lc localCommand) string {
	text := strings.TrimSpace(cmd.Short)
	if text == "" {
		text = "Run `fedex-pp-cli " + strings.Join(lc.path, " ") + "`."
	}
	switch {
	case lc.readOnly && lc.openWorld:
		text += " Read-only FedEx lookup."
	case lc.readOnly:
		text += " Reads the local shipment ledger only."
	default:
		text += " Changes the local address book only; never calls FedEx."
	}
	return text
}

func localFlagOption(flag *pflag.Flag) mcplib.ToolOption {
	usage := strings.TrimSpace(flag.Usage)
	if flag.DefValue != "" && flag.DefValue != "[]" {
		usage += " (default: " + flag.DefValue + ")"
	}
	switch flag.Value.Type() {
	case "bool":
		return mcplib.WithBoolean(flag.Name, mcplib.Description(usage))
	case "int", "int32", "int64", "float32", "float64":
		return mcplib.WithNumber(flag.Name, mcplib.Description(usage))
	default:
		return mcplib.WithString(flag.Name, mcplib.Description(usage))
	}
}

// localCommandArgs converts validated MCP arguments into a fixed argv. Every
// flag is passed as --name=value so a value can never be parsed as a flag.
func localCommandArgs(lc localCommand, cmd *cobra.Command, arguments map[string]any) ([]string, error) {
	allowed := map[string]bool{}
	for _, name := range lc.flags {
		allowed[name] = true
	}
	if lc.positional != noPositional {
		allowed[lc.argName] = true
	}
	keys := make([]string, 0, len(arguments))
	for key := range arguments {
		if !allowed[key] {
			return nil, fmt.Errorf("unsupported argument %q for %s", key, lc.name)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Explicit JSON + non-interactive flags rather than --agent, which also
	// turns on --compact and would strip fields from some command output.
	argv := append([]string{"--json", "--no-input", "--no-color"}, lc.path...)
	var positionals []string
	for _, key := range keys {
		value := arguments[key]
		if key == lc.argName && lc.positional != noPositional {
			values, err := positionalValues(lc, value)
			if err != nil {
				return nil, err
			}
			positionals = append(positionals, values...)
			continue
		}
		flag := cmd.LocalNonPersistentFlags().Lookup(key)
		if flag == nil {
			return nil, fmt.Errorf("unsupported argument %q for %s", key, lc.name)
		}
		text, err := flagValue(flag, value)
		if err != nil {
			return nil, err
		}
		argv = append(argv, "--"+key+"="+text)
	}
	if lc.positional == requiredOne && len(positionals) == 0 {
		return nil, fmt.Errorf("%s is required", lc.argName)
	}
	if len(positionals) > 0 {
		argv = append(argv, "--")
		argv = append(argv, positionals...)
	}
	return argv, nil
}

func positionalValues(lc localCommand, value any) ([]string, error) {
	var values []string
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) != "" {
			values = []string{typed}
		}
	case []any:
		if lc.positional != many {
			return nil, fmt.Errorf("%s must be a string", lc.argName)
		}
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s must contain only strings", lc.argName)
			}
			if strings.TrimSpace(text) != "" {
				values = append(values, text)
			}
		}
	default:
		return nil, fmt.Errorf("%s has an unsupported type", lc.argName)
	}
	if lc.positional != many && len(values) > 1 {
		return nil, fmt.Errorf("%s takes one value", lc.argName)
	}
	return values, nil
}

func flagValue(flag *pflag.Flag, value any) (string, error) {
	switch flag.Value.Type() {
	case "bool":
		b, ok := value.(bool)
		if !ok {
			return "", fmt.Errorf("%s must be a boolean", flag.Name)
		}
		return strconv.FormatBool(b), nil
	case "int", "int32", "int64":
		n, ok := value.(float64)
		if !ok || n != float64(int64(n)) {
			return "", fmt.Errorf("%s must be an integer", flag.Name)
		}
		return strconv.FormatInt(int64(n), 10), nil
	case "float32", "float64":
		n, ok := value.(float64)
		if !ok {
			return "", fmt.Errorf("%s must be a number", flag.Name)
		}
		return strconv.FormatFloat(n, 'f', -1, 64), nil
	default:
		text, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("%s must be a string", flag.Name)
		}
		return text, nil
	}
}

func makeLocalHandler(lc localCommand, cmd *cobra.Command, cliPath string, lookupErr error) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		if lookupErr != nil {
			return toolJSONError("companion_cli_missing", fmt.Sprintf("companion fedex-pp-cli binary not found (tried next to this server, FEDEX_CLI_PATH, and PATH): %v", lookupErr)), nil
		}
		argv, err := localCommandArgs(lc, cmd, request.GetArguments())
		if err != nil {
			return toolJSONError("invalid_request", err.Error()), nil
		}
		ctx, cancel := context.WithTimeout(ctx, localCommandTimeout)
		defer cancel()
		child := exec.CommandContext(ctx, cliPath, argv...)
		child.Env = append(os.Environ(), client.MCPChildEnv+"=1")
		out, err := child.CombinedOutput()
		if err != nil {
			return mcplib.NewToolResultError(string(out)), nil
		}
		return mcplib.NewToolResultText(string(out)), nil
	}
}

func localToolSummaries() []map[string]string {
	out := make([]map[string]string, 0, len(localCommands))
	for _, lc := range localCommands {
		out = append(out, map[string]string{"tool": lc.name, "command": strings.Join(lc.path, " ")})
	}
	return out
}

func toolCount() int {
	return len(endpointTools) + len(localCommands) + 2 // ship_bulk + context
}
