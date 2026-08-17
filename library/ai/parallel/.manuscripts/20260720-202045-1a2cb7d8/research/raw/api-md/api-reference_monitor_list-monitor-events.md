> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# List Monitor Events

> List events for a monitor, newest first.

Pass `event_group_id` to narrow results to a single execution. Otherwise
returns all executions newest-first; use `next_cursor` to paginate.
Set `include_completions=true` to also include no-change executions.



## OpenAPI

````yaml /public-openapi.json get /v1/monitors/{monitor_id}/events
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
  /v1/monitors/{monitor_id}/events:
    get:
      tags:
        - Monitor
      summary: List Monitor Events
      description: |-
        List events for a monitor, newest first.

        Pass `event_group_id` to narrow results to a single execution. Otherwise
        returns all executions newest-first; use `next_cursor` to paginate.
        Set `include_completions=true` to also include no-change executions.
      operationId: list_monitor_events_v1_monitors__monitor_id__events_get
      parameters:
        - name: monitor_id
          in: path
          required: true
          schema:
            type: string
            title: Monitor Id
        - name: event_group_id
          in: query
          required: false
          schema:
            anyOf:
              - type: string
              - type: 'null'
            description: >-
              Filter to a single execution. Values come from `event_group_id` in
              webhook events and listed events. Pagination params are ignored
              when set.
            title: Event Group Id
          description: >-
            Filter to a single execution. Values come from `event_group_id` in
            webhook events and listed events. Pagination params are ignored when
            set.
        - name: cursor
          in: query
          required: false
          schema:
            anyOf:
              - type: string
              - type: 'null'
            description: >-
              Pass `next_cursor` from a previous response to retrieve more
              events.
            title: Cursor
          description: Pass `next_cursor` from a previous response to retrieve more events.
        - name: limit
          in: query
          required: false
          schema:
            anyOf:
              - type: integer
                maximum: 100
                minimum: 1
              - type: 'null'
            description: >-
              Maximum number of events to return. Defaults to 20. Between 1 and
              100.
            title: Limit
          description: >-
            Maximum number of events to return. Defaults to 20. Between 1 and
            100.
        - name: include_completions
          in: query
          required: false
          schema:
            type: boolean
            description: >-
              When true, include completion events for executions that ran but
              detected no material changes. Useful for auditing execution
              history.
            default: false
            title: Include Completions
          description: >-
            When true, include completion events for executions that ran but
            detected no material changes. Useful for auditing execution history.
      responses:
        '200':
          description: Monitor events retrieved successfully.
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/PaginatedMonitorEvents'
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
          description: Monitor not found
          content:
            application/json:
              example:
                type: error
                error:
                  ref_id: fcb2b4f3-c75e-4186-87bc-caa1a8381331
                  message: Monitor not found
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '422':
          description: 'Unprocessable content: invalid cursor or request validation error'
          content:
            application/json:
              example:
                type: error
                error:
                  ref_id: fcb2b4f3-c75e-4186-87bc-caa1a8381331
                  message: >-
                    Unprocessable content: invalid cursor or request validation
                    error
              schema:
                $ref: '#/components/schemas/ErrorResponse'
      x-code-samples:
        - lang: Python
          source: |-
            from parallel import Parallel

            client = Parallel()

            page = client.monitor.events("monitor_id", limit=20)
            for event in page.events:
                print(event)
        - lang: TypeScript
          source: >-
            import Parallel from "parallel-web";


            const client = new Parallel();


            const page = await client.monitor.events('monitor_id', { limit: 20
            });

            for (const event of page.events) {
                console.log(event);
            }
