> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Stream Task Group Events

> Streams events from a TaskGroup: status updates and run completions.

The connection will remain open for up to an hour as long as at least one run in the
group is still active.



## OpenAPI

````yaml /public-openapi.json get /v1/tasks/groups/{taskgroup_id}/events
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
  /v1/tasks/groups/{taskgroup_id}/events:
    get:
      tags:
        - Tasks
      summary: Stream Task Group Events
      description: >-
        Streams events from a TaskGroup: status updates and run completions.


        The connection will remain open for up to an hour as long as at least
        one run in the

        group is still active.
      operationId: tasks_sessions_events_get_v1_tasks_groups__taskgroup_id__events_get
      parameters:
        - name: taskgroup_id
          in: path
          required: true
          schema:
            type: string
            title: Taskgroup Id
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
      responses:
        '200':
          description: Successful Response
          content:
            text/event-stream:
              schema:
                oneOf:
                  - $ref: '#/components/schemas/TaskGroupStatusEvent'
                  - $ref: '#/components/schemas/TaskRunEvent'
                  - $ref: '#/components/schemas/ErrorEvent'
                discriminator:
                  propertyName: type
                  mapping:
                    task_group_status:
                      $ref: '#/components/schemas/TaskGroupStatusEvent'
                    task_run.state:
                      $ref: '#/components/schemas/TaskRunEvent'
                    error:
                      $ref: '#/components/schemas/ErrorEvent'
                title: >-
                  Response 200 Tasks Sessions Events Get V1 Tasks Groups 
                  Taskgroup Id  Events Get
              example:
                type: task_group_status
                event_id: '123'
                status:
                  num_task_runs: 1
                  task_run_status_counts:
                    completed: 1
                  is_active: false
                  status_message: ''
                  modified_at: '2025-04-23T20:21:48.037943Z'
        '404':
          description: TaskGroup not found
          content:
            application/json:
              example:
                type: error
                error:
                  ref_id: fcb2b4f3-c75e-4186-87bc-caa1a8381331
                  message: TaskGroup not found
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
          source: |
            from parallel import Parallel

            client = Parallel()

            task_group_events = client.task_group.events("taskgroup_id")
            for event in task_group_events:
                print(event)
        - lang: TypeScript
          source: |
            import Parallel from "parallel-web";

            const client = new Parallel();

            const taskGroupEvents = await client.taskGroup.events(
                'taskgroup_id',
            );
            for await (const event of taskGroupEvents) {
                console.log(event);
            }
