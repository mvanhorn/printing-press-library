> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Retrieve FindAll Run Status

> Retrieve a FindAll run.



## OpenAPI

````yaml /public-openapi.json get /v1beta/findall/runs/{findall_id}
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
  /v1beta/findall/runs/{findall_id}:
    get:
      tags:
        - FindAll
      summary: Retrieve FindAll Run Status
      description: Retrieve a FindAll run.
      operationId: findall_runs_v1_get_v1beta_findall_runs__findall_id__get
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
                $ref: '#/components/schemas/FindAllRun'
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

            run = client.beta.findall.retrieve(
                findall_id="findall_56ccc4d188fb41a0803a935cf485c774",
            )

            print(f"FindAll run {run.findall_id} status: {run.status.status}")
        - lang: TypeScript
          source: >-
            import Parallel from "parallel-web";


            const client = new Parallel();


            const run = await
            client.beta.findall.retrieve("findall_56ccc4d188fb41a0803a935cf485c774");


            console.log(`FindAll run ${run.findall_id} status:
            ${run.status.status}`);
components:
  schemas:
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
    HTTPValidationError:
      properties:
        detail:
          items:
            $ref: '#/components/schemas/ValidationError'
          type: array
          title: Detail
      type: object
      title: HTTPValidationError
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