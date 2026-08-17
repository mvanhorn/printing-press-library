> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Add Runs to Task Group

> Initiates multiple task runs within a TaskGroup.



## OpenAPI

````yaml /public-openapi.json post /v1/tasks/groups/{taskgroup_id}/runs
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
  /v1/tasks/groups/{taskgroup_id}/runs:
    post:
      tags:
        - Tasks
      summary: Add Runs to Task Group
      description: Initiates multiple task runs within a TaskGroup.
      operationId: tasks_taskgroups_runs_post_v1_tasks_groups__taskgroup_id__runs_post
      parameters:
        - name: taskgroup_id
          in: path
          required: true
          schema:
            type: string
            title: Taskgroup Id
        - name: refresh_status
          in: query
          required: false
          schema:
            type: boolean
            default: true
            title: Refresh Status
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
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/TaskGroupRunRequest'
      responses:
        '200':
          description: Successful Response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/TaskGroupRunResponse'
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
            from parallel.types import McpServerParam
            from parallel.types.run_input_param import RunInputParam

            client = Parallel(api_key="API Key")
            group_status = client.task_group.add_runs(
                "taskgroup_id",
                inputs=[
                    RunInputParam(
                        input="What was the GDP of France in 2023?",
                        processor="base",
                        enable_events=True,
                        mcp_servers=[McpServerParam(
                            type="url",
                            name="parallel_web_search",
                            url="https://mcp.parallel.ai/v1beta/search_mcp",
                            headers={"x-api-key": "API Key"}
                        )]
                    )
                ]
            )
            print(group_status.status)
        - lang: TypeScript
          source: |-
            import Parallel from "parallel-web";

            const client = new Parallel();

            const groupStatus = await client.taskGroup.addRuns(
                'taskgroup_id',
                {
                    inputs: [
                        {
                            input: 'What was the GDP of France in 2023?',
                            processor: 'base',
                            enable_events: true,
                            mcp_servers: [{
                                type: 'url',
                                name: 'parallel_web_search',
                                url: 'https://mcp.parallel.ai/v1beta/search_mcp',
                                headers: {'x-api-key': 'API Key'}
                            }]
                        }
                    ]
                }
            );
            console.log(groupStatus.status);
components:
  schemas:
    TaskGroupRunRequest:
      properties:
        default_task_spec:
          anyOf:
            - $ref: '#/components/schemas/TaskSpec'
            - type: 'null'
          description: >-
            Default task spec to use for the runs. If task_spec is specified in
            a run, it overrides this default.
        inputs:
          items:
            $ref: '#/components/schemas/TaskRunInput'
          type: array
          title: Inputs
          description: >-
            List of task runs to execute. Up to 1,000 runs can be specified per
            request. If you'd like to add more runs, split them across multiple
            TaskGroup POST requests.
      type: object
      required:
        - inputs
      title: TaskGroupRunRequest
      description: Request to initiate new task runs in a task group.
    TaskGroupRunResponse:
      properties:
        status:
          $ref: '#/components/schemas/TaskGroupStatus'
          description: Status of the group.
        run_ids:
          items:
            type: string
          type: array
          title: Run IDs
          description: IDs of the newly created runs.
        run_cursor:
          anyOf:
            - type: string
            - type: 'null'
          title: Run Cursor
          description: >-
            Cursor for these runs in the run stream at
            taskgroup/runs?last_event_id=<run_cursor>. Empty for the first runs
            in the group.
        event_cursor:
          anyOf:
            - type: string
            - type: 'null'
          title: Event Cursor
          description: >-
            Cursor for these runs in the event stream at
            taskgroup/events?last_event_id=<event_cursor>. Empty for the first
            runs in the group.
      type: object
      required:
        - status
        - run_ids
        - run_cursor
        - event_cursor
      title: TaskGroupRunResponse
      description: Response from adding new task runs to a task group.
    HTTPValidationError:
      properties:
        detail:
          items:
            $ref: '#/components/schemas/ValidationError'
          type: array
          title: Detail
      type: object
      title: HTTPValidationError
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
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: x-api-key

````