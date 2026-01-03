package docsaf

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

// TextRepair provides utilities for detecting and fixing common PDF text extraction issues.
type TextRepair struct {
	// Configuration
	HeaderFooterMargin float64 // Percentage of page height to consider as header/footer region (0.1 = 10%)
	MinPagesSeen       int     // Minimum pages to analyze before detecting headers/footers

	// State for page-association detection
	pageFirstLines []string
	pageLastLines  []string
	pageCount      int
}

// NewTextRepair creates a new TextRepair with sensible defaults.
func NewTextRepair() *TextRepair {
	return &TextRepair{
		HeaderFooterMargin: 0.1, // 10% of page height
		MinPagesSeen:       3,   // Need at least 3 pages to detect patterns
		pageFirstLines:     make([]string, 0),
		pageLastLines:      make([]string, 0),
	}
}

// -------------------------------------------------------------------------
// Encoding Detection and Repair (ROT-N, Symbol Substitution)
// -------------------------------------------------------------------------

// EnglishLetterFrequency contains standard English letter frequencies.
var EnglishLetterFrequency = map[rune]float64{
	'e': 0.127, 't': 0.091, 'a': 0.082, 'o': 0.075, 'i': 0.070,
	'n': 0.067, 's': 0.063, 'h': 0.061, 'r': 0.060, 'd': 0.043,
	'l': 0.040, 'c': 0.028, 'u': 0.028, 'm': 0.024, 'w': 0.024,
	'f': 0.022, 'g': 0.020, 'y': 0.020, 'p': 0.019, 'b': 0.015,
	'v': 0.010, 'k': 0.008, 'j': 0.002, 'x': 0.002, 'q': 0.001,
	'z': 0.001,
}

// CommonSymbolSubstitutions maps common symbol substitutions in encoded PDFs.
// Key: encoded symbol, Value: decoded character
var CommonSymbolSubstitutions = map[rune]rune{
	'$': 'A', '%': 'B', '&': 'C', '\'': 'D', '(': 'E', ')': 'F',
	'*': 'G', '+': 'H', ',': 'I', '-': 'J', '.': 'K', '/': 'L',
	'0': 'M', '1': 'N', '2': 'O', '3': 'P', '4': 'Q', '5': 'R',
	'6': 'S', '7': 'T', '8': 'U', '9': 'V', ':': 'W', ';': 'X',
	'<': 'Y', '=': 'Z',
}

// DetectEncodingShift analyzes text to detect if it uses a shifted alphabet encoding.
// Returns the detected shift (0-25) and a confidence score (0.0-1.0).
// A shift of 0 means no encoding detected or text is normal.
func (tr *TextRepair) DetectEncodingShift(text string) (shift int, confidence float64) {
	// Skip if text is too short
	if len(text) < 50 {
		return 0, 0.0
	}

	// Calculate letter frequency of the text
	letterCount := make(map[rune]int)
	totalLetters := 0

	for _, r := range strings.ToLower(text) {
		if r >= 'a' && r <= 'z' {
			letterCount[r]++
			totalLetters++
		}
	}

	if totalLetters < 30 {
		return 0, 0.0 // Not enough letters to analyze
	}

	// Convert to frequency distribution
	textFreq := make(map[rune]float64)
	for r, count := range letterCount {
		textFreq[r] = float64(count) / float64(totalLetters)
	}

	// Test each possible shift (0-25)
	bestShift := 0
	bestScore := 0.0

	for testShift := 0; testShift < 26; testShift++ {
		// Apply shift to text frequency and compare with English
		score := tr.calculateFrequencyMatchScore(textFreq, testShift)
		if score > bestScore {
			bestScore = score
			bestShift = testShift
		}
	}

	// Calculate confidence based on how much better the best shift is vs shift=0
	noShiftScore := tr.calculateFrequencyMatchScore(textFreq, 0)

	// If best shift is 0, no encoding detected
	if bestShift == 0 {
		return 0, bestScore
	}

	// Confidence is based on improvement over no-shift
	confidence = (bestScore - noShiftScore) / (1.0 - noShiftScore + 0.001)
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0 {
		confidence = 0
	}

	return bestShift, confidence
}

