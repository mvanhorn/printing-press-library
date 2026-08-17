// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	pathpkg "path"
	"strings"
)

func authConfigFieldSet(names ...string) map[string]struct{} {
	fields := make(map[string]struct{}, len(names))
	for _, name := range names {
		fields[name] = struct{}{}
	}
	return fields
}

// auditedSafeAuthConfigFields is intentionally explicit. A generated schema
// refresh must classify every new field before the CLI will expose it.
var auditedSafeAuthConfigFields = authConfigFieldSet(
	"api_max_request_duration",
	"custom_oauth_enabled",
	"custom_oauth_max_providers",
	"db_max_pool_size",
	"db_max_pool_size_unit",
	"disable_signup",
	"external_anonymous_users_enabled",
	"external_apple_additional_client_ids",
	"external_apple_client_id",
	"external_apple_email_optional",
	"external_apple_enabled",
	"external_azure_client_id",
	"external_azure_email_optional",
	"external_azure_enabled",
	"external_azure_url",
	"external_bitbucket_client_id",
	"external_bitbucket_email_optional",
	"external_bitbucket_enabled",
	"external_discord_client_id",
	"external_discord_email_optional",
	"external_discord_enabled",
	"external_email_enabled",
	"external_facebook_client_id",
	"external_facebook_email_optional",
	"external_facebook_enabled",
	"external_figma_client_id",
	"external_figma_email_optional",
	"external_figma_enabled",
	"external_github_client_id",
	"external_github_email_optional",
	"external_github_enabled",
	"external_gitlab_client_id",
	"external_gitlab_email_optional",
	"external_gitlab_enabled",
	"external_gitlab_url",
	"external_google_additional_client_ids",
	"external_google_client_id",
	"external_google_email_optional",
	"external_google_enabled",
	"external_google_skip_nonce_check",
	"external_kakao_client_id",
	"external_kakao_email_optional",
	"external_kakao_enabled",
	"external_keycloak_client_id",
	"external_keycloak_email_optional",
	"external_keycloak_enabled",
	"external_keycloak_url",
	"external_linkedin_oidc_client_id",
	"external_linkedin_oidc_email_optional",
	"external_linkedin_oidc_enabled",
	"external_notion_client_id",
	"external_notion_email_optional",
	"external_notion_enabled",
	"external_phone_enabled",
	"external_slack_client_id",
	"external_slack_email_optional",
	"external_slack_enabled",
	"external_slack_oidc_client_id",
	"external_slack_oidc_email_optional",
	"external_slack_oidc_enabled",
	"external_spotify_client_id",
	"external_spotify_email_optional",
	"external_spotify_enabled",
	"external_twitch_client_id",
	"external_twitch_email_optional",
	"external_twitch_enabled",
	"external_twitter_client_id",
	"external_twitter_email_optional",
	"external_twitter_enabled",
	"external_web3_ethereum_enabled",
	"external_web3_solana_enabled",
	"external_workos_client_id",
	"external_workos_enabled",
	"external_workos_url",
	"external_x_client_id",
	"external_x_email_optional",
	"external_x_enabled",
	"external_zoom_client_id",
	"external_zoom_email_optional",
	"external_zoom_enabled",
	"hook_after_user_created_enabled",
	"hook_before_user_created_enabled",
	"hook_custom_access_token_enabled",
	"hook_mfa_verification_attempt_enabled",
	"hook_password_verification_attempt_enabled",
	"hook_send_email_enabled",
	"hook_send_sms_enabled",
	"jwt_exp",
	"mailer_allow_unverified_email_sign_ins",
	"mailer_autoconfirm",
	"mailer_notifications_email_changed_enabled",
	"mailer_notifications_identity_linked_enabled",
	"mailer_notifications_identity_unlinked_enabled",
	"mailer_notifications_mfa_factor_enrolled_enabled",
	"mailer_notifications_mfa_factor_unenrolled_enabled",
	"mailer_notifications_password_changed_enabled",
	"mailer_notifications_phone_changed_enabled",
	"mailer_otp_exp",
	"mailer_otp_length",
	"mailer_secure_email_change_enabled",
	"mailer_subjects_confirmation",
	"mailer_subjects_email_change",
	"mailer_subjects_email_changed_notification",
	"mailer_subjects_identity_linked_notification",
	"mailer_subjects_identity_unlinked_notification",
	"mailer_subjects_invite",
	"mailer_subjects_magic_link",
	"mailer_subjects_mfa_factor_enrolled_notification",
	"mailer_subjects_mfa_factor_unenrolled_notification",
	"mailer_subjects_password_changed_notification",
	"mailer_subjects_phone_changed_notification",
	"mailer_subjects_reauthentication",
	"mailer_subjects_recovery",
	"mailer_templates_confirmation_content",
	"mailer_templates_email_change_content",
	"mailer_templates_email_changed_notification_content",
	"mailer_templates_identity_linked_notification_content",
	"mailer_templates_identity_unlinked_notification_content",
	"mailer_templates_invite_content",
	"mailer_templates_magic_link_content",
	"mailer_templates_mfa_factor_enrolled_notification_content",
	"mailer_templates_mfa_factor_unenrolled_notification_content",
	"mailer_templates_password_changed_notification_content",
	"mailer_templates_phone_changed_notification_content",
	"mailer_templates_reauthentication_content",
	"mailer_templates_recovery_content",
	"mfa_max_enrolled_factors",
	"mfa_phone_enroll_enabled",
	"mfa_phone_max_frequency",
	"mfa_phone_otp_length",
	"mfa_phone_template",
	"mfa_phone_verify_enabled",
	"mfa_totp_enroll_enabled",
	"mfa_totp_verify_enabled",
	"mfa_web_authn_enroll_enabled",
	"mfa_web_authn_verify_enabled",
	"nimbus_oauth_client_id",
	"nimbus_oauth_email_optional",
	"oauth_server_allow_dynamic_registration",
	"oauth_server_authorization_path",
	"oauth_server_enabled",
	"passkey_enabled",
	"password_hibp_enabled",
	"password_min_length",
	"password_required_characters",
	"rate_limit_anonymous_users",
	"rate_limit_email_sent",
	"rate_limit_otp",
	"rate_limit_sms_sent",
	"rate_limit_token_refresh",
	"rate_limit_verify",
	"rate_limit_web3",
	"refresh_token_rotation_enabled",
	"saml_allow_encrypted_assertions",
	"saml_enabled",
	"saml_external_url",
	"security_captcha_enabled",
	"security_captcha_provider",
	"security_manual_linking_enabled",
	"security_refresh_token_reuse_interval",
	"security_sb_forwarded_for_enabled",
	"security_update_password_require_reauthentication",
	"sessions_inactivity_timeout",
	"sessions_single_per_user",
	"sessions_tags",
	"sessions_timebox",
	"site_url",
	"sms_autoconfirm",
	"sms_max_frequency",
	"sms_messagebird_originator",
	"sms_otp_exp",
	"sms_otp_length",
	"sms_provider",
	"sms_template",
	"sms_test_otp_valid_until",
	"sms_textlocal_sender",
	"sms_twilio_account_sid",
	"sms_twilio_content_sid",
	"sms_twilio_message_service_sid",
	"sms_twilio_verify_account_sid",
	"sms_twilio_verify_message_service_sid",
	"sms_vonage_from",
	"smtp_admin_email",
	"smtp_host",
	"smtp_max_frequency",
	"smtp_port",
	"smtp_sender_name",
	"smtp_user",
	"uri_allow_list",
	"webauthn_rp_display_name",
	"webauthn_rp_id",
	"webauthn_rp_origins",
)

