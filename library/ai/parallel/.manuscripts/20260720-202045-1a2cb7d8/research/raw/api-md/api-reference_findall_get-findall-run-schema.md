> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Get FindAll Run Schema



## OpenAPI

````yaml /public-openapi.json get /v1beta/findall/runs/{findall_id}/schema
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
  /v1beta/findall/runs/{findall_id}/schema:
    get:
      tags:
        - FindAll
      summary: Get FindAll Run Schema
      operationId: get_findall_schema_v1beta_findall_runs__findall_id__schema_get
      parameters:
        - name: findall_id
          in: path
          required: true
          schema:
            type: string
            title: Findall Id
        - name: parallel-beta
          in: header
          required: false
          schema:
            anyOf:
              - type: string
              - type: 'null'
            deprecated: true
            title: Parallel-Beta
            x-stainless-override-schema:
              x-stainless-param: betas
              x-stainless-extend-default: true
              type: array
              description: Optional header to specify the beta version(s) to enable.
              items:
                $ref: '#/components/schemas/ParallelBeta'
          deprecated: true
      responses:
        '200':
          description: Successful Response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FindAllSchema'
              example:
                objective: Find all AI companies that raised Series A funding in 2024
                entity_type: companies
                match_conditions:
                  - name: developing_ai_products_check
                    description: >-
                      Company must be developing artificial intelligence (AI)
                      products
                enrichments:
                  - processor: core
                    output_schema:
                      json_schema:
                        type: object
                        properties:
                          ceo_name:
                            type: string
                            description: >-
                              Name of the current CEO of the company. If the CEO
                              is not publicly known, provide the name of the
                              highest-ranking executive or founder. If no
                              information is available, return null.
                      type: json
                generator: core
                match_limit: 50
        '422':
          description: Validation Error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/HTTPValidationError'
      x-code-samples:
        - lang: Python
          source: |-
            from parallel import Parallel

            client = Parallel()

            schema = client.beta.findall.schema(
                findall_id="findall_56ccc4d188fb41a0803a935cf485c774",
            )

            print(f"Schema: {schema.model_dump_json(indent=2)}")
        - lang: TypeScript
          source: >-
            import Parallel from "parallel-web";


            const client = new Parallel();


            const schema = await
            client.beta.findall.schema("findall_56ccc4d188fb41a0803a935cf485c774");


            console.log(`Schema: ${JSON.stringify(schema, null, 2)}`);
components:
  schemas:
    FindAllSchema:
      properties:
        objective:
          type: string
          title: Objective
          description: Natural language objective of the FindAll run.
          examples:
            - Find all AI companies that raised Series A funding in 2024
        entity_type:
          type: string
          title: Entity Type
          description: Type of the entity for the FindAll run.
        match_conditions:
          items:
            $ref: '#/components/schemas/MatchCondition'
          type: array
          title: Match Conditions
          description: List of match conditions for the FindAll run.
        enrichments:
          anyOf:
            - items:
                $ref: '#/components/schemas/FindAllEnrichInput'
              type: array
            - type: 'null'
          title: Enrichments
          description: List of enrichment inputs for the FindAll run.
        generator:
          type: string
          enum:
            - base
            - core
            - pro
            - preview
          title: Generator
          description: The generator of the FindAll run.
          default: core
        match_limit:
          anyOf:
            - type: integer
            - type: 'null'
          title: Match Limit
          description: Max number of candidates to evaluate
      type: object
      required:
        - objective
        - entity_type
        - match_conditions
      title: FindAllSchema
      description: Response model for FindAll ingest.
    HTTPValidationError:
      properties:
        detail:
          items:
            $ref: '#/components/schemas/ValidationError'
          type: array
          title: Detail
      type: object
      title: HTTPValidationError
    MatchCondition:
      properties:
        name:
          type: string
          title: Name
          description: Name of the match condition.
        description:
          type: string
          title: Description
          description: >-
            Detailed description of the match condition. Include as much
            specific information as possible to help improve the quality and
            accuracy of Find All run results.
          examples:
            - >-
              Company must have SOC2 Type II certification (not Type I). Look
              for evidence in: trust centers, security/compliance pages, audit
              reports, or press releases specifically mentioning 'SOC2 Type II'.
              If no explicit SOC2 Type II mention is found, consider requirement
              not satisfied.
      type: object
      required:
        - name
        - description
      title: MatchCondition
      description: Match condition model for FindAll ingest.
    FindAllEnrichInput:
      properties:
        processor:
          type: string
          title: Processor
          description: Processor to use for the task.
          default: core
        output_schema:
          $ref: '#/components/schemas/JsonSchema'
          description: JSON schema for the enrichment output schema for the FindAll run.
        mcp_servers:
          anyOf:
            - items:
                $ref: '#/components/schemas/McpServer'
              type: array
            - type: 'null'
          title: Mcp Servers
          description: List of MCP servers to use for the task.
      type: object
      required:
        - output_schema
      title: FindAllEnrichInput
      description: Input model for FindAll enrich.
    ValidationError:
      properties:
        loc:
          items:
            anyOf:
              - type: string
              - type: integer
          type: array
          title: Location
        msg:
          type: string
          title: Message
        type:
          type: string
          title: Error Type
      type: object
      required:
        - loc
        - msg
        - type
      title: ValidationError
    JsonSchema:
      properties:
        json_schema:
          additionalProperties: true
          type: object
          title: Json Schema
          description: A JSON Schema object. Only a subset of JSON Schema is supported.
          examples:
            - additionalProperties: false
              properties:
                gdp:
                  description: >-
                    GDP in USD for the year, formatted like '$3.1 trillion
                    (2023)'
                  type: string
              required:
                - gdp
              type: object
        type:
          type: string
          const: json
          title: Type
          description: The type of schema being defined. Always `json`.
          default: json
      type: object
      required:
        - json_schema
      title: JsonSchema
      description: JSON schema for a task input or output.
    McpServer:
      properties:
        type:
          type: string
          const: url
          title: Type
          description: Type of MCP server being configured. Always `url`.
          default: url
        url:
          type: string
          title: Url
          description: URL of the MCP server.
        headers:
          anyOf:
            - additionalProperties:
                type: string
                format: password
                writeOnly: true
              type: object
            - type: 'null'
          title: Headers
          description: Headers for the MCP server.
        name:
          type: string
          title: Name
          description: Name of the MCP server.
        allowed_tools:
          anyOf:
            - items:
                type: string
              type: array
            - type: 'null'
          title: Allowed Tools
          description: List of allowed tools for the MCP server.
      type: object
      required:
        - url
        - name
      title: McpServer
      description: MCP server configuration.
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: x-api-key

````