> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Search

> Searches the web.

The legacy Search API reference (`/v1beta/search` endpoint) is available
[here](https://docs.parallel.ai/api-reference/legacy/search-beta/search), and
migration guide is [here](https://docs.parallel.ai/search/search-migration-guide).



## OpenAPI

````yaml /public-openapi.json post /v1/search
openapi: 3.1.0
info:
  title: Parallel API
  description: Parallel API
  contact:
    name: Parallel Support
    url: https://parallel.ai
    email: support@example.com
  version: 0.1.2
servers:
  - url: https://api.parallel.ai
    description: Parallel API
security:
  - ApiKeyAuth: []
tags:
  - name: Search
    description: >-
      Search returns ranked URLs with extended excerpts suitable for LLM
      consumption. Inputs are a natural-language objective and optional keyword
      queries. Source policies allow including or excluding specific domains and
      have configurable output sizes. The returned extended snippets contain
      dense, relevant information from relevant pages.

      - Result: ranked list with URL, title, and long text excerpts
  - name: Extract
    description: >-
      Extract returns excerpts or full content from one or more URLs. Inputs are
      a list of URLs and an optional search objective and keyword queries. The
      returned excerpts or full content is formatted as markdown and suitable
      for LLM consumption.

      - Result: excerpts or full content from the URL formatted as markdown
  - name: Tasks
    description: >-
      The Task API executes web research and extraction tasks. Clients submit a
      natural-language objective with an optional input schema; the service
      plans retrieval, fetches relevant URLs, and returns outputs that conform
      to a provided or inferred JSON schema. Supports deep research style
      queries and can return rich structured JSON outputs. Processors trade-off
      between cost, latency, and quality. Each processor supports calibrated
      confidences.

      - Output metadata: citations, excerpts, reasoning, and confidence per
      field


      Task Groups enable batch execution of many independent Task runs with
      group-level monitoring and failure handling.

      - Submit hundreds or thousands of Tasks as a single group

      - Observe group progress and receive results as they complete

      - Real-time updates via Server-Sent Events (SSE)

      - Add tasks to an existing group while it is running

      - Group-level retry and error aggregation
  - name: FindAll
    description: >-
      The FindAll API discovers and evaluates entities that match complex
      criteria from natural language objectives. Submit a high-level goal and
      the service automatically generates structured match conditions, discovers
      relevant candidates, and evaluates each against the criteria. Returns
      comprehensive results with detailed reasoning, citations, and confidence
      scores for each match decision. Streaming events and webhooks are
      supported.
  - name: Monitor
    description: >-
      The Monitor API watches the web for material changes on a fixed frequency.
      Each monitor runs once on creation and then on its configured schedule,
      emitting events when meaningful changes are detected.

      - `event_stream` monitors track a search query and emit an event for each
      new material change.

      - `snapshot` monitors track a specific task run's output and emit an event
      when the output changes.


      Results can be polled via the events endpoint or delivered via webhooks.
  - name: Chat API (Beta)
    description: >-
      The Chat API provides a programmatic chat-style text generation interface.
      It accepts a sequence of messages and returns model responses. Intended
      for assistant-like interactions and evaluation. Streaming responses are
      supported.
paths:
  /v1/search:
    post:
      tags:
        - Search
      summary: Search
      description: >-
        Searches the web.


        The legacy Search API reference (`/v1beta/search` endpoint) is available

        [here](https://docs.parallel.ai/api-reference/legacy/search-beta/search),
        and

        migration guide is
        [here](https://docs.parallel.ai/search/search-migration-guide).
      operationId: v1_search_v1_search_post
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/V1SearchRequest'
        required: true
      responses:
        '200':
          description: Successful Response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/V1SearchResponse'
              example:
                search_id: search_fcb2b4f3c75e418687bccaa1a8381331
                results:
                  - url: https://www.example.com
                    title: Sample webpage title
                    publish_date: '2024-01-15'
                    excerpts:
                      - Sample excerpt 1
                      - Sample excerpt 2
                session_id: session_REDACTED
        '422':
          description: Request validation error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
              example:
                type: error
                error:
                  ref_id: search_fcb2b4f3c75e418687bccaa1a8381331
                  message: Request validation error
      x-code-samples:
        - lang: Python
          source: |-
            from parallel import Parallel

            client = Parallel()

            search = client.search(
                objective="Find latest information about Parallel Web Systems. Focus on new product releases, benchmarks, or company announcements.",
                search_queries=["Parallel Web Systems products", "Parallel Web Systems announcements"],
            )
            print(search.results)
        - lang: TypeScript
          source: |-
            import Parallel from "parallel-web";

            const client = new Parallel();

            const search = await client.search({
                objective: "Find latest information about Parallel Web Systems. Focus on new product releases, benchmarks, or company announcements.",
                search_queries: ["Parallel Web Systems products", "Parallel Web Systems announcements"],
            });
            console.log(search.results);
        - lang: cURL
          source: |-
            curl --request POST \
                --url https://api.parallel.ai/v1/search \
                --header 'Content-Type: application/json' \
                --header 'x-api-key: <api-key>' \
                --data '{
                "objective": "Find latest information about Parallel Web Systems. Focus on new product releases, benchmarks, or company announcements.",
                "search_queries": ["Parallel Web Systems products", "Parallel Web Systems announcements"]
            }'
components:
  schemas:
    V1SearchRequest:
      properties:
        objective:
          anyOf:
            - type: string
            - type: 'null'
          title: Objective
          description: >-
            Natural-language description of the underlying question or goal
            driving the search. Used together with search_queries to focus
            results on the most relevant content. Should be self-contained with
            enough context to understand the intent of the search.
        search_queries:
          items:
            type: string
          type: array
          title: Search Queries
          description: >-
            Concise keyword search queries, 3-6 words each. At least one query
            is required, provide 2-3 for best results. Used together with
            objective to focus results on the most relevant content.
        mode:
          anyOf:
            - type: string
              enum:
                - turbo
                - basic
                - advanced
            - type: 'null'
          title: Mode
          description: >-
            Search mode preset: supported values are `turbo`, `basic`, and
            `advanced`. Turbo mode is optimized for the fastest responses. Basic
            mode offers low latency and works best with 2-3 high-quality
            search_queries. Advanced mode provides higher quality with more
            advanced retrieval and compression. Defaults to `advanced` when
            omitted.
        max_chars_total:
          anyOf:
            - type: integer
            - type: 'null'
          title: Max Chars Total
          description: Upper bound on total characters across excerpts from all results.
        session_id:
          anyOf:
            - type: string
              maxLength: 1000
            - type: 'null'
          title: Session Id
          description: >-
            Session identifier to track calls across separate search and extract
            calls, to be used as part of a larger task. Specifying it may give
            better contextual results for subsequent API calls.
        client_model:
          anyOf:
            - type: string
            - type: 'null'
          title: Client Model
          description: >-
            The model generating this request and consuming the results. Enables
            optimizations and tailors default settings for the model's
            capabilities.
          examples:
            - claude-opus-4-7
            - gpt-5.4
            - gemini-3.1-pro
        advanced_settings:
          anyOf:
            - $ref: '#/components/schemas/AdvancedSearchSettings'
            - type: 'null'
          description: >-
            Advanced configuration for source policy, fetch policy, and excerpt
            settings. May impact result quality and latency unless used
            carefully. When omitted, excerpts are enabled by default.
      additionalProperties: false
      type: object
      required:
        - search_queries
      title: V1SearchRequest
      description: Search request.
    V1SearchResponse:
      properties:
        search_id:
          type: string
          title: Search Id
          description: 'Search ID. Example: `search_cad0a6d2dec046bd95ae900527d880e7`'
        results:
          items:
            $ref: '#/components/schemas/V1WebSearchResult'
          type: array
          title: Results
          description: A list of search results, ordered by decreasing relevance.
        warnings:
          anyOf:
            - items:
                $ref: '#/components/schemas/Warning'
              type: array
            - type: 'null'
          title: Warnings
          description: Warnings for the search request, if any.
        usage:
          anyOf:
            - items:
                $ref: '#/components/schemas/UsageItem'
              type: array
            - type: 'null'
          title: Usage
          description: Usage metrics for the search request.
        session_id:
          type: string
          title: Session Id
          description: >-
            Session identifier, echoed back from the request if provided,
            otherwise generated by the server. Should be passed to future search
            and extract calls made by the agent as part of the same larger task.
          examples:
            - session_REDACTED
      type: object
      required:
        - search_id
        - results
        - session_id
      title: V1SearchResponse
      description: Search response.
    ErrorResponse:
      properties:
        type:
          type: string
          const: error
          title: Type
          description: Always 'error'.
        error:
          $ref: '#/components/schemas/Error'
          description: Error.
      type: object
      required:
        - type
        - error
      title: ErrorResponse
      description: Response object used for non-200 status codes.
    AdvancedSearchSettings:
      properties:
        source_policy:
          anyOf:
            - $ref: '#/components/schemas/SourcePolicy'
            - type: 'null'
          description: Domain and date filtering preferences for search results.
        fetch_policy:
          anyOf:
            - $ref: '#/components/schemas/FetchPolicy'
            - type: 'null'
          description: >-
            Fetch policy: determines when to return cached content from the
            index (faster) vs fetching live content (fresher). Default is to
            disable live fetch and return cached content from the index. Note:
            enabling live fetch significantly increases search latency because
            it requires fetching content from source websites.
        excerpt_settings:
          anyOf:
            - $ref: '#/components/schemas/V1ExcerptSettings'
            - type: 'null'
          description: >-
            Controls excerpt sizes. Provide excerpt settings for fine-grained
            control, or omit to use defaults.
        location:
          anyOf:
            - type: string
            - type: 'null'
          title: Location
          description: ISO 3166-1 alpha-2 country code for geo-targeted search results.
          examples:
            - us
            - gb
            - de
            - jp
        max_results:
          anyOf:
            - type: integer
            - type: 'null'
          title: Max Results
          description: >-
            Upper bound on the number of results to return. Defaults to 10 if
            not provided.
      additionalProperties: false
      type: object
      title: AdvancedSearchSettings
      description: >-
        Advanced search configuration.


        These settings may impact result quality and latency unless used
        carefully.

        See https://docs.parallel.ai/search/advanced-search-settings for more
        info.
    V1WebSearchResult:
      properties:
        url:
          type: string
          title: Url
          description: URL associated with the search result.
        title:
          anyOf:
            - type: string
            - type: 'null'
          title: Title
          description: Title of the webpage, if available.
        publish_date:
          anyOf:
            - type: string
            - type: 'null'
          title: Publish Date
          description: Publish date of the webpage in YYYY-MM-DD format, if available.
        excerpts:
          items:
            type: string
          type: array
          title: Excerpts
          description: Relevant excerpted content from the URL, formatted as markdown.
      type: object
      required:
        - url
        - excerpts
      title: V1WebSearchResult
      description: A single search result from the web search API.
    Warning:
      properties:
        type:
          type: string
          enum:
            - spec_validation_warning
            - input_validation_warning
            - warning
          title: Type
          description: >-
            Type of warning. Note that adding new warning types is considered a
            backward-compatible change.
          examples:
            - spec_validation_warning
            - input_validation_warning
        message:
          type: string
          title: Message
          description: Human-readable message.
        detail:
          anyOf:
            - additionalProperties: true
              type: object
            - type: 'null'
          title: Detail
          description: Optional detail supporting the warning.
      type: object
      required:
        - type
        - message
      title: Warning
      description: Human-readable message for a task.
    UsageItem:
      properties:
        name:
          type: string
          title: Name
          description: Name of the SKU.
          examples:
            - sku_search_additional_results
            - sku_extract_excerpts
        count:
          type: integer
          title: Count
          description: Count of the SKU.
          examples:
            - 1
      type: object
      required:
        - name
        - count
      title: UsageItem
      description: Usage item for a single operation.
    Error:
      properties:
        ref_id:
          type: string
          title: Reference ID
          description: Reference ID for the error.
        message:
          type: string
          title: Message
          description: Human-readable message.
        detail:
          anyOf:
            - additionalProperties: true
              type: object
            - type: 'null'
          title: Detail
          description: Optional detail supporting the error.
      type: object
      required:
        - ref_id
        - message
      title: Error
      description: An error message.
    SourcePolicy:
      properties:
        include_domains:
          items:
            type: string
          type: array
          title: Include Domains
          description: >-
            List of domains to restrict the results to. If specified, only
            sources from these domains will be included. Accepts plain domains
            (e.g., example.com, subdomain.example.gov) or bare domain extension
            starting with a period (e.g., .gov, .edu, .co.uk). The combined
            number of domains in include_domains and exclude_domains cannot
            exceed 200.
          examples:
            - - wikipedia.org
              - usa.gov
              - .edu
        exclude_domains:
          items:
            type: string
          type: array
          title: Exclude Domains
          description: >-
            List of domains to exclude from results. If specified, sources from
            these domains will be excluded. Accepts plain domains (e.g.,
            example.com, subdomain.example.gov) or bare domain extension
            starting with a period (e.g., .gov, .edu, .co.uk). The combined
            number of domains in include_domains and exclude_domains cannot
            exceed 200.
          examples:
            - - reddit.com
              - x.com
              - .ai
        after_date:
          anyOf:
            - type: string
              format: date
            - type: 'null'
          title: After Date
          description: >-
            Optional start date for filtering search results. Results will be
            limited to content published on or after this date. Provided as an
            RFC 3339 date string (YYYY-MM-DD).
          examples:
            - '2024-01-01'
      type: object
      title: SourcePolicy
      description: |-
        Source policy for web search results.

        This policy governs which sources are allowed/disallowed in results.
    FetchPolicy:
      properties:
        max_age_seconds:
          anyOf:
            - type: integer
            - type: 'null'
          title: Max Age Seconds
          description: >-
            Maximum age of cached content in seconds to trigger a live fetch.
            Minimum value 600 seconds (10 minutes).
          examples:
            - 86400
        timeout_seconds:
          anyOf:
            - type: number
            - type: 'null'
          title: Timeout Seconds
          description: >-
            Timeout in seconds for fetching live content if unavailable in
            cache.
          examples:
            - 60
        disable_cache_fallback:
          type: boolean
          title: Disable Cache Fallback
          description: >-
            If false, fallback to cached content older than max-age if live
            fetch fails or times out. If true, returns an error instead.
          default: false
      type: object
      title: FetchPolicy
      description: Policy for live fetching web results.
    V1ExcerptSettings:
      properties:
        max_chars_per_result:
          anyOf:
            - type: integer
            - type: 'null'
          title: Max Chars Per Result
          description: >-
            Optional upper bound on the total number of characters to include
            per url. Excerpts may contain fewer characters than this limit to
            maximize relevance and token efficiency.
      additionalProperties: false
      type: object
      title: V1ExcerptSettings
      description: Optional settings for returning relevant excerpts.
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: x-api-key

````