> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Create FindAll Run

> Starts a FindAll run.

This endpoint immediately returns a FindAll run object with status set to 'queued'.
You can get the run result snapshot using the GET /v1beta/findall/runs/{findall_id}/result endpoint.
You can track the progress of the run by:
- Polling the status using the GET /v1beta/findall/runs/{findall_id} endpoint,
- Subscribing to real-time updates via the /v1beta/findall/runs/{findall_id}/events
endpoint,
- Or specifying a webhook with relevant event types during run creation to receive
notifications.



## OpenAPI

````yaml /public-openapi.json post /v1beta/findall/runs
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
  /v1beta/findall/runs:
    post:
      tags:
        - FindAll
      summary: Create FindAll Run
      description: >-
        Starts a FindAll run.


        This endpoint immediately returns a FindAll run object with status set
        to 'queued'.

        You can get the run result snapshot using the GET
        /v1beta/findall/runs/{findall_id}/result endpoint.

        You can track the progress of the run by:

        - Polling the status using the GET /v1beta/findall/runs/{findall_id}
        endpoint,

        - Subscribing to real-time updates via the
        /v1beta/findall/runs/{findall_id}/events

        endpoint,

        - Or specifying a webhook with relevant event types during run creation
        to receive

        notifications.
      operationId: findall_runs_v1_v1beta_findall_runs_post
      parameters:
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
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/FindAllRunInput'
      responses:
        '200':
          description: Successful Response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FindAllRun'
              example:
                findall_id: findall_56ccc4d188fb41a0803a935cf485c774
                status:
                  status: queued
                  is_active: true
                  metrics:
                    generated_candidates_count: 0
                    matched_candidates_count: 0
                generator: base
                metadata: {}
                created_at: '2025-09-10T21:02:08.626446Z'
                modified_at: '2025-09-10T21:02:08.627376Z'
        '402':
          description: 'Payment required: insufficient credit in account'
          content:
            application/json:
              example:
                type: error
                error:
                  ref_id: fcb2b4f3-c75e-4186-87bc-caa1a8381331
                  message: 'Payment required: insufficient credit in account'
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '422':
          description: 'Unprocessable content: request validation error'
          content:
            application/json:
              example:
                type: error
                error:
                  ref_id: fcb2b4f3-c75e-4186-87bc-caa1a8381331
                  message: 'Unprocessable content: request validation error'
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '429':
          description: 'Too many requests: quota temporarily exceeded'
          content:
            application/json:
              example:
                type: error
                error:
                  ref_id: fcb2b4f3-c75e-4186-87bc-caa1a8381331
                  message: 'Too many requests: quota temporarily exceeded'
              schema:
                $ref: '#/components/schemas/ErrorResponse'
      x-code-samples:
        - lang: Python
          source: |-
            from parallel import Parallel

            client = Parallel()

            # Use the output from the ingest step, or provide your own values
            ingest = client.beta.findall.ingest(
                objective="Find all AI companies that raised Series A funding in 2024",
            )

            run = client.beta.findall.create(
                objective=ingest.objective,
                entity_type=ingest.entity_type,
                match_conditions=[mc.model_dump() for mc in ingest.match_conditions],
                generator="base",
                match_limit=10,
            )

            print(f"FindAll run {run.findall_id} created, response:")
            print(run.model_dump_json(indent=2))
        - lang: TypeScript
          source: |-
            import Parallel from "parallel-web";

            const client = new Parallel();

            // Use the output from the ingest step, or provide your own values
            const ingest = await client.beta.findall.ingest({
                objective: "Find all AI companies that raised Series A funding in 2024",
            });

            const run = await client.beta.findall.create({
                objective: ingest.objective,
                entity_type: ingest.entity_type,
                match_conditions: ingest.match_conditions,
                generator: "base",
                match_limit: 10,
            });

            console.log(`FindAll run ${run.findall_id} created, response:`);
            console.log(JSON.stringify(run, null, 2));
components:
  schemas:
    FindAllRunInput:
      properties:
        objective:
          type: string
          title: Objective
          description: Natural language objective of the FindAll run.
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
        generator:
          type: string
          enum:
            - base
            - core
            - pro
            - preview
          title: Generator
          description: Generator for the FindAll run. One of base, core, pro, preview.
        match_limit:
          type: integer
          title: Match Limit
          description: >-
            Maximum number of matches to find for this FindAll run. Must be
            between 5 and 1000 (inclusive). May return fewer results.
        exclude_list:
          anyOf:
            - items:
                $ref: '#/components/schemas/ExcludeCandidate'
              type: array
              maxItems: 10000
            - type: 'null'
          title: Exclude List
          description: >-
            List of entity names/IDs to exclude from results. At most 10,000
            entries are allowed.
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
        webhook:
          anyOf:
            - $ref: '#/components/schemas/Webhook'
            - type: 'null'
          description: Webhook for the FindAll run.
      type: object
      required:
        - objective
        - entity_type
        - match_conditions
        - generator
        - match_limit
      title: FindAllRunInput
      description: Input model for FindAll run.
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
    ExcludeCandidate:
      properties:
        name:
          type: string
          title: Name
          description: Name of the entity to exclude from results.
        url:
          type: string
          title: Url
          description: URL of the entity to exclude from results.
      type: object
      required:
        - name
        - url
      title: ExcludeCandidate
      description: Exclude candidate input model for FindAll run.
    Webhook:
      properties:
        url:
          type: string
          title: Url
          description: URL for the webhook.
        event_types:
          items:
            type: string
            enum:
              - task_run.status
          type: array
          title: Event Types
          description: Event types to send the webhook notifications for.
          default: []
      type: object
      required:
        - url
      title: Webhook
      description: Webhooks for Task Runs.
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
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: x-api-key

````