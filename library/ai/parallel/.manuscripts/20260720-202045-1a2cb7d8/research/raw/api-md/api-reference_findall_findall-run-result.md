> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# FindAll Run Result

> Retrieve the FindAll run result at the time of the request.



## OpenAPI

````yaml /public-openapi.json get /v1beta/findall/runs/{findall_id}/result
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
  /v1beta/findall/runs/{findall_id}/result:
    get:
      tags:
        - FindAll
      summary: FindAll Run Result
      description: Retrieve the FindAll run result at the time of the request.
      operationId: get_findall_result_v1beta_findall_runs__findall_id__result_get
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
                $ref: '#/components/schemas/FindAllRunResult'
              example:
                run:
                  findall_id: findall_56ccc4d188fb41a0803a935cf485c774
                  status:
                    status: running
                    is_active: true
                    metrics:
                      generated_candidates_count: 1
                      matched_candidates_count: 1
                  generator: base
                  metadata: {}
                  created_at: '2025-09-10T21:02:08.626446Z'
                  modified_at: '2025-09-10T21:02:08.627376Z'
                candidates:
                  - candidate_id: candidate_7594eb7c-4f4a-487f-9d0c-9d1e63ec240c
                    name: Cognition AI
                    url: cognition.ai
                    match_status: matched
                    output:
                      developing_ai_products_check: 'yes'
                      raised_series_a_2024_check: 'yes'
                    basis:
                      - field: developing_ai_products_check
                        citations:
                          - title: Cognition - Devin and Cognition AI
                            url: https://cognition.ai/
                            excerpts:
                              - >-
                                We're the makers of Devin, a collaborative AI
                                teammate that helps ambitious engineering teams
                                achieve more.
                              - >-
                                An applied AI lab building the future of
                                software engineering
                              - Cognition
                        reasoning: >-
                          The search results repeatedly state that Cognition AI
                          is an 'applied AI lab building the future of software
                          engineering' and that they developed 'Devin AI',
                          described as the 'world's first AI software engineer'.
                          This directly confirms they are developing AI
                          products.
                        confidence: high
                      - field: raised_series_a_2024_check
                        citations:
                          - title: >-
                              Cognition Labs Raises $21 Million Series A to
                              Support AI Coding Products
                            url: >-
                              https://voicebot.ai/2024/04/25/cognition-labs-raises-21-million-series-a-to-support-ai-coding-products/
                            excerpts:
                              - >-
                                Cognition Labs Raises $21 Million Series A to
                                Support AI Coding Products
                        reasoning: >-
                          The article from voicebot.ai, dated April 25, 2024,
                          states that Founders Fund led a "$21 million Series A
                          investment" for Cognition Labs. This confirms that
                          Series A funding was raised in 2024.
                        confidence: low
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


            run_result = client.beta.findall.result(
                findall_id="findall_56ccc4d188fb41a0803a935cf485c774",
            )


            print(f"FindAll run {run_result.run.findall_id} result:
            {run_result.model_dump_json(indent=2)}")
        - lang: TypeScript
          source: >-
            import Parallel from "parallel-web";


            const client = new Parallel();


            const runResult = await
            client.beta.findall.result("findall_56ccc4d188fb41a0803a935cf485c774");


            console.log(`FindAll run ${runResult.run.findall_id} result:
            ${JSON.stringify(runResult, null, 2)}`);
components:
  schemas:
    FindAllRunResult:
      properties:
        run:
          $ref: '#/components/schemas/FindAllRun'
          description: FindAll run object.
        candidates:
          items:
            $ref: '#/components/schemas/FindAllCandidate'
          type: array
          title: Candidates
          description: All evaluated candidates at the time of the snapshot.
        last_event_id:
          anyOf:
            - type: string
            - type: 'null'
          title: Last Event Id
          description: >-
            ID of the last event of the run at the time of the request. This can
            be used to resume streaming from the last event.
      type: object
      required:
        - run
        - candidates
      title: FindAllRunResult
      description: >-
        Complete FindAll search results.


        Represents a snapshot of a FindAll run, including run metadata and a
        list of

        candidate entities with their match status and details at the time the
        snapshot was

        taken.
    HTTPValidationError:
      properties:
        detail:
          items:
            $ref: '#/components/schemas/ValidationError'
          type: array
          title: Detail
      type: object
      title: HTTPValidationError
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