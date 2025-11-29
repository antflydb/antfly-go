package docsaf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

// MarkdownProcessor processes Markdown (.md) and MDX (.mdx) content using goldmark.
// It chunks content into sections by headings and extracts YAML frontmatter.
type MarkdownProcessor struct{}

// CanProcess returns true for markdown content types or .md/.mdx extensions.
func (mp *MarkdownProcessor) CanProcess(contentType, path string) bool {
	// Check MIME type first
	if strings.Contains(contentType, "text/markdown") ||
		strings.Contains(contentType, "text/x-markdown") {
		return true
	}
	// Fall back to extension
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".mdx")
}

// Process processes markdown content and returns document sections.
func (mp *MarkdownProcessor) Process(path, sourceURL, baseURL string, content []byte) ([]DocumentSection, error) {
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

// extractText extracts text content from an AST node.
func extractText(node ast.Node, source []byte) string {
	var buf bytes.Buffer
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		buf.Write(child.Text(source))
	}
	return strings.TrimSpace(buf.String())
}

// extractFrontmatter extracts YAML frontmatter from markdown content.
func extractFrontmatter(content []byte) (map[string]any, []byte) {
	if !bytes.HasPrefix(content, []byte("---\n")) && !bytes.HasPrefix(content, []byte("---\r\n")) {
		return nil, content
	}

	remaining := content[4:]
	endIdx := bytes.Index(remaining, []byte("\n---\n"))
	if endIdx == -1 {
		endIdx = bytes.Index(remaining, []byte("\n---\r\n"))
		if endIdx == -1 {
			return nil, content
		}
	}

	frontmatterYAML := remaining[:endIdx]
	var frontmatter map[string]any
	if err := yaml.Unmarshal(frontmatterYAML, &frontmatter); err != nil {
		return nil, content
	}

	contentStart := 4 + endIdx + 5
	if contentStart >= len(content) {
		return frontmatter, []byte{}
	}
	return frontmatter, content[contentStart:]
}

// generateID creates a unique ID for a section using SHA-256 hash.
func generateID(path, identifier string) string {
	hasher := sha256.New()
	hasher.Write([]byte(path + "|" + identifier))
	hash := hex.EncodeToString(hasher.Sum(nil))
	return "doc_" + hash[:16]
}

// generateSlug creates a GitHub-style URL slug from a heading.
func generateSlug(heading string) string {
	slug := strings.ToLower(heading)

	var result strings.Builder
	for _, ch := range slug {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			result.WriteRune(ch)
		} else if ch == ' ' || ch == '-' || ch == '_' {
			result.WriteRune('-')
		}
	}

	slugStr := result.String()
	slugStr = strings.ReplaceAll(slugStr, "--", "-")
	slugStr = strings.Trim(slugStr, "-")

	return slugStr
}

// transformURLPath removes .md/.mdx extensions from the path for cleaner URLs.
func transformURLPath(path string) string {
	path = strings.TrimSuffix(path, ".mdx")
	path = strings.TrimSuffix(path, ".md")
	return path
}

// DetectContentType detects the MIME type from a file path.
func DetectContentType(path string, content []byte) string {
	ext := filepath.Ext(path)
	if ext != "" {
		mimeType := mime.TypeByExtension(ext)
		if mimeType != "" {
			return mimeType
		}
	}

	switch strings.ToLower(ext) {
	case ".md", ".markdown", ".mdx":
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
