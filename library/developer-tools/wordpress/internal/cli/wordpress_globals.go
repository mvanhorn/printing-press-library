package cli

import (
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/client"
	"github.com/spf13/cobra"
)

type wordpressGlobalFlagState struct {
	fields    string
	fieldsSet bool
	embed     bool
	embedSet  bool
}

type wordpressFieldsValue struct{ state *wordpressGlobalFlagState }

func (v *wordpressFieldsValue) String() string {
	if v == nil || v.state == nil {
		return ""
	}
	return v.state.fields
}

func (v *wordpressFieldsValue) Set(value string) error {
	v.state.fields = value
	v.state.fieldsSet = true
	applyWordPressGlobalFlagState(v.state)
	return nil
}

func (*wordpressFieldsValue) Type() string { return "string" }

type wordpressEmbedValue struct{ state *wordpressGlobalFlagState }

func (v *wordpressEmbedValue) String() string {
	if v == nil || v.state == nil {
		return "false"
	}
	return strconv.FormatBool(v.state.embed)
}

func (v *wordpressEmbedValue) Set(value string) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	v.state.embed = parsed
	v.state.embedSet = true
	applyWordPressGlobalFlagState(v.state)
	return nil
}

func (*wordpressEmbedValue) Type() string     { return "bool" }
func (*wordpressEmbedValue) IsBoolFlag() bool { return true }

func registerWordPressGlobalFlags(rootCmd *cobra.Command) {
	// RootCmd can be built repeatedly by tests and the MCP mirror. Reset the
	// process-level client state before binding a fresh command tree.
	client.SetGlobalQueryParam("_fields", "")
	client.SetGlobalQueryParam("_embed", "")
	state := &wordpressGlobalFlagState{}
	rootCmd.PersistentFlags().Var(&wordpressFieldsValue{state: state}, "wp-fields", "WordPress server-side response fields (for example: id,title,link)")
	rootCmd.PersistentFlags().Var(&wordpressEmbedValue{state: state}, "embed", "Inline linked WordPress resources (_embed=1)")
	rootCmd.PersistentFlags().Lookup("embed").NoOptDefVal = "true"
}

func applyWordPressGlobalFlagState(state *wordpressGlobalFlagState) {
	if state == nil {
		return
	}
	if state.embedSet && state.embed {
		client.SetGlobalQueryParam("_embed", "1")
	} else {
		client.SetGlobalQueryParam("_embed", "")
	}
	if !state.fieldsSet {
		client.SetGlobalQueryParam("_fields", "")
		return
	}
	client.SetGlobalQueryParam("_fields", mergeWordPressFields(state.fields, state.embedSet && state.embed))
}

// mergeWordPressFields preserves explicit projection order and adds the two
// fields WordPress embedding requires when _fields and _embed are combined.
func mergeWordPressFields(fields string, embed bool) string {
	parts := strings.Split(fields, ",")
	result := make([]string, 0, len(parts)+2)
	seen := make(map[string]struct{}, len(parts)+2)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		result = append(result, part)
	}
	if embed {
		for _, required := range []string{"_links", "_embedded"} {
			if _, exists := seen[required]; exists {
				continue
			}
			seen[required] = struct{}{}
			result = append(result, required)
		}
	}
	return strings.Join(result, ",")
}
