package zerobouncego

// Validation status values returned by the API (Validate, ValidateBatch).
// Use for comparison: v.Status == zerobouncego.ValidateStatusValid
// Unknown/future API values are not listed; compare against ValidateResponse.Status as string.
const (
	ValidateStatusNone        = ""
	ValidateStatusValid       = "valid"
	ValidateStatusInvalid     = "invalid"
	ValidateStatusCatchAll    = "catch-all"
	ValidateStatusUnknown     = "unknown"
	ValidateStatusSpamtrap    = "spamtrap"
	ValidateStatusAbuse       = "abuse"
	ValidateStatusDoNotMail   = "do_not_mail"
)

// Validation sub-status values returned by the API (Validate, ValidateBatch).
// Use for comparison: v.SubStatus == zerobouncego.ValidateSubStatusAcceptAll
// Unknown/future API values are not listed; compare against ValidateResponse.SubStatus as string.
const (
	ValidateSubStatusNone                    = ""
	ValidateSubStatusAntispamSystem          = "antispam_system"
	ValidateSubStatusGreylisted              = "greylisted"
	ValidateSubStatusMailServerTemporaryErr  = "mail_server_temporary_error"
	ValidateSubStatusForcibleDisconnect      = "forcible_disconnect"
	ValidateSubStatusMailServerDidNotRespond = "mail_server_did_not_respond"
	ValidateSubStatusTimeoutExceeded         = "timeout_exceeded"
	ValidateSubStatusFailedSmtpConnection    = "failed_smtp_connection"
	ValidateSubStatusMailboxQuotaExceeded    = "mailbox_quota_exceeded"
	ValidateSubStatusExceptionOccurred       = "exception_occurred"
	ValidateSubStatusPossibleTrap            = "possible_trap"
	ValidateSubStatusRoleBased               = "role_based"
	ValidateSubStatusGlobalSuppression       = "global_suppression"
	ValidateSubStatusMailboxNotFound         = "mailbox_not_found"
	ValidateSubStatusNoDnsEntries            = "no_dns_entries"
	ValidateSubStatusFailedSyntaxCheck       = "failed_syntax_check"
	ValidateSubStatusPossibleTypo            = "possible_typo"
	ValidateSubStatusUnroutableIpAddress     = "unroutable_ip_address"
	ValidateSubStatusLeadingPeriodRemoved    = "leading_period_removed"
	ValidateSubStatusDoesNotAcceptMail       = "does_not_accept_mail"
	ValidateSubStatusAliasAddress            = "alias_address"
	ValidateSubStatusRoleBasedCatchAll        = "role_based_catch_all"
	ValidateSubStatusDisposable              = "disposable"
	ValidateSubStatusToxic                   = "toxic"
	ValidateSubStatusAlternate               = "alternate"
	ValidateSubStatusMxForward               = "mx_forward"
	ValidateSubStatusBlocked                 = "blocked"
	ValidateSubStatusAllowed                 = "allowed"
	ValidateSubStatusAcceptAll               = "accept_all"
	ValidateSubStatusRoleBasedAcceptAll      = "role_based_accept_all"
	ValidateSubStatusGold                    = "gold"
)
