// Copyright 2026 beetz12. Licensed under Apache-2.0.
// Draft-only personalized outreach composed from a caregiver's real care.com
// profile. Hand-authored; safe across regen. SENDING IS NOT IMPLEMENTED here -
// care.com's sendMessage mutation isn't captured yet, and the locked safety
// model is draft-and-confirm only (never blind-send).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// getCaregiver response subset used for personalization.
type careGetCaregiver struct {
	Member struct {
		ID        string `json:"id"`
		LegacyID  string `json:"legacyId"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Address   struct {
			City  string `json:"city"`
			State string `json:"state"`
		} `json:"address"`
	} `json:"member"`
	YearsOfExperience int  `json:"yearsOfExperience"`
	HasCareCheck      bool `json:"hasCareCheck"`
	EducationDegrees  []struct {
		SchoolName     string `json:"schoolName"`
		EducationLevel string `json:"educationLevel"`
	} `json:"educationDegrees"`
	Profiles struct {
		ChildCare struct {
			AgeGroups        []string       `json:"ageGroups"`
			NumberOfChildren int            `json:"numberOfChildren"`
			Qualities        map[string]any `json:"qualities"`
			Rates            []struct {
				HourlyRate struct {
					Amount flexStr `json:"amount"`
				} `json:"hourlyRate"`
			} `json:"rates"`
			Bio flexStr `json:"bio"`
		} `json:"childCareCaregiverProfile"`
		Common struct {
			Bio struct {
				ExperienceSummary string `json:"experienceSummary"`
			} `json:"bio"`
		} `json:"commonCaregiverProfile"`
	} `json:"profiles"`
}

var careAgeGroupLabels = map[string]string{
	"NEWBORN":           "newborns",
	"INFANT":            "infants",
	"TODDLER":           "toddlers",
	"EARLY_SCHOOL":      "preschoolers",
	"ELEMENTARY_SCHOOL": "school-age kids",
	"TEEN":              "teens",
}

// qualities booleans worth calling out, mapped to friendly phrasing.
var careQualityLabels = map[string]string{
	"certifiedNursingAssistant":       "a certified nursing assistant",
	"cprTrained":                      "CPR-trained",
	"firstAidTrained":                 "first-aid trained",
	"earlyChildDevelopmentCoursework": "early-childhood coursework",
	"earlyChildhoodEducation":         "an early-childhood-education background",
	"specialNeedsCare":                "experience with special-needs care",
	"nannyServices":                   "nanny experience",
	"comfortableWithPets":             "comfortable with pets",
	"doesNotSmoke":                    "a non-smoker",
	"ownTransportation":               "her own transportation",
}

func careJoinList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}

// composeCareOutreach builds a warm, specific draft from real profile facts.
func composeCareOutreach(g careGetCaregiver, about, from string) string {
	first := strings.TrimSpace(g.Member.FirstName)
	if first == "" {
		first = "there"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Hi %s,\n\n", first)

	// Opening + experience.
	open := "I came across your care.com profile and really liked what I saw"
	if g.YearsOfExperience > 0 {
		// ASCII-only: em-dashes mojibake when pasted into care.com / Mac apps.
		open += fmt.Sprintf(" - %d years of childcare experience is exactly the kind of depth we're hoping for", g.YearsOfExperience)
	}
	b.WriteString(open + ".\n\n")

	// Age groups.
	var ages []string
	for _, a := range g.Profiles.ChildCare.AgeGroups {
		if lbl, ok := careAgeGroupLabels[strings.ToUpper(a)]; ok {
			ages = append(ages, lbl)
		}
	}
	if len(ages) > 0 {
		fmt.Fprintf(&b, "It's wonderful that you work with %s. ", careJoinList(ages))
	}

	// A couple of standout credentials.
	var creds []string
	// deterministic order for stable output
	var qkeys []string
	for k, v := range g.Profiles.ChildCare.Qualities {
		if bv, ok := v.(bool); ok && bv {
			if lbl, ok := careQualityLabels[k]; ok {
				qkeys = append(qkeys, lbl)
			}
		}
	}
	sort.Strings(qkeys)
	if len(qkeys) > 2 {
		qkeys = qkeys[:2]
	}
	creds = append(creds, qkeys...)
	if len(g.EducationDegrees) > 0 && g.EducationDegrees[0].SchoolName != "" {
		creds = append(creds, "your studies at "+g.EducationDegrees[0].SchoolName)
	}
	if g.HasCareCheck {
		creds = append(creds, "a verified background check")
	}
	if len(creds) > 0 {
		fmt.Fprintf(&b, "A few things that caught my eye: %s.", careJoinList(creds))
	}
	b.WriteString("\n\n")

	// Family need.
	if strings.TrimSpace(about) == "" {
		about = "[Describe your family's needs here - e.g., part-time care for our two young kids, dates/hours, and anything important to you.]"
	}
	b.WriteString(about + "\n\n")

	b.WriteString("Would you be open to connecting to see if we might be a good fit? I'd love to hear about your approach and availability.\n\n")
	if strings.TrimSpace(from) == "" {
		from = "[Your name]"
	}
	fmt.Fprintf(&b, "Warmly,\n%s", from)
	return b.String()
}

func newCareOutreachCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "outreach",
		Short: "Compose outreach to caregivers (draft-only)",
		Long:  "Draft personalized outreach messages to caregivers from their real care.com profile. Sending is intentionally not implemented (draft-and-confirm safety model).",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCareOutreachDraftCmd(flags))
	cmd.AddCommand(newCareOutreachSendCmd(flags))
	return cmd
}

// careResolveCaregiver fetches a caregiver's name + legacy member id (needed as
// the send recipient id).
func careResolveCaregiver(ctx context.Context, flags *rootFlags, caregiverID string) (name, legacyID string, err error) {
	vars := map[string]any{
		"getCaregiverId":           caregiverID,
		"serviceId":                "CHILD_CARE",
		"shouldIncludeAllProfiles": true,
		"shouldGetMarkedAsHired":   false,
	}
	data, err := careGraphQL(ctx, flags, careQGetCaregiverOp, careQGetCaregiver, vars)
	if err != nil {
		return "", "", err
	}
	var wrap struct {
		G careGetCaregiver `json:"getCaregiver"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return "", "", err
	}
	name = strings.TrimSpace(wrap.G.Member.FirstName + " " + firstInitial(wrap.G.Member.LastName))
	return name, wrap.G.Member.LegacyID, nil
}

