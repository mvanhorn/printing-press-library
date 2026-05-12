# DataForSEO CLI

**Every DataForSEO endpoint, plus auto-mode routing, cost estimates, keyword pre-cleaning, and a local SQLite store no other DataForSEO tool has.**

A single-binary Go CLI over all 554 DataForSEO endpoints with offline SQLite-backed delta trackers, automatic Live↔Standard mode routing to dodge the 3.3× cost premium, and a pre-call cost estimator. Built to absorb everything the official MCP and the 6.4k-star claude-seo skill already do, then go further with rank tracking over a sitemap, AI-visibility deltas across ChatGPT/Claude/Gemini/Perplexity, and backlinks-since-last-run diffs.

## Install

The recommended path installs both the `dataforseo-pp-cli` binary and the `pp-dataforseo` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install dataforseo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install dataforseo --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dataforseo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-dataforseo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-dataforseo --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-dataforseo skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-dataforseo. The skill defines how its required CLI can be installed.
```

## Authentication

DataForSEO uses HTTP Basic. Set DATAFORSEO_LOGIN and DATAFORSEO_PASSWORD from app.dataforseo.com/api-access. Run `dataforseo-pp-cli doctor` to verify the credentials and surface your account balance before kicking off any cost-bearing call.

## Quick Start

```bash
# Verify credentials and check your account balance before any cost-bearing call.
dataforseo-pp-cli doctor --json


# Predict spend before a volume call so a 200-keyword batch doesn't surprise-bill you.
dataforseo-pp-cli cost estimate keywords_data/google_ads/search_volume/live --keywords keywords.txt


# Pre-clean the keyword list so one >10-word string doesn't fail the whole batch with the silent 40501 trap.
dataforseo-pp-cli keywords clean keywords.txt --json


# Auto-routes to Live for batches ≤5 or Standard (3.3× cheaper) for larger jobs.
dataforseo-pp-cli keywords volume --keywords keywords.txt --mode auto --json


# Weekly Monday-morning rank check over a sitemap, with SERP-feature deltas (AI Overview, featured snippet) flagged.
dataforseo-pp-cli rank track --sitemap https://fsmstumpgrinding.com/sitemap.xml --features --json


# Track brand mentions in ChatGPT / Claude / Gemini / Perplexity answers, week-over-week.
dataforseo-pp-cli ai-visibility track --brand "FSM Stump Grinding" --keywords keywords.txt --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`keywords clean`** — Pre-clean keywords before sending to Google Ads volume so one bad keyword doesn't poison the whole batch, and surface task-level 40501 errors that DataForSEO hides behind a top-level 20000.

  _Reach for this when you'd otherwise call the Google Ads volume endpoint directly. DataForSEO returns top-level 20000 (success) even when every task inside failed validation; this command refuses to send poisoned batches and exits non-zero on hidden errors._

  ```bash
  dataforseo-pp-cli keywords clean keywords.txt --json
  ```
- **`task bundle`** — Run the full Standard-mode lifecycle (task_post → tasks_ready poll → task_get → merge) as one command, with task IDs persisted to local SQLite so Ctrl-C or a laptop sleep doesn't lose the queue.

  _Use for any cost-sensitive batch job (which is most of them). One command replaces a Python poll-loop script and survives interruptions._

  ```bash
  dataforseo-pp-cli task bundle serp/google/organic/task_post --in batch.json --json
  ```
- **`search`** — Every result-returning API call mirrors into local SQLite (keywords, serp_results, backlinks, ai_mentions) with FTS5 indexes; offline `search` runs FTS5 MATCH over snippets, URLs, anchor text, and AI-answer excerpts without re-billing.

  _Use whenever you're tempted to re-call the same SERP/volume/backlink endpoint. The local store makes re-queries free._

  ```bash
  dataforseo-pp-cli search "tree service daytona" --json --select keyword,position,url
  ```
- **`keywords delta`** — Joins the current Google Ads volume API response against the last stored value per keyword in local SQLite and outputs movers sorted by absolute delta.

  _Reach for this whenever a content team asks 'which of the keywords we tracked last week moved.' DataForSEO doesn't return historical volume in one call; the local store enables this._

  ```bash
  dataforseo-pp-cli keywords delta --since 7d --json --select keyword,volume,delta
  ```
- **`rank track`** — Parse a sitemap or URL→keyword map, call `serp/google/organic/live/advanced` per keyword, diff position + SERP features (AI Overview, featured snippet, knowledge panel, PAA) against last run, and flag movers and feature gains/losses.

  _Use weekly across any tracked page inventory. Replaces a manual GSC + incognito-Google ritual with one command._

  ```bash
  dataforseo-pp-cli rank track --sitemap https://fsmstumpgrinding.com/sitemap.xml --features --json
  ```
- **`ai-visibility track`** — Call AI Optimization family endpoints (gemini, chat_gpt, llm_mentions, claude, perplexity) for a brand × keyword grid, store per-LLM mentions/excerpts in SQLite, and diff week-over-week presence + mention count per LLM.

  _Use weekly to track whether a brand's offer copy is being cited by ChatGPT/Claude/Gemini/Perplexity for buyer-intent queries. The mechanical alternative is asking each LLM by hand._

  ```bash
  dataforseo-pp-cli ai-visibility track --brand "FSM Stump Grinding" --keywords keywords.txt --json
  ```
- **`backlinks new`** — Snapshot `backlinks/summary` + `referring_domains` + `anchors` into local SQLite; on each run diff against the prior snapshot and surface newly-acquired referring domains with anchor text.

  _Use weekly during any link-acquisition campaign (citation submissions, guest posts, PR placements) to confirm new backlinks landed and anchor text is on-brand._

  ```bash
  dataforseo-pp-cli backlinks new --domain fsmstumpgrinding.com --json
  ```

### Reachability mitigation
- **`keywords volume`** — Auto-pick the cost-optimal execution mode based on batch size via `--mode auto`. Batches of 5 or fewer go through Live (sync, premium); larger batches route through Standard (queued, ~3.3× cheaper) with a managed poll loop.

  _Use whenever the batch size is uncertain. Pass `--mode auto` (the default) to avoid burning 3.3× the cost on a 200-keyword job because the user forgot to flip to Standard._

  ```bash
  dataforseo-pp-cli keywords volume --keywords keywords.txt --mode auto --json
  ```
- **`cost estimate`** — Predict spend before any live API call using a static price table per endpoint family multiplied by the planned input size. Optional --confirm-over flag gates spending above a threshold.

  _Reach for this before any batch-size or backlink call. Joey's #1 fear with DataForSEO is accidentally burning the $50 deposit on one careless command; this prevents that._

  ```bash
  dataforseo-pp-cli cost estimate keywords_data/google_ads/search_volume/live --keywords keywords.txt --confirm-over 1.00
  ```

## Usage

Run `dataforseo-pp-cli --help` for the full command reference and flag list.

## Commands

### ai-optimization

Manage ai optimization

- **`dataforseo-pp-cli ai-optimization ai-keyword-data-available-filters`** - Here you will find all the necessary information about filters that can be used with AI Keyword Data API endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/ai_keyword_data/available_filters'
- **`dataforseo-pp-cli ai-optimization ai-keyword-data-keywords-search-volume-live`** - This endpoint provides search volume data for your target keywords, reflecting their estimated usage in AI tools.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/ai_keyword_data/keywords_search_volume/live'
- **`dataforseo-pp-cli ai-optimization ai-keyword-data-locations-and-languages`** - Using this endpoint you can get the full list of locations and languages supported in AI Keyword Data API.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/ai_keyword_data/locations_and_languages'
- **`dataforseo-pp-cli ai-optimization chat-gpt-llm-responses-live`** - Live ChatGPT LLM Responses endpoint allows you to retrieve structured responses from a specific ChatGPT AI model, based on the input parameters.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/chat_gpt/llm_responses/live/'
- **`dataforseo-pp-cli ai-optimization chat-gpt-llm-responses-models`** - You will receive the list of available Chat GPT AI models by calling this API.
 
As a response of the API server, you will receive JSON-encoded data containing a tasks array with the information specific to the set tasks.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/chat_gpt/llm_responses/models/'
- **`dataforseo-pp-cli ai-optimization chat-gpt-llm-responses-task-get`** - Chat GPT LLM Responses endpoint allows you to retrieve structured responses from a specific Chat GPT model, based on the input parameters.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/chat_gpt/llm_responses/task_get/'
- **`dataforseo-pp-cli ai-optimization chat-gpt-llm-responses-task-post`** - ChatGPT LLM Responses endpoint allows you to retrieve structured responses from a specific ChatGPT model, based on the input parameters.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/chat_gpt/llm_responses/task_post/'
- **`dataforseo-pp-cli ai-optimization chat-gpt-llm-responses-tasks-ready`** - This endpoint is designed to provide you with a list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/chat_gpt/llm_responses/tasks_ready/'
- **`dataforseo-pp-cli ai-optimization chat-gpt-llm-scraper-languages`** - You will receive the list of languages by calling this API.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/chat_gpt/llm_scraper/languages/?bash'
- **`dataforseo-pp-cli ai-optimization chat-gpt-llm-scraper-live-advanced`** - Live ChatGPT LLM Scraper endpoint provides results from ChatGPT searches. The results are specific to the selected location (see the List of Locations) and language (see the List of Languages) parameters.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/chat_gpt/llm_scraper/live/advanced/'
- **`dataforseo-pp-cli ai-optimization chat-gpt-llm-scraper-live-html`** - Live ChatGPT LLM Scraper API HTML provides a raw HTML page of the results for the specified keyword, language, and location.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/chat_gpt/llm_scraper/live/html/'
- **`dataforseo-pp-cli ai-optimization chat-gpt-llm-scraper-locations`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/chat_gpt/llm_scraper/locations/?bash'
- **`dataforseo-pp-cli ai-optimization chat-gpt-llm-scraper-locations-country`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/chat_gpt/llm_scraper/locations/?bash'
- **`dataforseo-pp-cli ai-optimization chat-gpt-llm-scraper-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/chat_gpt/llm_scraper/task_get/advanced/?bash'
- **`dataforseo-pp-cli ai-optimization chat-gpt-llm-scraper-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/chat_gpt/llm_scraper/task_get/html/?bash'
- **`dataforseo-pp-cli ai-optimization chat-gpt-llm-scraper-task-post`** - ChatGPT LLM Scraper API provides results from ChatGPT searches. The results are specific to the selected location (see the List of Locations) and language (see the List of Languages) parameters.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/chat_gpt/llm_scraper/task_post/?bash'
- **`dataforseo-pp-cli ai-optimization chat-gpt-llm-scraper-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/chat_gpt/llm_scraper/tasks_ready/?bash'
- **`dataforseo-pp-cli ai-optimization claude-llm-responses-live`** - Live Claude LLM Responses endpoint allows you to retrieve structured responses from a specific Claude model, based on the input parameters.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/claude/llm_responses/live/'
- **`dataforseo-pp-cli ai-optimization claude-llm-responses-models`** - You will receive the list of available Claude AI models by calling this API.
 
As a response of the API server, you will receive JSON-encoded data containing a tasks array with the information specific to the set tasks.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/claude/llm_responses/models/'
- **`dataforseo-pp-cli ai-optimization claude-llm-responses-task-get`** - Claude LLM Responses endpoint allows you to retrieve structured responses from a specific Claude model, based on the input parameters.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/claude/llm_responses/task_get/'
- **`dataforseo-pp-cli ai-optimization claude-llm-responses-task-post`** - Claude LLM Responses endpoint allows you to retrieve structured responses from a specific Claude model, based on the input parameters.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/claude/llm_responses/task_post/'
- **`dataforseo-pp-cli ai-optimization claude-llm-responses-tasks-ready`** - This endpoint is designed to provide you with a list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/claude/llm_responses/tasks_ready/'
- **`dataforseo-pp-cli ai-optimization gemini-llm-responses-live`** - Live Gemini LLM Responses endpoint allows you to retrieve structured responses from a specific Gemini AI model, based on the input parameters.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/gemini/llm_responses/live/'
- **`dataforseo-pp-cli ai-optimization gemini-llm-responses-models`** - You will receive the list of available Gemini AI models by calling this API.
 
As a response of the API server, you will receive JSON-encoded data containing a tasks array with the information specific to the set tasks.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/gemini/llm_responses/models/'
- **`dataforseo-pp-cli ai-optimization gemini-llm-responses-task-get`** - Gemini LLM Responses endpoint allows you to retrieve structured responses from a specific Gemini model, based on the input parameters.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/gemini/llm_responses/task_get/'
- **`dataforseo-pp-cli ai-optimization gemini-llm-responses-task-post`** - Gemini LLM Responses endpoint allows you to retrieve structured responses from a specific Gemini model, based on the input parameters.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/gemini/llm_responses/task_post/'
- **`dataforseo-pp-cli ai-optimization gemini-llm-responses-tasks-ready`** - This endpoint is designed to provide you with a list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/gemini/llm_responses/tasks_ready/'
- **`dataforseo-pp-cli ai-optimization gemini-llm-scraper-languages`** - You will receive the list of languages by calling this API.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/gemini/llm_scraper/languages/?bash'
- **`dataforseo-pp-cli ai-optimization gemini-llm-scraper-live-advanced`** - Live Gemini LLM Scraper endpoint provides structured results from Gemini. The results are specific to the selected location (see the List of Locations), language (see the List of Languages), and keyword.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/gemini/llm_scraper/live/advanced/'
- **`dataforseo-pp-cli ai-optimization gemini-llm-scraper-live-html`** - Live Gemini LLM Scraper API HTML provides a raw HTML page of the results for the specified keyword, language (see the List of Languages), and location (see the List of Locations).
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/gemini/llm_scraper/live/html/'
- **`dataforseo-pp-cli ai-optimization gemini-llm-scraper-locations`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/gemini/llm_scraper/locations/?bash'
- **`dataforseo-pp-cli ai-optimization gemini-llm-scraper-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/gemini/llm_scraper/task_get/advanced/?bash'
- **`dataforseo-pp-cli ai-optimization gemini-llm-scraper-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/gemini/llm_scraper/task_get/html/?bash'
- **`dataforseo-pp-cli ai-optimization gemini-llm-scraper-task-post`** - Gemini LLM Scraper API provides structured results from Gemini. The results are specific to the selected location (see the List of Locations) and language (see the List of Languages), and keyword.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/gemini/llm_scraper/task_post/?bash'
- **`dataforseo-pp-cli ai-optimization gemini-llm-scraper-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/gemini/llm_scraper/tasks_ready/?bash'
- **`dataforseo-pp-cli ai-optimization llm-mentions-aggregated-metrics-live`** - Live LLM Mentions endpoint provides aggregated metrics for mentions of the keywords or domains specified in the target array of the request. The results are specific to the selected platform (google for Google’s  AI Overview or chat_gpt for ChatGPT), location and language parameters (see the List of Locations & Languages).
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/llm_mentions/aggregated_metrics/live/'
- **`dataforseo-pp-cli ai-optimization llm-mentions-available-filters`** - Here you will find all the necessary information about filters that can be used with AI Optimization LLM Mentions API endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/llm_mentions/filters/'
- **`dataforseo-pp-cli ai-optimization llm-mentions-cross-aggregated-metrics-live`** - Live LLM Mentions endpoint provides aggregated metrics grouped by custom keys for mentions of the keywords or domains specified in the target array of the request. Each item in the results array corresponds to the specified target. The results are specific to the selected platform (google for Google’s  AI Overview or chat_gpt for ChatGPT), location and language parameters (see the List of Locations & Languages).
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/llm_mentions/cross_aggregated_metrics/live/'
- **`dataforseo-pp-cli ai-optimization llm-mentions-locations-and-languages`** - Using this endpoint you can get the full list of locations and languages supported in AI Optimization LLM Mentions API.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/llm_mentions/locations_and_languages/'
- **`dataforseo-pp-cli ai-optimization llm-mentions-search-live`** - Live LLM Mentions Search endpoint provides mention data and related metrics from AI searches. The results are specific to the selected platform (google for Google’s  AI Overview or chat_gpt for ChatGPT), as well as location and language parameters (see the List of Locations & Languages).
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/llm_mentions/search/live/'
- **`dataforseo-pp-cli ai-optimization llm-mentions-top-domains-live`** - Live LLM Mentions Top Domains endpoint provides aggregated LLM mentions metrics grouped by the most frequently mentioned domains for the specified target. The results are specific to the selected platform (google for Google’s  AI Overview or chat_gpt for ChatGPT), location and language parameters (see the List of Locations & Languages).
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/llm_mentions/top_domains/live/'
- **`dataforseo-pp-cli ai-optimization llm-mentions-top-pages-live`** - Live LLM Mentions Top Pages endpoint provides aggregated LLM mentions metrics grouped by the most frequently mentioned pages for the specified target. The results are specific to the selected platform (google for Google’s  AI Overview or chat_gpt for ChatGPT), location and language parameters (see the List of Locations & Languages).
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/llm_mentions/top_pages/live/'
- **`dataforseo-pp-cli ai-optimization perplexity-llm-responses-live`** - Live Perplexity LLM Responses endpoint allows you to retrieve structured responses from a specific Perplexity AI model, based on the input parameters.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/perplexity/llm_responses/live/'
- **`dataforseo-pp-cli ai-optimization perplexity-llm-responses-models`** - You will receive the list of available Perplexity AI models by calling this API.
 
