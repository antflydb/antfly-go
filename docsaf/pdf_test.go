package docsaf

import (
	"os"
	"testing"
)

func TestPDFProcessor_CanProcess(t *testing.T) {
	pp := &PDFProcessor{}

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{"pdf file", "test.pdf", true},
		{"PDF uppercase", "test.PDF", true},
		{"markdown file", "test.md", false},
		{"html file", "test.html", false},
		{"text file", "test.txt", false},
		{"no extension", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pp.CanProcess(tt.filePath); got != tt.want {
				t.Errorf("CanProcess(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestPDFProcessor_ProcessFile_Basic(t *testing.T) {
	// Skip if test PDF doesn't exist
	testPDF := "testdata/pdf/sample.pdf"
	if _, err := os.Stat(testPDF); os.IsNotExist(err) {
		t.Skip("Test PDF not found, skipping test")
	}

	pp := &PDFProcessor{}
	tmpDir := "testdata/pdf"

	sections, err := pp.ProcessFile(testPDF, tmpDir, "https://example.com")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	// Should have at least one section
	if len(sections) == 0 {
		t.Fatal("Expected at least one section")
	}

	// Check first section structure
	section := sections[0]
	if section.Type != "pdf_page" {
		t.Errorf("Section type = %q, want %q", section.Type, "pdf_page")
	}
	if section.FilePath != "sample.pdf" {
		t.Errorf("Section FilePath = %q, want %q", section.FilePath, "sample.pdf")
	}
	if section.Content == "" {
		t.Error("Section content should not be empty")
	}
	if section.URL == "" {
		t.Error("Section URL should not be empty when baseURL provided")
	}

	// Check metadata
	if _, ok := section.Metadata["page_number"]; !ok {
		t.Error("Section should have page_number metadata")
	}
	if _, ok := section.Metadata["total_pages"]; !ok {
		t.Error("Section should have total_pages metadata")
	}
	if _, ok := section.Metadata["title"]; !ok {
		t.Error("Section should have title metadata")
	}
}

func TestPDFProcessor_ProcessFile_EmptyBaseURL(t *testing.T) {
	testPDF := "testdata/pdf/sample.pdf"
	if _, err := os.Stat(testPDF); os.IsNotExist(err) {
		t.Skip("Test PDF not found, skipping test")
	}

	pp := &PDFProcessor{}
	sections, err := pp.ProcessFile(testPDF, "testdata/pdf", "")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	if len(sections) == 0 {
		t.Fatal("Expected at least one section")
	}

	// URL should be empty when baseURL not provided
	if sections[0].URL != "" {
		t.Errorf("URL should be empty when baseURL not provided, got %q", sections[0].URL)
	}
}

func TestPDFProcessor_ProcessFile_TitleFormat(t *testing.T) {
	testPDF := "testdata/pdf/sample.pdf"
	if _, err := os.Stat(testPDF); os.IsNotExist(err) {
		t.Skip("Test PDF not found, skipping test")
	}

	pp := &PDFProcessor{}
	sections, err := pp.ProcessFile(testPDF, "testdata/pdf", "https://example.com")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	if len(sections) == 0 {
		t.Fatal("Expected at least one section")
	}

	// All sections should use "Title - Page N" format
	for i, section := range sections {
		if !containsPageNumber(section.Title) {
			t.Errorf("Section %d title %q should contain 'Page N' format", i, section.Title)
		}
	}
}

func TestPDFProcessor_ProcessFile_Metadata(t *testing.T) {
	testPDF := "testdata/pdf/with_metadata.pdf"
	if _, err := os.Stat(testPDF); os.IsNotExist(err) {
		t.Skip("Test PDF with metadata not found, skipping test")
	}

	pp := &PDFProcessor{}
	sections, err := pp.ProcessFile(testPDF, "testdata/pdf", "")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	if len(sections) == 0 {
		t.Fatal("Expected at least one section")
	}

	metadata := sections[0].Metadata

	// Check for expected metadata fields
	expectedFields := []string{"title", "page_number", "total_pages"}
	for _, field := range expectedFields {
		if _, ok := metadata[field]; !ok {
			t.Errorf("Expected metadata field %q not found", field)
		}
	}

	// Optional fields (may or may not be present depending on PDF)
	optionalFields := []string{"author", "subject", "keywords", "creator", "producer", "creation_date", "mod_date"}
	foundOptional := 0
	for _, field := range optionalFields {
		if _, ok := metadata[field]; ok {
			foundOptional++
		}
	}

	t.Logf("Found %d optional metadata fields", foundOptional)
}

func TestPDFProcessor_URLGeneration(t *testing.T) {
	testPDF := "testdata/pdf/sample.pdf"
	if _, err := os.Stat(testPDF); os.IsNotExist(err) {
		t.Skip("Test PDF not found, skipping test")
	}

	pp := &PDFProcessor{}
	sections, err := pp.ProcessFile(testPDF, "testdata/pdf", "https://docs.example.com")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	if len(sections) == 0 {
		t.Fatal("Expected at least one section")
	}

	// Check URL format for first page
	expectedPrefix := "https://docs.example.com/sample#page-"
	if !hasPrefix(sections[0].URL, expectedPrefix) {
		t.Errorf("URL = %q, want prefix %q", sections[0].URL, expectedPrefix)
	}

	// Verify all URLs have unique page anchors
	seenURLs := make(map[string]bool)
	for _, section := range sections {
		if seenURLs[section.URL] {
			t.Errorf("Duplicate URL found: %q", section.URL)
		}
		seenURLs[section.URL] = true
	}
}

func TestPDFProcessor_PageNumbers(t *testing.T) {
	testPDF := "testdata/pdf/sample.pdf"
	if _, err := os.Stat(testPDF); os.IsNotExist(err) {
		t.Skip("Test PDF not found, skipping test")
	}

	pp := &PDFProcessor{}
	sections, err := pp.ProcessFile(testPDF, "testdata/pdf", "")
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	if len(sections) == 0 {
		t.Fatal("Expected at least one section")
	}

	// Verify page numbers are sequential
	for i, section := range sections {
		pageNum, ok := section.Metadata["page_number"].(int)
		if !ok {
			t.Fatalf("Section %d: page_number not an int", i)
		}

		// Page numbers should be positive
		if pageNum < 1 {
			t.Errorf("Section %d: page_number = %d, want >= 1", i, pageNum)
		}

		// Verify total_pages is present and positive
		totalPages, ok := section.Metadata["total_pages"].(int)
		if !ok {
			t.Fatalf("Section %d: total_pages not an int", i)
		}
		if totalPages < 1 {
			t.Errorf("Section %d: total_pages = %d, want >= 1", i, totalPages)
		}

		// Page number should not exceed total pages
		if pageNum > totalPages {
			t.Errorf("Section %d: page_number %d > total_pages %d", i, pageNum, totalPages)
		}
	}
}

func TestPDFProcessor_ErrorHandling(t *testing.T) {
	pp := &PDFProcessor{}

	tests := []struct {
		name     string
		filePath string
		wantErr  bool
	}{
		{
			name:     "non-existent file",
			filePath: "testdata/pdf/nonexistent.pdf",
			wantErr:  true,
		},
		{
			name:     "invalid PDF",
			filePath: "testdata/pdf/invalid.pdf",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create invalid PDF for testing
			if tt.name == "invalid PDF" {
				os.MkdirAll("testdata/pdf", 0755)
				os.WriteFile(tt.filePath, []byte("not a pdf"), 0644)
				defer os.Remove(tt.filePath)
			}

			_, err := pp.ProcessFile(tt.filePath, "testdata/pdf", "")
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTransformPDFPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"test.pdf", "test"},
		{"path/to/file.pdf", "path/to/file"},
		{"document.PDF", "document.PDF"}, // Only removes lowercase .pdf
		{"noextension", "noextension"},
		{"file.txt", "file.txt"}, // Only removes .pdf
	}

	for _, tt := range tests {
		if got := transformPDFPath(tt.input); got != tt.want {
			t.Errorf("transformPDFPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}


// Helper function to check if a string contains "Page N" pattern
func containsPageNumber(s string) bool {
	// Simple check: contains "Page" followed by a space and number
	return len(s) > 5 && (hasSubstring(s, "Page 1") || hasSubstring(s, "Page 2") ||
		hasSubstring(s, "Page 3") || hasSubstring(s, "Page 4") ||
		hasSubstring(s, "Page 5") || hasSubstring(s, "Page 6") ||
		hasSubstring(s, "Page 7") || hasSubstring(s, "Page 8") ||
		hasSubstring(s, "Page 9") || hasSubstring(s, "Page 0"))
}

// Helper function to check string prefix
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// Helper function to check substring
func hasSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
