// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// whichEntry is one row of the curated capability index. The index is
// seeded at generation time from the same NovelFeature list that drives
// the SKILL.md feature section, so the command a `which` query returns
// is guaranteed to exist and to match what the skill advertises.
type whichEntry struct {
	Command      string `json:"command"`
	Description  string `json:"description"`
	Group        string `json:"group,omitempty"`
	WhyItMatters string `json:"why_it_matters,omitempty"`
}

// whichIndex is the curated list of capabilities this CLI advertises as
// its hero features. Endpoint-level commands are discoverable via
// `--help`; `which` exists to resolve a natural-language capability
// query to one of the commands the skill says matter most.
var whichIndex = []whichEntry{
	// --- Setup / introspection ---
	{Command: "gorgias-pp-cli doctor --json", Group: "setup",
		Description:  "Probes /account with the configured credentials and reports `credentials: valid` only when an authenticated call succeeds.",
		WhyItMatters: "Saves the first-five-minutes credential-debug cycle when wiring up an agent."},
	{Command: "gorgias-pp-cli auth setup", Group: "setup",
		Description:  "Walks through generating a Gorgias API key in the UI and saving it locally as Basic-auth credentials.",
		WhyItMatters: "Removes the guesswork of where to click in the Gorgias settings UI to mint a key."},
	{Command: "gorgias-pp-cli auth set-token <email> <api-key>", Group: "setup",
		Description:  "Saves the email + API key pair to the config file for non-interactive setups (CI, agents).",
		WhyItMatters: "Lets agents bootstrap credentials from secrets without prompting."},
	{Command: "gorgias-pp-cli auth status --json", Group: "setup",
		Description:  "Reports whether credentials are configured locally; does not call the API.",
		WhyItMatters: "Cheaper check than `doctor` when you only need to confirm a key is on disk."},
	{Command: "gorgias-pp-cli agent-context --json", Group: "setup",
		Description:  "Returns a machine-readable description of the CLI's capabilities, response envelopes, exit codes, and feature flags.",
		WhyItMatters: "Lets a calling agent discover what this CLI can do without parsing --help text."},
	{Command: "gorgias-pp-cli api", Group: "setup",
		Description:  "Lists every endpoint the CLI knows about, with its underlying HTTP method/path.",
		WhyItMatters: "Maps an agent's intent to the closest endpoint command when the natural-language query doesn't match a hero feature."},
	{Command: "gorgias-pp-cli version --json", Group: "setup",
		Description:  "Prints the CLI version (and git SHA when built via goreleaser).",
		WhyItMatters: "Lets downstream agents pin behaviour to a known build."},

	// --- Local mirror (sync / search / sql / analytics) ---
	{Command: "gorgias-pp-cli sync --resources tickets --since 7d", Group: "local",
		Description:  "Syncs API data to a local SQLite DB so subsequent searches, analytics, and joins run without hitting the API. Ticket --since uses documented order_by plus local filtering.",
		WhyItMatters: "Makes repeated agent-driven lookups (e.g. searching for similar past tickets) practical at scale."},
	{Command: "gorgias-pp-cli search <query> --agent", Group: "local",
		Description:  "Full-text search across synced tickets, customers, and messages backed by SQLite FTS5.",
		WhyItMatters: "Returns relevant past tickets without per-query API round-trips or rate-limit risk."},
	{Command: "gorgias-pp-cli sql \"SELECT ... FROM resources WHERE resource_type='tickets' ...\"", Group: "local",
		Description:  "Runs a read-only SQL query against the local SQLite mirror — filter the generic resources table by resource_type for tickets/customers/messages.",
		WhyItMatters: "Answers ad-hoc analytical questions (cohort, group-by, top-N) the live API does not expose."},
	{Command: "gorgias-pp-cli analytics ...", Group: "local",
		Description:  "Pre-built analytics queries (counts, rates, top-N) over the locally synced data.",
		WhyItMatters: "Skip the SQL when a packaged metric already exists for the question you have."},
	{Command: "gorgias-pp-cli pm stale --since 14d", Group: "local",
		Description:  "Lists tickets with no updates in the last N days, sourced from the local mirror.",
		WhyItMatters: "Surfaces stuck work for triage without hammering the live API."},
	{Command: "gorgias-pp-cli pm orphans", Group: "local",
		Description:  "Finds tickets missing required fields like assignee or team — sourced from the local mirror.",
		WhyItMatters: "Quick pipeline-hygiene check for a manager standup."},
	{Command: "gorgias-pp-cli pm load", Group: "local",
		Description:  "Shows ticket workload distribution per assignee from the local mirror.",
		WhyItMatters: "Answers 'who is overloaded?' without round-tripping the API per agent."},

	// --- Live operations on tickets ---
	{Command: "gorgias-pp-cli tickets list --view-id <id>", Group: "tickets",
		Description:  "Lists tickets, optionally constrained to a saved Gorgias view (matches the agent inbox UI).",
		WhyItMatters: "View-scoped listing maps directly to the inbox shape support agents already think in."},
	{Command: "gorgias-pp-cli tickets get <id>", Group: "tickets",
		Description:  "Fetches one ticket by ID, including assignee, tags, status, and channel metadata.",
		WhyItMatters: "Single-shot detail read; pair with `tickets messages-list` for the conversation body."},
	{Command: "gorgias-pp-cli tickets create --stdin", Group: "tickets",
		Description:  "Creates a new ticket — the full body can be piped via stdin or assembled from individual flags.",
		WhyItMatters: "Stdin path is how an LLM hands a full ticket payload to the CLI in one shot."},
	{Command: "gorgias-pp-cli tickets update <id> --status closed", Group: "tickets",
		Description:  "Updates fields on a ticket — status, assignee, tags, priority, snooze, etc.",
		WhyItMatters: "Status transitions are the most common write the agent will make on a ticket."},
	{Command: "gorgias-pp-cli tickets messages-create <ticket-id> --stdin", Group: "tickets",
		Description:  "Posts a new message on a ticket — public reply, internal note, or outbound channel message.",
		WhyItMatters: "This is the 'send the reply' verb for any agent generating customer-facing text."},
	{Command: "gorgias-pp-cli tickets tags-add <ticket-id> --tag-ids <ids>", Group: "tickets",
		Description:  "Attaches tags to a ticket — used for routing rules, reporting, and macro triggers.",
		WhyItMatters: "Tag operations are how agents signal classification decisions back into the Gorgias workflow."},
	{Command: "gorgias-pp-cli tickets delete <id> --ignore-missing", Group: "tickets",
		Description:  "Hard-deletes a ticket. With `--ignore-missing` a 404 collapses to a no-op so the call is safely retryable.",
		WhyItMatters: "Cleanup of test data or duplicates; the idempotent flag is critical for retry-friendly agent loops."},

	// --- Customer-side ---
	{Command: "gorgias-pp-cli customers create --email <email>", Group: "customers",
		Description:  "Creates a new customer with email/name/channels — typically called by an integration before opening a ticket.",
		WhyItMatters: "Prerequisite for tickets that originate outside Gorgias's own channels (webhook, custom UI)."},
	{Command: "gorgias-pp-cli customers merge --source-id <a> --target-id <b>", Group: "customers",
		Description:  "Merges one customer into another, keeping all history under the target ID.",
		WhyItMatters: "Reduces duplicate-customer noise that comes from email-vs-phone-vs-social fragmentation."},
	{Command: "gorgias-pp-cli customers update <id> --stdin", Group: "customers",
		Description:  "Updates a customer's name, channels, external IDs, or top-level fields.",
		WhyItMatters: "Keeps the customer record in sync after an external CRM update."},

	// --- Operations / live observability ---
	{Command: "gorgias-pp-cli tail tickets --interval 30s", Group: "ops",
		Description:  "Streams new and changed tickets by polling the API on an interval.",
		WhyItMatters: "Lets an agent or operator watch the inbox without writing their own polling loop."},
	{Command: "gorgias-pp-cli macros list", Group: "ops",
		Description:  "Lists canned-reply macros so an agent can pick one before composing.",
		WhyItMatters: "Reuses approved language instead of having the LLM regenerate boilerplate."},
	{Command: "gorgias-pp-cli rules list", Group: "ops",
		Description:  "Lists automation rules so you can see what routing/auto-reply behaviour is in effect before sending.",
		WhyItMatters: "Avoids fighting an existing rule that would override what the agent is about to do."},
	{Command: "gorgias-pp-cli satisfaction-surveys list", Group: "ops",
		Description:  "Lists CSAT survey responses with rating, comment, and the linked ticket.",
		WhyItMatters: "Pulls voice-of-customer signal into the agent loop for quality review."},
	{Command: "gorgias-pp-cli views list", Group: "ops",
		Description:  "Lists saved Gorgias views (saved filters) you can pass to `tickets list --view-id`.",
		WhyItMatters: "Discover the view-ID an agent should scope its inbox queries to."},
	{Command: "gorgias-pp-cli workflow status --json", Group: "ops",
		Description:  "Reports the current ticket archive workflow status (open/archived ratios, recent activity).",
		WhyItMatters: "Sanity check before bulk operations that touch live channels."},

	// --- Output shaping / reusability ---
	{Command: "gorgias-pp-cli export <resource> --since 30d", Group: "output",
		Description:  "Dumps a resource (tickets, customers, etc.) to JSON/CSV for archival or downstream pipelines.",
		WhyItMatters: "One-shot snapshot when you want a file rather than a synced DB."},
	{Command: "gorgias-pp-cli import <resource> --file <path>", Group: "output",
		Description:  "Bulk-creates records from a JSON/CSV file (e.g. seeding customers from a CRM dump).",
		WhyItMatters: "Avoids hand-rolled loops over individual `create` calls when seeding."},
	{Command: "gorgias-pp-cli profile save <name>", Group: "output",
		Description:  "Saves the current non-default flags as a named profile reusable via `--profile <name>`.",
		WhyItMatters: "Cuts repetition for an agent that uses the same flag set across calls."},
	{Command: "gorgias-pp-cli feedback \"text\"", Group: "output",
		Description:  "Records feedback about this CLI to a local ledger and optionally POSTs upstream.",
		WhyItMatters: "Lets agents log surprising responses so the maintainers see them."},
}

