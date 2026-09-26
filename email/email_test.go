package email

import (
	"fmt"
	"strings"
	"testing"
)

// TestNewSender creates a new sender with valid config.
func TestNewSender(t *testing.T) {
	config := Config{
		SMTPHost:     "smtp.example.com",
		SMTPPort:     587,
		SMTPUsername: "user",
		SMTPPassword: "pass",
		FromAddress:  "noreply@example.com",
		FromName:     "RSS Chat",
		Provider:     "smtp",
	}

	sender := NewSender(config)
	if sender == nil {
		t.Fatal("NewSender returned nil")
	}
	if sender.config.SMTPHost != config.SMTPHost {
		t.Errorf("Expected SMTPHost %q, got %q", config.SMTPHost, sender.config.SMTPHost)
	}
}

// TestSendConfirmationEmailWithMockSMTP tests confirmation email generation.
// We can't easily mock net/smtp.SendMail, so we test the email builder functions.
func TestBuildConfirmationEmailSignup(t *testing.T) {
	url := "https://example.com/confirm?code=abc123"
	html := buildConfirmationEmail(url, "signup")

	if !strings.Contains(html, "Welcome to rss.chat") {
		t.Error("HTML doesn't contain signup title")
	}
	if !strings.Contains(html, url) {
		t.Error("HTML doesn't contain confirmation URL")
	}
	if !strings.Contains(html, "Confirm Email") {
		t.Error("HTML doesn't contain button text")
	}
}

// TestBuildConfirmationEmailSignin tests signin email generation.
func TestBuildConfirmationEmailSignin(t *testing.T) {
	url := "https://example.com/confirm?code=xyz789"
	html := buildConfirmationEmail(url, "signin")

	if !strings.Contains(html, "Sign In to rss.chat") {
		t.Error("HTML doesn't contain signin title")
	}
	if !strings.Contains(html, url) {
		t.Error("HTML doesn't contain confirmation URL")
	}
}

// TestBuildConfirmationEmailDefault tests default email generation.
func TestBuildConfirmationEmailDefault(t *testing.T) {
	url := "https://example.com/confirm?code=default"
	html := buildConfirmationEmail(url, "unknown")

	if !strings.Contains(html, "Confirm Your Email") {
		t.Error("HTML doesn't contain default title")
	}
	if !strings.Contains(html, url) {
		t.Error("HTML doesn't contain confirmation URL")
	}
}

// TestBuildConfirmationEmailText tests plain text email generation.
func TestBuildConfirmationEmailTextSignup(t *testing.T) {
	url := "https://example.com/confirm?code=abc123"
	text := buildConfirmationEmailText(url, "signup")

	if !strings.Contains(text, "Welcome to rss.chat") {
		t.Error("Text doesn't contain signup title")
	}
	if !strings.Contains(text, url) {
		t.Error("Text doesn't contain confirmation URL")
	}
	if !strings.Contains(text, "Thank you for signing up") {
		t.Error("Text doesn't contain signup message")
	}
}

// TestBuildConfirmationEmailTextSignin tests signin plain text email.
func TestBuildConfirmationEmailTextSignin(t *testing.T) {
	url := "https://example.com/confirm?code=xyz789"
	text := buildConfirmationEmailText(url, "signin")

	if !strings.Contains(text, "Sign In to rss.chat") {
		t.Error("Text doesn't contain signin title")
	}
	if !strings.Contains(text, url) {
		t.Error("Text doesn't contain confirmation URL")
	}
}

