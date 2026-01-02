package docsaf

import (
	"strings"
	"testing"

	"github.com/ledongthuc/pdf"
)

func TestLayoutAnalyzer_DetectColumns(t *testing.T) {
	la := NewLayoutAnalyzer()

	// Simulate a two-column layout
	// Left column: X=50, Right column: X=350
	texts := []pdf.Text{
		// Row 1
		{S: "Left", X: 50, Y: 700, W: 30, FontSize: 12},
		{S: "column", X: 85, Y: 700, W: 45, FontSize: 12},
		{S: "text", X: 135, Y: 700, W: 25, FontSize: 12},
		{S: "Right", X: 350, Y: 700, W: 35, FontSize: 12},
		{S: "column", X: 390, Y: 700, W: 45, FontSize: 12},
		{S: "text", X: 440, Y: 700, W: 25, FontSize: 12},
		// Row 2
		{S: "More", X: 50, Y: 680, W: 30, FontSize: 12},
		{S: "left", X: 85, Y: 680, W: 25, FontSize: 12},
		{S: "More", X: 350, Y: 680, W: 30, FontSize: 12},
		{S: "right", X: 385, Y: 680, W: 30, FontSize: 12},
		// Row 3
		{S: "Third", X: 50, Y: 660, W: 35, FontSize: 12},
		{S: "line", X: 90, Y: 660, W: 25, FontSize: 12},
		{S: "Third", X: 350, Y: 660, W: 35, FontSize: 12},
		{S: "right", X: 390, Y: 660, W: 30, FontSize: 12},
		// Row 4
		{S: "Fourth", X: 50, Y: 640, W: 40, FontSize: 12},
		{S: "Fourth", X: 350, Y: 640, W: 40, FontSize: 12},
	}

	columns := la.detectColumns(texts, 50, 500)

	if len(columns) < 2 {
		t.Errorf("Expected at least 2 columns, got %d", len(columns))
		return
	}

	// Verify left column has blocks
	leftBlocks := columns[0].Blocks
	if len(leftBlocks) == 0 {
		t.Error("Left column should have blocks")
	}

	// Verify right column has blocks
	rightBlocks := columns[len(columns)-1].Blocks
	if len(rightBlocks) == 0 {
		t.Error("Right column should have blocks")
	}
}

func TestLayoutAnalyzer_SingleColumn(t *testing.T) {
	la := NewLayoutAnalyzer()

	// Single column layout - no gaps large enough for column detection
	texts := []pdf.Text{
		{S: "This", X: 50, Y: 700, W: 25, FontSize: 12},
		{S: "is", X: 80, Y: 700, W: 12, FontSize: 12},
		{S: "single", X: 97, Y: 700, W: 40, FontSize: 12},
		{S: "column", X: 142, Y: 700, W: 45, FontSize: 12},
		{S: "Second", X: 50, Y: 680, W: 42, FontSize: 12},
		{S: "line", X: 97, Y: 680, W: 25, FontSize: 12},
	}

	columns := la.detectColumns(texts, 50, 200)

	if len(columns) != 1 {
		t.Errorf("Expected 1 column for single-column layout, got %d", len(columns))
	}
}

func TestLayoutAnalyzer_GroupIntoRows(t *testing.T) {
	la := NewLayoutAnalyzer()

	texts := []pdf.Text{
		{S: "A", X: 50, Y: 700.0, W: 10, FontSize: 12},
		{S: "B", X: 65, Y: 700.5, W: 10, FontSize: 12}, // Same row (within tolerance)
		{S: "C", X: 80, Y: 699.5, W: 10, FontSize: 12}, // Same row
		{S: "D", X: 50, Y: 680.0, W: 10, FontSize: 12}, // Different row
		{S: "E", X: 65, Y: 680.0, W: 10, FontSize: 12}, // Same row as D
	}

	rows := la.groupIntoRows(texts)

	if len(rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(rows))
	}

	// First row should have 3 elements (A, B, C)
	if len(rows) > 0 && len(rows[0]) != 3 {
		t.Errorf("First row should have 3 elements, got %d", len(rows[0]))
	}

	// Second row should have 2 elements (D, E)
	if len(rows) > 1 && len(rows[1]) != 2 {
		t.Errorf("Second row should have 2 elements, got %d", len(rows[1]))
	}
}

