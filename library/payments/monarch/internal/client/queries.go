// Copyright 2026 kyle-kirkland. Licensed under Apache-2.0. See LICENSE.
// GraphQL query constants. The spec-derived placeholders were re-authored to
// match Monarch's live schema (operation names, field arguments, selection sets).
// Field shapes here are coupled to the Go struct definitions in
// internal/cli/*.go that consume them.

package client

const AccountsListQuery = `query Common_GetAccounts {
  accounts {
    id
    displayName
    currentBalance
    updatedAt
    syncDisabled
    isManual
    isAsset
    includeInNetWorth
    type { name display }
    institution { id name }
  }
}`

const BudgetsListQuery = `query Common_GetBudgetData($startMonth: Date!, $endMonth: Date!) {
  budgetData(startMonth: $startMonth, endMonth: $endMonth) {
    monthlyAmountsByCategory {
      category { id name }
    }
  }
}`

const CategoriesListQuery = `query Common_GetCategories {
  categories {
    id
    name
    icon
    group { id name type }
  }
}`

const GoalsListQuery = `query Web_GoalsV2 {
  goalsV2 {
    id
    name
    currentAmount
    targetAmount
    startingAmount
    completionPercent
    priority
    archivedAt
    completedAt
  }
}`

const HoldingsListQuery = `query Web_GetHoldings {
  portfolio {
    aggregateHoldings {
      edges {
        node {
          id
          quantity
          basis
          totalValue
          security { id name ticker currentPrice }
        }
      }
    }
  }
}`

const InstitutionsListQuery = `query Web_GetInstitutionsList {
  credentials {
    id
    institution { id name url }
  }
}`

const MeGetQuery = `query Common_GetMe {
  me {
    id
    name
    email
  }
}`

// RecurringListQuery aliases recurringTransactionItems as `recurringItems` so
// callers receive `{"recurringItems": [...]}` directly (no aggregatedRecurringItems
// wrapper exists in the live schema).
const RecurringListQuery = `query Common_RecurringTransactionItems($startDate: Date!, $endDate: Date!) {
  recurringItems: recurringTransactionItems(startDate: $startDate, endDate: $endDate) {
    date
    amount
    isPast
    stream {
      id
      frequency
      merchant { id name }
    }
  }
}`

const SubscriptionGetQuery = `query Common_GetSubscriptionStatus {
  subscription {
    id
    paymentSource
    isOnFreeTrial
    trialEndsAt
    nextPaymentAmount
  }
}`

const TagsListQuery = `query Common_GetHouseholdTransactionTags {
  householdTransactionTags {
    id
    name
    color
    transactionCount
  }
}`

const TransactionsGetQuery = `query Web_GetTransaction($id: UUID!) {
  getTransaction(id: $id) {
    id
    amount
    date
    notes
    plaidName
    isSplitTransaction
    merchant { id name }
    category { id name }
    account { id displayName }
    tags { id name }
    splitTransactions { id amount notes category { id name } }
  }
}`

// TransactionsListQuery uses the live signature where pagination args
// (limit, offset) sit on `results`, not on `allTransactions`. The `orderBy`
// argument was dropped because no scalar/enum form is currently accepted by
// the server; consumers that need a particular ordering must sort client-side.
const TransactionsListQuery = `query Web_GetTransactionsList($limit: Int, $offset: Int, $filters: TransactionFilterInput) {
  allTransactions(filters: $filters) {
    totalCount
    results(limit: $limit, offset: $offset) {
      id
      amount
      date
      merchant { id name }
      category { id name }
    }
  }
}`