components:
  schemas:
    PaginatedMonitorEvents:
      properties:
        events:
          items:
            oneOf:
              - $ref: '#/components/schemas/MonitorEventStreamEvent'
              - $ref: '#/components/schemas/MonitorSnapshotEvent'
              - $ref: '#/components/schemas/MonitorCompletionEvent'
              - $ref: '#/components/schemas/MonitorErrorEvent'
            discriminator:
              propertyName: event_type
              mapping:
                completion:
                  $ref: '#/components/schemas/MonitorCompletionEvent'
                error:
                  $ref: '#/components/schemas/MonitorErrorEvent'
                event_stream:
                  $ref: '#/components/schemas/MonitorEventStreamEvent'
                snapshot:
                  $ref: '#/components/schemas/MonitorSnapshotEvent'
          type: array
          title: Events
          description: Monitor events returned by this request, ordered newest first.
        next_cursor:
          anyOf:
            - type: string
            - type: 'null'
          title: Next Cursor
          description: >-
            Pass as `cursor` to retrieve more events. Absent when there are no
            more events.
        warnings:
          anyOf:
            - items:
                $ref: '#/components/schemas/Warning'
              type: array
            - type: 'null'
          title: Warnings
          description: Execution caveats for this page of events, e.g. compute limits.
      type: object
      required:
        - events
      title: PaginatedMonitorEvents
      description: Paginated list of monitor events, newest first.
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
    MonitorEventStreamEvent:
      properties:
        event_id:
          type: string
          title: Event Id
          description: >-
            Stable identifier for this event. Safe to use for client-side
            deduplication across pagination and retries.
        event_group_id:
          type: string
          title: Event Group Id
          description: ID of the event group that owns this event.
        event_date:
          anyOf:
            - type: string
            - type: 'null'
          title: Event Date
          description: >-
            Date when this event was produced. ISO 8601 date (YYYY-MM-DD) or
            partial (YYYY-MM or YYYY).
          examples:
            - '2026-04-07'
        event_type:
          type: string
          enum:
            - event_stream
          const: event_stream
          title: Event Type
          description: Discriminant for the event_stream event variant.
          default: event_stream
        output:
          oneOf:
            - $ref: '#/components/schemas/TaskRunTextOutput'
            - $ref: '#/components/schemas/TaskRunJsonOutput'
          title: Output
          description: Text or JSON output describing the detected change.
          discriminator:
            propertyName: type
            mapping:
              json:
                $ref: '#/components/schemas/TaskRunJsonOutput'
              text:
                $ref: '#/components/schemas/TaskRunTextOutput'
      type: object
      required:
        - event_id
        - event_group_id
        - event_date
        - output
      title: MonitorEventStreamEvent
      description: |-
        Append-only event from an event_stream monitor.

        Each event represents a distinct material change detected since the
        previous execution. Events are net-new relative to the cursor; clients
        should treat them as an append-only log.
    MonitorSnapshotEvent:
      properties:
        event_id:
          type: string
          title: Event Id
          description: >-
            Stable identifier for this event. Safe to use for client-side
            deduplication across pagination and retries.
        event_group_id:
          type: string
          title: Event Group Id
          description: ID of the event group that owns this event.
        event_date:
          anyOf:
            - type: string
            - type: 'null'
          title: Event Date
          description: >-
            Date when this event was produced. ISO 8601 date (YYYY-MM-DD) or
            partial (YYYY-MM or YYYY).
          examples:
            - '2026-04-07'
        event_type:
          type: string
          enum:
            - snapshot
          const: snapshot
          title: Event Type
          description: Discriminant for the snapshot event variant.
          default: snapshot
        changed_output:
          oneOf:
            - $ref: '#/components/schemas/TaskRunTextOutput'
            - $ref: '#/components/schemas/TaskRunJsonOutput'
          title: Changed Output
          description: >-
            Partial output containing only the fields that changed since the
            previous execution, each with its `basis` (reasoning and citations).
          discriminator:
            propertyName: type
            mapping:
              json:
                $ref: '#/components/schemas/TaskRunJsonOutput'
              text:
                $ref: '#/components/schemas/TaskRunTextOutput'
        previous_output:
          oneOf:
            - $ref: '#/components/schemas/TaskRunTextOutput'
            - $ref: '#/components/schemas/TaskRunJsonOutput'
          title: Previous Output
          description: The full output from the prior run, including all fields and basis.
          discriminator:
            propertyName: type
            mapping:
              json:
                $ref: '#/components/schemas/TaskRunJsonOutput'
              text:
                $ref: '#/components/schemas/TaskRunTextOutput'
      type: object
      required:
        - event_id
        - event_group_id
        - event_date
        - changed_output
        - previous_output
      title: MonitorSnapshotEvent
      description: >-
        Snapshot diff event emitted when a monitored task run's output changes.


        `changed_output` contains only the fields that changed since the
        previous execution,

        along with their `basis` (reasoning + citations). `previous_output`
        holds

        the complete output from the prior run for comparison.
    MonitorCompletionEvent:
      properties:
        event_type:
          type: string
          enum:
            - completion
          const: completion
          title: Event Type
          description: Discriminant for the completion event variant.
          default: completion
        timestamp:
          type: string
          format: date-time
          title: Timestamp
          description: >-
            Timestamp of when the monitor execution completed, as an RFC 3339
            string.
          examples:
            - '2025-01-15T10:30:00Z'
      type: object
      required:
        - timestamp
      title: MonitorCompletionEvent
      description: >-
        Emitted when a monitor execution ran but detected no material changes.


        Only returned when `include_completions=true` is passed to the list
        events

        endpoint. Useful for auditing execution history alongside content
        events.
    MonitorErrorEvent:
      properties:
        event_type:
          type: string
          enum:
            - error
          const: error
          title: Event Type
          description: Discriminant for the error event variant.
          default: error
        error_message:
          type: string
          title: Error Message
          description: Human-readable description of the failure.
        timestamp:
          type: string
          format: date-time
          title: Timestamp
          description: >-
            Timestamp of when the monitor execution failed, as an RFC 3339
            string.
          examples:
            - '2025-01-15T10:30:00Z'
      type: object
      required:
        - error_message
        - timestamp
      title: MonitorErrorEvent
      description: |-
        Emitted when a monitor execution failed (e.g. payment or quota error).

        Always included in the events list regardless of `include_completions`.
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