As a response of the API server, you will receive JSON-encoded data containing a tasks array with the information specific to the set tasks.
for more info please visit 'https://docs.dataforseo.com/v3/ai_optimization/perplexity/llm_responses/models/'

### app-data

Manage app data

- **`dataforseo-pp-cli app-data apple-app-info-task-get-advanced`** - This endpoint will provide you with information about the mobile application specified in a POST request. You will receive its ID, icon, description, reviews count, rating, images, and other data. The results are specific to the app_id parameter specified in the POST request.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/app_info/task_get/advanced/?bash'
- **`dataforseo-pp-cli app-data apple-app-info-task-post`** - This endpoint will provide you with information about the App Store application specified in the app_id field of the POST request.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/app_info/task_post/?bash'
- **`dataforseo-pp-cli app-data apple-app-info-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with a list of completed tasks that haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/app_info/tasks_ready/?bash'
- **`dataforseo-pp-cli app-data apple-app-list-task-get-advanced`** - This endpoint will provide you with a list of applications published in the top app charts on the App Store platform, including app IDs, ratings, prices, titles, and more. The results are specific to the app_collection as well as the location and language parameters specified in the POST request.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/app_list/task_get/advanced/?bash'
- **`dataforseo-pp-cli app-data apple-app-list-task-post`** - This endpoint will provide you with a list of mobile applications published in the top app charts on the App Store platform. The returned results are specific to the app collection as well as the language and location parameters.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/app_list/task_post/?bash'
- **`dataforseo-pp-cli app-data apple-app-list-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with a list of completed tasks that haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/app_list/tasks_ready/?bash'
- **`dataforseo-pp-cli app-data apple-app-listings-categories`** - This endpoint will provide you with a full list of app categories available on Apple App Store.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/app_listings/categories/?bash'
- **`dataforseo-pp-cli app-data apple-app-listings-search-live`** - This endpoint will provide you with a list of apps published on App Store along with additional information: its ID, icon, reviews count, rating, price, and other data. The results are specific to the title, description, and categories parameters specified in the API request.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/app_listings/search/live/?bash'
- **`dataforseo-pp-cli app-data apple-app-reviews-task-get-advanced`** - This endpoint will provide you with feedback data on applications listed on the App Store platform, including review ratings, review content, user profile info of each reviewer, review publication dates, and more. The results are specific to the app_id as well as the location and language parameters specified in the POST request.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/app_reviews/task_get/advanced/?bash'
- **`dataforseo-pp-cli app-data apple-app-reviews-task-post`** - This endpoint will provide you with reviews published on the App Store platform for the app specified in the app_id field. The returned results are specific to the indicated language and location parameters.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/app_reviews/task_post/?bash'
- **`dataforseo-pp-cli app-data apple-app-reviews-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with a list of completed tasks that haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/app_reviews/tasks_ready/?bash'
- **`dataforseo-pp-cli app-data apple-app-searches-task-get-advanced`** - This endpoint will provide you with a list of apps ranking on the App Store for the keyword specified in a POST request. You will also receive additional information about each application: its ID, icon, reviews count, rating, price, and other data. The results are specific to the keyword as well as location and language parameters specified in the POST request.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/app_searches/task_get/advanced/?bash'
- **`dataforseo-pp-cli app-data apple-app-searches-task-post`** - This endpoint will provide you with a list of apps ranking on the App Store for the specified keyword. The returned results are specific to the indicated keyword, as well as the location and language parameters.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/app_searches/task_post/?bash'
- **`dataforseo-pp-cli app-data apple-app-searches-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with a list of completed tasks that haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/app_searches/tasks_ready/?bash'
- **`dataforseo-pp-cli app-data apple-categories`** - This endpoint will provide you with a full list of app categories available on App Store.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/categories/?bash'
- **`dataforseo-pp-cli app-data apple-languages`** - By calling this endpoint you will receive the list of Apple languages supported in App Data API.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/languages/?bash'
- **`dataforseo-pp-cli app-data apple-locations`** - By calling this endpoint you will receive the list of Apple locations supported in App Data API.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/apple/locations/?bash'
- **`dataforseo-pp-cli app-data errors`** - By calling this endpoint you will receive information about the App Data API tasks that returned an error within the past 7 days.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/errors/?bash'
- **`dataforseo-pp-cli app-data google-app-info-task-get-advanced`** - This endpoint will provide you with information about the mobile application specified in a POST request. You will receive its ID, icon, description, reviews count, rating, number of installs, images, and other data. The results are specific to the app_id parameter specified in the POST request.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_info/task_get/advanced/?bash'
- **`dataforseo-pp-cli app-data google-app-info-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_info/task_get/html/?bash'
- **`dataforseo-pp-cli app-data google-app-info-task-post`** - This endpoint will provide you with information about the Google Play application specified in the app_id field of the POST request.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_info/task_post/?bash'
- **`dataforseo-pp-cli app-data google-app-info-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with a list of completed tasks that haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_info/tasks_ready/?bash'
- **`dataforseo-pp-cli app-data google-app-list-task-get-advanced`** - This endpoint will provide you with a list of applications published in the top charts on the Google Play platform, including app IDs, ratings, prices, titles, and more. The results are specific to the app_collection as well as the location and language parameters specified in the POST request.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_list/task_get/advanced/?bash'
- **`dataforseo-pp-cli app-data google-app-list-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_list/task_get/html/?bash'
- **`dataforseo-pp-cli app-data google-app-list-task-post`** - This endpoint will provide you with a list of mobile applications published in the top charts on the Google Play platform. The returned results are specific to the app collection as well as the the language and location parameters.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_list/task_post/?bash'
- **`dataforseo-pp-cli app-data google-app-list-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with a list of completed tasks that haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_list/tasks_ready/?bash'
- **`dataforseo-pp-cli app-data google-app-listings-categories`** - This endpoint will provide you with a full list of app categories available on Google Play.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_listings/categories/?bash'
- **`dataforseo-pp-cli app-data google-app-listings-search-live`** - This endpoint will provide you with a list of apps published on Google Play along with additional information: its ID, icon, reviews count, rating, price, and other data. The results are specific to the title, description, and categories parameters specified in the API request.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_listings/search/live/?bash'
- **`dataforseo-pp-cli app-data google-app-reviews-task-get-advanced`** - This endpoint will provide you with feedback data on applications listed on the Google Play platform, including review ratings, review content, user profile info of each reviewer, review publication dates, and more. The results are specific to the app_id as well as the location and language parameters specified in the POST request.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_reviews/task_get/advanced/?bash'
- **`dataforseo-pp-cli app-data google-app-reviews-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_reviews/task_get/html/?bash'
- **`dataforseo-pp-cli app-data google-app-reviews-task-post`** - This endpoint will provide you with reviews published on the Google Play platform for the app specified in the app_id field. The returned results are specific to the indicated language and location parameters.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_reviews/task_post/?bash'
- **`dataforseo-pp-cli app-data google-app-reviews-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with a list of completed tasks that haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_reviews/tasks_ready/?bash'
- **`dataforseo-pp-cli app-data google-app-searches-task-get-advanced`** - This endpoint will provide you with a list of apps ranking on Google Play for the keyword specified in a POST request. You will also receive additional information about each application: its ID, icon, reviews count, rating, price, and other data. The results are specific to the keyword as well as location and language parameters specified in the POST request.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_searches/task_get/advanced/?bash'
- **`dataforseo-pp-cli app-data google-app-searches-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_searches/task_get/html/?bash'
- **`dataforseo-pp-cli app-data google-app-searches-task-post`** - This endpoint will provide you with a list of apps ranking on Google Play for the specified keyword. The returned results are specific to the indicated keyword, as well as the language and location parameters.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_searches/task_post/?bash'
- **`dataforseo-pp-cli app-data google-app-searches-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with a list of completed tasks that haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_searches/tasks_ready/?bash'
- **`dataforseo-pp-cli app-data google-categories`** - This endpoint will provide you with a full list of app categories available on Google Play.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/categories/?bash'
- **`dataforseo-pp-cli app-data google-languages`** - By calling this endpoint you will receive the list of Google languages supported in App Data API.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/languages/?bash'
- **`dataforseo-pp-cli app-data google-locations`** - By calling this endpoint you will receive the list of Google locations supported in App Data API.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/locations/?bash'
- **`dataforseo-pp-cli app-data google-locations-country`** - By calling this endpoint you will receive the list of Google locations supported in App Data API.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/locations/?bash'
- **`dataforseo-pp-cli app-data id-list`** - This endpoint is designed to provide you with a list of IDs and metadata for all App Data tasks created within the specified time period, including both successful and uncompleted tasks.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/id_list/?bash'
- **`dataforseo-pp-cli app-data tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with a list of completed tasks that haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/app_data/google/app_searches/tasks_ready/?bash'

### appendix

Manage appendix

- **`dataforseo-pp-cli appendix errors`** - This endpoint returns a list of possible DataForSEO API errors and general status codes. Below you will find a list of HTTP response codes and internal messages. We recommend storing the data connected to error codes in your application log and designing a necessary system for handling related exceptional or error conditions.
for more info please visit 'https://docs.dataforseo.com/v3/appendix/errors/?bash'
- **`dataforseo-pp-cli appendix status`** - By calling this API you will receive detailed information about the current status of all our APIs and endpoints. You will also get a full issue description if a problem occurs.
for more info please visit 'https://docs.dataforseo.com/v3/appendix/status/?bash'
- **`dataforseo-pp-cli appendix user-data`** - You will receive detailed information about your API usage, prices, spending and other account details by calling this API.
for more info please visit 'https://docs.dataforseo.com/v3/appendix/user_data/?bash'
- **`dataforseo-pp-cli appendix webhook-resend`** - Using this endpoint you can resend webhooks (pingbacks and postbacks) for up to 100 specified tasks.
Note: Your account will not be double-charged for resending a webhook.
for more info please visit 'https://docs.dataforseo.com/v3/appendix/webhook_resend/?bash'

### backlinks

Manage backlinks

- **`dataforseo-pp-cli backlinks anchors-live`** - This endpoint will provide you with a detailed overview of anchors used when linking to the specified website with relevant backlink data for each of them.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/anchors/live/?bash'
- **`dataforseo-pp-cli backlinks available-filters`** - Backlinks API features plenty of parameters that support custom filtration. By applying filters to your POST requests, you will be able to effortlessly extract data that matches your requirements. Note that we do not charge any fees for using data filtering or sorting rules.
‌‌
Here you will find all the necessary information about filters that can be used with DataForSEO Backlinks API endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/filters/?bash'
- **`dataforseo-pp-cli backlinks bulk-live`** - This endpoint will provide you with the number of backlinks pointing to domains, subdomains, and pages specified in the targets array. The returned numbers correspond to all live backlinks, that is, total number of referring links with all attributes (e.g., nofollow, noreferrer, ugc, sponsored etc) that were found during the latest check.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/bulk_backlinks/live/?bash'
- **`dataforseo-pp-cli backlinks bulk-new-lost-live`** - This endpoint will provide you with the number of new and lost backlinks for the domains, subdomains, and pages specified in the targets array.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/bulk_new_lost_backlinks/live/?bash'
- **`dataforseo-pp-cli backlinks bulk-new-lost-referring-domains-live`** - This endpoint will provide you with the number of referring domains pointing to the domains, subdomains and pages specified in the targets array.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/bulk_new_lost_referring_domains/live/?bash'
- **`dataforseo-pp-cli backlinks bulk-pages-summary-live`** - This endpoint will provide you with a comprehensive overview of backlinks and related data for a bulk of up to 1000 pages, domains, or subdomains. If you indicate a single page as a target, you will get comprehensive summary data on all backlinks for that page.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/bulk_pages_summary/live/?bash'
- **`dataforseo-pp-cli backlinks bulk-ranks-live`** - This endpoint will provide you with rank scores of the domains, subdomains, and pages specified in the targets array. The score is based on the number of referring domains pointing to the specified domains, subdomains, or pages. The rank values represent real-time data for the date of the request and range from 0 (no backlinks detected) to 1,000 (highest rank). A similar scoring system is used in Google’s Page Rank algorithm. You can learn more about rank scores in this help center article
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/bulk_ranks/live/?bash'
- **`dataforseo-pp-cli backlinks bulk-referring-domains-live`** - This endpoint will provide you with the number of referring domains pointing to domains, subdomains, and pages specified in the targets array. The returned numbers are based on all live referring domains, that is, total number of domains pointing to the target with any type of backlinks (e.g., nofollow, noreferrer, ugc, sponsored etc) that were found during the latest check.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/bulk_referring_domains/live/?bash'
- **`dataforseo-pp-cli backlinks bulk-spam-score-live`** - This endpoint will provide you with spam scores of the domains, subdomains, and pages you specified in the targets array. Spam Score is DataForSEO’s proprietary metric that indicates how “spammy” your target is on a scale from 0 to 100. You can learn more about Spam Score, how it is calculated, and signals it takes into account in this help center article
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/bulk_spam_score/live/?bash'
- **`dataforseo-pp-cli backlinks competitors-live`** - This endpoint will provide you with a list of competitors that share some part of the backlink profile with a target website, along with a number of backlink intersections and the rank of every competing website.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/competitors/live/?bash'
- **`dataforseo-pp-cli backlinks domain-intersection-live`** - This endpoint will provide you with the list of domains pointing to the specified websites. This endpoint is especially useful for creating a Link Gap feature that shows what domains link to your competitors but do not link out to your website.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/domain_intersection/live/?bash'
- **`dataforseo-pp-cli backlinks domain-pages-live`** - This endpoint will provide you with a detailed overview of domain pages with backlink data for each page.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/domain_pages/live/?bash'
- **`dataforseo-pp-cli backlinks domain-pages-summary-live`** - This endpoint will provide you with detailed summary data on all backlinks and related metrics for each page of the target domain or subdomain you specify. If you indicate a single page as a target, you will get comprehensive summary data on all backlinks for that page.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/domain_pages_summary/live/?bash'
- **`dataforseo-pp-cli backlinks errors`** - By calling this endpoint you will receive information about the Backlinks API tasks that returned an error within the past 7 days.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/errors/?bash'
- **`dataforseo-pp-cli backlinks history-live`** - This endpoint will provide you with historical backlinks data back to the beginning of 2019. You can receive the number of backlinks a given domain had in a specific time period, the number of new & lost backlinks, referring domains, and more.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/history/live/?bash'
- **`dataforseo-pp-cli backlinks id-list`** - This endpoint is designed to provide you with a list of IDs and metadata for all Backlinks tasks created within the specified time period, including both successful and uncompleted tasks.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/id_list/?bash'
- **`dataforseo-pp-cli backlinks index`** - This endpoint will provide you with the total number of backlinks, domains, and pages our database contains for the moment when you make a request. You will also get stats for the last 12 months.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/index/?bash'
- **`dataforseo-pp-cli backlinks live`** - This endpoint will provide you with a list of backlinks and relevant data for the specified domain, subdomain, or webpage.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/backlinks/live/?bash'
- **`dataforseo-pp-cli backlinks page-intersection-live`** - This endpoint will provide you with the list of referring pages pointing to the specified targets. It is especially useful for finding the backlinks that point to your competitors but don’t point to your website.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/page_intersection/live/?bash'
- **`dataforseo-pp-cli backlinks referring-domains-live`** - This endpoint will provide you with a detailed overview of referring domains pointing to the target you specify.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/referring_domains/live/?bash'
- **`dataforseo-pp-cli backlinks referring-networks-live`** - This endpoint will provide you with a detailed overview of referring IPs and subnets pointing to the target you specify.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/referring_networks/live/?bash'
- **`dataforseo-pp-cli backlinks summary-live`** - This endpoint will provide you with an overview of backlinks data available for a given domain, subdomain, or webpage.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/summary/live/?bash'
- **`dataforseo-pp-cli backlinks timeseries-new-lost-summary-live`** - This endpoint will provide you with the number of new and lost backlinks and referring domains for the domain specified in the target field.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/timeseries_new_lost_summary/live/?bash'
- **`dataforseo-pp-cli backlinks timeseries-summary-live`** - This endpoint will provide you with an overview of backlink data for the target domain available during a period between the two indicated dates. Backlink metrics will be grouped by the time range that you define: day, week, month, or year.
for more info please visit 'https://docs.dataforseo.com/v3/backlinks/timeseries_summary/live/?bash'

### business-data

Manage business data

- **`dataforseo-pp-cli business-data business-listings-available-filters`** - Here you will find all the necessary information about filters that can be used with Business Listings API.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/business_listings/filters/?bash'
- **`dataforseo-pp-cli business-data business-listings-categories`** - This endpoint will provide you with the list of top categories by business count.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/business_listings/categories/?bash'
- **`dataforseo-pp-cli business-data business-listings-categories-aggregation-live`** - Business Listings Categories Aggregation endpoint provides results containing information about groups of related categories along with the number of entities in each category. The provided results are specific to the specified parameters.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/business_listings/categories_aggregation/live/?bash'
- **`dataforseo-pp-cli business-data business-listings-locations`** - You will receive the list of locations by this API call. You can also download the full list of supported locations in the CSV format (last updated 2026-04-06).
for more info please visit 'https://docs.dataforseo.com/v3/business_data/business_listings/locations/?bash'
- **`dataforseo-pp-cli business-data business-listings-search-live`** - Business Listings Search API provides results containing information about business entities listed on Google Maps in the specified categories. You will receive the address, contacts, rating, working hours, and other relevant data. The provided results are specific to the selected location (see the List of Locations) settings.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/business_listings/search/live/?bash'
- **`dataforseo-pp-cli business-data errors`** - By calling this endpoint you will receive information about the Business Data API tasks that returned an error within the past 7 days.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/errors/?bash'
- **`dataforseo-pp-cli business-data google-extended-reviews-task-get`** - The returned results are specific to the indicated local establishment name, search engine, location and language parameters. We emulate set location and search engine with the highest accuracy so that the results you receive will match the actual search results for the specified parameters at the time of task setting. You can always check the returned results accessing the check_url in the Incognito mode to make sure the received data is entirely relevant. Note that user preferences, search history, and other personalized search factors are ignored by our system and thus would not be reflected in the returned results.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/extended_reviews/task_get/?bash'
- **`dataforseo-pp-cli business-data google-extended-reviews-task-post`** - This endpoint provides results from the “Reviews” element of Google SERPs, including not only Google user reviews but also reviews from other reputable sources (e.g., TripAdvisor, Yelp, Trustpilot). The results are specific to the selected location (see the List of Locations) and language (see the List of Languages) parameters.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/extended_reviews/task_post/?bash'
- **`dataforseo-pp-cli business-data google-extended-reviews-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you don’t use the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/extended_reviews/tasks_ready/?bash'
- **`dataforseo-pp-cli business-data google-hotel-info-live-advanced`** - Google Hotel Info will provide you with structured data available for a specific hotel entity on the Google Hotels platform: such as service description, location details, rating, amenities, reviews, images, prices, and more.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/hotel_info/live/advanced/?bash'
- **`dataforseo-pp-cli business-data google-hotel-info-live-html`** - Google Hotel Info will provide you with unstructured HTML data available for a specific hotel entity on the Google Hotels platform: such as service description, location details, rating, amenities, reviews, images, prices, and more.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/hotel_info/live/html/?bash'
- **`dataforseo-pp-cli business-data google-hotel-info-task-get-advanced`** - for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/hotel_info/task_get/advanced/?bash'
- **`dataforseo-pp-cli business-data google-hotel-info-task-get-html`** - for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/hotel_info/task_get/html/?bash'
- **`dataforseo-pp-cli business-data google-hotel-info-task-post`** - Google Hotel Info will provide you with structured data available for a specific hotel entity on the Google Hotels platform: such as service description, location details, rating, amenities, reviews, images, prices, and more.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/hotel_info/task_post/?bash'
- **`dataforseo-pp-cli business-data google-hotel-info-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you don’t use the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/hotel_info/tasks_ready/?bash'
- **`dataforseo-pp-cli business-data google-hotel-searches-live`** - Hotel Searches API provides results containing information about different hotels listed on Google Hotels. The provided results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/hotel_searches/live/?bash'
- **`dataforseo-pp-cli business-data google-hotel-searches-task-get`** - for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/hotel_searches/task_get/?bash'
- **`dataforseo-pp-cli business-data google-hotel-searches-task-post`** - Hotel Searches API provides results containing information about different hotels listed on Google. The provided results are specific to the keyword, selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/hotel_searches/task_post/?bash'
- **`dataforseo-pp-cli business-data google-hotel-searches-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you don’t use the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/hotel_searches/tasks_ready/?bash'
- **`dataforseo-pp-cli business-data google-languages`** - You will receive the list of languages by calling this API.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/languages/?bash'
- **`dataforseo-pp-cli business-data google-locations`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/locations/?bash'
- **`dataforseo-pp-cli business-data google-locations-country`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/locations/?bash'
- **`dataforseo-pp-cli business-data google-my-business-info-live`** - Business Data API provides results containing information about specific business entity from Google. The provided results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/my_business_info/live/?bash'
- **`dataforseo-pp-cli business-data google-my-business-info-task-get`** - for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/my_business_info/task_get/?bash'
- **`dataforseo-pp-cli business-data google-my-business-info-task-post`** - Business Data API provides results containing information about specific business entity from Google. The provided results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/my_business_info/task_post/?bash'
- **`dataforseo-pp-cli business-data google-my-business-info-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you don’t use the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/my_business_info/tasks_ready/?bash'
- **`dataforseo-pp-cli business-data google-my-business-updates-task-get`** - for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/my_business_updates/task_get/?bash'
- **`dataforseo-pp-cli business-data google-my-business-updates-task-post`** - This endpoints provides the latest updates of a specific business entity from Google SERP. The provided results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/my_business_updates/task_post/?bash'
- **`dataforseo-pp-cli business-data google-my-business-updates-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you don’t use the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/my_business_updates/tasks_ready/?bash'
- **`dataforseo-pp-cli business-data google-questions-and-answers-live`** - This endpoint will provide you with a detailed overview of questions and answers associated with a specific business entity listed on Google My Business. By submitting a request to this endpoint, you can access comprehensive data on the inquiries and responses related to a particular business, including the full text of the questions and answers, as well as metadata such as timestamps, user information.
 
