> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Retrieve Task Run Result

> Retrieves a run result by run_id, blocking until the run is completed.



## OpenAPI

````yaml /public-openapi.json get /v1/tasks/runs/{run_id}/result
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
  /v1/tasks/runs/{run_id}/result:
    get:
      tags:
        - Tasks
      summary: Retrieve Task Run Result
      description: Retrieves a run result by run_id, blocking until the run is completed.
      operationId: tasks_runs_result_get_v1_tasks_runs__run_id__result_get
      parameters:
        - name: run_id
          in: path
          required: true
          schema:
            type: string
            title: Run Id
        - name: timeout
          in: query
          required: false
          schema:
            type: integer
            default: 600
            title: Timeout
        - name: parallel-beta
          in: header
          required: false
          schema:
            anyOf:
              - type: string
              - type: 'null'
            title: Parallel-Beta
            x-stainless-override-schema:
              x-stainless-param: betas
              x-stainless-extend-default: true
              type: array
              description: Optional header to specify the beta version(s) to enable.
              items:
                $ref: '#/components/schemas/ParallelBeta'
      responses:
        '200':
          description: Successful Response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/TaskRunResult'
              example:
                run:
                  run_id: trun_9907962f83aa4d9d98fd7f4bf745d654
                  interaction_id: trun_9907962f83aa4d9d98fd7f4bf745d654
                  status: completed
                  is_active: false
                  processor: core
                  metadata:
                    my_key: my_value
                  created_at: '2025-04-23T20:21:48.037943Z'
                  modified_at: '2025-04-23T20:21:48.037943Z'
                output:
                  basis: []
                  type: json
                  content:
                    gdp: $3.1 trillion (2023)
        '401':
          description: 'Unauthorized: invalid or missing credentials'
          content:
            application/json:
              example:
                type: error
                error:
                  ref_id: fcb2b4f3-c75e-4186-87bc-caa1a8381331
                  message: 'Unauthorized: invalid or missing credentials'
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '404':
          description: Run failed or run id not found
          content:
            application/json:
              example:
                type: error
                error:
                  ref_id: fcb2b4f3-c75e-4186-87bc-caa1a8381331
                  message: Run failed or run id not found
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '408':
          description: Request timed out; run still active
          content:
            application/json:
              example:
                type: error
                error:
                  ref_id: fcb2b4f3-c75e-4186-87bc-caa1a8381331
                  message: Request timed out; run still active
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '422':
          description: Request validation error
          content:
            application/json:
              example:
                type: error
                error:
                  ref_id: fcb2b4f3-c75e-4186-87bc-caa1a8381331
                  message: Request validation error
              schema:
                $ref: '#/components/schemas/ErrorResponse'
      x-code-samples:
        - lang: Python
          source: |-
            from parallel import Parallel

            client = Parallel()

            task_run_result = client.task_run.result(run_id="run_id")
            print(task_run_result.output)
        - lang: TypeScript
          source: |-
            import Parallel from "parallel-web";

            const client = new Parallel();

            const taskRunResult = await client.taskRun.result('run_id');
            console.log(taskRunResult.output);
