// Copyright 2026 dndbeyond-community and contributors. Licensed under Apache-2.0.
//
// Local, read-only character snapshot importer. This is intentionally a
// preserved novel command rather than a D&D Beyond endpoint mirror: D&D
// Beyond does not publish a supported character-data API, and this command
// must not discover or replay private browser traffic.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"github.com/spf13/cobra"
)

const (
	characterSnapshotMaxBytes int64 = 10 << 20
	characterPDFMaxBytes      int64 = 25 << 20
)

// pp:data-source local
func init() {
	whichIndex = append(whichIndex, whichEntry{
		Command:      "characters inspect",
		Description:  "Normalize a user-supplied character-sheet snapshot or exported PDF for read-only sharing.",
		Group:        "characters sheet beyond20 snapshot pdf share",
		WhyItMatters: "Keeps character data local and bounded while making it easy for an agent or MCP caller to share sheet information.",
	})
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.Short = "Read public D&D Beyond rules pages and local character snapshots or PDFs."
		root.Long = "Read public D&D Beyond rules pages and normalize user-supplied character snapshots or exported PDFs locally.\n\n" +
			"Add --agent to any command for JSON output + non-interactive mode.\n" +
			"Character inspection never logs in, fetches private data, or stores raw input."
		addNovelCommandIfAbsent(root, newCharactersCmd(flags))
	})
}

func newCharactersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "characters",
		Short: "Normalize a user-supplied character-sheet snapshot.",
		Long: "Normalize a user-supplied Beyond20 or D&D Beyond character JSON snapshot, or an exported D&D Beyond PDF, for read-only sharing.\n\n" +
			"This command never logs in, fetches D&D Beyond, stores the raw input, or rolls dice.",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
	}
	cmd.AddCommand(newCharactersInspectCmd(flags))
	return cmd
}

func newCharactersInspectCmd(flags *rootFlags) *cobra.Command {
	var filePath string
	var fromStdin bool
	var inputFormat string
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Read and normalize a local character JSON snapshot or exported PDF.",
		Long: "Read a local JSON/PDF file (or stdin) containing a Beyond20 character object or an exported D&D Beyond character sheet. " +
			"Only shareable sheet fields are returned; raw input, account fields, notes, and secrets are omitted.",
		Example: "  dndbeyond-pp-cli characters inspect --file character.json --agent\n" +
			"  dndbeyond-pp-cli characters inspect --file character.pdf --agent\n" +
			"  dndbeyond-pp-cli characters inspect --stdin --format pdf --agent",
		Args: cobra.NoArgs,
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if (filePath == "") == !fromStdin {
				return errors.New("provide exactly one of --file <path> or --stdin")
			}
			data, format, err := readCharacterInput(filePath, fromStdin, inputFormat, cmd.InOrStdin())
			if err != nil {
				return err
			}
			out, err := normalizeCharacterSnapshotWithFormat(data, format)
			if err != nil {
				return err
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), out, flags, map[string]any{
				"source":     "local",
				"read_only":  true,
				"raw_stored": false,
			})
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to a user-supplied character JSON snapshot or exported PDF")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read a character snapshot from stdin")
	cmd.Flags().StringVar(&inputFormat, "format", "auto", "Input format: auto, json, or pdf (stdin requires --format pdf for PDF input)")
	return cmd
}

func readCharacterInput(filePath string, fromStdin bool, format string, stdin io.Reader) ([]byte, string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "auto"
	}
	if format != "auto" && format != "json" && format != "pdf" {
		return nil, "", fmt.Errorf("unsupported character input format %q (use auto, json, or pdf)", format)
	}
	if fromStdin {
		limit := characterSnapshotMaxBytes
		if format == "pdf" {
			limit = characterPDFMaxBytes
		}
		data, err := io.ReadAll(io.LimitReader(stdin, limit+1))
		if err != nil {
			return nil, "", fmt.Errorf("read character input from stdin: %w", err)
		}
		if int64(len(data)) > limit {
			return nil, "", fmt.Errorf("character %s exceeds %d MiB limit", format, limit>>20)
		}
		if format == "pdf" {
			snapshot, err := extractCharacterPDF(data, "stdin")
			return snapshot, "D&D Beyond exported PDF form", err
		}
		return data, "Beyond20/D&D Beyond JSON snapshot", nil
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("stat character input %q: %w", filePath, err)
	}
	if info.IsDir() {
		return nil, "", fmt.Errorf("character input path %q is a directory", filePath)
	}
	if format == "auto" {
		if strings.EqualFold(filepath.Ext(filePath), ".pdf") {
			format = "pdf"
		} else {
			format = "json"
		}
	}
	limit := characterSnapshotMaxBytes
	if format == "pdf" {
		limit = characterPDFMaxBytes
	}
	if info.Size() > limit {
		return nil, "", fmt.Errorf("character %s exceeds %d MiB limit", format, limit>>20)
	}
	// #nosec G304 -- the user intentionally selects a local export path; this
	// command is read-only and does not expose a server-side file boundary.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("read character input %q: %w", filePath, err)
	}
	if format == "pdf" {
		snapshot, err := extractCharacterPDF(data, filepath.Base(filePath))
		return snapshot, "D&D Beyond exported PDF form", err
	}
	return data, "Beyond20/D&D Beyond JSON snapshot", nil
}

