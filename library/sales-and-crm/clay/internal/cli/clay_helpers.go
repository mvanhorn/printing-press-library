// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Clay domain helpers shared by the novel commands.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/clay/internal/client"
)

// clayField is one column in a Clay table.
type clayField struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	TypeSettings json.RawMessage `json:"typeSettings,omitempty"`
	IsLocked     bool            `json:"isLocked,omitempty"`
}

// clayTable is the subset of a Clay table document the novel commands use.
type clayTable struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	WorkbookID  string      `json:"workbookId"`
	FirstViewID string      `json:"firstViewId"`
	Fields      []clayField `json:"fields"`
}

// typeSettings decodes the parts of a column's typeSettings the CLI reasons about.
type typeSettings struct {
	FormulaType      string          `json:"formulaType,omitempty"`
	FormulaText      string          `json:"formulaText,omitempty"`
	Formula          string          `json:"formula,omitempty"`
	FormulaPrompt    string          `json:"formulaPrompt,omitempty"`
	MappedResultPath string          `json:"mappedResultPath,omitempty"`
	ActionKey        string          `json:"actionKey,omitempty"`
	ActionPackageID  string          `json:"actionPackageId,omitempty"`
	ActionVersion    int             `json:"actionVersion,omitempty"`
	AuthAccountID    string          `json:"authAccountId,omitempty"`
	RunAsButton      bool            `json:"runAsButton,omitempty"`
	UseStaticIP      bool            `json:"useStaticIP,omitempty"`
	InputsBinding    json.RawMessage `json:"inputsBinding,omitempty"`
	DataTypeSettings json.RawMessage `json:"dataTypeSettings,omitempty"`
}

func (f clayField) settings() typeSettings {
	var ts typeSettings
	if len(f.TypeSettings) > 0 {
		_ = json.Unmarshal(f.TypeSettings, &ts)
	}
	return ts
}

// formulaBody returns whichever formula field this column actually uses.
func (t typeSettings) formulaBody() string {
	if t.FormulaText != "" {
		return t.FormulaText
	}
	return t.Formula
}

// fieldRefPattern matches Clay's {{f_...}} column references inside formulas.
var fieldRefPattern = regexp.MustCompile(`\{\{\s*(f_[A-Za-z0-9_]+)\s*\}\}`)

// formulaRefs returns the distinct field ids a formula references, in order.
func formulaRefs(formula string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, m := range fieldRefPattern.FindAllStringSubmatch(formula, -1) {
		if len(m) < 2 || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, m[1])
	}
	return out
}

// fieldIDPattern matches a generated Clay field id. Used to tell an actual id
// apart from a column whose *name* merely begins with "f_", which is legal and
// must still be remapped by name.
var fieldIDPattern = regexp.MustCompile(`^f_[A-Za-z0-9_]+$`)

// namedRefPattern matches human-authored {{Column Name}} references.
var namedRefPattern = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

// resolveRefsToNames rewrites {{f_id}} into {{Column Name}} for display and
// editing.
//
// It deliberately refuses to render a column name that is itself a real field
// id in this table. Doing so would break the round trip: the write path treats
// an id-shaped token that names a real field as an explicit id reference, so
// rendering column "f_other" for field f_realid would silently rebind the
// formula to f_other on the next save. Keeping the raw id is less readable and
// strictly correct.
func resolveRefsToNames(formula string, byID map[string]clayField) string {
	return fieldRefPattern.ReplaceAllStringFunc(formula, func(m string) string {
		sub := fieldRefPattern.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		f, ok := byID[sub[1]]
		if !ok || f.Name == "" {
			return m
		}
		// The name collides with some other field's id: rendering it would not
		// survive a save. Leave the id in place.
		if other, clash := byID[f.Name]; clash && other.ID != f.ID {
			return m
		}
		return "{{" + f.Name + "}}"
	})
}

// resolveNamesToRefs rewrites {{Column Name}} into {{f_id}} before writing a
// user-authored formula.
//
// Precedence is deliberately the opposite of blueprint apply. Here the author
// may type an explicit field id, so an id-shaped token that names a real field
// in this table binds to that field. Only when the token is not a real field id
// do we fall back to a column-name lookup. A column literally named like a
// different real field id is reported as ambiguous rather than silently bound.
func resolveNamesToRefs(formula string, byName map[string]clayField, byID map[string]clayField) (string, []string, []string) {
	var unknown []string
	var ambiguous []string
	out := namedRefPattern.ReplaceAllStringFunc(formula, func(m string) string {
		sub := namedRefPattern.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		token := strings.TrimSpace(sub[1])

		// An explicit, real field id wins: the author asked for that field.
		if _, isRealID := byID[token]; isRealID {
			if named, clash := byName[strings.ToLower(token)]; clash && named.ID != token {
				ambiguous = append(ambiguous, token)
			}
			return m
		}
		// Otherwise resolve by column name, which also covers a column whose
		// name merely looks like a field id.
		if f, ok := byName[strings.ToLower(token)]; ok {
			return "{{" + f.ID + "}}"
		}
		// Unrecognized but id-shaped: pass through and let the API judge it.
		if fieldIDPattern.MatchString(token) {
			return m
		}
		unknown = append(unknown, token)
		return m
	})
	sort.Strings(unknown)
	sort.Strings(ambiguous)
	return out, unknown, ambiguous
}

func indexByID(fields []clayField) map[string]clayField {
	m := make(map[string]clayField, len(fields))
	for _, f := range fields {
		m[f.ID] = f
	}
	return m
}

func indexByName(fields []clayField) map[string]clayField {
	m := make(map[string]clayField, len(fields))
	for _, f := range fields {
		m[strings.ToLower(f.Name)] = f
	}
	return m
}

// resolveWorkspace picks the workspace id from an explicit flag, then env.
func resolveWorkspace(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit), nil
	}
	for _, k := range []string{"CLAY_WORKSPACE_ID", "CLAY_WORKSPACE"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("workspace id is required: pass --workspace or set CLAY_WORKSPACE_ID")
}

// fetchTable loads a table with its full field list.
func fetchTable(ctx context.Context, c *client.Client, ws, tableID string) (*clayTable, error) {
	path := fmt.Sprintf("/workspaces/%s/tables/%s/details", ws, tableID)
	raw, err := c.Get(ctx, path, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching table %s: %w", tableID, err)
	}
	// Clay returns either the bare table or {"table": {...}}.
	var wrapper struct {
		Table *clayTable `json:"table"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil && wrapper.Table != nil && wrapper.Table.ID != "" {
		return wrapper.Table, nil
	}
	var t clayTable
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("parsing table %s: %w", tableID, err)
	}
	if t.ID == "" {
		return nil, fmt.Errorf("table %s returned no id", tableID)
	}
	return &t, nil
}

// runStatusCounts is the per-column enrichment status payload.
type runStatusCounts struct {
	StatusCountsByField map[string][]struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	} `json:"statusCountsByField"`
}

func fetchRunStatus(ctx context.Context, c *client.Client, ws, tableID string) (*runStatusCounts, error) {
	path := fmt.Sprintf("/workspaces/%s/tables/%s/fields/runstatus", ws, tableID)
	raw, err := c.Get(ctx, path, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching run status for %s: %w", tableID, err)
	}
	var rs runStatusCounts
	if err := json.Unmarshal(raw, &rs); err != nil {
		return nil, fmt.Errorf("parsing run status for %s: %w", tableID, err)
	}
	return &rs, nil
}

// lowerTrim normalizes a user-supplied column name for map lookups.
func lowerTrim(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
