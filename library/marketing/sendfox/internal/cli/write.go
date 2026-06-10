package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/config"

	"github.com/spf13/cobra"
)

func newWriteCmd(flags *rootFlags) *cobra.Command {
	var (
		topic          string
		listID         int
		sendNow        bool
		abSubjects     int
		fromTopicsFile string
		scheduleWeekly bool
		tone           string
		length         string
		fromName       string
		fromEmail      string
		model          string
	)

	cmd := &cobra.Command{
		Use:   "write",
		Short: "AI-write a newsletter campaign using Claude",
		Long: `write generates a complete newsletter campaign from a plain-English topic brief.

Claude writes the subject line, preview text, and full HTML email body. The
campaign is created as a draft in SendFox. Use --send to send immediately.

Requires ANTHROPIC_API_KEY environment variable. Get one at console.anthropic.com.`,
		Example: strings.Trim(`
  # Write a draft newsletter on a topic
  sendfox-pp-cli write --topic "5 habits that changed my morning routine"

  # Write and send immediately to list 42
  sendfox-pp-cli write --topic "My year in review" --list 42 --send

  # Get 3 subject line options and pick one
  sendfox-pp-cli write --topic "AI tools every creator needs" --ab-subjects 3

  # Schedule a week of newsletters from a topics file
  sendfox-pp-cli write --from-topics topics.txt --schedule-weekly --list 42

  # Write in a specific tone
  sendfox-pp-cli write --topic "Q2 update" --tone professional --length long`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil_isVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would write campaign")
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if cfg.AuthHeader() == "" {
				return authErr(fmt.Errorf("not configured; run: sendfox-pp-cli setup"))
			}

			anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
			if anthropicKey == "" {
				return fmt.Errorf("ANTHROPIC_API_KEY not set; get one at console.anthropic.com")
			}

			// Resolve sender defaults from config
			if fromName == "" {
				fromName = cfg.FromName
			}
			if fromEmail == "" {
				fromEmail = cfg.FromEmail
			}
			if fromName == "" || fromEmail == "" {
				return fmt.Errorf("sender details not configured; run: sendfox-pp-cli setup (or pass --from-name and --from-email)")
			}
			if listID == 0 {
				listID = cfg.DefaultListID
			}

			w := cmd.OutOrStdout()

			// Batch mode: --from-topics file
			if fromTopicsFile != "" {
				return runBatchWrite(cmd, cfg, anthropicKey, fromTopicsFile, listID, scheduleWeekly,
					fromName, fromEmail, tone, length, model, flags)
			}

			// Single campaign mode
			if topic == "" {
				return fmt.Errorf("--topic is required (or use --from-topics for batch mode)")
			}

			fmt.Fprintf(w, "Writing newsletter on: %q\n", topic)
			fmt.Fprintf(w, "Tone: %s  Length: %s  Model: %s\n", tone, length, model)
			fmt.Fprintln(w, "")

			// Generate content with Claude
			content, genErr := generateCampaignContent(anthropicKey, topic, tone, length, model, abSubjects)
			if genErr != nil {
				return fmt.Errorf("generating content: %w", genErr)
			}

			// If A/B subjects requested, let user pick
			subject := content.Subject
			if abSubjects > 1 && len(content.SubjectVariants) > 0 && !flags.noInput {
				subject, err = pickSubject(w, content.SubjectVariants)
				if err != nil {
					return err
				}
			} else if abSubjects > 1 && len(content.SubjectVariants) > 0 {
				// Non-interactive: print all subjects and use first
				fmt.Fprintln(w, "Subject options (using first in non-interactive mode):")
				for i, s := range content.SubjectVariants {
					fmt.Fprintf(w, "  %d. %s\n", i+1, s.Subject)
				}
				subject = content.SubjectVariants[0].Subject
			}

			fmt.Fprintf(w, "Subject: %s\n", subject)
			fmt.Fprintf(w, "Preview: %s\n", content.PreviewText)
			fmt.Fprintln(w, "")

			// Build campaign payload
			payload := map[string]any{
				"title":        fmt.Sprintf("Campaign: %s", topic),
				"subject":      subject,
				"html":         content.HTML,
				"from_name":    fromName,
				"from_email":   fromEmail,
				"preview_text": content.PreviewText,
			}
			if listID > 0 {
				payload["lists"] = []int{listID}
			}

			// Create draft
			c, cErr := flags.newClient()
			if cErr != nil {
				return cErr
			}

			campaignData, _, apiErr := c.Post("/campaigns", payload)
			if apiErr != nil {
				return classifyAPIError(apiErr, flags)
			}

			var campaign struct {
				ID      int    `json:"id"`
				Title   string `json:"title"`
				Subject string `json:"subject"`
			}
			_ = json.Unmarshal(campaignData, &campaign)

			fmt.Fprintf(w, "Campaign draft created (ID: %d)\n", campaign.ID)

			if sendNow {
				if listID == 0 {
					return fmt.Errorf("--send requires a list; pass --list <id> or configure a default with: sendfox-pp-cli setup")
				}
				fmt.Fprint(w, "Sending... ")
				_, _, sendErr := c.Post(fmt.Sprintf("/campaigns/%d/send", campaign.ID), nil)
				if sendErr != nil {
					return classifyAPIError(sendErr, flags)
				}
				fmt.Fprintln(w, green("Sent!"))
			} else {
				fmt.Fprintf(w, "To review: sendfox-pp-cli campaigns get --id %d\n", campaign.ID)
				fmt.Fprintf(w, "To send:   sendfox-pp-cli campaigns send --id %d\n", campaign.ID)
			}

			if flags.asJSON {
				return printJSONFiltered(w, map[string]any{
					"id":           campaign.ID,
					"subject":      subject,
					"preview_text": content.PreviewText,
					"sent":         sendNow,
					"list_id":      listID,
				}, flags)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "Newsletter topic or brief (plain English)")
	cmd.Flags().IntVar(&listID, "list", 0, "Subscriber list ID to assign (uses config default if not set)")
	cmd.Flags().BoolVar(&sendNow, "send", false, "Send the campaign immediately after creating the draft")
	cmd.Flags().IntVar(&abSubjects, "ab-subjects", 1, "Number of subject line options to generate (1-5)")
	cmd.Flags().StringVar(&fromTopicsFile, "from-topics", "", "File with one topic per line — generates one campaign per topic")
	cmd.Flags().BoolVar(&scheduleWeekly, "schedule-weekly", false, "Schedule batch campaigns 7 days apart (use with --from-topics)")
	cmd.Flags().StringVar(&tone, "tone", "conversational", "Writing tone: conversational, professional, urgent, friendly")
	cmd.Flags().StringVar(&length, "length", "medium", "Email length: short (200-300 words), medium (400-600), long (700-900)")
	cmd.Flags().StringVar(&fromName, "from-name", "", "Sender display name (overrides config default)")
	cmd.Flags().StringVar(&fromEmail, "from-email", "", "Sender email address (overrides config default)")
	cmd.Flags().StringVar(&model, "model", "claude-haiku-4-5-20251001", "Claude model to use for generation")

	return cmd
}

type campaignContent struct {
	Subject         string
	PreviewText     string
	HTML            string
	SubjectVariants []subjectVariant
}

type subjectVariant struct {
	Subject   string `json:"subject"`
	Rationale string `json:"rationale"`
}

func generateCampaignContent(apiKey, topic, tone, length, model string, numSubjects int) (*campaignContent, error) {
	wordCount := map[string]string{
		"short":  "200-300 words",
		"medium": "400-600 words",
		"long":   "700-900 words",
	}[length]
	if wordCount == "" {
		wordCount = "400-600 words"
	}

	subjectInstruction := "a single compelling subject line"
	if numSubjects > 1 {
		subjectInstruction = fmt.Sprintf("%d distinct subject line options, each with a brief rationale", numSubjects)
	}

	prompt := fmt.Sprintf(`You are writing a newsletter email for a content creator.

Topic: %s
Tone: %s
Length: %s

Write the following:
1. Subject line(s): %s
2. Preview text: A 1-sentence inbox preview (max 130 chars, no trailing ellipsis)
3. Email body: Full HTML email body only (no <html>/<head>/<body> wrappers). Use inline styles. Write %s of engaging, %s content. Start directly with the email content — no preamble.

Return ONLY valid JSON in this exact format:
{
  "subject": "Main subject line",
  "subject_variants": [
    {"subject": "Option 1", "rationale": "Why this works"},
    {"subject": "Option 2", "rationale": "Why this works"}
  ],
  "preview_text": "One sentence preview",
  "html": "<p style=\"...\">[full email HTML]</p>"
}

If only 1 subject was requested, subject_variants should have 1 item.`, topic, tone, wordCount, subjectInstruction, wordCount, tone)

	reqBody := map[string]any{
		"model":      model,
		"max_tokens": 4096,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	httpClient := &http.Client{Timeout: 120 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling Claude API: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Claude API error %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing Claude response: %w", err)
	}
	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("empty response from Claude")
	}

	text := apiResp.Content[0].Text
	// Strip markdown code fences if present
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}

	var result struct {
		Subject         string           `json:"subject"`
		SubjectVariants []subjectVariant `json:"subject_variants"`
		PreviewText     string           `json:"preview_text"`
		HTML            string           `json:"html"`
	}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("parsing generated content (Claude returned non-JSON): %w\n\nRaw output:\n%s", err, text[:min(500, len(text))])
	}

	return &campaignContent{
		Subject:         result.Subject,
		PreviewText:     result.PreviewText,
		HTML:            result.HTML,
		SubjectVariants: result.SubjectVariants,
	}, nil
}

