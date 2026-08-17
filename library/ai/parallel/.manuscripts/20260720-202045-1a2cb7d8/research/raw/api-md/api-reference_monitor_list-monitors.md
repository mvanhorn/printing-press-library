> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# List Monitors

> List monitors ordered by creation time, newest first.

Monitors are sorted by `created_at` descending. `limit` defaults to 100.
Use `next_cursor` from the response and pass it as `cursor` to fetch the
next page. Pagination ends when `next_cursor` is absent.

By default only `active` monitors are returned. Pass `status=cancelled`
or both values to include cancelled monitors.

The legacy Monitor API (`/v1alpha/monitors` endpoints) is documented under
the `Monitor (Alpha)` tag.



## OpenAPI

````yaml /public-openapi.json get /v1/monitors
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
  /v1/monitors:
    get:
      tags:
        - Monitor
      summary: List Monitors
      description: >-
        List monitors ordered by creation time, newest first.


        Monitors are sorted by `created_at` descending. `limit` defaults to 100.

        Use `next_cursor` from the response and pass it as `cursor` to fetch the

        next page. Pagination ends when `next_cursor` is absent.


        By default only `active` monitors are returned. Pass `status=cancelled`

        or both values to include cancelled monitors.


        The legacy Monitor API (`/v1alpha/monitors` endpoints) is documented
        under

        the `Monitor (Alpha)` tag.
      operationId: list_monitors_v1_monitors_get
      parameters:
        - name: cursor
          in: query
          required: false
          schema:
            anyOf:
              - type: string
              - type: 'null'
            description: >-
              Pagination token from `next_cursor` in a previous response. Omit
              to start from the most recently created monitor.
            title: Cursor
          description: >-
            Pagination token from `next_cursor` in a previous response. Omit to
            start from the most recently created monitor.
        - name: limit
          in: query
          required: false
          schema:
            anyOf:
              - type: integer
                maximum: 10000
                minimum: 1
              - type: 'null'
            description: >-
              Maximum number of monitors to return. Defaults to 100. Between 1
              and 10000.
            title: Limit
          description: >-
            Maximum number of monitors to return. Defaults to 100. Between 1 and
            10000.
        - name: type
          in: query
          required: false
          schema:
            anyOf:
              - type: array
                items:
                  type: string
                  enum:
                    - event_stream
                    - snapshot
              - type: 'null'
            description: >-
              Filter by monitor type. Pass multiple times to filter by multiple
              values. Omit to return all types.
            title: Type
          description: >-
            Filter by monitor type. Pass multiple times to filter by multiple
            values. Omit to return all types.
        - name: status
          in: query
          required: false
          schema:
            anyOf:
              - type: array
                items:
                  enum:
                    - active
                    - cancelled
                  type: string
              - type: 'null'
            description: >-
              Filter by monitor status. Pass multiple times to filter by
              multiple values. Defaults to `active` only.
            title: Status
          description: >-
            Filter by monitor status. Pass multiple times to filter by multiple
            values. Defaults to `active` only.
      responses:
        '200':
          description: Paginated list of monitors.
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/PaginatedMonitorResponse'
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

            page = client.monitor.list(limit=10)
            for monitor in page.monitors:
                print(monitor.monitor_id, monitor.status)
        - lang: TypeScript
          source: |-
            import Parallel from "parallel-web";

            const client = new Parallel();

            const page = await client.monitor.list({ limit: 10 });
            for (const monitor of page.monitors) {
                console.log(monitor.monitor_id, monitor.status);
            }
