// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/svc"
	"github.com/spf13/cobra"
)

// newNotesCmd is the `notes` parent command grouping note-management
// subcommands (currently `notes add`).
func newNotesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notes",
		Short: "Manage notes in your collection (add)",
		Long:  "Note-management commands for your AnkiWeb collection. Currently supports adding notes; see 'notes add --help'.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNotesAddCmd(flags))
	return cmd
}

func newNotesAddCmd(flags *rootFlags) *cobra.Command {
	var deck, noteType, tags string
	var fieldKV []string

	cmd := &cobra.Command{
		Use:   "add [field-values...]",
		Short: "Add a note (card) to one of your decks (requires login)",
		Long: "Add a note to your AnkiWeb collection via /svc/editor/add-or-update.\n" +
			"Provide field values positionally (e.g. notes add \"Bonjour\" \"Hello\") or by name\n" +
			"with --field Name=Value. Defaults to your default note type and deck; override\n" +
			"with --type / --deck (run 'notetypes' to see valid names). This writes to your\n" +
			"real collection — preview with --dry-run, and it asks for confirmation unless --yes.",
		Example: strings.Trim(`
  ankiweb-pp-cli notes add "Bonjour" "Hello" --deck "Words & phrases" --dry-run
  ankiweb-pp-cli notes add --field Front=Bonjour --field Back=Hello --tags french --yes
  ankiweb-pp-cli notetypes   # list valid --type / --deck names`, "\n"),
		// NOT mcp:read-only — this command mutates the user's collection.
		Annotations: map[string]string{"pp:endpoint": "editor.add", "pp:method": "POST", "pp:path": "/svc/editor/add-or-update"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && len(fieldKV) == 0 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"status": "ok", "verify": true}, flags)
			}

			c, _, err := flags.newEditorClient()
			if err != nil {
				return err
			}

			// Fast path: when --deck and --type are already numeric IDs and field
			// values are given positionally, skip get-info-for-adding entirely.
			// This avoids a round-trip that is only needed to resolve names → IDs
			// and look up field names.
			ntIDFast, ntFastErr := strconv.ParseUint(strings.TrimSpace(noteType), 10, 64)
			dkIDFast, dkFastErr := strconv.ParseUint(strings.TrimSpace(deck), 10, 64)
			if ntFastErr == nil && dkFastErr == nil && len(args) > 0 && len(fieldKV) == 0 {
				fieldMap := map[string]string{}
				for i, v := range args {
					fieldMap[fmt.Sprintf("field%d", i+1)] = v
				}
				plan := map[string]any{
					"deck":      deck,
					"note_type": noteType,
					"fields":    fieldMap,
					"tags":      tags,
				}
				if flags.dryRun {
					plan["status"] = "dry-run"
					return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
				}
				if !flags.yes && !flags.noInput {
					fmt.Fprintf(cmd.ErrOrStderr(), "Add note to deck %s (note type %s)? [y/N] ", deck, noteType)
					reader := bufio.NewReader(cmd.InOrStdin())
					line, _ := reader.ReadString('\n')
					if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
						return usageErr(fmt.Errorf("aborted (pass --yes to skip confirmation)"))
					}
				}
				req := svc.BuildAddNoteRequest(ntIDFast, dkIDFast, args, tags)
				if _, status, err := c.PostBytes(cmd.Context(), "/svc/editor/add-or-update", req); err != nil {
					if status == http.StatusForbidden || status == http.StatusUnauthorized {
						return authErr(errAuthEditor())
					}
					return classifyAPIError(err, flags)
				}
				plan["status"] = "added"
				return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
			}

			// Slow path: fetch note types + decks + defaults to resolve names.
			data, status, err := c.PostBytes(cmd.Context(), "/svc/editor/get-info-for-adding", nil)
			if err != nil {
				if status == http.StatusForbidden || status == http.StatusUnauthorized {
					return authErr(errAuthEditor())
				}
				return classifyAPIError(err, flags)
			}
			info, err := svc.DecodeAddInfo(data)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			nt, err := resolveNoteType(info, noteType)
			if err != nil {
				return err
			}
			dk, err := resolveDeck(info, deck)
			if err != nil {
				return err
			}

			values, fieldMap, err := resolveFieldValues(info, nt, args, fieldKV)
			if err != nil {
				return err
			}

			ntID, _ := strconv.ParseUint(nt.ID, 10, 64)
			dkID, _ := strconv.ParseUint(dk.ID, 10, 64)
			req := svc.BuildAddNoteRequest(ntID, dkID, values, tags)

			plan := map[string]any{
				"deck":      dk.Name,
				"note_type": nt.Name,
				"fields":    fieldMap,
				"tags":      tags,
			}

			if flags.dryRun {
				plan["status"] = "dry-run"
				return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
			}

			if !flags.yes && !flags.noInput {
				fmt.Fprintf(cmd.ErrOrStderr(), "Add note to deck %q (note type %q)? [y/N] ", dk.Name, nt.Name)
				reader := bufio.NewReader(cmd.InOrStdin())
				line, _ := reader.ReadString('\n')
				if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
					return usageErr(fmt.Errorf("aborted (pass --yes to skip confirmation)"))
				}
			}

			if _, status, err := c.PostBytes(cmd.Context(), "/svc/editor/add-or-update", req); err != nil {
				if status == http.StatusForbidden || status == http.StatusUnauthorized {
					return authErr(errAuthEditor())
				}
				return classifyAPIError(err, flags)
			}
			plan["status"] = "added"
			return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
		},
	}
	cmd.Flags().StringVar(&deck, "deck", "", "Target deck name or id (default: your default deck)")
	cmd.Flags().StringVar(&noteType, "type", "", "Note type name (default: your default note type)")
	cmd.Flags().StringArrayVar(&fieldKV, "field", nil, "Field as Name=Value (repeatable); alternative to positional values")
	cmd.Flags().StringVar(&tags, "tags", "", "Space-separated tags to attach to the note")
	return cmd
}

