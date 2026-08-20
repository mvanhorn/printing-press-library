> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Extract

> Extracts relevant content from specific web URLs.

The legacy Extract API reference (`/v1beta/extract` endpoint) is available
[here](https://docs.parallel.ai/api-reference/legacy/extract-beta/extract), and
migration guide is [here](https://docs.parallel.ai/extract/extract-migration-guide).



## OpenAPI

````yaml /public-openapi.json post /v1/extract
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
  /v1/extract:
    post:
      tags:
        - Extract
      summary: Extract
      description: >-
        Extracts relevant content from specific web URLs.


        The legacy Extract API reference (`/v1beta/extract` endpoint) is
        available

        [here](https://docs.parallel.ai/api-reference/legacy/extract-beta/extract),
        and

        migration guide is
        [here](https://docs.parallel.ai/extract/extract-migration-guide).
      operationId: extract_v1_extract_post
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/V1ExtractRequest'
        required: true
      responses:
        '200':
          description: Successful Response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/V1ExtractResponse'
              example:
                extract_id: extract_8a911eb27c7a4afaa20d0d9dc98d07c0
                results:
                  - url: https://www.example.com
                    title: Example Title
                    excerpts:
                      - Excerpted text ...
                    full_content: Full content ...
                errors:
                  - url: https://www.example.com
                    error_type: fetch_error
                    http_status_code: 500
                    content: Error fetching content from https://www.example.com
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
                  ref_id: extract_8a911eb27c7a4afaa20d0d9dc98d07c0
                  message: Request validation error
      x-code-samples:
        - lang: Python
          source: |-
            from parallel import Parallel

            client = Parallel()

            extract = client.extract(
                urls=["https://www.example.com"],
                objective="Summarize the page",
            )
            print(extract.results)
        - lang: TypeScript
          source: |-
            import Parallel from "parallel-web";

            const client = new Parallel();

            const extract = await client.extract({
                urls: ["https://www.example.com"],
                objective: "Summarize the page",
            });
            console.log(extract.results);
        - lang: cURL
          source: |-
            curl --request POST \
                --url https://api.parallel.ai/v1/extract \
                --header 'Content-Type: application/json' \
                --header 'x-api-key: <api-key>' \
                --data '{
                "urls": ["https://www.example.com"],
                "objective": "Summarize the page"
            }'
components:
  schemas:
    V1ExtractRequest:
      properties:
        urls:
          items:
            type: string
          type: array
          title: Urls
          description: URLs to extract content from. Up to 20 URLs.
        objective:
          anyOf:
            - type: string
            - type: 'null'
          title: Objective
          description: >-
            As in SearchRequest, a natural-language description of the
            underlying question or goal driving the request. Used together with
            search_queries to focus excerpts on the most relevant content.
        search_queries:
          anyOf:
            - items:
                type: string
              type: array
            - type: 'null'
          title: Search Queries
          description: >-
            Optional keyword search queries, as in SearchRequest. Used together
            with objective to focus excerpts on the most relevant content.
        max_chars_total:
          anyOf:
            - type: integer
            - type: 'null'
          title: Max Chars Total
          description: >-
            Upper bound on total characters across excerpts from all extracted
            results.
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
            - $ref: '#/components/schemas/AdvancedExtractSettings'
            - type: 'null'
          description: >-
            Advanced configuration for fetch policy, excerpt settings, and full
            content settings. May impact result quality and latency unless used
            carefully. When omitted, excerpts are enabled and full content is
            disabled by default.
      additionalProperties: false
      type: object
      required:
        - urls
      title: V1ExtractRequest
      description: Extract request.
    V1ExtractResponse:
      properties:
        extract_id:
          type: string
          title: Extract Id
          description: Extract request ID, e.g. `extract_cad0a6d2dec046bd95ae900527d880e7`
        results:
          items:
            $ref: '#/components/schemas/V1ExtractResult'
          type: array
          title: Results
          description: Successful extract results.
        errors:
          items:
            $ref: '#/components/schemas/ExtractError'
          type: array
          title: Errors
          description: 'Extract errors: requested URLs not in the results.'
        warnings:
          anyOf:
            - items:
                $ref: '#/components/schemas/Warning'
              type: array
            - type: 'null'
          title: Warnings
          description: Warnings for the extract request, if any.
        usage:
          anyOf:
            - items:
                $ref: '#/components/schemas/UsageItem'
              type: array
            - type: 'null'
          title: Usage
          description: Usage metrics for the extract request.
        session_id:
          type: string
          title: Session Id
          description: >-
            Session identifier. Echoed back from the request if provided,
            otherwise generated by the server. Should be passed to future search
            and extract calls made by the agent as part of the same larger task.
          examples:
            - session_REDACTED
      type: object
      required:
        - extract_id
        - results
        - errors
        - session_id
      title: V1ExtractResponse
      description: Extract response.
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
    AdvancedExtractSettings:
      properties:
        fetch_policy:
          anyOf:
            - $ref: '#/components/schemas/FetchPolicy'
            - type: 'null'
          description: >-
            Fetch policy: determines when to return cached content from the
            index (faster) vs fetching live content (fresher). Default is to use
            a dynamic policy based on the search objective and url. Note:
            enabling live fetch significantly increases extract latency because
            it requires fetching content from source websites.
        excerpt_settings:
          anyOf:
            - $ref: '#/components/schemas/V1ExcerptSettings'
            - type: 'null'
          description: >-
            Controls excerpt sizes. Provide excerpt settings for fine-grained
            control, or omit to use defaults.
        full_content:
          anyOf:
            - $ref: '#/components/schemas/FullContentSettings'
            - type: boolean
          title: Full Content
          description: >-
            Controls full content extraction. Set to true to enable with
            defaults, false to disable, or provide FullContentSettings for
            fine-grained control.
          default: false
      additionalProperties: false
      type: object
      title: AdvancedExtractSettings
      description: >-
        Advanced extract configuration.


        These settings may impact result quality and latency unless used
        carefully.

        See https://docs.parallel.ai/search/advanced-extract-settings for more
        info.
    V1ExtractResult:
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
        full_content:
          anyOf:
            - type: string
            - type: 'null'
          title: Full Content
          description: Full content from the URL formatted as markdown, if requested.
      type: object
      required:
        - url
        - excerpts
      title: V1ExtractResult
      description: Extract result for a single URL.
    ExtractError:
      properties:
        url:
          type: string
          title: Url
        error_type:
          type: string
          title: Error Type
          description: Error type.
        http_status_code:
          anyOf:
            - type: integer
            - type: 'null'
          title: Http Status Code
          description: HTTP status code, if available.
        content:
          anyOf:
            - type: string
            - type: 'null'
          title: Content
          description: Content returned for http client or server errors, if any.
      type: object
      required:
        - url
        - error_type
        - http_status_code
        - content
      title: ExtractError
      description: Extract error details.
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
    FullContentSettings:
      properties:
        max_chars_per_result:
          anyOf:
            - type: integer
            - type: 'null'
          title: Max Chars Per Result
          description: >-
            Optional limit on the number of characters to include in the full
            content for each url. Full content always starts at the beginning of
            the page and is truncated at the limit if necessary.
      type: object
      title: FullContentSettings
      description: Optional settings for returning full content.
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: x-api-key

````