package docsaf

import (
	"testing"
)

func TestTextRepair_DetectEncodingShift(t *testing.T) {
	tr := NewTextRepair()

	tests := []struct {
		name           string
		text           string
		wantShift      int
		wantConfidence float64 // minimum confidence
	}{
		{
			name:           "normal english text",
			text:           "The quick brown fox jumps over the lazy dog. This is a sample text with normal English.",
			wantShift:      0,
			wantConfidence: 0.0,
		},
		{
			name:           "ROT3 encoded text",
			text:           "Wkh txlfn eurzq ira mxpsv ryhu wkh odcb grj. Wklv lv d vdpsoh whaw.",
			wantShift:      3,
			wantConfidence: 0.3,
		},
		{
			// ROT13 with longer text for better detection
			name:           "ROT13 encoded text",
			text:           "Gur dhvpx oebja sbk whzcf bire gur ynml qbt. Guvf vf n fnzcyr grkg jvgu ybgf bs jbeqf sbe orggre qrgrpgvba.",
			wantShift:      13,
			wantConfidence: 0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shift, confidence := tr.DetectEncodingShift(tt.text)
			if shift != tt.wantShift {
				t.Errorf("DetectEncodingShift() shift = %d, want %d", shift, tt.wantShift)
			}
			if tt.wantConfidence > 0 && confidence < tt.wantConfidence {
				t.Errorf("DetectEncodingShift() confidence = %f, want >= %f", confidence, tt.wantConfidence)
			}
		})
	}
}