The provided results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
 
Your account will be billed for every 20 questions, the maximum number of answers returned for each question is 5.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/questions_and_answers/live/?bash'
- **`dataforseo-pp-cli business-data google-questions-and-answers-task-get`** - for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/questions_and_answers/task_get/?bash'
- **`dataforseo-pp-cli business-data google-questions-and-answers-task-post`** - This endpoint will provide you with a detailed overview of questions and answers associated with a specific business entity listed on Google My Business. By submitting a request to this endpoint, you can access comprehensive data on the inquiries and responses related to a particular business, including the full text of the questions and answers, as well as metadata such as timestamps, user information.
 
The provided results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
 
Your account will be billed for every 20 questions, the maximum number of answers returned for each question is 5.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/questions_and_answers/task_post/?bash'
- **`dataforseo-pp-cli business-data google-questions-and-answers-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you don’t use the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/questions_and_answers/tasks_ready/?bash'
- **`dataforseo-pp-cli business-data google-reviews-task-get`** - The returned results are specific to the indicated local establishment name, search engine, location and language parameters. We emulate set location and search engine with the highest accuracy so that the results you receive will match the actual search results for the specified parameters at the time of task setting. You can always check the returned results accessing the check_url in the Incognito mode to make sure the received data is entirely relevant. Note that user preferences, search history, and other personalized search factors are ignored by our system and thus would not be reflected in the returned results.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/reviews/task_get/?bash'
- **`dataforseo-pp-cli business-data google-reviews-task-post`** - This endpoint provides results from the “Reviews” element of Google SERPs. The results are specific to the selected location (see the List of Locations) and language (see the List of Languages) parameters.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/reviews/task_post/?bash'
- **`dataforseo-pp-cli business-data google-reviews-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you don’t use the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/reviews/tasks_ready/?bash'
- **`dataforseo-pp-cli business-data id-list`** - This endpoint is designed to provide you with a list of IDs and metadata for all Business Data tasks created within the specified time period, including both successful and uncompleted tasks.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/id_list/?bash'
- **`dataforseo-pp-cli business-data social-media-pinterest-live`** - Social Media Pinterest API will provide you with data on pins made from the specified URLs. Pins on Pinterest correspond to content saves. For each specified page URL, you will get the number of content saves to Pinterest made using the Pinterest Save Button placed on that page.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/social_media/pinterest/live/?bash'
- **`dataforseo-pp-cli business-data social-media-reddit-live`** - Social Media Reddit API provides information for each share of the target webpage on Reddit. For each specified Reddit URL, you will get subreddit and author names, permalink, title, and the number of subreddit members.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/social_media/reddit/live/?bash'
- **`dataforseo-pp-cli business-data tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you don’t use the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/google/my_business_info/tasks_ready/?bash'
- **`dataforseo-pp-cli business-data tripadvisor-languages`** - You will receive the list of languages by calling this API.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/tripadvisor/languages/?bash'
- **`dataforseo-pp-cli business-data tripadvisor-locations`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task. Note that supported location types in Tripadvisor Business Data API are City and Region only.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/tripadvisor/locations/?bash'
- **`dataforseo-pp-cli business-data tripadvisor-locations-country`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task. Note that supported location types in Tripadvisor Business Data API are City and Region only.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/tripadvisor/locations/?bash'
- **`dataforseo-pp-cli business-data tripadvisor-reviews-task-get`** - This endpoint provides feedback data on businesses listed on the Tripadvisor platform, including their locations, ratings, review content and count. The results are specific to the URL path indicated in the POST request.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/tripadvisor/reviews/task_get/?bash'
- **`dataforseo-pp-cli business-data tripadvisor-reviews-task-post`** - This endpoint provides results from the “Reviews” element on the Tripadvisor platform. The results are specific to the URL path or keyword you indicate, and and the selected location (see the List of Locations).
for more info please visit 'https://docs.dataforseo.com/v3/business_data/tripadvisor/reviews/task_post/?bash'
- **`dataforseo-pp-cli business-data tripadvisor-reviews-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you don’t use the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/tripadvisor/reviews/tasks_ready/?bash'
- **`dataforseo-pp-cli business-data tripadvisor-search-task-get`** - This endpoint will provide you with data on businesses listed on the Tripadvisor platform. The results obtained through this endpoint are specific to the location (see the List of Tripadvisor Locations) and keyword parameters used in the POST request.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/tripadvisor/search/task_get/?bash'
- **`dataforseo-pp-cli business-data tripadvisor-search-task-post`** - This endpoint provides a list of business profiles listed on the Tripadvisor platform. The returned results are relevant to the specified keyword and the selected location (see the List of Locations).
for more info please visit 'https://docs.dataforseo.com/v3/business_data/tripadvisor/search/task_post/?bash'
- **`dataforseo-pp-cli business-data tripadvisor-search-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you don’t use the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/tripadvisor/search/tasks_ready/?bash'
- **`dataforseo-pp-cli business-data trustpilot-reviews-task-get`** - This endpoint provides reviews published on the Trustpilot platform The returned results are specific to the indicated business entity. We emulate set parameters with the highest accuracy so that the results you receive will match the actual search results for the specified parameters at the time of task setting. You can always check the returned results accessing the check_url in the Incognito mode to make sure the received data is entirely relevant. Note that user preferences, search history, and other personalized search factors are ignored by our system and thus would not be reflected in the returned results.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/trustpilot/reviews/task_get/?bash'
- **`dataforseo-pp-cli business-data trustpilot-reviews-task-post`** - This endpoint provides reviews published on the Trustpilot platform for the local establishment specified in the domain field.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/trustpilot/reviews/task_post/?bash'
- **`dataforseo-pp-cli business-data trustpilot-reviews-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you don’t use the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/trustpilot/reviews/tasks_ready/?bash'
- **`dataforseo-pp-cli business-data trustpilot-search-task-get`** - This endpoint provides a list of business profiles listed on the Trustpilot platform. The returned results are relevant to the keyword specified in a POST request. We emulate set parameters with the highest accuracy so that the results you receive match the actual search results for the specified parameters at the time of task setting. You can always check the returned results accessing the check_url in the Incognito mode to make sure the received data is entirely relevant. Note that user preferences, search history, and other personalized search factors are ignored by our system and thus will not be reflected in the returned results.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/trustpilot/search/task_get/?bash'
- **`dataforseo-pp-cli business-data trustpilot-search-task-post`** - This endpoint provides a list of business profiles listed on the Trustpilot platform. The returned results are relevant to the specified keyword.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/trustpilot/search/task_post/?bash'
- **`dataforseo-pp-cli business-data trustpilot-search-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you don’t use the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/business_data/trustpilot/search/tasks_ready/?bash'

### content-analysis

Manage content analysis

- **`dataforseo-pp-cli content-analysis available-filters`** - Here you will find all the necessary information about filters that can be used with Content Analysis API endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/content_analysis/filters/?bash'
- **`dataforseo-pp-cli content-analysis categories`** - We use Google product and service categories. This endpoint will provide you with the full list of available categories.
You can also download the CSV file by this link.
for more info please visit 'https://docs.dataforseo.com/v3/content_analysis/categories/?bash'
- **`dataforseo-pp-cli content-analysis category-trends-live`** - This endpoint will provide you with data on all citations in the target category for the indicated date range.
for more info please visit 'https://docs.dataforseo.com/v3/content_analysis/category_trends/live/?bash'
- **`dataforseo-pp-cli content-analysis id-list`** - This endpoint is designed to provide you with a list of IDs and metadata for all Content Analysis tasks created within the specified time period, including both successful and uncompleted tasks.
for more info please visit 'https://docs.dataforseo.com/v3/content_analysis/id_list/?bash'
- **`dataforseo-pp-cli content-analysis languages`** - You will receive the list of languages by calling this API.
 
As a response of the API server, you will receive JSON-encoded data containing a tasks array with the information specific to the set tasks.
for more info please visit 'https://docs.dataforseo.com/v3/content_analysis/languages/?bash'
- **`dataforseo-pp-cli content-analysis locations`** - You will receive the list of locations by this API call.
for more info please visit 'https://docs.dataforseo.com/v3/content_analysis/locations/?bash'
- **`dataforseo-pp-cli content-analysis phrase-trends-live`** - This endpoint will provide you with data on all citations of the target keyword for the indicated date range.
for more info please visit 'https://docs.dataforseo.com/v3/content_analysis/phrase_trends/live/?bash'
- **`dataforseo-pp-cli content-analysis rating-distribution-live`** - This endpoint will provide you with rating distribution data for the keyword and other parameters specified in the request.
for more info please visit 'https://docs.dataforseo.com/v3/content_analysis/rating_distribution/live/?bash'
- **`dataforseo-pp-cli content-analysis search-live`** - This endpoint will provide you with detailed citation data available for the target keyword.
for more info please visit 'https://docs.dataforseo.com/v3/content_analysis/search/live/?bash'
- **`dataforseo-pp-cli content-analysis sentiment-analysis-live`** - This endpoint will provide you with sentiment analysis data for the citations available for the target keyword.
for more info please visit 'https://docs.dataforseo.com/v3/content_analysis/sentiment_analysis/live/?bash'
- **`dataforseo-pp-cli content-analysis summary-live`** - This endpoint will provide you with an overview of citation data available for the target keyword.
for more info please visit 'https://docs.dataforseo.com/v3/content_analysis/summary/live/?bash'

### dataforseo-labs

Manage dataforseo labs

- **`dataforseo-pp-cli dataforseo-labs amazon-bulk-search-volume-live`** - This endpoint will provide you with search volume values for a maximum of 1,000 keywords in one API request. Here search volume represents the approximate number of monthly searches for a keyword on Amazon. The returned results are specific to the keywords, location, and language parameters specified in a POST request.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/amazon/bulk_search_volume/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs amazon-product-competitors-live`** - This endpoint will provide you with a list of products that intersect with a target asin in Amazon SERPs. The data can help you identify product competitors for any listing published on Amazon. The returned results are specific to the asin as well as the location and language parameters specified in a POST request.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/amazon/product_competitors/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs amazon-product-keyword-intersections-live`** - This endpoint will provide you with a list of keywords for which the target products intersect in Amazon SERP. The returned results are specific to the asins specified in a POST request. Learn more about ASIN in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/amazon/product_keyword_intersections/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs amazon-product-rank-overview-live`** - This endpoint will provide you with ranking data from organic and paid Amazon SERPs for the target products. The returned results are specific to the asins specified in a POST request. Learn more about ASIN in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/amazon/product_rank_overview/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs amazon-ranked-keywords-live`** - This endpoint will provide you with a list of keywords the target product ranks for on Amazon. The returned results are specific to the asin specified in a POST request. Learn more about ASIN in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/amazon/ranked_keywords/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs amazon-related-keywords-live`** - The Related Keywords endpoint provides keywords appearing in the
  "Related Searches" section on Amazon.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/amazon/related_keywords/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs apple-app-competitors-live`** - This endpoint will provide you with a list of mobile applications that intersect with the target app for its ranking keywords on App Store. You will obtain the IDs of competitor apps along with search volume and ranking data on competitor ranking keywords.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/apple/app_competitors/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs apple-app-intersection-live`** - This endpoint will provide you with a list of keywords for which the mobile applications specified in the app_ids object rank within the same App Store SERP.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/apple/app_intersection/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs apple-bulk-app-metrics-live`** - This endpoint will provide you with ranking metrics for up to 1000 App Store applications.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/apple/bulk_app_metrics/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs apple-keywords-for-app-live`** - This endpoint will provide you with a list of keywords for which the target app ranks on App Store. You will obtain keyword data and discover the app’s ranking position for each returned keyword.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/apple/keywords_for_app/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs available-filters`** - Here you will find all the necessary information about filters that can be used with DataForSEO Labs API endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/filters/?bash'
- **`dataforseo-pp-cli dataforseo-labs categories`** - We use Google product and service categories. This endpoint will provide you with the full list of available categories.
You can also download the CSV file by this link.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/categories_list/?bash'
- **`dataforseo-pp-cli dataforseo-labs errors`** - By calling this endpoint you will receive information about the DataForSEO Labs API tasks that returned an error within the past 7 days.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/errors/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-app-competitors-live`** - This endpoint will provide you with a list of mobile applications that intersect with the target app for its ranking keywords on Google Play. You will obtain the IDs of competitor apps along with search volume and ranking data on competitor ranking keywords.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/app_competitors/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-app-intersection-live`** - This endpoint will provide you with a list of keywords for which the mobile applications specified in the app_ids object rank within the same Google Play SERP.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/app_intersection/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-available-history`** - By calling this endpoint, you will find obtain a list of dates available for setting in the first_date and second_date fields of the Domain Metrics by Categories endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/available_history/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-bulk-app-metrics-live`** - This endpoint will provide you with ranking metrics for up to 1000 Google Play applications.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/bulk_app_metrics/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-bulk-keyword-difficulty-live`** - This endpoint will provide you with the Keyword Difficulty metric for a maximum of 1,000 keywords in one API request. Keyword Difficulty stands for the relative difficulty of ranking in the first top-10 organic results for the related keyword. Keyword Difficulty in DataForSEO API responses indicates the chance of getting in top-10 organic results for a keyword on a logarithmic scale from 0 to 100.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/bulk_keyword_difficulty/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-bulk-traffic-estimation-live`** - This endpoint will provide you with estimated monthly traffic volumes for up to 1,000 domains, subdomains, or webpages. Along with organic search traffic estimations, you will also get separate values for paid search, featured snippet, and local pack results.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/bulk_traffic_estimation/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-categories-for-domain-live`** - This endpoint will provide you with Google product or service categories that include keywords the domain ranks for in search. Furthermore, you will obtain general rankings and traffic data for the keywords under a certain category.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/categories_for_domain/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-categories-for-keywords-languages`** - Using this endpoint you can get the full list of languages supported for the Google Categories for Keywords endpoint of DataForSEO Labs API.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/categories_for_keywords/languages/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-categories-for-keywords-live`** - This endpoint will provide you with Google product and service categories related for each specified keyword. You can indicate a maximum of 1,000 keywords in one API request.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/categories_for_keywords/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-competitors-domain-live`** - This endpoint will provide you with a full overview of ranking and traffic data of the competitor domains from organic and paid search. In addition to that, you will get the metrics specific to the keywords both competitor domains and your domain rank for within the same SERP.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/competitors_domain/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-domain-intersection-live`** - This endpoint will provide you with the keywords for which both specified domains rank within the same SERP. You will get search volume, competition, cost-per-click and other data on each intersecting keyword. Along with that, you will get data on the first and second domain’s SERP element discovered for this keyword, as well as the estimated traffic volume and cost of ad traffic. Domain Intersection endpoint supports organic, paid, local pack, and featured snippet results.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/domain_intersection/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-domain-metrics-by-categories-live`** - This endpoint will provide you with dynamics of change in metrics of domains relevant to the specified product and service categories. You will receive historical ranking data from Google SERPs, along with valuable current and historical domain metrics, such as ETV, impressions ETV, estimated paid traffic cost, the total count of SERPs that contain domains, and more.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/domain_metrics_by_categories/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-domain-rank-overview-live`** - This endpoint will provide you with ranking and traffic data from organic and paid search for the specified domain. You will be able to review the domain ranking distribution in SERPs as well as estimated monthly traffic volume for both organic and paid results.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/domain_rank_overview/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-historical-bulk-traffic-estimation-live`** - This endpoint will provide you with historical monthly traffic volumes for up to 1,000 domains collected within the specified time range through October 2020. If you do not specify the range, data will be returned for the previous 12 months. Along with organic search traffic estimations, you will also get separate values for paid search, featured snippet, and local pack results.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/historical_bulk_traffic_estimation/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-historical-keyword-data-live`** - This endpoint provides Google historical keyword data for specified keywords, including search volume, cost-per-click, competition values for paid search, monthly searches, and search volume trends. You can get historical keyword  data since August, 2021, depending on keywords along with location and language combination. You can find the list of supported locations and languages here.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/historical_keyword_data/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-historical-rank-overview-live`** - This endpoint will provide you with historical data on rankings and traffic of the specified domain, such as domain ranking distribution in SERPs and estimated monthly traffic volume for both organic and paid results.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/historical_rank_overview/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-historical-serps-live`** - This endpoint will provide you with Google SERPs collected within the specified time frame. You will also receive a complete overview of featured snippets and other extra elements that were present within the specified dates. The data will allow you to analyze the dynamics of keyword rankings over time for the specified keyword and location.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/historical_serps/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-keyword-ideas-live`** - The Keyword Ideas endpoint provides search terms that are relevant to the product or service categories of the specified keywords. The algorithm selects the keywords which fall into the same categories as the seed keywords specified in a POST array.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/keyword_ideas/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-keyword-overview-live`** - This endpoint provides Google keyword data for specified keywords. For each keyword, you will receive current cost-per-click, competition values for paid search, search volume, search intent, monthly searches, as well as SERP and backlink information. Additionally, you can obtain clickstream data, such as clickstream search volume, by specifying the include_clickstream_data parameter.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/keyword_overview/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-keyword-suggestions-live`** - The Keyword Suggestions endpoint provides search queries that include the specified seed keyword.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/keyword_suggestions/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-keywords-for-app-live`** - This endpoint will provide you with a list of keywords for which the target app ranks on Google Play. You will obtain keyword data and discover the app’s ranking position for each returned keyword.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/keywords_for_app/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-keywords-for-categories-live`** - This endpoint will provide you with a list of keywords relevant to the specified product categories. You will get the search volume rate for the last month, search volume trend for the previous 12 months, as well as current cost-per-click and competition values for each keyword.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/keywords_for_categories/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-keywords-for-site-live`** - The Keywords For Site endpoint will provide you with a list of keywords relevant to the target domain. Each keyword is supplied with relevant categories, search volume data for the last month, cost-per-click, competition, and search volume trend values for the past 12 months.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/keywords_for_site/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-page-intersection-live`** - This endpoint will provide you with the keywords for which specified pages rank within the same SERP. You will get search volume, competition, cost-per-click data on each intersecting keyword. Along with that, you will get data on SERP elements that specified pages rank for in search results, as well as the estimated traffic volume and cost of ad traffic. Page Intersection endpoint supports organic, paid, local pack and featured snippet results.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/page_intersection/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-ranked-keywords-live`** - This endpoint will provide you with the list of keywords that any domain or webpage is ranking for. You will also get SERP elements related to the keyword position, as well as monthly searches and other data relevant to the returned keywords.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/ranked_keywords/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-related-keywords-live`** - The Related Keywords endpoint provides keywords appearing in the
  "searches related to" SERP element
You can get up to 4680 keyword ideas by specifying the search depth. Each related keyword comes with the list of relevant product categories, search volume rate for the last month, search volume trend for the previous 12 months, as well as current cost-per-click and competition values.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/related_keywords/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-relevant-pages-live`** - for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/relevant_pages/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-search-intent-live`** - This endpoint will provide you with search intent data for up to 1,000 keywords. For each keyword that you specify when setting a task, the API will return the keyword’s search intent and intent probability. Besides the highest probable search intent, the results will also provide you with other likely search intent(s) and their probability.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/search_intent/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-serp-competitors-live`** - This endpoint will provide you with a list of domains ranking for the keywords you specify. You will also get SERP rankings, rating, estimated traffic volume, and visibility values the provided domains gain from the specified keywords.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/serp_competitors/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-subdomains-live`** - This endpoint will provide you with a list of subdomains of the specified domain, along with the ranking distribution across organic and paid search. In addition to that, you will also get the estimated traffic volume of subdomains based on search volume and impressions.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/subdomains/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs google-top-searches-live`** - The Top Searches endpoint of DataForSEO Labs API can provide you with over 7 billion keywords from the DataForSEO Keyword Database. Each keyword in the API response is provided with a set of relevant keyword data with Google Ads metrics, product categories, and Google SERP data.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/google/top_searches/live/?bash'
- **`dataforseo-pp-cli dataforseo-labs id-list`** - This endpoint is designed to provide you with a list of IDs and metadata for all DataForSEO Labs tasks created within the specified time period, including both successful and uncompleted tasks.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/id_list/?bash'
- **`dataforseo-pp-cli dataforseo-labs locations-and-languages`** - Using this endpoint you can get the full list of locations and languages supported in DataForSEO Labs API. Available sources currently include Google, Bing, and Amazon search engines. However, you should note that Amazon and Bing locations and languages are currently limited to the US/English.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/locations_and_languages/?bash'
- **`dataforseo-pp-cli dataforseo-labs status`** - By calling this endpoint, you will find out when the DataForSEO Labs data was last updated. The API response will provide separate update dates for the Google, Bing, and Amazon endpoints of DataForSEO Labs API.
for more info please visit 'https://docs.dataforseo.com/v3/dataforseo_labs/status/?bash'

