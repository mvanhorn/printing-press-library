// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/internal/config"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/internal/store"

	"github.com/spf13/cobra"
)

func newCallTraceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var live bool

	cmd := &cobra.Command{
		Use:   "call-trace [CallSid]",
		Short: "Given a CallSid, return the Call + Recordings + Transcriptions + Conference participation in one structured output",
		Long: `Stitches every resource related to one Call into a single JSON envelope:
the Call record, all Recordings on that call, any Transcriptions of those
recordings, and any Conference Participants tied to the call.

Reads the local store first; with --live, falls back to API calls when the
local cache does not have a row for the CallSid.

Three round trips collapsed to one command. No other Twilio tool stitches the
chain.`,
		Example: `  twilio-pp-cli call-trace CAxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx --json
  twilio-pp-cli call-trace CAxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx --live --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			callSid := strings.TrimSpace(args[0])
			if !strings.HasPrefix(callSid, "CA") {
				return usageErr(fmt.Errorf("CallSid must start with 'CA' (got %q)", callSid))
			}

			envelope := map[string]any{"call_sid": callSid}

			if dbPath == "" {
				dbPath = defaultDBPath("twilio-pp-cli")
			}
			db, derr := store.OpenWithContext(cmd.Context(), dbPath)
			if derr == nil {
				defer db.Close()
				// Look up the Call record from the local store.
				var callJSON sql_NullJSON
				err := db.DB().QueryRowContext(cmd.Context(),
					`SELECT data FROM calls_json WHERE id = ? LIMIT 1`, callSid).Scan(&callJSON)
				if err == nil && callJSON.Valid {
					var callObj map[string]any
					if jerr := json.Unmarshal(callJSON.RawMessage, &callObj); jerr == nil {
						envelope["call"] = callObj
					}
				}

				// Recordings tied to this call.
				rows, rerr := db.DB().QueryContext(cmd.Context(),
					`SELECT data FROM calls_recordings_json WHERE json_extract(data, '$.call_sid') = ?`, callSid)
				if rerr == nil {
					recs := collectJSONRows(rows)
					if len(recs) > 0 {
						envelope["recordings"] = recs
					}
				}

				// Transcriptions on those recordings (best-effort: filtered by call sid).
				tRows, terr := db.DB().QueryContext(cmd.Context(),
					`SELECT data FROM calls_transcriptions_json WHERE json_extract(data, '$.call_sid') = ?`, callSid)
				if terr == nil {
					trs := collectJSONRows(tRows)
					if len(trs) > 0 {
						envelope["transcriptions"] = trs
					}
				}

				// Conference participants where call_sid matches.
				pRows, perr := db.DB().QueryContext(cmd.Context(),
					`SELECT data FROM participants_json WHERE json_extract(data, '$.call_sid') = ?`, callSid)
				if perr == nil {
					ps := collectJSONRows(pRows)
					if len(ps) > 0 {
						envelope["conference_participants"] = ps
					}
				}
			}

			// Live fallback: if --live OR we found nothing in the store, hit the API.
			if live || envelope["call"] == nil {
				if c, cerr := flags.newClient(); cerr == nil {
					accountSid := getAccountSidFromConfig(flags)
					if accountSid != "" {
						basePath := fmt.Sprintf("/2010-04-01/Accounts/%s", accountSid)
						if envelope["call"] == nil {
							if data, err := c.Get(fmt.Sprintf("%s/Calls/%s.json", basePath, callSid), nil); err == nil {
								var obj map[string]any
								if jerr := json.Unmarshal(data, &obj); jerr == nil {
									envelope["call"] = obj
								}
							}
						}
						if envelope["recordings"] == nil {
							if data, err := c.Get(fmt.Sprintf("%s/Calls/%s/Recordings.json", basePath, callSid), nil); err == nil {
								if recs := unwrapList(data, "recordings"); recs != nil {
									envelope["recordings"] = recs
								}
							}
						}
					}
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&live, "live", false, "Force live API calls instead of local-first lookup")
	return cmd
}

// sql_NullJSON wraps json.RawMessage with sql.Scanner so it can be used in
// QueryRow().Scan(&n) calls when the column may be NULL. The underscore in
// the type name disambiguates it from any database/sql.Null* shim and signals
// it is a CLI-local helper, not a generic library type.
type sql_NullJSON struct {
	json.RawMessage
	Valid bool
}

func (n *sql_NullJSON) Scan(v any) error {
	if v == nil {
		n.Valid = false
		return nil
	}
	switch x := v.(type) {
	case []byte:
		n.RawMessage = json.RawMessage(append([]byte(nil), x...))
	case string:
		n.RawMessage = json.RawMessage([]byte(x))
	default:
		return fmt.Errorf("unexpected scan type %T", v)
	}
	n.Valid = true
	return nil
}

// collectJSONRows reads JSON-blob rows into a []any of unmarshalled objects.
// Returns nil when rows is empty or any row fails to parse.
func collectJSONRows(rows interface {
	Next() bool
	Scan(...any) error
	Close() error
}) []any {
	defer rows.Close()
	var out []any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil
		}
		var obj any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		out = append(out, obj)
	}
	return out
}

// unwrapList pulls the named list out of a Twilio paginated envelope.
// Twilio responses look like {"recordings": [...], "page": 0, ...}; this
// returns the list under the named key, or nil if the shape doesn't match.
func unwrapList(data json.RawMessage, key string) []any {
	var env map[string]json.RawMessage
	if err := json.Unmarshal(data, &env); err != nil {
		return nil
	}
	raw, ok := env[key]
	if !ok {
		return nil
	}
	var list []any
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil
	}
	return list
}

// getAccountSidFromConfig pulls the Account SID from the config file or env
// for use in URL path construction. Returns empty string when not configured.
func getAccountSidFromConfig(flags *rootFlags) string {
	if flags == nil {
		return ""
	}
	cfg, err := config.Load(flags.configPath)
	if err != nil || cfg == nil {
		return ""
	}
	return cfg.AccountSid
}