func TestTextRepair_DecodeShiftedText(t *testing.T) {
	tr := NewTextRepair()

	tests := []struct {
		name  string
		text  string
		shift int
		want  string
	}{
		{
			name:  "ROT3 decode",
			text:  "Wkh txlfn eurzq ira",
			shift: 3,
			want:  "The quick brown fox",
		},
		{
			name:  "ROT13 decode",
			text:  "Gur dhvpx oebja sbk",
			shift: 13,
			want:  "The quick brown fox",
		},
		{
			name:  "no shift",
			text:  "Hello World",
			shift: 0,
			want:  "Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tr.DecodeShiftedText(tt.text, tt.shift)
			if got != tt.want {
				t.Errorf("DecodeShiftedText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTextRepair_DetectSymbolSubstitution(t *testing.T) {
	tr := NewTextRepair()

	tests := []struct {
		name           string
		text           string
		wantDetected   bool
		wantConfidence float64
	}{
		{
			name:         "normal text",
			text:         "Hello World this is normal text with proper words and sentences",
			wantDetected: false,
		},
		{
			// Symbol substitution with enough symbols to trigger detection
			// This simulates text where A-Z are replaced with symbols
			name:         "symbol encoded text",
			text:         "$ % & ' ( ) * + , - . / 0 1 2 3 4 5 $ % & ' ( ) * + , - . / 0 1 2 3 4 5 6 7 8",
			wantDetected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			substMap, confidence := tr.DetectSymbolSubstitution(tt.text)
			detected := substMap != nil && confidence > 0.3
			if detected != tt.wantDetected {
				t.Errorf("DetectSymbolSubstitution() detected = %v, want %v (confidence=%f)",
					detected, tt.wantDetected, confidence)
			}
		})
	}
}

func TestTextRepair_HeaderFooterDetection(t *testing.T) {
	tr := NewTextRepair()

	// Simulate multiple pages with same header/footer
	pages := []string{
		"COURT REPORTING INC\nThis is page 1 content.\nPage 1",
		"COURT REPORTING INC\nThis is page 2 content.\nPage 2",
		"COURT REPORTING INC\nThis is page 3 content.\nPage 3",
		"COURT REPORTING INC\nThis is page 4 content.\nPage 4",
	}

	for _, page := range pages {
		tr.RecordPageContent(page)
	}

	headers := tr.GetDetectedHeaders()
	footers := tr.GetDetectedFooters()

	// Should detect "COURT REPORTING INC" as header
	if len(headers) == 0 {
		t.Error("Expected to detect at least one header pattern")
	} else {
		found := false
		for _, h := range headers {
			if h == "COURT REPORTING INC" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected header 'COURT REPORTING INC', got %v", headers)
		}
	}

	// Should detect page number pattern as footer (but normalized away)
	// The actual footer detection may vary based on normalization
	t.Logf("Detected footers: %v", footers)
}

func TestTextRepair_RemoveHeadersFooters(t *testing.T) {
	tr := NewTextRepair()

	headers := []string{"COURT REPORTING INC"}
	footers := []string{"Page 1"}

	text := `COURT REPORTING INC
This is the main content.
More content here.
Page 1`

	result := tr.RemoveHeadersFooters(text, headers, footers)

	// Should not contain header
	if containsLine(result, "COURT REPORTING INC") {
		t.Error("Header should have been removed")
	}

	// Should contain main content
	if !containsLine(result, "This is the main content.") {
		t.Error("Main content should be preserved")
	}
}

func containsLine(text, line string) bool {
	for _, l := range splitLines(text) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(text string) []string {
	var lines []string
	start := 0
	for i, c := range text {
		if c == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}

func TestTextRepair_DetectFragmentedText(t *testing.T) {
	tr := NewTextRepair()

	tests := []struct {
		name           string
		text           string
		wantFragmented bool
	}{
		{
			name:           "normal text",
			text:           "This is normal text with proper words and complete sentences that are not fragmented.",
			wantFragmented: false,
		},
		{
			// Two runs of 5+ single chars to trigger detection
			name:           "fragmented text with multiple runs",
			text:           "Header T h i s i s f r a g m e n t e d middle text A n d t h i s i s a l s o footer",
			wantFragmented: true,
		},
		{
			// Single run of 3+ chars is now detected as fragmented
			name:           "single fragmented run",
			text:           "Some normal words and t h e n s o m e",
			wantFragmented: true, // Changed: 3+ single chars now triggers detection
		},
		{
			// Short run (2 chars) should not trigger
			name:           "short non-fragmented run",
			text:           "This is a b normal text with proper formatting",
			wantFragmented: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tr.DetectFragmentedText(tt.text)
			if got != tt.wantFragmented {
				t.Errorf("DetectFragmentedText() = %v, want %v", got, tt.wantFragmented)
			}
		})
	}
}

func TestTextRepair_RepairFragmentedText(t *testing.T) {
	tr := NewTextRepair()

	text := "Normal word t h i s i s f r a g m e n t e d another word"
	result := tr.RepairFragmentedText(text)

	// The fragmented section should be merged
	// Exact output depends on implementation, but single chars should be merged
	if result == text {
		t.Error("RepairFragmentedText() should have changed the text")
	}
	t.Logf("Repaired: %s", result)
}

func TestTextRepair_LevenshteinDistance(t *testing.T) {
	tr := NewTextRepair()

	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "adc", 1},
		{"abc", "xyz", 3},
		{"kitten", "sitting", 3},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := tr.levenshteinDistance(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestLayoutAnalyzer_WithDepositionMode(t *testing.T) {
	la := NewLayoutAnalyzer()

	// Default values
	if la.ColumnGapThreshold != 30.0 {
		t.Errorf("Default ColumnGapThreshold = %f, want 30.0", la.ColumnGapThreshold)
	}
	if la.FilterLineNumbers {
		t.Error("Default FilterLineNumbers should be false")
	}

	// Apply deposition mode
	la.WithDepositionMode()

	if la.ColumnGapThreshold != 12.0 {
		t.Errorf("Deposition ColumnGapThreshold = %f, want 12.0", la.ColumnGapThreshold)
	}
	if !la.FilterLineNumbers {
		t.Error("Deposition FilterLineNumbers should be true")
	}
	if la.MinRowsForColumnPct != 75 {
		t.Errorf("Deposition MinRowsForColumnPct = %d, want 75", la.MinRowsForColumnPct)
	}
}

func TestNewLayoutAnalyzerWithConfig(t *testing.T) {
	cfg := LayoutConfig{
		ColumnGapThreshold:  15.0,
		RowTolerance:        2.5,
		MinRowsForColumnPct: 50,
		FilterLineNumbers:   true,
	}

	la := NewLayoutAnalyzerWithConfig(cfg)

	if la.ColumnGapThreshold != 15.0 {
		t.Errorf("ColumnGapThreshold = %f, want 15.0", la.ColumnGapThreshold)
	}
	if la.RowTolerance != 2.5 {
		t.Errorf("RowTolerance = %f, want 2.5", la.RowTolerance)
	}
	if la.MinRowsForColumnPct != 50 {
		t.Errorf("MinRowsForColumnPct = %d, want 50", la.MinRowsForColumnPct)
	}
	if !la.FilterLineNumbers {
		t.Error("FilterLineNumbers should be true")
	}
	if la.AutoDetectLayout {
		t.Error("AutoDetectLayout should be false when using explicit config")
	}
}

func TestTextRepair_DetectMirroredText(t *testing.T) {
	tr := NewTextRepair()

	tests := []struct {
		name           string
		text           string
		wantMirrored   bool
		minConfidence  float64
	}{
		{
			name:         "normal English text",
			text:         "The quick brown fox jumps over the lazy dog. This is a sample of normal English text with proper word order.",
			wantMirrored: false,
		},
		{
			name:          "reversed English text",
			text:          "ehT kciuq nworb xof spmuj revo eht yzal god. sihT si a elpmas fo desrever txet.",
			wantMirrored:  true,
			minConfidence: 0.3,
		},
		{
			name:          "mixed reversed words",
			text:          "eht dna rof era tub ton uoy lla nac dah reh saw eno ruo tuo yad teg sah mih sih",
			wantMirrored:  true,
			minConfidence: 0.3,
		},
		{
			name:         "short text",
			text:         "Hello world",
			wantMirrored: false, // Too short to analyze
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := tr.DetectMirroredText(tt.text)
			isMirrored := confidence >= 0.3

			if isMirrored != tt.wantMirrored {
				t.Errorf("DetectMirroredText() mirrored = %v (confidence=%.2f), want mirrored = %v",
					isMirrored, confidence, tt.wantMirrored)
			}

			if tt.wantMirrored && tt.minConfidence > 0 && confidence < tt.minConfidence {
				t.Errorf("DetectMirroredText() confidence = %.2f, want >= %.2f",
					confidence, tt.minConfidence)
			}
		})
	}
}

func TestTextRepair_RepairMirroredText(t *testing.T) {
	tr := NewTextRepair()

	tests := []struct {
		name     string
		text     string
		wantFixed bool
	}{
		{
			name:      "word-reversed text",
			text:      "ehT kciuq nworb xof spmuj revo eht yzal god",
			wantFixed: true,
		},
		{
			name:      "normal text unchanged",
			text:      "The quick brown fox jumps over the lazy dog",
			wantFixed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.RepairMirroredText(tt.text)
			wasFixed := result != tt.text

			if wasFixed != tt.wantFixed {
				t.Errorf("RepairMirroredText() fixed = %v, want %v\nInput:  %s\nOutput: %s",
					wasFixed, tt.wantFixed, tt.text, result)
			}

			if tt.wantFixed {
				// Verify the repaired text looks more like English
				origScore := tr.DetectMirroredText(tt.text)
				repairedScore := tr.DetectMirroredText(result)
				if repairedScore >= origScore {
					t.Errorf("Repaired text should have lower mirrored score: orig=%.2f, repaired=%.2f",
						origScore, repairedScore)
				}
				t.Logf("Repaired: %s (score: %.2f -> %.2f)", result, origScore, repairedScore)
			}
		})
	}
}

func TestTextRepair_BigramExtraction(t *testing.T) {
	tr := NewTextRepair()

	text := "hello world"
	bigrams := tr.extractBigrams(text)

	// Check expected bigrams
	expectedBigrams := []string{"he", "el", "ll", "lo", "wo", "or", "rl", "ld"}
	for _, expected := range expectedBigrams {
		if _, ok := bigrams[expected]; !ok {
			t.Errorf("Expected bigram %q not found", expected)
		}
	}

	// Check that non-adjacent letters don't form bigrams
	if _, ok := bigrams["ow"]; ok {
		t.Error("Bigram 'ow' should not exist (space between 'o' and 'w')")
	}
}