func readCharacterSnapshot(filePath string, fromStdin bool, stdin io.Reader) ([]byte, error) {
	if fromStdin {
		data, err := io.ReadAll(io.LimitReader(stdin, characterSnapshotMaxBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read character snapshot from stdin: %w", err)
		}
		if int64(len(data)) > characterSnapshotMaxBytes {
			return nil, fmt.Errorf("character snapshot exceeds %d MiB limit", characterSnapshotMaxBytes>>20)
		}
		return data, nil
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat character snapshot %q: %w", filePath, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("character snapshot path %q is a directory", filePath)
	}
	if info.Size() > characterSnapshotMaxBytes {
		return nil, fmt.Errorf("character snapshot exceeds %d MiB limit", characterSnapshotMaxBytes>>20)
	}
	// #nosec G304 -- the user intentionally selects a local snapshot path; this
	// command is read-only and does not expose a server-side file boundary.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read character snapshot %q: %w", filePath, err)
	}
	return data, nil
}

func extractCharacterPDF(data []byte, source string) ([]byte, error) {
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		return nil, errors.New("character PDF input does not start with a PDF header")
	}
	var exported bytes.Buffer
	// PDF inspection must not create or read a user-level pdfcpu configuration
	// directory; this command is local, read-only, and uses core fonts only.
	api.DisableConfigDir()
	if err := api.ExportFormJSON(bytes.NewReader(data), &exported, source, model.NewDefaultConfiguration()); err == nil {
		return characterSnapshotFromPDFFormJSON(exported.Bytes())
	} else if !strings.Contains(strings.ToLower(err.Error()), "no form available") {
		return nil, fmt.Errorf("extract character PDF form fields: %w", err)
	}

	// Some D&D Beyond exports contain page-level Widget annotations with /V
	// values but omit the AcroForm /Fields tree. pdfcpu's normal form exporter
	// correctly rejects those as having no form; recover the orphaned widgets
	// without changing the PDF or starting a browser.
	widgets, err := extractPDFWidgetFields(data)
	if err != nil {
		return nil, fmt.Errorf("extract character PDF widget fields: %w", err)
	}
	return characterSnapshotFromPDFWidgetFields(widgets)
}

type pdfWidgetField struct {
	Label string
	Value string
}

func extractPDFWidgetFields(data []byte) ([]pdfWidgetField, error) {
	ctx, err := api.ReadContext(bytes.NewReader(data), model.NewDefaultConfiguration())
	if err != nil {
		return nil, err
	}
	if err := ctx.XRefTable.EnsurePageCount(); err != nil {
		return nil, err
	}
	fields := make([]pdfWidgetField, 0, 100)
	for page := 1; page <= ctx.PageCount; page++ {
		pageDict, _, _, err := ctx.XRefTable.PageDict(page, false)
		if err != nil {
			return nil, err
		}
		annotsObject, ok := pageDict.Find("Annots")
		if !ok {
			continue
		}
		annots, err := ctx.XRefTable.DereferenceArray(annotsObject)
		if err != nil {
			return nil, err
		}
		for _, annotObject := range annots {
			annot, err := ctx.XRefTable.DereferenceDict(annotObject)
			if err != nil {
				continue
			}
			if subtype := annot.Subtype(); subtype == nil || *subtype != "Widget" {
				continue
			}
			effective := annot
			if !hasPDFTextEntry(ctx, effective, "T") && !hasPDFTextEntry(ctx, effective, "V") {
				if parentObject, ok := effective.Find("Parent"); ok {
					if parent, parentErr := ctx.XRefTable.DereferenceDict(parentObject); parentErr == nil {
						effective = parent
					}
				}
			}
			label := pdfDictText(ctx, effective, "T")
			if label == "" {
				label = pdfDictText(ctx, effective, "TU")
			}
			value := pdfDictText(ctx, effective, "V")
			if label == "" || value == "" {
				continue
			}
			fields = append(fields, pdfWidgetField{Label: label, Value: value})
			if len(fields) >= 100 {
				return fields, nil
			}
		}
	}
	if len(fields) == 0 {
		return nil, errors.New("character PDF contains no populated widget fields")
	}
	return fields, nil
}