### domain-analytics

Manage domain analytics

- **`dataforseo-pp-cli domain-analytics errors`** - By calling this endpoint you will receive information about the Domain Analytics API tasks that returned an error within the past 7 days.
for more info please visit 'https://docs.dataforseo.com/v3/domain_analytics/errors/?bash'
- **`dataforseo-pp-cli domain-analytics id-list`** - This endpoint is designed to provide you with a list of IDs and metadata for all Domain Analytics tasks created within the specified time period, including both successful and uncompleted tasks.
for more info please visit 'https://docs.dataforseo.com/v3/domain_analytics/id_list/?bash'
- **`dataforseo-pp-cli domain-analytics technologies-aggregation-technologies-live`** - The Aggregation Technologies endpoint will provide you with a list of the most popular technologies websites use alongside the technologies you specify. Alternatively, you can specify technology categories or groups to obtain wider stats.
for more info please visit 'https://docs.dataforseo.com/v3/domain_analytics/technologies/aggregation_technologies/live/?bash'
- **`dataforseo-pp-cli domain-analytics technologies-available-filters`** - Here you will find all the necessary information about filters that can be used with Domain Analytics Technologies API endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/domain_analytics/technologies/filters/?bash'
- **`dataforseo-pp-cli domain-analytics technologies-domain-technologies-live`** - Using this endpoint you will get a list of technologies used in a particular domain.
for more info please visit 'https://docs.dataforseo.com/v3/domain_analytics/technologies/domain_technologies/live/?bash'
- **`dataforseo-pp-cli domain-analytics technologies-domains-by-html-terms-live`** - This endpoint provides domains based on the HTML terms they use on their homepage. In addition to the list of domains, you will also get their technology profiles, the country and language they belong to, and other related data.
for more info please visit 'https://docs.dataforseo.com/v3/domain_analytics/technologies/domains_by_html_terms/live/?bash'
- **`dataforseo-pp-cli domain-analytics technologies-domains-by-technology-live`** - This endpoint provides domains based on the technology they use. In addition to the list of domains, you will also get their technology profiles, the country and language they belong to, and other related data.
for more info please visit 'https://docs.dataforseo.com/v3/domain_analytics/technologies/domains_by_technology/live/?bash'
- **`dataforseo-pp-cli domain-analytics technologies-languages`** - You will receive the list of languages by calling this API.
 
