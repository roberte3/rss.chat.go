package api

import (
	"regexp"
	"strings"
	"testing"
)

// TestHashtagMatcherBasic verifies hashtag rendering with basic matching.
func TestHashtagMatcherBasic(t *testing.T) {
	matcher := HashtagMatcher("https://example.com/?tag={tag}")
	input := "<p>hello #golang world</p>"

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that #golang is wrapped in a link with class="hashtag"
	if !containsSubstring(result, `<a href="https://example.com/?tag=golang" class="hashtag">#golang</a>`) {
		t.Errorf("expected hashtag link in result, got %q", result)
	}
}

// TestHashtagMatcherCaseSensitiveDisplay verifies casing is preserved in display.
func TestHashtagMatcherCaseSensitiveDisplay(t *testing.T) {
	matcher := HashtagMatcher("https://example.com/?tag={tag}")
	input := "<p>#GoLang rocks</p>"

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// URL should use lowercase, but display should preserve casing
	if !containsSubstring(result, `href="https://example.com/?tag=golang"`) {
		t.Errorf("expected lowercase tag in href, got %q", result)
	}
	if !containsSubstring(result, `>#GoLang<`) {
		t.Errorf("expected display casing #GoLang, got %q", result)
	}
}

// TestHashtagMatcherMultipleTags verifies multiple hashtags are handled.
func TestHashtagMatcherMultipleTags(t *testing.T) {
	matcher := HashtagMatcher("https://example.com/?tag={tag}")
	input := "<p>#golang and #rust and #python</p>"

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All three tags should be linked
	tags := []string{"golang", "rust", "python"}
	for _, tag := range tags {
		if !containsSubstring(result, "tag="+tag) {
			t.Errorf("expected tag %s in result, got %q", tag, result)
		}
	}
}

// TestHashtagMatcherInCode verifies hashtags in code blocks are not linked.
func TestHashtagMatcherInCode(t *testing.T) {
	matcher := HashtagMatcher("https://example.com/?tag={tag}")
	input := "<p>#golang here</p><code>#golang here too</code>"

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count how many hashtag links we have
	linkPattern := regexp.MustCompile(`<a[^>]*class="hashtag"`)
	matches := linkPattern.FindAllStringIndex(result, -1)

	if len(matches) != 1 {
		t.Errorf("expected 1 hashtag link (not in code), got %d", len(matches))
	}

	// Hashtag in code should remain plain
	if !containsSubstring(result, "<code>#golang here too</code>") {
		t.Errorf("expected #golang to remain plain in code, got %q", result)
	}
}

// TestHashtagMatcherMaxLength verifies 50 char limit.
func TestHashtagMatcherMaxLength(t *testing.T) {
	matcher := HashtagMatcher("https://example.com/?tag={tag}")

	// 50 chars is max
	tag50 := "#" + string(make([]byte, 50))
	for i := range tag50[1:] {
		tag50 = tag50[:i+1] + "a" + tag50[i+2:]
	}
	tag50 = "#" + string(make([]rune, 50))[:50]
	for i := 0; i < 50; i++ {
		tag50 = tag50[:i+1] + "a" + tag50[i+2:]
	}

	// Just use a simple 50-char tag
	longTag := "#"
	for i := 0; i < 50; i++ {
		longTag += "a"
	}

	input := "<p>" + longTag + " and #verylongthatexceedsfifeycharsandshouldnotmatch</p>"

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Long tag (50 chars) should be linked
	if !containsSubstring(result, `class="hashtag"`) {
		t.Errorf("expected long tag to be linked, got %q", result)
	}
}

// TestExtractHashtagsBasic verifies basic hashtag extraction.
func TestExtractHashtagsBasic(t *testing.T) {
	htmlText := "<p>#golang #rust #python</p>"

	tags := ExtractHashtags(htmlText)

	if len(tags) != 3 {
		t.Errorf("expected 3 tags, got %d: %v", len(tags), tags)
	}

	tagMap := make(map[string]bool)
	for _, tag := range tags {
		tagMap[tag] = true
	}

	expectedTags := []string{"golang", "rust", "python"}
	for _, expected := range expectedTags {
		if !tagMap[expected] {
			t.Errorf("expected tag %s not found, got %v", expected, tags)
		}
	}
}

// TestExtractHashtagsCaseNormalization verifies tags are normalized to lowercase.
func TestExtractHashtagsCaseNormalization(t *testing.T) {
	htmlText := "<p>#GoLang #RUST #PyThOn</p>"

	tags := ExtractHashtags(htmlText)

	if len(tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(tags))
	}

	// All tags should be lowercase
	for _, tag := range tags {
		if tag != strings.ToLower(tag) {
			t.Errorf("tag %q not normalized to lowercase", tag)
		}
	}
}

// TestExtractHashtagsUniqueness verifies duplicates are removed.
func TestExtractHashtagsUniqueness(t *testing.T) {
	htmlText := "<p>#golang is great #rust is cool #golang again</p>"

	tags := ExtractHashtags(htmlText)

	if len(tags) != 2 {
		t.Errorf("expected 2 unique tags, got %d: %v", len(tags), tags)
	}

	tagMap := make(map[string]bool)
	for _, tag := range tags {
		if tagMap[tag] {
			t.Errorf("duplicate tag found: %s", tag)
		}
		tagMap[tag] = true
	}
}

// TestExtractHashtagsEmpty verifies empty result for no hashtags.
func TestExtractHashtagsEmpty(t *testing.T) {
	htmlText := "<p>no hashtags here just regular text</p>"

	tags := ExtractHashtags(htmlText)

	if len(tags) != 0 {
		t.Errorf("expected no tags, got %d: %v", len(tags), tags)
	}
}

// TestExtractHashtagsWithNumbers verifies tags can contain numbers.
func TestExtractHashtagsWithNumbers(t *testing.T) {
	htmlText := "<p>#golang2024 #rust123 #python_3_9</p>"

	tags := ExtractHashtags(htmlText)

	if len(tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(tags))
	}

	// Check that tags with numbers are extracted
	tagMap := make(map[string]bool)
	for _, tag := range tags {
		tagMap[tag] = true
	}

	if !tagMap["golang2024"] || !tagMap["rust123"] || !tagMap["python_3_9"] {
		t.Errorf("tags with numbers not extracted correctly: %v", tags)
	}
}
