// Copyright 2026 Chris Hatton and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: member provision from CSV. Hand-filled scaffold.

// pp:data-source live
package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/sigma-computing/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/sigma-computing/internal/cliutil"
	"github.com/spf13/cobra"
)

// provisionRow is one parsed CSV row describing a member to provision.
type provisionRow struct {
	Email      string            `json:"email"`
	FirstName  string            `json:"firstName"`
	LastName   string            `json:"lastName"`
	MemberType string            `json:"memberType"`
	Team       string            `json:"team,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

func newNovelMemberProvisionCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagIdempotent bool

	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Create or update members in bulk from a CSV, assigning teams and user attributes idempotently in one pass.",
		Example: strings.Trim(`
  sigma-computing-pp-cli member provision --from new-hires.csv --dry-run
  sigma-computing-pp-cli member provision --from new-hires.csv --idempotent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(flagFrom) == "" {
				return fmt.Errorf("missing required flag --from: path to the new-hires CSV (columns: email,firstName,lastName,memberType[,team][,attributes])")
			}

			out := cmd.OutOrStdout()
			planning := dryRunOK(flags) || cliutil.IsVerifyEnv()

			f, err := os.Open(flagFrom)
			if err != nil {
				// In a dry-run/verify plan the CSV may not exist yet; report the
				// plan against the path rather than failing.
				if planning {
					fmt.Fprintf(out, "would provision members from %q (file not present; nothing parsed)\n", flagFrom)
					return nil
				}
				return fmt.Errorf("opening --from %q: %w", flagFrom, err)
			}
			defer f.Close()

			rows, err := parseProvisionCSV(f)
			if err != nil {
				return fmt.Errorf("parsing --from %q: %w", flagFrom, err)
			}

			// Short-circuit for dry-run / verify env BEFORE any HTTP.
			if planning {
				for _, r := range rows {
					fmt.Fprintf(out, "would create %s (%s %s, type=%s, team=%s, attrs=%d)\n",
						r.Email, r.FirstName, r.LastName, r.MemberType, emptyDash(r.Team), len(r.Attributes))
				}
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type rowResult struct {
				Email  string `json:"email"`
				Result string `json:"result"` // created|exists|error
				Detail string `json:"detail,omitempty"`
			}
			var results []rowResult

			for _, r := range rows {
				res := rowResult{Email: r.Email}

				// Surface lookup failures instead of swallowing them: a transient
				// error here would otherwise look like "member doesn't exist" and
				// trigger a duplicate create (defeating --idempotent).
				existingID, lookErr := lookupMemberIDViaAPI(cmd, c, r.Email)
				if lookErr != nil {
					res.Result = "error"
					res.Detail = "could not check for existing member: " + lookErr.Error()
					results = append(results, res)
					continue
				}
				memberID := existingID
				if existingID != "" {
					if flagIdempotent {
						res.Result = "exists"
						res.Detail = "member already exists; treated as no-op"
					} else {
						// Without --idempotent an existing member is a hard error,
						// so callers notice unexpected collisions in a batch.
						res.Result = "error"
						res.Detail = "member already exists; pass --idempotent to skip existing members"
						results = append(results, res)
						continue
					}
				} else {
					body := map[string]any{
						"email":     r.Email,
						"firstName": r.FirstName,
						"lastName":  r.LastName,
					}
					if r.MemberType != "" {
						body["memberType"] = r.MemberType
					}
					resp, status, perr := c.Post(cmd.Context(), "/v2/members", body)
					if perr != nil {
						res.Result = "error"
						res.Detail = perr.Error()
						results = append(results, res)
						continue
					}
					if status < 200 || status >= 300 {
						res.Result = "error"
						res.Detail = fmt.Sprintf("status %d: %s", status, string(resp))
						results = append(results, res)
						continue
					}
					res.Result = "created"
					var created map[string]any
					if json.Unmarshal(resp, &created) == nil {
						memberID = firstString(created, "memberId", "id")
					}
					// If the create succeeded but we can't resolve the new id, team
					// and attribute assignment below will be skipped — say so rather
					// than reporting a clean "created".
					if memberID == "" && (r.Team != "" || len(r.Attributes) > 0) {
						res.Detail = strings.TrimSpace(res.Detail + " created, but could not resolve new memberId; team/attributes NOT applied")
					}
				}

				// Assign team if given.
				if r.Team != "" && memberID != "" {
					if err := provisionAssignTeam(cmd, flags, c, memberID, r.Team); err != nil {
						res.Detail = strings.TrimSpace(res.Detail + " team-assign error: " + err.Error())
					}
				}
				// Set user attributes if given.
				for name, val := range r.Attributes {
					if memberID == "" {
						break
					}
					if err := provisionSetAttribute(cmd, c, memberID, name, val); err != nil {
						res.Detail = strings.TrimSpace(res.Detail + " attr-set error: " + err.Error())
					}
				}

				results = append(results, res)
			}

			if wantJSON(flags, cmd) {
				return flags.printJSON(cmd, results)
			}
			for _, r := range results {
				if r.Detail != "" {
					fmt.Fprintf(out, "%s: %s (%s)\n", r.Email, r.Result, r.Detail)
				} else {
					fmt.Fprintf(out, "%s: %s\n", r.Email, r.Result)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Path to the new-hires CSV (columns: email,firstName,lastName,memberType[,team][,attributes])")
	cmd.Flags().BoolVar(&flagIdempotent, "idempotent", false, "Treat an existing member (matched by email) as a no-op instead of an error")
	return cmd
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// parseProvisionCSV parses the new-hires CSV. The first record is the header
// row; columns are matched by name (case-insensitive). Required: email,
// firstName, lastName. Optional: memberType, team, attributes (k=v;k=v).
// Pure function for testability.
func parseProvisionCSV(r io.Reader) ([]provisionRow, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true

	records, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("empty CSV: expected a header row with at least email,firstName,lastName")
	}

	header := records[0]
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, req := range []string{"email", "firstname", "lastname"} {
		if _, ok := idx[req]; !ok {
			return nil, fmt.Errorf("missing required CSV column %q (header has: %s)", req, strings.Join(header, ","))
		}
	}

	get := func(rec []string, col string) string {
		if i, ok := idx[col]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}

	var out []provisionRow
	for lineNo, rec := range records[1:] {
		if len(rec) == 0 || (len(rec) == 1 && strings.TrimSpace(rec[0]) == "") {
			continue
		}
		email := get(rec, "email")
		if email == "" {
			return nil, fmt.Errorf("row %d: missing email", lineNo+2)
		}
		row := provisionRow{
			Email:      email,
			FirstName:  get(rec, "firstname"),
			LastName:   get(rec, "lastname"),
			MemberType: get(rec, "membertype"),
			Team:       get(rec, "team"),
			Attributes: parseAttributes(get(rec, "attributes")),
		}
		out = append(out, row)
	}
	return out, nil
}

// parseAttributes parses a "k=v;k=v" attribute string into a map. Empty input
// yields nil.
func parseAttributes(s string) map[string]string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	out := map[string]string{}
	for _, pair := range strings.Split(s, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		key := strings.TrimSpace(kv[0])
		if key == "" {
			continue
		}
		val := ""
		if len(kv) == 2 {
			val = strings.TrimSpace(kv[1])
		}
		out[key] = val
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// provisionAssignTeam resolves a team name (or id) and adds the member to it
// via PATCH /v2/teams/{teamId}/members {add:[memberId]}.
func provisionAssignTeam(cmd *cobra.Command, flags *rootFlags, c *client.Client, memberID, team string) error {
	teamID := team
	if db, err := openStore(cmd); err == nil {
		defer db.Close()
		var id string
		row := db.DB().QueryRow(`SELECT COALESCE(NULLIF(team_id,''), id) FROM teams WHERE LOWER(name) = LOWER(?) LIMIT 1`, team)
		if row.Scan(&id) == nil && id != "" {
			teamID = id
		}
	}
	body := map[string]any{"add": []string{memberID}}
	resp, status, err := c.Patch(cmd.Context(), fmt.Sprintf("/v2/teams/%s/members", teamID), body)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("status %d: %s", status, string(resp))
	}
	return nil
}

// provisionSetAttribute resolves a user-attribute name to its id and assigns a
// value to the member via POST /v2/user-attributes/{id}/users.
func provisionSetAttribute(cmd *cobra.Command, c *client.Client, memberID, name, val string) error {
	attrID, err := lookupUserAttributeID(cmd, c, name)
	if err != nil {
		return err
	}
	if attrID == "" {
		return fmt.Errorf("no user attribute named %q", name)
	}
	body := map[string]any{
		"assignments": []map[string]any{
			{
				"userId": memberID,
				"value":  map[string]any{"val": val, "type": "string"},
			},
		},
	}
	resp, status, err := c.Post(cmd.Context(), fmt.Sprintf("/v2/user-attributes/%s/users", attrID), body)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("status %d: %s", status, string(resp))
	}
	return nil
}

// lookupUserAttributeID resolves a user-attribute name to its id via a
// paginated scan of GET /v2/user-attributes (so an attribute on any page is
// found, not just page one).
func lookupUserAttributeID(cmd *cobra.Command, c *client.Client, name string) (string, error) {
	entries, err := getAllEntries(cmd, c, "/v2/user-attributes", nil)
	if err != nil {
		return "", err
	}
	want := strings.ToLower(strings.TrimSpace(name))
	for _, e := range entries {
		if strings.ToLower(firstString(e, "name", "attributeName")) == want {
			return firstString(e, "userAttributeId", "attributeId", "id"), nil
		}
	}
	return "", nil
}
