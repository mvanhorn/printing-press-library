// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: diff live calendar reality across all accounts against the
// approved calendars.yaml. Hand-implemented.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/gauth"
	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/manifest"
	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/verdict"
)

// Finding kinds for manifest check. Findings drive exit code 3; the
// unmanifested list and timezone notes are informational.
const (
	findingMissingUpstream  = "missing_upstream"
	findingRoleInsufficient = "role_insufficient"
)

type manifestFinding struct {
	Kind       string `json:"kind"`
	Account    string `json:"account"`
	Calendar   string `json:"calendar"`
	Declared   string `json:"declared_role,omitempty"`
	AccessRole string `json:"access_role,omitempty"`
	Detail     string `json:"detail"`
}

type unmanifestedCalendar struct {
	Account    string `json:"account"`
	Calendar   string `json:"calendar"`
	Summary    string `json:"summary,omitempty"`
	AccessRole string `json:"access_role"`
	Primary    bool   `json:"primary,omitempty"`
}

type accountTimezone struct {
	Account  string `json:"account"`
	Timezone string `json:"timezone"`
	Error    string `json:"error,omitempty"`
}

type manifestCheckOutput struct {
	Findings     []manifestFinding      `json:"findings"`
	Unmanifested []unmanifestedCalendar `json:"unmanifested"`
	Timezones    []accountTimezone      `json:"timezones"`
	TZMismatch   bool                   `json:"tz_mismatch"`
	Coverage     verdict.Coverage       `json:"coverage"`
}

// calendarListEntry mirrors the calendarList item fields the check consumes.
type calendarListEntry struct {
	ID         string `json:"id"`
	Summary    string `json:"summary"`
	AccessRole string `json:"accessRole"`
	Primary    bool   `json:"primary"`
	Deleted    bool   `json:"deleted"`
}

type calendarListPage struct {
	Etag          string              `json:"etag"`
	NextPageToken string              `json:"nextPageToken"`
	Items         []calendarListEntry `json:"items"`
}

// fetchCalendarList pulls the account's full calendarList via the no-cache
// read path (its result feeds a verdict).
func fetchCalendarList(cmd *cobra.Command, g verdictGetter, account string) ([]calendarListEntry, verdict.Source) {
	src := verdict.Source{
		Account:   account,
		Calendar:  "calendarList",
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}
	params := map[string]string{"maxResults": "250", "showHidden": "true"}
	var items []calendarListEntry
	for page := 0; ; page++ {
		if page >= verdictMaxPages {
			src.Error = fmt.Sprintf("calendarList pagination exceeded %d pages", verdictMaxPages)
			return nil, src
		}
		raw, err := g.GetNoCache(cmd.Context(), "/users/me/calendarList", params)
		if err != nil {
			src.Error = err.Error()
			return nil, src
		}
		var pageBody calendarListPage
		if err := json.Unmarshal(raw, &pageBody); err != nil {
			src.Error = fmt.Sprintf("unparseable calendarList response: %v", err)
			return nil, src
		}
		if page == 0 {
			src.EtagPresent = pageBody.Etag != ""
		}
		items = append(items, pageBody.Items...)
		if pageBody.NextPageToken == "" {
			break
		}
		params["pageToken"] = pageBody.NextPageToken
	}
	return items, src
}

// roleSatisfied reports whether a live accessRole covers a declared manifest
// role: write needs writer|owner; read needs at least freeBusyReader.
func roleSatisfied(declared, accessRole string) bool {
	switch declared {
	case manifest.RoleWrite:
		return accessRole == "writer" || accessRole == "owner"
	case manifest.RoleRead:
		return accessRole == "freeBusyReader" || accessRole == "reader" || accessRole == "writer" || accessRole == "owner"
	}
	return false
}

// skeletonRole maps a live accessRole to the manifest role it supports.
func skeletonRole(accessRole string) string {
	if accessRole == "writer" || accessRole == "owner" {
		return manifest.RoleWrite
	}
	return manifest.RoleRead
}