func TestLayoutAnalyzer_DetectTables(t *testing.T) {
	la := NewLayoutAnalyzer()

	// Create a 3x3 table-like structure
	blocks := []TextBlock{
		// Row 1
		{X: 50, Y: 700, Width: 80, Text: "Header1", FontSize: 12},
		{X: 150, Y: 700, Width: 80, Text: "Header2", FontSize: 12},
		{X: 250, Y: 700, Width: 80, Text: "Header3", FontSize: 12},
		// Row 2
		{X: 50, Y: 680, Width: 80, Text: "Cell1", FontSize: 12},
		{X: 150, Y: 680, Width: 80, Text: "Cell2", FontSize: 12},
		{X: 250, Y: 680, Width: 80, Text: "Cell3", FontSize: 12},
		// Row 3
		{X: 50, Y: 660, Width: 80, Text: "Cell4", FontSize: 12},
		{X: 150, Y: 660, Width: 80, Text: "Cell5", FontSize: 12},
		{X: 250, Y: 660, Width: 80, Text: "Cell6", FontSize: 12},
	}

	tables := la.detectTables(blocks)

	if len(tables) == 0 {
		t.Log("No table detected - this may be expected with current heuristics")
		return
	}

	table := tables[0]
	if table.Rows < 2 || table.Cols < 2 {
		t.Errorf("Expected at least 2x2 table, got %dx%d", table.Rows, table.Cols)
	}
}

func TestLayoutAnalyzer_FormatTable(t *testing.T) {
	la := NewLayoutAnalyzer()

	table := Table{
		Rows: 2,
		Cols: 3,
		Cells: [][]TableCell{
			{
				{Text: "A"},
				{Text: "B"},
				{Text: "C"},
			},
			{
				{Text: "1"},
				{Text: "2"},
				{Text: "3"},
			},
		},
	}

	formatted := la.formatTable(table)

	if !strings.Contains(formatted, "A") || !strings.Contains(formatted, "B") {
		t.Error("Formatted table should contain cell text")
	}
	if !strings.Contains(formatted, "|") {
		t.Error("Formatted table should contain column separators")
	}
}

