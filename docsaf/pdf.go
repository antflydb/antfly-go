package docsaf

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFProcessor processes PDF (.pdf) files using the ledongthuc/pdf library.
// It chunks files into sections by pages and extracts metadata from the PDF Info dictionary.
type PDFProcessor struct{}

// CanProcess returns true for .pdf files.
func (pp *PDFProcessor) CanProcess(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.HasSuffix(lower, ".pdf")
}

// ProcessFile processes a PDF file and returns document sections.
// Each page becomes a separate section, with text extracted via GetPlainText().
// Pages without extractable text are skipped.
func (pp *PDFProcessor) ProcessFile(filePath, baseDir, baseURL string) ([]DocumentSection, error) {
	// Convert baseDir to absolute path to ensure correct relative path calculation
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		absBaseDir = baseDir
	}

	// Convert filePath to absolute path
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		absFilePath = filePath
	}

	relPath, _ := filepath.Rel(absBaseDir, absFilePath)

	// Open PDF file
	file, reader, err := pdf.Open(absFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %w", err)
	}
	defer file.Close()

	// Extract document-level metadata
	docMetadata := pp.extractMetadata(reader, relPath)

	// Process each page
	totalPages := reader.NumPage()
	sections := make([]DocumentSection, 0, totalPages)

	for pageNum := 1; pageNum <= totalPages; pageNum++ {
		page := reader.Page(pageNum)
		if page.V.IsNull() {
			continue // Skip invalid pages
		}

		// Extract text content
		content, err := page.GetPlainText(nil)
		if err != nil {
			continue // Skip pages with extraction errors
		}
		content = strings.TrimSpace(content)

		// Skip empty pages
		if content == "" {
			continue
		}

		// Determine section title
		title := pp.getSectionTitle(pageNum, docMetadata)

		// Generate URL with page anchor
		url := ""
		if baseURL != "" {
			cleanPath := transformPDFPath(relPath)
			url = fmt.Sprintf("%s/%s#page-%d", baseURL, cleanPath, pageNum)
		}

		// Create section
		section := DocumentSection{
			ID:       generateID(relPath, fmt.Sprintf("page_%d", pageNum)),
			FilePath: relPath,
			Title:    title,
			Content:  content,
			Type:     "pdf_page",
			URL:      url,
			Metadata: pp.mergeSectionMetadata(docMetadata, map[string]any{
				"page_number": pageNum,
				"total_pages": totalPages,
			}),
		}

		sections = append(sections, section)
	}

	return sections, nil
}

// pdfMetadata holds extracted PDF document metadata.
type pdfMetadata struct {
	Title        string
	Author       string
	Subject      string
	Keywords     string
	Creator      string
	Producer     string
	CreationDate string
	ModDate      string
}

// extractMetadata extracts metadata from the PDF Info dictionary.
// Returns all available metadata fields, using filename as fallback for title.
func (pp *PDFProcessor) extractMetadata(reader *pdf.Reader, relPath string) map[string]any {
	metadata := make(map[string]any)

	// Access Info dictionary through Trailer
	trailer := reader.Trailer()
	if trailer.IsNull() {
		// No trailer, use filename as title
		metadata["title"] = filepath.Base(relPath)
		return metadata
	}

	info := trailer.Key("Info")
	if info.IsNull() {
		// No Info dict, use filename as title
		metadata["title"] = filepath.Base(relPath)
		return metadata
	}

	// Extract standard PDF metadata fields
	if title := info.Key("Title"); !title.IsNull() {
		if titleStr := title.Text(); titleStr != "" {
			metadata["title"] = titleStr
		}
	}

	if author := info.Key("Author"); !author.IsNull() {
		if authorStr := author.Text(); authorStr != "" {
			metadata["author"] = authorStr
		}
	}

	if subject := info.Key("Subject"); !subject.IsNull() {
		if subjectStr := subject.Text(); subjectStr != "" {
			metadata["subject"] = subjectStr
		}
	}

	if keywords := info.Key("Keywords"); !keywords.IsNull() {
		if keywordsStr := keywords.Text(); keywordsStr != "" {
			metadata["keywords"] = keywordsStr
		}
	}

	if creator := info.Key("Creator"); !creator.IsNull() {
		if creatorStr := creator.Text(); creatorStr != "" {
			metadata["creator"] = creatorStr
		}
	}

	if producer := info.Key("Producer"); !producer.IsNull() {
		if producerStr := producer.Text(); producerStr != "" {
			metadata["producer"] = producerStr
		}
	}

	if creationDate := info.Key("CreationDate"); !creationDate.IsNull() {
		if dateStr := creationDate.Text(); dateStr != "" {
			metadata["creation_date"] = dateStr
		}
	}

	if modDate := info.Key("ModDate"); !modDate.IsNull() {
		if dateStr := modDate.Text(); dateStr != "" {
			metadata["mod_date"] = dateStr
		}
	}

	// Fallback to filename if title not found
	if _, hasTitle := metadata["title"]; !hasTitle {
		metadata["title"] = filepath.Base(relPath)
	}

	return metadata
}

// getSectionTitle determines the section title for a page.
// Returns "DocumentTitle - Page N" format.
// Note: The ledongthuc/pdf library's Outline structure does not provide page number mappings,
// so we cannot use bookmark titles for section names.
func (pp *PDFProcessor) getSectionTitle(pageNum int, docMetadata map[string]any) string {
	// Use "DocumentTitle - Page N" format
	docTitle := "Document"
	if title, ok := docMetadata["title"].(string); ok && title != "" {
		docTitle = title
	}

	return fmt.Sprintf("%s - Page %d", docTitle, pageNum)
}

// mergeSectionMetadata merges document-level and section-level metadata.
func (pp *PDFProcessor) mergeSectionMetadata(docMeta, sectionMeta map[string]any) map[string]any {
	merged := make(map[string]any)

	// Copy document-level metadata
	for k, v := range docMeta {
		merged[k] = v
	}

	// Overlay section-level metadata
	for k, v := range sectionMeta {
		merged[k] = v
	}

	return merged
}

// transformPDFPath removes .pdf extension from the file path for cleaner URLs.
// Example: "content/docs/guide.pdf" -> "content/docs/guide"
func transformPDFPath(relPath string) string {
	return strings.TrimSuffix(relPath, ".pdf")
}