func hasPDFTextEntry(ctx *model.Context, dict types.Dict, key string) bool {
	object, ok := dict.Find(key)
	if !ok {
		return false
	}
	_, err := ctx.XRefTable.DereferenceText(object)
	return err == nil
}

func pdfDictText(ctx *model.Context, dict types.Dict, key string) string {
	object, ok := dict.Find(key)
	if !ok {
		return ""
	}
	value, err := ctx.XRefTable.DereferenceText(object)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func characterSnapshotFromPDFWidgetFields(fields []pdfWidgetField) ([]byte, error) {
	character := make(map[string]any)
	pdfFields := make([]any, 0, len(fields))
	for _, field := range fields {
		addPDFCharacterField(character, &pdfFields, field.Label, field.Value)
	}
	if len(character) == 0 && len(pdfFields) == 0 {
		return nil, errors.New("character PDF contains no populated shareable widget fields")
	}
	character["pdf_fields"] = pdfFields
	return json.Marshal(map[string]any{"character": character})
}

type pdfFormGroup struct {
	Forms []pdfForm `json:"forms"`
}

type pdfForm struct {
	TextFields        []pdfTextField        `json:"textfield"`
	DateFields        []pdfTextField        `json:"datefield"`
	CheckBoxes        []pdfCheckBox         `json:"checkbox"`
	RadioButtonGroups []pdfRadioButtonGroup `json:"radiobuttongroup"`
	ComboBoxes        []pdfTextField        `json:"combobox"`
	ListBoxes         []pdfListBox          `json:"listbox"`
}

type pdfTextField struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	AltName string `json:"altname"`
	Value   string `json:"value"`
}

type pdfCheckBox struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	AltName string `json:"altname"`
	Value   bool   `json:"value"`
}

type pdfRadioButtonGroup struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	AltName string `json:"altname"`
	Value   string `json:"value"`
}

type pdfListBox struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	AltName string   `json:"altname"`
	Values  []string `json:"values"`
}

func characterSnapshotFromPDFFormJSON(data []byte) ([]byte, error) {
	var group pdfFormGroup
	if err := json.Unmarshal(data, &group); err != nil {
		return nil, fmt.Errorf("decode extracted PDF form fields: %w", err)
	}
	if len(group.Forms) == 0 {
		return nil, errors.New("character PDF contains no form fields")
	}

	character := make(map[string]any)
	pdfFields := make([]any, 0, 100)
	for _, form := range group.Forms {
		for _, field := range form.TextFields {
			addPDFCharacterField(character, &pdfFields, pdfFieldLabel(field.ID, field.Name, field.AltName), field.Value)
		}
		for _, field := range form.DateFields {
			addPDFCharacterField(character, &pdfFields, pdfFieldLabel(field.ID, field.Name, field.AltName), field.Value)
		}
		for _, field := range form.CheckBoxes {
			addPDFCharacterField(character, &pdfFields, pdfFieldLabel(field.ID, field.Name, field.AltName), field.Value)
		}
		for _, field := range form.RadioButtonGroups {
			addPDFCharacterField(character, &pdfFields, pdfFieldLabel(field.ID, field.Name, field.AltName), field.Value)
		}
		for _, field := range form.ComboBoxes {
			addPDFCharacterField(character, &pdfFields, pdfFieldLabel(field.ID, field.Name, field.AltName), field.Value)
		}
		for _, field := range form.ListBoxes {
			addPDFCharacterField(character, &pdfFields, pdfFieldLabel(field.ID, field.Name, field.AltName), field.Values)
		}
	}
	if len(character) == 0 && len(pdfFields) == 0 {
		return nil, errors.New("character PDF contains no populated shareable fields")
	}
	if len(pdfFields) > 100 {
		pdfFields = pdfFields[:100]
	}
	character["pdf_fields"] = pdfFields
	return json.Marshal(map[string]any{"character": character})
}

