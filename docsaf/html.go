package docsaf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// HTMLProcessor processes HTML (.html, .htm) files using goquery.
// It chunks files into sections by headings and extracts metadata from the document head.
type HTMLProcessor struct{}

// CanProcess returns true for .html and .htm files.
func (hp *HTMLProcessor) CanProcess(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm")
}

// ProcessFile processes an HTML file and returns document sections.
// Files are chunked by headings (h1-h6), and metadata is extracted from the <head>.
func (hp *HTMLProcessor) ProcessFile(filePath, baseDir, baseURL string) ([]DocumentSection, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

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

	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract document-level metadata
	docMetadata := hp.extractMetadata(doc)

	// Extract sections by headings
	sections := hp.extractSections(doc, relPath, baseURL, docMetadata)

	return sections, nil
}

// extractMetadata extracts metadata from the HTML <head> section.
// It collects <title>, <meta> tags, Open Graph, and Twitter Card metadata.
func (hp *HTMLProcessor) extractMetadata(doc *goquery.Document) map[string]any {
	metadata := make(map[string]any)

	// Extract <title>
	if title := doc.Find("title").First().Text(); title != "" {
		metadata["title"] = strings.TrimSpace(title)
	}

	// Extract meta tags
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		// Standard meta tags: <meta name="description" content="...">
		if name, exists := s.Attr("name"); exists {
			if content, exists := s.Attr("content"); exists {
				metadata["meta_"+name] = content
			}
		}

		// Open Graph and Twitter Card: <meta property="og:title" content="...">
		if property, exists := s.Attr("property"); exists {
			if content, exists := s.Attr("content"); exists {
				if strings.HasPrefix(property, "og:") || strings.HasPrefix(property, "twitter:") {
					metadata[property] = content
				}
			}
		}
	})

	return metadata
}

// extractSections extracts document sections from the HTML document.
// It creates separate sections for each heading (h1-h6), extracting content
// between headings. If no headings are found, it creates a single section
// for the entire document.
func (hp *HTMLProcessor) extractSections(doc *goquery.Document, relPath, baseURL string,
	docMetadata map[string]any) []DocumentSection {

	var sections []DocumentSection
	slugs := newSlugCounter() // Track duplicate slugs

	// Find all headings (h1-h6)
	headings := doc.Find("h1, h2, h3, h4, h5, h6")

	if headings.Length() == 0 {
		// No headings - create single section for entire document
		return []DocumentSection{hp.createFullDocSection(doc, relPath, baseURL, docMetadata)}
	}

	// Process each heading and extract content until next heading
	headings.Each(func(i int, heading *goquery.Selection) {
		// Extract heading info
		headingText := strings.TrimSpace(heading.Text())
		headingLevel := hp.getHeadingLevel(heading)
		headingID := heading.AttrOr("id", "")

		// Generate slug (use existing ID or generate from text)
		slug := headingID
		if slug == "" {
			slug = generateSlug(headingText) // Reuse from markdown.go
		}
		slug = slugs.unique(slug) // Handle duplicates

		// Extract content between this heading and the next
		content := hp.extractSectionContent(heading)

		// Use document title for first section if available
		title := headingText
		if i == 0 {
			if docTitle, ok := docMetadata["title"].(string); ok && docTitle != "" {
				title = docTitle
			}
		}

		// Generate URL
		url := ""
		if baseURL != "" {
			cleanPath := transformHTMLPath(relPath)
			url = baseURL + "/" + cleanPath + "#" + slug
		}

		sections = append(sections, DocumentSection{
			ID:       generateID(relPath, headingText),
			FilePath: relPath,
			Title:    title,
			Content:  content,
			Type:     "html_section",
			URL:      url,
			Metadata: hp.mergeSectionMetadata(docMetadata, map[string]any{
				"heading_level": headingLevel,
				"heading_id":    headingID,
			}),
		})
	})

	return sections
}

