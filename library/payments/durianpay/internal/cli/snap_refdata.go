// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Static reference data for the offline `explain` and `sandbox simulate`
// commands. Sourced from Durianpay's SNAP "Response Code Handling" reference
// and the sandbox "Simulating Test Pay Outs" / "Simulating Test Payments"
// docs. These tables ship embedded so the commands work with no network and
// no local store.

// snapService maps a SNAP service code to its service name and relative path.
// The service code is the middle two digits of a 7-digit SNAP response code
// (e.g. 18 in 2001800 = Bank Transfer).
type snapService struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// snapServices is the full table of SNAP services from the response-code
// reference's "Table of Service Code".
var snapServices = []snapService{
	{Code: "73", Name: "Access Token (B2B)", Path: "/v1.0/access-token/b2b"},
	{Code: "11", Name: "Balance Inquiry", Path: "/v1.0/balance-inquiry"},
	{Code: "16", Name: "Bank Account Inquiry", Path: "/v1.0/account-inquiry-external"},
	{Code: "18", Name: "Bank Transfer", Path: "/v1.0/transfer-interbank"},
	{Code: "36", Name: "Bank Transfer Status Inquiry", Path: "/v1.0/transfer/status"},
	{Code: "37", Name: "E-Wallet Account Inquiry", Path: "/v1.0/emoney/account-inquiry"},
	{Code: "38", Name: "E-Wallet Transfer", Path: "/v1.0/emoney/topup"},
	{Code: "39", Name: "E-Wallet Transfer Inquiry Status", Path: "/v1.0/emoney/topup-status"},
}

// snapCase is one row of the SNAP response-code list. ServiceCode is "any"
// when the case applies regardless of service.
type snapCase struct {
	HTTPCode    string `json:"http_code"`
	ServiceCode string `json:"service_code"`
	CaseCode    string `json:"case_code"`
	Message     string `json:"message"`
	Description string `json:"description"`
	Success     bool   `json:"success"`
}

// snapCases is the full SNAP response-code list (successful + error rows).
var snapCases = []snapCase{
	// Successful responses
	{HTTPCode: "200", ServiceCode: "any", CaseCode: "00", Message: "Successful", Description: "Successfully received in Durianpay's side (does not mean sent to beneficiary successfully)", Success: true},
	{HTTPCode: "202", ServiceCode: "any", CaseCode: "00", Message: "Request In Progress", Description: "Transaction is still processing", Success: true},
	// Error responses
	{HTTPCode: "400", ServiceCode: "any", CaseCode: "00", Message: "Bad Request", Description: "General request failed error, including message parsing failed."},
	{HTTPCode: "400", ServiceCode: "any", CaseCode: "01", Message: "Invalid Field Format {field name}", Description: "Invalid format"},
	{HTTPCode: "400", ServiceCode: "any", CaseCode: "02", Message: "Invalid Mandatory Field {field name}", Description: "Missing or invalid format on mandatory field"},
	{HTTPCode: "401", ServiceCode: "any", CaseCode: "00", Message: "Unauthorized [reason]", Description: "General unauthorized error (No Interface Def, API is Invalid, Oauth Failed, Verify Client Secret Fail, Client Forbidden Access API, Unknown Client, Key not Found, Invalid Signature)"},
	{HTTPCode: "401", ServiceCode: "any", CaseCode: "01", Message: "Invalid Token (B2B)", Description: "Token found in request is invalid (Access Token Not Exist, Access Token Expiry)"},
	{HTTPCode: "401", ServiceCode: "any", CaseCode: "03", Message: "Token Not Found (B2B)", Description: "Token not found in the system. This occurs on any API that requires token as input parameter"},
	{HTTPCode: "403", ServiceCode: "any", CaseCode: "00", Message: "Transaction Expired", Description: "Transaction expired"},
	{HTTPCode: "403", ServiceCode: "any", CaseCode: "18", Message: "Inactive Card/Account/Customer", Description: "Indicates inactive account"},
	{HTTPCode: "404", ServiceCode: "any", CaseCode: "01", Message: "Transaction Not Found", Description: "Transaction not found"},
	{HTTPCode: "404", ServiceCode: "any", CaseCode: "11", Message: "Invalid Card/Account/Customer [info]/Virtual Account", Description: "Card information may be invalid, or the card account may be blacklisted, or Virtual Account number may be invalid."},
	{HTTPCode: "409", ServiceCode: "any", CaseCode: "00", Message: "Conflict", Description: "Cannot use same X-EXTERNAL-ID in same day"},
	{HTTPCode: "409", ServiceCode: "any", CaseCode: "01", Message: "Duplicate partnerReferenceNo", Description: "Transaction has previously been processed indicates the same partnerReferenceNo already created"},
	{HTTPCode: "500", ServiceCode: "any", CaseCode: "00", Message: "General Error", Description: "General Error"},
	{HTTPCode: "500", ServiceCode: "any", CaseCode: "01", Message: "Internal Server Error", Description: "Unknown Internal Server Failure, Please inquiry the status or retry the process again"},
	{HTTPCode: "504", ServiceCode: "any", CaseCode: "00", Message: "Timeout", Description: "Timeout from the issuer"},
}

// sandboxRule is one sandbox-simulation scenario. Scenario is the stable
// flag value (--scenario); Method scopes it to a product surface (--method).
type sandboxRule struct {
	Scenario string `json:"scenario"`
	Method   string `json:"method"`
	Title    string `json:"title"`
	HowTo    string `json:"how_to"`
	Magic    string `json:"magic"`
}

