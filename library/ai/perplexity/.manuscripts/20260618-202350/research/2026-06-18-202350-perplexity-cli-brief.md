# Perplexity CLI Brief

## API Identity
- **Domain:** Research / answer synthesis / conversation memory.
- **Users:** People who already use Perplexity as a search-and-research scratchpad and want to keep that trail in their own knowledge system.
- **Data profile:** Recent threads, full transcripts, exports, browser-session cookies, and search results that should land in local raw knowledge.

## Product Framing
- **Goal:** Make Perplexity a terminal-first research backend.
- **Primary value:** Export traces out of Perplexity and preserve them locally.
- **Secondary value:** Let agents reuse the browser session instead of paying for the API.

## Top Workflows
1. Export a thread as Markdown, PDF, or DOCX.
2. Read a full transcript by UUID or slug.
3. List recent threads from the signed-in account.
4. Log in with Chrome cookies or a live browser session.

## Why It Exists
- Perplexity already holds the user's live research trail.
- Existing tools mostly assume paid API access.
- The browser session is the source of truth when the user wants to avoid API costs.

## Success Criteria
- Trace export is easy to automate.
- Recent research is discoverable from the CLI.
- Browser-session auth works without a paid API key.