As a response of the API server, you will receive JSON-encoded data containing a tasks array with the information specific to the set tasks.
for more info please visit 'https://docs.dataforseo.com/v3/domain_analytics/technologies/languages/?bash'
- **`dataforseo-pp-cli domain-analytics technologies-locations`** - You will receive the list of locations by this API call.
for more info please visit 'https://docs.dataforseo.com/v3/domain_analytics/technologies/locations/?bash'
- **`dataforseo-pp-cli domain-analytics technologies-technologies`** - This endpoint will provide you with the full list of available technologies structured by technology groups and categories each particular technology belongs to.
for more info please visit 'https://docs.dataforseo.com/v3/domain_analytics/technologies/technologies/?bash'
- **`dataforseo-pp-cli domain-analytics technologies-technologies-summary-live`** - The Technologies Summary endpoint will provide you with the number of domains across different countries and languages that use the specified technology names, technology groups, or technology categories.
for more info please visit 'https://docs.dataforseo.com/v3/domain_analytics/technologies/technologies_summary/live/?bash'
- **`dataforseo-pp-cli domain-analytics technologies-technology-stats-live`** - The Technology Stats endpoint will provide you with historical data on the number of domains across different countries and languages that use the specified technology.
for more info please visit 'https://docs.dataforseo.com/v3/domain_analytics/technologies/technology_stats/live/?bash'
- **`dataforseo-pp-cli domain-analytics whois-available-filters`** - Here you will find all the necessary information about filters that can be used with Domain Analytics Whois API.
for more info please visit 'https://docs.dataforseo.com/v3/domain_analytics/whois/filters/?bash'
- **`dataforseo-pp-cli domain-analytics whois-overview-live`** - This endpoint will provide you with Whois data enriched with backlink stats, and ranking and traffic info from organic and paid search results. Using this endpoint you will be able to get all these data for the domains matching the parameters you specify in the request.
for more info please visit 'https://docs.dataforseo.com/v3/domain_analytics/whois/overview/live/?bash'

### keywords-data

Manage keywords data

- **`dataforseo-pp-cli keywords-data bing-audience-estimation-industries`** - By calling this API you will receive the list of industries with industry_id supported by Bing Ads Audience Estimation endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/audience_estimation/industries/?bash'
- **`dataforseo-pp-cli keywords-data bing-audience-estimation-job-functions`** - By calling this API you will receive the list of job functions with job_function_id supported by Bing Ads Audience Estimation endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/audience_estimation/job_functions/?bash'
- **`dataforseo-pp-cli keywords-data bing-audience-estimation-live`** - This endpoint provides estimated audience size for an ad campaign based on specified targeting criteria. It returns data on the total estimated audience, such as suggested bid and budget for an ad campaign and estimated engagement metrics.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/audience_estimation/live/?bash'
- **`dataforseo-pp-cli keywords-data bing-audience-estimation-task-get`** - for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/audience_estimation/task_get/?bash'
- **`dataforseo-pp-cli keywords-data bing-audience-estimation-task-post`** - This endpoint provides estimated audience size for an ad campaign based on specified targeting criteria. It returns data on the total estimated audience, such as suggested bid and budget for an ad campaign and estimated engagement metrics.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/audience_estimation/task_post/?bash'
- **`dataforseo-pp-cli keywords-data bing-audience-estimation-tasks-ready`** - This endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/audience_estimation/tasks_ready/?bash'
- **`dataforseo-pp-cli keywords-data bing-keyword-performance-live`** - You can receive a set of keyword performance stats for a group of keywords depending on the specified match type, location and language parameters. Ad position, clicks, impressions, and other keyword metrics are aggregated for the last month for one or all of the following device types: mobile, desktop, tablet.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/keyword_performance/live/?bash'
- **`dataforseo-pp-cli keywords-data bing-keyword-performance-locations-and-languages`** - Using this endpoint you can get the full list of locations and languages supported in Keyword Performance endpoints of Bing Keywords Data API.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/keyword_performance/locations_and_languages/?bash'
- **`dataforseo-pp-cli keywords-data bing-keyword-performance-task-get`** - You can receive a set of keyword performance stats for a group of keywords depending on the specified match type, location and language parameters. Ad position, clicks, impressions, and other keyword metrics are aggregated for the last month for one or all of the following device types: mobile, desktop, tablet.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/keyword_performance/task_get/?bash'
- **`dataforseo-pp-cli keywords-data bing-keyword-performance-task-post`** - You can receive a set of keyword performance stats for a group of keywords depending on the specified match type, location and language parameters. Ad position, clicks, impressions, and other keyword metrics are aggregated for the last month for one or all of the following device types: mobile, desktop, tablet.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/keyword_performance/task_post/?bash'
- **`dataforseo-pp-cli keywords-data bing-keyword-performance-tasks-ready`** - This endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/keyword_performance/tasks_ready/?bash'
- **`dataforseo-pp-cli keywords-data bing-keywords-for-keywords-live`** - This endpoint will select the relevant keywords for the specified ones. Set up to 200 keywords and get the results, which are suggested by Bing Ads for your query. You can get up to 3000 keyword suggestions using this function.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/keywords_for_keywords/live/?bash'
- **`dataforseo-pp-cli keywords-data bing-keywords-for-keywords-task-get`** - This endpoint will select relevant keywords for the specified terms. Set up to 200 keywords and get the results, which are suggested by Bing Ads for your query. You can get up to 3000 keyword suggestions using this function.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/keywords_for_keywords/task_get/?bash'
- **`dataforseo-pp-cli keywords-data bing-keywords-for-keywords-task-post`** - This endpoint will select relevant keywords for the specified terms. Set up to 200 keywords and get the results, which are suggested by Bing Ads for your query. You can get up to 3000 keyword suggestions using this function.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/keywords_for_keywords/task_post/?bash'
- **`dataforseo-pp-cli keywords-data bing-keywords-for-keywords-tasks-ready`** - This endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/keywords_for_keywords/tasks_ready/?bash'
- **`dataforseo-pp-cli keywords-data bing-keywords-for-site-live`** - This endpoint will provide you with a list of keywords relevant to the specified URL along with their search volume for the last month, search volume trend for up to 24 past months (for estimating search volume dynamics), current cost-per-click and competition values for paid search. The maximum number of returned keywords is 3000.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/keywords_for_site/live/?bash'
- **`dataforseo-pp-cli keywords-data bing-keywords-for-site-task-get`** - This endpoint will provide you with a list of keywords relevant to the specified website along with their search volume for the last month, search volume trend for the last year (for estimating search volume dynamics), current cost-per-click and competition level for paid search. The maximum number of returned keywords is 3000.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/keywords_for_site/task_get/?bash'
- **`dataforseo-pp-cli keywords-data bing-keywords-for-site-task-post`** - This endpoint will provide you with a list of keywords relevant to the specified website along with their search volume for the last month, search volume trend for up to 24 past months (for estimating search volume dynamics), current cost-per-click and competition level for paid search. The maximum number of returned keywords is 3000.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/keywords_for_site/task_post/?bash'
- **`dataforseo-pp-cli keywords-data bing-keywords-for-site-tasks-ready`** - This endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/keywords_for_site/tasks_ready/?bash'
- **`dataforseo-pp-cli keywords-data bing-languages`** - By calling this API you will receive the list of languages supported by Bing Ads API.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/languages/?bash'
- **`dataforseo-pp-cli keywords-data bing-locations`** - By calling this API you will receive the list of locations supported in Bing Ads API.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/locations/?bash'
- **`dataforseo-pp-cli keywords-data bing-search-volume-history-live`** - This endpoint will provide you with historical search volume data for up to 1000 keywords in one request. You can get search volume for keywords in monthly, weekly, or daily format and specify the device type.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/search_volume_history/live/?bash'
- **`dataforseo-pp-cli keywords-data bing-search-volume-history-locations-and-languages`** - By calling this API you will receive the list of locations and languages supported by Bing ‘Search Volume History’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/search_volume_history/locations_and_languages/?bash'
- **`dataforseo-pp-cli keywords-data bing-search-volume-history-task-get`** - for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/search_volume_history/task_get/?bash'
- **`dataforseo-pp-cli keywords-data bing-search-volume-history-task-post`** - This endpoint will provide you with historical search volume data for up to 1000 keywords in one request. You can get search volume for keywords in monthly, weekly, or daily format and specify the device type.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/search_volume_history/task_post/?bash'
- **`dataforseo-pp-cli keywords-data bing-search-volume-history-tasks-ready`** - This endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/search_volume_history/tasks_ready/?bash'
- **`dataforseo-pp-cli keywords-data bing-search-volume-live`** - This endpoint will provide you with search volume data for the last month, search volume trend for up to 24 past months (that will let you estimate search volume dynamics), current cost-per-click and competition values for paid search.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/search_volume/live/?bash'
- **`dataforseo-pp-cli keywords-data bing-search-volume-task-get`** - for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/search_volume/task_get/?bash'
- **`dataforseo-pp-cli keywords-data bing-search-volume-task-post`** - This endpoint will provide you with search volume data for the last month, search volume trend for up to 24 past months (that will let you estimate search volume dynamics), current cost-per-click and competition values for paid search.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/search_volume/task_post/?bash'
- **`dataforseo-pp-cli keywords-data bing-search-volume-tasks-ready`** - This endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/bing/search_volume/tasks_ready/?bash'
- **`dataforseo-pp-cli keywords-data clickstream-data-bulk-search-volume-live`** - The Bulk Clickstream Search Volume endpoint of DataForSEO Keywords Data API is designed to provide clickstream-based search volume data for up to 1000 keywords in a single Live request. What’s more, it offers historical search volume values for up to 12 months (depending on keywords, location, and language parameters).
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/clickstream_data/bulk_search_volume/live/?bash'
- **`dataforseo-pp-cli keywords-data clickstream-data-dataforseo-search-volume-live`** - for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/clickstream_data/dataforseo_search_volume/live/?bash'
- **`dataforseo-pp-cli keywords-data clickstream-data-global-search-volume-live`** - The Clickstream Global Search Volume endpoint of DataForSEO Keywords Data API is designed to provide clickstream-based search volume data for up to 1000 keywords in a single Live request. What’s more, it offers geographical distribution of clickstream search volume values across all available locations.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/clickstream_data/global_search_volume/live/?bash'
- **`dataforseo-pp-cli keywords-data clickstream-data-locations-and-languages`** - Using this endpoint you can get the full list of locations and languages supported in DataForSEO Clickstream Data API.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/clickstream_data/locations_and_languages/?bash'
- **`dataforseo-pp-cli keywords-data dataforseo-trends-demography-live`** - This endpoint will provide you with the demographic breakdown (by age and gender) of keyword popularity per each specified term based on DataForSEO Trends data. You can check keyword trends for Google Search, Google News, and Google Shopping.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/dataforseo_trends/demography/live/?bash'
- **`dataforseo-pp-cli keywords-data dataforseo-trends-explore-live`** - This endpoint will provide you with the keyword popularity data from DataForSEO Trends. You can check keyword trends for Google Search, Google News, and Google Shopping.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/dataforseo_trends/explore/live/?bash'
- **`dataforseo-pp-cli keywords-data dataforseo-trends-locations`** - You will receive the list of DataForSEO Trends locations by calling this API. You can filter the list of locations by country when setting a task. Please note that the minimum geographic scope supported for the DataForSEO Trends API is country level.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/dataforseo_trends/locations/?bash'
- **`dataforseo-pp-cli keywords-data dataforseo-trends-locations-country`** - You will receive the list of DataForSEO Trends locations by calling this API. You can filter the list of locations by country when setting a task. Please note that the minimum geographic scope supported for the DataForSEO Trends API is country level.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/dataforseo_trends/locations/?bash'
- **`dataforseo-pp-cli keywords-data dataforseo-trends-merged-data-live`** - This endpoint will provide you with the keyword popularity data from DataForSEO Trends. In addition to keyword popularity rate over the given time range, you will get location-specific keyword popularity data, and a demographic breakdown of keyword popularity per each specified term along with comparative values.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/dataforseo_trends/merged_data/live/?bash'
- **`dataforseo-pp-cli keywords-data dataforseo-trends-subregion-interests-live`** - This endpoint will provide you with location-specific keyword popularity data from DataForSEO Trends. You can check keyword trends for Google Search, Google News, and Google Shopping.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/dataforseo_trends/subregion_interests/live/?bash'
- **`dataforseo-pp-cli keywords-data errors`** - By calling this endpoint you will receive information about the Keywords Data API tasks that returned an error within the past 7 days.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/errors/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-ad-traffic-by-keywords-live`** - for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/ad_traffic_by_keywords/live/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-ad-traffic-by-keywords-task-get`** - for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/ad_traffic_by_keywords/task_get/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-ad-traffic-by-keywords-task-post`** - for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/ad_traffic_by_keywords/task_post/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-ad-traffic-by-keywords-tasks-ready`** - This endpoint is designed to provide you with a list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/ad_traffic_by_keywords/tasks_ready/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-keywords-for-keywords-live`** - Note that Google Ads Keywords Data API is based on the latest version of the Google Ads API that has replaced legacy Google AdWords API. If you’re using DataForSEO Google AdWords API, you need to upgrade to DataForSEO Google Ads API.
‌‌
This endpoint will provide relevant keywords for the specified terms. Set up to 20 keywords in the keywords array and get keyword suggestions from Google Ads.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/keywords_for_keywords/live/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-keywords-for-keywords-task-get`** - Note that Google Ads Keywords Data API is based on the latest version of the Google Ads API that has replaced legacy Google AdWords API. If you’re using DataForSEO Google AdWords API, you need to upgrade to DataForSEO Google Ads API.
‌
This endpoint will select relevant keywords for the specified terms. Set up to 20 keywords and get the results, which are suggested by Google Ads for your query.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/keywords_for_keywords/task_get/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-keywords-for-keywords-task-post`** - Note that Google Ads Keywords Data API is based on the latest version of the Google Ads API that has replaced legacy Google AdWords API. If you’re using DataForSEO Google AdWords API, you need to upgrade to DataForSEO Google Ads API.
‌‌
This endpoint will provide relevant keywords for the specified terms. Set up to 20 keywords in the keywords array and get keyword suggestions from Google Ads. You can get up to 20,000 keyword suggestions with all essential keyword data in response to one request.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/keywords_for_keywords/task_post/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-keywords-for-keywords-tasks-ready`** - This endpoint is designed to provide you with a list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/keywords_for_keywords/tasks_ready/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-keywords-for-site-live`** - Note that Google Ads Keywords Data API is based on the latest version of the Google Ads API that has replaced legacy Google AdWords API. If you’re using DataForSEO Google AdWords API, you need to upgrade to DataForSEO Google Ads API.
‌‌
This endpoint will provide you with a list of keywords relevant to the specified domain along with their bids, search volumes for the last month, search volume trends for the last year (for estimating search volume dynamics), and competition levels.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/keywords_for_site/live/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-keywords-for-site-task-get`** - Note that Google Ads Keywords Data API is based on the latest version of the Google Ads API that has replaced legacy Google AdWords API. If you’re using DataForSEO Google AdWords API, you need to upgrade to DataForSEO Google Ads API.
‌‌
This endpoint will provide you with a list of keywords relevant to the specified domain along with their bids, search volumes for the last month, search volume trends for the last year (for estimating search volume dynamics), and competition levels.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/keywords_for_site/task_get/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-keywords-for-site-task-post`** - for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/keywords_for_site/task_post/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-keywords-for-site-tasks-ready`** - This endpoint is designed to provide you with a list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/keywords_for_site/tasks_ready/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-languages`** - By calling this API you will receive the list of languages supported by Keywords Data API.
‌
‌‌As a response of the API server, you will receive JSON-encoded data containing a tasks array with the information about available languages.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/languages/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-locations`** - We use Google Geographical Targeting. You can refer to Google Ads Target Types page to review the full list of possible location types. With Keywords Data API, you can select any location type supported by Google, except for “Okrug”.
Postal Codes can be used to set a task, albeit API response will not return data for such tasks.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/locations/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-locations-country`** - We use Google Geographical Targeting. You can refer to Google Ads Target Types page to review the full list of possible location types. With Keywords Data API, you can select any location type supported by Google, except for “Okrug”.
Postal Codes can be used to set a task, albeit API response will not return data for such tasks.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/locations/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-search-volume-live`** - Note that Google Ads Keywords Data API is based on the latest version of the Google Ads API that has replaced legacy Google AdWords API. If you’re using DataForSEO Google AdWords API, you need to upgrade to DataForSEO Google Ads API.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/search_volume/live/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-search-volume-task-get`** - Note that Google Ads Keywords Data API is based on the latest version of the Google Ads API that has replaced legacy Google AdWords API. If you’re using DataForSEO Google AdWords API, you need to upgrade to DataForSEO Google Ads API.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/search_volume/task_get/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-search-volume-task-post`** - Note that Google Ads Keywords Data API is based on the latest version of the Google Ads API that has replaced legacy Google AdWords API. If you’re using DataForSEO Google AdWords API, you need to upgrade to DataForSEO Google Ads API.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/search_volume/task_post/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-search-volume-tasks-ready`** - This endpoint is designed to provide you with a list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/search_volume/tasks_ready/?bash'
- **`dataforseo-pp-cli keywords-data google-ads-status`** - By calling this endpoint, you will know if Google updated keyword data for the previous month. Generally, Google updates keyword data in the middle of the month. So, if Google updated its data in October, you would be able to see the actual search volume, cost-per-click, competition, and other metrics for September. If Google didn’t update its data in October, the latest information would be available for August.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_ads/status/?bash'
- **`dataforseo-pp-cli keywords-data google-trends-categories`** - By calling this API you will receive the list of categories supported by Google Trends API.
‌
‌‌As a response of the API server, you will receive JSON-encoded data containing a tasks array with the information about available categories.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_trends/categories/?bash'
- **`dataforseo-pp-cli keywords-data google-trends-explore-live`** - This endpoint will provide you with the keyword popularity data from the ‘Explore’ feature of Google Trends. You can check keyword trends for Google Search, Google News, Google Images, Google Shopping, and YouTube.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_trends/explore/live/?bash'
- **`dataforseo-pp-cli keywords-data google-trends-explore-task-get`** - for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_trends/explore/task_get/?bash'
- **`dataforseo-pp-cli keywords-data google-trends-explore-task-post`** - This endpoint will provide you with the keyword popularity data from the ‘Explore’ feature of Google Trends. You can check keyword trends for Google Search, Google News, Google Images, Google Shopping, and YouTube.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_trends/explore/task_post/?bash'
- **`dataforseo-pp-cli keywords-data google-trends-explore-tasks-ready`** - This endpoint is designed to provide you with a list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_trends/explore/tasks_ready/?bash'
- **`dataforseo-pp-cli keywords-data google-trends-languages`** - By calling this API you will receive the list of languages supported by Google Trends API.
‌
‌‌As a response of the API server, you will receive JSON-encoded data containing a tasks array with the information about available languages.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_trends/languages/?bash'
- **`dataforseo-pp-cli keywords-data google-trends-locations`** - for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_trends/locations/?bash'
- **`dataforseo-pp-cli keywords-data google-trends-locations-country`** - for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/google_trends/locations/?bash'
- **`dataforseo-pp-cli keywords-data id-list`** - This endpoint is designed to provide you with a list of IDs and metadata for all Keywords Data tasks created within the specified time period, including both successful and uncompleted tasks.
for more info please visit 'https://docs.dataforseo.com/v3/keywords_data/id_list/?bash'

### merchant

Manage merchant

- **`dataforseo-pp-cli merchant amazon-asin-task-get-advanced`** - This endpoint will provide you with information about the product and ASINs of all its modifications listed on Amazon.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/amazon/asin/task_get/advanced/?bash'
- **`dataforseo-pp-cli merchant amazon-asin-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/merchant/amazon/asin/task_get/html/?bash'
- **`dataforseo-pp-cli merchant amazon-asin-task-post`** - This endpoint will provide you with a full list of ASINs assigned to different modifications of a product.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/amazon/asin/task_post/?bash'
- **`dataforseo-pp-cli merchant amazon-asin-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/amazon/asin/tasks_ready/?bash'
- **`dataforseo-pp-cli merchant amazon-languages`** - You will receive the list of supported Amazon languages by calling this API.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/amazon/languages/?bash'
- **`dataforseo-pp-cli merchant amazon-locations`** - You will receive the list of supported Amazon locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/amazon/locations/?bash'
- **`dataforseo-pp-cli merchant amazon-locations-country`** - You will receive the list of supported Amazon locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/amazon/locations/?bash'
- **`dataforseo-pp-cli merchant amazon-products-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/merchant/amazon/products/task_get/advanced/?bash'
- **`dataforseo-pp-cli merchant amazon-products-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/merchant/amazon/products/task_get/html/?bash'
- **`dataforseo-pp-cli merchant amazon-products-task-post`** - This endpoint provides results from Amazon product listings according to the specified keyword (product name), location, and language parameters.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/amazon/products/task_post/?bash'
- **`dataforseo-pp-cli merchant amazon-products-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/amazon/products/tasks_ready/?bash'
- **`dataforseo-pp-cli merchant amazon-sellers-task-get-advanced`** - This endpoint provides a list of sellers of the specified product on Amazon. The data provided for each seller includes related product condition, pricing, shipment, and rating details.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/amazon/sellers/task_get/advanced/?bash'
- **`dataforseo-pp-cli merchant amazon-sellers-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/merchant/amazon/sellers/task_get/html/?bash'
- **`dataforseo-pp-cli merchant amazon-sellers-task-post`** - This endpoint provides a list of sellers of the specified product on Amazon. The data provided for each seller includes related product condition, pricing, shipment, and rating details.
The results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/amazon/sellers/task_post/?bash'
- **`dataforseo-pp-cli merchant amazon-sellers-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/amazon/sellers/tasks_ready/?bash'
- **`dataforseo-pp-cli merchant errors`** - By calling this endpoint you will receive information about the Merchant API tasks that returned an error within the past 7 days.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/errors/?bash'
- **`dataforseo-pp-cli merchant google-languages`** - You will receive the list of supported Google Shopping languages by calling this API.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/google/languages/?bash'
- **`dataforseo-pp-cli merchant google-locations`** - for more info please visit 'https://docs.dataforseo.com/v3/merchant/google/locations/?bash'
- **`dataforseo-pp-cli merchant google-locations-country`** - for more info please visit 'https://docs.dataforseo.com/v3/merchant/google/locations/?bash'
- **`dataforseo-pp-cli merchant google-product-info-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/merchant/google/product_info/task_get/advanced/?bash'
- **`dataforseo-pp-cli merchant google-product-info-task-post`** - This endpoint provides data on a product listed on Google Shopping, including product description, images, rating, variations, specifications and sellers. In order to set a task, you have to specify one of the following fields:  product_id, data_docid, or gid.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/google/product_info/task_post/?bash'
- **`dataforseo-pp-cli merchant google-product-info-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/google/product_info/tasks_ready/?bash'
- **`dataforseo-pp-cli merchant google-products-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/merchant/google/products/task_get/advanced/?bash'
- **`dataforseo-pp-cli merchant google-products-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/merchant/google/products/task_get/html/?bash'
- **`dataforseo-pp-cli merchant google-products-task-post`** - Google Shopping Products endpoint will provide you with the list of products found on Google Shopping for the specified query. The results include product title, description in Google Shopping SERP, product rank, price, reviews and rating as well as the related domain.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/google/products/task_post/?bash'
- **`dataforseo-pp-cli merchant google-products-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/google/products/tasks_ready/?bash'
- **`dataforseo-pp-cli merchant google-sellers-ad-url`** - Google Shopping Sellers Ad URL is designed to provide you with a full URL of the advertisement containing all additional parameters set by the seller.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/google/sellers/ad_url/?bash'
- **`dataforseo-pp-cli merchant google-sellers-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/merchant/google/sellers/task_get/advanced/?bash'
- **`dataforseo-pp-cli merchant google-sellers-task-post`** - Google Shopping Sellers endpoint will provide you with the list of top 10 sellers that listed the specified product on Google Shopping. The provided data for each seller includes related product base and total price, shipment and purchase details and special offers. The results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/google/sellers/task_post/?bash'
- **`dataforseo-pp-cli merchant google-sellers-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/google/sellers/tasks_ready/?bash'
- **`dataforseo-pp-cli merchant id-list`** - This endpoint is designed to provide you with a list of IDs and metadata for all Merchant tasks created within the specified time period, including both successful and uncompleted tasks.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/id_list/?bash'
- **`dataforseo-pp-cli merchant tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/merchant/google/products/tasks_ready/?bash'

