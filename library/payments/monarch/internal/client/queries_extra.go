// Copyright 2026 kyle-kirkland. Licensed under Apache-2.0. See LICENSE.
// Hand-authored GraphQL queries for novel-feature commands (transcendence layer).
// These compose data the spec-derived queries don't surface in one round-trip.

package client

// TransactionsSummaryQuery returns aggregate spend/income totals over a window,
// grouped by category. Used by snapshot, networth explain, cashflow forecast,
// and cashflow monthly-memo.
const TransactionsSummaryQuery = `query Web_GetTransactionsSummary($filters: TransactionFilterInput) {
  aggregates(filters: $filters) {
    summary {
      sum
      count
      avg
      max
      sumIncome
      sumExpense
    }
  }
}`

// BudgetsStatusQuery returns per-budget actual-vs-allocation for a date window.
// Used by budgets burn and cashflow monthly-memo.
const BudgetsStatusQuery = `query Common_GetJointPlanningData($startDate: Date!, $endDate: Date!) {
  budgetData(startMonth: $startDate, endMonth: $endDate) {
    monthlyAmountsByCategory {
      category { id name }
      monthlyAmounts {
        month
        plannedCashFlowAmount
        actualAmount
        remainingAmount
      }
    }
  }
}`

// --- Mutations ---
// These mirror the Monarch web app's mutation surface (operation names follow
// the `Common_*` / `Web_*` convention). Field selections capture the canonical
// success/error envelope; payload shapes follow monarchmoney's published
// reverse-engineered conventions.

const TransactionsCreateMutation = `mutation Common_CreateTransaction($input: CreateTransactionMutationInput!) {
  createTransaction(input: $input) {
    transaction { id amount date }
    errors { message fieldErrors { field messages } }
  }
}`

const TransactionsUpdateMutation = `mutation Web_TransactionDrawerUpdateTransaction($input: UpdateTransactionMutationInput!) {
  updateTransaction(input: $input) {
    transaction { id amount date notes }
    errors { message fieldErrors { field messages } }
  }
}`

const TransactionsDeleteMutation = `mutation Common_DeleteTransactionMutation($input: DeleteTransactionMutationInput!) {
  deleteTransaction(input: $input) {
    deleted
    errors { message }
  }
}`

const TransactionsSplitsSetMutation = `mutation Common_UpdateTransactionSplitMutation($input: UpdateTransactionSplitMutationInput!) {
  updateTransactionSplit(input: $input) {
    transaction { id hasSplitTransactions splitTransactions { id amount notes category { id name } } }
    errors { message }
  }
}`

const TransactionsTagsSetMutation = `mutation Web_SetTransactionTags($input: SetTransactionTagsInput!) {
  setTransactionTags(input: $input) {
    transaction { id tags { id name } }
    errors { message }
  }
}`

const AccountsRefreshMutation = `mutation Common_ForceRefreshAccounts($input: ForceRefreshAccountsInput!) {
  forceRefreshAccounts(input: $input) {
    success
    errors { message }
  }
}`

// CategoriesCreateMutation uses scalar variables because Monarch's introspection
// is disabled, so resolving the input object's actual GraphQL type name (it is
// not `CreateCategoryMutationInput`) is not feasible from the outside. Inlining
// the input literal with scalar variables sidesteps the type-name lookup.
const CategoriesCreateMutation = `mutation Web_CreateCategory($name: String!, $group: ID!, $icon: String!) {
  createCategory(input: {name: $name, group: $group, icon: $icon}) {
    category { id name icon group { id name } }
    errors { message fieldErrors { field messages } }
  }
}`

const CategoriesDeleteMutation = `mutation Web_DeleteCategory($id: UUID!) {
  deleteCategory(id: $id) {
    deleted
    errors { message }
  }
}`

const GoalsCreateMutation = `mutation Common_CreateGoalV2($input: CreateGoalV2MutationInput!) {
  createGoalV2(input: $input) {
    goal { id name targetAmount }
    errors { message fieldErrors { field messages } }
  }
}`

const GoalsDeleteMutation = `mutation Common_DeleteGoalV2($id: UUID!) {
  deleteGoalV2(id: $id) {
    deleted
    errors { message }
  }
}`

const TagsCreateMutation = `mutation Common_CreateTransactionTag($name: String!, $color: String!) {
  createTransactionTag(input: {name: $name, color: $color}) {
    tag { id name color }
    errors { message fieldErrors { field messages } }
  }
}`

const TagsDeleteMutation = `mutation Common_DeleteHouseholdTransactionTag($tagId: ID!) {
  deleteTransactionTag(tagId: $tagId) {
    errors { message fieldErrors { field messages } }
  }
}`

const BudgetsSetAmountMutation = `mutation Common_UpdateBudgetItem($input: UpdateOrCreateBudgetItemMutationInput!) {
  updateOrCreateBudgetItem(input: $input) {
    budgetItem { id plannedCashFlowAmount }
  }
}`

// RecurringSearchQuery searches the merchant catalog by name fragment so users
// can pick a merchant to add as a new recurring stream. Backed by the live
// `merchants(search: ...)` field; the spec's was an empty stub.
const RecurringSearchQuery = `query Common_SearchMerchantsForRecurring($search: String) {
  merchants(search: $search) {
    id
    name
    transactionCount
  }
}`

// AccountsTypesQuery returns the catalog of account types and subtypes used by
// Monarch (depository, brokerage, real_estate, vehicle, etc.).
const AccountsTypesQuery = `query Web_GetAccountTypeOptions {
  accountTypeOptions {
    type { name display }
    subtype { name display }
  }
}`

// AccountsBalanceHistoryQuery returns each account's recentBalances series
// starting from $startDate. Pairs with the spec-mapped accounts balance-history
// command, which previously POSTed an empty body.
const AccountsBalanceHistoryQuery = `query Common_GetAccountsBalanceHistory($startDate: Date!) {
  accounts {
    id
    displayName
    recentBalances(startDate: $startDate)
  }
}`

// AccountsRefreshStatusQuery reports whether any account has an in-flight sync.
// Used by accounts refresh-status; the spec's was an empty stub.
const AccountsRefreshStatusQuery = `query Common_GetAccountsRefreshStatus {
  accounts {
    id
    displayName
    syncDisabled
    hasSyncInProgress
    updatedAt
  }
}`

// CategoryGroupsQuery lists the top-level category-group taxonomy.
const CategoryGroupsQuery = `query Common_GetCategoryGroups {
  categoryGroups {
    id
    name
    type
  }
}`

// CreditReportQuery returns liability-account rows from the credit-report side
// of Monarch. statementBalance/pastDueAmount fields don't exist on the live
// schema; only liabilityType + account are exposed.
const CreditReportQuery = `query Common_GetCreditReportLiabilityAccounts {
  creditReportLiabilityAccounts {
    liabilityType
    account {
      id
      displayName
      currentBalance
      institution { id name }
    }
  }
}`

// NetworthSnapshotsQuery returns time-series net-worth snapshots within a date
// window. Used by networth history, networth explain, and cashflow monthly-memo.
// Selection set is `aggregateSnapshots` (not `accountSnapshots`); the net-worth
// total field is named `balance` upstream.
const NetworthSnapshotsQuery = `query Common_GetAggregateSnapshots($filters: AggregateSnapshotFilters) {
  aggregateSnapshots(filters: $filters) {
    date
    balance
    assetsBalance
    liabilitiesBalance
  }
}`