// whichMatch pairs an index entry with its ranking score for a query.
// Higher score means stronger match. The ranker is naive (exact token
// then substring then group tag) because 20-40 entries do not need
// semantic retrieval - a ranker upgrade is a future change that would
// not break this contract.
type whichMatch struct {
	Entry whichEntry `json:"entry"`
	Score int        `json:"score"`
}

// rankWhich returns up to `limit` best matches for `query` against the
// index, sorted by descending score. Score breakdown:
//
//	+3  exact token match on the command's leaf or full path
//	+2  substring match on the command (any part)
//	+2  substring match on the description
//	+1  group tag contains the query as a word
//
// Ties break on declaration order in the index. An empty query returns
// every entry at score 0 in declaration order - this is the "list all"
// behavior the skill documents for broad agent discovery.
// whichStopwords are short English glue words that produce noisy scores
// when used as tokens — they appear in nearly every description and the
// agent rarely cares about matching them. Dropped before ranking.
var whichStopwords = map[string]bool{
	"a": true, "an": true, "the": true, "to": true, "of": true,
	"and": true, "or": true, "for": true, "in": true, "on": true,
	"is": true, "it": true, "i": true, "my": true, "this": true,
	"that": true, "with": true, "from": true, "by": true,
}

func rankWhich(index []whichEntry, query string, limit int) []whichMatch {
	if limit <= 0 {
		limit = 3
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		out := make([]whichMatch, 0, len(index))
		for _, e := range index {
			out = append(out, whichMatch{Entry: e, Score: 0})
		}
		return out
	}
	rawTokens := strings.Fields(q)
	qTokens := make([]string, 0, len(rawTokens))
	for _, t := range rawTokens {
		if !whichStopwords[t] {
			qTokens = append(qTokens, t)
		}
	}
	// If the query was nothing but stopwords, fall back to the original
	// token set so the user still gets a result.
	if len(qTokens) == 0 {
		qTokens = rawTokens
	}

	scored := make([]whichMatch, 0, len(index))
	for i, e := range index {
		score := whichScoreEntry(e, q, qTokens)
		scored = append(scored, whichMatch{Entry: e, Score: score})
		_ = i
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	// Drop zero-score matches when the query was non-empty; agents
	// branching on exit code rely on "no match" meaning no confidence.
	filtered := scored[:0]
	for _, m := range scored {
		if m.Score > 0 {
			filtered = append(filtered, m)
		}
	}
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

func whichScoreEntry(e whichEntry, query string, qTokens []string) int {
	score := 0
	cmd := strings.ToLower(e.Command)
	cmdTokens := strings.Fields(cmd)
	desc := strings.ToLower(e.Description)
	why := strings.ToLower(e.WhyItMatters)
	group := strings.ToLower(e.Group)

	// Exact token match on the command path (any token).
	for _, qt := range qTokens {
		for _, ct := range cmdTokens {
			if qt == ct {
				score += 3
				break
			}
		}
	}
	// Substring match on the full command (covers hyphenated leaves).
	if strings.Contains(cmd, query) {
		score += 2
	}
	// Substring match on the full description or why-it-matters phrase.
	// Phrase hits anywhere in the entry are strong evidence of intent.
	if strings.Contains(desc, query) || strings.Contains(why, query) {
		score += 2
	}
	// Per-token presence in description + why-it-matters: makes multi-word
	// queries like "send a reply" partially match entries that frame the
	// verb in the why-it-matters line ("This is the 'send the reply'
	// verb...") even when the description uses different phrasing. Capped
	// so a query that happens to dump many short words can't outscore a
	// real cmd-token hit.
	hay := desc + " " + why
	descHits := 0
	for _, qt := range qTokens {
		if len(qt) >= 3 && strings.Contains(hay, qt) {
			descHits++
		}
	}
	if descHits > 3 {
		descHits = 3
	}
	score += descHits
	// Group tag match.
	if group != "" {
		for _, qt := range qTokens {
			if strings.Contains(group, qt) {
				score += 1
				break
			}
		}
	}
	return score
}

func newWhichCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "which [query]",
		Short: "Find the command that implements a capability",
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2",
		},
		Example: `  gorgias-pp-cli which "stale tickets"
  gorgias-pp-cli which "bottleneck"
  gorgias-pp-cli which --limit 1 "send message"
  gorgias-pp-cli which                                # list the full capability index`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(whichIndex) == 0 {
				return usageErr(fmt.Errorf("this CLI has no curated capability index; run '--help' to see every command"))
			}
			query := strings.Join(args, " ")
			matches := rankWhich(whichIndex, query, limit)

			// Empty query returns the whole index at score 0 (listing mode).
			if strings.TrimSpace(query) == "" {
				return renderWhich(cmd, flags, rankWhichAll(whichIndex))
			}

			if len(matches) == 0 {
				// Under --json, return an empty matches envelope at exit 0
				// so agents can branch on `matches.length == 0` instead of
				// parsing a usage error message. Non-JSON keeps the typed
				// exit-2 path so terminal users see the help hint.
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"matches": []whichMatch{},
					}, flags)
				}
				// Truncate the query in the error message so a misbehaving
				// caller can't drive 10KB of stderr per call.
				echo := query
				if len(echo) > 80 {
					echo = echo[:77] + "..."
				}
				return usageErr(fmt.Errorf("no match for %q; try '%s --help' for the full command list", echo, cmd.Root().Name()))
			}
			return renderWhich(cmd, flags, matches)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 3, "Maximum number of matches to return")
	return cmd
}

