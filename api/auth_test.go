package api

import (
	"strings"
	"testing"
)

// AuthenticateUser had no coverage. These pin the behaviour the switch to
// crypto/subtle had to preserve — the timing property itself is not asserted,
// because a wall-clock measurement of it would be flaky and prove little on a
// shared CI runner. What is asserted is that every near-miss is still rejected.
func TestAuthenticateUser(t *testing.T) {
	_, conn, _ := setupTestServer(t)
	const secret = "correct-horse-battery-staple-secret"
	insertTestUser(t, conn, "alice", secret)

	tests := []struct {
		name    string
		email   string
		code    string
		wantErr string // substring; empty means success
	}{
		{"valid credentials", "alice@example.com", secret, ""},
		{"wrong code", "alice@example.com", "totally-wrong", "invalid authentication code"},
		// The prefix cases are what a timing attack walks through: each must be
		// rejected exactly like any other wrong code.
		{"code is a prefix of the secret", "alice@example.com", secret[:len(secret)-1], "invalid authentication code"},
		{"code differs in last byte only", "alice@example.com", secret[:len(secret)-1] + "X", "invalid authentication code"},
		{"code differs in first byte only", "alice@example.com", "X" + secret[1:], "invalid authentication code"},
		{"code longer than secret", "alice@example.com", secret + "X", "invalid authentication code"},
		{"empty code", "alice@example.com", "", "credentials required"},
		{"empty email", "", secret, "credentials required"},
		{"unknown email", "nobody@example.com", secret, "user not found"},
		{"case-different secret", "alice@example.com", strings.ToUpper(secret), "invalid authentication code"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := AuthenticateUser(conn, tt.email, tt.code)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("err = %v, want success", err)
				}
				if user == nil || user.Screenname != "alice" {
					t.Fatalf("user = %+v, want alice", user)
				}
				return
			}

			if err == nil {
				t.Fatalf("err = nil, want %q — credentials were accepted", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %q, want it to mention %q", err, tt.wantErr)
			}
			if user != nil {
				t.Error("a user was returned alongside an error")
			}
		})
	}
}