func pdfFieldLabel(id, name, altName string) string {
	for _, value := range []string{name, altName, id} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "field"
}

func addPDFCharacterField(character map[string]any, pdfFields *[]any, label string, value any) {
	if strings.TrimSpace(label) == "" || isSensitiveCharacterKey(label) || isPDFNarrativeField(label) || !pdfFieldHasValue(value) {
		return
	}
	semanticPDFField(character, label, value)
	if len(*pdfFields) < 100 {
		*pdfFields = append(*pdfFields, map[string]any{"name": boundCharacterString(label), "value": value})
	}
}

func isPDFNarrativeField(label string) bool {
	key := strings.ToLower(strings.NewReplacer(" ", "", "_", "", "-", "", ".", "", "/", "").Replace(label))
	for _, fragment := range []string{"description", "personality", "ideal", "bond", "flaw", "appearance", "history", "goal", "alignment", "height", "weight", "age"} {
		if strings.Contains(key, fragment) {
			return true
		}
	}
	return false
}

func pdfFieldHasValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(v) != ""
	case []string:
		return len(v) > 0
	case bool:
		return v
	default:
		return true
	}
}

func semanticPDFField(character map[string]any, label string, value any) bool {
	key := strings.ToLower(strings.NewReplacer(" ", "", "_", "", "-", "", ".", "", "/", "").Replace(label))
	text := strings.TrimSpace(fmt.Sprint(value))
	switch {
	case key == "charactername" || key == "charname" || key == "name":
		character["name"] = text
	case key == "race" || strings.HasSuffix(key, "race"):
		character["race"] = text
	case strings.Contains(key, "class") && strings.Contains(key, "level"):
		character["classes"] = text
		if total := sumCharacterLevels(text); total > 0 {
			character["level"] = total
		} else {
			character["level"] = text
		}
	case key == "class" || key == "classname" || strings.HasSuffix(key, "class"):
		character["classes"] = text
	case key == "level" || strings.HasSuffix(key, "level"):
		character["level"] = text
	case key == "hp" || key == "currenthp" || strings.HasSuffix(key, "currenthp"):
		character["hp"] = value
	case key == "maxhp" || key == "maximumhp" || strings.HasSuffix(key, "maxhp"):
		character["max_hp"] = value
	case key == "ac" || key == "armorclass" || strings.HasSuffix(key, "armorclass"):
		character["ac"] = value
	case key == "proficiency" || key == "proficiencybonus" || key == "profbonus" || strings.HasSuffix(key, "proficiencybonus"):
		character["proficiency"] = value
	case key == "speed" || strings.HasSuffix(key, "speed"):
		character["speed"] = value
	case key == "initiative" || key == "init" || strings.HasSuffix(key, "initiative"):
		character["initiative"] = value
	default:
		for ability, aliases := range map[string][]string{
			"str": {"strength", "str"}, "dex": {"dexterity", "dex"}, "con": {"constitution", "con"},
			"int": {"intelligence", "int"}, "wis": {"wisdom", "wis"}, "cha": {"charisma", "cha"},
		} {
			for _, alias := range aliases {
				if key == alias || strings.HasPrefix(key, alias+"score") || strings.HasPrefix(key, alias+"ability") {
					abilities, _ := character["abilities"].(map[string]any)
					if abilities == nil {
						abilities = make(map[string]any)
						character["abilities"] = abilities
					}
					entry, _ := abilities[ability].(map[string]any)
					if entry == nil {
						entry = make(map[string]any)
						abilities[ability] = entry
					}
					if strings.Contains(key, "modifier") || strings.HasSuffix(key, "mod") {
						entry["modifier"] = value
					} else {
						entry["score"] = value
					}
					return true
				}
			}
		}
	}
	return true
}

func sumCharacterLevels(value string) int {
	total := 0
	for _, token := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ' ' || r == '/' || r == ',' || r == ':' || r == '(' || r == ')'
	}) {
		level, err := strconv.Atoi(token)
		if err == nil && level > 0 {
			total += level
		}
	}
	return total
}

func boundCharacterString(value string) string {
	if len(value) > 4000 {
		return value[:4000] + "…"
	}
	return value
}