// rankWhichAll is a narrow helper used by the "empty query lists the
// index" path. It returns every entry in declaration order at score 0
// so the render path treats them uniformly.
func rankWhichAll(index []whichEntry) []whichMatch {
	out := make([]whichMatch, 0, len(index))
	for _, e := range index {
		out = append(out, whichMatch{Entry: e, Score: 0})
	}
	return out
}

func renderWhich(cmd *cobra.Command, flags *rootFlags, matches []whichMatch) error {
	w := cmd.OutOrStdout()
	// Output shape follows the same rule as every other generated
	// command: JSON when the caller asked for it OR when stdout is not
	// a terminal; table when a human is looking.
	asJSON := flags.asJSON
	if !asJSON && !isTerminal(w) {
		asJSON = true
	}
	if asJSON {
		// JSON envelope: {matches: [...]}. The wrap is critical:
		// printJSONFiltered's --compact path uses compactListFields
		// (allowlist) for top-level arrays, which would strip
		// entry/score keys; routing through compactObjectFields
		// (blocklist) via an object envelope preserves them.
		if matches == nil {
			matches = []whichMatch{}
		}
		return printJSONFiltered(w, map[string]any{"matches": matches}, flags)
	}
	fmt.Fprintf(w, "%-24s  %-8s  %s\n", "COMMAND", "SCORE", "DESCRIPTION")
	for _, m := range matches {
		fmt.Fprintf(w, "%-24s  %-8d  %s\n", m.Entry.Command, m.Score, m.Entry.Description)
	}
	return nil
}
