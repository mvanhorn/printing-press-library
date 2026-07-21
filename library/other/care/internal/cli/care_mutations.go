// Copyright 2026 beetz12. Licensed under Apache-2.0.
// care.com GraphQL MUTATION documents captured from the authenticated web app.
// Hand-authored; safe across regen. WRITE operations - callers must gate these
// behind explicit confirmation (see care_outreach.go send + care_favorite.go).

package cli

const careMSendMessageOp = "sendCareNeedMessage"
const careMSendMessage = `mutation sendCareNeedMessage($input: SendCareNeedMessageInput!) {
  sendCareNeedMessage(input: $input) {
    ... on SendCareNeedMessageSuccess {
      __typename
      dummy
    }
    ... on SendCareNeedMessageRecipientsError {
      __typename
      failedRecipientIds
      failureType
      technicalMessages
      requestId
    }
    ... on SendCareNeedMessageFailure {
      __typename
      technicalMessage
      requestId
    }
    ... on SendCareNeedMessageDetectedConcernsFailure {
      validationId
      detectedConcerns {
        type
        text
        endIndex
        startIndex
        __typename
      }
      __typename
    }
    __typename
  }
}`

const careMFavoriteProviderOp = "FavoriteProvider"
const careMFavoriteProvider = `mutation FavoriteProvider($input: FavoriteProviderInput!) {
  favoriteProvider(input: $input) {
    ... on FavoriteCaregiverPayload {
      __typename
      favorite {
        favoriteStatus
        id
        __typename
      }
    }
    ... on FavoriteProviderInvalidServiceProfile {
      message
      __typename
    }
    __typename
  }
}`
