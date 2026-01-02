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
// Each page becomes a separate section, with text extracted via GetTextByRow()
// for better handling of tables and complex layouts.
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

		// Try both extraction methods and pick the better result
		// GetPlainText works better for most text, but fails on tables
		// extractTextByRow handles tables but can scramble some text
		plainText, plainErr := page.GetPlainText(nil)
		rowText := pp.extractTextByRow(page)

		pageContent := pp.chooseBestExtraction(plainText, rowText, plainErr)
		pageContent = cleanPDFText(pageContent)
		pageContent = stripGarbledHeaders(pageContent)

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

// chooseBestExtraction picks the better extraction result between
// GetPlainText and extractTextByRow methods.
// - Uses rowText if plainText has concatenation issues (many long words without spaces)
// - Uses plainText if rowText has scrambling issues (reversed character sequences)
// - Falls back to plainText if rowText is empty or has obvious errors
func (pp *PDFProcessor) chooseBestExtraction(plainText, rowText string, plainErr error) string {
	// If row extraction failed, use plain text
	if rowText == "" {
		if plainErr != nil {
			return ""
		}
		return plainText
	}

	// If plain text failed, use row text
	if plainErr != nil || plainText == "" {
		return rowText
	}

	// Check for concatenation issues in plain text (suggests table/column layout)
	// Long stretches without spaces indicate missing word boundaries
	plainHasConcatIssue := pp.hasConcatenationIssues(plainText)

	// Check for scrambling issues in row text (reversed characters)
	rowHasScrambling := pp.hasScrambledText(rowText)

	// Decision logic:
	// 1. If plain text has concatenation issues and row text doesn't have scrambling, use row
	// 2. If row text has scrambling, prefer plain text
	// 3. Default to plain text (it's usually more reliable)
	if plainHasConcatIssue && !rowHasScrambling {
		return rowText
	}

	return plainText
}

// hasConcatenationIssues checks if text has patterns indicating missing word boundaries.
// This includes:
// - Very long "words" (like concatenated table headers)
// - Numbers stuck to words (like court transcript line numbers: "12led", "7York")
func (pp *PDFProcessor) hasConcatenationIssues(text string) bool {
	words := strings.Fields(text)
	longWordCount := 0
	numberStuckCount := 0

	for _, word := range words {
		// Strip punctuation for analysis
		cleanWord := strings.Trim(word, ".,;:!?()[]{}\"'")

		// Check for very long words (table concatenation)
		if len(cleanWord) > 40 {
			longWordCount++
		}

		// Check for numbers stuck to words (court transcript line numbers)
		// Pattern: digit(s) followed by lowercase letter, or letter followed by digit(s)
		if len(cleanWord) > 2 {
			// Check for "12led", "13that" pattern (number prefix)
			if pp.hasNumberWordConcat(cleanWord) {
				numberStuckCount++
			}
		}
	}

	// Concatenation issues if:
	// - 3+ very long words (tables), OR
	// - 5+ number-stuck patterns (court transcripts)
	return longWordCount > 3 || numberStuckCount > 5
}

// hasNumberWordConcat checks if a word has numbers concatenated with letters
// like "12led", "7York", "fine2" (court transcript line numbers)
func (pp *PDFProcessor) hasNumberWordConcat(word string) bool {
	// Check for digit-to-letter transition (e.g., "12led")
	for i := 0; i < len(word)-1; i++ {
		curr := word[i]
		next := word[i+1]

		// Digit followed by lowercase letter
		if curr >= '0' && curr <= '9' && next >= 'a' && next <= 'z' {
			return true
		}
		// Letter followed by digit (but not common patterns like "v1", "2nd")
		if (curr >= 'a' && curr <= 'z' || curr >= 'A' && curr <= 'Z') &&
			next >= '0' && next <= '9' {
			// Skip common patterns
			rest := word[i+1:]
			if rest == "1" || rest == "2" || rest == "3" || rest == "s" {
				continue
			}
			return true
		}
	}
	return false
}

