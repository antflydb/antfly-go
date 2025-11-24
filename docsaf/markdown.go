package docsaf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

// MarkdownProcessor processes Markdown (.md) and MDX (.mdx) files using goldmark.
// It chunks files into sections by headings and extracts YAML frontmatter.
type MarkdownProcessor struct{}

// CanProcess returns true for .md and .mdx files.
func (mp *MarkdownProcessor) CanProcess(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".mdx")
}

// ProcessFile processes a Markdown or MDX file and returns document sections.
// Files are chunked by headings (h1, h2, etc.), and YAML frontmatter is extracted.
func (mp *MarkdownProcessor) ProcessFile(filePath, baseDir, baseURL string) ([]DocumentSection, error) {
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
	isMDX := strings.HasSuffix(strings.ToLower(filePath), ".mdx")

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
	err = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
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

				// Use frontmatter title if available for first section, otherwise use heading
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
				// Add frontmatter to metadata if present
				if frontmatter != nil {
					metadata["frontmatter"] = frontmatter
				}

				// Generate URL if baseURL is provided
				url := ""
				if baseURL != "" {
					slug := generateSlug(headingText)
					cleanPath := transformURLPath(relPath)
					url = baseURL + "/" + cleanPath + "#" + slug
				}

				currentSection = &DocumentSection{
					ID:       generateID(relPath, headingText),
					FilePath: relPath,
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

	// If no sections found (no headings), create one section for the entire file
	if len(sections) == 0 {
		docType := "markdown_section"
		if isMDX {
			docType = "mdx_section"
		}

		// Use frontmatter title if available, otherwise use filename
		title := filepath.Base(filePath)
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

		// Generate URL if baseURL is provided (no anchor for files without headings)
		url := ""
		if baseURL != "" {
			cleanPath := transformURLPath(relPath)
			url = baseURL + "/" + cleanPath
		}

		sections = append(sections, DocumentSection{
			ID:       generateID(relPath, filepath.Base(filePath)),
			FilePath: relPath,
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
// Returns the parsed frontmatter map and the content without frontmatter.
// Frontmatter must be at the beginning of the file between --- delimiters.
func extractFrontmatter(content []byte) (map[string]any, []byte) {
	// Check if content starts with ---
	if !bytes.HasPrefix(content, []byte("---\n")) && !bytes.HasPrefix(content, []byte("---\r\n")) {
		return nil, content
	}

	// Find the closing ---
	remaining := content[4:] // Skip opening "---\n"
	endIdx := bytes.Index(remaining, []byte("\n---\n"))
	if endIdx == -1 {
		endIdx = bytes.Index(remaining, []byte("\n---\r\n"))
		if endIdx == -1 {
			// No closing delimiter found
			return nil, content
		}
	}

	// Extract frontmatter YAML
	frontmatterYAML := remaining[:endIdx]
	var frontmatter map[string]any
	if err := yaml.Unmarshal(frontmatterYAML, &frontmatter); err != nil {
		// Failed to parse, return original content
		return nil, content
	}

	// Return content after frontmatter
	contentStart := 4 + endIdx + 5 // "---\n" + yaml + "\n---\n"
	if contentStart >= len(content) {
		return frontmatter, []byte{}
	}
	return frontmatter, content[contentStart:]
}

// generateID creates a unique ID for a section using SHA-256 hash.
// The ID format is: doc_<hash(filePath + identifier)[:16]>
func generateID(filePath, identifier string) string {
	hasher := sha256.New()
	hasher.Write([]byte(filePath + "|" + identifier))
	hash := hex.EncodeToString(hasher.Sum(nil))
	return "doc_" + hash[:16]
}

// generateSlug creates a GitHub-style URL slug from a heading.
// Example: "Getting Started" -> "getting-started"
func generateSlug(heading string) string {
	// Convert to lowercase
	slug := strings.ToLower(heading)

	// Replace spaces and special characters with hyphens
	var result strings.Builder
	for _, ch := range slug {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			result.WriteRune(ch)
		} else if ch == ' ' || ch == '-' || ch == '_' {
			result.WriteRune('-')
		}
		// Skip other special characters
	}

	// Remove duplicate hyphens and trim
	slugStr := result.String()
	slugStr = strings.ReplaceAll(slugStr, "--", "-")
	slugStr = strings.Trim(slugStr, "-")

	return slugStr
}

// transformURLPath removes .md/.mdx extensions from the file path for cleaner URLs.
// Example: "content/docs/downloads.mdx" -> "content/docs/downloads"
func transformURLPath(relPath string) string {
	// Remove .mdx or .md extension
	relPath = strings.TrimSuffix(relPath, ".mdx")
	relPath = strings.TrimSuffix(relPath, ".md")
	return relPath
}
