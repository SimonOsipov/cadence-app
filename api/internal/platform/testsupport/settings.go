package testsupport

// The identity provider's mail settings, by the names it actually reads.
//
// They are here rather than beside the harness that sets them because two
// gates need them: the integration suite configures a container with them, and
// the fast gate reads them out of docker-compose.yml to compare the
// deployment's values with the constants this codebase derives from. Everything
// in cycle.go is behind the integration build tag, which the fast gate does not
// compile.

// OTPExpiryVariable is the lifetime of a one-time link.
const OTPExpiryVariable = "GOTRUE_MAILER_OTP_EXP"

// MailerMaxFrequencyVariable is the shortest gap GoTrue allows between two
// emails to one person.
//
// SMTP_, not MAILER_, and the name is worth the line because the wrong one is
// silent. Measured against v2.194.0 on 2026-08-16: with
// GOTRUE_MAILER_MAX_FREQUENCY set to 2s the provider still refused a second
// /recover "after 56 seconds" — its one-minute default — and with this name it
// refused for the two seconds it was given.
const MailerMaxFrequencyVariable = "GOTRUE_SMTP_MAX_FREQUENCY"

// EmailsPerHourVariable is a quota for the whole instance rather than a gap per
// person, and it is the one limit that reaches the admin /invite. Measured
// against v2.194.0 on 2026-08-16 by running the harness at a quota of two,
// where the third email of a run — an invitation — was refused with the same
// over_email_send_rate_limit the gap gives and the message "email rate limit
// exceeded".
const EmailsPerHourVariable = "GOTRUE_RATE_LIMIT_EMAIL_SENT"
