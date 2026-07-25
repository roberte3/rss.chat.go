package email

import (
	"fmt"
	"net/smtp"
	"strings"
)

// Config holds email sending configuration.
type Config struct {
	// SMTP settings
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	FromAddress  string
	FromName     string

	// Service settings (for SES/Sendgrid, not yet implemented)
	Provider string // "smtp", "ses", "sendgrid"
}

// Sender handles email sending.
type Sender struct {
	config Config
}

// NewSender creates a new email sender.
func NewSender(config Config) *Sender {
	return &Sender{config: config}
}

// SendConfirmationEmail sends a confirmation email with a magic link.
func (s *Sender) SendConfirmationEmail(email string, confirmationURL string, operationType string) error {
	if s.config.Provider != "smtp" {
		// For v1, only SMTP is implemented
		return fmt.Errorf("email provider %q not implemented", s.config.Provider)
	}

	subject := "Confirm your email address"
	switch operationType {
	case "signup":
		subject = "Welcome to rss.chat — Confirm your email"
	case "signin":
		subject = "Sign in to rss.chat"
	}

	htmlBody := buildConfirmationEmail(confirmationURL, operationType)
	textBody := buildConfirmationEmailText(confirmationURL, operationType)

	return s.sendSMTP(email, subject, htmlBody, textBody)
}

// sendSMTP sends an email via SMTP.
func (s *Sender) sendSMTP(to string, subject string, htmlBody string, textBody string) error {
	// Build email headers
	from := s.config.FromAddress
	if s.config.FromName != "" {
		from = fmt.Sprintf("%s <%s>", s.config.FromName, s.config.FromAddress)
	}

	message := fmt.Sprintf(
		"To: %s\r\nFrom: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n%s",
		to, from, subject, htmlBody,
	)

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)
	auth := smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)

	err := smtp.SendMail(addr, auth, s.config.FromAddress, []string{to}, []byte(message))
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}

// buildConfirmationEmail builds an HTML confirmation email.
func buildConfirmationEmail(confirmationURL string, operationType string) string {
	title := "Confirm Your Email"
	buttonText := "Confirm Email"
	message := "Click the button below to confirm your email address."

	switch operationType {
	case "signup":
		title = "Welcome to rss.chat"
		message = "Thank you for signing up! Click the button below to confirm your email address and create your account."
	case "signin":
		title = "Sign In to rss.chat"
		message = "Click the button below to sign in to your rss.chat account."
	}

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif; }
    .container { max-width: 600px; margin: 0 auto; padding: 20px; }
    .header { text-align: center; margin-bottom: 30px; }
    .content { background: #f5f5f5; padding: 20px; border-radius: 8px; }
    .button-container { text-align: center; margin: 20px 0; }
    .button { display: inline-block; padding: 12px 24px; background: #007AFF; color: white; text-decoration: none; border-radius: 6px; font-weight: 500; }
    .button:hover { background: #0051D5; }
    .footer { text-align: center; color: #666; font-size: 12px; margin-top: 20px; }
    .code { font-family: monospace; background: white; padding: 4px 8px; border-radius: 4px; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>rss.chat</h1>
    </div>
    <div class="content">
      <h2>%s</h2>
      <p>%s</p>
      <div class="button-container">
        <a href="%s" class="button">%s</a>
      </div>
      <p style="color: #666; font-size: 14px;">Or copy and paste this link in your browser:<br>
        <a href="%s" style="color: #007AFF; word-break: break-all;">%s</a>
      </p>
    </div>
    <div class="footer">
      <p>This link will expire in 24 hours. If you didn't request this email, you can safely ignore it.</p>
    </div>
  </div>
</body>
</html>
`, title, message, confirmationURL, buttonText, confirmationURL, confirmationURL)

	return html
}

// buildConfirmationEmailText builds a plain text confirmation email.
func buildConfirmationEmailText(confirmationURL string, operationType string) string {
	title := "Confirm Your Email"
	message := "Click the link below to confirm your email address."

	switch operationType {
	case "signup":
		title = "Welcome to rss.chat"
		message = "Thank you for signing up! Click the link below to confirm your email address and create your account."
	case "signin":
		title = "Sign In to rss.chat"
		message = "Click the link below to sign in to your rss.chat account."
	}

	return fmt.Sprintf(`%s

%s

%s

This link will expire in 24 hours. If you didn't request this email, you can safely ignore it.
`, title, message, confirmationURL)
}

// ExtractConfirmationCode extracts the confirmation code from a URL.
// Expected format: urlredirect?emailconfirmed=true&email=...&code=...&screenname=...
func ExtractConfirmationCode(query string) (email string, code string, screenname string, ok bool) {
	params := parseQueryString(query)

	email, emailOk := params["email"]
	code, codeOk := params["code"]
	screenname, screenOk := params["screenname"]

	return email, code, screenname, emailOk && codeOk && screenOk
}

// parseQueryString is a simple query string parser.
func parseQueryString(query string) map[string]string {
	params := make(map[string]string)
	pairs := strings.Split(query, "&")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			params[kv[0]] = kv[1]
		}
	}
	return params
}
