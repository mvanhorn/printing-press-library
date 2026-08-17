> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Delete Key

> Delete an API key from an app



## OpenAPI

````yaml https://api.parallel.ai/account/service/openapi.json delete /service/v1/apps/{app_id}/keys/{api_key_id}
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
  /service/v1/apps/{app_id}/keys/{api_key_id}:
    delete:
      tags:
        - Keys
      summary: Delete Key
      description: Delete an API key from an app
      operationId: delete_key_service_v1_apps__app_id__keys__api_key_id__delete
      parameters:
        - name: app_id
          in: path
          required: true
          schema:
            type: string
            format: uuid
            title: App Id
        - name: api_key_id
          in: path
          required: true
          schema:
            type: string
            title: Api Key Id
      responses:
        '200':
          description: Successful Response
          content:
            application/json:
              schema: {}
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