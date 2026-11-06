// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/cliutil"

	"github.com/spf13/cobra"
)

type onboardInput struct {
	Email string
	Name  string
	Tags  []string
}

type onboardPerson struct {
	Email          string   `json:"email"`
	Name           string   `json:"name,omitempty"`
	Action         string   `json:"action"`
	Spaces         []int64  `json:"spaces"`
	SpaceGroups    []int64  `json:"space_groups"`
	AccessGroups   []int64  `json:"access_groups"`
	Tags           []string `json:"tags"`
	SendInvitation bool     `json:"send_invitation"`
}

type onboardFailure struct {
	Email string `json:"email"`
	Step  string `json:"step"`
	Error string `json:"error"`
}

type onboardPlan struct {
	DryRun           bool             `json:"dry_run"`
	Applied          bool             `json:"applied"`
	People           []onboardPerson  `json:"people"`
	TagsToCreate     []string         `json:"tags_to_create"`
	TagsToCreateNote string           `json:"tags_to_create_note,omitempty"`
	Failures         []onboardFailure `json:"failures,omitempty"`
}

func newNovelOnboardCmd(flags *rootFlags) *cobra.Command {
	var csvFile, name string
	var tags []string
	var spaces, spaceGroups, accessGroups []int64
	var send bool
	cmd := &cobra.Command{
		Use:   "onboard [email...]",
		Short: "Add people to the community, their spaces, space groups and access groups, and tag them",
		Example: strings.Trim(`
  circle-pp-cli onboard ada@example.com --tag founding --space 1000969
  circle-pp-cli onboard --from-csv founding.csv --tag founding --yes
`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "live", "mcp:read-flags": "from-csv",
			"pp:preview-happy-path": "true", "pp:happy-args": "email=test-member@example.com",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "onboard")
			}
			invalid := func(err error) error { _ = cmd.Usage(); return usageErr(err) }
			if len(args) == 0 && strings.TrimSpace(csvFile) == "" {
				return invalid(fmt.Errorf("provide at least one email or --from-csv file"))
			}
			if send && cliutil.IsAnyHarness() {
				if err := writeHarnessRefusal(cmd.OutOrStdout(), flags, "onboard --send"); err != nil {
					return err
				}
				return usageErr(fmt.Errorf("onboard --send is refused under a Printing Press harness"))
			}
			for _, ids := range [][]int64{spaces, spaceGroups, accessGroups} {
				for _, id := range ids {
					if id <= 0 {
						return invalid(fmt.Errorf("membership IDs must be positive"))
					}
				}
			}
			people := make([]onboardInput, 0)
			for _, email := range args {
				people = append(people, onboardInput{Email: email, Name: name})
			}
			if csvFile != "" {
				f, err := os.Open(csvFile)
				if err != nil {
					return err
				}
				rows, parseErr := parseOnboardCSV(f)
				closeErr := f.Close()
				if parseErr != nil {
					return invalid(parseErr)
				}
				if closeErr != nil {
					return closeErr
				}
				people = append(people, rows...)
			}
			people, err := dedupeOnboardPeople(people)
			if err != nil {
				return invalid(err)
			}
			if len(people) == 0 {
				return invalid(fmt.Errorf("no emails found in the supplied inputs"))
			}
			for i := range people {
				if people[i].Name == "" {
					people[i].Name = name
				}
				people[i].Tags = onboardUniqueTags(append(people[i].Tags, tags...))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Plans must reflect current members and tags rather than cached responses.
			c.NoCache = true
			existing := make(map[string]bool)
			ready := make([]onboardInput, 0, len(people))
			failures := make([]onboardFailure, 0)
			for _, person := range people {
				raw, err := c.Get(ctx, "/community_members/search", onboardMemberSearchParams(person.Email))
				var apiErr *client.APIError
				if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
					ready = append(ready, person)
					continue
				}
				if err == nil {
					err = checkOnboardResponse(raw)
				}
				if err != nil {
					failures = append(failures, onboardFailure{Email: person.Email, Step: "search member", Error: err.Error()})
					continue
				}
				existing[strings.ToLower(person.Email)] = true
				ready = append(ready, person)
			}
			tagIDs, err := fetchOnboardTags(ctx, c)
			if err != nil {
				return err
			}
			plan := buildOnboardPlan(ready, existing, tagIDs, spaces, spaceGroups, accessGroups, send)
			plan.Failures = failures
			if flags.yes {
				applyOnboardPlan(ctx, c, &plan, tagIDs)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				err = printJSONFiltered(cmd.OutOrStdout(), plan, flags)
			} else {
				rows := make([][]string, 0, len(plan.People))
				for _, person := range plan.People {
					rows = append(rows, []string{person.Email, person.Action, fmt.Sprint(person.Spaces), fmt.Sprint(person.SpaceGroups), fmt.Sprint(person.AccessGroups), strings.Join(person.Tags, ", "), strconv.FormatBool(person.SendInvitation)})
				}
				err = flags.printTable(cmd, []string{"Email", "Action", "Spaces", "Space groups", "Access groups", "Tags", "Send invitation"}, rows)
				if err == nil && len(plan.TagsToCreate) > 0 {
					_, err = fmt.Fprintln(cmd.OutOrStdout(), "Tags to create (created as private tags):", strings.Join(plan.TagsToCreate, ", "))
				}
				if err == nil && !flags.yes {
					_, err = fmt.Fprintln(cmd.OutOrStdout(), "Re-run with --yes to apply.")
				}
				for _, failure := range plan.Failures {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s (%s): %s\n", failure.Email, failure.Step, failure.Error)
				}
			}
			if err != nil {
				return err
			}
			if len(plan.Failures) > 0 {
				return fmt.Errorf("onboarding failed for one or more steps; see failures")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&csvFile, "from-csv", "", "Read people from a CSV file with email, name, and semicolon-separated tags columns.")
	cmd.Flags().StringVar(&name, "name", "", "Use this display name when no CSV name is supplied.")
	cmd.Flags().Int64SliceVar(&spaces, "space", nil, "Add each person to these space IDs.")
	cmd.Flags().Int64SliceVar(&spaceGroups, "space-group", nil, "Add each person to these space group IDs.")
	cmd.Flags().Int64SliceVar(&accessGroups, "access-group", nil, "Add each person to these access group IDs.")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "Apply these tag names to each person, creating missing tags on apply.")
	cmd.Flags().BoolVar(&send, "send", false, "Send Circle invitation emails to new members when applying with --yes.")
	return cmd
}

