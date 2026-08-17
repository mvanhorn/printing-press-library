> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Add to Balance

> Charge the organization's default payment method and add the amount to the prepaid credit balance. The default payment method configured on the org's Stripe customer is always used; no payment method id is accepted from the client.



## OpenAPI

````yaml https://api.parallel.ai/account/service/openapi.json post /service/v1/balance/add
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
  /service/v1/balance/add:
    post:
      tags:
        - Balance
      summary: Add to Balance
      description: >-
        Charge the organization's default payment method and add the amount to
        the prepaid credit balance. The default payment method configured on the
        org's Stripe customer is always used; no payment method id is accepted
        from the client.
      operationId: add_balance_service_v1_balance_add_post
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/AddBalanceRequest'
        required: true
      responses:
        '200':
          description: Successful Response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/BalanceResponse'
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
    AddBalanceRequest:
      properties:
        amount_cents:
          type: integer
          maximum: 10000
          exclusiveMinimum: 0
          title: Amount Cents
          description: >-
            Amount in cents to charge and add to the balance. Must be between 1
            and 10000 cents ($100.00).
        idempotency_key:
          type: string
          maxLength: 128
          minLength: 8
          title: Idempotency Key
          description: >-
            Required idempotency key. Stripe dedupes the charge server-side for
            at least 24h when the same key is submitted again for the same org
            (see https://docs.stripe.com/api/idempotent_requests). Pick a
            high-entropy value (e.g. a UUID) so distinct agent attempts do not
            collide.
      type: object
      required:
        - amount_cents
        - idempotency_key
      title: AddBalanceRequest
      description: Request to charge the org's default payment method and add to balance.
    BalanceResponse:
      properties:
        org_id:
          type: string
          title: Org Id
          description: Organization ID
        credit_balance_cents:
          type: number
          title: Credit Balance Cents
          description: >-
            Total available prepaid balance in cents (credits + prepaid
            commits). Always 0 when will_invoice is true.
        pending_debit_balance_cents:
          type: number
          title: Pending Debit Balance Cents
          description: >-
            Balance in cents currently held for inflight tasks plus charges not
            yet synced to the billing provider. Always 0 when will_invoice is
            true.
        will_invoice:
          type: boolean
          title: Will Invoice
          description: >-
            True if this organization is billed by invoice (postpaid) rather
            than from a prepaid credit balance. Invoice-only orgs cannot add
            balance via this API.
      type: object
      required:
        - org_id
        - credit_balance_cents
        - pending_debit_balance_cents
        - will_invoice
      title: BalanceResponse
      description: Balance response for service clients.
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