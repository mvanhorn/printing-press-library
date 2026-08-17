> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Fast Entity Search

> Return ranked entities matching a natural language objective.

This endpoint performs a best-effort search optimized for low latency. To keep
responses fast, it returns a fixed set of attributes and supports queries of
limited complexity.

For comprehensive match evaluation and enrichment, use the
[FindAll API](https://docs.parallel.ai/findall-api/findall-quickstart).



## OpenAPI

````yaml /public-openapi.json post /v1beta/findall/entity-search
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
  /v1beta/findall/entity-search:
    post:
      tags:
        - FindAll
      summary: Fast Entity Search
      description: >-
        Return ranked entities matching a natural language objective.


        This endpoint performs a best-effort search optimized for low latency.
        To keep

        responses fast, it returns a fixed set of attributes and supports
        queries of

        limited complexity.


        For comprehensive match evaluation and enrichment, use the

        [FindAll API](https://docs.parallel.ai/findall-api/findall-quickstart).
      operationId: findall_entity_search_v1beta_findall_entity_search_post
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/FindAllEntitySearchRequest'
        required: true
      responses:
        '200':
          description: Successful Response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FindAllEntitySearchResponse'
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

            response = client.beta.findall.entity_search(
                entity_type="companies",
                objective="AI startups that raised Series A in 2024",
                match_limit=100,
            )

            for entity in response.entities:
                print(f"{entity.name}: {entity.url}")
        - lang: TypeScript
          source: |-
            import Parallel from "parallel-web";

            const client = new Parallel();

            const response = await client.beta.findall.entity_search({
                entity_type: "companies",
                objective: "AI startups that raised Series A in 2024",
                match_limit: 100,
            });

            for (const entity of response.entities) {
                console.log(`${entity.name}: ${entity.url}`);
            }
components:
  schemas:
    FindAllEntitySearchRequest:
      properties:
        entity_type:
          type: string
          enum:
            - people
            - companies
          title: Entity Type
          description: Type of entity to search for.
        objective:
          type: string
          title: Objective
          description: Natural language description of target entities.
        match_limit:
          type: integer
          maximum: 1000
          minimum: 5
          title: Match Limit
          description: >-
            Maximum number of entities to return. Must be between 5 and 1000
            (inclusive). May return fewer results. Defaults to 100.
          default: 100
      type: object
      required:
        - entity_type
        - objective
      title: FindAllEntitySearchRequest
    FindAllEntitySearchResponse:
      properties:
        entity_set_id:
          type: string
          title: Entity Set Id
          description: >-
            Entity set request ID. Example:
            `entity_set_cad0a6d2dec046bd95ae900527d880e7`
        entities:
          items:
            $ref: '#/components/schemas/EntityItem'
          type: array
          title: Entities
          description: Ranked list of entities.
      type: object
      required:
        - entity_set_id
        - entities
      title: FindAllEntitySearchResponse
    HTTPValidationError:
      properties:
        detail:
          items:
            $ref: '#/components/schemas/ValidationError'
          type: array
          title: Detail
      type: object
      title: HTTPValidationError
    EntityItem:
      properties:
        name:
          type: string
          title: Name
          description: Entity name.
        url:
          type: string
          title: Url
          description: Canonical URL for the entity.
        description:
          type: string
          title: Description
          description: Descriptive text about the entity.
      type: object
      required:
        - name
        - url
        - description
      title: EntityItem
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
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: x-api-key

````