package docsaf

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// HTMLProcessor processes HTML (.html, .htm) content using goquery.
// It chunks content into sections by headings and extracts metadata from the document head.
type HTMLProcessor struct{}

// CanProcess returns true for HTML content types or .html/.htm extensions.
func (hp *HTMLProcessor) CanProcess(contentType, path string) bool {
	// Check MIME type first
	if strings.Contains(contentType, "text/html") {
		return true
	}
	// Fall back to extension
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm")
}

// Process processes HTML content and returns document sections.
func (hp *HTMLProcessor) Process(path, sourceURL, baseURL string, content []byte) ([]DocumentSection, error) {
	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract document-level metadata
	docMetadata := hp.extractMetadata(doc)
	if sourceURL != "" {
		docMetadata["source_url"] = sourceURL
	}

	// Extract sections by headings
	sections := hp.extractSections(doc, path, baseURL, docMetadata)

	return sections, nil
}

// extractMetadata extracts metadata from the HTML <head> section.
func (hp *HTMLProcessor) extractMetadata(doc *goquery.Document) map[string]any {
	metadata := make(map[string]any)

	if title := doc.Find("title").First().Text(); title != "" {
		metadata["title"] = strings.TrimSpace(title)
	}

	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if name, exists := s.Attr("name"); exists {
			if content, exists := s.Attr("content"); exists {
				metadata["meta_"+name] = content
			}
		}

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
func (hp *HTMLProcessor) extractSections(doc *goquery.Document, path, baseURL string,
	docMetadata map[string]any) []DocumentSection {

	var sections []DocumentSection
	slugs := newSlugCounter()

	headings := doc.Find("h1, h2, h3, h4, h5, h6")

	if headings.Length() == 0 {
		return []DocumentSection{hp.createFullDocSection(doc, path, baseURL, docMetadata)}
	}

	headings.Each(func(i int, heading *goquery.Selection) {
		headingText := strings.TrimSpace(heading.Text())
		headingLevel := hp.getHeadingLevel(heading)
		headingID := heading.AttrOr("id", "")

		slug := headingID
		if slug == "" {
			slug = generateSlug(headingText)
		}
		slug = slugs.unique(slug)

		content := hp.extractSectionContent(heading)

		title := headingText
		if i == 0 {
			if docTitle, ok := docMetadata["title"].(string); ok && docTitle != "" {
				title = docTitle
			}
		}

		url := ""
		if baseURL != "" {
			cleanPath := transformHTMLPath(path)
			url = baseURL + "/" + cleanPath + "#" + slug
		}

		sections = append(sections, DocumentSection{
			ID:       generateID(path, headingText),
			FilePath: path,
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
func (hp *HTMLProcessor) extractSectionContent(heading *goquery.Selection) string {
	var content strings.Builder

	heading.NextAll().EachWithBreak(func(i int, s *goquery.Selection) bool {
		tagName := goquery.NodeName(s)

		if hp.isHeading(tagName) {
			return false
		}

		if tagName == "script" || tagName == "style" || tagName == "noscript" {
			return true
		}

		text := strings.TrimSpace(s.Text())
		if text != "" {
			content.WriteString(text)
			content.WriteString("\n\n")
		}

		return true
	})

	return strings.TrimSpace(content.String())
}

// createFullDocSection creates a single section for documents without headings.
func (hp *HTMLProcessor) createFullDocSection(doc *goquery.Document, path, baseURL string,
	docMetadata map[string]any) DocumentSection {

	docClone := doc.Clone()
	docClone.Find("script, style, noscript").Remove()

	content := ""
	if body := docClone.Find("body").First(); body.Length() > 0 {
		content = strings.TrimSpace(body.Text())
	} else {
		content = strings.TrimSpace(docClone.Text())
	}

	title := filepath.Base(path)
	if docTitle, ok := docMetadata["title"].(string); ok && docTitle != "" {
		title = docTitle
	}

	url := ""
	if baseURL != "" {
		url = baseURL + "/" + transformHTMLPath(path)
	}

	return DocumentSection{
		ID:       generateID(path, "document"),
		FilePath: path,
		Title:    title,
		Content:  content,
		Type:     "html_document",
		URL:      url,
		Metadata: hp.mergeSectionMetadata(docMetadata, map[string]any{
			"no_headings": true,
		}),
	}
}

func (hp *HTMLProcessor) isHeading(tagName string) bool {
	return tagName == "h1" || tagName == "h2" || tagName == "h3" ||
		tagName == "h4" || tagName == "h5" || tagName == "h6"
}

func (hp *HTMLProcessor) getHeadingLevel(s *goquery.Selection) int {
	tagName := goquery.NodeName(s)
	if len(tagName) == 2 && tagName[0] == 'h' && tagName[1] >= '1' && tagName[1] <= '6' {
		return int(tagName[1] - '0')
	}
	return 0
}

func (hp *HTMLProcessor) mergeSectionMetadata(docMeta, sectionMeta map[string]any) map[string]any {
	merged := make(map[string]any)
	for k, v := range docMeta {
		merged[k] = v
	}
	for k, v := range sectionMeta {
		merged[k] = v
	}
	return merged
}

// transformHTMLPath removes .html/.htm extensions from the path for cleaner URLs.
func transformHTMLPath(path string) string {
	path = strings.TrimSuffix(path, ".html")
	path = strings.TrimSuffix(path, ".htm")
	return path
}

// slugCounter tracks slug usage to handle duplicate heading slugs.
type slugCounter struct {
	counts map[string]int
}

func newSlugCounter() *slugCounter {
	return &slugCounter{counts: make(map[string]int)}
}

func (sc *slugCounter) unique(slug string) string {
	count, exists := sc.counts[slug]
	sc.counts[slug] = count + 1

	if !exists {
		return slug
	}
	return fmt.Sprintf("%s-%d", slug, count)
}