// sandboxRules is the full set of sandbox-simulation scenarios extracted from
// the "Simulating Test Pay Outs" and "Simulating Test Payments" docs.
var sandboxRules = []sandboxRule{
	{
		Scenario: "valid-account",
		Method:   "bank-transfer",
		Title:    "Valid bank account (account validation)",
		Magic:    "beneficiaryAccountNo = even number",
		HowTo:    "Use an even number in beneficiaryAccountNo to make Bank Account Validation return a valid account.",
	},
	{
		Scenario: "invalid-account",
		Method:   "bank-transfer",
		Title:    "Invalid bank account (account validation)",
		Magic:    "beneficiaryAccountNo = odd number",
		HowTo:    "Use an odd number in beneficiaryAccountNo to make Bank Account Validation return an invalid account.",
	},
	{
		Scenario: "valid-account",
		Method:   "ewallet",
		Title:    "Valid e-wallet account (account validation)",
		Magic:    "customerNumber = even number",
		HowTo:    "Use an even number in customerNumber to make E-Wallet Account Validation return a valid account.",
	},
	{
		Scenario: "invalid-account",
		Method:   "ewallet",
		Title:    "Invalid e-wallet account (account validation)",
		Magic:    "customerNumber = odd number",
		HowTo:    "Use an odd number in customerNumber to make E-Wallet Account Validation return an invalid account.",
	},
	{
		Scenario: "pending",
		Method:   "bank-transfer",
		Title:    "Accepted transaction - processing (bank transfer)",
		Magic:    "beneficiaryAccountNo = even number",
		HowTo:    "Use an even number in beneficiaryAccountNo. The transfer is accepted and stays in 'processing'; the callback/inquiry status reflects this until you mark it success in the Simulator.",
	},
	{
		Scenario: "failed",
		Method:   "bank-transfer",
		Title:    "Failed transaction (bank transfer)",
		Magic:    "beneficiaryAccountNo = odd number",
		HowTo:    "Use an odd number in beneficiaryAccountNo to force the bank transfer to fail; the callback/inquiry status reflects the failed transaction.",
	},
	{
		Scenario: "pending",
		Method:   "ewallet",
		Title:    "Accepted transaction - processing (e-wallet transfer)",
		Magic:    "customerNumber = even number",
		HowTo:    "Use an even number in customerNumber. The e-wallet transfer is accepted and stays in 'processing' until you mark it success in the Simulator.",
	},
	{
		Scenario: "failed",
		Method:   "ewallet",
		Title:    "Failed transaction (e-wallet transfer)",
		Magic:    "customerNumber = odd number",
		HowTo:    "Use an odd number in customerNumber to force the e-wallet transfer to fail; the callback/inquiry status reflects the failed transaction.",
	},
	{
		Scenario: "success",
		Method:   "bank-transfer",
		Title:    "Success transaction (bank transfer)",
		Magic:    "Simulator → Disbursements → Mark as success",
		HowTo:    "Sandbox cannot auto-succeed a transfer. Create the (even-account) transfer, then in Sandbox Mode open Simulator → Disbursements and click 'Mark as success'. A success callback is then sent to your webhook.",
	},
	{
		Scenario: "success",
		Method:   "ewallet",
		Title:    "Success transaction (e-wallet transfer)",
		Magic:    "Simulator → Disbursements → Mark as success",
		HowTo:    "Sandbox cannot auto-succeed a transfer. Create the (even-account) transfer, then in Sandbox Mode open Simulator → Disbursements and click 'Mark as success'.",
	},
	{
		Scenario: "success",
		Method:   "payment",
		Title:    "Mark a sandbox payment as success",
		Magic:    "Simulator → mark status = success",
		HowTo:    "Create a payment in sandbox (same request body and endpoints as LIVE, only the API key differs), then in Sandbox Mode open the Simulator page and manually mark the transaction's status. A matching payment webhook is sent to your configured URL.",
	},
	{
		Scenario: "failed",
		Method:   "payment",
		Title:    "Mark a sandbox payment as failed",
		Magic:    "Simulator → mark status = failed",
		HowTo:    "Create a payment in sandbox, then in Sandbox Mode open the Simulator page and manually mark the transaction's status as failed. A matching payment webhook is sent to your configured URL.",
	},
	{
		Scenario: "pending",
		Method:   "payment",
		Title:    "Mark a sandbox payment as pending/processing",
		Magic:    "Simulator → mark status = processing",
		HowTo:    "Create a payment in sandbox, then use the Simulator page to mark the transaction's status as processing.",
	},
}

// lookupSnapCase finds the response-code row matching the given service and
// case code. A row with ServiceCode "any" matches every service. An exact
// service-code match wins over an "any" match. Returns false when no row
// matches the http+case combination.
func lookupSnapCase(httpCode, serviceCode, caseCode string) (snapCase, bool) {
	var anyMatch snapCase
	var foundAny bool
	for _, c := range snapCases {
		if c.HTTPCode != httpCode || c.CaseCode != caseCode {
			continue
		}
		if c.ServiceCode == serviceCode {
			return c, true
		}
		if c.ServiceCode == "any" {
			anyMatch = c
			foundAny = true
		}
	}
	return anyMatch, foundAny
}

// lookupSnapService returns the service row for a 2-digit service code.
func lookupSnapService(serviceCode string) (snapService, bool) {
	for _, s := range snapServices {
		if s.Code == serviceCode {
			return s, true
		}
	}
	return snapService{}, false
}