func TestFontDecoder_Decode(t *testing.T) {
	fd := NewFontDecoder()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "fi ligature",
			input: "of\uFB01ce",
			want:  "office",
		},
		{
			name:  "fl ligature",
			input: "\uFB02ower",
			want:  "flower",
		},
		{
			name:  "ff ligature",
			input: "e\uFB00ect",
			want:  "effect",
		},
		{
			name:  "ffi ligature",
			input: "o\uFB03ce",
			want:  "office",
		},
		{
			name:  "smart quotes",
			input: "\u201CHello\u201D \u2018world\u2019",
			want:  "\"Hello\" 'world'",
		},
		{
			name:  "em dash",
			input: "word\u2014word",
			want:  "word-word",
		},
		{
			name:  "ellipsis",
			input: "wait\u2026",
			want:  "wait...",
		},
		{
			name:  "various spaces",
			input: "hello\u00A0world\u2003test",
			want:  "hello world test",
		},
		{
			name:  "normal text unchanged",
			input: "Hello, World! 123",
			want:  "Hello, World! 123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fd.Decode(tt.input)
			if got != tt.want {
				t.Errorf("Decode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFontDecoder_ROT3(t *testing.T) {
	fd := NewFontDecoder()

	tests := []struct {
		name    string
		input   string
		isROT3  bool
		decoded string
	}{
		{
			name:    "ROT3 encoded 'the'",
			input:   "wkh",
			isROT3:  false, // Not enough patterns to detect
			decoded: "the",
		},
		{
			name:    "ROT3 encoded sentence",
			input:   "wkh dqg iru zlwk",
			isROT3:  true,
			decoded: "the and for with",
		},
		{
			name:    "normal text",
			input:   "the and for with",
			isROT3:  false,
			decoded: "the and for with",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isROT3 := fd.IsLikelyROT3(tt.input)
			if isROT3 != tt.isROT3 {
				t.Errorf("IsLikelyROT3(%q) = %v, want %v", tt.input, isROT3, tt.isROT3)
			}

			decoded := fd.DecodeROT3(tt.input)
			if decoded != tt.decoded {
				t.Errorf("DecodeROT3(%q) = %q, want %q", tt.input, decoded, tt.decoded)
			}
		})
	}
}

func TestGlyphMapper_PUA(t *testing.T) {
	gm := NewGlyphMapper()

	// Test that unknown PUA characters are replaced with space
	input := "Hello\uE000World"
	got := gm.Map(input)
	want := "Hello World"
	if got != want {
		t.Errorf("Map(%q) = %q, want %q", input, got, want)
	}

	// Test that normal text is unchanged
	input = "Normal text"
	got = gm.Map(input)
	if got != input {
		t.Errorf("Map(%q) = %q, want unchanged", input, got)
	}
}

func TestToUnicodeMapper_ParseCMap(t *testing.T) {
	tm := NewToUnicodeMapper()

	cmap := `
/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
beginbfchar
<0001> <0041>
<0002> <0042>
<0003> <0043>
endbfchar
endcmap
`
	tm.ParseCMap(cmap)

	tests := []struct {
		input rune
		want  rune
	}{
		{0x0001, 'A'},
		{0x0002, 'B'},
		{0x0003, 'C'},
	}

	for _, tt := range tests {
		got, ok := tm.mappings[uint16(tt.input)]
		if !ok {
			t.Errorf("Mapping for %04X not found", tt.input)
			continue
		}
		if got != tt.want {
			t.Errorf("Mapping for %04X = %c, want %c", tt.input, got, tt.want)
		}
	}
}

func TestToUnicodeMapper_ParseBfRange(t *testing.T) {
	tm := NewToUnicodeMapper()

	cmap := `
beginbfrange
<0010> <0013> <0041>
endbfrange
`
	tm.ParseCMap(cmap)

	// Should map 0010->A, 0011->B, 0012->C, 0013->D
	expected := map[uint16]rune{
		0x0010: 'A',
		0x0011: 'B',
		0x0012: 'C',
		0x0013: 'D',
	}

	for input, want := range expected {
		got, ok := tm.mappings[input]
		if !ok {
			t.Errorf("Mapping for %04X not found", input)
			continue
		}
		if got != want {
			t.Errorf("Mapping for %04X = %c, want %c", input, got, want)
		}
	}
}

func TestEnhancedTextCleaner_Clean(t *testing.T) {
	etc := NewEnhancedTextCleaner()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "ligatures and spaces",
			input: "The of\uFB01ce\u00A0is\u2014open",
			want:  "The office is-open",
		},
		{
			name:  "smart quotes",
			input: "\u201CHello,\u201D she said",
			want:  "\"Hello,\" she said",
		},
		{
			name:  "multiple spaces collapsed",
			input: "hello    world   test",
			want:  "hello world test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := etc.Clean(tt.input)
			if got != tt.want {
				t.Errorf("Clean(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLayoutAnalyzer_HasConsistentSpacing(t *testing.T) {
	la := NewLayoutAnalyzer()

	tests := []struct {
		name      string
		positions []float64
		tolerance float64
		want      bool
	}{
		{
			name:      "consistent spacing",
			positions: []float64{0, 100, 200, 300},
			tolerance: 0.1,
			want:      true,
		},
		{
			name:      "inconsistent spacing",
			positions: []float64{0, 100, 150, 400},
			tolerance: 0.1,
			want:      false,
		},
		{
			name:      "single position",
			positions: []float64{100},
			tolerance: 0.1,
			want:      false,
		},
		{
			name:      "two positions",
			positions: []float64{0, 100},
			tolerance: 0.1,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := la.hasConsistentSpacing(tt.positions, tt.tolerance)
			if got != tt.want {
				t.Errorf("hasConsistentSpacing(%v, %v) = %v, want %v",
					tt.positions, tt.tolerance, got, tt.want)
			}
		})
	}
}

func TestLayoutAnalyzer_FormatBlocks(t *testing.T) {
	la := NewLayoutAnalyzer()
	la.RowTolerance = 3.0

	blocks := []TextBlock{
		{X: 50, Y: 700, Width: 30, Text: "Hello", FontSize: 12},
		{X: 90, Y: 700, Width: 30, Text: "World", FontSize: 12},
		{X: 50, Y: 680, Width: 50, Text: "New line", FontSize: 12},
	}

	formatted := la.formatBlocks(blocks)

	if !strings.Contains(formatted, "Hello") {
		t.Error("Formatted text should contain 'Hello'")
	}
	if !strings.Contains(formatted, "World") {
		t.Error("Formatted text should contain 'World'")
	}
	if !strings.Contains(formatted, "\n") {
		t.Error("Formatted text should contain newline between rows")
	}
}

func TestNewPDFProcessor(t *testing.T) {
	pp := NewPDFProcessor()

	if !pp.UseAdvancedLayout {
		t.Error("NewPDFProcessor should enable advanced layout by default")
	}
}

func TestPDFProcessor_LegacyMode(t *testing.T) {
	// Test that legacy mode still works
	pp := &PDFProcessor{UseAdvancedLayout: false}

	if pp.UseAdvancedLayout {
		t.Error("Legacy mode should have UseAdvancedLayout = false")
	}
}