components:
  schemas:
    TaskGroupStatusEvent:
      properties:
        type:
          type: string
          const: task_group_status
          title: Type
          description: Event type; always 'task_group_status'.
        event_id:
          type: string
          title: Event ID
          description: Cursor to resume the event stream.
        status:
          $ref: '#/components/schemas/TaskGroupStatus'
          description: Task group status object.
      type: object
      required:
        - type
        - event_id
        - status
      title: TaskGroupStatusEvent
      description: Event indicating an update to group status.
    TaskRunEvent:
      properties:
        type:
          type: string
          const: task_run.state
          title: Type
          description: Event type; always 'task_run.state'.
        event_id:
          anyOf:
            - type: string
            - type: 'null'
          title: Event ID
          description: >-
            Cursor to resume the event stream. Always empty for non Task Group
            runs.
        input:
          anyOf:
            - $ref: '#/components/schemas/TaskRunInput'
            - type: 'null'
          description: Input to the run; included only if requested.
        run:
          $ref: '#/components/schemas/TaskRun'
          description: Task run object.
        output:
          anyOf:
            - oneOf:
                - $ref: '#/components/schemas/TaskRunTextOutput'
                - $ref: '#/components/schemas/TaskRunJsonOutput'
              discriminator:
                propertyName: type
                mapping:
                  json:
                    $ref: '#/components/schemas/TaskRunJsonOutput'
                  text:
                    $ref: '#/components/schemas/TaskRunTextOutput'
            - type: 'null'
          title: Output
          description: >-
            Output from the run; included only if requested and if status ==
            `completed`.
      type: object
      required:
        - type
        - event_id
        - run
      title: TaskRunEvent
      description: |-
        Event when a task run transitions to a non-active status.

        May indicate completion, cancellation, or failure.
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
    TaskGroupStatus:
      properties:
        num_task_runs:
          type: integer
          title: Num Task Runs
          description: Number of task runs in the group.
        task_run_status_counts:
          additionalProperties:
            type: integer
          propertyNames:
            enum:
              - queued
              - action_required
              - running
              - completed
              - failed
              - cancelling
              - cancelled
          type: object
          title: Task Run Status Counts
          description: Number of task runs with each status.
        is_active:
          type: boolean
          title: Is Active
          description: >-
            True if at least one run in the group is currently active, i.e.
            status is one of {'cancelling', 'queued', 'running'}.
        status_message:
          anyOf:
            - type: string
            - type: 'null'
          title: Status Message
          description: Human-readable status message for the group.
        modified_at:
          anyOf:
            - type: string
            - type: 'null'
          title: Modified At
          description: >-
            Timestamp of the last status update to the group, as an RFC 3339
            string.
          examples:
            - '2025-04-24T18:56:22.513132Z'
      type: object
      required:
        - num_task_runs
        - task_run_status_counts
        - is_active
        - status_message
        - modified_at
      title: TaskGroupStatus
      description: Status of a task group.
    TaskRunInput:
      properties:
        processor:
          type: string
          title: Processor
          description: Processor to use for the task.
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
          description: >-
            User-provided metadata stored with the run. Keys and values must be
            strings with a maximum length of 16 and 512 characters respectively.
        source_policy:
          anyOf:
            - $ref: '#/components/schemas/SourcePolicy'
            - type: 'null'
          description: >-
            Optional source policy governing preferred and disallowed domains in
            web search results.
        advanced_settings:
          anyOf:
            - $ref: '#/components/schemas/TaskAdvancedSettings'
            - type: 'null'
          description: Advanced search configuration for the task run.
        task_spec:
          anyOf:
            - $ref: '#/components/schemas/TaskSpec'
            - type: 'null'
          description: Task specification. If unspecified, defaults to auto output schema.
        input:
          anyOf:
            - type: string
            - additionalProperties: true
              type: object
          title: Input
          description: Input to the task, either text or a JSON object.
          examples:
            - What was the GDP of France in 2023?
            - '{"country": "France", "year": 2023}'
        previous_interaction_id:
          anyOf:
            - type: string
            - type: 'null'
          title: Previous Interaction Id
          description: Interaction ID to use as context for this request.
        mcp_servers:
          anyOf:
            - items:
                $ref: '#/components/schemas/McpServer'
              type: array
            - type: 'null'
          title: Mcp Servers
          description: Optional list of MCP servers to use for the run.
        enable_events:
          anyOf:
            - type: boolean
            - type: 'null'
          title: Enable Events
          description: >-
            Controls tracking of task run execution progress. When set to true,
            progress events are recorded and can be accessed via the [Task Run
            events](https://docs.parallel.ai/api-reference) endpoint. When
            false, no progress events are tracked. Note that progress tracking
            cannot be enabled after a run has been created. The flag is set to
            true by default for premium processors (pro and above).
        webhook:
          anyOf:
            - $ref: '#/components/schemas/Webhook'
            - type: 'null'
          description: >-
            Callback URL (webhook endpoint) that will receive an HTTP POST when
            the run completes. 

            This feature is not available via the Python SDK.
      type: object
      required:
        - processor
        - input
      title: TaskRunInput
      description: Request to run a task.
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
    TaskAdvancedSettings:
      properties:
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
      type: object
      title: TaskAdvancedSettings
      description: Advanced search configuration for a task run.
    TaskSpec:
      properties:
        output_schema:
          anyOf:
            - $ref: '#/components/schemas/JsonSchema'
            - $ref: '#/components/schemas/TextSchema'
            - $ref: '#/components/schemas/AutoSchema'
            - type: string
          title: Output Schema
          description: >-
            JSON schema or text fully describing the desired output from the
            task. Descriptions of output fields will determine the form and
            content of the response. A bare string is equivalent to a text
            schema with the same description.
        input_schema:
          anyOf:
            - type: string
            - $ref: '#/components/schemas/JsonSchema'
            - $ref: '#/components/schemas/TextSchema'
            - type: 'null'
          title: Input Schema
          description: >-
            Optional JSON schema or text description of expected input to the
            task. A bare string is equivalent to a text schema with the same
            description.
      type: object
      required:
        - output_schema
      title: TaskSpec
      description: >-
        Specification for a task.


        Auto output schemas can be specified by setting
        `output_schema={"type":"auto"}`. Not

        specifying a TaskSpec is the same as setting an auto output schema.


        For convenience bare strings are also accepted as input or output
        schemas.
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
    TextSchema:
      properties:
        description:
          anyOf:
            - type: string
            - type: 'null'
          title: Description
          description: A text description of the desired output from the task.
          examples:
            - GDP in USD for the year, formatted like '$3.1 trillion (2023)'
        type:
          type: string
          const: text
          title: Type
          description: The type of schema being defined. Always `text`.
          default: text
      type: object
      title: TextSchema
      description: Text description for a task input or output.
    AutoSchema:
      properties:
        type:
          type: string
          const: auto
          title: Type
          description: The type of schema being defined. Always `auto`.
          default: auto
      type: object
      title: AutoSchema
      description: Auto schema for a task input or output.
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