// hasScrambledText checks for patterns that suggest text was
// extracted in wrong order (reversed character sequences).
func (pp *PDFProcessor) hasScrambledText(text string) bool {
	// Common reversed patterns that indicate scrambling
	// These are common English words that would appear reversed
	scrambledPatterns := []string{
		":morF",  // "From:"
		"siht",   // "this"
		"taht",   // "that"
		"htiw",   // "with"
		"eht ",   // "the "
		" eht",   // " the"
		"dna ",   // "and "
		" dna",   // " and"
		"rof ",   // "for "
		" rof",   // " for"
		"uoy ",   // "you "
		" uoy",   // " you"
		"saw ",   // "was "
		" saw",   // " was"
		"erew",   // "were"
		"era ",   // "are "
	}

	lower := strings.ToLower(text)
	matchCount := 0
	for _, pattern := range scrambledPatterns {
		if strings.Contains(lower, pattern) {
			matchCount++
		}
	}

	// If 2+ reversed patterns found, likely scrambled
	return matchCount >= 2
}

// extractTextByRow extracts text from a page using Content().Text
// with smart word boundary detection based on character X/Y coordinates.
// This handles tables and multi-column layouts better than GetPlainText().
func (pp *PDFProcessor) extractTextByRow(page pdf.Page) string {
	content := page.Content()
	if len(content.Text) == 0 {
		return ""
	}

	// Group characters by Y coordinate (with tolerance) to form rows
	rows := pp.groupTextByRows(content.Text, 3.0) // 3pt Y tolerance

	var result strings.Builder
	for i, row := range rows {
		if i > 0 {
			result.WriteRune('\n')
		}
		rowText := pp.buildRowText(row)
		result.WriteString(rowText)
	}

	return result.String()
}

// groupTextByRows groups text elements by Y coordinate to form logical rows.
// Returns rows sorted top-to-bottom, with elements within each row sorted left-to-right.
func (pp *PDFProcessor) groupTextByRows(texts []pdf.Text, yTolerance float64) [][]pdf.Text {
	if len(texts) == 0 {
		return nil
	}

	// Build rows by grouping nearby Y coordinates
	type rowBucket struct {
		yMin, yMax float64
		texts      []pdf.Text
	}

	var buckets []rowBucket

	for _, t := range texts {
		// Skip newline markers and empty strings
		if t.S == "\n" || t.S == "" {
			continue
		}

		// Find existing bucket or create new one
		found := false
		for i := range buckets {
			if t.Y >= buckets[i].yMin-yTolerance && t.Y <= buckets[i].yMax+yTolerance {
				buckets[i].texts = append(buckets[i].texts, t)
				if t.Y < buckets[i].yMin {
					buckets[i].yMin = t.Y
				}
				if t.Y > buckets[i].yMax {
					buckets[i].yMax = t.Y
				}
				found = true
				break
			}
		}
		if !found {
			buckets = append(buckets, rowBucket{yMin: t.Y, yMax: t.Y, texts: []pdf.Text{t}})
		}
	}

	// Sort buckets by Y (top to bottom in PDF = higher Y first)
	for i := 0; i < len(buckets)-1; i++ {
		for j := i + 1; j < len(buckets); j++ {
			if buckets[j].yMax > buckets[i].yMax {
				buckets[i], buckets[j] = buckets[j], buckets[i]
			}
		}
	}

	// Sort texts within each bucket by X (left to right)
	rows := make([][]pdf.Text, len(buckets))
	for i, bucket := range buckets {
		texts := bucket.texts
		for a := 0; a < len(texts)-1; a++ {
			for b := a + 1; b < len(texts); b++ {
				if texts[b].X < texts[a].X {
					texts[a], texts[b] = texts[b], texts[a]
				}
			}
		}
		rows[i] = texts
	}

	return rows
}

// buildRowText builds a string from a row of text elements,
// inserting spaces based on X coordinate gaps.
func (pp *PDFProcessor) buildRowText(texts []pdf.Text) string {
	if len(texts) == 0 {
		return ""
	}

	var result strings.Builder
	const defaultSpaceThreshold = 3.0 // points

	for i, t := range texts {
		if i > 0 {
			prev := texts[i-1]
			prevEnd := prev.X + prev.W
			gap := t.X - prevEnd

			// Use font-based threshold if available, otherwise default
			threshold := defaultSpaceThreshold
			if prev.FontSize > 0 {
				// Space is typically 0.2-0.3 of font size
				threshold = prev.FontSize * 0.25
			}

			// Insert space if gap exceeds threshold
			if gap > threshold {
				result.WriteRune(' ')
			}
		}
		result.WriteString(t.S)
	}

	return result.String()
}