func onboardMemberSearchParams(email string) map[string]string {
	// Silent onboarding creates inactive members, which the default search excludes.
	return map[string]string{"email": email, "status": "all"}
}

func parseOnboardCSV(r io.Reader) ([]onboardInput, error) {
	reader := csv.NewReader(r)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}
	columns := make(map[string]int)
	for i, field := range header {
		key := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(field, "\ufeff")))
		if _, ok := columns[key]; ok {
			return nil, fmt.Errorf("duplicate CSV header %q", key)
		}
		columns[key] = i
	}
	if _, ok := columns["email"]; !ok {
		return nil, fmt.Errorf("CSV requires an email column")
	}
	people := make([]onboardInput, 0)
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV row: %w", err)
		}
		if strings.TrimSpace(strings.Join(row, "")) == "" {
			continue
		}
		value := func(key string) string {
			if i, ok := columns[key]; ok {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
		person := onboardInput{Email: value("email"), Name: value("name"), Tags: onboardUniqueTags(strings.Split(value("tags"), ";"))}
		if err := validateOnboardEmail(person.Email); err != nil {
			return nil, err
		}
		people = append(people, person)
	}
	return people, nil
}

func validateOnboardEmail(email string) error {
	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || strings.ContainsAny(email, " \t\r\n") || !strings.Contains(parts[1], ".") || strings.HasPrefix(parts[1], ".") || strings.HasSuffix(parts[1], ".") {
		return fmt.Errorf("invalid email %q", email)
	}
	return nil
}

func onboardUniqueTags(tags []string) []string {
	result := make([]string, 0)
	seen := make(map[string]bool)
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		key := strings.ToLower(tag)
		if tag != "" && !seen[key] {
			result = append(result, tag)
			seen[key] = true
		}
	}
	return result
}

func dedupeOnboardPeople(people []onboardInput) ([]onboardInput, error) {
	result := make([]onboardInput, 0)
	seen := make(map[string]int)
	for _, person := range people {
		person.Email = strings.TrimSpace(person.Email)
		if err := validateOnboardEmail(person.Email); err != nil {
			return nil, err
		}
		key := strings.ToLower(person.Email)
		if i, ok := seen[key]; ok {
			result[i].Tags = onboardUniqueTags(append(result[i].Tags, person.Tags...))
			if result[i].Name == "" {
				result[i].Name = person.Name
			}
		} else {
			seen[key] = len(result)
			person.Tags = onboardUniqueTags(person.Tags)
			result = append(result, person)
		}
	}
	return result, nil
}

func buildOnboardPlan(inputs []onboardInput, existing map[string]bool, tagIDs map[string]int64, spaces, groups, access []int64, send bool) onboardPlan {
	plan := onboardPlan{People: make([]onboardPerson, 0, len(inputs)), TagsToCreate: make([]string, 0)}
	for _, input := range inputs {
		action := "create"
		if existing[strings.ToLower(input.Email)] {
			action = "update-memberships"
		}
		tags := onboardUniqueTags(input.Tags)
		plan.People = append(plan.People, onboardPerson{
			Email: input.Email, Name: input.Name, Action: action,
			Spaces: append(make([]int64, 0), spaces...), SpaceGroups: append(make([]int64, 0), groups...), AccessGroups: append(make([]int64, 0), access...),
			Tags: tags, SendInvitation: send && action == "create",
		})
		for _, tag := range tags {
			if tagIDs[strings.ToLower(tag)] == 0 {
				plan.TagsToCreate = append(plan.TagsToCreate, tag)
			}
		}
	}
	plan.TagsToCreate = onboardUniqueTags(plan.TagsToCreate)
	if len(plan.TagsToCreate) > 0 {
		plan.TagsToCreateNote = "(created as private tags)"
	}
	return plan
}

