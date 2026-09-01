package testsupport

// The identity provider's settings this codebase names, by the names it reads.
//
// Here rather than beside the harness that sets them because the fast gate needs them too — it
// reads docker-compose.yml and compares — and cycle.go is behind the integration tag it does not
// compile.

// OTPExpiryVariable is the lifetime of a one-time link.
const OTPExpiryVariable = "GOTRUE_MAILER_OTP_EXP"

// MailerMaxFrequencyVariable is the shortest gap GoTrue allows between two emails to one person.
//
// SMTP_, not MAILER_, and the wrong name is silent. Measured against v2.194.0 on 2026-08-16: with
// GOTRUE_MAILER_MAX_FREQUENCY set to 2s the provider still refused a second /recover "after 56
// seconds" — its one-minute default — and with this name it refused for the two seconds it was given.
const MailerMaxFrequencyVariable = "GOTRUE_SMTP_MAX_FREQUENCY"

// PasswordMinLengthVariable is the shortest password GoTrue will accept. Unset it enforces six —
// measured against v2.194.0 on 2026-09-01 — and the product does not leave it unset.
const PasswordMinLengthVariable = "GOTRUE_PASSWORD_MIN_LENGTH"

// RedirectAllowListVariable is where a link may land — TestTheAllowListDecidesWhereALinkLands
// measures what an uncovered address costs.
const RedirectAllowListVariable = "GOTRUE_URI_ALLOW_LIST"

// DisableSignupVariable closes the provider's public registration routes: an account appears only
// by the clinic's invitation, the identity block's third invariant. Unlike the settings above, this
// pair is read only under the integration tag.
const DisableSignupVariable = "GOTRUE_DISABLE_SIGNUP"

const SignupDisabled = "true"

// EmailsPerHourVariable is a quota for the whole instance rather than a gap per person, and the one
// limit that reaches the admin /invite. Measured against v2.194.0 on 2026-08-16 by running the
// harness at a quota of two: the third email of the run — an invitation — was refused with the same
// over_email_send_rate_limit the gap gives and the message "email rate limit exceeded".
const EmailsPerHourVariable = "GOTRUE_RATE_LIMIT_EMAIL_SENT"
