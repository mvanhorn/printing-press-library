> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Stream FindAll Events

> Stream events from a FindAll run.

Args:
    request: The Shapi request
    findall_id: The FindAll run ID
    last_event_id: Optional event ID to resume from.
    timeout: Optional timeout in seconds. If None, keep connection alive
    as long as the run is going. If set, stop after specified duration.



## OpenAPI

````yaml /public-openapi.json get /v1beta/findall/runs/{findall_id}/events
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
  /v1beta/findall/runs/{findall_id}/events:
    get:
      tags:
        - FindAll
      summary: Stream FindAll Events
      description: |-
        Stream events from a FindAll run.

        Args:
            request: The Shapi request
            findall_id: The FindAll run ID
            last_event_id: Optional event ID to resume from.
            timeout: Optional timeout in seconds. If None, keep connection alive
            as long as the run is going. If set, stop after specified duration.
      operationId: get_findall_events_v1beta_findall_runs__findall_id__events_get
      parameters:
        - name: findall_id
          in: path
          required: true
          schema:
            type: string
            title: Findall Id
        - name: last_event_id
          in: query
          required: false
          schema:
            anyOf:
              - type: string
              - type: 'null'
            title: Last Event Id
        - name: timeout
          in: query
          required: false
          schema:
            anyOf:
              - type: number
              - type: 'null'
            title: Timeout
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
            text/event-stream:
              schema:
                oneOf:
                  - $ref: '#/components/schemas/FindAllSchemaUpdatedEvent'
                  - $ref: '#/components/schemas/FindAllRunStatusEvent'
                  - $ref: '#/components/schemas/FindAllCandidateMatchStatusEvent'
                  - $ref: '#/components/schemas/ErrorEvent'
                discriminator:
                  propertyName: type
                  mapping:
                    findall.schema.updated:
                      $ref: '#/components/schemas/FindAllSchemaUpdatedEvent'
                    findall.status:
                      $ref: '#/components/schemas/FindAllRunStatusEvent'
                    findall.candidate.generated:
                      $ref: '#/components/schemas/FindAllCandidateMatchStatusEvent'
                    findall.candidate.matched:
                      $ref: '#/components/schemas/FindAllCandidateMatchStatusEvent'
                    findall.candidate.unmatched:
                      $ref: '#/components/schemas/FindAllCandidateMatchStatusEvent'
                    findall.candidate.discarded:
                      $ref: '#/components/schemas/FindAllCandidateMatchStatusEvent'
                    findall.candidate.enriched:
                      $ref: '#/components/schemas/FindAllCandidateMatchStatusEvent'
                    error:
                      $ref: '#/components/schemas/ErrorEvent'
                title: >-
                  Response 200 Get Findall Events V1Beta Findall Runs  Findall
                  Id  Events Get
              example:
                type: findall.candidate.generated
                timestamp: '2025-09-10T21:02:08.626446Z'
                event_id: 56cee734dbc84172bfc491327f2a0183
                data:
                  candidate_id: candidate_52e1e30b-4e0a-49d8-82eb-79e64e0ed015
                  name: Pika
                  url: pika.art
                  description: >-
                    Pika is an AI video generation platform that creates and
                    edits videos from text prompts or images.
                  match_status: generated
        '404':
          description: FindAll run not found
          content:
            application/json:
              example:
                type: error
                error:
                  ref_id: fcb2b4f3-c75e-4186-87bc-caa1a8381331
                  message: FindAll run not found
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '422':
          description: Validation Error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/HTTPValidationError'
      x-code-samples:
        - lang: Python
          source: >-
            from parallel import Parallel


            client = Parallel()


            # Event types: findall.candidate.generated,
            findall.candidate.matched,

            # findall.candidate.unmatched, findall.candidate.discarded,

            # findall.candidate.enriched

            events = client.beta.findall.events(
                findall_id="findall_56ccc4d188fb41a0803a935cf485c774",
            )


            for event in events:
                print(f"Event [{event.type}]: {event.model_dump_json()}")
        - lang: TypeScript
          source: >-
            import Parallel from "parallel-web";


            const client = new Parallel();


            // Event types: findall.candidate.generated,
            findall.candidate.matched,

            // findall.candidate.unmatched, findall.candidate.discarded,

            // findall.candidate.enriched

            const events = await
            client.beta.findall.events("findall_56ccc4d188fb41a0803a935cf485c774");


            for await (const event of events) {
                console.log(`Event [${event.type}]: ${JSON.stringify(event)}`);
            }