var characterSensitiveKeys = map[string]bool{
	"account": true, "accountid": true, "account_id": true, "auth": true,
	"backstory": true, "cookie": true, "email": true, "login": true,
	"notes": true, "password": true, "player": true, "playername": true,
	"private": true, "secret": true, "session": true, "token": true,
	"userid": true, "user_id": true, "username": true,
}

var characterAllowedKeys = map[string]bool{
	"abilities": true, "ability": true, "ac": true, "action": true, "actions": true,
	"attack_bonus": true, "avatar": true, "classes": true, "damage": true,
	"dice": true, "equipment": true, "features": true, "hp": true, "id": true,
	"initiative": true, "level": true, "max_hp": true, "max-hp": true, "modifier": true,
	"name": true, "proficiency": true, "race": true, "range": true, "saving_throws": true,
	"saves": true, "score": true, "skills": true, "source": true, "speed": true, "spells": true,
	"type": true, "uses": true, "equipped": true, "quantity": true,
	"qty": true, "weight": true, "slot": true,
	"pdf_fields": true, "value": true,
}

func normalizeCharacterSnapshot(data []byte) ([]byte, error) {
	return normalizeCharacterSnapshotWithFormat(data, "Beyond20/D&D Beyond JSON snapshot")
}

func normalizeCharacterSnapshotWithFormat(data []byte, inputFormat string) ([]byte, error) {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode character snapshot JSON: %w", err)
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("character snapshot must be a JSON object")
	}
	// Beyond20 messages and several export tools wrap the character object.
	for _, key := range []string{"character", "data", "payload"} {
		if nested, ok := obj[key].(map[string]any); ok {
			obj = nested
			break
		}
	}

	character := make(map[string]any)
	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		canonical := canonicalCharacterKey(key)
		if isSensitiveCharacterKey(canonical) || !characterAllowedKeys[canonical] {
			continue
		}
		// Start nested values at depth 1 so container keys such as ability
		// abbreviations and class names are retained while their descendants
		// still pass through the sensitive-key filter.
		value, ok := sanitizeCharacterValue(obj[key], 1)
		if ok {
			character[canonical] = value
		}
	}
	if len(character) == 0 {
		return nil, errors.New("character snapshot contains no supported shareable fields")
	}
	result := map[string]any{
		"schema":       "dndbeyond-character-snapshot/v1",
		"character":    character,
		"read_only":    true,
		"input_format": inputFormat,
		"privacy": map[string]any{
			"raw_input_stored":                      false,
			"identity_and_narrative_fields_omitted": true,
			"remote_requests_made":                  false,
		},
	}
	return json.Marshal(result)
}

func canonicalCharacterKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.ReplaceAll(key, " ", "_")
	key = strings.ReplaceAll(key, "-", "_")
	return strings.ToLower(key)
}

func isSensitiveCharacterKey(key string) bool {
	canonical := strings.ToLower(strings.ReplaceAll(canonicalCharacterKey(key), "_", ""))
	if characterSensitiveKeys[canonical] {
		return true
	}
	for _, fragment := range []string{"email", "token", "cookie", "password", "secret", "account", "username", "userid", "player", "backstory", "notes", "private", "session", "phone", "address", "ssn", "dateofbirth", "dob"} {
		if strings.Contains(canonical, fragment) {
			return true
		}
	}
	return false
}

func sanitizeCharacterValue(value any, depth int) (any, bool) {
	if depth > 4 {
		return nil, false
	}
	switch v := value.(type) {
	case nil, bool, float64:
		return v, true
	case string:
		if len(v) > 4000 {
			return v[:4000] + "…", true
		}
		return v, true
	case []any:
		out := make([]any, 0, minInt(len(v), 100))
		for i, item := range v {
			if i >= 100 {
				break
			}
			if clean, ok := sanitizeCharacterValue(item, depth+1); ok {
				out = append(out, clean)
			}
		}
		return out, true
	case map[string]any:
		out := make(map[string]any)
		for key, item := range v {
			canonical := canonicalCharacterKey(key)
			if isSensitiveCharacterKey(canonical) || (depth >= 2 && !characterAllowedKeys[canonical]) {
				continue
			}
			if clean, ok := sanitizeCharacterValue(item, depth+1); ok {
				out[canonical] = clean
			}
		}
		return out, true
	default:
		return fmt.Sprint(v), true
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
