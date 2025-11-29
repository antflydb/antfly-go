package docsaf

import (
	"testing"
)

func TestHTMLProcessor_CanProcess(t *testing.T) {
	hp := &HTMLProcessor{}

	tests := []struct {
		name        string
		contentType string
		path        string
		want        bool
	}{
		{"html file", "", "test.html", true},
		{"htm file", "", "test.htm", true},
		{"HTML uppercase", "", "test.HTML", true},
		{"html content type", "text/html", "test", true},
		{"html content type with charset", "text/html; charset=utf-8", "test", true},
		{"markdown file", "", "test.md", false},
		{"text file", "", "test.txt", false},
		{"no extension", "", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hp.CanProcess(tt.contentType, tt.path); got != tt.want {
				t.Errorf("CanProcess(%q, %q) = %v, want %v", tt.contentType, tt.path, got, tt.want)
			}
		})
	}
}

func TestHTMLProcessor_Process_WithHeadings(t *testing.T) {
	hp := &HTMLProcessor{}

	htmlContent := []byte(`<!DOCTYPE html>
<html>
<head>
	<title>Test Document</title>
	<meta name="description" content="A test document">
	<meta property="og:title" content="OG Test">
</head>
<body>
	<h1>Introduction</h1>
	<p>This is the introduction section.</p>
	<p>It has multiple paragraphs.</p>

	<h2>Getting Started</h2>
	<p>Here's how to get started.</p>

	<h2>Advanced Topics</h2>
	<p>Advanced content here.</p>

	<h3>Subsection</h3>
	<p>Subsection content.</p>
</body>
</html>`)

	sections, err := hp.Process("test.html", "", "https://example.com", htmlContent)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should have 4 sections (h1, h2, h2, h3)
	if len(sections) != 4 {
		t.Errorf("Expected 4 sections, got %d", len(sections))
	}

	// Check first section
	if sections[0].Title != "Test Document" {
		t.Errorf("First section title = %q, want %q", sections[0].Title, "Test Document")
	}
	if sections[0].Type != "html_section" {
		t.Errorf("First section type = %q, want %q", sections[0].Type, "html_section")
	}
	if !contains(sections[0].Content, "introduction section") {
		t.Errorf("First section content missing expected text")
	}
	if sections[0].URL != "https://example.com/test#introduction" {
		t.Errorf("First section URL = %q, want %q", sections[0].URL, "https://example.com/test#introduction")
	}

	// Check metadata
	if level, ok := sections[0].Metadata["heading_level"].(int); !ok || level != 1 {
		t.Errorf("First section heading_level = %v, want 1", sections[0].Metadata["heading_level"])
	}
	if title, ok := sections[0].Metadata["title"].(string); !ok || title != "Test Document" {
		t.Errorf("Metadata title = %q, want %q", title, "Test Document")
	}

	// Check second section
	if sections[1].Title != "Getting Started" {
		t.Errorf("Second section title = %q, want %q", sections[1].Title, "Getting Started")
	}
}

func TestHTMLProcessor_Process_NoHeadings(t *testing.T) {
	hp := &HTMLProcessor{}

	htmlContent := []byte(`<!DOCTYPE html>
<html>
<head>
	<title>Simple Page</title>
</head>
<body>
	<p>This is a simple page without headings.</p>
	<p>Just some content.</p>
</body>
</html>`)

	sections, err := hp.Process("simple.html", "", "https://example.com", htmlContent)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if len(sections) != 1 {
		t.Errorf("Expected 1 section, got %d", len(sections))
	}

	section := sections[0]
	if section.Title != "Simple Page" {
		t.Errorf("Section title = %q, want %q", section.Title, "Simple Page")
	}
	if section.Type != "html_document" {
		t.Errorf("Section type = %q, want %q", section.Type, "html_document")
	}
	if section.URL != "https://example.com/simple" {
		t.Errorf("Section URL = %q, want %q", section.URL, "https://example.com/simple")
	}
}

