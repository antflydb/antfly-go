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
// Sections are merged if they would be too small (under MinTokensPerSection tokens).
type MarkdownProcessor struct {
	// MinTokensPerSection is the minimum token count before splitting into a new section.
	// If 0, defaults to 500 tokens. Set to 1 to split on every heading (original behavior).
	MinTokensPerSection int
}

// estimateTokens provides a rough token count estimate.
// Uses ~1.3 tokens per word as a heuristic for English text.
func estimateTokens(text string) int {
	words := len(strings.Fields(text))
	return int(float64(words) * 1.3)
}

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

// headingStackEntry represents a heading in the hierarchy stack
type headingStackEntry struct {
	level int
	title string
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
	var preambleBuffer bytes.Buffer // Buffer for content before first heading
	seenHeading := false

	// Track heading hierarchy for section_path
	var headingStack []headingStackEntry

	// Walk the AST and extract sections by headings
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if heading, ok := n.(*ast.Heading); ok {
				seenHeading = true

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

				// Update heading stack: pop entries with level >= current
				for len(headingStack) > 0 && headingStack[len(headingStack)-1].level >= heading.Level {
					headingStack = headingStack[:len(headingStack)-1]
				}
				// Push current heading onto stack
				headingStack = append(headingStack, headingStackEntry{
					level: heading.Level,
					title: headingText,
				})

				// Build section path from stack
				sectionPath := make([]string, len(headingStack))
				for i, entry := range headingStack {
					sectionPath[i] = entry.title
				}

				// Use frontmatter title if available for first section (only if no preamble content)
				sectionTitle := headingText
				if len(sections) == 0 && preambleBuffer.Len() == 0 && frontmatter != nil {
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

				// Use full section path for ID to ensure uniqueness (e.g., "Overview" may appear multiple times)
				sectionPathID := strings.Join(sectionPath, " > ")

				currentSection = &DocumentSection{
					ID:          generateID(path, sectionPathID),
					FilePath:    path,
					Title:       sectionTitle,
					Type:        docType,
					URL:         url,
					SectionPath: sectionPath,
					Metadata:    metadata,
				}

				// Skip appending heading text to content - it's already the Title
				return ast.WalkSkipChildren, nil
			}

			// Append content - use preambleBuffer before first heading, contentBuffer after
			// Only extract from leaf text nodes to avoid duplicating text from container nodes
			if textNode, ok := n.(*ast.Text); ok {
				targetBuffer := &preambleBuffer
				if seenHeading && currentSection != nil {
					targetBuffer = &contentBuffer
				}
				targetBuffer.Write(textNode.Segment.Value(contentWithoutFrontmatter))
				if textNode.SoftLineBreak() {
					targetBuffer.WriteString("\n")
				}
			}
			// Also extract content from HTML blocks (e.g., MDX components like <Questions>)
			if htmlBlock, ok := n.(*ast.HTMLBlock); ok {
				targetBuffer := &preambleBuffer
				if seenHeading && currentSection != nil {
					targetBuffer = &contentBuffer
				}
				for i := 0; i < htmlBlock.Lines().Len(); i++ {
					line := htmlBlock.Lines().At(i)
					targetBuffer.Write(line.Value(contentWithoutFrontmatter))
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

	// Create preamble section for content before first heading (if any)
	preambleContent := strings.TrimSpace(preambleBuffer.String())
	if preambleContent != "" && len(sections) > 0 {
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
			"is_mdx":   isMDX,
			"preamble": true,
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

		preambleSection := DocumentSection{
			ID:       generateID(path, "_preamble"),
			FilePath: path,
			Title:    title,
			Content:  preambleContent,
			Type:     docType,
			URL:      url,
			Metadata: metadata,
		}

		// Prepend preamble section to the beginning
		sections = append([]DocumentSection{preambleSection}, sections...)
	}

	// Merge small sections to ensure minimum token count
	minTokens := mp.MinTokensPerSection
	if minTokens == 0 {
		minTokens = 500 // default
	}
	if minTokens > 1 && len(sections) > 1 {
		sections = mergeSmallerSections(sections, minTokens)
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

// mergeSmallerSections merges consecutive sections that are below the minimum token threshold.
// It combines small sections with the next section to ensure each chunk has enough context.
func mergeSmallerSections(sections []DocumentSection, minTokens int) []DocumentSection {
	if len(sections) <= 1 {
		return sections
	}

	var merged []DocumentSection
	var accumulator *DocumentSection
	var accumulatedContent strings.Builder

	for i, section := range sections {
		if accumulator == nil {
			// Start a new accumulator
			accumulator = &DocumentSection{
				ID:          section.ID,
				FilePath:    section.FilePath,
				Title:       section.Title,
				Type:        section.Type,
				URL:         section.URL,
				SectionPath: section.SectionPath,
				Metadata:    section.Metadata,
			}
			accumulatedContent.WriteString(section.Content)
		} else {
			// Append to accumulator
			accumulatedContent.WriteString("\n\n")
			accumulatedContent.WriteString(section.Content)
		}

		accumulatedTokens := estimateTokens(accumulatedContent.String())

		// Flush if we've reached the threshold or this is the last section
		if accumulatedTokens >= minTokens || i == len(sections)-1 {
			accumulator.Content = strings.TrimSpace(accumulatedContent.String())
			merged = append(merged, *accumulator)
			accumulator = nil
			accumulatedContent.Reset()
		}
	}

	return merged
}

// transformURLPath removes .md/.mdx extensions from the path for cleaner URLs.
func transformURLPath(path string) string {
	path = strings.TrimSuffix(path, ".mdx")
	path = strings.TrimSuffix(path, ".md")
	return path
}

// ExtractQuestions extracts questions from markdown/MDX content.
// It looks for questions in:
// 1. Frontmatter "questions" field
// 2. <Questions> MDX components inline in the content
func (mp *MarkdownProcessor) ExtractQuestions(path, sourceURL string, content []byte) []Question {
	extractor := &QuestionsExtractor{}

	// Extract frontmatter
	frontmatter, contentWithoutFrontmatter := extractFrontmatter(content)

	return extractor.ExtractFromMDXContent(path, sourceURL, contentWithoutFrontmatter, frontmatter)
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