components:
  schemas:
    PaginatedMonitorResponse:
      properties:
        monitors:
          items:
            $ref: '#/components/schemas/MonitorResponse'
          type: array
          title: Monitors
          description: List of monitors for the current page.
        next_cursor:
          anyOf:
            - type: string
            - type: 'null'
          title: Next Cursor
          description: >-
            Opaque pagination token. Pass as `cursor` to retrieve the next page.
            Absent when there are no more pages.
      type: object
      required:
        - monitors
      title: PaginatedMonitorResponse
      description: Paginated list of monitors.
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
    MonitorResponse:
      properties:
        type:
          type: string
          enum:
            - event_stream
            - snapshot
          title: Type
          description: The type of monitor.
          examples:
            - event_stream
            - snapshot
        monitor_id:
          type: string
          title: Monitor ID
          description: ID of the monitor.
        status:
          type: string
          enum:
            - active
            - cancelled
          title: Status
          description: Status of the monitor.
          examples:
            - active
            - cancelled
        frequency:
          type: string
          title: Frequency
          description: >-
            Frequency of the monitor. Format: '<number><unit>' where unit is 'h'
            (hours), 'd' (days), or 'w' (weeks). Must be between 1h and 30d
            (inclusive).
          examples:
            - 1h
            - 12h
            - 1d
            - 7d
            - 30d
        processor:
          type: string
          enum:
            - lite
            - base
          title: Processor
          description: >-
            Processor to use for the monitor. `lite` is faster and cheaper;
            `base` performs more thorough analysis at higher cost and latency.
            Defaults to `lite`.
          examples:
            - lite
            - base
        webhook:
          anyOf:
            - $ref: '#/components/schemas/MonitorWebhook'
            - type: 'null'
          description: Webhook configuration for the monitor.
        metadata:
          anyOf:
            - additionalProperties:
                type: string
              type: object
            - type: 'null'
          title: Metadata
          description: >-
            User-provided metadata stored with the monitor and echoed back in
            webhook notifications and GET responses, so you can map events to
            objects in your application. Keys: max 16 chars; values: max 512
            chars.
          examples:
            - slack_thread_id: '1234567890.123456'
              user_id: U123ABC
        created_at:
          type: string
          format: date-time
          title: Created At
          description: Timestamp of the creation of the monitor, as an RFC 3339 string.
          examples:
            - '2025-01-15T10:30:00Z'
        last_run_at:
          anyOf:
            - type: string
            - type: 'null'
          title: Last Run At
          description: Timestamp of the last run for the monitor, as an RFC 3339 string.
          examples:
            - '2025-01-15T10:30:00Z'
        settings:
          anyOf:
            - $ref: '#/components/schemas/MonitorEventStreamResponseSettings'
            - $ref: '#/components/schemas/MonitorSnapshotResponseSettings'
          title: Settings
          description: >-
            Type-specific configuration. Shape is determined by `type`:
            `MonitorEventStreamResponseSettings` for `event_stream`,
            `MonitorSnapshotResponseSettings` for `snapshot`.
        output:
          anyOf:
            - $ref: '#/components/schemas/MonitorSnapshotOutput'
            - type: 'null'
          description: >-
            Runtime output state. Present only for `snapshot` monitors; `null`
            for `event_stream` monitors.
      type: object
      required:
        - type
        - monitor_id
        - status
        - frequency
        - processor
        - created_at
        - settings
      title: MonitorResponse
      description: >-
        Response object for a monitor.


        The `type` field at the root determines the concrete shape of
        `settings`:

        `event_stream` uses `MonitorEventStreamResponseSettings`, and `snapshot`

        uses `MonitorSnapshotResponseSettings`. Snapshot monitors also carry an

        `output` field (`MonitorSnapshotOutput`) with the latest computed state.
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
    MonitorWebhook:
      properties:
        url:
          type: string
          title: Url
          description: URL for the webhook.
          examples:
            - https://example.com/webhook
        event_types:
          items:
            type: string
            enum:
              - monitor.event.detected
              - monitor.execution.completed
              - monitor.execution.failed
          type: array
          title: Event Types
          description: Event types to send the webhook notifications for.
      type: object
      required:
        - url
      title: MonitorWebhook
      description: Webhook configuration for a monitor.
    MonitorEventStreamResponseSettings:
      properties:
        query:
          type: string
          title: Query
          description: The search query being monitored.
          examples:
            - Extract recent news about AI
        output_schema:
          anyOf:
            - $ref: '#/components/schemas/JsonSchema'
            - type: 'null'
          description: >-
            JSON schema that constrains and structures the event output. When
            set, events are returned as JSON objects matching this schema.
        include_backfill:
          anyOf:
            - type: boolean
            - type: 'null'
          title: Include Backfill
          description: >-
            If true, the first execution returns a sample of recent historical
            events matching the query (preview only — not exhaustive). If false
            or omitted, only events from the monitor's creation date onward are
            returned. Subsequent executions are always incremental.
        advanced_settings:
          anyOf:
            - $ref: '#/components/schemas/AdvancedMonitorSettings'
            - type: 'null'
          description: Advanced monitor configuration.
      type: object
      required:
        - query
      title: MonitorEventStreamResponseSettings
      description: Type-specific response fields for an `event_stream` monitor.
    MonitorSnapshotResponseSettings:
      properties:
        task_run_id:
          type: string
          title: Task Run Id
          description: ID of the task run used as the monitoring baseline.
        query:
          type: string
          title: Query
          description: >-
            The original task input from the baseline task run that this monitor
            tracks.
          examples:
            - Extract recent news about AI
        output_schema:
          anyOf:
            - $ref: '#/components/schemas/JsonSchema'
            - type: 'null'
          description: >-
            JSON schema derived from the baseline task run that constrains and
            structures the event output.
      type: object
      required:
        - task_run_id
        - query
      title: MonitorSnapshotResponseSettings
      description: Configuration settings for a `snapshot` monitor.
    MonitorSnapshotOutput:
      properties:
        latest_snapshot:
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
          title: Latest Snapshot
          description: >-
            Task run output from the most recent completed execution of this
            snapshot monitor — same structure as the output of the original task
            run the monitor was created from. `null` until the first run
            completes.
      type: object
      title: MonitorSnapshotOutput
      description: Runtime output state for a `snapshot` monitor.
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
    AdvancedMonitorSettings:
      properties:
        source_policy:
          anyOf:
            - $ref: '#/components/schemas/SourcePolicy'
            - type: 'null'
          description: >-
            Domain filtering preferences: preferred and disallowed domains for
            monitor search results.
          examples:
            - exclude_domains:
                - reddit.com
                - x.com
                - .ai
              include_domains:
                - wikipedia.org
                - usa.gov
                - .edu
        location:
          anyOf:
            - type: string
            - type: 'null'
          title: Location
          description: ISO 3166-1 alpha-2 country code for geo-targeted monitor results.
          examples:
            - us
            - gb
            - de
            - jp
      type: object
      title: AdvancedMonitorSettings
      description: Advanced monitor configuration.
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