// calculateFrequencyMatchScore calculates how well the text frequency matches English
// when shifted by the given amount.
func (tr *TextRepair) calculateFrequencyMatchScore(textFreq map[rune]float64, shift int) float64 {
	// Calculate dot product of shifted text frequency with English frequency
	dotProduct := 0.0
	textMag := 0.0
	engMag := 0.0

	for r := 'a'; r <= 'z'; r++ {
		// Shift the character back
		shiftedR := 'a' + (r-'a'+rune(26-shift))%26

		textF := textFreq[r]
		engF := EnglishLetterFrequency[shiftedR]

		dotProduct += textF * engF
		textMag += textF * textF
		engMag += engF * engF
	}

	// Cosine similarity
	if textMag == 0 || engMag == 0 {
		return 0.0
	}
	return dotProduct / (math.Sqrt(textMag) * math.Sqrt(engMag))
}

// DecodeShiftedText decodes text that has been shifted by the specified amount.
func (tr *TextRepair) DecodeShiftedText(text string, shift int) string {
	if shift == 0 {
		return text
	}

	var result strings.Builder
	result.Grow(len(text))

	for _, r := range text {
		if r >= 'A' && r <= 'Z' {
			// Shift uppercase back
			shifted := 'A' + (r-'A'+rune(26-shift))%26
			result.WriteRune(shifted)
		} else if r >= 'a' && r <= 'z' {
			// Shift lowercase back
			shifted := 'a' + (r-'a'+rune(26-shift))%26
			result.WriteRune(shifted)
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// DetectSymbolSubstitution checks if text uses symbol-to-letter substitution.
// Returns the substitution map and confidence score.
func (tr *TextRepair) DetectSymbolSubstitution(text string) (map[rune]rune, float64) {
	// Count symbols that could be substituted letters
	symbolCount := 0
	letterCount := 0

	for _, r := range text {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			letterCount++
		} else if _, isSubst := CommonSymbolSubstitutions[r]; isSubst {
			symbolCount++
		}
	}

	// If high symbol ratio compared to letters, likely substitution
	total := symbolCount + letterCount
	if total < 20 {
		return nil, 0.0
	}

	symbolRatio := float64(symbolCount) / float64(total)

	// If >30% symbols that could be letters, likely encoded
	if symbolRatio > 0.3 {
		return CommonSymbolSubstitutions, symbolRatio
	}

	return nil, 0.0
}

// DecodeSymbolSubstitution applies symbol substitution decoding to text.
func (tr *TextRepair) DecodeSymbolSubstitution(text string, substMap map[rune]rune) string {
	if substMap == nil {
		return text
	}

	var result strings.Builder
	result.Grow(len(text))

	for _, r := range text {
		if sub, ok := substMap[r]; ok {
			result.WriteRune(sub)
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// AutoDecodeText automatically detects and decodes text with encoding issues.
// Returns the decoded text and a description of what was fixed.
func (tr *TextRepair) AutoDecodeText(text string) (decoded string, fixed string) {
	// Try symbol substitution first
	substMap, substConf := tr.DetectSymbolSubstitution(text)
	if substConf > 0.3 {
		decoded = tr.DecodeSymbolSubstitution(text, substMap)
		// Now check if the result needs shift decoding
		shift, shiftConf := tr.DetectEncodingShift(decoded)
		if shiftConf > 0.5 && shift != 0 {
			decoded = tr.DecodeShiftedText(decoded, shift)
			return decoded, "symbol_substitution+shift"
		}
		return decoded, "symbol_substitution"
	}

	// Try shift encoding
	shift, shiftConf := tr.DetectEncodingShift(text)
	if shiftConf > 0.5 && shift != 0 {
		decoded = tr.DecodeShiftedText(text, shift)
		return decoded, "shift"
	}

	return text, ""
}

// -------------------------------------------------------------------------
// Mirrored/Reversed Text Detection and Repair (Bigram Analysis)
// -------------------------------------------------------------------------

// EnglishBigramFrequency contains the top English bigram frequencies.
// These are used to detect reversed text by comparing bigram distributions.
var EnglishBigramFrequency = map[string]float64{
	"th": 0.0356, "he": 0.0307, "in": 0.0243, "er": 0.0205, "an": 0.0199,
	"re": 0.0185, "on": 0.0176, "at": 0.0149, "en": 0.0145, "nd": 0.0135,
	"ti": 0.0134, "es": 0.0134, "or": 0.0128, "te": 0.0120, "of": 0.0117,
	"ed": 0.0117, "is": 0.0113, "it": 0.0112, "al": 0.0109, "ar": 0.0107,
	"st": 0.0105, "to": 0.0104, "nt": 0.0104, "ng": 0.0095, "se": 0.0093,
	"ha": 0.0093, "as": 0.0087, "ou": 0.0087, "io": 0.0083, "le": 0.0083,
	"ve": 0.0083, "co": 0.0079, "me": 0.0079, "de": 0.0076, "hi": 0.0076,
	"ri": 0.0073, "ro": 0.0073, "ic": 0.0070, "ne": 0.0069, "ea": 0.0069,
	"ra": 0.0069, "ce": 0.0065, "li": 0.0062, "ch": 0.0060, "ll": 0.0058,
	"be": 0.0058, "ma": 0.0057, "si": 0.0055, "om": 0.0055, "ur": 0.0054,
}

// CommonReversedWords contains common English words and their reversed forms.
// Used as a secondary check for mirrored text.
var CommonReversedWords = map[string]string{
	"eht": "the", "dna": "and", "rof": "for", "era": "are", "tub": "but",
	"ton": "not", "uoy": "you", "lla": "all", "nac": "can", "dah": "had",
	"reh": "her", "saw": "was", "eno": "one", "ruo": "our", "tuo": "out",
	"yad": "day", "teg": "get", "sah": "has", "mih": "him", "sih": "his",
	"woh": "how", "nam": "man", "wen": "new", "won": "now", "dlo": "old",
	"ees": "see", "emit": "time", "yrev": "very", "nehw": "when", "ohw": "who",
	"yob": "boy", "did": "did", "sti": "its", "tel": "let", "tup": "put",
	"yas": "say", "ehs": "she", "owt": "two", "yaw": "way", "taht": "that",
	"siht": "this", "htiw": "with", "evah": "have", "morf": "from", "yeht": "they",
	"neeb": "been", "evol": "love", "ekam": "make", "erom": "more", "ylno": "only",
	"revo": "over", "hcus": "such", "ekat": "take", "naht": "than", "meht": "them",
	"neht": "then", "eseht": "these", "gniht": "thing", "kniht": "think",
	"erehw": "where", "hcihw": "which", "dlrow": "world", "dluow": "would",
	"tuoba": "about", "retfa": "after", "niaga": "again", "tsniaga": "against",
}

// DetectMirroredText analyzes text to detect if it appears to be reversed/mirrored.
// Returns a confidence score (0.0-1.0) where higher values indicate more likely mirroring.
func (tr *TextRepair) DetectMirroredText(text string) float64 {
	text = strings.ToLower(text)

	// Skip if text is too short
	if len(text) < 30 {
		return 0.0
	}

	// Method 1: Check for reversed common words
	wordScore := tr.detectReversedWords(text)

	// Method 2: Bigram frequency analysis
	bigramScore := tr.detectReversedBigrams(text)

	// Combine scores - if either is high, text is likely mirrored
	// Weight bigram analysis more heavily as it's more robust
	combinedScore := wordScore*0.4 + bigramScore*0.6

	return combinedScore
}

// detectReversedWords checks for known reversed English words in the text.
func (tr *TextRepair) detectReversedWords(text string) float64 {
	words := strings.Fields(text)
	if len(words) < 5 {
		return 0.0
	}

	reversedCount := 0
	checkedCount := 0

	for _, word := range words {
		// Clean the word
		word = strings.Trim(word, ".,!?;:\"'()[]{}$%")
		if len(word) < 3 || len(word) > 10 {
			continue
		}

		checkedCount++
		if _, isReversed := CommonReversedWords[word]; isReversed {
			reversedCount++
		}
	}

	if checkedCount < 5 {
		return 0.0
	}

	// Return ratio of reversed words found
	return float64(reversedCount) / float64(checkedCount) * 3.0 // Scale up since we won't find all
}

// detectReversedBigrams uses bigram frequency analysis to detect mirrored text.
// Compares bigram frequencies of original text vs reversed text against English baseline.
func (tr *TextRepair) detectReversedBigrams(text string) float64 {
	// Extract bigrams from original text
	originalBigrams := tr.extractBigrams(text)
	if len(originalBigrams) < 10 {
		return 0.0
	}

	// Calculate score for original text
	originalScore := tr.calculateBigramScore(originalBigrams)

	// Calculate score for reversed text
	reversedText := tr.reverseString(text)
	reversedBigrams := tr.extractBigrams(reversedText)
	reversedScore := tr.calculateBigramScore(reversedBigrams)

	// If reversed text scores significantly higher, original is mirrored
	if reversedScore > originalScore*1.2 {
		// Confidence based on how much better reversed is
		confidence := (reversedScore - originalScore) / (reversedScore + 0.001)
		if confidence > 1.0 {
			confidence = 1.0
		}
		return confidence
	}

	return 0.0
}

// extractBigrams extracts all letter bigrams from text.
func (tr *TextRepair) extractBigrams(text string) map[string]int {
	bigrams := make(map[string]int)
	text = strings.ToLower(text)

	prevLetter := rune(0)
	for _, r := range text {
		if r >= 'a' && r <= 'z' {
			if prevLetter != 0 {
				bigram := string([]rune{prevLetter, r})
				bigrams[bigram]++
			}
			prevLetter = r
		} else {
			prevLetter = 0 // Reset on non-letter
		}
	}

	return bigrams
}

// calculateBigramScore calculates how well a bigram distribution matches English.
func (tr *TextRepair) calculateBigramScore(bigrams map[string]int) float64 {
	if len(bigrams) == 0 {
		return 0.0
	}

	// Count total bigrams
	total := 0
	for _, count := range bigrams {
		total += count
	}

	// Calculate weighted sum based on English bigram frequencies
	score := 0.0
	for bigram, count := range bigrams {
		if englishFreq, ok := EnglishBigramFrequency[bigram]; ok {
			// Weight by how common this bigram is in English
			score += englishFreq * float64(count) / float64(total)
		}
	}

	return score
}

// reverseString reverses a string, preserving Unicode characters.
func (tr *TextRepair) reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// RepairMirroredText reverses text that has been detected as mirrored.
// It can operate at word level or full text level.
func (tr *TextRepair) RepairMirroredText(text string) string {
	// First, check if the whole text should be reversed
	confidence := tr.DetectMirroredText(text)
	if confidence < 0.3 {
		return text // Not mirrored
	}

	// Try word-by-word reversal first (handles some PDF extraction patterns)
	wordReversed := tr.reverseWords(text)
	wordReversedScore := tr.DetectMirroredText(wordReversed)

	// Try full text reversal
	fullReversed := tr.reverseString(text)
	fullReversedScore := tr.DetectMirroredText(fullReversed)

	// Return the version that scores lowest (least mirrored)
	if wordReversedScore < fullReversedScore && wordReversedScore < confidence {
		return wordReversed
	}
	if fullReversedScore < confidence {
		return fullReversed
	}

	return text // Return original if neither reversal helps
}

// reverseWords reverses each word in the text individually.
func (tr *TextRepair) reverseWords(text string) string {
	words := strings.Fields(text)
	for i, word := range words {
		words[i] = tr.reverseString(word)
	}
	return strings.Join(words, " ")
}

// AutoRepairMirroredText detects and repairs mirrored text if confidence is high enough.
// Returns the repaired text and whether repair was applied.
func (tr *TextRepair) AutoRepairMirroredText(text string) (string, bool) {
	confidence := tr.DetectMirroredText(text)
	if confidence >= 0.4 {
		repaired := tr.RepairMirroredText(text)
		// Verify repair actually improved things
		newConfidence := tr.DetectMirroredText(repaired)
		if newConfidence < confidence*0.5 {
			return repaired, true
		}
	}
	return text, false
}

// -------------------------------------------------------------------------
// Header/Footer Detection via Page-Association
// -------------------------------------------------------------------------

// RecordPageContent records the first and last lines of a page for pattern detection.
func (tr *TextRepair) RecordPageContent(pageText string) {
	lines := strings.Split(strings.TrimSpace(pageText), "\n")
	if len(lines) == 0 {
		return
	}

	tr.pageCount++

	// Record first line (header candidate)
	firstLine := strings.TrimSpace(lines[0])
	if len(firstLine) > 0 {
		tr.pageFirstLines = append(tr.pageFirstLines, firstLine)
	}

	// Record last line (footer candidate)
	lastLine := strings.TrimSpace(lines[len(lines)-1])
	if len(lastLine) > 0 {
		tr.pageLastLines = append(tr.pageLastLines, lastLine)
	}
}

// GetDetectedHeaders returns headers detected across multiple pages.
// A line is considered a header if it appears (with edit distance tolerance) on most pages.
func (tr *TextRepair) GetDetectedHeaders() []string {
	return tr.findRepeatingPatterns(tr.pageFirstLines, 0.6)
}

// GetDetectedFooters returns footers detected across multiple pages.
func (tr *TextRepair) GetDetectedFooters() []string {
	return tr.findRepeatingPatterns(tr.pageLastLines, 0.6)
}

// findRepeatingPatterns finds lines that appear on at least threshold fraction of pages.
func (tr *TextRepair) findRepeatingPatterns(lines []string, threshold float64) []string {
	if len(lines) < tr.MinPagesSeen {
		return nil
	}

	// Group similar lines using edit distance
	type lineGroup struct {
		representative string
		count          int
	}

	var groups []lineGroup

	for _, line := range lines {
		found := false
		for i := range groups {
			if tr.isSimilar(line, groups[i].representative) {
				groups[i].count++
				found = true
				break
			}
		}
		if !found {
			groups = append(groups, lineGroup{representative: line, count: 1})
		}
	}

	// Find groups that appear on enough pages
	minCount := int(float64(len(lines)) * threshold)
	var patterns []string

	for _, g := range groups {
		if g.count >= minCount {
			patterns = append(patterns, g.representative)
		}
	}

	return patterns
}

// isSimilar checks if two lines are similar using Levenshtein-like distance.
// Returns true if the normalized edit distance is < 0.3
func (tr *TextRepair) isSimilar(a, b string) bool {
	// Normalize: lowercase, remove page numbers
	aNorm := tr.normalizeForComparison(a)
	bNorm := tr.normalizeForComparison(b)

	if aNorm == bNorm {
		return true
	}

	// Calculate simple edit distance for short strings
	maxLen := len(aNorm)
	if len(bNorm) > maxLen {
		maxLen = len(bNorm)
	}
	if maxLen == 0 {
		return true
	}

	dist := tr.levenshteinDistance(aNorm, bNorm)
	normalizedDist := float64(dist) / float64(maxLen)

	return normalizedDist < 0.3
}

// normalizeForComparison normalizes text for header/footer comparison.
// Removes page numbers, dates, and other variable content.
func (tr *TextRepair) normalizeForComparison(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))

	// Remove page numbers like "Page 1", "1", "- 1 -", etc.
	pageNumPattern := regexp.MustCompile(`(?i)(page\s*)?\d+(\s*of\s*\d+)?|^\s*-?\s*\d+\s*-?\s*$`)
	s = pageNumPattern.ReplaceAllString(s, "")

	// Remove common date patterns
	datePattern := regexp.MustCompile(`\d{1,2}[/-]\d{1,2}[/-]\d{2,4}`)
	s = datePattern.ReplaceAllString(s, "")

	return strings.TrimSpace(s)
}

// levenshteinDistance calculates the Levenshtein edit distance between two strings.
func (tr *TextRepair) levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Limit comparison length for performance
	if len(a) > 100 {
		a = a[:100]
	}
	if len(b) > 100 {
		b = b[:100]
	}

	aRunes := []rune(a)
	bRunes := []rune(b)

	// Use two-row optimization
	prev := make([]int, len(bRunes)+1)
	curr := make([]int, len(bRunes)+1)

	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(aRunes); i++ {
		curr[0] = i
		for j := 1; j <= len(bRunes); j++ {
			cost := 1
			if aRunes[i-1] == bRunes[j-1] {
				cost = 0
			}
			curr[j] = min(min(prev[j]+1, curr[j-1]+1), prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}

	return prev[len(bRunes)]
}

// RemoveHeadersFooters removes detected headers and footers from page text.
func (tr *TextRepair) RemoveHeadersFooters(pageText string, headers, footers []string) string {
	lines := strings.Split(pageText, "\n")
	if len(lines) == 0 {
		return pageText
	}

	var result []string

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			result = append(result, line)
			continue
		}

		// Check if this line matches a header (first few lines)
		if i < 3 {
			isHeader := false
			for _, h := range headers {
				if tr.isSimilar(trimmed, h) {
					isHeader = true
					break
				}
			}
			if isHeader {
				continue // Skip this line
			}
		}

		// Check if this line matches a footer (last few lines)
		if i >= len(lines)-3 {
			isFooter := false
			for _, f := range footers {
				if tr.isSimilar(trimmed, f) {
					isFooter = true
					break
				}
			}
			if isFooter {
				continue // Skip this line
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// -------------------------------------------------------------------------
// Fragmented Text Detection and Repair
// -------------------------------------------------------------------------

// DetectFragmentedText checks if text has abnormal single-character word sequences.
// Returns true if the text appears to be fragmented.
func (tr *TextRepair) DetectFragmentedText(text string) bool {
	words := strings.Fields(text)
	if len(words) < 5 {
		return false
	}

	// Count sequences of single-char words
	totalSingleCharRuns := 0
	currentRun := 0

	for _, word := range words {
		if len(word) == 1 && unicode.IsLetter(rune(word[0])) {
			currentRun++
		} else {
			// Any run of 3+ single chars indicates fragmentation
			if currentRun >= 3 {
				totalSingleCharRuns++
			}
			currentRun = 0
		}
	}
	if currentRun >= 3 {
		totalSingleCharRuns++
	}

	// If we have any run of 3+ single-char words, likely fragmented
	return totalSingleCharRuns >= 1
}

// RepairFragmentedText attempts to merge fragmented word sequences.
// Handles patterns like:
//   - "L A N G I N O" → "LANGINO" (consecutive single chars)
//   - "w orry" → "worry" (single char + fragment)
//   - "t a k en" → "taken" (mixed single chars + fragment)
func (tr *TextRepair) RepairFragmentedText(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var result []string
	var currentMerge strings.Builder
	inFragmentedSequence := false

	for i, word := range words {
		isSingleChar := len(word) == 1 && unicode.IsLetter(rune(word[0]))

		// Check context for fragmentation
		prevSingle := i > 0 && len(words[i-1]) == 1 && unicode.IsLetter(rune(words[i-1][0]))
		nextSingle := i < len(words)-1 && len(words[i+1]) == 1 && unicode.IsLetter(rune(words[i+1][0]))

		if isSingleChar && (prevSingle || nextSingle) {
			// Single char in a sequence - merge it
			currentMerge.WriteString(word)
			inFragmentedSequence = true
			continue
		}

		if inFragmentedSequence && !isSingleChar {
			// We were in a fragmented sequence and hit a non-single char
			// Check if this fragment should be appended (no space before it in original)
			// Heuristic: if merged text + fragment is <= 20 chars and all letters, merge
			potentialWord := currentMerge.String() + word
			if len(potentialWord) <= 20 && isAllLetters(potentialWord) {
				currentMerge.WriteString(word)
				result = append(result, currentMerge.String())
				currentMerge.Reset()
				inFragmentedSequence = false
				continue
			}
		}

		// Flush any merged text
		if currentMerge.Len() > 0 {
			result = append(result, currentMerge.String())
			currentMerge.Reset()
		}
		inFragmentedSequence = false

		result = append(result, word)
	}

	// Flush final merge
	if currentMerge.Len() > 0 {
		result = append(result, currentMerge.String())
	}

	return strings.Join(result, " ")
}

// isAllLetters returns true if all runes in s are letters.
func isAllLetters(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// -------------------------------------------------------------------------
// Deposition Transcript Detection
// -------------------------------------------------------------------------

// DepositionLayoutConfig provides optimized settings for deposition transcripts.
var DepositionLayoutConfig = LayoutConfig{
	ColumnGapThreshold:  12.0, // Narrow gap for line number column
	RowTolerance:        2.0,  // Tight row grouping
	MinRowsForColumnPct: 75,   // Line numbers appear on 75%+ of rows
	FilterLineNumbers:   true, // Don't include line numbers in output
}

// LayoutConfig allows customization of layout analysis parameters.
type LayoutConfig struct {
	ColumnGapThreshold  float64 // Minimum gap for column detection
	RowTolerance        float64 // Y tolerance for row grouping
	MinRowsForColumnPct int     // % of rows that must have gap for column detection
	FilterLineNumbers   bool    // Whether to filter out line number columns
}

// DefaultLayoutConfig returns standard layout configuration.
func DefaultLayoutConfig() LayoutConfig {
	return LayoutConfig{
		ColumnGapThreshold:  30.0,
		RowTolerance:        3.0,
		MinRowsForColumnPct: 25,
		FilterLineNumbers:   false,
	}
}

// DetectDepositionLayout checks if a page appears to be a deposition transcript.
// Deposition transcripts have:
// - Line numbers 1-25 (or similar) in a narrow left column
// - Consistent line spacing
// - Q: and A: question/answer format
func (tr *TextRepair) DetectDepositionLayout(texts []pdf.Text) bool {
	if len(texts) < 50 {
		return false
	}

	// Find the leftmost X position (potential line number column)
	minX := texts[0].X
	for _, t := range texts {
		if t.X < minX {
			minX = t.X
		}
	}

	// Check for line numbers (1-25) in left column
	lineNumbersFound := 0
	lineNumberPattern := regexp.MustCompile(`^[1-9]$|^1[0-9]$|^2[0-5]$`)

	for _, t := range texts {
		// Only check leftmost column (within 30pt of min X)
		if t.X-minX < 30 {
			if lineNumberPattern.MatchString(strings.TrimSpace(t.S)) {
				lineNumbersFound++
			}
		}
	}

	// If we found 20+ line numbers, likely a deposition
	if lineNumbersFound >= 20 {
		return true
	}

	// Also check for Q: and A: patterns anywhere
	qaCount := 0
	for _, t := range texts {
		s := strings.TrimSpace(t.S)
		if s == "Q" || s == "A" || strings.HasPrefix(s, "Q:") || strings.HasPrefix(s, "A:") ||
			strings.HasPrefix(s, "Q.") || strings.HasPrefix(s, "A.") {
			qaCount++
		}
	}

	// If we found Q/A patterns with some line numbers, likely deposition
	return qaCount >= 5 && lineNumbersFound >= 10
}

// FilterLineNumberColumn removes the leftmost column if it contains only line numbers.
func (tr *TextRepair) FilterLineNumberColumn(texts []pdf.Text) []pdf.Text {
	if len(texts) == 0 {
		return texts
	}

	// Find min X
	minX := texts[0].X
	for _, t := range texts {
		if t.X < minX {
			minX = t.X
		}
	}

	// Determine column boundary (find gap after line numbers)
	// Sort by X to find natural gap
	type xCount struct {
		x     float64
		count int
	}

	xCounts := make(map[int]int)
	for _, t := range texts {
		bucket := int(t.X / 5) // 5pt buckets
		xCounts[bucket]++
	}

	// Find the first significant gap (line number column is usually 20-40pt wide)
	var buckets []int
	for b := range xCounts {
		buckets = append(buckets, b)
	}
	sort.Ints(buckets)

	lineNumColumnEnd := minX + 40 // Default assumption

	for i := 0; i < len(buckets)-1; i++ {
		gap := (buckets[i+1] - buckets[i]) * 5
		if gap >= 10 && float64(buckets[i])*5+minX < 60 {
			// Found a gap near the left margin
			lineNumColumnEnd = float64(buckets[i]+1) * 5
			break
		}
	}

	// Filter out texts in the line number column
	var filtered []pdf.Text
	for _, t := range texts {
		if t.X > lineNumColumnEnd || t.X-minX > 40 {
			filtered = append(filtered, t)
		}
	}

	return filtered
}
