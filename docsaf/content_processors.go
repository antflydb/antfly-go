package docsaf

import (
	"bytes"
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// MarkdownContentProcessor processes Markdown content using goldmark.
// It chunks content into sections by headings and extracts YAML frontmatter.
type MarkdownContentProcessor struct{}

// CanProcess returns true for markdown content types or .md/.mdx extensions.
func (mp *MarkdownContentProcessor) CanProcess(contentType, path string) bool {
	// Check MIME type first
	if strings.Contains(contentType, "text/markdown") ||
		strings.Contains(contentType, "text/x-markdown") {
		return true
	}
	// Fall back to extension
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".mdx")
}

// ProcessContent processes markdown content and returns document sections.
func (mp *MarkdownContentProcessor) ProcessContent(path, sourceURL, baseURL string, content []byte) ([]DocumentSection, error) {
	isMDX := strings.HasSuffix(strings.ToLower(path), ".mdx")

	// Extract frontmatter if present
	frontmatter, contentWithoutFrontmatter := extractFrontmatter(content)

	// Parse markdown using goldmark
	md := goldmark.New()
	reader := text.NewReader(contentWithoutFrontmatter)
	doc := md.Parser().Parse(reader)

	var sections []DocumentSection
	var currentSection *DocumentSection
	var contentBuffer bytes.Buffer

	// Walk the AST and extract sections by headings
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if heading, ok := n.(*ast.Heading); ok {
				// Save previous section
				if currentSection != nil {
					currentSection.Content = strings.TrimSpace(contentBuffer.String())
					if currentSection.Content != "" {
						sections = append(sections, *currentSection)
					}
					contentBuffer.Reset()
				}

				// Extract heading text
				headingText := extractText(heading, contentWithoutFrontmatter)

				// Use frontmatter title if available for first section
				sectionTitle := headingText
				if len(sections) == 0 && frontmatter != nil {
					if title, ok := frontmatter["title"].(string); ok && title != "" {
						sectionTitle = title
					}
				}

				// Create new section
				docType := "markdown_section"
				if isMDX {
					docType = "mdx_section"
				}

				metadata := map[string]any{
					"heading_level": heading.Level,
					"is_mdx":        isMDX,
				}
				if frontmatter != nil {
					metadata["frontmatter"] = frontmatter
				}
				if sourceURL != "" {
					metadata["source_url"] = sourceURL
				}

				// Generate URL if baseURL is provided
				url := ""
				if baseURL != "" {
					slug := generateSlug(headingText)
					cleanPath := transformURLPath(path)
					url = baseURL + "/" + cleanPath + "#" + slug
				}

				currentSection = &DocumentSection{
					ID:       generateID(path, headingText),
					FilePath: path,
					Title:    sectionTitle,
					Type:     docType,
					URL:      url,
					Metadata: metadata,
				}
			}

			// Append content to current section
			if currentSection != nil {
				contentBuffer.Write(n.Text(contentWithoutFrontmatter))
				if _, ok := n.(*ast.Text); ok && n.NextSibling() == nil {
					contentBuffer.WriteString("\n")
				}
			}
		}
		return ast.WalkContinue, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse markdown: %w", err)
	}

	// Save last section
	if currentSection != nil {
		currentSection.Content = strings.TrimSpace(contentBuffer.String())
		if currentSection.Content != "" {
			sections = append(sections, *currentSection)
		}
	}

	// If no sections found (no headings), create one section for the entire content
	if len(sections) == 0 {
		docType := "markdown_section"
		if isMDX {
			docType = "mdx_section"
		}

		// Use frontmatter title if available, otherwise use filename
		title := filepath.Base(path)
		if frontmatter != nil {
			if fmTitle, ok := frontmatter["title"].(string); ok && fmTitle != "" {
				title = fmTitle
			}
		}

		metadata := map[string]any{
			"is_mdx":      isMDX,
			"no_headings": true,
		}
		if frontmatter != nil {
			metadata["frontmatter"] = frontmatter
		}
		if sourceURL != "" {
			metadata["source_url"] = sourceURL
		}

		// Generate URL if baseURL is provided
		url := ""
		if baseURL != "" {
			cleanPath := transformURLPath(path)
			url = baseURL + "/" + cleanPath
		}

		sections = append(sections, DocumentSection{
			ID:       generateID(path, filepath.Base(path)),
			FilePath: path,
			Title:    title,
			Content:  string(contentWithoutFrontmatter),
			Type:     docType,
			URL:      url,
			Metadata: metadata,
		})
	}

	return sections, nil
}

// HTMLContentProcessor processes HTML content using goquery.
// It chunks content into sections by headings and extracts metadata from the document head.
type HTMLContentProcessor struct{}

// CanProcess returns true for HTML content types or .html/.htm extensions.
func (hp *HTMLContentProcessor) CanProcess(contentType, path string) bool {
	// Check MIME type first
	if strings.Contains(contentType, "text/html") {
		return true
	}
	// Fall back to extension
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm")
}

// ProcessContent processes HTML content and returns document sections.
func (hp *HTMLContentProcessor) ProcessContent(path, sourceURL, baseURL string, content []byte) ([]DocumentSection, error) {
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
func (hp *HTMLContentProcessor) extractMetadata(doc *goquery.Document) map[string]any {
	metadata := make(map[string]any)

	// Extract <title>
	if title := doc.Find("title").First().Text(); title != "" {
		metadata["title"] = strings.TrimSpace(title)
	}

	// Extract meta tags
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
func (hp *HTMLContentProcessor) extractSections(doc *goquery.Document, path, baseURL string,
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
func (hp *HTMLContentProcessor) extractSectionContent(heading *goquery.Selection) string {
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
func (hp *HTMLContentProcessor) createFullDocSection(doc *goquery.Document, path, baseURL string,
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

func (hp *HTMLContentProcessor) isHeading(tagName string) bool {
	return tagName == "h1" || tagName == "h2" || tagName == "h3" ||
		tagName == "h4" || tagName == "h5" || tagName == "h6"
}

func (hp *HTMLContentProcessor) getHeadingLevel(s *goquery.Selection) int {
	tagName := goquery.NodeName(s)
	if len(tagName) == 2 && tagName[0] == 'h' && tagName[1] >= '1' && tagName[1] <= '6' {
		return int(tagName[1] - '0')
	}
	return 0
}

func (hp *HTMLContentProcessor) mergeSectionMetadata(docMeta, sectionMeta map[string]any) map[string]any {
	merged := make(map[string]any)
	for k, v := range docMeta {
		merged[k] = v
	}
	for k, v := range sectionMeta {
		merged[k] = v
	}
	return merged
}

// DetectContentType detects the MIME type from a file path and optional content.
func DetectContentType(path string, content []byte) string {
	// Try to detect from extension first
	ext := filepath.Ext(path)
	if ext != "" {
		mimeType := mime.TypeByExtension(ext)
		if mimeType != "" {
			return mimeType
		}
	}

	// Common extensions not always in mime database
	switch strings.ToLower(ext) {
	case ".md", ".markdown":
		return "text/markdown"
	case ".mdx":
		return "text/markdown"
	case ".html", ".htm":
		return "text/html"
	case ".pdf":
		return "application/pdf"
	case ".yaml", ".yml":
		return "application/x-yaml"
	case ".json":
		return "application/json"
	case ".txt":
		return "text/plain"
	}

	return "application/octet-stream"
}