// auditedSensitiveAuthConfigFields is the matching explicit deny set for the
// current generated AuthConfigResponse schema. Auth Hook URIs are denied too:
// valid HTTP(S) URLs can embed credentials in userinfo or query parameters.
var auditedSensitiveAuthConfigFields = authConfigFieldSet(
	"external_apple_secret",
	"external_azure_secret",
	"external_bitbucket_secret",
	"external_discord_secret",
	"external_facebook_secret",
	"external_figma_secret",
	"external_github_secret",
	"external_gitlab_secret",
	"external_google_secret",
	"external_kakao_secret",
	"external_keycloak_secret",
	"external_linkedin_oidc_secret",
	"external_notion_secret",
	"external_slack_oidc_secret",
	"external_slack_secret",
	"external_spotify_secret",
	"external_twitch_secret",
	"external_twitter_secret",
	"external_workos_secret",
	"external_x_secret",
	"external_zoom_secret",
	"hook_after_user_created_secrets",
	"hook_after_user_created_uri",
	"hook_before_user_created_secrets",
	"hook_before_user_created_uri",
	"hook_custom_access_token_secrets",
	"hook_custom_access_token_uri",
	"hook_mfa_verification_attempt_secrets",
	"hook_mfa_verification_attempt_uri",
	"hook_password_verification_attempt_secrets",
	"hook_password_verification_attempt_uri",
	"hook_send_email_secrets",
	"hook_send_email_uri",
	"hook_send_sms_secrets",
	"hook_send_sms_uri",
	"nimbus_oauth_client_secret",
	"security_captcha_secret",
	"sms_messagebird_access_key",
	"sms_test_otp",
	"sms_textlocal_api_key",
	"sms_twilio_auth_token",
	"sms_twilio_verify_auth_token",
	"sms_vonage_api_key",
	"sms_vonage_api_secret",
	"smtp_pass",
)

