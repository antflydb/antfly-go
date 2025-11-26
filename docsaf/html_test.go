package docsaf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHTMLProcessor_CanProcess(t *testing.T) {
	hp := &HTMLProcessor{}

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{"html file", "test.html", true},
		{"htm file", "test.htm", true},
		{"HTML uppercase", "test.HTML", true},
		{"markdown file", "test.md", false},
		{"text file", "test.txt", false},
		{"no extension", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hp.CanProcess(tt.filePath); got != tt.want {
				t.Errorf("CanProcess(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestHTMLProcessor_ProcessFile_WithHeadings(t *testing.T) {
	hp := &HTMLProcessor{}

	// Create temporary HTML file with headings
	htmlContent := `<!DOCTYPE html>
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
</html>`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.html")
	if err := os.WriteFile(filePath, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	sections, err := hp.ProcessFile(filePath, tmpDir, "https://example.com")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	// Should have 4 sections (h1, h2, h2, h3)
	if len(sections) != 4 {
		t.Errorf("Expected 4 sections, got %d", len(sections))
	}

	// Check first section
	if sections[0].Title != "Test Document" { // Should use <title> from metadata
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
	if desc, ok := sections[0].Metadata["meta_description"].(string); !ok || desc != "A test document" {
		t.Errorf("Metadata description = %q, want %q", desc, "A test document")
	}

	// Check second section
	if sections[1].Title != "Getting Started" {
		t.Errorf("Second section title = %q, want %q", sections[1].Title, "Getting Started")
	}
	if !contains(sections[1].Content, "get started") {
		t.Errorf("Second section content missing expected text")
	}

	// Check third section (h2)
	if sections[2].Title != "Advanced Topics" {
		t.Errorf("Third section title = %q, want %q", sections[2].Title, "Advanced Topics")
	}
	if contains(sections[2].Content, "Subsection content") {
		t.Errorf("Third section should not contain h3 content (lower level heading)")
	}

	// Check fourth section (h3 subsection)
	if sections[3].Title != "Subsection" {
		t.Errorf("Fourth section title = %q, want %q", sections[3].Title, "Subsection")
	}
	if level, ok := sections[3].Metadata["heading_level"].(int); !ok || level != 3 {
		t.Errorf("Fourth section heading_level = %v, want 3", sections[3].Metadata["heading_level"])
	}
}

func TestHTMLProcessor_ProcessFile_NoHeadings(t *testing.T) {
	hp := &HTMLProcessor{}

	htmlContent := `<!DOCTYPE html>
<html>
<head>
	<title>Simple Page</title>
</head>
<body>
	<p>This is a simple page without headings.</p>
	<p>Just some content.</p>
</body>
</html>`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "simple.html")
	if err := os.WriteFile(filePath, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	sections, err := hp.ProcessFile(filePath, tmpDir, "https://example.com")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	// Should have 1 section for the entire document
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
	if !contains(section.Content, "simple page without headings") {
		t.Errorf("Section content missing expected text")
	}
	if section.URL != "https://example.com/simple" {
		t.Errorf("Section URL = %q, want %q", section.URL, "https://example.com/simple")
	}

	// Check no_headings metadata
	if noHeadings, ok := section.Metadata["no_headings"].(bool); !ok || !noHeadings {
		t.Errorf("Expected no_headings metadata to be true")
	}
}

func TestHTMLProcessor_ProcessFile_DuplicateHeadings(t *testing.T) {
	hp := &HTMLProcessor{}

	htmlContent := `<!DOCTYPE html>
<html>
<body>
	<h1>Introduction</h1>
	<p>First intro.</p>

	<h2>Introduction</h2>
	<p>Second intro.</p>

	<h2>Introduction</h2>
	<p>Third intro.</p>
</body>
</html>`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "duplicate.html")
	if err := os.WriteFile(filePath, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	sections, err := hp.ProcessFile(filePath, tmpDir, "https://example.com")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	if len(sections) != 3 {
		t.Fatalf("Expected 3 sections, got %d", len(sections))
	}

	// Check that URLs have unique anchors
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

func TestHTMLProcessor_ProcessFile_WithExistingIDs(t *testing.T) {
	hp := &HTMLProcessor{}

	htmlContent := `<!DOCTYPE html>
<html>
<body>
	<h1 id="custom-intro">Introduction</h1>
	<p>Content here.</p>

	<h2 id="setup">Getting Started</h2>
	<p>Setup content.</p>
</body>
</html>`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "with-ids.html")
	if err := os.WriteFile(filePath, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	sections, err := hp.ProcessFile(filePath, tmpDir, "https://example.com")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	// Check that existing IDs are used in URLs
	if sections[0].URL != "https://example.com/with-ids#custom-intro" {
		t.Errorf("First section URL = %q, want %q", sections[0].URL, "https://example.com/with-ids#custom-intro")
	}
	if sections[1].URL != "https://example.com/with-ids#setup" {
		t.Errorf("Second section URL = %q, want %q", sections[1].URL, "https://example.com/with-ids#setup")
	}

	// Check heading_id metadata
	if id, ok := sections[0].Metadata["heading_id"].(string); !ok || id != "custom-intro" {
		t.Errorf("First section heading_id = %q, want %q", id, "custom-intro")
	}
}

func TestHTMLProcessor_ProcessFile_SkipsScriptAndStyle(t *testing.T) {
	hp := &HTMLProcessor{}

	htmlContent := `<!DOCTYPE html>
<html>
<body>
	<h1>Content</h1>
	<p>Visible text.</p>
	<script>console.log('this should not appear');</script>
	<style>body { color: red; }</style>
	<noscript>Please enable JavaScript.</noscript>
	<p>More visible text.</p>
</body>
</html>`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "with-script.html")
	if err := os.WriteFile(filePath, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	sections, err := hp.ProcessFile(filePath, tmpDir, "")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	content := sections[0].Content
	if contains(content, "console.log") {
		t.Errorf("Content should not contain script content")
	}
	if contains(content, "color: red") {
		t.Errorf("Content should not contain style content")
	}
	if contains(content, "Please enable JavaScript") {
		t.Errorf("Content should not contain noscript content")
	}
	if !contains(content, "Visible text") {
		t.Errorf("Content should contain visible text")
	}
}

func TestHTMLProcessor_ProcessFile_HTMLFragment(t *testing.T) {
	hp := &HTMLProcessor{}

	// HTML fragment without <html>, <head>, <body>
	htmlContent := `<h1>Title</h1>
<p>Some content here.</p>
<h2>Section</h2>
<p>More content.</p>`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "fragment.html")
	if err := os.WriteFile(filePath, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	sections, err := hp.ProcessFile(filePath, tmpDir, "")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	// Should still parse and extract sections
	if len(sections) != 2 {
		t.Errorf("Expected 2 sections, got %d", len(sections))
	}

	if sections[0].Title != "Title" {
		t.Errorf("First section title = %q, want %q", sections[0].Title, "Title")
	}
}

func TestHTMLProcessor_ProcessFile_EmptyBaseURL(t *testing.T) {
	hp := &HTMLProcessor{}

	htmlContent := `<h1>Test</h1><p>Content</p>`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.html")
	if err := os.WriteFile(filePath, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	sections, err := hp.ProcessFile(filePath, tmpDir, "")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	// URLs should be empty when baseURL is not provided
	if sections[0].URL != "" {
		t.Errorf("URL should be empty when baseURL not provided, got %q", sections[0].URL)
	}
}

func TestHTMLProcessor_ExtractMetadata(t *testing.T) {
	hp := &HTMLProcessor{}

	htmlContent := `<!DOCTYPE html>
<html>
<head>
	<title>My Page Title</title>
	<meta name="description" content="Page description">
	<meta name="keywords" content="test, html">
	<meta property="og:title" content="OG Title">
	<meta property="og:description" content="OG Description">
	<meta property="twitter:card" content="summary">
</head>
<body></body>
</html>`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "metadata.html")
	if err := os.WriteFile(filePath, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	sections, err := hp.ProcessFile(filePath, tmpDir, "")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	metadata := sections[0].Metadata

	tests := []struct {
		key  string
		want string
	}{
		{"title", "My Page Title"},
		{"meta_description", "Page description"},
		{"meta_keywords", "test, html"},
		{"og:title", "OG Title"},
		{"og:description", "OG Description"},
		{"twitter:card", "summary"},
	}

	for _, tt := range tests {
		if got, ok := metadata[tt.key].(string); !ok || got != tt.want {
			t.Errorf("Metadata[%q] = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestSlugCounter_Unique(t *testing.T) {
	sc := newSlugCounter()

	// First occurrence should return original slug
	if got := sc.unique("test"); got != "test" {
		t.Errorf("unique('test') = %q, want 'test'", got)
	}

	// Second occurrence should append -1
	if got := sc.unique("test"); got != "test-1" {
		t.Errorf("unique('test') = %q, want 'test-1'", got)
	}

	// Third occurrence should append -2
	if got := sc.unique("test"); got != "test-2" {
		t.Errorf("unique('test') = %q, want 'test-2'", got)
	}

	// Different slug should return original
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
		{"path/to/file.htm", "path/to/file"},
		{"noextension", "noextension"},
		{"file.md", "file.md"}, // Only removes .html/.htm
	}

	for _, tt := range tests {
		if got := transformHTMLPath(tt.input); got != tt.want {
			t.Errorf("transformHTMLPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// Helper function to check if a string contains a substring
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