components:
  schemas:
    FindAllSchemaUpdatedEvent:
      properties:
        type:
          type: string
          enum:
            - findall.schema.updated
          const: findall.schema.updated
          title: Type
          description: Event type; always 'findall.schema.updated'.
        timestamp:
          type: string
          format: date-time
          title: Timestamp
          description: Timestamp of the event.
        event_id:
          type: string
          title: Event Id
          description: Unique event identifier for the event.
        data:
          $ref: '#/components/schemas/FindAllSchema'
          description: Updated FindAll schema.
      type: object
      required:
        - type
        - timestamp
        - event_id
        - data
      title: FindAllSchemaUpdatedEvent
      description: Event containing full snapshot of FindAll run state.
    FindAllRunStatusEvent:
      properties:
        type:
          type: string
          enum:
            - findall.status
          const: findall.status
          title: Type
          description: Event type; always 'findall.status'.
        timestamp:
          type: string
          format: date-time
          title: Timestamp
          description: Timestamp of the event.
        event_id:
          type: string
          title: Event Id
          description: Unique event identifier for the event.
        data:
          $ref: '#/components/schemas/FindAllRun'
          description: Updated FindAll run information.
      type: object
      required:
        - type
        - timestamp
        - event_id
        - data
      title: FindAllRunStatusEvent
      description: Event containing status update for FindAll run.
    FindAllCandidateMatchStatusEvent:
      properties:
        type:
          type: string
          enum:
            - findall.candidate.generated
            - findall.candidate.matched
            - findall.candidate.unmatched
            - findall.candidate.discarded
            - findall.candidate.enriched
          title: Type
          description: >-
            Event type; one of findall.candidate.generated,
            findall.candidate.matched, findall.candidate.unmatched,
            findall.candidate.discarded, findall.candidate.enriched.
        timestamp:
          type: string
          format: date-time
          title: Timestamp
          description: Timestamp of the event.
        event_id:
          type: string
          title: Event Id
          description: Unique event identifier for the event.
        data:
          $ref: '#/components/schemas/FindAllCandidate'
          description: The candidate whose match status has been updated.
      type: object
      required:
        - type
        - timestamp
        - event_id
        - data
      title: FindAllCandidateMatchStatusEvent
      description: Event containing a candidate whose match status has changed.
    ErrorEvent:
      properties:
        type:
          type: string
          const: error
          title: Type
          description: Event type; always 'error'.
        error:
          $ref: '#/components/schemas/Error'
          description: Error.
      type: object
      required:
        - type
        - error
      title: ErrorEvent
      description: Event indicating an error.
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
    HTTPValidationError:
      properties:
        detail:
          items:
            $ref: '#/components/schemas/ValidationError'
          type: array
          title: Detail
      type: object
      title: HTTPValidationError
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
    FindAllRun:
      properties:
        findall_id:
          type: string
          title: FindAll ID
          description: ID of the FindAll run.
        status:
          $ref: '#/components/schemas/FindAllRunStatus'
          description: Status object for the FindAll run.
        generator:
          type: string
          enum:
            - base
            - core
            - pro
            - preview
          title: Generator
          description: Generator for the FindAll run.
        metadata:
          anyOf:
            - additionalProperties:
                anyOf:
                  - type: string
                  - type: integer
                  - type: number
                  - type: boolean
              type: object
            - type: 'null'
          title: Metadata
          description: Metadata for the FindAll run.
        created_at:
          anyOf:
            - type: string
            - type: 'null'
          title: Created At
          description: Timestamp of the creation of the run, in RFC 3339 format.
        modified_at:
          anyOf:
            - type: string
            - type: 'null'
          title: Modified At
          description: >-
            Timestamp of the latest modification to the FindAll run result, in
            RFC 3339 format.
      type: object
      required:
        - findall_id
        - status
        - generator
      title: FindAllRun
      description: FindAll run object with status and metadata.
    FindAllCandidate:
      properties:
        candidate_id:
          type: string
          title: Candidate ID
          description: ID of the candidate.
        name:
          type: string
          title: Name
          description: Name of the candidate.
        url:
          type: string
          title: Url
          description: >-
            URL that provides context or details of the entity for
            disambiguation.
        description:
          anyOf:
            - type: string
            - type: 'null'
          title: Description
          description: >-
            Brief description of the entity that can help answer whether entity
            satisfies the query.
        match_status:
          type: string
          enum:
            - generated
            - matched
            - unmatched
            - discarded
          title: Match Status
          description: >-
            Status of the candidate. One of generated, matched, unmatched,
            discarded.
        output:
          anyOf:
            - additionalProperties: true
              type: object
            - type: 'null'
          title: Output
          description: >-
            Results of the match condition evaluations for this candidate. This
            object contains the structured output that determines whether the
            candidate matches the overall FindAll objective.
        basis:
          anyOf:
            - items:
                $ref: '#/components/schemas/FieldBasis'
              type: array
            - type: 'null'
          title: Basis
          description: List of FieldBasis objects supporting the output.
      type: object
      required:
        - candidate_id
        - name
        - url
        - match_status
      title: FindAllCandidate
      description: >-
        Candidate for a find all run that may end up as a match.


        Contains all the candidate's metadata and the output of the match
        conditions.

        A candidate is a match if all match conditions are satisfied.
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
    FindAllRunStatus:
      properties:
        status:
          type: string
          enum:
            - queued
            - action_required
            - running
            - completed
            - failed
            - cancelling
            - cancelled
          title: Status
          description: Status of the FindAll run.
        is_active:
          type: boolean
          title: Is Active
          description: Whether the FindAll run is active
        metrics:
          $ref: '#/components/schemas/FindAllCandidateMetrics'
          description: Candidate metrics for the FindAll run.
        termination_reason:
          anyOf:
            - type: string
              enum:
                - low_match_rate
                - match_limit_met
                - candidates_exhausted
                - user_cancelled
                - error_occurred
                - timeout
                - insufficient_funds
            - type: 'null'
          title: Termination Reason
          description: Reason for termination when FindAll run is in terminal status.
      type: object
      required:
        - status
        - is_active
        - metrics
      title: FindAllRunStatus
      description: Status object for FindAll run.
    FieldBasis:
      description: Citations and reasoning supporting one field of a task output.
      properties:
        field:
          description: Name of the output field.
          title: Field
          type: string
        citations:
          default: []
          description: List of citations supporting the output field.
          items:
            $ref: '#/components/schemas/Citation'
          title: Citations
          type: array
        reasoning:
          description: Reasoning for the output field.
          title: Reasoning
          type: string
        confidence:
          anyOf:
            - type: string
            - type: 'null'
          default: null
          description: >-
            Confidence level for the output field. Only certain processors
            provide confidence levels.
          examples:
            - low
            - medium
            - high
          title: Confidence
      required:
        - field
        - reasoning
      title: FieldBasis
      type: object
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
    FindAllCandidateMetrics:
      properties:
        generated_candidates_count:
          type: integer
          title: Generated Candidates Count
          description: Number of candidates that were selected.
          default: 0
        matched_candidates_count:
          type: integer
          title: Matched Candidates Count
          description: Number of candidates that evaluated to matched.
          default: 0
      type: object
      title: FindAllCandidateMetrics
      description: Metrics object for FindAll run.
    Citation:
      description: A citation for a task output.
      properties:
        title:
          anyOf:
            - type: string
            - type: 'null'
          default: null
          description: Title of the citation.
          title: Title
        url:
          description: URL of the citation.
          title: Url
          type: string
        excerpts:
          anyOf:
            - items:
                type: string
              type: array
            - type: 'null'
          default: null
          description: >-
            Excerpts from the citation supporting the output. Only certain
            processors provide excerpts.
          title: Excerpts
      required:
        - url
      title: Citation
      type: object
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: x-api-key

````