func pickSubject(w io.Writer, variants []subjectVariant) (string, error) {
	fmt.Fprintln(w, "Subject line options:")
	for i, v := range variants {
		fmt.Fprintf(w, "  %d. %s\n", i+1, v.Subject)
		if v.Rationale != "" {
			fmt.Fprintf(w, "     %s\n", v.Rationale)
		}
	}
	fmt.Fprint(w, "\nChoose subject (1-"+strconv.Itoa(len(variants))+"): ")
	r := bufio.NewReader(os.Stdin)
	line, _ := readLine(r)
	line = strings.TrimSpace(line)
	idx, err := strconv.Atoi(line)
	if err != nil || idx < 1 || idx > len(variants) {
		return variants[0].Subject, nil
	}
	return variants[idx-1].Subject, nil
}

func runBatchWrite(cmd *cobra.Command, cfg *config.Config, anthropicKey, topicsFile string,
	listID int, scheduleWeekly bool, fromName, fromEmail, tone, length, model string, flags *rootFlags) error {

	data, err := os.ReadFile(topicsFile)
	if err != nil {
		return fmt.Errorf("reading topics file %s: %w", topicsFile, err)
	}

	var topics []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			topics = append(topics, line)
		}
	}
	if len(topics) == 0 {
		return fmt.Errorf("no topics found in %s", topicsFile)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Generating %d campaigns from %s...\n\n", len(topics), topicsFile)

	c, err := flags.newClient()
	if err != nil {
		return err
	}

	var scheduleBase time.Time
	if scheduleWeekly {
		scheduleBase = time.Now().UTC().Add(24 * time.Hour)
	}

	for i, topic := range topics {
		fmt.Fprintf(w, "[%d/%d] Writing: %q\n", i+1, len(topics), topic)

		content, genErr := generateCampaignContent(anthropicKey, topic, tone, length, model, 1)
		if genErr != nil {
			fmt.Fprintf(w, "  ERROR: %v\n", genErr)
			continue
		}

		payload := map[string]any{
			"title":        fmt.Sprintf("Campaign: %s", topic),
			"subject":      content.Subject,
			"html":         content.HTML,
			"from_name":    fromName,
			"from_email":   fromEmail,
			"preview_text": content.PreviewText,
		}
		if listID > 0 {
			payload["lists"] = []int{listID}
		}
		if scheduleWeekly && listID > 0 {
			scheduledAt := scheduleBase.Add(time.Duration(i) * 7 * 24 * time.Hour)
			payload["scheduled_at"] = scheduledAt.Format(time.RFC3339)
			fmt.Fprintf(w, "  Scheduled: %s\n", scheduledAt.Format("Mon Jan 2, 2006 at 15:04 UTC"))
		}

		campaignData, _, apiErr := c.Post("/campaigns", payload)
		if apiErr != nil {
			fmt.Fprintf(w, "  ERROR creating campaign: %v\n", apiErr)
			continue
		}

		var campaign struct {
			ID int `json:"id"`
		}
		_ = json.Unmarshal(campaignData, &campaign)
		fmt.Fprintf(w, "  Created campaign ID %d — subject: %s\n", campaign.ID, content.Subject)

		// Small delay to respect rate limits
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Fprintf(w, "\nDone. %d campaigns created.\n", len(topics))
	fmt.Fprintln(w, "Review them: sendfox-pp-cli campaigns list")
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