// careSendMessage sends a message via care.com's official, moderated send path
// (sendCareNeedMessage) - NOT directly to Stream Chat. This keeps care.com's
// trust-and-safety scanning and record-keeping intact.
func careSendMessage(ctx context.Context, flags *rootFlags, jobID, recipientLegacyID, message string) error {
	input := map[string]any{
		"careNeedIdentifier": map[string]any{"id": jobID, "type": "JOB_REQUEST"},
		"message":            message,
		"recipientMemberIds": []string{recipientLegacyID},
	}
	data, err := careGraphQL(ctx, flags, careMSendMessageOp, careMSendMessage, map[string]any{"input": input})
	if err != nil {
		return err
	}
	var res struct {
		Send struct {
			Typename         string   `json:"__typename"`
			TechnicalMessage string   `json:"technicalMessage"`
			FailedRecipients []string `json:"failedRecipientIds"`
			DetectedConcerns []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"detectedConcerns"`
		} `json:"sendCareNeedMessage"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return fmt.Errorf("parsing send result: %w", err)
	}
	if res.Send.Typename == "SendCareNeedMessageSuccess" {
		return nil
	}
	detail := res.Send.TechnicalMessage
	if len(res.Send.DetectedConcerns) > 0 {
		var flagged []string
		for _, c := range res.Send.DetectedConcerns {
			flagged = append(flagged, c.Type)
		}
		detail += fmt.Sprintf(" (care.com flagged your message content: %v - edit and retry)", flagged)
	}
	if len(res.Send.FailedRecipients) > 0 {
		detail += fmt.Sprintf(" (failed: %v)", res.Send.FailedRecipients)
	}
	if detail != "" {
		detail = " - " + detail
	}
	return fmt.Errorf("care.com rejected the send: %s%s", res.Send.Typename, detail)
}

func newCareOutreachSendCmd(flags *rootFlags) *cobra.Command {
	var caregiverID, memberID, jobID, message, messageFile string
	var confirm bool
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a message to a caregiver under one of your jobs (dry-run unless --confirm)",
		Long: "Sends a real message to a caregiver via care.com, in the context of one of your job posts.\n" +
			"SAFETY: dry-run by default - it prints exactly what would be sent and sends NOTHING.\n" +
			"You must pass --confirm to actually send. Provide the caregiver by --to <caregiverId>\n" +
			"(the CLI resolves their member id) or --to-member-id <legacyId> directly.",
		// Examples never carry the confirm flag: a send example would fire for
		// real when copy-pasted or exercised by a live dogfood. The confirm flag
		// is documented in the Long text above; the default is always a preview.
		Example: strings.Trim(`
  # preview only (nothing sent):
  care-pp-cli outreach send --to <caregiverId> --job 12345678 --message "Hi! ..."
  care-pp-cli outreach send --to <caregiverId> --job 12345678 --message-file draft.txt`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if message == "" && messageFile != "" {
				b, err := os.ReadFile(messageFile)
				if err != nil {
					return fmt.Errorf("reading --message-file: %w", err)
				}
				message = strings.TrimSpace(string(b))
			}
			if strings.TrimSpace(message) == "" {
				return usageErr(fmt.Errorf("--message or --message-file is required"))
			}
			if caregiverID == "" && memberID == "" {
				return usageErr(fmt.Errorf("provide --to <caregiverId> or --to-member-id <legacyId>"))
			}
			to := flags.timeout
			if to <= 0 {
				to = 30 * time.Second
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), to)
			defer cancel()

			// Resolve the job (by name or id); defaults to the active job set by `find`.
			job, err := careResolveJob(ctx, flags, jobID)
			if err != nil {
				return err
			}
			jobID = job.ID

			name := "(caregiver " + memberID + ")"
			if caregiverID != "" {
				n, lid, rerr := careResolveCaregiver(ctx, flags, caregiverID)
				if rerr != nil {
					return rerr
				}
				if lid == "" {
					return fmt.Errorf("could not resolve a member id for caregiver %s; pass --to-member-id <legacyId>", caregiverID)
				}
				name, memberID = n, lid
			}

			w := cmd.OutOrStdout()
			// Always show exactly what would be sent.
			fmt.Fprintf(w, "To:      %s (member %s)\n", name, memberID)
			fmt.Fprintf(w, "Job:     %s (%s)\n", job.Title, job.ID)
			fmt.Fprintf(w, "Message:\n%s\n\n", message)

			if !confirm {
				fmt.Fprintln(w, "DRY RUN - nothing sent. Re-run with --confirm to send.")
				return nil
			}

			if err := careSendMessage(ctx, flags, jobID, memberID, message); err != nil {
				return err
			}
			fmt.Fprintf(w, "Sent to %s.\n", name)
			// Record the outreach locally for dedupe. Surface (don't swallow)
			// failures so the user knows the local state is stale after a send.
			if caregiverID == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: contact not recorded locally (sent via --to-member-id; no caregiver id to key on)")
			} else if merr := careMarkContacted(caregiverID); merr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: message sent, but recording the contact locally failed: %v\n", merr)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&caregiverID, "to", "", "caregiver id (UUID from `find`); the CLI resolves their member id")
	cmd.Flags().StringVar(&memberID, "to-member-id", "", "recipient legacy member id (if you already have it)")
	cmd.Flags().StringVar(&jobID, "job", "", "job to message under, by name or id (defaults to the active job set by `find`)")
	cmd.Flags().StringVar(&message, "message", "", "message text")
	cmd.Flags().StringVar(&messageFile, "message-file", "", "read message text from a file")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "actually send (default is dry-run)")
	return cmd
}