// pp:data-source live
func newNovelManifestCheckCmd(flags *rootFlags) *cobra.Command {
	var flagEmitSkeleton bool

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Diff live calendar reality across all accounts against the approved calendars.yaml",
		Long: `For every gauth profile, lists the live calendarList and diffs it against
calendars.yaml:

  findings (exit 3): manifest entries missing upstream, and entries whose
    live accessRole no longer satisfies the declared role (write needs
    writer|owner; read needs at least freeBusyReader).
  unmanifested (informational): live calendars not in the manifest — new
    calendars surface here until the operator approves them into the manifest.
  timezones: each account's calendar timezone setting, with a mismatch
    note when accounts disagree.

--emit-skeleton prints a ready-to-edit calendars.yaml built from live
discovery to stdout (redirect it into the gauth config dir), skipping the
diff. Exit codes: 0 clean, 3 findings present, 4 no findings but at least
one account could not be read (partial coverage).`,
		Example: `  google-calendar-pp-cli manifest check
  google-calendar-pp-cli manifest check --json
  google-calendar-pp-cli manifest check --emit-skeleton > ~/.config/google-calendar-pp-cli/gauth/calendars.yaml`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3,4"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dir := gauth.ConfigDir(flags.authDir)
			profiles, err := gauth.LoadProfiles(dir)
			if err != nil {
				return err
			}
			names := make([]string, 0, len(profiles))
			for _, p := range profiles {
				names = append(names, p.Name)
			}
			var m *manifest.Manifest
			if !flagEmitSkeleton {
				m, err = manifest.LoadValidated(dir, names)
				if err != nil {
					return err
				}
			}

			liveByAccount := map[string][]calendarListEntry{}
			var sources []verdict.Source
			timezones := make([]accountTimezone, 0, len(profiles))
			for _, p := range profiles {
				c, cerr := flags.clientFor(p.Name)
				if cerr != nil {
					sources = append(sources, verdict.Source{
						Account:   p.Name,
						Calendar:  "calendarList",
						FetchedAt: time.Now().UTC().Format(time.RFC3339),
						Error:     cerr.Error(),
					})
					timezones = append(timezones, accountTimezone{Account: p.Name, Error: cerr.Error()})
					continue
				}
				items, src := fetchCalendarList(cmd, c, p.Name)
				sources = append(sources, src)
				if src.Error == "" {
					liveByAccount[p.Name] = items
				}
				tz := accountTimezone{Account: p.Name}
				if raw, terr := c.GetNoCache(cmd.Context(), "/users/me/settings/"+url.PathEscape("timezone"), nil); terr != nil {
					tz.Error = terr.Error()
				} else {
					var setting struct {
						Value string `json:"value"`
					}
					if uerr := json.Unmarshal(raw, &setting); uerr != nil {
						tz.Error = fmt.Sprintf("unparseable timezone setting: %v", uerr)
					} else {
						tz.Timezone = setting.Value
					}
				}
				timezones = append(timezones, tz)
			}
			cov := verdict.BuildCoverage(sources)

			if flagEmitSkeleton {
				skel := manifest.Manifest{}
				for _, p := range profiles {
					items, ok := liveByAccount[p.Name]
					if !ok {
						continue
					}
					for _, item := range items {
						if item.Deleted {
							continue
						}
						skel.Calendars = append(skel.Calendars, manifest.Entry{
							Account: p.Name,
							ID:      item.ID,
							Role:    skeletonRole(item.AccessRole),
							Note:    strings.TrimSpace(fmt.Sprintf("%s (accessRole=%s)", item.Summary, item.AccessRole)),
						})
					}
				}
				b, merr := yaml.Marshal(skel)
				if merr != nil {
					return fmt.Errorf("marshaling skeleton: %w", merr)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "# calendars.yaml skeleton from live discovery (%s)\n# Review roles and prune before adopting: role write requires writer|owner upstream.\n%s", time.Now().UTC().Format(time.RFC3339), b)
				if !cov.Complete {
					return exitDegraded("skeleton is INCOMPLETE: %d/%d accounts read — do not adopt it as-is", cov.Checked, cov.Of)
				}
				return nil
			}

			out := manifestCheckOutput{
				Findings:     []manifestFinding{},
				Unmanifested: []unmanifestedCalendar{},
				Timezones:    timezones,
				Coverage:     cov,
			}
			manifested := map[string]map[string]manifest.Entry{}
			for _, e := range m.Calendars {
				if manifested[e.Account] == nil {
					manifested[e.Account] = map[string]manifest.Entry{}
				}
				manifested[e.Account][e.ID] = e
			}
			for _, e := range m.Calendars {
				live, ok := liveByAccount[e.Account]
				if !ok {
					continue // account read failed → coverage already degrades; a missing_upstream claim here would be a false positive
				}
				var found *calendarListEntry
				for i := range live {
					if live[i].ID == e.ID && !live[i].Deleted {
						found = &live[i]
						break
					}
				}
				if found == nil {
					out.Findings = append(out.Findings, manifestFinding{
						Kind:     findingMissingUpstream,
						Account:  e.Account,
						Calendar: e.ID,
						Declared: e.Role,
						Detail:   "manifested calendar absent from the live calendarList (removed, unshared, or deleted)",
					})
					continue
				}
				if !roleSatisfied(e.Role, found.AccessRole) {
					out.Findings = append(out.Findings, manifestFinding{
						Kind:       findingRoleInsufficient,
						Account:    e.Account,
						Calendar:   e.ID,
						Declared:   e.Role,
						AccessRole: found.AccessRole,
						Detail:     fmt.Sprintf("declared role %q needs more than live accessRole %q", e.Role, found.AccessRole),
					})
				}
			}
			for _, p := range profiles {
				live, ok := liveByAccount[p.Name]
				if !ok {
					continue
				}
				for _, item := range live {
					if item.Deleted {
						continue
					}
					if _, ok := manifested[p.Name][item.ID]; !ok {
						out.Unmanifested = append(out.Unmanifested, unmanifestedCalendar{
							Account:    p.Name,
							Calendar:   item.ID,
							Summary:    item.Summary,
							AccessRole: item.AccessRole,
							Primary:    item.Primary,
						})
					}
				}
			}
			tzSeen := map[string]bool{}
			for _, tz := range out.Timezones {
				if tz.Timezone != "" {
					tzSeen[tz.Timezone] = true
				}
			}
			out.TZMismatch = len(tzSeen) > 1
			sort.SliceStable(out.Findings, func(i, j int) bool {
				if out.Findings[i].Account != out.Findings[j].Account {
					return out.Findings[i].Account < out.Findings[j].Account
				}
				return out.Findings[i].Calendar < out.Findings[j].Calendar
			})

			if err := emitVerdict(cmd, flags, out, func(w io.Writer) {
				if len(out.Findings) == 0 {
					fmt.Fprintln(w, "manifest clean: every entry present upstream with sufficient access")
				}
				for _, f := range out.Findings {
					fmt.Fprintf(w, "FINDING %-18s %s/%s: %s\n", f.Kind, f.Account, f.Calendar, f.Detail)
				}
				for _, u := range out.Unmanifested {
					fmt.Fprintf(w, "unmanifested: %s/%s %q (accessRole=%s)\n", u.Account, u.Calendar, u.Summary, u.AccessRole)
				}
				for _, tz := range out.Timezones {
					if tz.Error != "" {
						fmt.Fprintf(w, "timezone %s: unavailable (%s)\n", tz.Account, tz.Error)
					} else {
						fmt.Fprintf(w, "timezone %s: %s\n", tz.Account, tz.Timezone)
					}
				}
				if out.TZMismatch {
					fmt.Fprintln(w, "note: accounts disagree on calendar timezone — cross-account wall-clock reasoning needs care")
				}
				fmt.Fprintln(w, coverageSummary(cov))
				coverageErrorLines(w, cov)
			}); err != nil {
				return err
			}
			if len(out.Findings) > 0 {
				return exitFindings("%d manifest finding(s) (coverage %d/%d)", len(out.Findings), cov.Checked, cov.Of)
			}
			if !cov.Complete {
				return exitDegraded("no findings, but coverage incomplete (%d/%d accounts read)", cov.Checked, cov.Of)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagEmitSkeleton, "emit-skeleton", false, "Print a calendars.yaml skeleton built from live discovery to stdout instead of diffing")
	return cmd
}