### on-page

Manage on page

- **`dataforseo-pp-cli on-page available-filters`** - OnPage API supports plenty of customizable crawling parameters that allow you to adapt the extraction of website data to your requirements and modify the thresholds for various performance indicators.
‌‌
Here you will find all the necessary information about filters and thresholds that can be used with DataForSEO OnPage API endpoints.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/filters_and_thresholds/?bash'
- **`dataforseo-pp-cli on-page content-parsing`** - This endpoint allows parsing the content on any page you specify and will return the structured content of the target page, including link URLs, anchors, headings, and textual content.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/content_parsing/?bash'
- **`dataforseo-pp-cli on-page content-parsing-live`** - This endpoint allows parsing the content on any page you specify and will return the structured content of the target page, including link URLs, anchors, headings, and textual content.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/content_parsing/live/?bash'
- **`dataforseo-pp-cli on-page duplicate-content`** - This endpoint returns a list of pages that have content similar to the page specified in the request. The response also contains data related to page performance and the similarity index that indicates how similar the compared pages are.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/duplicate_content/?bash'
- **`dataforseo-pp-cli on-page duplicate-tags`** - This endpoint returns a list of pages that contain duplicate title or description tags. The response also contains data related to page performance.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/duplicate_tags/?bash'
- **`dataforseo-pp-cli on-page errors`** - By calling this endpoint you will receive information about the OnPage API tasks that returned an error within the past 7 days.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/errors/?bash'
- **`dataforseo-pp-cli on-page force-stop`** - This endpoint is designed to force stop the crawl process of websites you specified in a task. The execution of all the tasks associated with the IDs indicated in your request to this endpoint will be stopped. You will still be able to obtain the data on pages that have been scanned until the crawling process was stopped.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/force_stop/?bash'
- **`dataforseo-pp-cli on-page id-list`** - This endpoint is designed to provide you with a list of IDs and metadata for all On-Page tasks created within the specified time period, including both successful and uncompleted tasks.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/id_list/?bash'
- **`dataforseo-pp-cli on-page instant-pages`** - Using this function you will get page-specific data with detailed information on how well a particular page is optimized for organic search.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/instant_pages/?bash'
- **`dataforseo-pp-cli on-page keyword-density`** - This endpoint will provide you with keyword density and keyword frequency data for terms appearing on the specified website or web page. You can filter and sort the data that will be retrieved with this API call.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/keyword_density/?bash'
- **`dataforseo-pp-cli on-page lighthouse-audits`** - The OnPage Lighthouse API is based on Google’s open-source Lighthouse project and provides data on the quality of web pages.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/lighthouse/audits/?bash'
- **`dataforseo-pp-cli on-page lighthouse-languages`** - You will receive the list of languages by calling this API.
 
As a response of the API server, you will receive JSON-encoded data containing a tasks array with the information specific to the set tasks.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/lighthouse/languages/?bash'
- **`dataforseo-pp-cli on-page lighthouse-live-json`** - The OnPage Lighthouse API is based on Google’s open-source Lighthouse project for measuring the quality of web pages and web apps.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/lighthouse/live/json/?bash'
- **`dataforseo-pp-cli on-page lighthouse-task-get-json`** - The OnPage Lighthouse API is based on Google’s open-source Lighthouse project for measuring the quality of web pages and web apps. This endpoint will provide you with the results of Lighthouse Audit. Use the id received in the response of your Task POST request to get the results. The response will include data about all categories and audits specified in the Task POST. By default, the response will include all available data about the webpage including its performance, accessibility, progressive web apps, SEO, and compliance with best practices.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/lighthouse/task_get/json/?bash'
- **`dataforseo-pp-cli on-page lighthouse-task-post`** - The OnPage Lighthouse API is based on Google’s open-source Lighthouse project for measuring the quality of web pages and web apps.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/lighthouse/task_post/?bash'
- **`dataforseo-pp-cli on-page lighthouse-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/lighthouse/tasks_ready/?bash'
- **`dataforseo-pp-cli on-page lighthouse-versions`** - OnPage Lighthouse API is based on Google’s open-source Lighthouse project and provides data on the quality of web pages.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/lighthouse/versions/?bash'
- **`dataforseo-pp-cli on-page links`** - This endpoint will provide you with a list of internal and external links detected on a target website.
The following link types are supported:
anchor – links that point to a specific portion of a webpage;
image – links that point to an image;
canonical – links that point to a canonical page;
meta – links with meta http-equiv=refresh ;
alternate – links with link rel="alternate" pointing to an alternative version of a webpage ;
redirect – links with redirect status.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/links/?bash'
- **`dataforseo-pp-cli on-page microdata`** - This endpoint is designed to validate structured JSON-LD data and Microdata. Using this function you will obtain microdata available on the specified page of the target website and detailed results of its validation.
To use this endpoint, set the validate_micromarkup parameter to true in the POST request to OnPage API.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/microdata/?bash'
- **`dataforseo-pp-cli on-page non-indexable`** - This endpoint returns a list of pages that are blocked from being indexed by Google and other search engines through robots.txt, HTTP headers, or meta tags settings.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/non_indexable/?bash'
- **`dataforseo-pp-cli on-page page-screenshot`** - Using this endpoint, you can capture a full high-quality screenshot of any webpage. In this way, you can review the target page as the DataForSEO crawler and Googlebot see it.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/page_screenshot/?bash'
- **`dataforseo-pp-cli on-page pages`** - This endpoint returns a list of crawled pages with on-page check-ups and other metrics related to the page performance.
Using this function you will get page-specific data with detailed information on how well your pages are optimized for search.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/pages/?bash'
- **`dataforseo-pp-cli on-page pages-by-resource`** - This endpoint will return the list of pages where a specific resource is located. Using this function you will also get the data related to the pages that contain a specified resource.
You can get the URL of a resource using the Resources endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/page_by_resource/?bash'
- **`dataforseo-pp-cli on-page raw-html`** - This endpoint returns the HTML of a page you indicate in the request.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/raw_html/?bash'
- **`dataforseo-pp-cli on-page redirect-chains`** - Redirect chains occur when there are at least two redirects between the initial URL and the destination URL. For example, if page A redirects to page B which redirects to page C, such a series of redirects is considered a redirect chain. Sometimes, if page B redirects back to page A, the redirect chain becomes closed and is considered a redirect loop.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/redirect_chains/?bash'
- **`dataforseo-pp-cli on-page resources`** - This endpoint will provide you with a list of resources, including images, scripts, stylesheets, and broken elements.
You will get a detailed overview of every resource found on the crawled pages.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/resources/?bash'
- **`dataforseo-pp-cli on-page summary`** - Using this function, you can get the overall information on a website as well as drill down into exact on-page issues of a website that has been scanned. As a result, you will know what functions to use for receiving detailed data for each of the found issues.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/summary/?bash'
- **`dataforseo-pp-cli on-page task-post`** - OnPage API checks websites for 60+ customizable on-page parameters defines and displays all found flaws and opportunities for optimization so that you can easily fix them. It checks meta tags, duplicate content, image tags, response codes, and other parameters on every page. You can find the full list of OnPage API check-up parameters in the Pages section.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/task_post/?bash'
- **`dataforseo-pp-cli on-page tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with a list of completed tasks, which results haven’t been collected yet.
for more info please visit 'https://docs.dataforseo.com/v3/on_page-tasks_ready/?bash'
- **`dataforseo-pp-cli on-page waterfall`** - This endpoint is designed to provide you with the page speed insights. Using this function you can get detailed information about the page loading time, time to secure connection, the time it takes to load page resources, and so on.
for more info please visit 'https://docs.dataforseo.com/v3/on_page/waterfall/?bash'

