package api

import (
	"database/sql"
	"testing"

	"github.com/roberte3/rss.chat.go/db"
)

func TestMentionMatcherBasic(t *testing.T) {
	conn := setupTestMentionDB(t)
	defer conn.Close()

	// Create test user
	insertTestUser(t, conn, "alice", "secret123")

	matcher := MentionMatcher(conn, "https://example.com/?screenname={screenname}")
	input := "<p>hello @alice world</p>"

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that @alice is wrapped in a link with class="mention"
	if !containsSubstring(result, `<a href="https://example.com/?screenname=alice" class="mention">@alice</a>`) {
		t.Errorf("expected mention link in result, got %q", result)
	}
}

func TestMentionMatcherCaseInsensitive(t *testing.T) {
	conn := setupTestMentionDB(t)
	defer conn.Close()

	// Create user with lowercase screenname
	insertTestUser(t, conn, "alice", "secret123")

	matcher := MentionMatcher(conn, "https://example.com/?screenname={screenname}")
	input := "<p>hello @ALICE world</p>"

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that @ALICE is linked with canonical name alice
	if !containsSubstring(result, `href="https://example.com/?screenname=alice"`) {
		t.Errorf("expected mention link with canonical screenname, got %q", result)
	}
	// But display text should preserve original casing
	if !containsSubstring(result, `>@ALICE<`) {
		t.Errorf("expected display text @ALICE to preserve casing, got %q", result)
	}
}

func TestMentionMatcherNonexistentUser(t *testing.T) {
	conn := setupTestMentionDB(t)
	defer conn.Close()

	matcher := MentionMatcher(conn, "https://example.com/?screenname={screenname}")
	input := "<p>hello @nonexistent world</p>"

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// @nonexistent should remain plain text
	if !containsSubstring(result, "@nonexistent") {
		t.Errorf("expected @nonexistent to remain in output, got %q", result)
	}
	// But should NOT be in a link
	if containsSubstring(result, `<a href`) {
		t.Errorf("expected @nonexistent to NOT be linked, got %q", result)
	}
}

func TestMentionMatcherBoundarySpace(t *testing.T) {
	conn := setupTestMentionDB(t)
	defer conn.Close()

	insertTestUser(t, conn, "alice", "secret123")

	matcher := MentionMatcher(conn, "https://example.com/?screenname={screenname}")
	input := "<p>hello @alice world</p>"

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Space before @alice should be preserved
	if !containsSubstring(result, "hello <a") {
		t.Errorf("expected space before mention link, got %q", result)
	}
}

func TestMentionMatcherBoundaryParen(t *testing.T) {
	conn := setupTestMentionDB(t)
	defer conn.Close()

	insertTestUser(t, conn, "alice", "secret123")

	matcher := MentionMatcher(conn, "https://example.com/?screenname={screenname}")
	input := "<p>hello (@alice) world</p>"

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Paren before @alice should be preserved, mention should be linked
	if !containsSubstring(result, "(<a") {
		t.Errorf("expected paren before mention link, got %q", result)
	}
}

func TestMentionMatcherEmailNotMention(t *testing.T) {
	conn := setupTestMentionDB(t)
	defer conn.Close()

	insertTestUser(t, conn, "alice", "secret123")

	matcher := MentionMatcher(conn, "https://example.com/?screenname={screenname}")
	input := "<p>contact alice@example.com</p>"

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// @example should NOT be a mention (preceded by letter, not boundary)
	if containsSubstring(result, `<a href="https://example.com/?screenname=example"`) {
		t.Errorf("expected email address NOT to contain mention link, got %q", result)
	}
}

func TestMentionMatcherMultipleMentions(t *testing.T) {
	conn := setupTestMentionDB(t)
	defer conn.Close()

	insertTestUser(t, conn, "alice", "secret123")
	insertTestUser(t, conn, "bob", "secret456")

	matcher := MentionMatcher(conn, "https://example.com/?screenname={screenname}")
	input := "<p>hello @alice and @bob</p>"

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both should be linked
	if !containsSubstring(result, `href="https://example.com/?screenname=alice"`) {
		t.Errorf("expected alice mention link, got %q", result)
	}
	if !containsSubstring(result, `href="https://example.com/?screenname=bob"`) {
		t.Errorf("expected bob mention link, got %q", result)
	}
}

func TestMentionMatcherInCode(t *testing.T) {
	conn := setupTestMentionDB(t)
	defer conn.Close()

	insertTestUser(t, conn, "alice", "secret123")

	matcher := MentionMatcher(conn, "https://example.com/?screenname={screenname}")
	input := "<p>hello @alice</p><code>@alice</code>"

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Mention outside code should be linked
	if !containsSubstring(result, "<p>") || !containsSubstring(result, `<a href="https://example.com/?screenname=alice"`) {
		// Note: checking for <a> inside <p>, not worrying about exact order
		t.Logf("Result: %q", result)
	}

	// Mention in code should NOT be linked
	countLinkMatches := countOccurrences(result, `<a href="https://example.com/?screenname=alice"`)
	if countLinkMatches != 1 {
		t.Errorf("expected exactly 1 mention link (not in code), got %d", countLinkMatches)
	}
	if !containsSubstring(result, "<code>@alice</code>") {
		t.Errorf("expected @alice to remain plain in code, got %q", result)
	}
}

func TestExtractMentions(t *testing.T) {
	conn := setupTestMentionDB(t)
	defer conn.Close()

	insertTestUser(t, conn, "alice", "secret123")
	insertTestUser(t, conn, "bob", "secret456")

	htmlText := "<p>hello @alice and @bob and @nonexistent</p>"

	mentions, err := ExtractMentions(conn, htmlText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return alice and bob, but not nonexistent
	if len(mentions) != 2 {
		t.Errorf("expected 2 mentions, got %d: %v", len(mentions), mentions)
	}

	mentionMap := make(map[string]bool)
	for _, m := range mentions {
		mentionMap[m] = true
	}

	if !mentionMap["alice"] || !mentionMap["bob"] {
		t.Errorf("expected alice and bob in mentions, got %v", mentions)
	}
	if mentionMap["nonexistent"] {
		t.Errorf("did not expect nonexistent in mentions, got %v", mentions)
	}
}

func TestExtractMentionsEmpty(t *testing.T) {
	conn := setupTestMentionDB(t)
	defer conn.Close()

	htmlText := "<p>no mentions here</p>"

	mentions, err := ExtractMentions(conn, htmlText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mentions) != 0 {
		t.Errorf("expected no mentions, got %v", mentions)
	}
}

// Helper functions

func setupTestMentionDB(t *testing.T) *sql.DB {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory database: %v", err)
	}

	if err := db.CreateTables(conn); err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	return conn
}

func countOccurrences(text, substr string) int {
	count := 0
	start := 0
	for {
		pos := findSubstringPos(text[start:], substr)
		if pos == -1 {
			break
		}
		count++
		start += pos + len(substr)
	}
	return count
}

func findSubstringPos(text, substr string) int {
	for i := 0; i <= len(text)-len(substr); i++ {
		if text[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