// IsAuthConfigPath canonicalizes path and query syntax before applying the
// policy. This keeps dot segments, repeated slashes, encoded separators, and a
// query suffix from bypassing redaction when the HTTP stack normalizes them.
func IsAuthConfigPath(rawPath string) bool {
	candidate := rawPath
	if strings.HasPrefix(rawPath, "/") {
		if queryIndex := strings.IndexAny(candidate, "?#"); queryIndex >= 0 {
			candidate = candidate[:queryIndex]
		}
		if decoded, err := url.PathUnescape(candidate); err == nil {
			candidate = decoded
		}
	} else if parsed, err := url.Parse(rawPath); err == nil {
		candidate = parsed.Path
	} else if queryIndex := strings.IndexAny(candidate, "?#"); queryIndex >= 0 {
		candidate = candidate[:queryIndex]
	}
	candidate = strings.ReplaceAll(candidate, `\`, "/")
	candidate = pathpkg.Clean("/" + strings.TrimLeft(candidate, "/"))
	parts := strings.Split(strings.Trim(candidate, "/"), "/")
	return len(parts) == 5 &&
		parts[0] == "v1" &&
		parts[1] == "projects" &&
		parts[2] != "" &&
		parts[3] == "config" &&
		parts[4] == "auth"
}

// IsSensitiveAuthConfigField lets the CLI reject secret-valued flags without
// duplicating the audited response classification.
func IsSensitiveAuthConfigField(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	_, sensitive := auditedSensitiveAuthConfigFields[name]
	return sensitive
}

func isSensitiveAuthConfigField(name string) bool {
	return IsSensitiveAuthConfigField(name)
}

// SanitizeAuthConfigResponse removes all secret-bearing and unknown fields.
// Callers must apply it before caching, formatting, delivery, or MCP output.
func SanitizeAuthConfigResponse(data json.RawMessage) (json.RawMessage, error) {
	return sanitizeAuthConfigObject(data, false)
}

// ValidateAuthConfigRequest ensures live and dry-run PATCH requests share the
// same audited schema. Unknown fields fail closed instead of being sent live
// after disappearing from a redacted preview; values are never included in an
// error message.
func ValidateAuthConfigRequest(data json.RawMessage) error {
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil || input == nil {
		return fmt.Errorf("expected a JSON object")
	}
	for name, value := range input {
		_, safe := auditedSafeAuthConfigFields[name]
		_, sensitive := auditedSensitiveAuthConfigFields[name]
		if !safe && !sensitive {
			return fmt.Errorf("auth config field %q is not in the audited schema", name)
		}
		if !isAuthConfigScalar(value) {
			return fmt.Errorf("auth config field %q had an unexpected non-scalar value", name)
		}
	}
	return nil
}

// sanitizeAuthConfigRequestPreview preserves the names of credential fields so
// dry runs remain useful, but replaces their values before writing to stderr.
func sanitizeAuthConfigRequestPreview(data json.RawMessage) (json.RawMessage, error) {
	if err := ValidateAuthConfigRequest(data); err != nil {
		return nil, err
	}
	return sanitizeAuthConfigObject(data, true)
}

func sanitizeAuthConfigObject(data json.RawMessage, keepRedactedFieldNames bool) (json.RawMessage, error) {
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil || input == nil {
		return nil, fmt.Errorf("expected a JSON object")
	}

	output := make(map[string]any, len(input))
	for name, value := range input {
		if isSensitiveAuthConfigField(name) {
			if keepRedactedFieldNames {
				output[name] = "[REDACTED]"
			}
			continue
		}
		if _, approved := auditedSafeAuthConfigFields[name]; !approved {
			continue
		}
		if !isAuthConfigScalar(value) {
			return nil, fmt.Errorf("auth config field %q had an unexpected non-scalar value", name)
		}
		output[name] = value
	}
	return json.Marshal(output)
}

func isAuthConfigScalar(value any) bool {
	switch value.(type) {
	case nil, bool, float64, string:
		return true
	default:
		return false
	}
}