### serp

Manage serp

- **`dataforseo-pp-cli serp ai-summary`** - The purpose of the Live SERP API AI Summary endpoint is to provide a summary of the content found on any SERP and generate a response based on the user’s specified prompt.
To obtain results, you have to specify task_id, which you can find in the response to the POST request.
Learn more in our Help Center.
for more info please visit 'https://docs.dataforseo.com/v3/serp/ai_summary/?bash'
- **`dataforseo-pp-cli serp baidu-languages`** - You will receive the list of languages by calling this API. You can also download the full list of supported languages in the CSV format (last updated 2026-04-06).
for more info please visit 'https://docs.dataforseo.com/v3/serp/baidu/languages/?bash'
- **`dataforseo-pp-cli serp baidu-locations`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/serp/baidu/locations/?bash'
- **`dataforseo-pp-cli serp baidu-locations-country`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/serp/baidu/locations/?bash'
- **`dataforseo-pp-cli serp baidu-organic-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/baidu/organic/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp baidu-organic-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/baidu/organic/task_get/html/?bash'
- **`dataforseo-pp-cli serp baidu-organic-task-get-regular`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/baidu/organic/task_get/regular/?bash'
- **`dataforseo-pp-cli serp baidu-organic-task-post`** - Baidu SERP API provides top 10 search engine results. These results are specific to the selected location (see the List of Locations) and other settings.
for more info please visit 'https://docs.dataforseo.com/v3/serp/baidu/organic/task_post/?bash'
- **`dataforseo-pp-cli serp baidu-organic-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/baidu/organic/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp baidu-organic-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/baidu/organic/tasks_ready/?bash'
- **`dataforseo-pp-cli serp bing-languages`** - You will receive the list of languages by calling this API.
for more info please visit 'https://docs.dataforseo.com/v3/serp/bing/languages/?bash'
- **`dataforseo-pp-cli serp bing-locations`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/serp/bing/locations/?bash'
- **`dataforseo-pp-cli serp bing-locations-country`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/serp/bing/locations/?bash'
- **`dataforseo-pp-cli serp bing-organic-live-advanced`** - Live SERP provides real-time data on top 100 search engine results for the specified keyword, search engine, and location. This endpoint will supply a complete overview of featured snippets and other extra elements of SERPs.
for more info please visit 'https://docs.dataforseo.com/v3/serp/bing/organic/live/advanced/?bash'
- **`dataforseo-pp-cli serp bing-organic-live-html`** - Live SERP HTML provides a raw HTML page of search engine results for the specified keyword, search engine, and location.
for more info please visit 'https://docs.dataforseo.com/v3/serp/bing/organic/live/html/?bash'
- **`dataforseo-pp-cli serp bing-organic-live-regular`** - Live SERP provides real-time data on search engine results for the specified keyword, search engine, and location.
for more info please visit 'https://docs.dataforseo.com/v3/serp/bing/organic/live/regular/?bash'
- **`dataforseo-pp-cli serp bing-organic-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/bing/organic/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp bing-organic-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/bing/organic/task_get/html/?bash'
- **`dataforseo-pp-cli serp bing-organic-task-get-regular`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/bing/organic/task_get/regular/?bash'
- **`dataforseo-pp-cli serp bing-organic-task-post`** - SERP API provides search engine results. These results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/serp/bing/organic/task_post/?bash'
- **`dataforseo-pp-cli serp bing-organic-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/bing/organic/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp bing-organic-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/bing/organic/tasks_ready/?bash'
- **`dataforseo-pp-cli serp errors`** - By calling this endpoint you will receive information about the SERP API tasks that returned an error within the past 7 days.
for more info please visit 'https://docs.dataforseo.com/v3/serp/errors/?bash'
- **`dataforseo-pp-cli serp google-ads-advertisers-locations`** - As a response of the API server, you will receive JSON-encoded data containing a tasks array with the information specific to the set tasks.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ads_advertisers/locations/?bash'
- **`dataforseo-pp-cli serp google-ads-advertisers-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ads_advertisers/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-ads-advertisers-task-post`** - Google Ads Advertisers provides information on advertisers that run campaigns on Google Ads based on the Ads Transparency platform. ‌‌
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ads_advertisers/task_post/?bash'
- **`dataforseo-pp-cli serp google-ads-advertisers-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ads_advertisers/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-ads-search-locations`** - for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ads_search/locations/?bash'
- **`dataforseo-pp-cli serp google-ads-search-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ads_search/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-ads-search-task-post`** - Google Ads Search provides information on ads that are run by advertisers on Google Ads. Information is based on the Ads Transparency platform and adapted for the convenience of DataForSEO users. ‌‌
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ads_search/task_post/?bash'
- **`dataforseo-pp-cli serp google-ads-search-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ads_search/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-ai-mode-languages`** - You will receive the list of languages by calling this API.
 
