// Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mvanhorn/printing-press-library/library/marketing/bing-ads/internal/cliutil"
	"github.com/spf13/cobra"
)

func newCustomerManagementSignupCustomerCmd(flags *rootFlags) *cobra.Command {
	var bodyAccountAccountFinancialStatus string
	var bodyAccountAccountLifeCycleStatus string
	var bodyAccountAccountMode string
	var bodyAccountAutoTagType string
	var bodyAccountBackUpPaymentInstrumentId string
	var bodyAccountBillToCustomerId string
	var bodyAccountBillingThresholdAmount float64
	var bodyAccountBusinessAddressCityName string
	var bodyAccountBusinessAddressCountryCode string
	var bodyAccountBusinessAddressPostalCode string
	var bodyAccountBusinessAddressProvinceCode string
	var bodyAccountBusinessAddressProvinceName string
	var bodyAccountBusinessAddressStreetAddress string
	var bodyAccountBusinessAddressStreetAddress2 string
	var bodyAccountCurrencyCode string
	var bodyAccountForwardCompatibilityMap string
	var bodyAccountId string
	var bodyAccountLanguage string
	var bodyAccountLastModifiedByUserId string
	var bodyAccountLastModifiedTime string
	var bodyAccountLinkedAgencies string
	var bodyAccountName string
	var bodyAccountNumber string
	var bodyAccountParentCustomerId string
	var bodyAccountPauseReason int
	var bodyAccountPaymentMethodId string
	var bodyAccountPaymentMethodType string
	var bodyAccountPrimaryUserId string
	var bodyAccountSalesHouseCustomerId string
	var bodyAccountSoldToPaymentInstrumentId string
	var bodyAccountTaxCertificateStatus string
	var bodyAccountTaxCertificateTaxCertificateBlobContainerName string
	var bodyAccountTaxCertificateTaxCertificates string
	var bodyAccountTaxInformation string
	var bodyAccountTimeStamp string
	var bodyAccountTimeZone string
	var bodyCustomerCustomerAddressCityName string
	var bodyCustomerCustomerAddressCountryCode string
	var bodyCustomerCustomerAddressPostalCode string
	var bodyCustomerCustomerAddressProvinceCode string
	var bodyCustomerCustomerAddressProvinceName string
	var bodyCustomerCustomerAddressStreetAddress string
	var bodyCustomerCustomerAddressStreetAddress2 string
	var bodyCustomerCustomerFinancialStatus string
	var bodyCustomerCustomerLifeCycleStatus string
	var bodyCustomerForwardCompatibilityMap string
	var bodyCustomerId string
	var bodyCustomerIndustry string
	var bodyCustomerLastModifiedByUserId string
	var bodyCustomerLastModifiedTime string
	var bodyCustomerMarketCountry string
	var bodyCustomerMarketLanguage string
	var bodyCustomerName string
	var bodyCustomerNumber string
	var bodyCustomerServiceLevel string
	var bodyCustomerTimeStamp string
	var bodyParentCustomerId string
	var bodyUserAuthenticationToken string
	var bodyUserContactInfoContactByPhone bool
	var bodyUserContactInfoContactByPostalMail bool
	var bodyUserContactInfoEmail string
	var bodyUserContactInfoEmailFormat string
	var bodyUserContactInfoFax string
	var bodyUserContactInfoHomePhone string
	var bodyUserContactInfoId string
	var bodyUserContactInfoMobile string
	var bodyUserContactInfoPhone1 string
	var bodyUserContactInfoPhone2 string
	var bodyUserCustomerId string
	var bodyUserForwardCompatibilityMap string
	var bodyUserId string
	var bodyUserJobTitle string
	var bodyUserLastModifiedByUserId string
	var bodyUserLastModifiedTime string
	var bodyUserLcid string
	var bodyUserNameFirstName string
	var bodyUserNameLastName string
	var bodyUserNameMiddleInitial string
	var bodyUserPassword string
	var bodyUserSecretAnswer string
	var bodyUserSecretQuestion string
	var bodyUserTimeStamp string
	var bodyUserUserLifeCycleStatus string
	var bodyUserUserName string
	var bodyUserId2 string
	var bodyUserInvitationAccountIds string
	var bodyUserInvitationCustomerId string
	var bodyUserInvitationEmail string
	var bodyUserInvitationExpirationDate string
	var bodyUserInvitationFirstName string
	var bodyUserInvitationId string
	var bodyUserInvitationLastName string
	var bodyUserInvitationLcid string
	var bodyUserInvitationRoleId int
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "signup-customer",
		Short:       "signup_customer",
		Example:     "  bing-ads-pp-cli customer-management signup-customer",
		Annotations: map[string]string{"pp:endpoint": "customer-management.signup-customer", "pp:method": "POST", "pp:path": "/CustomerManagement/v13/Customer/Signup"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
			}
			path := "/CustomerManagement/v13/Customer/Signup"
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			var body any
			if stdinBody {
				stdinData, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				var jsonBody map[string]any
				if err := json.Unmarshal(stdinData, &jsonBody); err != nil {
					return fmt.Errorf("parsing stdin JSON: %w", err)
				}
				body = jsonBody
			} else {
				bodyMap := map[string]any{}
				body = bodyMap
				{
					nestedAccount := map[string]any{}
					if cmd.Flags().Changed("account-account-financial-status") || bodyAccountAccountFinancialStatus != "" {
						nestedAccount["AccountFinancialStatus"] = bodyAccountAccountFinancialStatus
					}
					if cmd.Flags().Changed("account-account-life-cycle-status") || bodyAccountAccountLifeCycleStatus != "" {
						nestedAccount["AccountLifeCycleStatus"] = bodyAccountAccountLifeCycleStatus
					}
					if cmd.Flags().Changed("account-account-mode") || bodyAccountAccountMode != "" {
						nestedAccount["AccountMode"] = bodyAccountAccountMode
					}
					if cmd.Flags().Changed("account-auto-tag-type") || bodyAccountAutoTagType != "" {
						nestedAccount["AutoTagType"] = bodyAccountAutoTagType
					}
					if cmd.Flags().Changed("account-back-up-payment-instrument-id") || bodyAccountBackUpPaymentInstrumentId != "" {
						nestedAccount["BackUpPaymentInstrumentId"] = bodyAccountBackUpPaymentInstrumentId
					}
					if cmd.Flags().Changed("account-bill-to-customer-id") || bodyAccountBillToCustomerId != "" {
						nestedAccount["BillToCustomerId"] = bodyAccountBillToCustomerId
					}
					if cmd.Flags().Changed("account-billing-threshold-amount") || bodyAccountBillingThresholdAmount != 0.0 {
						nestedAccount["BillingThresholdAmount"] = bodyAccountBillingThresholdAmount
					}
					{
						nestedAccountBusinessAddress := map[string]any{}
						if cmd.Flags().Changed("account-business-address-city-name") || bodyAccountBusinessAddressCityName != "" {
							nestedAccountBusinessAddress["CityName"] = bodyAccountBusinessAddressCityName
						}
						if cmd.Flags().Changed("account-business-address-country-code") || bodyAccountBusinessAddressCountryCode != "" {
							nestedAccountBusinessAddress["CountryCode"] = bodyAccountBusinessAddressCountryCode
						}
						if cmd.Flags().Changed("account-business-address-postal-code") || bodyAccountBusinessAddressPostalCode != "" {
							nestedAccountBusinessAddress["PostalCode"] = bodyAccountBusinessAddressPostalCode
						}
						if cmd.Flags().Changed("account-business-address-province-code") || bodyAccountBusinessAddressProvinceCode != "" {
							nestedAccountBusinessAddress["ProvinceCode"] = bodyAccountBusinessAddressProvinceCode
						}
						if cmd.Flags().Changed("account-business-address-province-name") || bodyAccountBusinessAddressProvinceName != "" {
							nestedAccountBusinessAddress["ProvinceName"] = bodyAccountBusinessAddressProvinceName
						}
						if cmd.Flags().Changed("account-business-address-street-address") || bodyAccountBusinessAddressStreetAddress != "" {
							nestedAccountBusinessAddress["StreetAddress"] = bodyAccountBusinessAddressStreetAddress
						}
						if cmd.Flags().Changed("account-business-address-street-address2") || bodyAccountBusinessAddressStreetAddress2 != "" {
							nestedAccountBusinessAddress["StreetAddress2"] = bodyAccountBusinessAddressStreetAddress2
						}
						if len(nestedAccountBusinessAddress) > 0 {
							nestedAccount["BusinessAddress"] = nestedAccountBusinessAddress
						}
					}
					if cmd.Flags().Changed("account-currency-code") || bodyAccountCurrencyCode != "" {
						nestedAccount["CurrencyCode"] = bodyAccountCurrencyCode
					}
					if cmd.Flags().Changed("account-forward-compatibility-map") || bodyAccountForwardCompatibilityMap != "" {
						var parsedAccountForwardCompatibilityMap any
						if err := json.Unmarshal([]byte(bodyAccountForwardCompatibilityMap), &parsedAccountForwardCompatibilityMap); err != nil {
							return fmt.Errorf("parsing --account-forward-compatibility-map JSON: %w", err)
						}
						asArray, ok := parsedAccountForwardCompatibilityMap.([]any)
						if !ok {
							return fmt.Errorf("--account-forward-compatibility-map must be a JSON array, got JSON %T", parsedAccountForwardCompatibilityMap)
						}
						nestedAccount["ForwardCompatibilityMap"] = asArray
					}
					if cmd.Flags().Changed("account-id") || bodyAccountId != "" {
						nestedAccount["Id"] = bodyAccountId
					}
					if cmd.Flags().Changed("account-language") || bodyAccountLanguage != "" {
						nestedAccount["Language"] = bodyAccountLanguage
					}
					if cmd.Flags().Changed("account-last-modified-by-user-id") || bodyAccountLastModifiedByUserId != "" {
						nestedAccount["LastModifiedByUserId"] = bodyAccountLastModifiedByUserId
					}
					if cmd.Flags().Changed("account-last-modified-time") || bodyAccountLastModifiedTime != "" {
						nestedAccount["LastModifiedTime"] = bodyAccountLastModifiedTime
					}
					if cmd.Flags().Changed("account-linked-agencies") || bodyAccountLinkedAgencies != "" {
						var parsedAccountLinkedAgencies any
						if err := json.Unmarshal([]byte(bodyAccountLinkedAgencies), &parsedAccountLinkedAgencies); err != nil {
							return fmt.Errorf("parsing --account-linked-agencies JSON: %w", err)
						}
						asArray, ok := parsedAccountLinkedAgencies.([]any)
						if !ok {
							return fmt.Errorf("--account-linked-agencies must be a JSON array, got JSON %T", parsedAccountLinkedAgencies)
						}
						nestedAccount["LinkedAgencies"] = asArray
					}
					if cmd.Flags().Changed("account-name") || bodyAccountName != "" {
						nestedAccount["Name"] = bodyAccountName
					}
					if cmd.Flags().Changed("account-number") || bodyAccountNumber != "" {
						nestedAccount["Number"] = bodyAccountNumber
					}
					if cmd.Flags().Changed("account-parent-customer-id") || bodyAccountParentCustomerId != "" {
						nestedAccount["ParentCustomerId"] = bodyAccountParentCustomerId
					}
					if cmd.Flags().Changed("account-pause-reason") || bodyAccountPauseReason != 0 {
						nestedAccount["PauseReason"] = bodyAccountPauseReason
					}
					if cmd.Flags().Changed("account-payment-method-id") || bodyAccountPaymentMethodId != "" {
						nestedAccount["PaymentMethodId"] = bodyAccountPaymentMethodId
					}
					if cmd.Flags().Changed("account-payment-method-type") || bodyAccountPaymentMethodType != "" {
						nestedAccount["PaymentMethodType"] = bodyAccountPaymentMethodType
					}
					if cmd.Flags().Changed("account-primary-user-id") || bodyAccountPrimaryUserId != "" {
						nestedAccount["PrimaryUserId"] = bodyAccountPrimaryUserId
					}
					if cmd.Flags().Changed("account-sales-house-customer-id") || bodyAccountSalesHouseCustomerId != "" {
						nestedAccount["SalesHouseCustomerId"] = bodyAccountSalesHouseCustomerId
					}
					if cmd.Flags().Changed("account-sold-to-payment-instrument-id") || bodyAccountSoldToPaymentInstrumentId != "" {
						nestedAccount["SoldToPaymentInstrumentId"] = bodyAccountSoldToPaymentInstrumentId
					}
					{
						nestedAccountTaxCertificate := map[string]any{}
						if cmd.Flags().Changed("account-tax-certificate-status") || bodyAccountTaxCertificateStatus != "" {
							nestedAccountTaxCertificate["Status"] = bodyAccountTaxCertificateStatus
						}
						if cmd.Flags().Changed("account-tax-certificate-tax-certificate-blob-container-name") || bodyAccountTaxCertificateTaxCertificateBlobContainerName != "" {
							nestedAccountTaxCertificate["TaxCertificateBlobContainerName"] = bodyAccountTaxCertificateTaxCertificateBlobContainerName
						}
						if cmd.Flags().Changed("account-tax-certificate-tax-certificates") || bodyAccountTaxCertificateTaxCertificates != "" {
							var parsedAccountTaxCertificateTaxCertificates any
							if err := json.Unmarshal([]byte(bodyAccountTaxCertificateTaxCertificates), &parsedAccountTaxCertificateTaxCertificates); err != nil {
								return fmt.Errorf("parsing --account-tax-certificate-tax-certificates JSON: %w", err)
							}
							asArray, ok := parsedAccountTaxCertificateTaxCertificates.([]any)
							if !ok {
								return fmt.Errorf("--account-tax-certificate-tax-certificates must be a JSON array, got JSON %T", parsedAccountTaxCertificateTaxCertificates)
							}
							nestedAccountTaxCertificate["TaxCertificates"] = asArray
						}
						if len(nestedAccountTaxCertificate) > 0 {
							nestedAccount["TaxCertificate"] = nestedAccountTaxCertificate
						}
					}
					if cmd.Flags().Changed("account-tax-information") || bodyAccountTaxInformation != "" {
						var parsedAccountTaxInformation any
						if err := json.Unmarshal([]byte(bodyAccountTaxInformation), &parsedAccountTaxInformation); err != nil {
							return fmt.Errorf("parsing --account-tax-information JSON: %w", err)
						}
						asArray, ok := parsedAccountTaxInformation.([]any)
						if !ok {
							return fmt.Errorf("--account-tax-information must be a JSON array, got JSON %T", parsedAccountTaxInformation)
						}
						nestedAccount["TaxInformation"] = asArray
					}
					if cmd.Flags().Changed("account-time-stamp") || bodyAccountTimeStamp != "" {
						nestedAccount["TimeStamp"] = bodyAccountTimeStamp
					}
					if cmd.Flags().Changed("account-time-zone") || bodyAccountTimeZone != "" {
						nestedAccount["TimeZone"] = bodyAccountTimeZone
					}
					if len(nestedAccount) > 0 {
						bodyMap["Account"] = nestedAccount
					}
				}
				{
					nestedCustomer := map[string]any{}
					{
						nestedCustomerCustomerAddress := map[string]any{}
						if cmd.Flags().Changed("customer-customer-address-city-name") || bodyCustomerCustomerAddressCityName != "" {
							nestedCustomerCustomerAddress["CityName"] = bodyCustomerCustomerAddressCityName
						}
						if cmd.Flags().Changed("customer-customer-address-country-code") || bodyCustomerCustomerAddressCountryCode != "" {
							nestedCustomerCustomerAddress["CountryCode"] = bodyCustomerCustomerAddressCountryCode
						}
						if cmd.Flags().Changed("customer-customer-address-postal-code") || bodyCustomerCustomerAddressPostalCode != "" {
							nestedCustomerCustomerAddress["PostalCode"] = bodyCustomerCustomerAddressPostalCode
						}
						if cmd.Flags().Changed("customer-customer-address-province-code") || bodyCustomerCustomerAddressProvinceCode != "" {
							nestedCustomerCustomerAddress["ProvinceCode"] = bodyCustomerCustomerAddressProvinceCode
						}
						if cmd.Flags().Changed("customer-customer-address-province-name") || bodyCustomerCustomerAddressProvinceName != "" {
							nestedCustomerCustomerAddress["ProvinceName"] = bodyCustomerCustomerAddressProvinceName
						}
						if cmd.Flags().Changed("customer-customer-address-street-address") || bodyCustomerCustomerAddressStreetAddress != "" {
							nestedCustomerCustomerAddress["StreetAddress"] = bodyCustomerCustomerAddressStreetAddress
						}
						if cmd.Flags().Changed("customer-customer-address-street-address2") || bodyCustomerCustomerAddressStreetAddress2 != "" {
							nestedCustomerCustomerAddress["StreetAddress2"] = bodyCustomerCustomerAddressStreetAddress2
						}
						if len(nestedCustomerCustomerAddress) > 0 {
							nestedCustomer["CustomerAddress"] = nestedCustomerCustomerAddress
						}
					}
					if cmd.Flags().Changed("customer-customer-financial-status") || bodyCustomerCustomerFinancialStatus != "" {
						nestedCustomer["CustomerFinancialStatus"] = bodyCustomerCustomerFinancialStatus
					}
					if cmd.Flags().Changed("customer-customer-life-cycle-status") || bodyCustomerCustomerLifeCycleStatus != "" {
						nestedCustomer["CustomerLifeCycleStatus"] = bodyCustomerCustomerLifeCycleStatus
					}
					if cmd.Flags().Changed("customer-forward-compatibility-map") || bodyCustomerForwardCompatibilityMap != "" {
						var parsedCustomerForwardCompatibilityMap any
						if err := json.Unmarshal([]byte(bodyCustomerForwardCompatibilityMap), &parsedCustomerForwardCompatibilityMap); err != nil {
							return fmt.Errorf("parsing --customer-forward-compatibility-map JSON: %w", err)
						}
						asArray, ok := parsedCustomerForwardCompatibilityMap.([]any)
						if !ok {
							return fmt.Errorf("--customer-forward-compatibility-map must be a JSON array, got JSON %T", parsedCustomerForwardCompatibilityMap)
						}
						nestedCustomer["ForwardCompatibilityMap"] = asArray
					}
					if cmd.Flags().Changed("customer-id") || bodyCustomerId != "" {
						nestedCustomer["Id"] = bodyCustomerId
					}
					if cmd.Flags().Changed("customer-industry") || bodyCustomerIndustry != "" {
						nestedCustomer["Industry"] = bodyCustomerIndustry
					}
					if cmd.Flags().Changed("customer-last-modified-by-user-id") || bodyCustomerLastModifiedByUserId != "" {
						nestedCustomer["LastModifiedByUserId"] = bodyCustomerLastModifiedByUserId
					}
					if cmd.Flags().Changed("customer-last-modified-time") || bodyCustomerLastModifiedTime != "" {
						nestedCustomer["LastModifiedTime"] = bodyCustomerLastModifiedTime
					}
					if cmd.Flags().Changed("customer-market-country") || bodyCustomerMarketCountry != "" {
						nestedCustomer["MarketCountry"] = bodyCustomerMarketCountry
					}
					if cmd.Flags().Changed("customer-market-language") || bodyCustomerMarketLanguage != "" {
						nestedCustomer["MarketLanguage"] = bodyCustomerMarketLanguage
					}
					if cmd.Flags().Changed("customer-name") || bodyCustomerName != "" {
						nestedCustomer["Name"] = bodyCustomerName
					}
					if cmd.Flags().Changed("customer-number") || bodyCustomerNumber != "" {
						nestedCustomer["Number"] = bodyCustomerNumber
					}
					if cmd.Flags().Changed("customer-service-level") || bodyCustomerServiceLevel != "" {
						nestedCustomer["ServiceLevel"] = bodyCustomerServiceLevel
					}
					if cmd.Flags().Changed("customer-time-stamp") || bodyCustomerTimeStamp != "" {
						nestedCustomer["TimeStamp"] = bodyCustomerTimeStamp
					}
					if len(nestedCustomer) > 0 {
						bodyMap["Customer"] = nestedCustomer
					}
				}
				if cmd.Flags().Changed("parent-customer-id") || bodyParentCustomerId != "" {
					bodyMap["ParentCustomerId"] = bodyParentCustomerId
				}
				{
					nestedUser := map[string]any{}
					if cmd.Flags().Changed("user-authentication-token") || bodyUserAuthenticationToken != "" {
						nestedUser["AuthenticationToken"] = bodyUserAuthenticationToken
					}
					{
						nestedUserContactInfo := map[string]any{}
						if cmd.Flags().Changed("user-contact-info-contact-by-phone") {
							nestedUserContactInfo["ContactByPhone"] = bodyUserContactInfoContactByPhone
						}
						if cmd.Flags().Changed("user-contact-info-contact-by-postal-mail") {
							nestedUserContactInfo["ContactByPostalMail"] = bodyUserContactInfoContactByPostalMail
						}
						if cmd.Flags().Changed("user-contact-info-email") || bodyUserContactInfoEmail != "" {
							nestedUserContactInfo["Email"] = bodyUserContactInfoEmail
						}
						if cmd.Flags().Changed("user-contact-info-email-format") || bodyUserContactInfoEmailFormat != "" {
							nestedUserContactInfo["EmailFormat"] = bodyUserContactInfoEmailFormat
						}
						if cmd.Flags().Changed("user-contact-info-fax") || bodyUserContactInfoFax != "" {
							nestedUserContactInfo["Fax"] = bodyUserContactInfoFax
						}
						if cmd.Flags().Changed("user-contact-info-home-phone") || bodyUserContactInfoHomePhone != "" {
							nestedUserContactInfo["HomePhone"] = bodyUserContactInfoHomePhone
						}
						if cmd.Flags().Changed("user-contact-info-id") || bodyUserContactInfoId != "" {
							nestedUserContactInfo["Id"] = bodyUserContactInfoId
						}
						if cmd.Flags().Changed("user-contact-info-mobile") || bodyUserContactInfoMobile != "" {
							nestedUserContactInfo["Mobile"] = bodyUserContactInfoMobile
						}
						if cmd.Flags().Changed("user-contact-info-phone1") || bodyUserContactInfoPhone1 != "" {
							nestedUserContactInfo["Phone1"] = bodyUserContactInfoPhone1
						}
						if cmd.Flags().Changed("user-contact-info-phone2") || bodyUserContactInfoPhone2 != "" {
							nestedUserContactInfo["Phone2"] = bodyUserContactInfoPhone2
						}
						if len(nestedUserContactInfo) > 0 {
							nestedUser["ContactInfo"] = nestedUserContactInfo
						}
					}
					if cmd.Flags().Changed("user-customer-id") || bodyUserCustomerId != "" {
						nestedUser["CustomerId"] = bodyUserCustomerId
					}
					if cmd.Flags().Changed("user-forward-compatibility-map") || bodyUserForwardCompatibilityMap != "" {
						var parsedUserForwardCompatibilityMap any
						if err := json.Unmarshal([]byte(bodyUserForwardCompatibilityMap), &parsedUserForwardCompatibilityMap); err != nil {
							return fmt.Errorf("parsing --user-forward-compatibility-map JSON: %w", err)
						}
						asArray, ok := parsedUserForwardCompatibilityMap.([]any)
						if !ok {
							return fmt.Errorf("--user-forward-compatibility-map must be a JSON array, got JSON %T", parsedUserForwardCompatibilityMap)
						}
						nestedUser["ForwardCompatibilityMap"] = asArray
					}
					if cmd.Flags().Changed("user-id") || bodyUserId != "" {
						nestedUser["Id"] = bodyUserId
					}
					if cmd.Flags().Changed("user-job-title") || bodyUserJobTitle != "" {
						nestedUser["JobTitle"] = bodyUserJobTitle
					}
					if cmd.Flags().Changed("user-last-modified-by-user-id") || bodyUserLastModifiedByUserId != "" {
						nestedUser["LastModifiedByUserId"] = bodyUserLastModifiedByUserId
					}
					if cmd.Flags().Changed("user-last-modified-time") || bodyUserLastModifiedTime != "" {
						nestedUser["LastModifiedTime"] = bodyUserLastModifiedTime
					}
					if cmd.Flags().Changed("user-lcid") || bodyUserLcid != "" {
						nestedUser["Lcid"] = bodyUserLcid
					}
					{
						nestedUserName := map[string]any{}
						if cmd.Flags().Changed("user-name-first-name") || bodyUserNameFirstName != "" {
							nestedUserName["FirstName"] = bodyUserNameFirstName
						}
						if cmd.Flags().Changed("user-name-last-name") || bodyUserNameLastName != "" {
							nestedUserName["LastName"] = bodyUserNameLastName
						}
						if cmd.Flags().Changed("user-name-middle-initial") || bodyUserNameMiddleInitial != "" {
							nestedUserName["MiddleInitial"] = bodyUserNameMiddleInitial
						}
						if len(nestedUserName) > 0 {
							nestedUser["Name"] = nestedUserName
						}
					}
					if cmd.Flags().Changed("user-password") || bodyUserPassword != "" {
						nestedUser["Password"] = bodyUserPassword
					}
					if cmd.Flags().Changed("user-secret-answer") || bodyUserSecretAnswer != "" {
						nestedUser["SecretAnswer"] = bodyUserSecretAnswer
					}
					if cmd.Flags().Changed("user-secret-question") || bodyUserSecretQuestion != "" {
						nestedUser["SecretQuestion"] = bodyUserSecretQuestion
					}
					if cmd.Flags().Changed("user-time-stamp") || bodyUserTimeStamp != "" {
						nestedUser["TimeStamp"] = bodyUserTimeStamp
					}
					if cmd.Flags().Changed("user-user-life-cycle-status") || bodyUserUserLifeCycleStatus != "" {
						nestedUser["UserLifeCycleStatus"] = bodyUserUserLifeCycleStatus
					}
					if cmd.Flags().Changed("user-user-name") || bodyUserUserName != "" {
						nestedUser["UserName"] = bodyUserUserName
					}
					if len(nestedUser) > 0 {
						bodyMap["User"] = nestedUser
					}
				}
				if cmd.Flags().Changed("user-id-2") || bodyUserId2 != "" {
					bodyMap["UserId"] = bodyUserId2
				}
				{
					nestedUserInvitation := map[string]any{}
					if cmd.Flags().Changed("user-invitation-account-ids") {
						parsedUserInvitationAccountIds, parseErr := cliutil.ParseStringList(bodyUserInvitationAccountIds)
						if parseErr != nil {
							return fmt.Errorf("parsing --user-invitation-account-ids list: %w", parseErr)
						}
						nestedUserInvitation["AccountIds"] = parsedUserInvitationAccountIds
					}
					if cmd.Flags().Changed("user-invitation-customer-id") || bodyUserInvitationCustomerId != "" {
						nestedUserInvitation["CustomerId"] = bodyUserInvitationCustomerId
					}
					if cmd.Flags().Changed("user-invitation-email") || bodyUserInvitationEmail != "" {
						nestedUserInvitation["Email"] = bodyUserInvitationEmail
					}
					if cmd.Flags().Changed("user-invitation-expiration-date") || bodyUserInvitationExpirationDate != "" {
						nestedUserInvitation["ExpirationDate"] = bodyUserInvitationExpirationDate
					}
					if cmd.Flags().Changed("user-invitation-first-name") || bodyUserInvitationFirstName != "" {
						nestedUserInvitation["FirstName"] = bodyUserInvitationFirstName
					}
					if cmd.Flags().Changed("user-invitation-id") || bodyUserInvitationId != "" {
						nestedUserInvitation["Id"] = bodyUserInvitationId
					}
					if cmd.Flags().Changed("user-invitation-last-name") || bodyUserInvitationLastName != "" {
						nestedUserInvitation["LastName"] = bodyUserInvitationLastName
					}
					if cmd.Flags().Changed("user-invitation-lcid") || bodyUserInvitationLcid != "" {
						nestedUserInvitation["Lcid"] = bodyUserInvitationLcid
					}
					if cmd.Flags().Changed("user-invitation-role-id") || bodyUserInvitationRoleId != 0 {
						nestedUserInvitation["RoleId"] = bodyUserInvitationRoleId
					}
					if len(nestedUserInvitation) > 0 {
						bodyMap["UserInvitation"] = nestedUserInvitation
					}
				}
			}
			data, statusCode, err := c.PostWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			// Inspect the mutate response body for a partial-failure-shaped
			// field (e.g. Google Ads `partialFailureError`). Several Google
			// APIs return 200 OK with a partial-failure field when some
			// operations in the batch failed; ignoring it silently swallows
			// real failures. Detection runs before output-mode selection so
			// the exit code is consistent regardless of how stdout is
			// rendered. --dry-run short-circuits because no real request
			// was sent.
			var partialFailure *partialFailureReport
			if !flags.dryRun && statusCode >= 200 && statusCode < 300 {
				partialFailure = detectPartialFailure(data)
				if partialFailure != nil {
					fmt.Fprintf(os.Stderr, "warning: partial failure detected in %s response: %s\n", "customer-management", partialFailure.Message)
					if len(partialFailure.ResourceNames) > 0 {
						fmt.Fprintf(os.Stderr, "         succeeded: %d operation(s)\n", len(partialFailure.ResourceNames))
					}
				}
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				// Check if response contains an array (directly or wrapped in "data")
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
					} else {
						if partialFailure != nil && !flags.allowPartialFailure {
							return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "customer-management", partialFailure.Message))
						}
						return nil
					}
				} else {
					var wrapped struct {
						Data []map[string]any `json:"data"`
					}
					if json.Unmarshal(data, &wrapped) == nil && len(wrapped.Data) > 0 {
						if err := printAutoTable(cmd.OutOrStdout(), wrapped.Data); err != nil {
							fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
						} else {
							if partialFailure != nil && !flags.allowPartialFailure {
								return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "customer-management", partialFailure.Message))
							}
							return nil
						}
					}
				}
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				if flags.quiet {
					if partialFailure != nil && !flags.allowPartialFailure {
						return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "customer-management", partialFailure.Message))
					}
					return nil
				}
				envelope := map[string]any{
					"action":   "post",
					"resource": "customer-management",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300 && (partialFailure == nil || flags.allowPartialFailure),
				}
				if flags.agent {
					envelope["meta"] = map[string]any{"source": "live"}
				}
				if partialFailure != nil {
					envelope["partial_failure"] = partialFailure
				}
				if flags.dryRun {
					envelope["dry_run"] = true
					envelope["status"] = 0
					envelope["success"] = false
				}
				// Verify-mode synthetic envelope detection runs against RAW data
				// (before --compact/--select filtering) so the sentinel field is
				// guaranteed to be visible even if the operator passes a filter
				// flag that would otherwise strip it. Surfaces a top-level
				// verify_noop signal + flips success to false. Mirrors the dry_run
				// shape above.
				if len(data) > 0 {
					var rawParsed any
					if err := json.Unmarshal(data, &rawParsed); err == nil {
						if m, ok := rawParsed.(map[string]any); ok {
							if v, ok := m["__pp_verify_synthetic__"].(bool); ok && v {
								envelope["verify_noop"] = true
								envelope["success"] = false
							}
						}
					}
				}
				// Apply --compact and --select to the API response before wrapping.
				// --select wins when both are set: explicit field choice trumps the
				// generic high-gravity allow-list. Otherwise --compact still applies
				// when --agent is on but the user did not name fields.
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered, map[string]bool{"AccountId": true, "AccountNumber": true, "CreateTime": true, "CustomerId": true, "CustomerNumber": true})
				}
				if len(filtered) > 0 {
					var parsed any
					if err := json.Unmarshal(filtered, &parsed); err == nil {
						if flags.agent {
							envelope["results"] = parsed
						} else {
							envelope["data"] = parsed
						}
					}
				}
				envelopeJSON, err := json.Marshal(envelope)
				if err != nil {
					return err
				}
				resultKey := "data"
				if flags.agent {
					resultKey = "results"
				}
				structured, err := wrapPlatformStructuredOutput(json.RawMessage(envelopeJSON), flags, resultKey, true)
				if err != nil {
					return err
				}
				if perr := printOutput(cmd.OutOrStdout(), structured, true); perr != nil {
					return perr
				}
				if partialFailure != nil && !flags.allowPartialFailure {
					return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "customer-management", partialFailure.Message))
				}
				return nil
			}
			// Fall-through for mutate paths that did not hit the table or
			// asJSON branches: --quiet, --csv, --plain, and default terminal
			// raw output. printOutputWithFlags renders the body, then the
			// typed partial-failure exit fires unless --allow-partial-failure
			// downgrades it. Without this guard a partial failure would exit
			// 0 for these output modes — the exact silent-swallow regression
			// the surrounding patch is preventing for asJSON / piped output.
			if perr := printOutputWithFlags(cmd.OutOrStdout(), data, flags); perr != nil {
				return perr
			}
			if partialFailure != nil && !flags.allowPartialFailure {
				return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "customer-management", partialFailure.Message))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&bodyAccountAccountFinancialStatus, "account-account-financial-status", "", "Account financial status")
	cmd.Flags().StringVar(&bodyAccountAccountLifeCycleStatus, "account-account-life-cycle-status", "", "Account life cycle status")
	cmd.Flags().StringVar(&bodyAccountAccountMode, "account-account-mode", "", "Account mode")
	cmd.Flags().StringVar(&bodyAccountAutoTagType, "account-auto-tag-type", "", "Auto tag type")
	cmd.Flags().StringVar(&bodyAccountBackUpPaymentInstrumentId, "account-back-up-payment-instrument-id", "", "Back up payment instrument id")
	cmd.Flags().StringVar(&bodyAccountBillToCustomerId, "account-bill-to-customer-id", "", "Bill to customer id")
	cmd.Flags().Float64Var(&bodyAccountBillingThresholdAmount, "account-billing-threshold-amount", 0.0, "Billing threshold amount")
	cmd.Flags().StringVar(&bodyAccountBusinessAddressCityName, "account-business-address-city-name", "", "City name")
	cmd.Flags().StringVar(&bodyAccountBusinessAddressCountryCode, "account-business-address-country-code", "", "Country code")
	cmd.Flags().StringVar(&bodyAccountBusinessAddressPostalCode, "account-business-address-postal-code", "", "Postal code")
	cmd.Flags().StringVar(&bodyAccountBusinessAddressProvinceCode, "account-business-address-province-code", "", "Province code")
	cmd.Flags().StringVar(&bodyAccountBusinessAddressProvinceName, "account-business-address-province-name", "", "Province name")
	cmd.Flags().StringVar(&bodyAccountBusinessAddressStreetAddress, "account-business-address-street-address", "", "Street address")
	cmd.Flags().StringVar(&bodyAccountBusinessAddressStreetAddress2, "account-business-address-street-address2", "", "Street address2")
	cmd.Flags().StringVar(&bodyAccountCurrencyCode, "account-currency-code", "", "Currency code")
	cmd.Flags().StringVar(&bodyAccountForwardCompatibilityMap, "account-forward-compatibility-map", "", "Forward compatibility map")
	cmd.Flags().StringVar(&bodyAccountId, "account-id", "", "Id")
	cmd.Flags().StringVar(&bodyAccountLanguage, "account-language", "", "Language")
	cmd.Flags().StringVar(&bodyAccountLastModifiedByUserId, "account-last-modified-by-user-id", "", "Last modified by user id")
	cmd.Flags().StringVar(&bodyAccountLastModifiedTime, "account-last-modified-time", "", "Last modified time")
	cmd.Flags().StringVar(&bodyAccountLinkedAgencies, "account-linked-agencies", "", "Linked agencies")
	cmd.Flags().StringVar(&bodyAccountName, "account-name", "", "Name")
	cmd.Flags().StringVar(&bodyAccountNumber, "account-number", "", "Number")
	cmd.Flags().StringVar(&bodyAccountParentCustomerId, "account-parent-customer-id", "", "Parent customer id")
	cmd.Flags().IntVar(&bodyAccountPauseReason, "account-pause-reason", 0, "Pause reason")
	cmd.Flags().StringVar(&bodyAccountPaymentMethodId, "account-payment-method-id", "", "Payment method id")
	cmd.Flags().StringVar(&bodyAccountPaymentMethodType, "account-payment-method-type", "", "Payment method type")
	cmd.Flags().StringVar(&bodyAccountPrimaryUserId, "account-primary-user-id", "", "Primary user id")
	cmd.Flags().StringVar(&bodyAccountSalesHouseCustomerId, "account-sales-house-customer-id", "", "Sales house customer id")
	cmd.Flags().StringVar(&bodyAccountSoldToPaymentInstrumentId, "account-sold-to-payment-instrument-id", "", "Sold to payment instrument id")
	cmd.Flags().StringVar(&bodyAccountTaxCertificateStatus, "account-tax-certificate-status", "", "Status")
	cmd.Flags().StringVar(&bodyAccountTaxCertificateTaxCertificateBlobContainerName, "account-tax-certificate-tax-certificate-blob-container-name", "", "Tax certificate blob container name")
	cmd.Flags().StringVar(&bodyAccountTaxCertificateTaxCertificates, "account-tax-certificate-tax-certificates", "", "Tax certificates")
	cmd.Flags().StringVar(&bodyAccountTaxInformation, "account-tax-information", "", "Tax information")
	cmd.Flags().StringVar(&bodyAccountTimeStamp, "account-time-stamp", "", "Time stamp")
	cmd.Flags().StringVar(&bodyAccountTimeZone, "account-time-zone", "", "Time zone")
	cmd.Flags().StringVar(&bodyCustomerCustomerAddressCityName, "customer-customer-address-city-name", "", "City name")
	cmd.Flags().StringVar(&bodyCustomerCustomerAddressCountryCode, "customer-customer-address-country-code", "", "Country code")
	cmd.Flags().StringVar(&bodyCustomerCustomerAddressPostalCode, "customer-customer-address-postal-code", "", "Postal code")
	cmd.Flags().StringVar(&bodyCustomerCustomerAddressProvinceCode, "customer-customer-address-province-code", "", "Province code")
	cmd.Flags().StringVar(&bodyCustomerCustomerAddressProvinceName, "customer-customer-address-province-name", "", "Province name")
	cmd.Flags().StringVar(&bodyCustomerCustomerAddressStreetAddress, "customer-customer-address-street-address", "", "Street address")
	cmd.Flags().StringVar(&bodyCustomerCustomerAddressStreetAddress2, "customer-customer-address-street-address2", "", "Street address2")
	cmd.Flags().StringVar(&bodyCustomerCustomerFinancialStatus, "customer-customer-financial-status", "", "Customer financial status")
	cmd.Flags().StringVar(&bodyCustomerCustomerLifeCycleStatus, "customer-customer-life-cycle-status", "", "Customer life cycle status")
	cmd.Flags().StringVar(&bodyCustomerForwardCompatibilityMap, "customer-forward-compatibility-map", "", "Forward compatibility map")
	cmd.Flags().StringVar(&bodyCustomerId, "customer-id", "", "Id")
	cmd.Flags().StringVar(&bodyCustomerIndustry, "customer-industry", "", "Industry")
	cmd.Flags().StringVar(&bodyCustomerLastModifiedByUserId, "customer-last-modified-by-user-id", "", "Last modified by user id")
	cmd.Flags().StringVar(&bodyCustomerLastModifiedTime, "customer-last-modified-time", "", "Last modified time")
	cmd.Flags().StringVar(&bodyCustomerMarketCountry, "customer-market-country", "", "Market country")
	cmd.Flags().StringVar(&bodyCustomerMarketLanguage, "customer-market-language", "", "Market language")
	cmd.Flags().StringVar(&bodyCustomerName, "customer-name", "", "Name")
	cmd.Flags().StringVar(&bodyCustomerNumber, "customer-number", "", "Number")
	cmd.Flags().StringVar(&bodyCustomerServiceLevel, "customer-service-level", "", "Service level")
	cmd.Flags().StringVar(&bodyCustomerTimeStamp, "customer-time-stamp", "", "Time stamp")
	cmd.Flags().StringVar(&bodyParentCustomerId, "parent-customer-id", "", "Parent customer id")
	cmd.Flags().StringVar(&bodyUserAuthenticationToken, "user-authentication-token", "", "Authentication token")
	cmd.Flags().BoolVar(&bodyUserContactInfoContactByPhone, "user-contact-info-contact-by-phone", false, "Contact by phone")
	cmd.Flags().BoolVar(&bodyUserContactInfoContactByPostalMail, "user-contact-info-contact-by-postal-mail", false, "Contact by postal mail")
	cmd.Flags().StringVar(&bodyUserContactInfoEmail, "user-contact-info-email", "", "Email")
	cmd.Flags().StringVar(&bodyUserContactInfoEmailFormat, "user-contact-info-email-format", "", "Email format")
	cmd.Flags().StringVar(&bodyUserContactInfoFax, "user-contact-info-fax", "", "Fax")
	cmd.Flags().StringVar(&bodyUserContactInfoHomePhone, "user-contact-info-home-phone", "", "Home phone")
	cmd.Flags().StringVar(&bodyUserContactInfoId, "user-contact-info-id", "", "Id")
	cmd.Flags().StringVar(&bodyUserContactInfoMobile, "user-contact-info-mobile", "", "Mobile")
	cmd.Flags().StringVar(&bodyUserContactInfoPhone1, "user-contact-info-phone1", "", "Phone1")
	cmd.Flags().StringVar(&bodyUserContactInfoPhone2, "user-contact-info-phone2", "", "Phone2")
	cmd.Flags().StringVar(&bodyUserCustomerId, "user-customer-id", "", "Customer id")
	cmd.Flags().StringVar(&bodyUserForwardCompatibilityMap, "user-forward-compatibility-map", "", "Forward compatibility map")
	cmd.Flags().StringVar(&bodyUserId, "user-id", "", "Id")
	cmd.Flags().StringVar(&bodyUserJobTitle, "user-job-title", "", "Job title")
	cmd.Flags().StringVar(&bodyUserLastModifiedByUserId, "user-last-modified-by-user-id", "", "Last modified by user id")
	cmd.Flags().StringVar(&bodyUserLastModifiedTime, "user-last-modified-time", "", "Last modified time")
	cmd.Flags().StringVar(&bodyUserLcid, "user-lcid", "", "Lcid")
	cmd.Flags().StringVar(&bodyUserNameFirstName, "user-name-first-name", "", "First name")
	cmd.Flags().StringVar(&bodyUserNameLastName, "user-name-last-name", "", "Last name")
	cmd.Flags().StringVar(&bodyUserNameMiddleInitial, "user-name-middle-initial", "", "Middle initial")
	cmd.Flags().StringVar(&bodyUserPassword, "user-password", "", "Password")
	cmd.Flags().StringVar(&bodyUserSecretAnswer, "user-secret-answer", "", "Secret answer")
	cmd.Flags().StringVar(&bodyUserSecretQuestion, "user-secret-question", "", "Secret question")
	cmd.Flags().StringVar(&bodyUserTimeStamp, "user-time-stamp", "", "Time stamp")
	cmd.Flags().StringVar(&bodyUserUserLifeCycleStatus, "user-user-life-cycle-status", "", "User life cycle status")
	cmd.Flags().StringVar(&bodyUserUserName, "user-user-name", "", "User name")
	cmd.Flags().StringVar(&bodyUserId2, "user-id-2", "", "User id")
	cmd.Flags().StringVar(&bodyUserInvitationAccountIds, "user-invitation-account-ids", "", "Account ids")
	cmd.Flags().StringVar(&bodyUserInvitationCustomerId, "user-invitation-customer-id", "", "Customer id")
	cmd.Flags().StringVar(&bodyUserInvitationEmail, "user-invitation-email", "", "Email")
	cmd.Flags().StringVar(&bodyUserInvitationExpirationDate, "user-invitation-expiration-date", "", "Expiration date")
	cmd.Flags().StringVar(&bodyUserInvitationFirstName, "user-invitation-first-name", "", "First name")
	cmd.Flags().StringVar(&bodyUserInvitationId, "user-invitation-id", "", "Id")
	cmd.Flags().StringVar(&bodyUserInvitationLastName, "user-invitation-last-name", "", "Last name")
	cmd.Flags().StringVar(&bodyUserInvitationLcid, "user-invitation-lcid", "", "Lcid")
	cmd.Flags().IntVar(&bodyUserInvitationRoleId, "user-invitation-role-id", 0, "Role id")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin (use this for deeply nested fields not exposed as flags)")

	return cmd
}