func TestHTMLProcessor_Process_DuplicateHeadings(t *testing.T) {
	hp := &HTMLProcessor{}

	htmlContent := []byte(`<!DOCTYPE html>
<html>
<body>
	<h1>Introduction</h1>
	<p>First intro.</p>

	<h2>Introduction</h2>
	<p>Second intro.</p>

	<h2>Introduction</h2>
	<p>Third intro.</p>
</body>
</html>`)

	sections, err := hp.Process("duplicate.html", "", "https://example.com", htmlContent)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if len(sections) != 3 {
		t.Fatalf("Expected 3 sections, got %d", len(sections))
	}

	expectedURLs := []string{
		"https://example.com/duplicate#introduction",
		"https://example.com/duplicate#introduction-1",
		"https://example.com/duplicate#introduction-2",
	}

	for i, expected := range expectedURLs {
		if sections[i].URL != expected {
			t.Errorf("Section %d URL = %q, want %q", i, sections[i].URL, expected)
		}
	}
}

func TestHTMLProcessor_Process_WithExistingIDs(t *testing.T) {
	hp := &HTMLProcessor{}

	htmlContent := []byte(`<!DOCTYPE html>
<html>
<body>
	<h1 id="custom-intro">Introduction</h1>
	<p>Content here.</p>

	<h2 id="setup">Getting Started</h2>
	<p>Setup content.</p>
</body>
</html>`)

	sections, err := hp.Process("with-ids.html", "", "https://example.com", htmlContent)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if sections[0].URL != "https://example.com/with-ids#custom-intro" {
		t.Errorf("First section URL = %q, want %q", sections[0].URL, "https://example.com/with-ids#custom-intro")
	}
	if sections[1].URL != "https://example.com/with-ids#setup" {
		t.Errorf("Second section URL = %q, want %q", sections[1].URL, "https://example.com/with-ids#setup")
	}
}

func TestHTMLProcessor_Process_SkipsScriptAndStyle(t *testing.T) {
	hp := &HTMLProcessor{}

	htmlContent := []byte(`<!DOCTYPE html>
<html>
<body>
	<h1>Content</h1>
	<p>Visible text.</p>
	<script>console.log('this should not appear');</script>
	<style>body { color: red; }</style>
	<noscript>Please enable JavaScript.</noscript>
	<p>More visible text.</p>
</body>
</html>`)

	sections, err := hp.Process("with-script.html", "", "", htmlContent)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	content := sections[0].Content
	if contains(content, "console.log") {
		t.Errorf("Content should not contain script content")
	}
	if contains(content, "color: red") {
		t.Errorf("Content should not contain style content")
	}
	if !contains(content, "Visible text") {
		t.Errorf("Content should contain visible text")
	}
}

func TestHTMLProcessor_Process_EmptyBaseURL(t *testing.T) {
	hp := &HTMLProcessor{}

	htmlContent := []byte(`<h1>Test</h1><p>Content</p>`)

	sections, err := hp.Process("test.html", "", "", htmlContent)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if sections[0].URL != "" {
		t.Errorf("URL should be empty when baseURL not provided, got %q", sections[0].URL)
	}
}

func TestSlugCounter_Unique(t *testing.T) {
	sc := newSlugCounter()

	if got := sc.unique("test"); got != "test" {
		t.Errorf("unique('test') = %q, want 'test'", got)
	}

	if got := sc.unique("test"); got != "test-1" {
		t.Errorf("unique('test') = %q, want 'test-1'", got)
	}

	if got := sc.unique("test"); got != "test-2" {
		t.Errorf("unique('test') = %q, want 'test-2'", got)
	}

	if got := sc.unique("other"); got != "other" {
		t.Errorf("unique('other') = %q, want 'other'", got)
	}
}

func TestTransformHTMLPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"test.html", "test"},
		{"test.htm", "test"},
		{"path/to/file.html", "path/to/file"},
		{"noextension", "noextension"},
		{"file.md", "file.md"},
	}

	for _, tt := range tests {
		if got := transformHTMLPath(tt.input); got != tt.want {
			t.Errorf("transformHTMLPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