As a response of the API server, you will receive JSON-encoded data containing a tasks array with the information specific to the set tasks.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ai_mode/languages/?bash'
- **`dataforseo-pp-cli serp google-ai-mode-live-advanced`** - Google AI Mode SERP API provides search results from the AI Mode feature of Google Search.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ai_mode/live/advanced/?bash'
- **`dataforseo-pp-cli serp google-ai-mode-live-html`** - Live SERP HTML provides a raw HTML page of 100 search engine results for the specified keyword, search engine, and location.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ai_mode/live/html/?bash'
- **`dataforseo-pp-cli serp google-ai-mode-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ai_mode/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-ai-mode-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ai_mode/task_get/html/?bash'
- **`dataforseo-pp-cli serp google-ai-mode-task-post`** - Google AI Mode SERP API provides search results from the AI Mode feature of Google Search.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ai_mode/task_post/?bash'
- **`dataforseo-pp-cli serp google-ai-mode-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ai_mode/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp google-ai-mode-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/ai_mode/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-autocomplete-live-advanced`** - Google Autocomplete is a feature within Google Search that improves the search experience by allowing users to complete searches they started to type. DataForSEO SERP API will provide you with all the suggestions Google Autocomplete offers for a particular keyword, the position of the cursor pointer, and the search client.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/autocomplete/live/advanced/?bash'
- **`dataforseo-pp-cli serp google-autocomplete-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/autocomplete/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-autocomplete-task-post`** - Google Autocomplete is a feature within Google Search that improves the search experience by allowing users to complete searches they started to type. DataForSEO SERP API will provide you with all the suggestions Google Autocomplete offers for a particular keyword, the position of the cursor pointer, and the search client.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/autocomplete/task_post/?bash'
- **`dataforseo-pp-cli serp google-autocomplete-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/autocomplete/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp google-autocomplete-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/autocomplete/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-dataset-info-live-advanced`** - Live Google Dataset Info provides real-time data on the dataset you specify in the request. You will get data from a page of the dataset displayed separately from the SERP. It contains information about dataset content, authors, licenses, and description on the SERP.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/dataset_info/live/advanced/?bash'
- **`dataforseo-pp-cli serp google-dataset-info-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/dataset_info/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-dataset-info-task-post`** - Google Dataset Info API provides detailed information about the dataset you specify in the POST request. You will get data from a page of the dataset displayed separately from the SERP. It contains information about dataset content, authors, licenses, and description on the SERP.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/dataset_info/task_post/?bash'
- **`dataforseo-pp-cli serp google-dataset-info-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/dataset_info/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp google-dataset-info-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/dataset_info/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-dataset-search-live-advanced`** - Live Google Dataset Search provides real-time data on the top 20 Google Dataset search engine results. These results are specific to the indicated keyword. You can specify other parameters optionally.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/dataset_search/live/advanced/?bash'
- **`dataforseo-pp-cli serp google-dataset-search-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/dataset_search/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-dataset-search-task-post`** - Google Dataset Search API provides top 20 Google Dataset search engine results. These results are specific to the indicated keyword. You can specify other parameters optionally.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/dataset_search/task_post/?bash'
- **`dataforseo-pp-cli serp google-dataset-search-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/dataset_search/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp google-dataset-search-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/dataset_search/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-events-live-advanced`** - Live Google Events SERP provides real-time data from Google Events Search for the specified keyword and location. Note that Google Events SERP API works for the English language only.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/events/live/advanced/?bash'
- **`dataforseo-pp-cli serp google-events-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/events/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-events-task-post`** - Google Events SERP provides data from Google Events Search for the specified keyword and location (see the List of Locations). Note that Google Events SERP API works for the English language only.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/events/task_post/?bash'
- **`dataforseo-pp-cli serp google-events-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/events/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp google-events-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/events/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-finance-explore-live-advanced`** - Live Google Finance Explore provides real-time data from the ‘Explore’ tab of Google Finance. These results are specific to the parameters you specify in the request: location and language.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_explore/live/advanced/?bash'
- **`dataforseo-pp-cli serp google-finance-explore-live-html`** - Live SERP HTML provides raw HTML page from the ‘Explore’ tab of Google Finance. These results are specific to the parameters you specify in the request: location and language.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_explore/live/html/?bash'
- **`dataforseo-pp-cli serp google-finance-explore-task-get-advanced`** - Live Google Finance Explore provides real-time data from the ‘Explore’ tab of Google Finance. These results are specific to the parameters you specify in the request: ticker in the keyword field, location and language.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_explore/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-finance-explore-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_explore/task_get/html/?bash'
- **`dataforseo-pp-cli serp google-finance-explore-task-post`** - Google Finance Explore API provides real-time data from the ‘Explore’ tab of Google Finance. These results are specific to the parameters you specify in the request: location and language.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_explore/task_post/?bash'
- **`dataforseo-pp-cli serp google-finance-explore-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_explore/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-finance-markets-live-advanced`** - Live Google Finance Markets provides real-time data from the ‘Markets’ tab of Google Finance. These results are specific to the parameters you specify in the request: location, language, and market_type.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_markets/live/advanced/?bash'
- **`dataforseo-pp-cli serp google-finance-markets-live-html`** - Live SERP HTML provides raw HTML from the ‘Markets’ tab of Google Finance. These results are specific to the parameters you specify in the request: location and language.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_markets/live/html/?bash'
- **`dataforseo-pp-cli serp google-finance-markets-task-get-advanced`** - Google Finance Markets API provides real-time data from the ‘Markets’ tab of Google Finance. These results are specific to the parameters you specify in the request: ticker in the keyword field, location and language.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_markets/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-finance-markets-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_markets/task_get/html/?bash'
- **`dataforseo-pp-cli serp google-finance-markets-task-post`** - Google Finance Markets API provides real-time data from the ‘Markets’ tab of Google Finance. These results are specific to the parameters you specify in the request:  location, language, and market_type.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_markets/task_post/?bash'
- **`dataforseo-pp-cli serp google-finance-markets-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_markets/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-finance-quote-live-advanced`** - Live Google Finance Quote provides real-time data from the ‘Quote’ tab of Google Finance. These results are specific to the parameters you specify in the request: ticker in the keyword field, location and language.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_quote/live/advanced/?bash'
- **`dataforseo-pp-cli serp google-finance-quote-live-html`** - Live SERP HTML provides raw HTML from the ‘Quote’ tab of Google Finance. These results are specific to the parameters you specify in the request: ticker in the keyword field, location and language.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_quote/live/html/?bash'
- **`dataforseo-pp-cli serp google-finance-quote-task-get-advanced`** - Live Google Finance Quote provides real-time data from the ‘Quote’ tab of Google Finance. These results are specific to the parameters you specify in the request: ticker in the keyword field, location and language.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_quote/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-finance-quote-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_quote/task_get/html/?bash'
- **`dataforseo-pp-cli serp google-finance-quote-task-post`** - Google Finance Quote provides real-time data from the ‘Quote’ tab of Google Finance. These results are specific to the parameters you specify in the request: ticker in the keyword field, location and language.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_quote/task_post/?bash'
- **`dataforseo-pp-cli serp google-finance-quote-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_quote/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-finance-ticker-search-live-advanced`** - Live Google Finance Ticker Search allows you to search for financial instruments available on Google Finance along with additional information. The result is specific to the parameters you specify in the request: keyword (name of a company or financial instrument) in the keyword field, location and language.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_ticker_search/live/advanced/?bash'
- **`dataforseo-pp-cli serp google-finance-ticker-search-task-get-advanced`** - Google Finance Ticker Search allows you to search for financial instruments available on Google Finance along with additional information. The result is specific to the parameters you specify in the request: keyword (name of a company or financial instrument) in the keyword field, location and language.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_ticker_search/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-finance-ticker-search-task-post`** - Google Finance Ticker Search allows you to search for financial instruments available on Google Finance along with additional information. The result is specific to the parameters you specify in the request: keyword (name of a company or financial instrument) in the keyword field, location and language.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_ticker_search/task_post/?bash'
- **`dataforseo-pp-cli serp google-finance-ticker-search-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/finance_ticker_search/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-images-live-advanced`** - Live Google Images SERP provides real-time data on top 100 images results for the specified keyword, search engine, and location.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/images/live/advanced/?bash'
- **`dataforseo-pp-cli serp google-images-live-html`** - Live SERP HTML provides a raw HTML page of 100 search engine results for the specified keyword, search engine, and location.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/images/live/html/?bash'
- **`dataforseo-pp-cli serp google-images-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/images/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-images-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/images/task_get/html/?bash'
- **`dataforseo-pp-cli serp google-images-task-post`** - SERP API provides top 100 search engine results. These results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/images/task_post/?bash'
- **`dataforseo-pp-cli serp google-images-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/images/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp google-images-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/images/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-jobs-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/jobs/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-jobs-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/jobs/task_get/html/?bash'
- **`dataforseo-pp-cli serp google-jobs-task-post`** - This endpoint will provide you with SERP data from the Google Jobs search engine. The returned results are specific to the keyword as well as the language and location parameters of the POST request.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/jobs/task_post/?bash'
- **`dataforseo-pp-cli serp google-jobs-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/jobs/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp google-jobs-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/jobs/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-languages`** - You will receive the list of languages by calling this API.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/languages/?bash'
- **`dataforseo-pp-cli serp google-local-finder-live-advanced`** - Live Google Local_finder SERP provides real-time search engine results for the specified keyword and location. By default, you can get up to 20 results for desktop and up to 10 results for mobile.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/local_finder/live/advanced/?bash'
- **`dataforseo-pp-cli serp google-local-finder-live-html`** - Live Google Local Finder SERP HTML provides a raw HTML page of the search engine results for the specified keyword, search engine, and location.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/local_finder/live/html/?bash'
- **`dataforseo-pp-cli serp google-local-finder-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/local_finder/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-local-finder-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/local_finder/task_get/html/?bash'
- **`dataforseo-pp-cli serp google-local-finder-task-post`** - Google Local Finder SERP API provides top search engine results specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/local_finder/task_post/?bash'
- **`dataforseo-pp-cli serp google-local-finder-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/local_finder/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp google-local-finder-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/local_finder/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-locations`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/locations/?bash'
- **`dataforseo-pp-cli serp google-locations-country`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/locations/?bash'
- **`dataforseo-pp-cli serp google-maps-live-advanced`** - Live Google Maps SERP provides real-time data on top 100 search engine results for the specified keyword, search engine, and location.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/maps/live/advanced/?bash'
- **`dataforseo-pp-cli serp google-maps-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/maps/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-maps-task-post`** - SERP API provides top 100 search engine results. These results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/maps/task_post/?bash'
- **`dataforseo-pp-cli serp google-maps-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/maps/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp google-maps-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/maps/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-news-live-advanced`** - Live Google News SERP provides real-time data on top search engine results for the specified keyword, search engine, and location.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/news/live/advanced/?bash'
- **`dataforseo-pp-cli serp google-news-live-html`** - Live SERP HTML provides a raw HTML page of 10 search engine results for the specified keyword, search engine, and location.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/news/live/html/?bash'
- **`dataforseo-pp-cli serp google-news-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/news/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-news-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/news/task_get/html/?bash'
- **`dataforseo-pp-cli serp google-news-task-post`** - SERP API provides top search engine results. These results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/news/task_post/?bash'
- **`dataforseo-pp-cli serp google-news-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/news/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp google-news-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/news/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-organic-live-advanced`** - Live SERP provides real-time data on top search engine results for the specified keyword, search engine, and location. This endpoint will supply a complete overview of featured snippets and other extra elements of SERPs.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/organic/live/advanced/?bash'
- **`dataforseo-pp-cli serp google-organic-live-html`** - Live SERP HTML provides a raw HTML page of search engine results for the specified keyword, search engine, and location.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/organic/live/html/?bash'
- **`dataforseo-pp-cli serp google-organic-live-regular`** - Live SERP provides real-time data on search engine results for the specified keyword, search engine, and location.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/organic/live/regular/?bash'
- **`dataforseo-pp-cli serp google-organic-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/organic/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-organic-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/organic/task_get/html/?bash'
- **`dataforseo-pp-cli serp google-organic-task-get-regular`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/organic/task_get/regular/?bash'
- **`dataforseo-pp-cli serp google-organic-task-post`** - SERP API provides top 10 search engine results by default. These results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/organic/task_post/?bash'
- **`dataforseo-pp-cli serp google-organic-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/organic/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp google-organic-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/organic/tasks_ready/?bash'
- **`dataforseo-pp-cli serp google-search-by-image-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/search_by_image/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp google-search-by-image-task-post`** - Google Search By Image SERP API provides up to top 100 search engine results based on the image you specified. These results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/search_by_image/task_post/?bash'
- **`dataforseo-pp-cli serp google-search-by-image-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/search_by_image/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp google-search-by-image-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/search_by_image/tasks_ready/?bash'
- **`dataforseo-pp-cli serp id-list`** - This endpoint is designed to provide you with a list of IDs and metadata for all SERP tasks created within the specified time period, including both successful and uncompleted tasks.
for more info please visit 'https://docs.dataforseo.com/v3/serp/id_list/?bash'
- **`dataforseo-pp-cli serp naver-organic-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/naver/organic/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp naver-organic-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/naver/organic/task_get/html/?bash'
- **`dataforseo-pp-cli serp naver-organic-task-get-regular`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/naver/organic/task_get/regular/?bash'
- **`dataforseo-pp-cli serp naver-organic-task-post`** - Naver SERP API provides top 15 search engine results. Naver search results do not vary by location and language, and the search parameters for this search engine do not contain language and location variables. However, you can specify a keyword in any language, and the search engine results may vary depending on the language you used for specifying the search query.
for more info please visit 'https://docs.dataforseo.com/v3/serp/naver/organic/task_post/?bash'
- **`dataforseo-pp-cli serp naver-organic-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/naver/organic/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp naver-organic-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/naver/organic/tasks_ready/?bash'
- **`dataforseo-pp-cli serp screenshot`** - Using the Live Page Screenshot endpoint, you can capture a screenshot of any SERP page.
for more info please visit 'https://docs.dataforseo.com/v3/serp/screenshot/?bash'
- **`dataforseo-pp-cli serp seznam-languages`** - You will receive the list of languages by calling this API.
for more info please visit 'https://docs.dataforseo.com/v3/serp/seznam/languages/?bash'
- **`dataforseo-pp-cli serp seznam-locations`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/serp/seznam/locations/?bash'
- **`dataforseo-pp-cli serp seznam-locations-country`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/serp/seznam/locations/?bash'
- **`dataforseo-pp-cli serp seznam-organic-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/seznam/organic/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp seznam-organic-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/seznam/organic/task_get/html/?bash'
- **`dataforseo-pp-cli serp seznam-organic-task-get-regular`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/seznam/organic/task_get/regular/?bash'
- **`dataforseo-pp-cli serp seznam-organic-task-post`** - Seznam SERP API provides top 10 search engine results from one of the most popular search engines in the Czech Republic. Seznam is focused on the local search market, and thus supports the Czech language only.
for more info please visit 'https://docs.dataforseo.com/v3/serp/seznam/organic/task_post/?bash'
- **`dataforseo-pp-cli serp seznam-organic-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/seznam/organic/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp seznam-organic-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/seznam/organic/tasks_ready/?bash'
- **`dataforseo-pp-cli serp tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/google/organic/tasks_ready/?bash'
- **`dataforseo-pp-cli serp yahoo-languages`** - You will receive the list of languages by calling this API.
for more info please visit 'https://docs.dataforseo.com/v3/serp/yahoo/languages/?bash'
- **`dataforseo-pp-cli serp yahoo-locations`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/serp/yahoo/locations/?bash'
- **`dataforseo-pp-cli serp yahoo-locations-country`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/serp/yahoo/locations/?bash'
- **`dataforseo-pp-cli serp yahoo-organic-live-advanced`** - Live SERP provides real-time data on top search engine results. These results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/serp/yahoo/organic/live/advanced/?bash'
- **`dataforseo-pp-cli serp yahoo-organic-live-html`** - Live SERP HTML provides a raw HTML page of search engine results for the specified keyword, search engine, and location.
for more info please visit 'https://docs.dataforseo.com/v3/serp/yahoo/organic/live/html/?bash'
- **`dataforseo-pp-cli serp yahoo-organic-live-regular`** - Live Yahoo SERP provides real-time data on search engine results for the specified keyword, search engine, and location.
for more info please visit 'https://docs.dataforseo.com/v3/serp/yahoo/organic/live/regular/?bash'
- **`dataforseo-pp-cli serp yahoo-organic-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/yahoo/organic/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp yahoo-organic-task-get-html`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/yahoo/organic/task_get/html/?bash'
- **`dataforseo-pp-cli serp yahoo-organic-task-get-regular`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/yahoo/organic/task_get/regular/?bash'
- **`dataforseo-pp-cli serp yahoo-organic-task-post`** - SERP API provides top search engine results. These results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/serp/yahoo/organic/task_post/?bash'
- **`dataforseo-pp-cli serp yahoo-organic-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/yahoo/organic/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp yahoo-organic-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/yahoo/organic/tasks_ready/?bash'
- **`dataforseo-pp-cli serp youtube-languages`** - You will receive the list of languages by calling this API.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/languages/?bash'
- **`dataforseo-pp-cli serp youtube-locations`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/locations/?bash'
- **`dataforseo-pp-cli serp youtube-locations-country`** - You will receive the list of locations by this API call. You can filter the list of locations by country when setting a task.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/locations/?bash'
- **`dataforseo-pp-cli serp youtube-organic-live-advanced`** - Live SERP provides real-time data on the top 20 blocks of YouTube search engine results. These results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/organic/live/advanced/'
- **`dataforseo-pp-cli serp youtube-organic-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/organic/task_get/advanced/'
- **`dataforseo-pp-cli serp youtube-organic-task-post`** - YouTube Organic API provides the top 20 blocks of search engine results. These results are specific to the selected location (see the List of Locations) and language (see the List of Languages) settings.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/organic/task_post/'
- **`dataforseo-pp-cli serp youtube-organic-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/organic/tasks_fixed/'
- **`dataforseo-pp-cli serp youtube-organic-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/organic/tasks_ready/'
- **`dataforseo-pp-cli serp youtube-video-comments-live-advanced`** - Live YouTube Comments provides real-time data on comments on the video you specify in the request. You will get the top 20 comments on the video as well as information about the author, and key comment metrics.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/video_comments/live/advanced/?bash'
- **`dataforseo-pp-cli serp youtube-video-comments-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/video_comments/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp youtube-video-comments-task-post`** - YouTube Comments API provides data on comments on the video you specify in the request. You will get the top 20 comments on the video as well as information about the author, and key comment metrics.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/video_comments/task_post/?bash'
- **`dataforseo-pp-cli serp youtube-video-comments-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/video_comments/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp youtube-video-comments-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/video_comments/tasks_ready/?bash'
- **`dataforseo-pp-cli serp youtube-video-info-live-advanced`** - Live YouTube Video Info provides real-time data on the video you specify in the request. You will get data from the watching page containing key video and content metrics as well as the channel where the video is published.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/video_info/live/advanced/?bash'
- **`dataforseo-pp-cli serp youtube-video-info-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/video_info/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp youtube-video-info-task-post`** - YouTube Video Info API provides detailed information about the video you specify in the POST request. You will get data from the watching page containing key video and content metrics as well as the channel where the video is published.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/video_info/task_post/?bash'
- **`dataforseo-pp-cli serp youtube-video-info-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/video_info/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp youtube-video-info-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/video_info/tasks_ready/?bash'
- **`dataforseo-pp-cli serp youtube-video-subtitles-live-advanced`** - Live YouTube Subtitles provides real-time data on subtitles in the video you specify in the request. You will get data from the watching page containing subtitled text, its language, and duration in the video.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/video_subtitles/live/advanced/?bash'
- **`dataforseo-pp-cli serp youtube-video-subtitles-task-get-advanced`** - Description of the fields for sending a request:
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/video_subtitles/task_get/advanced/?bash'
- **`dataforseo-pp-cli serp youtube-video-subtitles-task-post`** - YouTube Subtitles API provides data on all subtitles in the video you specify in the POST request. You will get data from the watching page containing subtitled text, its language, and duration in the video.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/video_subtitles/task_post/?bash'
- **`dataforseo-pp-cli serp youtube-video-subtitles-tasks-fixed`** - The ‘Tasks Fixed’ endpoint is designed to provide you with the list of re-parsed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed re-parsed tasks using this endpoint. Then, you can re-collect the fixed results using the ‘Task GET’ endpoint.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/video_subtitles/tasks_fixed/?bash'
- **`dataforseo-pp-cli serp youtube-video-subtitles-tasks-ready`** - The ‘Tasks Ready’ endpoint is designed to provide you with the list of completed tasks, which haven’t been collected yet. If you use the Standard method without specifying the postback_url, you can receive the list of id for all completed tasks using this endpoint. Then, you can collect the results using the ‘Task GET’ endpoint.
Learn more about task completion and obtaining a list of completed tasks in this help center article.
for more info please visit 'https://docs.dataforseo.com/v3/serp/youtube/video_subtitles/tasks_ready/?bash'


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
dataforseo-pp-cli ai-optimization ai-keyword-data-available-filters

# JSON for scripting and agents
dataforseo-pp-cli ai-optimization ai-keyword-data-available-filters --json

# Filter to specific fields
dataforseo-pp-cli ai-optimization ai-keyword-data-available-filters --json --select id,name,status

# Dry run — show the request without sending
dataforseo-pp-cli ai-optimization ai-keyword-data-available-filters --dry-run

# Agent mode — JSON + compact + no prompts in one flag
dataforseo-pp-cli ai-optimization ai-keyword-data-available-filters --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-dataforseo -g
```

Then invoke `/pp-dataforseo <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add dataforseo dataforseo-pp-mcp -e DATAFORSEO_LOGIN=<your-login> -e DATAFORSEO_PASSWORD=<your-password>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dataforseo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `DATAFORSEO_LOGIN` and `DATAFORSEO_PASSWORD` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "dataforseo": {
      "command": "dataforseo-pp-mcp",
      "env": {
        "DATAFORSEO_LOGIN": "<your-login>",
        "DATAFORSEO_PASSWORD": "<your-password>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
dataforseo-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/dataforseo-documentation-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `DATAFORSEO_LOGIN` | per_call | Yes | Account login from app.dataforseo.com/api-access. |
| `DATAFORSEO_PASSWORD` | per_call | Yes | Account password from app.dataforseo.com/api-access. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `dataforseo-pp-cli doctor` to check credentials
- Verify the environment variables are set: `echo $DATAFORSEO_LOGIN $DATAFORSEO_PASSWORD`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Top-level status_code 20000 but every keyword volume is null** — Task-level 40501 — one keyword failed validation. Re-run with `keywords clean <file>` first to strip the offending input; that pre-flight is what catches the silent 40501 trap.
- **Live Google Ads call returns HTTP 429** — Hit the 12/min Live Google Ads limit. Use `keywords volume --mode auto` to route to Standard for batches >5, or `--rate-limit 12rpm`.
- **Standard-mode task stuck "in_progress" forever** — `tasks_ready` has a refresh delay >1000 tasks/min. Use `dataforseo-pp-cli task ls` to inspect the local ledger and `task bundle` to resume.
- **$50 deposit burning faster than expected** — Run `cost estimate <subcommand> <args>` before any batch call. Add `--confirm-over 1.00` to gate any command above $1.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**claude-seo**](https://github.com/AgriciDaniel/claude-seo) — Markdown (6400 stars)
- [**dataforseo-mcp-server-typescript**](https://github.com/dataforseo/mcp-server-typescript) — TypeScript (196 stars)
- [**dataforseo-claude**](https://github.com/zubair-trabzada/dataforseo-claude) — Markdown (60 stars)
- [**PythonClient**](https://github.com/dataforseo/PythonClient) — Python (44 stars)
- [**TypeScriptClient**](https://github.com/dataforseo/TypeScriptClient) — TypeScript (34 stars)
- [**dataforseo-skill-claude**](https://github.com/nikhilbhansali/dataforseo-skill-claude) — Python (29 stars)
- [**n8n-nodes-dataforseo**](https://github.com/dataforseo/n8n-nodes-dataforseo) — TypeScript (16 stars)
- [**dataforseo-ai-mcp-server**](https://github.com/aimonk2025/dataforseo-ai-mcp-server) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
