// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: multi-publication columnar sync — populates the columnar
// posts/publications/subscribers/drafts tables (ported from substack-creator's
// columnar store) that the eight transcendence commands query. The lift's
// generic `sync` only writes the `resources` table single-publication; this
// command discovers every publication the user owns and accumulates each one's
// posts/drafts/subscribers into the columnar tables keyed by publication_id, so
// cross-publication commands (portfolio, posts best, grep, subs cross-sell,
// schedule board, posts twin/pair[s], subs churn) return real data.
// Recorded in .printing-press-patches.json.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

	"github.com/spf13/cobra"
)

// ownedPublication is a publication the authenticated user authors/administers.
type ownedPublication struct {
	ID        string // numeric publication_id as a string (columnar PK)
	Subdomain string // host label used to route per-pub Creator requests
	Name      string
}

func newPortfolioSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath        string
		postLimit     int
		includeDrafts bool
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Discover every publication you own and populate the columnar tables.",
		Long: `Multi-publication columnar sync. Discovers every publication you author or
administer (via /subscriptions/page_v2 publicationUsers), then for each owned
publication fetches its published posts, drafts, and subscribers and writes them
into the local columnar posts/publications/subscribers/drafts tables, keyed by
publication_id and accumulating across publications.

This is what the transcendence commands read: portfolio, posts best/twin/pair[s],
grep, schedule board, subs churn/cross-sell. Run it after 'auth login'.

The discovery anchor is the currently-configured publication (SUBSTACK_PUBLICATION).
Its /publication object is fetched directly; any additional owned publications are
resolved from the subscriptions page and synced too when their subdomain is known.`,
		Example: `  export SUBSTACK_PUBLICATION=trevinsays
  substack-pp-cli portfolio sync --json`,
		Annotations: map[string]string{"pp:novel": "portfolio sync"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("substack-pp-cli")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer s.Close()

			report := portfolioSyncReport{Publications: []pubSyncReport{}}

			owned, err := discoverOwnedPublications(cmd.Context(), c, cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if len(owned) == 0 {
				return fmt.Errorf("no owned publications discovered; ensure SUBSTACK_PUBLICATION is set to a publication you own and you are authenticated (run 'auth login')")
			}

			for _, pub := range owned {
				pr := syncOnePublication(cmd.Context(), c, s, pub, postLimit, includeDrafts, cmd.ErrOrStderr())
				report.Publications = append(report.Publications, pr)
				report.PostsTotal += pr.Posts
				report.DraftsTotal += pr.Drafts
				report.SubscribersTotal += pr.Subscribers
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				raw, _ := json.Marshal(report)
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n", bold("Portfolio sync complete"))
			fmt.Fprintln(w, strings.Repeat("─", 64))
			for _, pr := range report.Publications {
				fmt.Fprintf(w, "  %-22s posts=%d drafts=%d subscribers=%d\n",
					truncate(pr.Subdomain, 22), pr.Posts, pr.Drafts, pr.Subscribers)
				if pr.Warning != "" {
					fmt.Fprintf(w, "    warning: %s\n", pr.Warning)
				}
			}
			fmt.Fprintln(w, strings.Repeat("─", 64))
			fmt.Fprintf(w, "%d publication(s); %d posts, %d drafts, %d subscribers total.\n",
				len(report.Publications), report.PostsTotal, report.DraftsTotal, report.SubscribersTotal)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&postLimit, "post-limit", 100, "Max published posts to fetch per publication")
	cmd.Flags().BoolVar(&includeDrafts, "drafts", true, "Also sync drafts into the columnar drafts table")
	return cmd
}

type portfolioSyncReport struct {
	Publications     []pubSyncReport `json:"publications"`
	PostsTotal       int             `json:"posts_total"`
	DraftsTotal      int             `json:"drafts_total"`
	SubscribersTotal int             `json:"subscribers_total"`
}

type pubSyncReport struct {
	PublicationID string `json:"publication_id"`
	Subdomain     string `json:"subdomain"`
	Name          string `json:"name"`
	Posts         int    `json:"posts"`
	Drafts        int    `json:"drafts"`
	Subscribers   int    `json:"subscribers"`
	Warning       string `json:"warning,omitempty"`
}

// setPublicationContext points the client at a publication's Creator host by
// overriding the {publication} template var. The generated client substitutes
// it into every https://{publication}.substack.com/... request path. The stdlib
// cookie jar carries the right session cookie across the custom-domain 301 hop.
func setPublicationContext(c *client.Client, subdomain string) {
	if c.Config == nil {
		c.Config = &config.Config{}
	}
	if c.Config.TemplateVars == nil {
		c.Config.TemplateVars = map[string]string{}
	}
	c.Config.TemplateVars["publication"] = subdomain
}

// discoverOwnedPublications resolves the publications the authenticated user
// owns. publicationUsers in /subscriptions/page_v2 lists {publication_id, role}
// for every publication the user authors/administers — that's the owned set.
// Subdomains for those IDs come from /publication on the configured host (the
// anchor) plus the page's publications[] name-map when present. Single-pub users
// get exactly one entry; the configured SUBSTACK_PUBLICATION is always the
// anchor so a one-publication account works without any cross-pub resolution.
func discoverOwnedPublications(ctx context.Context, c *client.Client, warnW interface{ Write([]byte) (int, error) }) ([]ownedPublication, error) {
	configuredSub := strings.TrimSpace(os.Getenv("SUBSTACK_PUBLICATION"))

	ownedIDs := map[string]bool{}
	subdomainByID := map[string]string{}
	nameByID := map[string]string{}

	// publicationUsers => owned publication IDs.
	page, err := c.Get(ctx, "https://substack.com/api/v1/subscriptions/page_v2", nil)
	if err == nil {
		var env map[string]json.RawMessage
		if json.Unmarshal(page, &env) == nil {
			var pubUsers []map[string]any
			_ = json.Unmarshal(env["publicationUsers"], &pubUsers)
			for _, pu := range pubUsers {
				id := store.ResourceIDString(store.LookupFieldValue(pu, "publication_id"))
				if id != "" {
					ownedIDs[id] = true
				}
			}
			// publications[] in the page is a name/subdomain map for some pubs.
			var pubs []map[string]any
			_ = json.Unmarshal(env["publications"], &pubs)
			for _, p := range pubs {
				id := store.ResourceIDString(store.LookupFieldValue(p, "id"))
				if id == "" {
					continue
				}
				if sub := pubStringField(p, "subdomain"); sub != "" {
					subdomainByID[id] = sub
				}
				if name := pubStringField(p, "name"); name != "" {
					nameByID[id] = name
				}
			}
		}
	} else {
		fmt.Fprintf(warnW, "warning: could not read subscriptions page for ownership discovery: %v\n", err)
	}

	// Anchor: fetch the configured publication's full object directly. This is
	// authoritative for subdomain/custom_domain/name and confirms ownership.
	if configuredSub != "" {
		setPublicationContext(c, configuredSub)
		pubObj, perr := c.Get(ctx, "https://{publication}.substack.com/api/v1/publication", nil)
		if perr == nil {
			var obj map[string]any
			if json.Unmarshal(pubObj, &obj) == nil {
				id := store.ResourceIDString(store.LookupFieldValue(obj, "id"))
				if id != "" {
					ownedIDs[id] = true // configured pub is owned by definition of access
					subdomainByID[id] = configuredSub
					if name := pubStringField(obj, "name"); name != "" {
						nameByID[id] = name
					}
				}
			}
		} else {
			fmt.Fprintf(warnW, "warning: could not fetch /publication for %q: %v\n", configuredSub, perr)
		}
	}

	var out []ownedPublication
	for id := range ownedIDs {
		sub := subdomainByID[id]
		if sub == "" {
			// No subdomain resolvable for this owned ID: we cannot route
			// per-pub Creator requests without one. Skip with a warning rather
			// than guessing a host. The anchor pub always has a subdomain.
			fmt.Fprintf(warnW, "warning: owned publication %s has no resolvable subdomain; skipping its columnar sync\n", id)
			continue
		}
		out = append(out, ownedPublication{ID: id, Subdomain: sub, Name: nameByID[id]})
	}
	return out, nil
}

// syncOnePublication fetches and upserts one owned publication's posts, drafts,
// and subscribers into the columnar tables.
func syncOnePublication(ctx context.Context, c *client.Client, s *store.Store, pub ownedPublication, postLimit int, includeDrafts bool, warnW interface{ Write([]byte) (int, error) }) pubSyncReport {
	pr := pubSyncReport{PublicationID: pub.ID, Subdomain: pub.Subdomain, Name: pub.Name}
	setPublicationContext(c, pub.Subdomain)

	// Publication identity row. Re-fetch /publication so subscriber counts and
	// custom_domain land in the columnar publications table.
	pubObj := fetchPublicationObject(ctx, c, pub)
	if pubObj != nil {
		raw, _ := json.Marshal(pubObj)
		if err := s.UpsertPublication(pubObj, raw); err != nil {
			pr.Warning = appendWarning(pr.Warning, fmt.Sprintf("publication upsert: %v", err))
		}
	}

	// Posts: /post_management/published carries engagement + metadata.
	posts := fetchPublishedPosts(ctx, c, postLimit, warnW)
	if n, err := s.UpsertPostsForPublication(pub.ID, posts); err != nil {
		pr.Warning = appendWarning(pr.Warning, fmt.Sprintf("posts upsert: %v", err))
	} else {
		pr.Posts = n
	}

	// Drafts: global /drafts with numeric publication_id avoids custom-domain
	// Creator-host 403s for publications that serve authoring through a custom
	// domain.
	if includeDrafts {
		drafts := fetchDrafts(ctx, c, pub.ID, warnW)
		if n, err := s.UpsertDraftsForPublication(pub.ID, drafts); err != nil {
			pr.Warning = appendWarning(pr.Warning, fmt.Sprintf("drafts upsert: %v", err))
		} else {
			pr.Drafts = n
		}
	}

	// Subscribers: /publication_launch_checklist returns recent subscriber rows
	// with membership_state and user.handle. Real per-subscriber identity is
	// the handle (emails are not exposed by these endpoints); we project it into
	// the columnar `email` column so cross-sell/churn (which key on email) join
	// across publications on a stable identity.
	subs := fetchSubscribers(ctx, c, warnW)
	if n, err := s.UpsertSubscribersForPublication(pub.ID, subs); err != nil {
		pr.Warning = appendWarning(pr.Warning, fmt.Sprintf("subscribers upsert: %v", err))
	} else {
		pr.Subscribers = n
	}

	return pr
}

// fetchPublicationObject returns the owned publication's identity object,
// enriched with subdomain/subscriber counts so the columnar publications columns
// populate. Counts come from /publication_launch_checklist subscriber rows.
func fetchPublicationObject(ctx context.Context, c *client.Client, pub ownedPublication) map[string]any {
	data, err := c.Get(ctx, "https://{publication}.substack.com/api/v1/publication", nil)
	if err != nil {
		return nil
	}
	var obj map[string]any
	if json.Unmarshal(data, &obj) != nil {
		return nil
	}
	// Ensure subdomain is set even if the API object omits it.
	if pubStringField(obj, "subdomain") == "" {
		obj["subdomain"] = pub.Subdomain
	}
	// Subscriber counts: derive from the checklist subscriber sample.
	free, paid, _ := fetchSubscriberCounts(ctx, c)
	obj["subscriber_count"] = free + paid
	obj["paid_subscriber_count"] = paid
	return obj
}

// fetchSubscriberCounts returns (freeCount, paidCount, total) derived from the
// publication_launch_checklist subscriber rows.
func fetchSubscriberCounts(ctx context.Context, c *client.Client) (int, int, int) {
	subs := fetchSubscribers(ctx, c, nil)
	free, paid := 0, 0
	for _, sub := range subs {
		switch pubStringField(sub, "tier") {
		case "paid", "founding":
			paid++
		default:
			free++
		}
	}
	return free, paid, free + paid
}

// publishedPostsPageSize is the maximum page size /post_management/published
// accepts; larger values 400 with "Invalid value". We paginate by offset to
// gather up to the caller's limit.
const publishedPostsPageSize = 12

// fetchPublishedPosts pulls the publication's published posts (with engagement),
// paginating by offset because the endpoint caps page size at
// publishedPostsPageSize.
func fetchPublishedPosts(ctx context.Context, c *client.Client, limit int, warnW interface{ Write([]byte) (int, error) }) []map[string]any {
	if limit <= 0 {
		limit = 100
	}
	var out []map[string]any
	for offset := 0; offset < limit; offset += publishedPostsPageSize {
		params := map[string]string{
			"order_by":        "post_date",
			"order_direction": "desc",
			"offset":          fmt.Sprintf("%d", offset),
			"limit":           fmt.Sprintf("%d", publishedPostsPageSize),
			"type":            "newsletter",
		}
		data, err := c.Get(ctx, "https://{publication}.substack.com/api/v1/post_management/published", params)
		if err != nil {
			warnf(warnW, "fetch published posts (offset %d): %v", offset, err)
			break
		}
		rows := extractObjects(data, "posts")
		for _, p := range rows {
			out = append(out, normalizePost(p))
		}
		if len(rows) < publishedPostsPageSize {
			break // last page reached
		}
	}
	return out
}

// normalizePost maps the published-post API shape onto the columnar posts
// columns. The API exposes reaction_count/comment_count; views/opens/clicks are
// not in this endpoint's payload, so they stay 0 (the columns exist for parity
// with substack-creator's schema and a future stats endpoint).
func normalizePost(p map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range p {
		out[k] = v
	}
	out["likes"] = numField(p, "reaction_count")
	out["comments"] = numField(p, "comment_count")
	out["restacks"] = numField(p, "restacks")
	// publish_date for window filters; the API field is post_date.
	if pubStringField(p, "publish_date") == "" {
		out["publish_date"] = pubStringField(p, "post_date")
	}
	// canonical_url for grep/twin output.
	if pubStringField(p, "canonical_url") == "" {
		if slug := pubStringField(p, "slug"); slug != "" {
			out["canonical_url"] = "https://" + currentPublicationHost() + "/p/" + slug
		}
	}
	// paywalled flag from audience.
	aud := pubStringField(p, "audience")
	out["paywalled"] = boolToInt(aud == "only_paid" || aud == "founding")
	// body_markdown for grep snippets: use title+subtitle when body absent.
	if pubStringField(p, "body_markdown") == "" {
		out["body_markdown"] = strings.TrimSpace(pubStringField(p, "title") + " " + pubStringField(p, "subtitle"))
	}
	return out
}

// fetchDrafts pulls the publication's drafts.
func fetchDrafts(ctx context.Context, c *client.Client, publicationID string, warnW interface{ Write([]byte) (int, error) }) []map[string]any {
	data, err := c.Get(ctx, globalAPIPath("/drafts"), map[string]string{"publication_id": publicationID})
	if err != nil {
		warnf(warnW, "fetch drafts: %v", err)
		return nil
	}
	// /drafts returns {"results":{"posts":[...]}} or {"posts":[...]}.
	rows := extractObjects(data, "posts")
	out := make([]map[string]any, 0, len(rows))
	for _, d := range rows {
		row := map[string]any{}
		for k, v := range d {
			row[k] = v
		}
		// drafts carry draft_title; map to title for the columnar column.
		if pubStringField(d, "title") == "" {
			row["title"] = pubStringField(d, "draft_title")
		}
		if pubStringField(d, "subtitle") == "" {
			row["subtitle"] = pubStringField(d, "draft_subtitle")
		}
		if pubStringField(d, "last_edited") == "" {
			row["last_edited"] = pubStringField(d, "draft_updated_at")
		}
		out = append(out, row)
	}
	return out
}

// fetchSubscribers pulls recent subscriber rows from the launch-checklist
// endpoint and projects them onto the columnar subscribers columns. Substack
// does not expose emails via the accessible Creator endpoints, so user.handle
// is used as the stable per-subscriber identity stored in the `email` column —
// that's the join key cross-sell and churn use across publications.
func fetchSubscribers(ctx context.Context, c *client.Client, warnW interface{ Write([]byte) (int, error) }) []map[string]any {
	data, err := c.Get(ctx, "https://{publication}.substack.com/api/v1/publication_launch_checklist", nil)
	if err != nil {
		warnf(warnW, "fetch subscribers: %v", err)
		return nil
	}
	rows := extractObjects(data, "subscribers")
	out := make([]map[string]any, 0, len(rows))
	for _, sub := range rows {
		row := map[string]any{}
		for k, v := range sub {
			row[k] = v
		}
		// Identity: prefer the nested user.handle, fall back to user_id.
		handle := ""
		name := ""
		if user, ok := sub["user"].(map[string]any); ok {
			handle = pubStringField(user, "handle")
			name = pubStringField(user, "name")
		}
		if handle == "" {
			handle = store.ResourceIDString(store.LookupFieldValue(sub, "user_id"))
		}
		row["email"] = handle
		row["name"] = name
		row["tier"] = classifyMembership(pubStringField(sub, "membership_state"), boolField(sub, "is_founding"))
		row["status"] = pubStringField(sub, "membership_state")
		row["subscribed_at"] = pubStringField(sub, "created_at")
		out = append(out, row)
	}
	return out
}

// classifyMembership maps Substack's membership_state to the free/paid/founding
// tier vocabulary the churn/cross-sell commands classify on.
func classifyMembership(state string, founding bool) string {
	if founding {
		return "founding"
	}
	switch strings.ToLower(state) {
	case "paid", "comp", "gift":
		return "paid"
	default:
		return "free"
	}
}

// ---------------------------------------------------------------------------
// small helpers
// ---------------------------------------------------------------------------

// extractObjects pulls an array of objects from a response. It tries the
// {"<key>":[...]} envelope, then {"results":{"<key>":[...]}}, then a bare array.
func extractObjects(data json.RawMessage, key string) []map[string]any {
	var direct []map[string]any
	if json.Unmarshal(data, &direct) == nil && len(direct) > 0 {
		return direct
	}
	var env map[string]json.RawMessage
	if json.Unmarshal(data, &env) != nil {
		return nil
	}
	if raw, ok := env[key]; ok {
		var rows []map[string]any
		if json.Unmarshal(raw, &rows) == nil {
			return rows
		}
	}
	// nested under results
	if raw, ok := env["results"]; ok {
		var inner map[string]json.RawMessage
		if json.Unmarshal(raw, &inner) == nil {
			if r2, ok := inner[key]; ok {
				var rows []map[string]any
				if json.Unmarshal(r2, &rows) == nil {
					return rows
				}
			}
		}
	}
	return nil
}

func pubStringField(obj map[string]any, key string) string {
	v := store.LookupFieldValue(obj, key)
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return store.ResourceIDString(v)
}

func numField(obj map[string]any, key string) int {
	v := store.LookupFieldValue(obj, key)
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	case string:
		var n int
		fmt.Sscanf(t, "%d", &n)
		return n
	}
	return 0
}

func boolField(obj map[string]any, key string) bool {
	v := store.LookupFieldValue(obj, key)
	b, _ := v.(bool)
	return b
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func currentPublicationHost() string {
	sub := strings.TrimSpace(os.Getenv("SUBSTACK_PUBLICATION"))
	if sub == "" {
		return "substack.com"
	}
	return sub + ".substack.com"
}

func appendWarning(existing, add string) string {
	if existing == "" {
		return add
	}
	return existing + "; " + add
}

func warnf(w interface{ Write([]byte) (int, error) }, format string, args ...any) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "warning: "+format+"\n", args...)
}