func newCareOutreachDraftCmd(flags *rootFlags) *cobra.Command {
	var about, from string
	cmd := &cobra.Command{
		Use:   "draft <caregiverId>",
		Short: "Draft a personalized message to a caregiver (does NOT send)",
		Long:  "Fetches a caregiver's care.com profile and composes a warm, specific draft that references their real experience, the ages they work with, and their credentials. Prints the draft for you to review and send yourself - sending is not wired (draft-and-confirm safety model).",
		Example: strings.Trim(`
  care-pp-cli outreach draft 44444444-4444-4444-4444-444444444444 --from "The Fixture family"
  care-pp-cli outreach draft <id> --about "We need part-time care for our kids for a few days this summer." --from "Alex"`, "\n"),
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			caregiverID := args[0]
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would draft outreach for caregiver %s (no send)\n", caregiverID)
				return nil
			}
			to := flags.timeout
			if to <= 0 {
				to = 30 * time.Second
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), to)
			defer cancel()

			vars := map[string]any{
				"getCaregiverId":           caregiverID,
				"serviceId":                "CHILD_CARE",
				"shouldIncludeAllProfiles": true,
				"shouldGetMarkedAsHired":   false,
			}
			data, err := careGraphQL(ctx, flags, careQGetCaregiverOp, careQGetCaregiver, vars)
			if err != nil {
				return err
			}
			var wrap struct {
				GetCaregiver careGetCaregiver `json:"getCaregiver"`
			}
			if err := json.Unmarshal(data, &wrap); err != nil {
				return fmt.Errorf("parsing caregiver: %w", err)
			}
			if wrap.GetCaregiver.Member.FirstName == "" && wrap.GetCaregiver.YearsOfExperience == 0 {
				return fmt.Errorf("no caregiver found for id %s (check the id, or run: care-pp-cli auth refresh)", caregiverID)
			}
			draft := composeCareOutreach(wrap.GetCaregiver, about, from)

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				out := map[string]any{
					"caregiver_id": caregiverID,
					"name":         strings.TrimSpace(wrap.GetCaregiver.Member.FirstName + " " + firstInitial(wrap.GetCaregiver.Member.LastName)),
					"draft":        draft,
					"sent":         false,
					"note":         "draft only - sending is not implemented (draft-and-confirm safety model)",
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "--- DRAFT for %s %s (NOT sent) ---\n\n", wrap.GetCaregiver.Member.FirstName, firstInitial(wrap.GetCaregiver.Member.LastName))
			fmt.Fprintln(w, draft)
			fmt.Fprintf(w, "\n------------------------------\n")
			fmt.Fprintln(w, "Review and send this yourself in care.com. (CLI sending is not wired: draft-and-confirm safety model.)")
			return nil
		},
	}
	cmd.Flags().StringVar(&about, "about", "", "your family's needs / the role (woven into the draft)")
	cmd.Flags().StringVar(&from, "from", "", "how to sign the message")
	return cmd
}
