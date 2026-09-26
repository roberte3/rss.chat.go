package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/roberte3/rss.chat.go/config"
	"github.com/roberte3/rss.chat.go/db"
)

// MockEmailSender captures sent emails for testing.
type MockEmailSender struct {
	SentEmails []EmailRecord
}

// EmailRecord represents a sent email.
type EmailRecord struct {
	Recipient        string
	ConfirmationURL  string
	OperationType    string
}

// SendConfirmationEmail records the email.
func (m *MockEmailSender) SendConfirmationEmail(recipient string, confirmationURL string, operationType string) error {
	m.SentEmails = append(m.SentEmails, EmailRecord{
		Recipient:       recipient,
		ConfirmationURL: confirmationURL,
		OperationType:   operationType,
	})
	return nil
}

// TestSendConfirmingEmailWithMockSender tests the email endpoint with a mock sender.
func TestSendConfirmingEmailWithMockSender(t *testing.T) {
	_, conn, handler := setupTestServer(t)
	defer conn.Close()

	// Set up mock email sender
	mockSender := &MockEmailSender{}
	handler.SetEmailSender(mockSender)

	// Create an empty config
	handler.Config = &config.Config{
		MyDomain: "http://localhost:8080",
	}

	// Make request
	req := httptest.NewRequest("GET", "/sendconfirmingemail?email=alice@example.com&urlredirect=http://example.com/start", nil)
	w := httptest.NewRecorder()

	handler.SendConfirmingEmail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check that email was sent
	if len(mockSender.SentEmails) != 1 {
		t.Fatalf("Expected 1 email sent, got %d", len(mockSender.SentEmails))
	}

	email := mockSender.SentEmails[0]
	if email.Recipient != "alice@example.com" {
		t.Errorf("Expected recipient alice@example.com, got %s", email.Recipient)
	}
	if !strings.Contains(email.ConfirmationURL, "emailconfirmed=true") {
		t.Errorf("Expected confirmation URL in email, got %s", email.ConfirmationURL)
	}
	if email.OperationType != "signup" {
		t.Errorf("Expected operation type signup, got %s", email.OperationType)
	}
}

// TestCreateNewUserWithMockSender tests user creation with email sending.
func TestCreateNewUserWithMockSender(t *testing.T) {
	_, conn, handler := setupTestServer(t)
	defer conn.Close()

	// Set up mock email sender
	mockSender := &MockEmailSender{}
	handler.SetEmailSender(mockSender)

	// Set up config
	handler.Config = &config.Config{
		MyDomain: "http://localhost:8080",
	}

	// Make request to create new user
	req := httptest.NewRequest(
		"GET",
		"/createnewuser?email=bob@example.com&name=bob&urlredirect=http://example.com/done",
		nil,
	)
	w := httptest.NewRecorder()

	handler.CreateNewUser(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check that user was created
	user, err := db.GetUserInfoByEmail(conn, "bob@example.com")
	if err != nil {
		t.Fatalf("Error getting user: %v", err)
	}
	if user == nil {
		t.Fatal("User was not created")
	}
	if user.Screenname != "bob" {
		t.Errorf("Expected screenname bob, got %s", user.Screenname)
	}

	// Check that email was sent
	if len(mockSender.SentEmails) != 1 {
		t.Fatalf("Expected 1 email sent, got %d", len(mockSender.SentEmails))
	}

	email := mockSender.SentEmails[0]
	if email.Recipient != "bob@example.com" {
		t.Errorf("Expected recipient bob@example.com, got %s", email.Recipient)
	}
	if email.OperationType != "signup" {
		t.Errorf("Expected operation type signup, got %s", email.OperationType)
	}
}

// TestSendConfirmingEmailWithoutSender tests fallback when sender is not configured.
func TestSendConfirmingEmailWithoutSender(t *testing.T) {
	_, conn, handler := setupTestServer(t)
	defer conn.Close()

	// Don't set email sender (it remains nil)
	handler.Config = &config.Config{
		MyDomain: "http://localhost:8080",
	}

	// Make request
	req := httptest.NewRequest("GET", "/sendconfirmingemail?email=alice@example.com&urlredirect=http://example.com/start", nil)
	w := httptest.NewRecorder()

	handler.SendConfirmingEmail(w, req)

	// Should still succeed, just log instead of sending
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestSendConfirmingEmailErrorHandling tests error handling with a failing sender.
type FailingSender struct{}

func (f *FailingSender) SendConfirmationEmail(recipient string, confirmationURL string, operationType string) error {
	return fmt.Errorf("SMTP connection failed")
}

// TestSendConfirmingEmailWithFailingSender tests behavior when email sender fails.
func TestSendConfirmingEmailWithFailingSender(t *testing.T) {
	_, conn, handler := setupTestServer(t)
	defer conn.Close()

	// Set up failing sender
	handler.SetEmailSender(&FailingSender{})
	handler.Config = &config.Config{
		MyDomain: "http://localhost:8080",
	}

	// Make request
	req := httptest.NewRequest("GET", "/sendconfirmingemail?email=alice@example.com&urlredirect=http://example.com/start", nil)
	w := httptest.NewRecorder()

	handler.SendConfirmingEmail(w, req)

	// Should fail - endpoint returns error due to rate limiting or email failure
	if w.Code < 400 {
		t.Errorf("Expected error status code (>= 400), got %d", w.Code)
	}
}

// TestCreateNewUserEmailWithFailingSender tests that user creation still succeeds even if email fails.
func TestCreateNewUserEmailWithFailingSender(t *testing.T) {
	_, conn, handler := setupTestServer(t)
	defer conn.Close()

	// Set up failing sender
	handler.SetEmailSender(&FailingSender{})
	handler.Config = &config.Config{
		MyDomain: "http://localhost:8080",
	}

	// Make request to create new user
	req := httptest.NewRequest(
		"GET",
		"/createnewuser?email=charlie@example.com&name=charlie&urlredirect=http://example.com/done",
		nil,
	)
	w := httptest.NewRecorder()

	handler.CreateNewUser(w, req)

	// Should succeed even though email failed
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 (user created), got %d: %s", w.Code, w.Body.String())
	}

	// User should still be created
	user, err := db.GetUserInfoByEmail(conn, "charlie@example.com")
	if err != nil {
		t.Fatalf("Error getting user: %v", err)
	}
	if user == nil {
		t.Fatal("User was not created")
	}
}

// TestSendConfirmingEmailIncludesScreenname tests that screenname is included for existing users.
func TestSendConfirmingEmailIncludesScreenname(t *testing.T) {
	_, conn, handler := setupTestServer(t)
	defer conn.Close()

	// Create a user first - insertTestUser creates user with email testuser@example.com
	insertTestUser(t, conn, "existing", "secret123")

	// Set up mock sender
	mockSender := &MockEmailSender{}
	handler.SetEmailSender(mockSender)
	handler.Config = &config.Config{
		MyDomain: "http://localhost:8080",
	}

	// Send confirmation email to existing user (insertTestUser creates email as screenname@example.com)
	req := httptest.NewRequest(
		"GET",
		"/sendconfirmingemail?email=existing@example.com&urlredirect=http://example.com/start",
		nil,
	)
	w := httptest.NewRecorder()

	handler.SendConfirmingEmail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check that email includes screenname in URL
	if len(mockSender.SentEmails) != 1 {
		t.Fatalf("Expected 1 email sent, got %d", len(mockSender.SentEmails))
	}

	email := mockSender.SentEmails[0]
	if !strings.Contains(email.ConfirmationURL, "screenname=existing") {
		t.Errorf("Expected screenname in URL, got %s", email.ConfirmationURL)
	}
}