// stripGarbledHeaders removes ROT3-encoded or otherwise garbled headers
// that appear in some PDFs (e.g., court reporter watermarks).
// These appear as random-looking letter sequences with spaces between characters.
func stripGarbledHeaders(text string) string {
	// First, try to strip garbled prefix from beginning of text
	text = stripGarbledPrefix(text)

	// Then filter out entirely garbled lines
	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		if isGarbledLine(line) {
			continue // skip this line
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// stripGarbledPrefix removes garbled text from the start of content
// Pattern: sequences of single chars with spaces like "$ J U H Q % O D Q G R"
func stripGarbledPrefix(text string) string {
	// Look for pattern of single-char words at start
	// e.g., "$ J U H Q % O D Q G R ... actual content"
	words := strings.Fields(text)
	if len(words) < 10 {
		return text
	}

	// Count consecutive single-character words at start
	singleCharRun := 0
	for _, word := range words {
		if len(word) == 1 || (len(word) <= 3 && containsOnlySpecialChars(word)) {
			singleCharRun++
		} else {
			break
		}
	}

	// If we found a significant run of single chars (>10), strip them
	if singleCharRun >= 10 {
		// Find the position after the garbled prefix
		// We'll skip past the single-char words
		remaining := words[singleCharRun:]
		return strings.Join(remaining, " ")
	}

	return text
}

// containsOnlySpecialChars checks if string has only special/symbol chars
func containsOnlySpecialChars(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			if len(s) > 1 {
				return false
			}
		}
	}
	return true
}

// isGarbledLine detects lines that appear to be garbled/encoded text
func isGarbledLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 10 {
		return false
	}

	// Count single-character "words" (like "J U H Q")
	words := strings.Fields(trimmed)
	if len(words) < 5 {
		return false
	}

	singleCharCount := 0
	replacementCount := 0

	for _, word := range words {
		if len(word) == 1 {
			singleCharCount++
		}
		for _, r := range word {
			if r == '\uFFFD' || r == '�' {
				replacementCount++
			}
		}
	}

	// If >60% of words are single characters, likely garbled
	singleCharRatio := float64(singleCharCount) / float64(len(words))
	if singleCharRatio > 0.6 {
		return true
	}

	// If many replacement characters, likely garbled
	if replacementCount > 3 {
		return true
	}

	return false
}

// cleanPDFText sanitizes text extracted from PDFs by:
// - Replacing Unicode replacement characters (U+FFFD) with spaces (preserve word boundaries)
// - Replacing Private Use Area characters with spaces
// - Removing zero-width and formatting characters
// - Replacing block/geometric shapes (redactions like ■, □) with spaces
// - Collapsing excessive whitespace
func cleanPDFText(text string) string {
	var result strings.Builder
	result.Grow(len(text))

	prevSpace := false
	for _, r := range text {
		switch {
		// Replace Unicode replacement character with space (preserves word boundaries)
		case r == '\uFFFD':
			if !prevSpace {
				result.WriteRune(' ')
				prevSpace = true
			}
		// Replace Private Use Area characters with space (often corrupted spaces/ligatures)
		case r >= '\uE000' && r <= '\uF8FF':
			if !prevSpace {
				result.WriteRune(' ')
				prevSpace = true
			}
		// Remove zero-width and formatting characters entirely (no visual impact)
		case r == '\u200B' || // Zero-width space
			r == '\u200C' || // Zero-width non-joiner
			r == '\u200D' || // Zero-width joiner
			r == '\uFEFF' || // Byte order mark / zero-width no-break space
			r == '\u00AD' || // Soft hyphen
			r == '\u2060' || // Word joiner
			r == '\u180E': // Mongolian vowel separator
			continue
		// Replace block/geometric shapes with space (redactions often replace words)
		case r >= '\u2580' && r <= '\u259F': // Block elements
			if !prevSpace {
				result.WriteRune(' ')
				prevSpace = true
			}
		case r >= '\u25A0' && r <= '\u25FF': // Geometric shapes (includes ■ □ ▪ ▫ etc.)
			if !prevSpace {
				result.WriteRune(' ')
				prevSpace = true
			}
		// Skip control characters except newline and tab
		case r < ' ' && r != '\n' && r != '\t':
			continue
		// Normalize whitespace (collapse multiple spaces)
		case r == ' ' || r == '\t':
			if !prevSpace {
				result.WriteRune(' ')
				prevSpace = true
			}
		case r == '\n':
			result.WriteRune('\n')
			prevSpace = true
		default:
			result.WriteRune(r)
			prevSpace = false
		}
	}

	return strings.TrimSpace(result.String())
}
