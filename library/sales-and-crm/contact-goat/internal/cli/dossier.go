// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// dossier: unified single-person view built from LinkedIn + Happenstance +
// (optionally) Deepline. Composes a single JSON document with per-source
// sub-objects. Caches the composed dossier for 7 days by default.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/store"

	"github.com/spf13/cobra"
)

const dossierTTL = 7 * 24 * time.Hour

type dossierOutput struct {
	Person       flagshipPerson  `json:"person"`
	LinkedIn     json.RawMessage `json:"linkedin,omitempty"`
	Happenstance json.RawMessage `json:"happenstance,omitempty"`
	Deepline     json.RawMessage `json:"deepline,omitempty"`
	CachedAt     string          `json:"cached_at,omitempty"`
	Source       string          `json:"source,omitempty"`
}

func newDossierCmd(flags *rootFlags) *cobra.Command {
	var (
		sections    []string
		noCache     bool
		enrichEmail bool
		deeplineKey string
	)
	cmd := &cobra.Command{
		Use:   "dossier <person>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Build a unified person dossier across LinkedIn, Happenstance, and Deepline",
		Long: `Compose a single view of a person from all configured sources.

Sections (default all free):
  profile   - LinkedIn get_person_profile with all blocks
  research  - Happenstance research/history entry if cached, else POST
  email     - Deepline person-enrich (requires --enrich-email, costs credits)

Dossiers are cached in the local people table for 7 days unless --no-cache
is passed. Pass --enrich-email to spend Deepline credits for email/phone.`,
		Example: `  contact-goat-pp-cli dossier https://www.linkedin.com/in/patrickcollison
  contact-goat-pp-cli dossier "Patrick Collison" --sections profile,research
  contact-goat-pp-cli dossier williamhgates --enrich-email --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			want := parseSections(sections)

			// Preflight: if we're about to reach Deepline, fail fast when the
			// key is missing rather than letting the dossier run through
			// LinkedIn + Happenstance before surfacing the auth gap.
			if shouldPreflightDossier(sections, enrichEmail) {
				if key, _ := resolveDeeplineKey(deeplineKey); key == "" {
					return authErr(fmt.Errorf("dossier --enrich-email needs DEEPLINE_API_KEY (set env or pass --deepline-key)\nhint: keys at https://code.deepline.com/dashboard/api-keys\n      or run 'deepline auth status --reveal' if you've already authenticated with the Deepline CLI"))
				}
			}

			ctx, cancel := signalCtx(cmd.Context())
			defer cancel()

			// Check cache.
			if !noCache {
				if cached, err := loadCachedDossier(target); err == nil && cached != nil {
					cached.Source = "cache"
					return emitDossier(cmd, flags, cached)
				}
			}

			out := &dossierOutput{Source: "live"}

			// Resolve person (LinkedIn URL or name).
			if want["profile"] {
				resolved, raw, err := resolveWarmIntroTarget(ctx, target, "auto")
				if err != nil {
					return err
				}
				out.Person = resolved
				if len(raw) > 0 {
					out.LinkedIn = raw
				}
			} else {
				// User doesn't want profile: synthesize a stub.
				out.Person = flagshipPerson{Name: target, LinkedInURL: maybeLIURL(target)}
			}

			// Parallel: Happenstance research lookup + optional Deepline enrich.
			var wg sync.WaitGroup
			var hpErr, dlErr error
			var hpRaw, dlRaw json.RawMessage

			if want["research"] {
				wg.Add(1)
				go func() {
					defer wg.Done()
					c, err := flags.newClientRequireCookies("happenstance")
					if err != nil {
						hpErr = err
						return
					}
					// Try to find an existing research entry mentioning the name.
					r, err := findHappenstanceResearch(c, out.Person.Name, out.Person.LinkedInURL)
					if err != nil {
						hpErr = err
						return
					}
					hpRaw = r
				}()
			}

			if want["email"] && enrichEmail {
				wg.Add(1)
				go func() {
					defer wg.Done()
					key, _ := resolveDeeplineKey(deeplineKey)
					if key == "" {
						dlErr = errors.New("no Deepline API key")
						return
					}
					if !flags.yes {
						dlErr = errors.New("dossier --enrich-email requires --yes (Deepline charges credits)")
						return
					}
					if out.Person.LinkedInURL == "" {
						dlErr = errors.New("dossier --enrich-email requires a LinkedIn URL target")
						return
					}
					raw, _, err := deeplinePersonEnrich(ctx, key, out.Person.LinkedInURL)
					if err != nil {
						dlErr = err
						return
					}
					dlRaw = raw
				}()
			}

			wg.Wait()
			if hpErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: Happenstance research lookup failed: %v\n", hpErr)
			}
			if dlErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: Deepline enrich failed: %v\n", dlErr)
			}
			if len(hpRaw) > 0 {
				out.Happenstance = hpRaw
			}
			if len(dlRaw) > 0 {
				out.Deepline = dlRaw
			}

			out.CachedAt = nowISO()
			// Persist a condensed Person row + cache the full dossier.
			if !noCache {
				persistDossier(out)
			}
			return emitDossier(cmd, flags, out)
		},
	}
	cmd.Flags().StringSliceVar(&sections, "sections", []string{"profile", "research", "email"}, "Sections to include: profile,research,email")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Ignore cached dossiers and overwrite the store")
	cmd.Flags().BoolVar(&enrichEmail, "enrich-email", false, "Activate Deepline person-enrich for email/phone (costs credits)")
	cmd.Flags().StringVar(&deeplineKey, "deepline-key", "", "Deepline API key (default from $DEEPLINE_API_KEY)")
	return cmd
}

func parseSections(in []string) map[string]bool {
	out := map[string]bool{}
	if len(in) == 0 {
		out["profile"] = true
		out["research"] = true
		out["email"] = true
		return out
	}
	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))
		for _, part := range strings.Split(s, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out[part] = true
			}
		}
	}
	return out
}

// findHappenstanceResearch inspects /api/research/recent + /api/research/history
// for entries mentioning the target's name or LinkedIn URL. Returns the raw
// research object (JSON) or an error.
func findHappenstanceResearch(c *client.Client, name, linkedinURL string) (json.RawMessage, error) {
	raw, err := c.Get("/api/research/recent", nil)
	if err != nil {
		return nil, fmt.Errorf("research/recent: %w", err)
	}
	payload := extractResponseData(raw)
	if hit := scanResearchForMatch(payload, name, linkedinURL); hit != nil {
		return hit, nil
	}
	// Try history if recent missed.
	raw, err = c.Get("/api/research/history", nil)
	if err != nil {
		return nil, nil // non-fatal — no research available.
	}
	payload = extractResponseData(raw)
	if hit := scanResearchForMatch(payload, name, linkedinURL); hit != nil {
		return hit, nil
	}
	return nil, nil
}

// scanResearchForMatch walks a list of research entries and returns the first
// whose subject/title contains the name or linkedin_url.
func scanResearchForMatch(payload json.RawMessage, name, linkedinURL string) json.RawMessage {
	var arr []map[string]any
	if err := json.Unmarshal(payload, &arr); err != nil {
		return nil
	}
	lcName := strings.ToLower(strings.TrimSpace(name))
	lcURL := strings.ToLower(strings.TrimSpace(linkedinURL))
	for _, entry := range arr {
		blob, _ := json.Marshal(entry)
		lc := strings.ToLower(string(blob))
		if (lcName != "" && strings.Contains(lc, lcName)) ||
			(lcURL != "" && strings.Contains(lc, lcURL)) {
			raw, _ := json.Marshal(entry)
			return raw
		}
	}
	return nil
}

// maybeLIURL canonicalizes a bare slug into a full LinkedIn URL; passes URLs
// through unchanged.
func maybeLIURL(target string) string {
	if strings.Contains(target, "linkedin.com") {
		return target
	}
	if strings.ContainsAny(target, " \t") || strings.Contains(target, ".") || strings.Contains(target, "/") {
		return ""
	}
	return "https://www.linkedin.com/in/" + strings.TrimPrefix(target, "@") + "/"
}

// loadCachedDossier looks up an existing people row by linkedin_url /
// happenstance_uuid derived from the target. Returns nil when missing or
// expired.
func loadCachedDossier(target string) (*dossierOutput, error) {
	dbPath := defaultDBPath("contact-goat-pp-cli")
	s, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	var person *store.Person
	if u := maybeLIURL(target); u != "" {
		person, err = s.GetPersonByLinkedInURL(u)
		if err != nil {
			return nil, err
		}
	}
	if person == nil && store.IsUUID(target) {
		person, err = s.GetPersonByUUID(target)
		if err != nil {
			return nil, err
		}
	}
	if person == nil {
		return nil, nil
	}
	if time.Since(person.LastSeen) > dossierTTL {
		return nil, nil
	}
	out := &dossierOutput{
		Person: flagshipPerson{
			Name:             person.FullName,
			LinkedInURL:      person.LinkedInURL,
			HappenstanceUUID: person.HappenstanceUUID,
			Title:            person.Title,
			Company:          person.Company,
			Location:         person.Location,
			ImageURL:         person.ImageURL,
			Sources:          person.Sources,
		},
		LinkedIn:     person.LIData,
		Happenstance: person.HPData,
		Deepline:     person.DLData,
		CachedAt:     person.LastSeen.UTC().Format(time.RFC3339),
	}
	return out, nil
}

// persistDossier saves a composed dossier into the people table.
func persistDossier(d *dossierOutput) {
	dbPath := defaultDBPath("contact-goat-pp-cli")
	s, err := store.Open(dbPath)
	if err != nil {
		return
	}
	defer s.Close()
	data := map[string]any{
		"full_name":         d.Person.Name,
		"linkedin_url":      d.Person.LinkedInURL,
		"happenstance_uuid": d.Person.HappenstanceUUID,
		"title":             d.Person.Title,
		"company":           d.Person.Company,
		"location":          d.Person.Location,
		"image_url":         d.Person.ImageURL,
		"sources":           shortSources(d.Person.Sources),
		"li_data":           d.LinkedIn,
		"hp_data":           d.Happenstance,
		"dl_data":           d.Deepline,
	}
	if _, err := s.UpsertPerson(data); err != nil {
		fmt.Fprintf(os.Stderr, "warning: persist dossier: %v\n", err)
	}
}

func emitDossier(cmd *cobra.Command, flags *rootFlags, d *dossierOutput) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Dossier: %s\n", d.Person.Name)
	if d.Person.LinkedInURL != "" {
		fmt.Fprintf(w, "  linkedin: %s\n", d.Person.LinkedInURL)
	}
	if d.Person.Company != "" {
		fmt.Fprintf(w, "  company:  %s\n", d.Person.Company)
	}
	if d.Person.Title != "" {
		fmt.Fprintf(w, "  title:    %s\n", d.Person.Title)
	}
	if d.Person.Location != "" {
		fmt.Fprintf(w, "  location: %s\n", d.Person.Location)
	}
	fmt.Fprintf(w, "  sources:  %s\n", strings.Join(d.Person.Sources, ","))
	fmt.Fprintf(w, "  source:   %s\n", d.Source)
	if d.CachedAt != "" {
		fmt.Fprintf(w, "  cached_at: %s\n", d.CachedAt)
	}
	if len(d.LinkedIn) > 0 {
		fmt.Fprintln(w, "\n--- LinkedIn profile ---")
		printIndentedJSON(w, d.LinkedIn)
	}
	if len(d.Happenstance) > 0 {
		fmt.Fprintln(w, "\n--- Happenstance research ---")
		printIndentedJSON(w, d.Happenstance)
	}
	if len(d.Deepline) > 0 {
		fmt.Fprintln(w, "\n--- Deepline enrichment ---")
		printIndentedJSON(w, d.Deepline)
	}
	return nil
}

func printIndentedJSON(w interface{ Write(p []byte) (int, error) }, raw json.RawMessage) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		_, _ = w.Write(raw)
		return
	}
	b, _ := json.MarshalIndent(v, "  ", "  ")
	_, _ = w.Write(b)
	_, _ = w.Write([]byte("\n"))
}

// urlValid is a light sanity check on URLs to short-circuit empty dossier
// targets. Currently unused externally, kept for future email enrich fallback.
func urlValid(s string) bool {
	if s == "" {
		return false
	}
	_, err := url.Parse(s)
	return err == nil
}

var _ = context.Background
var _ = urlValid
