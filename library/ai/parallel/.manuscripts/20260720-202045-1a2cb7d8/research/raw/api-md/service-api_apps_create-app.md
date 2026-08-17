> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Create App

> Create a new app for the authenticated organization



## OpenAPI

````yaml https://api.parallel.ai/account/service/openapi.json post /service/v1/apps
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
  /service/v1/apps:
    post:
      tags:
        - Apps
      summary: Create App
      description: Create a new app for the authenticated organization
      operationId: create_app_service_v1_apps_post
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateAppRequestModel'
        required: true
      responses:
        '200':
          description: Successful Response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/CreateAppResponseModel'
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
    CreateAppRequestModel:
      properties:
        app_name:
          type: string
          title: App Name
          description: App name
        allow_oauth_app_names:
          type: boolean
          title: Allow Oauth App Names
          description: >-
            Mark OAuth reserved app-name prefixes ('OAuth client at ...',
            'Device auth for ...') as expected. Set only by the platform's OAuth
            key-share flows; ignored on the service API path.
          default: false
      type: object
      required:
        - app_name
      title: CreateAppRequestModel
      description: Model for creating app request.
    CreateAppResponseModel:
      properties:
        app_id:
          type: string
          title: App Id
          description: App ID
      type: object
      required:
        - app_id
      title: CreateAppResponseModel
      description: Model for creating app response.
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