func checkOnboardResponse(raw json.RawMessage) error {
	var result struct {
		Success *bool  `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return err
	}
	if result.Success != nil && !*result.Success {
		return fmt.Errorf("circle API: %s", result.Message)
	}
	return nil
}

func fetchOnboardTags(ctx context.Context, c *client.Client) (map[string]int64, error) {
	tags := make(map[string]int64)
	for page := 1; ; page++ {
		raw, err := c.Get(ctx, "/member_tags", map[string]string{"page": strconv.Itoa(page), "per_page": "100"})
		if err != nil {
			return nil, err
		}
		if err := checkOnboardResponse(raw); err != nil {
			return nil, err
		}
		var result struct {
			HasNextPage *bool `json:"has_next_page"`
			Records     []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"records"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, err
		}
		if result.Records == nil || result.HasNextPage == nil {
			return nil, fmt.Errorf("member tags response is missing records or has_next_page")
		}
		if err := validateOnboardTagPage(len(result.Records), *result.HasNextPage); err != nil {
			return nil, err
		}
		for _, tag := range result.Records {
			if tag.ID <= 0 || strings.TrimSpace(tag.Name) == "" {
				return nil, fmt.Errorf("invalid member tag in Circle response")
			}
			tags[strings.ToLower(tag.Name)] = tag.ID
		}
		if !*result.HasNextPage {
			return tags, nil
		}
	}
}

func validateOnboardTagPage(count int, hasNextPage bool) error {
	if count == 0 && hasNextPage {
		return fmt.Errorf("member tags response has an empty page with has_next_page=true")
	}
	return nil
}

func parseOnboardTagID(raw json.RawMessage) (int64, error) {
	var result struct {
		ID        int64 `json:"id"`
		MemberTag struct {
			ID int64 `json:"id"`
		} `json:"member_tag"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return 0, err
	}
	if result.ID > 0 {
		return result.ID, nil
	}
	if result.MemberTag.ID > 0 {
		return result.MemberTag.ID, nil
	}
	return 0, fmt.Errorf("create tag returned no ID")
}

func applyOnboardPlan(ctx context.Context, c *client.Client, plan *onboardPlan, tagIDs map[string]int64) {
	tagErrors := make(map[string]error)
	post := func(endpoint string, body any) (json.RawMessage, error) {
		raw, _, err := c.Post(ctx, endpoint, body)
		if err == nil {
			err = checkOnboardResponse(raw)
		}
		if err == nil {
			plan.Applied = true
		}
		return raw, err
	}
	for _, tag := range plan.TagsToCreate {
		raw, err := post("/member_tags", onboardTagBody(tag))
		var id int64
		if err == nil {
			id, err = parseOnboardTagID(raw)
		}
		if err != nil {
			tagErrors[strings.ToLower(tag)] = err
		} else {
			tagIDs[strings.ToLower(tag)] = id
		}
	}
	for _, person := range plan.People {
		fail := func(step string, err error) {
			plan.Failures = append(plan.Failures, onboardFailure{Email: person.Email, Step: step, Error: err.Error()})
		}
		write := func(step, endpoint string, body any) bool {
			_, err := post(endpoint, body)
			if err != nil {
				fail(step, err)
			}
			return err == nil
		}
		ids := make([]int64, 0, len(person.Tags))
		missingTag := false
		for _, tag := range person.Tags {
			if err := tagErrors[strings.ToLower(tag)]; err != nil {
				fail("create tag "+tag, err)
				missingTag = true
			} else {
				ids = append(ids, tagIDs[strings.ToLower(tag)])
			}
		}
		if person.Action == "create" {
			// Do not create a partially configured member when a required tag failed.
			if missingTag {
				continue
			}
			body := map[string]any{"email": person.Email, "skip_invitation": !person.SendInvitation, "space_ids": person.Spaces, "space_group_ids": person.SpaceGroups, "member_tag_ids": ids}
			if person.Name != "" {
				body["name"] = person.Name
			}
			if !write("create member", "/community_members", body) {
				continue
			}
		} else {
			for _, id := range person.Spaces {
				write(fmt.Sprintf("add space %d", id), "/space_members", map[string]any{"email": person.Email, "space_id": id})
			}
			for _, id := range person.SpaceGroups {
				write(fmt.Sprintf("add space group %d", id), "/space_group_members", map[string]any{"email": person.Email, "space_group_id": id})
			}
			for _, id := range ids {
				write(fmt.Sprintf("apply tag %d", id), "/tagged_members", map[string]any{"user_email": person.Email, "member_tag_id": id})
			}
		}
		for _, id := range person.AccessGroups {
			write(fmt.Sprintf("add access group %d", id), "/access_groups/"+strconv.FormatInt(id, 10)+"/community_members", map[string]any{"email": person.Email})
		}
	}
}

func onboardTagBody(name string) map[string]any {
	return map[string]any{"name": name, "display_format": "label", "is_public": false}
}
