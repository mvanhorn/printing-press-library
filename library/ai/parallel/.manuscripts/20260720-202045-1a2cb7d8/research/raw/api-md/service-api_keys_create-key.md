> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Create Key

> Create a new API key for an app



## OpenAPI

````yaml https://api.parallel.ai/account/service/openapi.json post /service/v1/apps/{app_id}/keys
openapi: 3.1.0
info:
  title: FastAPI
  version: 0.1.0
servers:
  - url: https://api.parallel.ai/account
    description: Parallel Account API
security: []
tags:
  - name: Apps
    description: Application management endpoints
  - name: Keys
    description: API key management endpoints
  - name: Balance
    description: Organization balance endpoints
  - name: Service
    description: Service utility endpoints
paths:
  /service/v1/apps/{app_id}/keys:
    post:
      tags:
        - Keys
      summary: Create Key
      description: Create a new API key for an app
      operationId: create_key_service_v1_apps__app_id__keys_post
      parameters:
        - name: app_id
          in: path
          required: true
          schema:
            type: string
            format: uuid
            title: App Id
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateApiKeyRequestModel'
      responses:
        '200':
          description: Successful Response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/CreateKeyResponse'
        '422':
          description: Validation Error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/HTTPValidationError'
      security:
        - BearerAuth: []
components:
  schemas:
    CreateApiKeyRequestModel:
      properties:
        api_key_name:
          type: string
          title: Api Key Name
          description: API Key Name
      type: object
      required:
        - api_key_name
      title: CreateApiKeyRequestModel
      description: Model for create API key request V2.
    CreateKeyResponse:
      properties:
        api_key_id:
          type: string
          title: Api Key Id
          description: API Key ID
        api_key_name:
          type: string
          title: Api Key Name
          description: API Key Name
        app_id:
          type: string
          title: App Id
          description: App ID
        app_name:
          type: string
          title: App Name
          description: App Name
        created_by_user_id:
          type: string
          title: Created By User Id
          description: Created by User ID
        created_by_user_email:
          type: string
          title: Created By User Email
          description: Created by User Email
        display_value:
          type: string
          title: Display Value
          description: Display Value
        raw_api_key:
          type: string
          format: password
          title: Raw Api Key
          description: Raw API Key
          writeOnly: true
        created_at:
          type: integer
          title: Created At
          description: Created At
      type: object
      required:
        - api_key_id
        - api_key_name
        - app_id
        - app_name
        - created_by_user_id
        - created_by_user_email
        - display_value
        - raw_api_key
        - created_at
      title: CreateKeyResponse
      description: Flattened response for the service create-key endpoint.
    HTTPValidationError:
      properties:
        detail:
          items:
            $ref: '#/components/schemas/ValidationError'
          type: array
          title: Detail
      type: object
      title: HTTPValidationError
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
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
      description: >-
        Send `Authorization: Bearer <access_token>`. This must be an account API
        access token minted via Parallel's OAuth device flow, not a standard API
        key. See the [account API docs](/integrations/account-api).

````