package docsaf

import (
	"bytes"
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFProcessor processes PDF (.pdf) content using the ledongthuc/pdf library.
// It chunks content into sections by pages and extracts metadata from the PDF Info dictionary.
type PDFProcessor struct{}

// CanProcess returns true for PDF content types or .pdf extensions.
func (pp *PDFProcessor) CanProcess(contentType, path string) bool {
	if strings.Contains(contentType, "application/pdf") {
		return true
	}
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".pdf")
}

// Process processes PDF content and returns document sections.
// Each page becomes a separate section, with text extracted via GetPlainText().
func (pp *PDFProcessor) Process(path, sourceURL, baseURL string, content []byte) ([]DocumentSection, error) {
	// Create a reader from bytes
	reader, err := pdf.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF: %w", err)
	}

	// Extract document-level metadata
	docMetadata := pp.extractMetadata(reader, path)
	if sourceURL != "" {
		docMetadata["source_url"] = sourceURL
	}

	// Process each page
	totalPages := reader.NumPage()
	sections := make([]DocumentSection, 0, totalPages)

	for pageNum := 1; pageNum <= totalPages; pageNum++ {
		page := reader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		pageContent, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		pageContent = strings.TrimSpace(pageContent)

		if pageContent == "" {
			continue
		}

		title := pp.getSectionTitle(pageNum, docMetadata)

		url := ""
		if baseURL != "" {
			cleanPath := transformPDFPath(path)
			url = fmt.Sprintf("%s/%s#page-%d", baseURL, cleanPath, pageNum)
		}

		section := DocumentSection{
			ID:       generateID(path, fmt.Sprintf("page_%d", pageNum)),
			FilePath: path,
			Title:    title,
			Content:  pageContent,
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

// extractMetadata extracts metadata from the PDF Info dictionary.
func (pp *PDFProcessor) extractMetadata(reader *pdf.Reader, path string) map[string]any {
	metadata := make(map[string]any)

	trailer := reader.Trailer()
	if trailer.IsNull() {
		metadata["title"] = filepath.Base(path)
		return metadata
	}

	info := trailer.Key("Info")
	if info.IsNull() {
		metadata["title"] = filepath.Base(path)
		return metadata
	}

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

	if _, hasTitle := metadata["title"]; !hasTitle {
		metadata["title"] = filepath.Base(path)
	}

	return metadata
}

func (pp *PDFProcessor) getSectionTitle(pageNum int, docMetadata map[string]any) string {
	docTitle := "Document"
	if title, ok := docMetadata["title"].(string); ok && title != "" {
		docTitle = title
	}
	return fmt.Sprintf("%s - Page %d", docTitle, pageNum)
}

func (pp *PDFProcessor) mergeSectionMetadata(docMeta, sectionMeta map[string]any) map[string]any {
	merged := make(map[string]any)
	maps.Copy(merged, docMeta)
	maps.Copy(merged, sectionMeta)
	return merged
}

func transformPDFPath(path string) string {
	return strings.TrimSuffix(path, ".pdf")
}