components:
  schemas:
    TaskRunResult:
      properties:
        run:
          $ref: '#/components/schemas/TaskRun'
          description: Task run object with status 'completed'.
        output:
          oneOf:
            - $ref: '#/components/schemas/TaskRunTextOutput'
            - $ref: '#/components/schemas/TaskRunJsonOutput'
          title: Output
          description: Output from the task conforming to the output schema.
          discriminator:
            propertyName: type
            mapping:
              json:
                $ref: '#/components/schemas/TaskRunJsonOutput'
              text:
                $ref: '#/components/schemas/TaskRunTextOutput'
      type: object
      required:
        - run
        - output
      title: TaskRunResult
      description: Result of a task run.
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
    TaskRun:
      properties:
        run_id:
          type: string
          title: Run ID
          description: ID of the task run.
          examples:
            - trun_e0083b6aac0544eb8686e8d2a76533d2
        interaction_id:
          type: string
          title: Interaction ID
          description: >-
            Identifier for this interaction. Pass this value as
            `previous_interaction_id` to reuse context for a future request.
          examples:
            - trun_e0083b6aac0544eb8686e8d2a76533d2
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
          description: Status of the run.
          examples:
            - queued
            - action_required
            - running
            - completed
            - failed
            - cancelling
            - cancelled
        is_active:
          type: boolean
          title: Is Active
          description: >-
            Whether the run is currently active, i.e. status is one of
            {'cancelling', 'queued', 'running'}.
        warnings:
          anyOf:
            - items:
                $ref: '#/components/schemas/Warning'
              type: array
            - type: 'null'
          title: Warnings
          description: Warnings for the run, if any.
          examples:
            - []
        error:
          anyOf:
            - $ref: '#/components/schemas/Error'
            - type: 'null'
          description: Error for the run, present only if status is 'failed'.
        processor:
          type: string
          title: Processor
          description: Processor used for the run.
          examples:
            - base
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
          description: User-provided metadata stored with the run.
          examples:
            - {}
        taskgroup_id:
          anyOf:
            - type: string
            - type: 'null'
          title: Taskgroup ID
          description: ID of the taskgroup to which the run belongs.
        created_at:
          anyOf:
            - type: string
            - type: 'null'
          title: Created At
          description: Timestamp of the creation of the task, as an RFC 3339 string.
          examples:
            - '2025-04-24T18:56:22.513132Z'
        modified_at:
          anyOf:
            - type: string
            - type: 'null'
          title: Modified At
          description: >-
            Timestamp of the last modification to the task, as an RFC 3339
            string.
          examples:
            - '2025-04-24T18:56:22.513132Z'
      type: object
      required:
        - run_id
        - interaction_id
        - status
        - is_active
        - processor
        - created_at
        - modified_at
      title: TaskRun
      description: Status of a task run.
    TaskRunTextOutput:
      properties:
        basis:
          items:
            $ref: '#/components/schemas/FieldBasis'
          type: array
          title: Basis
          description: Basis for the output. The basis has a single field 'output'.
        type:
          type: string
          const: text
          title: Type
          description: >-
            The type of output being returned, as determined by the output
            schema of the task spec.
        mcp_tool_calls:
          anyOf:
            - items:
                $ref: '#/components/schemas/McpToolCall'
              type: array
            - type: 'null'
          title: Mcp Tool Calls
          description: MCP tool calls made by the task.
        beta_fields:
          anyOf:
            - additionalProperties: true
              type: object
            - type: 'null'
          title: Beta Fields
          description: >-
            Deprecated. mcp-server-2025-07-17 is now included directly in the
            output (e.g. mcp_tool_calls).
          deprecated: true
        content:
          type: string
          title: Content
          description: Text output from the task.
      type: object
      required:
        - basis
        - type
        - content
      title: TaskRunTextOutput
      description: Output from a task that returns text.
    TaskRunJsonOutput:
      properties:
        basis:
          items:
            $ref: '#/components/schemas/FieldBasis'
          type: array
          title: Basis
          description: >-
            Basis for each top-level field in the JSON output. Per-list-element
            basis entries are available only when the `parallel-beta:
            field-basis-2025-11-25` header is supplied.
        type:
          type: string
          const: json
          title: Type
          description: >-
            The type of output being returned, as determined by the output
            schema of the task spec.
        mcp_tool_calls:
          anyOf:
            - items:
                $ref: '#/components/schemas/McpToolCall'
              type: array
            - type: 'null'
          title: Mcp Tool Calls
          description: MCP tool calls made by the task.
        beta_fields:
          anyOf:
            - additionalProperties: true
              type: object
            - type: 'null'
          title: Beta Fields
          description: >-
            Deprecated. mcp-server-2025-07-17 is now included directly in the
            output (e.g. mcp_tool_calls).
          deprecated: true
        content:
          additionalProperties: true
          type: object
          title: Content
          description: >-
            Output from the task as a native JSON object, as determined by the
            output schema of the task spec.
        output_schema:
          anyOf:
            - additionalProperties: true
              type: object
            - type: 'null'
          title: Output Schema
          description: >-
            Output schema for the Task Run. Populated only if the task was
            executed with an auto schema.
      type: object
      required:
        - basis
        - type
        - content
      title: TaskRunJsonOutput
      description: Output from a task that returns JSON.
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
    McpToolCall:
      properties:
        tool_call_id:
          type: string
          title: Tool Call ID
          description: Identifier for the tool call.
        server_name:
          type: string
          title: Server Name
          description: Name of the MCP server.
        tool_name:
          type: string
          title: Tool Name
          description: Name of the tool being called.
        arguments:
          type: string
          title: Arguments
          description: Arguments used to call the MCP tool.
        content:
          anyOf:
            - type: string
            - type: 'null'
          title: Content
          description: Output received from the tool call, if successful.
        error:
          anyOf:
            - type: string
            - type: 'null'
          title: Error
          description: Error message if the tool call failed.
      type: object
      required:
        - tool_call_id
        - server_name
        - tool_name
        - arguments
      title: McpToolCall
      description: Result of an MCP tool call.
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