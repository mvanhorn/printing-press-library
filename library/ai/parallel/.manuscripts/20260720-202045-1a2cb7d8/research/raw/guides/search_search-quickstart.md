> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Search API Quickstart

> Execute natural language web searches and retrieve LLM-optimized excerpts with the Parallel Search API

<div className="sr-only" aria-hidden="false">
  For AI agents: a documentation index is available at [https://docs.parallel.ai/llms.txt](https://docs.parallel.ai/llms.txt). The full text of all docs is at [https://docs.parallel.ai/llms-full.txt](https://docs.parallel.ai/llms-full.txt). You may also fetch any page as Markdown by appending `.md` to its URL or sending `Accept: text/markdown`.
</div>

The **Search API** takes a natural language objective and returns relevant excerpts
optimized for LLMs, replacing multiple keyword searches with a single call for broad or
complex queries.

<Tip>
  **Available via MCP**: Search is available as a tool as part of the Parallel Search MCP. Our MCP is optimized for best practices on Search and Extract usage. [Start here](/integrations/mcp/search-mcp) with MCP for your use case. If you're interested in direct use of the API, follow the steps below.
</Tip>

<Note>
  **Looking for people or companies?** [Entity Search](/findall-api/entity-search) is a fast, synchronous people-and-company search — the real-time counterpart to FindAll — returning a set of matching results in seconds. It is distinct from the Search API, which retrieves pages and excerpts across the whole web.
</Note>

## 1. Set Up Prerequisites

Generate your API key on [Platform](https://platform.parallel.ai). Then, set up with the TypeScript SDK, Python SDK or with cURL:

<CodeGroup>
  ```bash cURL theme={"system"}
  echo "Install curl and jq via brew, apt, or your favorite package manager"
  export PARALLEL_API_KEY="PARALLEL_API_KEY"
  ```

  ```bash Python theme={"system"}
  pip install parallel-web
  export PARALLEL_API_KEY="PARALLEL_API_KEY"
  ```

  ```bash TypeScript theme={"system"}
  npm install parallel-web
  export PARALLEL_API_KEY="PARALLEL_API_KEY"
  ```
</CodeGroup>

## 2. Execute Your First Search

### Sample Request

<CodeGroup>
  ```bash cURL theme={"system"}
  curl https://api.parallel.ai/v1/search \
    -H "Content-Type: application/json" \
    -H "x-api-key: $PARALLEL_API_KEY" \
    -d '{
      "objective": "Find latest information about Parallel Web Systems. Focus on new product releases, benchmarks, or company announcements.",
      "search_queries": [
        "Parallel Web Systems products",
        "Parallel Web Systems announcements"
      ]
    }'
  ```

  ```python Python theme={"system"}
  import os
  from parallel import Parallel

  client = Parallel(api_key=os.environ["PARALLEL_API_KEY"])

  search = client.search(
      objective="Find latest information about Parallel Web Systems. Focus on new product releases, benchmarks, or company announcements.",
      search_queries=[
          "Parallel Web Systems products",
          "Parallel Web Systems announcements"
      ],
  )

  for result in search.results:
      print(f"{result.title}: {result.url}")
      for excerpt in result.excerpts:
          print(excerpt[:200])

  ```

  ```typescript TypeScript theme={"system"}
  import Parallel from "parallel-web";

  const client = new Parallel({ apiKey: process.env.PARALLEL_API_KEY });

  async function main() {
      const search = await client.search({
          objective: "Find latest information about Parallel Web Systems. Focus on new product releases, benchmarks, or company announcements.",
          search_queries: [
              "Parallel Web Systems products",
              "Parallel Web Systems announcements"
          ],
      });

      for (const result of search.results) {
          console.log(`${result.title}: ${result.url}`);
          for (const excerpt of result.excerpts) {
              console.log(excerpt.slice(0, 200));
          }
      }
  }

  main().catch(console.error);
  ```
</CodeGroup>

<Tip>
  This request uses the default `advanced` mode (\~3s, highest quality). For latency-sensitive use cases, add `"mode": "turbo"` as a top-level request field for a p50 of 200ms at a lower cost. The rest of the request stays the same. See [Search Modes](/search/modes) for a runnable example.
</Tip>

### Sample Response

The API returns a JSON response with the following structure. The SDK examples above iterate over `results` to print each result's title, URL, and excerpts. If any non-fatal input adjustments or validation warnings have occurred, they will be in the warnings field.

```json [expandable] theme={"system"}
{
  "search_id": "search_8a911eb27c7a4afaa20d0d9dc98d07c0",
  "results": [
    {
      "url": "https://www.linkedin.com/posts/analytics-india-magazine_former-twitter-ceo-parag-agrawals-new-startup-activity-7394607179896426497-xdcJ",
      "title": "Parallel Web Systems raises $100M for AI web access - LinkedIn",
      "publish_date": null,
      "excerpts": [
        "Parallel Web Systems raises $100M for AI web access - LinkedIn\nHis new startup, Parallel Web Systems, has just secured a massive $100 million Series A round at a striking $740 million post-money valuation."
      ]
    },
    {
      "url": "https://www.reuters.com/business/ex-twitter-ceo-agrawals-ai-search-startup-parallel-raises-100-million-2025-11-12",
      "title": "Ex-Twitter CEO Agrawal's AI search startup Parallel raises ... - Reuters",
      "publish_date": null,
      "excerpts": [
        "Section Title: Information you can trust > Follow Us > LSEG Products\nContent:\nWorkspace, opens new tab — Access unmatched financial data, news and content in a highly-customised workflow experience on desktop, web and mobile.\n\n... (content truncated)"
      ]
    },
    {
      "url": "https://www.linkedin.com/posts/tbpn_parag-agrawal-and-the-team-at-parallel-web-activity-7395150680069627905-oMIZ",
      "title": "Parallel Web Systems raises $100M for AI data search. - LinkedIn",
      "publish_date": null,
      "excerpts": [
        "Nov 14, 2025 · Today we're excited to announce new funding for LangChain (at a $1.25B valuation) to allow us to build the platform for agent engineering."
      ]
    },
    {
      "url": "https://parallel.ai/products/search",
      "title": "The best web search for your AI | Parallel | Parallel Web Systems | Web Search & Research APIs Built for AI Agents",
      "publish_date": null,
      "excerpts": [
        "Section Title: The best web search for your AI > We optimize every web token in the context window\nContent:\nThis means agent responses are more accurate and lower cost.\n\nBrowseComp benchmark proving Parallel's enterprise deep research API delivers 48% accuracy vs GPT-4's 1% browsing capability.\n\n... (content truncated)"
      ]
    },
    {
      "url": "https://startups.gallery/companies/parallel",
      "title": "Parallel | startups.gallery",
      "publish_date": null,
      "excerpts": [
        "Section Title: Parallel\nContent:\nAt Parallel Web Systems, we are bringing a new web to life: it's built with, by, and for AIs. Our work spans innovations across crawling, indexing, ranking, retrieval, and reasoning systems. Our first product is a set of APIs for AIs to do more with web data. Founded by former CEO/CTO of Twitter, Parag Agrawal.\n\nBacked by First Round Capital, Index Ventures, Khosla Ventures."
      ]
    },
    {
      "url": "https://www.instagram.com/popular/find-latest-information-about-parallel-web-systems-focus-on-new-product-releases-benchmarks-or-company-announcements",
      "title": "Find Latest Information About Parallel Web Systems Focus On New ...",
      "publish_date": null,
      "excerpts": [
        "Find Latest Information About Parallel Web Systems Focus On New ...\nWatch short videos about find latest information about parallel web systems focus on new product releases benchmarks or company announcements from people"
      ]
    },
    {
      "url": "https://parallel.ai/",
      "title": "Parallel Web Systems | Infrastructure for intelligence on the web",
      "publish_date": null,
      "excerpts": [
        "Section Title: Highest accuracy at every price point\nContent:\nState of the art across the most challenging benchmarks.\n\nSection Title: Towards a programmatic web for AIs\nContent:\nParallel is building new interfaces, infrastructure, and business models for AIs to work with the web.\n\n... (content truncated)"
      ]
    },
    {
      "url": "https://www.linkedin.com/company/parallel-web",
      "title": "Parallel Web Systems",
      "publish_date": null,
      "excerpts": [
        "Section Title: Parallel Web Systems > About us\nContent:\nAt Parallel Web Systems, we are bringing a new web to life: it's built with, by, and for AIs. Our work spans innovations across crawling, indexing, ranking, retrieval, and reasoning systems. Our first product is a set of APIs for AIs to do more with web data.\n\nHeadquarters: Palo Alto, California. Company size: 11-50 employees.\n\n... (content truncated)"
      ]
    },
    {
      "url": "https://www.prnewswire.com/news-releases/genpact-and-parallel-web-systems-partner-to-drive-tangible-efficiency-from-ai-systems-302736563.html",
      "title": "Genpact and Parallel Web Systems Partner to Drive Tangible ...",
      "publish_date": null,
      "excerpts": [
        "NEW YORK and PALO ALTO, Calif., April 8, 2026 /PRNewswire/ -- Genpact and Parallel Web Systems today announced a partnership leveraging Parallel's AI-native web agents and web search tools to transform how Genpact addresses enterprise challenges in information search and retrieval across business operations.\n\n... (content truncated)"
      ]
    },
    {
      "url": "https://aimagazine.com/magazines/parag-agrawals-parallel-web-systems-raises-100m-for-ai",
      "title": "How Parag Agrawal's Parallel Web Systems Raised $100m for AI | AI Magazine",
      "publish_date": "2025-11-19",
      "excerpts": [
        "Parallel Web Systems, a startup from former Twitter CEO Parag Agrawal, has secured US$100m in a Series A funding round. The investment round was co-led by Kleiner Perkins and Index Ventures with participation from other backers, including Khosla Ventures.\n\nFounded in 2023 and officially launched in August 2025, Parallel Web Systems develops APIs that enable AI systems to search the live web for current information needed to perform tasks.\n\n... (content truncated)"
      ]
    }
  ],
  "warnings": null,
  "usage": [
    {
      "name": "sku_search",
      "count": 1
    }
  ],
  "session_id": "session_REDACTED"
}
```

## Next Steps

* **[Best Practices](/search/best-practices)** — learn how to craft effective objectives and search queries
* **[Search Modes](/search/modes)** — explore mode presets for different use cases
* **[API Reference](/api-reference/search/search)** — full parameter specifications, constraints, and response schema
* **[Rate Limits](/getting-started/rate-limits)** — default quotas per product
