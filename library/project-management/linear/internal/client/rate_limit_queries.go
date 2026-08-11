package client

// GraphQL documents for Linear's rate limit introspection.
//
// Field names verified against the live schema introspection: Query
// .rateLimitStatus returns RateLimitPayload! (identifier, kind, limits) and
// each RateLimitResultPayload carries type, requestedAmount, allowedAmount,
// period, remainingAmount and reset. period is a duration in milliseconds
// and reset is a UNIX timestamp in milliseconds.

const RateLimitStatusQuery = `query {
  rateLimitStatus {
    identifier
    kind
    limits {
      type
      requestedAmount
      allowedAmount
      period
      remainingAmount
      reset
    }
  }
}`