// resolveNoteType picks the note type by (case-insensitive) name, or the
// account default when name is empty.
func resolveNoteType(info svc.AddInfo, name string) (svc.NoteType, error) {
	if strings.TrimSpace(name) == "" {
		for _, nt := range info.NoteTypes {
			if nt.ID == info.DefaultNoteTypeID {
				return nt, nil
			}
		}
		if len(info.NoteTypes) > 0 {
			return info.NoteTypes[0], nil
		}
		return svc.NoteType{}, apiErr(fmt.Errorf("no note types available on this account"))
	}
	for _, nt := range info.NoteTypes {
		if strings.EqualFold(nt.Name, name) || nt.ID == name {
			return nt, nil
		}
	}
	return svc.NoteType{}, notFoundErr(fmt.Errorf("note type %q not found; available: %s", name, noteTypeNames(info)))
}

// resolveDeck picks the deck by name or id, or the account default when empty.
func resolveDeck(info svc.AddInfo, nameOrID string) (svc.AddDeck, error) {
	if strings.TrimSpace(nameOrID) == "" {
		for _, d := range info.Decks {
			if d.ID == info.DefaultDeckID {
				return d, nil
			}
		}
		if len(info.Decks) > 0 {
			return info.Decks[0], nil
		}
		return svc.AddDeck{}, apiErr(fmt.Errorf("no decks available on this account"))
	}
	for _, d := range info.Decks {
		if strings.EqualFold(d.Name, nameOrID) || d.ID == nameOrID {
			return d, nil
		}
	}
	return svc.AddDeck{}, notFoundErr(fmt.Errorf("deck %q not found; available: %s", nameOrID, deckNames(info)))
}

// resolveFieldValues builds the ordered field-value slice the add request needs
// and a name->value map for the dry-run/confirmation display. Positional args
// are taken in order. --field Name=Value pairs are mapped to the note type's
// field order when the type's field names are known (the default note type);
// otherwise the values are sent in the order the flags were given.
func resolveFieldValues(info svc.AddInfo, nt svc.NoteType, args, fieldKV []string) ([]string, map[string]string, error) {
	fieldMap := map[string]string{}
	if len(fieldKV) > 0 {
		type kv struct{ name, val string }
		var pairs []kv
		for _, f := range fieldKV {
			i := strings.IndexByte(f, '=')
			if i < 0 {
				return nil, nil, usageErr(fmt.Errorf("--field must be Name=Value, got %q", f))
			}
			n := strings.TrimSpace(f[:i])
			pairs = append(pairs, kv{n, f[i+1:]})
			fieldMap[n] = f[i+1:]
		}
		// Order by the note type's field names when known (default note type).
		if nt.ID == info.DefaultNoteTypeID && len(info.DefaultFields) > 0 {
			byName := map[string]string{}
			for _, p := range pairs {
				byName[strings.ToLower(p.name)] = p.val
			}
			var values []string
			for _, fn := range info.DefaultFields {
				values = append(values, byName[strings.ToLower(fn)])
			}
			return values, fieldMap, nil
		}
		var values []string
		for _, p := range pairs {
			values = append(values, p.val)
		}
		return values, fieldMap, nil
	}

	// Positional values, in order.
	if len(args) == 0 {
		return nil, nil, usageErr(fmt.Errorf("provide field values positionally or with --field Name=Value"))
	}
	for i, v := range args {
		name := fmt.Sprintf("field%d", i+1)
		if nt.ID == info.DefaultNoteTypeID && i < len(info.DefaultFields) {
			name = info.DefaultFields[i]
		}
		fieldMap[name] = v
	}
	return args, fieldMap, nil
}

func noteTypeNames(info svc.AddInfo) string {
	var n []string
	for _, nt := range info.NoteTypes {
		n = append(n, nt.Name)
	}
	return strings.Join(n, ", ")
}

func deckNames(info svc.AddInfo) string {
	var n []string
	for _, d := range info.Decks {
		n = append(n, d.Name)
	}
	return strings.Join(n, ", ")
}