// extractSectionContent extracts text content between a heading and the next heading.
// It stops at the next heading (any level), since each heading becomes its own section.
func (hp *HTMLProcessor) extractSectionContent(heading *goquery.Selection) string {
	var content strings.Builder

	// Traverse siblings until we hit another heading
	heading.NextAll().EachWithBreak(func(i int, s *goquery.Selection) bool {
		tagName := goquery.NodeName(s)

		// Stop at any heading (each heading is its own section)
		if hp.isHeading(tagName) {
			return false // Break iteration
		}

		// Skip script, style, noscript
		if tagName == "script" || tagName == "style" || tagName == "noscript" {
			return true // Continue
		}

		// Extract text content
		text := strings.TrimSpace(s.Text())
		if text != "" {
			content.WriteString(text)
			content.WriteString("\n\n")
		}

		return true // Continue
	})

	return strings.TrimSpace(content.String())
}

// createFullDocSection creates a single section for documents without headings.
func (hp *HTMLProcessor) createFullDocSection(doc *goquery.Document, relPath, baseURL string,
	docMetadata map[string]any) DocumentSection {

	// Clone document to avoid modifying original
	docClone := doc.Clone()

	// Remove script, style, noscript elements
	docClone.Find("script, style, noscript").Remove()

	// Extract text from body if it exists, otherwise from entire document
	content := ""
	if body := docClone.Find("body").First(); body.Length() > 0 {
		content = strings.TrimSpace(body.Text())
	} else {
		content = strings.TrimSpace(docClone.Text())
	}

	// Use title from metadata or filename
	title := filepath.Base(relPath)
	if docTitle, ok := docMetadata["title"].(string); ok && docTitle != "" {
		title = docTitle
	}

	// Generate URL (no anchor for documents without headings)
	url := ""
	if baseURL != "" {
		url = baseURL + "/" + transformHTMLPath(relPath)
	}

	return DocumentSection{
		ID:       generateID(relPath, "document"),
		FilePath: relPath,
		Title:    title,
		Content:  content,
		Type:     "html_document",
		URL:      url,
		Metadata: hp.mergeSectionMetadata(docMetadata, map[string]any{
			"no_headings": true,
		}),
	}
}

// isHeading returns true if the tag name is a heading (h1-h6).
func (hp *HTMLProcessor) isHeading(tagName string) bool {
	return tagName == "h1" || tagName == "h2" || tagName == "h3" ||
		tagName == "h4" || tagName == "h5" || tagName == "h6"
}

// getHeadingLevel extracts the heading level from a heading element.
// Returns 1 for h1, 2 for h2, etc. Returns 0 if not a heading.
func (hp *HTMLProcessor) getHeadingLevel(s *goquery.Selection) int {
	tagName := goquery.NodeName(s)
	if len(tagName) == 2 && tagName[0] == 'h' && tagName[1] >= '1' && tagName[1] <= '6' {
		return int(tagName[1] - '0') // "h2" -> 2
	}
	return 0
}

// mergeSectionMetadata merges document-level and section-level metadata.
func (hp *HTMLProcessor) mergeSectionMetadata(docMeta, sectionMeta map[string]any) map[string]any {
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

// transformHTMLPath removes .html/.htm extensions from the file path for cleaner URLs.
// Example: "content/docs/guide.html" -> "content/docs/guide"
func transformHTMLPath(relPath string) string {
	// Remove .html/.htm extension
	relPath = strings.TrimSuffix(relPath, ".html")
	relPath = strings.TrimSuffix(relPath, ".htm")
	return relPath
}

// slugCounter tracks slug usage to handle duplicate heading slugs.
type slugCounter struct {
	counts map[string]int
}

// newSlugCounter creates a new slug counter.
func newSlugCounter() *slugCounter {
	return &slugCounter{counts: make(map[string]int)}
}

// unique returns a unique slug by appending a counter for duplicates.
// First occurrence: "getting-started"
// Second occurrence: "getting-started-1"
// Third occurrence: "getting-started-2"
func (sc *slugCounter) unique(slug string) string {
	count, exists := sc.counts[slug]
	sc.counts[slug] = count + 1

	if !exists {
		return slug // First occurrence
	}
	return fmt.Sprintf("%s-%d", slug, count)
}
