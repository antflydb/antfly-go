package docsaf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
func (mp *MarkdownProcessor) ProcessFile(filePath, baseDir string) ([]DocumentSection, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	relPath, _ := filepath.Rel(baseDir, filePath)
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

				currentSection = &DocumentSection{
					ID:        generateID(relPath, headingText),
					FilePath:  relPath,
					Title:     sectionTitle,
					Type:      docType,
					Metadata:  metadata,
					CreatedAt: time.Now(),
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

		sections = append(sections, DocumentSection{
			ID:        generateID(relPath, filepath.Base(filePath)),
			FilePath:  relPath,
			Title:     title,
			Content:   string(contentWithoutFrontmatter),
			Type:      docType,
			Metadata:  metadata,
			CreatedAt: time.Now(),
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
