---
name: pp-elevenlabs
description: "Printing Press CLI for Elevenlabs. This is the documentation for the ElevenLabs API. You can use this API to use our service programmatically, this is..."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - elevenlabs-pp-cli
    install:
      - kind: go
        bins: [elevenlabs-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/elevenlabs/cmd/elevenlabs-pp-cli
---

# Elevenlabs — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `elevenlabs-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install elevenlabs --cli-only
   ```
2. Verify: `elevenlabs-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/elevenlabs/cmd/elevenlabs-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

This is the documentation for the ElevenLabs API. You can use this API to use our service programmatically, this is done by using your API key. You can find your API key in the dashboard at https://elevenlabs.io/app/settings/api-keys.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent audio planning
- **`voice discover`** — Search owned and shared ElevenLabs voices through one compact, JSON-first command.

  _Agents can pick a voice before generation without paging through incompatible raw endpoints._

  ```bash
  elevenlabs-pp-cli voice discover --source all --limit 5 --agent
  ```
- **`tts resolve`** — Resolve a voice, model, output format, default settings, and subscription context before rendering.

  _Agents can validate generation choices cheaply before spending quota on audio._

  ```bash
  elevenlabs-pp-cli tts resolve --voice JBFqnCBsd6RMkjVDRZzb --model auto --agent
  ```

### Audio artifact workflows
- **`tts render`** — Render text to an audio file and print a structured manifest with file path, byte count, model, and voice.

  _Agents get a durable audio artifact plus machine-readable metadata in one step._

  ```bash
  elevenlabs-pp-cli tts render --voice JBFqnCBsd6RMkjVDRZzb --text 'Hello from ElevenLabs.' --out hello.mp3 --agent
  ```
- **`dialogue cast`** — Render speaker-labelled dialogue lines by resolving speaker-to-voice assignments and saving audio.

  _Agents can generate multi-speaker dialogue from scripts without manually constructing JSON voice-id payloads._

  ```bash
  elevenlabs-pp-cli dialogue cast --line narrator='Welcome.' --cast narrator=JBFqnCBsd6RMkjVDRZzb --out dialogue.mp3 --agent
  ```

## Command Reference

**audio-isolation** — Manage audio isolation

- `elevenlabs-pp-cli audio-isolation audio_isolation` — Removes background noise from audio
- `elevenlabs-pp-cli audio-isolation delete-history-item` — Deletes a specific audio isolation history item and the associated media files.
- `elevenlabs-pp-cli audio-isolation get-history` — Returns a list of all your audio isolation generations.
- `elevenlabs-pp-cli audio-isolation stream` — Removes background noise from audio and streams the result

**audio-native** — Manage audio native

- `elevenlabs-pp-cli audio-native create-project` — Creates Audio Native enabled project, optionally starts conversion and returns project ID and embeddable HTML snippet.
- `elevenlabs-pp-cli audio-native update-content-from-url` — Finds an AudioNative project matching the provided URL, extracts content from the URL, updates the project content,...

**convai** — Manage convai

- `elevenlabs-pp-cli convai add-documentation-to-knowledge-base` — Uploads a file or reference a webpage to use as part of the shared knowledge base
- `elevenlabs-pp-cli convai add-mcp-server-tool-approval-route` — Add approval for a specific MCP tool when using per-tool approval mode.
- `elevenlabs-pp-cli convai add-mcp-tool-config-override-route` — Create configuration overrides for a specific MCP tool.
- `elevenlabs-pp-cli convai add-tool-route` — Add a new tool to the available tools in the workspace.
- `elevenlabs-pp-cli convai agent-testing-bulk-move-route` — Moves multiple tests or folders from one folder to another.
- `elevenlabs-pp-cli convai assign-conversation-tags-route` — Assign one or more conversation tags to a conversation. Tags that are already assigned are ignored. Tags must belong...
- `elevenlabs-pp-cli convai cancel-batch-call` — Cancel a running batch call and set all recipients to cancelled status.
- `elevenlabs-pp-cli convai cancel-file-upload-route` — Remove a file upload from a conversation. Only possible if the file hasn't already been used in the conversation.
- `elevenlabs-pp-cli convai create-agent-deployment-route` — Create a new deployment for an agent
- `elevenlabs-pp-cli convai create-agent-draft-route` — Create a new draft for an agent
- `elevenlabs-pp-cli convai create-agent-response-test-route` — Creates a new agent response test.
- `elevenlabs-pp-cli convai create-agent-route` — Create an agent from a config object
- `elevenlabs-pp-cli convai create-agent-test-folder-route` — Creates a folder for organizing agent tests.
- `elevenlabs-pp-cli convai create-batch-call` — Submit a batch call request to schedule calls for multiple recipients.
- `elevenlabs-pp-cli convai create-branch-route` — Create a new branch from a given version of any branch
- `elevenlabs-pp-cli convai create-conversation-tag-route` — Create a new conversation tag for the workspace.
- `elevenlabs-pp-cli convai create-environment-variable` — Create a new environment variable for the workspace
- `elevenlabs-pp-cli convai create-file-document-route` — Create a knowledge base document generated form the uploaded file.
- `elevenlabs-pp-cli convai create-folder-route` — Create a folder used for grouping documents together.
- `elevenlabs-pp-cli convai create-mcp-server-route` — Create a new MCP server configuration in the workspace.
- `elevenlabs-pp-cli convai create-phone-number-route` — Import Phone Number from provider configuration (Twilio or SIP trunk)
- `elevenlabs-pp-cli convai create-secret-route` — Create a new secret for the workspace
- `elevenlabs-pp-cli convai create-text-document-route` — Create a knowledge base document containing the provided text.
- `elevenlabs-pp-cli convai create-url-document-route` — Create a knowledge base document generated by scraping the given webpage.
- `elevenlabs-pp-cli convai delete-agent-draft-route` — Delete a draft for an agent
- `elevenlabs-pp-cli convai delete-agent-route` — Delete an agent
- `elevenlabs-pp-cli convai delete-agent-test-folder-route` — Deletes an agent test folder by ID. Use force=true to delete a non-empty folder and all its contents.
- `elevenlabs-pp-cli convai delete-batch-call` — Permanently delete a batch call and all recipient records. Conversations remain in history.
- `elevenlabs-pp-cli convai delete-chat-response-test-route` — Deletes an agent response test by ID.
- `elevenlabs-pp-cli convai delete-conversation-route` — Delete a particular conversation
- `elevenlabs-pp-cli convai delete-conversation-tag-route` — Delete a conversation tag. Restricted to the tag owner or a workspace admin.
- `elevenlabs-pp-cli convai delete-knowledge-base-document` — Delete a document or folder from the knowledge base.
- `elevenlabs-pp-cli convai delete-mcp-server-route` — Delete a specific MCP server configuration from the workspace.
- `elevenlabs-pp-cli convai delete-phone-number-route` — Delete Phone Number by ID
- `elevenlabs-pp-cli convai delete-rag-index` — Delete RAG index for the knowledgebase document.
- `elevenlabs-pp-cli convai delete-secret-route` — Delete a workspace secret if it's not in use
- `elevenlabs-pp-cli convai delete-tool-route` — Delete tool from the workspace.
- `elevenlabs-pp-cli convai delete-whatsapp-account` — Delete a WhatsApp account
- `elevenlabs-pp-cli convai duplicate-agent-route` — Create a new agent by duplicating an existing one
- `elevenlabs-pp-cli convai get-agent-knowledge-base-size` — Returns the number of pages in the agent's knowledge base.
- `elevenlabs-pp-cli convai get-agent-knowledge-base-summaries-route` — Gets multiple knowledge base document summaries by their IDs.
- `elevenlabs-pp-cli convai get-agent-link-route` — Get the current link used to share the agent with others
- `elevenlabs-pp-cli convai get-agent-llm-expected-cost-calculation` — Calculates expected number of LLM tokens needed for the specified agent.
- `elevenlabs-pp-cli convai get-agent-response-test-route` — Gets an agent response test by ID.
- `elevenlabs-pp-cli convai get-agent-response-tests-summaries-route` — Gets multiple agent response tests by their IDs. Returns a dictionary mapping test IDs to test summaries.
- `elevenlabs-pp-cli convai get-agent-route` — Retrieve config for an agent
- `elevenlabs-pp-cli convai get-agent-summaries-route` — Returns summaries for the specified agents.
- `elevenlabs-pp-cli convai get-agent-test-folder-route` — Gets an agent test folder by ID, including its folder path.
- `elevenlabs-pp-cli convai get-agent-topics-route` — Returns the latest topic discovery run results for a given agent.
- `elevenlabs-pp-cli convai get-agent-widget-route` — Retrieve the widget configuration for an agent
- `elevenlabs-pp-cli convai get-agents-route` — Returns a list of your agents and their metadata.
- `elevenlabs-pp-cli convai get-api-integration-docs` — Get MDX documentation for all integrations that have it. Returns docs for publicly available integrations when...
- `elevenlabs-pp-cli convai get-batch-call` — Get detailed information about a batch call including all recipients.
- `elevenlabs-pp-cli convai get-branch-route` — Get information about a single agent branch
- `elevenlabs-pp-cli convai get-branches-route` — Returns a list of branches an agent has
- `elevenlabs-pp-cli convai get-conversation-audio-route` — Get the audio recording of a particular conversation
- `elevenlabs-pp-cli convai get-conversation-histories-route` — Get all conversations of agents that user owns. With option to restrict to a specific agent.
- `elevenlabs-pp-cli convai get-conversation-history-route` — Get the details of a particular conversation
- `elevenlabs-pp-cli convai get-conversation-signed-link` — Get a signed url to start a conversation with an agent with an agent that requires authorization
- `elevenlabs-pp-cli convai get-conversation-sip-messages` — Get SIP messages associated with a conversation's phone call
- `elevenlabs-pp-cli convai get-conversation-tag-route` — Get a conversation tag by ID.
- `elevenlabs-pp-cli convai get-conversation-users-route` — Get distinct users from conversations with pagination.
- `elevenlabs-pp-cli convai get-dashboard-settings-route` — Retrieve Convai dashboard settings for the workspace
- `elevenlabs-pp-cli convai get-documentation-chunk-from-knowledge-base` — Get details about a specific documentation part used by RAG.
- `elevenlabs-pp-cli convai get-documentation-chunks-from-knowledge-base` — Get all RAG chunks for a specific knowledge base document.
- `elevenlabs-pp-cli convai get-documentation-from-knowledge-base` — Get details about a specific documentation making up the agent's knowledge base
- `elevenlabs-pp-cli convai get-environment-variable` — Get a specific environment variable by ID
- `elevenlabs-pp-cli convai get-knowledge-base-content` — Get the entire content of a document from the knowledge base
- `elevenlabs-pp-cli convai get-knowledge-base-dependent-agents` — Get a list of agents depending on this knowledge base document
- `elevenlabs-pp-cli convai get-knowledge-base-list-route` — Get a list of available knowledge base documents
- `elevenlabs-pp-cli convai get-knowledge-base-source-file-url` — Get a signed URL to download the original source file of a file-type document from the knowledge base
- `elevenlabs-pp-cli convai get-live-count` — Get the live count of the ongoing conversations.
- `elevenlabs-pp-cli convai get-livekit-token` — Get a WebRTC session token for real-time communication.
- `elevenlabs-pp-cli convai get-mcp-route` — Retrieve a specific MCP server configuration from the workspace.
- `elevenlabs-pp-cli convai get-mcp-tool-config-override-route` — Retrieve configuration overrides for a specific MCP tool.
- `elevenlabs-pp-cli convai get-or-create-rag-indexes` — Retrieves and/or creates RAG indexes for multiple knowledge base documents in a single request. Maximum 100 items...
- `elevenlabs-pp-cli convai get-phone-number-route` — Retrieve Phone Number details by ID
- `elevenlabs-pp-cli convai get-public-llm-expected-cost-calculation` — Returns a list of LLM models and the expected cost for using them based on the provided values.
- `elevenlabs-pp-cli convai get-rag-index-overview` — Provides total size and other information of RAG indexes used by knowledgebase documents
- `elevenlabs-pp-cli convai get-rag-indexes` — Provides information about all RAG indexes of the specified knowledgebase document.
- `elevenlabs-pp-cli convai get-secret-dependencies-route` — Get paginated list of resources that depend on a specific secret, filtered by resource type.
- `elevenlabs-pp-cli convai get-secret-route` — Get a workspace secret by ID
- `elevenlabs-pp-cli convai get-secrets-route` — Get all workspace secrets for the user
- `elevenlabs-pp-cli convai get-settings-route` — Retrieve Convai settings for the workspace
- `elevenlabs-pp-cli convai get-signed-url-deprecated` — Get a signed url to start a conversation with an agent with an agent that requires authorization
- `elevenlabs-pp-cli convai get-test-invocation-route` — Gets a test invocation by ID.
- `elevenlabs-pp-cli convai get-tool-dependent-agents-route` — Get a list of agents depending on this tool
- `elevenlabs-pp-cli convai get-tool-executions-route` — Get paginated list of tool executions for a specific tool.
- `elevenlabs-pp-cli convai get-tool-route` — Get tool that is available in the workspace.
- `elevenlabs-pp-cli convai get-tools-route` — Get all available tools in the workspace.
- `elevenlabs-pp-cli convai get-version-metadata-route` — Get metadata for a specific agent version
- `elevenlabs-pp-cli convai get-whatsapp-account` — Get a WhatsApp account
- `elevenlabs-pp-cli convai get-workspace-batch-calls` — Get all batch calls for the current workspace.
- `elevenlabs-pp-cli convai handle-sip-trunk-outbound-call` — Handle an outbound call via SIP trunk
- `elevenlabs-pp-cli convai handle-twilio-outbound-call` — Handle an outbound call via Twilio
- `elevenlabs-pp-cli convai list-available-llms` — Returns a list of available LLM models that can be used with agents, including their capabilities and any...
- `elevenlabs-pp-cli convai list-chat-response-tests-route` — Lists all agent response tests with pagination support and optional search filtering.
- `elevenlabs-pp-cli convai list-conversation-tags-route` — List conversation tags for the workspace, ordered by most recently created first.
- `elevenlabs-pp-cli convai list-environment-variables` — List all environment variables for the workspace with optional filtering
- `elevenlabs-pp-cli convai list-mcp-server-tools-route` — Retrieve all tools available for a specific MCP server configuration.
- `elevenlabs-pp-cli convai list-mcp-servers-route` — Retrieve all MCP server configurations available in the workspace.
- `elevenlabs-pp-cli convai list-phone-numbers-route` — Retrieve all Phone Numbers
- `elevenlabs-pp-cli convai list-sip-messages` — Get SIP messages for a phone number
- `elevenlabs-pp-cli convai list-test-invocations-route` — Lists all test invocations with pagination support and optional search filtering.
- `elevenlabs-pp-cli convai list-whatsapp-accounts` — List all WhatsApp accounts
- `elevenlabs-pp-cli convai merge-branch-into-target` — Merge a branch into a target branch
- `elevenlabs-pp-cli convai patch-agent-settings-route` — Patches an Agent settings
- `elevenlabs-pp-cli convai post-agent-avatar-route` — Sets the avatar for an agent displayed in the widget
- `elevenlabs-pp-cli convai post-conversation-feedback-route` — Send the feedback for the given conversation
- `elevenlabs-pp-cli convai post-knowledge-base-bulk-move-route` — Moves multiple entities from one folder to another.
- `elevenlabs-pp-cli convai post-knowledge-base-move-route` — Moves the entity from one folder to another.
- `elevenlabs-pp-cli convai rag-index-status` — In case the document is not RAG indexed, it triggers rag indexing task, otherwise it just returns the current status.
- `elevenlabs-pp-cli convai refresh-url-document-route` — Manually refresh a URL document by re-fetching its content from the source URL.
- `elevenlabs-pp-cli convai register-twilio-call` — Register a Twilio call and return TwiML to connect the call
- `elevenlabs-pp-cli convai remove-mcp-server-tool-approval-route` — Remove approval for a specific MCP tool when using per-tool approval mode.
- `elevenlabs-pp-cli convai remove-mcp-tool-config-override-route` — Remove configuration overrides for a specific MCP tool.
- `elevenlabs-pp-cli convai resubmit-tests-route` — Resubmits specific test runs from a test invocation.
- `elevenlabs-pp-cli convai retry-batch-call` — Retry a batch call, calling failed and no-response recipients again.
- `elevenlabs-pp-cli convai run-agent-test-suite-route` — Run selected tests on the agent with provided configuration. If the agent configuration is provided, it will be used...
- `elevenlabs-pp-cli convai run-conversation-analysis` — Run the analysis for a conversation using the agent's current evaluation criteria and data collection settings.
- `elevenlabs-pp-cli convai run-conversation-simulation-route` — Run a conversation between the agent and a simulated user.
- `elevenlabs-pp-cli convai run-conversation-simulation-route-stream` — Run a conversation between the agent and a simulated user and stream back the response. Response is streamed back as...
- `elevenlabs-pp-cli convai search-knowledge-base-content-route` — Fuzzy text search over knowledge base document content
- `elevenlabs-pp-cli convai smart-search-conversation-messages-route` — Search conversation transcripts by semantic similarity to surface relevant messages based on meaning and intent,...
- `elevenlabs-pp-cli convai text-search-conversation-messages-route` — Search through conversation transcript messages by full-text and fuzzy search
- `elevenlabs-pp-cli convai unassign-conversation-tag-route` — Remove a single conversation tag from a conversation.
- `elevenlabs-pp-cli convai update-agent-response-test-route` — Updates an agent response test by ID.
- `elevenlabs-pp-cli convai update-agent-test-folder-route` — Updates an agent test folder. Currently only supports updating the folder name.
- `elevenlabs-pp-cli convai update-branch-route` — Update agent branch properties such as archiving status and protection level
- `elevenlabs-pp-cli convai update-conversation-tag-route` — Update a conversation tag's title and/or description. Restricted to the tag owner or a workspace admin.
- `elevenlabs-pp-cli convai update-dashboard-settings-route` — Update Convai dashboard settings for the workspace
- `elevenlabs-pp-cli convai update-document-route` — Update the name and/or content of a document.
- `elevenlabs-pp-cli convai update-environment-variable` — Replace an environment variable's values. Use null to remove an environment (except production).
- `elevenlabs-pp-cli convai update-file-document-route` — Update the source file of a file document. The document name, content, and metadata are updated to reflect the new...
- `elevenlabs-pp-cli convai update-mcp-server-approval-policy-route` — Update the approval policy configuration for an MCP server. DEPRECATED: Use PATCH /mcp-servers/{id} endpoint instead.
- `elevenlabs-pp-cli convai update-mcp-server-config-route` — Update the configuration settings for an MCP server.
- `elevenlabs-pp-cli convai update-mcp-tool-config-override-route` — Update configuration overrides for a specific MCP tool.
- `elevenlabs-pp-cli convai update-phone-number-route` — Update assigned agent of a phone number
- `elevenlabs-pp-cli convai update-secret-route` — Update an existing secret for the workspace
- `elevenlabs-pp-cli convai update-settings-route` — Update Convai settings for the workspace
- `elevenlabs-pp-cli convai update-tool-route` — Update tool that is available in the workspace.
- `elevenlabs-pp-cli convai update-whatsapp-account` — Update a WhatsApp account
- `elevenlabs-pp-cli convai upload-file-route` — Upload an image or PDF file for a conversation. Returns a unique file ID that can be used to reference the file in...
- `elevenlabs-pp-cli convai whatsapp-outbound-call` — Make an outbound call via WhatsApp
- `elevenlabs-pp-cli convai whatsapp-outbound-message` — Send an outbound message via WhatsApp

**docs** — Manage docs

- `elevenlabs-pp-cli docs` — Redirect To Mintlify

**dubbing** — Manage dubbing

- `elevenlabs-pp-cli dubbing add-language` — Adds the given ElevenLab Turbo V2/V2.5 language code to the resource. Does not automatically generate...
- `elevenlabs-pp-cli dubbing create` — Dubs a provided audio or video file into given language.
- `elevenlabs-pp-cli dubbing create-clip` — Creates a new segment in dubbing resource with a start and end time for the speaker in every available language....
- `elevenlabs-pp-cli dubbing create-speaker` — Create A New Speaker
- `elevenlabs-pp-cli dubbing delete` — Deletes a dubbing project.
- `elevenlabs-pp-cli dubbing delete-segment` — Deletes a single segment from the dubbing.
- `elevenlabs-pp-cli dubbing dub` — Regenerate the dubs for either the entire resource or the specified segments/languages. Will automatically...
- `elevenlabs-pp-cli dubbing get-dubbed-metadata` — Returns metadata about a dubbing project, including whether it's still in progress or not
- `elevenlabs-pp-cli dubbing get-resource` — Given a dubbing ID generated from the '/v1/dubbing' endpoint with studio enabled, returns the dubbing resource.
- `elevenlabs-pp-cli dubbing get-similar-voices-for-speaker` — Fetch the top 10 similar voices to a speaker, including the voice IDs, names, descriptions, and, where possible, a...
- `elevenlabs-pp-cli dubbing list-dubs` — List the dubs you have access to.
- `elevenlabs-pp-cli dubbing migrate-segments` — Change the attribution of one or more segments to a different speaker.
- `elevenlabs-pp-cli dubbing render` — Regenerate the output media for a language using the latest Studio state. Please ensure all segments have been...
- `elevenlabs-pp-cli dubbing transcribe` — Regenerate the transcriptions for the specified segments. Does not automatically regenerate translations or dubs.
- `elevenlabs-pp-cli dubbing translate` — Regenerate the translations for either the entire resource or the specified segments/languages. Will automatically...
- `elevenlabs-pp-cli dubbing update-segment-language` — Modifies a single segment with new text and/or start/end times. Will update the values for only a specific language...
- `elevenlabs-pp-cli dubbing update-speaker` — Amend the metadata associated with a speaker, such as their voice. Both voice cloning and using voices from the...

**forced-alignment** — Force align an audio file to a text transcript to get precise word-level and character level timing information. Response is a list of characters with their start and end times as milliseconds elapsed from the start of the recording.

- `elevenlabs-pp-cli forced-alignment` — Force align an audio file to text. Use this endpoint to get the timing information for each character and word in an...

**history** — Manage history

- `elevenlabs-pp-cli history delete-speech-item` — Delete a history item by its ID
- `elevenlabs-pp-cli history download-speech-items` — Download one or more history items. If one history item ID is provided, we will return a single audio file. If more...
- `elevenlabs-pp-cli history get-speech` — Returns a list of your generated audio.
- `elevenlabs-pp-cli history get-speech-item-by-id` — Retrieves a history item.

**models** — Access the different models of the platform.

- `elevenlabs-pp-cli models` — Gets a list of available models.

**music** — Manage music

- `elevenlabs-pp-cli music compose-detailed` — Compose a song from a prompt or a composition plan.
- `elevenlabs-pp-cli music compose-plan` — Generate a composition plan from a prompt.
- `elevenlabs-pp-cli music generate` — Compose a song from a prompt or a composition plan.
- `elevenlabs-pp-cli music separate-song-stems` — Separate an audio file into individual stems. This endpoint might have high latency, depending on the length of the...
- `elevenlabs-pp-cli music stream-compose` — Stream a composed song from a prompt or a composition plan.
- `elevenlabs-pp-cli music upload-song` — Upload a music file to be later used for inpainting. Only available to enterprise clients with access to the...
- `elevenlabs-pp-cli music video-to` — Generate background music from one or more video files. Videos are combined in order. Optional description and style...

**pronunciation-dictionaries** — Manage pronunciation dictionaries

- `elevenlabs-pp-cli pronunciation-dictionaries add-from-file` — Creates a new pronunciation dictionary from a lexicon .PLS file
- `elevenlabs-pp-cli pronunciation-dictionaries add-from-rules` — Creates a new pronunciation dictionary from provided rules.
- `elevenlabs-pp-cli pronunciation-dictionaries get-metadata` — Get a list of the pronunciation dictionaries you have access to and their metadata
- `elevenlabs-pp-cli pronunciation-dictionaries get-pronunciation-dictionary-metadata` — Get metadata for a pronunciation dictionary
- `elevenlabs-pp-cli pronunciation-dictionaries patch-pronunciation-dictionary` — Partially update the pronunciation dictionary without changing the version

**service-accounts** — Manage service accounts

- `elevenlabs-pp-cli service-accounts` — List all service accounts in the workspace

**shared-voices** — Manage shared voices

- `elevenlabs-pp-cli shared-voices` — Retrieves a list of shared voices.

**similar-voices** — Manage similar voices

- `elevenlabs-pp-cli similar-voices` — Returns a list of shared voices similar to the provided audio sample. If neither similarity_threshold nor top_k is...

**single-use-token** — Manage single use token

- `elevenlabs-pp-cli single-use-token` — Generate a time limited single-use token with embedded authentication for frontend clients.

**sound-generation** — Manage sound generation

- `elevenlabs-pp-cli sound-generation` — Turn text into sound effects for your videos, voice-overs or video games using the most advanced sound effects...

**speech-engine** — Manage speech engine

- `elevenlabs-pp-cli speech-engine create` — Create a new Speech Engine resource
- `elevenlabs-pp-cli speech-engine delete` — Delete a Speech Engine resource
- `elevenlabs-pp-cli speech-engine get` — Retrieve a Speech Engine resource
- `elevenlabs-pp-cli speech-engine list` — Returns a paginated list of Speech Engine resources.
- `elevenlabs-pp-cli speech-engine update` — Update a Speech Engine resource (partial update)

**speech-to-speech** — Create speech by combining the style and content of an audio file you upload with a voice of your choice.

- `elevenlabs-pp-cli speech-to-speech <voice_id>` — Transform audio from one voice to another. Maintain full control over emotion, timing and delivery.

**speech-to-text** — Transcribe your audio files with detailed speaker annotations and precise timestamps using our cutting-edge model.

- `elevenlabs-pp-cli speech-to-text delete-transcript-by-id` — Delete a previously generated transcript by its ID.
- `elevenlabs-pp-cli speech-to-text get-transcript-by-id` — Retrieve a previously generated transcript by its ID.
- `elevenlabs-pp-cli speech-to-text speech_to_text` — Transcribe an audio or video file. If webhook is set to true, the request will be processed asynchronously and...

**studio** — Access, create and convert Studio Projects programmatically, only specifically whitelisted accounts can access the Studio API. If you need access please contact our sales team.

- `elevenlabs-pp-cli studio add-chapter` — Creates a new chapter either as blank or from a URL.
- `elevenlabs-pp-cli studio add-project` — Creates a new Studio project, it can be either initialized as blank, from a document or from a URL.
- `elevenlabs-pp-cli studio convert-chapter-endpoint` — Starts conversion of a specific chapter.
- `elevenlabs-pp-cli studio convert-project-endpoint` — Starts conversion of a Studio project and all of its chapters.
- `elevenlabs-pp-cli studio create-podcast` — Create and auto-convert a podcast project. Currently, the LLM cost is covered by us but you will still be charged...
- `elevenlabs-pp-cli studio delete-chapter-endpoint` — Deletes a chapter.
- `elevenlabs-pp-cli studio delete-project` — Deletes a Studio project.
- `elevenlabs-pp-cli studio edit-chapter` — Updates a chapter.
- `elevenlabs-pp-cli studio edit-project` — Updates the specified Studio project by setting the values of the parameters passed.
- `elevenlabs-pp-cli studio edit-project-content` — Updates Studio project content.
- `elevenlabs-pp-cli studio get-chapter-by-id-endpoint` — Returns information about a specific chapter.
- `elevenlabs-pp-cli studio get-chapter-snapshot-endpoint` — Returns the chapter snapshot.
- `elevenlabs-pp-cli studio get-chapter-snapshots` — Gets information about all the snapshots of a chapter. Each snapshot can be downloaded as audio. Whenever a chapter...
- `elevenlabs-pp-cli studio get-chapters` — Returns a list of a Studio project's chapters.
- `elevenlabs-pp-cli studio get-project-by-id` — Returns information about a specific Studio project. This endpoint returns more detailed information about a project...
- `elevenlabs-pp-cli studio get-project-muted-tracks-endpoint` — Returns a list of chapter IDs that have muted tracks in a project.
- `elevenlabs-pp-cli studio get-project-snapshot-endpoint` — Returns the project snapshot.
- `elevenlabs-pp-cli studio get-project-snapshots` — Retrieves a list of snapshots for a Studio project.
- `elevenlabs-pp-cli studio get-projects` — Returns a list of your Studio projects with metadata.
- `elevenlabs-pp-cli studio stream-chapter-snapshot-audio` — Stream the audio from a chapter snapshot. Use `GET /v1/studio/projects/{project_id}/chapters/{chapter_id}/snapshots`...
- `elevenlabs-pp-cli studio stream-project-snapshot-archive-endpoint` — Returns a compressed archive of the Studio project's audio.
- `elevenlabs-pp-cli studio stream-project-snapshot-audio-endpoint` — Stream the audio from a Studio project snapshot.
- `elevenlabs-pp-cli studio update-pronunciation-dictionaries` — Create a set of pronunciation dictionaries acting on a project. This will automatically mark text within this...

**text-to-dialogue** — Manage text to dialogue

- `elevenlabs-pp-cli text-to-dialogue full-with-timestamps` — Generate dialogue from text with precise character-level timing information for audio-text synchronization.
- `elevenlabs-pp-cli text-to-dialogue stream` — Converts a list of text and voice ID pairs into speech (dialogue) and returns an audio stream.
- `elevenlabs-pp-cli text-to-dialogue stream-with-timestamps` — Converts a list of text and voice ID pairs into speech (dialogue) and returns a stream of JSON blobs containing...
- `elevenlabs-pp-cli text-to-dialogue text_to_dialogue` — Converts a list of text and voice ID pairs into speech (dialogue) and returns audio.

**text-to-speech** — Convert text into lifelike speech using a voice of your choice.

- `elevenlabs-pp-cli text-to-speech <voice_id>` — Converts text into speech using a voice of your choice and returns audio.

**text-to-voice** — Manage text to voice

- `elevenlabs-pp-cli text-to-voice create-voice` — Create a voice from previously generated voice preview. This endpoint should be called after you fetched a...
- `elevenlabs-pp-cli text-to-voice design` — Design a voice via a prompt. This method returns a list of voice previews. Each preview has a generated_voice_id and...
- `elevenlabs-pp-cli text-to-voice text_to_voice` — **Deprecated.** Use `POST /v1/text-to-voice/design` instead. Generate a custom voice based on voice description....

**usage** — Manage usage

- `elevenlabs-pp-cli usage` — (Deprecated) This endpoint is deprecated. Use /v1/workspace/analytics/query/usage-by-product-over-time instead....

**user** — Manage user

- `elevenlabs-pp-cli user get-info` — Gets information about the user
- `elevenlabs-pp-cli user get-subscription-info` — Gets extended information about the users subscription

**voices** — Access to voices created either by you or ElevenLabs.

- `elevenlabs-pp-cli voices add` — Add a new voice to your collection of voices in VoiceLab.
- `elevenlabs-pp-cli voices add-pvc-samples` — Add audio samples to a PVC voice
- `elevenlabs-pp-cli voices add-sharing` — Add a shared voice to your collection of voices.
- `elevenlabs-pp-cli voices create-pvc` — Creates a new PVC voice with metadata but no samples
- `elevenlabs-pp-cli voices delete` — Deletes a voice by its ID.
- `elevenlabs-pp-cli voices delete-pvc-sample` — Delete a sample from a PVC voice.
- `elevenlabs-pp-cli voices edit-pvc` — Edit PVC voice metadata
- `elevenlabs-pp-cli voices edit-pvc-sample` — Update a PVC voice sample - apply noise removal, select speaker, change trim times or file name.
- `elevenlabs-pp-cli voices get` — Returns a list of all available voices for a user. Stops working once the user's workspace exceeds 500 voices.
- `elevenlabs-pp-cli voices get-by-id` — Returns metadata about a specific voice.
- `elevenlabs-pp-cli voices get-pvc-captcha` — Get captcha for PVC voice verification.
- `elevenlabs-pp-cli voices get-pvc-sample-audio` — Retrieve the first 30 seconds of voice sample audio with or without noise removal.
- `elevenlabs-pp-cli voices get-pvc-sample-speakers` — Retrieve the status of the speaker separation process and the list of detected speakers if complete.
- `elevenlabs-pp-cli voices get-pvc-sample-visual-waveform` — Retrieve the visual waveform of a voice sample.
- `elevenlabs-pp-cli voices get-settings-default` — Gets the default settings for voices. 'similarity_boost' corresponds to'Clarity + Similarity Enhancement' in the web...
- `elevenlabs-pp-cli voices get-speaker-audio` — Retrieve the separated audio for a specific speaker.
- `elevenlabs-pp-cli voices get-user-v2` — Gets a list of all available voices for a user with search, filtering and pagination.
- `elevenlabs-pp-cli voices request-pvc-manual-verification` — Request manual verification for a PVC voice.
- `elevenlabs-pp-cli voices run-pvc-training` — Start PVC training process for a voice.
- `elevenlabs-pp-cli voices start-speaker-separation` — Start speaker separation process for a sample
- `elevenlabs-pp-cli voices verify-pvc-captcha` — Submit captcha verification for PVC voice.

**workspace** — Access to workspace related endpoints.

- `elevenlabs-pp-cli workspace add-member` — Adds a member of your workspace to the specified group. Requires `group_members_manage` permission.
- `elevenlabs-pp-cli workspace create-auth-connection` — Create a new OAuth2 auth connection for the workspace
- `elevenlabs-pp-cli workspace create-webhook-route` — Create a new webhook for the workspace with the specified authentication type.
- `elevenlabs-pp-cli workspace delete-auth-connection` — Delete Workspace Auth Connection
- `elevenlabs-pp-cli workspace delete-invite` — Invalidates an existing email invitation. The invitation will still show up in the inbox it has been delivered to,...
- `elevenlabs-pp-cli workspace delete-webhook-route` — Delete the specified workspace webhook
- `elevenlabs-pp-cli workspace edit-webhook-route` — Update the specified workspace webhook
- `elevenlabs-pp-cli workspace get-audit-logs` — Returns the audit log for the workspace. Requires enterprise tier and the audit_log_read permission.
- `elevenlabs-pp-cli workspace get-groups-endpoint` — Get all groups in the workspace
- `elevenlabs-pp-cli workspace get-resource-metadata` — Gets the metadata of a resource by ID.
- `elevenlabs-pp-cli workspace get-webhooks-route` — List all webhooks for a workspace
- `elevenlabs-pp-cli workspace invite-user` — Sends an email invitation to join your workspace to the provided email. If the user doesn't have an account they...
- `elevenlabs-pp-cli workspace invite-users-bulk` — Sends email invitations to join your workspace to the provided emails. Requires all email addresses to be part of a...
- `elevenlabs-pp-cli workspace list-auth-connections` — Get all auth connections for the workspace
- `elevenlabs-pp-cli workspace remove-member` — Removes a member from the specified group. Requires `group_members_manage` permission.
- `elevenlabs-pp-cli workspace requests-list` — Returns a list of API requests. Supports filtering by time range, column filters, and search terms. At least one of...
- `elevenlabs-pp-cli workspace search-groups` — Searches for user groups in the workspace. Multiple or no groups may be returned.
- `elevenlabs-pp-cli workspace share-resource-endpoint` — Grants a role on a workspace resource to a user or a group. It overrides any existing role this user/service...
- `elevenlabs-pp-cli workspace unshare-resource-endpoint` — Removes any existing role on a workspace resource from a user, service account, group or workspace api key. To...
- `elevenlabs-pp-cli workspace update-member` — Updates attributes of a workspace member. Apart from the email identifier, all parameters will remain unchanged...
- `elevenlabs-pp-cli workspace usage-by-product-over-time` — Returns credit usage broken down by product type over time. The response is a tabular structure with columns,...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
elevenlabs-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `elevenlabs-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export ELEVENLABS_API_KEY="<your-key>"
```

Or persist it in `~/.config/elevenlabs-documentation-pp-cli/config.toml`.

Run `elevenlabs-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  elevenlabs-pp-cli models --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
elevenlabs-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
elevenlabs-pp-cli feedback --stdin < notes.txt
elevenlabs-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.elevenlabs-pp-cli/feedback.jsonl`. They are never POSTed unless `ELEVENLABS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ELEVENLABS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
elevenlabs-pp-cli profile save briefing --json
elevenlabs-pp-cli --profile briefing models
elevenlabs-pp-cli profile list --json
elevenlabs-pp-cli profile show briefing
elevenlabs-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `elevenlabs-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/elevenlabs/cmd/elevenlabs-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add elevenlabs-pp-mcp -- elevenlabs-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which elevenlabs-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   elevenlabs-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `elevenlabs-pp-cli <command> --help`.