// TestExtractConfirmationCode extracts confirmation parameters.
func TestExtractConfirmationCode(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		expectedEmail  string
		expectedCode   string
		expectedScreen string
		expectedOk     bool
	}{
		{
			name:           "valid parameters",
			query:          "email=alice@example.com&code=abc123&screenname=alice",
			expectedEmail:  "alice@example.com",
			expectedCode:   "abc123",
			expectedScreen: "alice",
			expectedOk:     true,
		},
		{
			name:           "different order",
			query:          "code=xyz789&screenname=bob&email=bob@example.com",
			expectedEmail:  "bob@example.com",
			expectedCode:   "xyz789",
			expectedScreen: "bob",
			expectedOk:     true,
		},
		{
			name:           "missing code",
			query:          "email=alice@example.com&screenname=alice",
			expectedEmail:  "",
			expectedCode:   "",
			expectedScreen: "",
			expectedOk:     false,
		},
		{
			name:           "missing email",
			query:          "code=abc123&screenname=alice",
			expectedEmail:  "",
			expectedCode:   "",
			expectedScreen: "",
			expectedOk:     false,
		},
		{
			name:           "missing screenname",
			query:          "email=alice@example.com&code=abc123",
			expectedEmail:  "",
			expectedCode:   "",
			expectedScreen: "",
			expectedOk:     false,
		},
		{
			name:           "empty query",
			query:          "",
			expectedEmail:  "",
			expectedCode:   "",
			expectedScreen: "",
			expectedOk:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email, code, screen, ok := ExtractConfirmationCode(tt.query)
			if ok != tt.expectedOk {
				t.Errorf("Expected ok=%v, got %v", tt.expectedOk, ok)
			}
			if ok {
				if email != tt.expectedEmail {
					t.Errorf("Expected email %q, got %q", tt.expectedEmail, email)
				}
				if code != tt.expectedCode {
					t.Errorf("Expected code %q, got %q", tt.expectedCode, code)
				}
				if screen != tt.expectedScreen {
					t.Errorf("Expected screen %q, got %q", tt.expectedScreen, screen)
				}
			}
		})
	}
}

// TestParseQueryString parses query strings correctly.
func TestParseQueryString(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected map[string]string
	}{
		{
			name:  "single parameter",
			query: "key=value",
			expected: map[string]string{
				"key": "value",
			},
		},
		{
			name:  "multiple parameters",
			query: "a=1&b=2&c=3",
			expected: map[string]string{
				"a": "1",
				"b": "2",
				"c": "3",
			},
		},
		{
			name:  "duplicate keys (last wins)",
			query: "key=first&key=second",
			expected: map[string]string{
				"key": "second",
			},
		},
		{
			name:     "empty query",
			query:    "",
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseQueryString(tt.query)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d parameters, got %d", len(tt.expected), len(result))
			}
			for key, expectedVal := range tt.expected {
				if val, ok := result[key]; !ok || val != expectedVal {
					t.Errorf("Key %q: expected %q, got %q", key, expectedVal, val)
				}
			}
		})
	}
}

// TestSendConfirmationEmailUnsupportedProvider tests error on unsupported provider.
func TestSendConfirmationEmailUnsupportedProvider(t *testing.T) {
	sender := NewSender(Config{
		Provider: "sendgrid", // Not yet implemented
	})

	err := sender.SendConfirmationEmail("test@example.com", "http://example.com/confirm", "signup")
	if err == nil {
		t.Error("Expected error for unsupported provider")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("Expected 'not implemented' in error, got: %v", err)
	}
}

// MockSMTPDialer allows testing SMTP without a real server.
// This demonstrates how the sender would be used in integration tests.
type MockSender struct {
	LastRecipient string
	LastSubject   string
	LastBody      string
	SendError     error
}

// SendConfirmationEmail sends a test email.
func (m *MockSender) SendConfirmationEmail(email string, confirmationURL string, operationType string) error {
	if m.SendError != nil {
		return m.SendError
	}
	m.LastRecipient = email
	m.LastBody = confirmationURL
	return nil
}

// TestMockSender demonstrates mocking the email sender in tests.
func TestMockSender(t *testing.T) {
	mock := &MockSender{}
	email := "alice@example.com"
	url := "http://example.com/confirm"

	err := mock.SendConfirmationEmail(email, url, "signup")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if mock.LastRecipient != email {
		t.Errorf("Expected recipient %q, got %q", email, mock.LastRecipient)
	}
	if mock.LastBody != url {
		t.Errorf("Expected URL %q in body, got %q", url, mock.LastBody)
	}

	// Test with error
	mock.SendError = fmt.Errorf("SMTP error")
	err = mock.SendConfirmationEmail(email, url, "signin")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}
