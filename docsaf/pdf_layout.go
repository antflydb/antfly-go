package docsaf

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

// LayoutAnalyzer provides advanced PDF text extraction with column detection,
// table recognition, and improved reading order reconstruction.
type LayoutAnalyzer struct {
	// Configuration
	ColumnGapThreshold  float64 // Minimum gap width to consider as column separator (in points)
	RowTolerance        float64 // Y-coordinate tolerance for grouping into rows
	TableCellMinWidth   float64 // Minimum cell width to consider for table detection
	WordSpaceMultiplier float64 // Multiplier of font size to detect word boundaries

	// Extended options
	MinRowsForColumnPct int  // Minimum percentage of rows that must have gap for column detection (default 25)
	FilterLineNumbers   bool // Whether to filter out line number columns (for depositions)
	AutoDetectLayout    bool // Automatically detect and use optimal layout settings
	UseAdaptiveSpacing  bool // Use dynamic spacing threshold based on actual character spacing (default true)
}

// NewLayoutAnalyzer creates a LayoutAnalyzer with sensible defaults.
func NewLayoutAnalyzer() *LayoutAnalyzer {
	return &LayoutAnalyzer{
		ColumnGapThreshold:  30.0, // 30pt gap suggests column boundary
		RowTolerance:        3.0,  // 3pt Y tolerance for same row
		TableCellMinWidth:   20.0, // 20pt minimum cell width
		WordSpaceMultiplier: 0.3,  // 30% of font size = space
		MinRowsForColumnPct: 25,   // 25% of rows must have gap
		FilterLineNumbers:   false,
		AutoDetectLayout:    true, // Enable auto-detection by default
		UseAdaptiveSpacing:  true, // Enable adaptive spacing by default
	}
}

// NewLayoutAnalyzerWithConfig creates a LayoutAnalyzer with custom configuration.
// Note: UseAdaptiveSpacing defaults to false when using explicit configuration.
// Set it to true manually if you want adaptive spacing with custom settings.
func NewLayoutAnalyzerWithConfig(cfg LayoutConfig) *LayoutAnalyzer {
	return &LayoutAnalyzer{
		ColumnGapThreshold:  cfg.ColumnGapThreshold,
		RowTolerance:        cfg.RowTolerance,
		TableCellMinWidth:   20.0,
		WordSpaceMultiplier: 0.3,
		MinRowsForColumnPct: cfg.MinRowsForColumnPct,
		FilterLineNumbers:   cfg.FilterLineNumbers,
		AutoDetectLayout:    false, // Explicit config disables auto-detect
		UseAdaptiveSpacing:  false, // Explicit config uses fixed threshold
	}
}

// WithDepositionMode configures the analyzer for deposition transcript extraction.
// This uses tighter column detection and filters out line number columns.
func (la *LayoutAnalyzer) WithDepositionMode() *LayoutAnalyzer {
	la.ColumnGapThreshold = 12.0  // Narrow gaps for line number columns
	la.RowTolerance = 2.0         // Tight row grouping
	la.MinRowsForColumnPct = 75   // Line numbers on most rows
	la.FilterLineNumbers = true   // Remove line number column from output
	la.AutoDetectLayout = false   // Explicit mode
	return la
}

// TextBlock represents a block of text with position and content.
type TextBlock struct {
	X, Y          float64
	Width, Height float64
	Text          string
	FontSize      float64
	Chars         []pdf.Text // Original characters
}

// Column represents a detected column region.
type Column struct {
	Left, Right float64
	Blocks      []TextBlock
}

// TableCell represents a cell in a detected table.
type TableCell struct {
	Row, Col      int
	X, Y          float64
	Width, Height float64
	Text          string
}

// Table represents a detected table structure.
type Table struct {
	X, Y          float64
	Width, Height float64
	Rows          int
	Cols          int
	Cells         [][]TableCell
}

// ExtractWithLayout performs advanced text extraction with layout analysis.
// It detects columns, tables, and reconstructs proper reading order.
// When AutoDetectLayout is enabled, it automatically detects document type
// (e.g., deposition transcripts) and adjusts settings accordingly.
func (la *LayoutAnalyzer) ExtractWithLayout(page pdf.Page) string {
	content := page.Content()
	if len(content.Text) == 0 {
		return ""
	}

	// Filter out empty and newline-only text elements
	texts := la.filterTexts(content.Text)
	if len(texts) == 0 {
		return ""
	}

	// Auto-detect document type and adjust settings if enabled
	if la.AutoDetectLayout {
		tr := NewTextRepair()
		if tr.DetectDepositionLayout(texts) {
			// Switch to deposition mode for this extraction
			la.ColumnGapThreshold = 12.0
			la.RowTolerance = 2.0
			la.MinRowsForColumnPct = 75
			la.FilterLineNumbers = true
		}
	}

	// Filter out line number columns for deposition transcripts
	if la.FilterLineNumbers {
		tr := NewTextRepair()
		texts = tr.FilterLineNumberColumn(texts)
	}

	// Fix mirrored text (reversed character order due to PDF rendering issues)
	texts = la.fixMirroredTextByRow(texts)

	// Detect page boundaries
	pageLeft, pageRight, _, _ := la.getPageBounds(texts)

	// Detect columns
	columns := la.detectColumns(texts, pageLeft, pageRight)

	// Check for tables within each column
	var result strings.Builder
	for colIdx, col := range columns {
		if colIdx > 0 {
			result.WriteString("\n\n")
		}

		// Try to detect tables in this column
		tables := la.detectTables(col.Blocks)
		if len(tables) > 0 {
			result.WriteString(la.formatTablesAndText(col.Blocks, tables))
		} else {
			result.WriteString(la.formatBlocks(col.Blocks))
		}
	}

	return result.String()
}

// filterTexts removes empty and newline-only text elements.
func (la *LayoutAnalyzer) filterTexts(texts []pdf.Text) []pdf.Text {
	filtered := make([]pdf.Text, 0, len(texts))
	for _, t := range texts {
		s := strings.TrimSpace(t.S)
		if s != "" && s != "\n" {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// getPageBounds returns the bounding box of all text on the page.
func (la *LayoutAnalyzer) getPageBounds(texts []pdf.Text) (left, right, top, bottom float64) {
	if len(texts) == 0 {
		return 0, 0, 0, 0
	}

	left = texts[0].X
	right = texts[0].X + texts[0].W
	top = texts[0].Y
	bottom = texts[0].Y

	for _, t := range texts[1:] {
		if t.X < left {
			left = t.X
		}
		if t.X+t.W > right {
			right = t.X + t.W
		}
		if t.Y > top {
			top = t.Y
		}
		if t.Y < bottom {
			bottom = t.Y
		}
	}

	return left, right, top, bottom
}

// detectColumns identifies column boundaries based on consistent vertical gaps.
func (la *LayoutAnalyzer) detectColumns(texts []pdf.Text, pageLeft, pageRight float64) []Column {
	// Group texts into rows first
	rows := la.groupIntoRows(texts)
	if len(rows) == 0 {
		return nil
	}

	// Build a gap histogram across all rows
	// Each gap is recorded with its X position and width
	type gap struct {
		left, right float64
	}
	gapCounts := make(map[int]int) // Bucketed X position -> count

	bucketSize := 20.0 // 20pt buckets for gap positions (tolerates variable column widths)

	for _, row := range rows {
		// Sort row by X position
		sortedRow := make([]pdf.Text, len(row))
		copy(sortedRow, row)
		sort.Slice(sortedRow, func(i, j int) bool {
			return sortedRow[i].X < sortedRow[j].X
		})

		// Find gaps between consecutive text elements
		for i := 0; i < len(sortedRow)-1; i++ {
			gapLeft := sortedRow[i].X + sortedRow[i].W
			gapRight := sortedRow[i+1].X
			gapWidth := gapRight - gapLeft

			// Only consider significant gaps
			if gapWidth >= la.ColumnGapThreshold {
				// Use the center of the gap as the bucket key
				gapCenter := (gapLeft + gapRight) / 2
				bucket := int(gapCenter / bucketSize)
				gapCounts[bucket]++
			}
		}
	}

	// Find gaps that appear in many rows (consistent column boundaries)
	// Use configurable percentage (default 25%, deposition mode uses 75%)
	pct := la.MinRowsForColumnPct
	if pct <= 0 {
		pct = 25
	}
	minRowsForColumn := len(rows) * pct / 100
	if minRowsForColumn < 3 {
		minRowsForColumn = 3
	}

	var columnBoundaries []float64
	for bucket, count := range gapCounts {
		if count >= minRowsForColumn {
			columnBoundaries = append(columnBoundaries, float64(bucket)*bucketSize+bucketSize/2)
		}
	}

	// Sort boundaries
	sort.Float64s(columnBoundaries)

	// Merge nearby boundaries (within bucketSize*2)
	mergedBoundaries := []float64{}
	for _, b := range columnBoundaries {
		if len(mergedBoundaries) == 0 || b-mergedBoundaries[len(mergedBoundaries)-1] > bucketSize*2 {
			mergedBoundaries = append(mergedBoundaries, b)
		}
	}

	// Create columns based on boundaries
	if len(mergedBoundaries) == 0 {
		// No column boundaries detected - single column
		return []Column{{
			Left:   pageLeft,
			Right:  pageRight,
			Blocks: la.textsToBlocks(texts),
		}}
	}

	// Create column regions
	columns := make([]Column, len(mergedBoundaries)+1)
	columns[0] = Column{Left: pageLeft, Right: mergedBoundaries[0]}
	for i := 0; i < len(mergedBoundaries)-1; i++ {
		columns[i+1] = Column{Left: mergedBoundaries[i], Right: mergedBoundaries[i+1]}
	}
	columns[len(mergedBoundaries)] = Column{Left: mergedBoundaries[len(mergedBoundaries)-1], Right: pageRight}

	// Collect texts for each column
	columnTexts := make([][]pdf.Text, len(columns))
	for _, t := range texts {
		centerX := t.X + t.W/2
		for i := range columns {
			if centerX >= columns[i].Left && centerX <= columns[i].Right {
				columnTexts[i] = append(columnTexts[i], t)
				break
			}
		}
	}

	// Convert texts to blocks for each column using textsToBlocks
	// This properly groups characters into words and handles interleaved rows
	for i := range columns {
		if len(columnTexts[i]) > 0 {
			columns[i].Blocks = la.textsToBlocks(columnTexts[i])
		}
	}

	// Remove empty columns and sort blocks within each column
	nonEmptyColumns := make([]Column, 0, len(columns))
	for _, col := range columns {
		if len(col.Blocks) > 0 {
			// Sort blocks top-to-bottom, left-to-right
			sort.Slice(col.Blocks, func(i, j int) bool {
				if math.Abs(col.Blocks[i].Y-col.Blocks[j].Y) < la.RowTolerance {
					return col.Blocks[i].X < col.Blocks[j].X
				}
				return col.Blocks[i].Y > col.Blocks[j].Y // Higher Y = higher on page
			})
			nonEmptyColumns = append(nonEmptyColumns, col)
		}
	}

	return nonEmptyColumns
}

// fixMirroredTextByRow processes texts row by row and fixes mirrored runs.
// Mirrored text appears when PDFs use negative horizontal scaling or
// reversed glyph ordering, causing text like "PALM BEACH" to appear as "MLAP HCAEB".
func (la *LayoutAnalyzer) fixMirroredTextByRow(texts []pdf.Text) []pdf.Text {
	if len(texts) == 0 {
		return texts
	}

	// Group texts into rows
	rows := la.groupIntoRows(texts)

	// Fix mirrored runs in each row and collect results
	var result []pdf.Text
	for _, row := range rows {
		fixed := la.reverseMirroredRuns(row)
		result = append(result, fixed...)
	}

	return result
}

// isMirroredRun checks if a sequence of Text elements appears to be a mirrored run.
// Mirrored runs have compressed X positions AND X decreases in stream order.
//
// Key insight: Mirrored text has characters in the correct visual order in the stream,
// but with X positions that DECREASE (rendered right-to-left). Normal text has
// characters with X positions that INCREASE (rendered left-to-right).
//
// For all runs (2+ characters):
// 1. Compressed spacing: chars at nearly same X (< 4% of font size)
// 2. X decreasing: X positions decrease in stream order (right-to-left rendering)
//
// Some PDFs have compressed X coordinates but correct stream order (X increasing).
// These should NOT be reversed - they're just tightly spaced normal text.
func (la *LayoutAnalyzer) isMirroredRun(texts []pdf.Text) bool {
	if len(texts) < 2 {
		return false
	}

	// Calculate average spacing and check X direction in original (stream) order
	totalSpacing := 0.0
	xIncreasing := 0
	xDecreasing := 0

	for i := 1; i < len(texts); i++ {
		spacing := texts[i].X - texts[i-1].X

		// Track X direction
		if spacing > 0 {
			xIncreasing++
		} else if spacing < 0 {
			xDecreasing++
		}

		totalSpacing += math.Abs(spacing)
	}
	avgSpacing := totalSpacing / float64(len(texts)-1)

	// Calculate average font size
	totalFontSize := 0.0
	for _, t := range texts {
		totalFontSize += t.FontSize
	}
	avgFontSize := totalFontSize / float64(len(texts))
	if avgFontSize == 0 {
		avgFontSize = 10.0
	}

	// Must be compressed spacing (< 4% of font size)
	isCompressed := avgSpacing < avgFontSize*0.04

	// Must have X decreasing in stream order (mirrored rendering)
	// For 2-char runs: X must be strictly decreasing (xDecreasing == 1, xIncreasing == 0)
	// For 3+ char runs: X must be mostly decreasing (xDecreasing > xIncreasing)
	var isMirrored bool
	if len(texts) == 2 {
		// For 2-char, require strict decrease (the one transition must be decreasing)
		isMirrored = xDecreasing == 1 && xIncreasing == 0
	} else {
		// For 3+ chars, majority must be decreasing
		isMirrored = xDecreasing > xIncreasing
	}

	return isCompressed && isMirrored
}

// reverseMirroredRuns reverses the order of characters in mirrored runs
// and fixes their X positions to be in proper reading order.
// This should be called on a slice of Text elements from the same row.
//
// The algorithm works in stream order (original PDF order) to detect runs
// where X positions decrease (indicating right-to-left rendering).
func (la *LayoutAnalyzer) reverseMirroredRuns(texts []pdf.Text) []pdf.Text {
	if len(texts) == 0 {
		return texts
	}

	// Work in stream order to detect runs with decreasing X
	result := make([]pdf.Text, 0, len(texts))
	i := 0

	for i < len(texts) {
		// Find extent of this potential mirrored run in stream order
		// A mirrored run has compressed spacing AND mostly decreasing X
		runStart := i
		runEnd := i + 1

		for runEnd < len(texts) {
			spacing := texts[runEnd].X - texts[runEnd-1].X
			fontSize := texts[runEnd].FontSize
			if fontSize == 0 {
				fontSize = 10.0
			}

			// Check for compressed spacing (absolute value < 4% of font size)
			// This includes both slightly increasing and decreasing X
			if math.Abs(spacing) > fontSize*0.04 {
				break // Normal spacing, end of potential run
			}
			runEnd++
		}

		run := texts[runStart:runEnd]

		// Check if this run should be reversed
		// isMirroredRun handles both 2-char (strict compression) and 3+ char (compression + X direction)
		if len(run) >= 2 && la.isMirroredRun(run) {
			// Sort by X to get correct left-to-right order
			sorted := make([]pdf.Text, len(run))
			copy(sorted, run)
			sort.Slice(sorted, func(a, b int) bool {
				return sorted[a].X < sorted[b].X
			})

			// Reverse the sorted order to correct the mirroring
			reversed := make([]pdf.Text, len(sorted))
			for j := 0; j < len(sorted); j++ {
				reversed[j] = sorted[len(sorted)-1-j]
			}

			// Fix X positions: redistribute with tight spacing so chars merge into words
			// 20% of font size ensures chars are treated as same word (threshold is 30%)
			startX := sorted[0].X
			charSpacing := sorted[0].FontSize * 0.2
			if charSpacing == 0 {
				charSpacing = 2.0
			}
			for j := range reversed {
				reversed[j].X = startX + float64(j)*charSpacing
				reversed[j].W = charSpacing
			}

			result = append(result, reversed...)
		} else {
			result = append(result, run...)
		}

		i = runEnd
	}

	return result
}

// groupIntoRows groups text elements by Y coordinate.
func (la *LayoutAnalyzer) groupIntoRows(texts []pdf.Text) [][]pdf.Text {
	if len(texts) == 0 {
		return nil
	}

	type rowBucket struct {
		yMin, yMax float64
		texts      []pdf.Text
	}

	var buckets []rowBucket

	for _, t := range texts {
		found := false
		for i := range buckets {
			if t.Y >= buckets[i].yMin-la.RowTolerance && t.Y <= buckets[i].yMax+la.RowTolerance {
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

	// Post-process: split buckets that have multiple distinct Y clusters.
	// This handles cases where two rows at similar Y (e.g., Y=767 and Y=769.5)
	// get merged but would interleave when sorted by X.
	var finalBuckets []rowBucket
	for _, bucket := range buckets {
		split := la.splitInterleavedRows(bucket.texts)
		for _, row := range split {
			if len(row) == 0 {
				continue
			}
			yMin, yMax := row[0].Y, row[0].Y
			for _, t := range row[1:] {
				if t.Y < yMin {
					yMin = t.Y
				}
				if t.Y > yMax {
					yMax = t.Y
				}
			}
			finalBuckets = append(finalBuckets, rowBucket{yMin: yMin, yMax: yMax, texts: row})
		}
	}

	// Sort buckets by Y (top to bottom = higher Y first)
	sort.Slice(finalBuckets, func(i, j int) bool {
		return finalBuckets[i].yMax > finalBuckets[j].yMax
	})

	rows := make([][]pdf.Text, len(finalBuckets))
	for i, bucket := range finalBuckets {
		rows[i] = bucket.texts
	}

	return rows
}

// splitInterleavedRows checks if a row has multiple distinct Y values that would
// cause interleaving when sorted by X. If so, it splits them into separate rows.
// This handles PDFs with multiple header lines at similar Y positions.
func (la *LayoutAnalyzer) splitInterleavedRows(texts []pdf.Text) [][]pdf.Text {
	if len(texts) < 4 {
		return [][]pdf.Text{texts}
	}

	// Find distinct Y values
	yValues := make(map[float64][]pdf.Text)
	for _, t := range texts {
		// Round Y to 0.1 precision to group very close values
		roundedY := math.Round(t.Y*10) / 10
		yValues[roundedY] = append(yValues[roundedY], t)
	}

	// If only one distinct Y, no splitting needed
	if len(yValues) <= 1 {
		return [][]pdf.Text{texts}
	}

	// Check if this would cause interleaving:
	// If different Y values have overlapping X ranges, they'll interleave when sorted by X
	type yRange struct {
		y        float64
		xMin     float64
		xMax     float64
		texts    []pdf.Text
		charCount int
	}

	var ranges []yRange
	for y, yTexts := range yValues {
		if len(yTexts) < 3 {
			// Very few chars at this Y, probably noise
			continue
		}
		xMin, xMax := yTexts[0].X, yTexts[0].X
		for _, t := range yTexts[1:] {
			if t.X < xMin {
				xMin = t.X
			}
			if t.X > xMax {
				xMax = t.X
			}
		}
		ranges = append(ranges, yRange{y: y, xMin: xMin, xMax: xMax, texts: yTexts, charCount: len(yTexts)})
	}

	if len(ranges) <= 1 {
		return [][]pdf.Text{texts}
	}

	// Check for X overlap between any two Y ranges
	hasOverlap := false
	for i := 0; i < len(ranges) && !hasOverlap; i++ {
		for j := i + 1; j < len(ranges); j++ {
			// Check if ranges overlap: NOT (r1.max < r2.min OR r2.max < r1.min)
			if !(ranges[i].xMax < ranges[j].xMin || ranges[j].xMax < ranges[i].xMin) {
				// They overlap - check if both have some content
				// Use low threshold (3) since column detection may split rows
				if ranges[i].charCount >= 3 && ranges[j].charCount >= 3 {
					hasOverlap = true
					break
				}
			}
		}
	}

	if !hasOverlap {
		// No significant overlap, keep as single row
		return [][]pdf.Text{texts}
	}

	// Split into separate rows by rounded Y value
	result := make([][]pdf.Text, 0, len(yValues))
	for _, yTexts := range yValues {
		if len(yTexts) > 0 {
			result = append(result, yTexts)
		}
	}

	return result
}

// calculateMedianCharSpacing calculates the median spacing between consecutive characters
// on the same row. This provides a data-driven baseline for determining word boundaries.
// Returns 0 if there's insufficient data to calculate a reliable median.
func (la *LayoutAnalyzer) calculateMedianCharSpacing(texts []pdf.Text) float64 {
	if len(texts) < 10 {
		return 0
	}

	var spacings []float64
	for i := 1; i < len(texts); i++ {
		// Only consider characters on the same row (within Y tolerance)
		if math.Abs(texts[i].Y-texts[i-1].Y) < la.RowTolerance {
			spacing := texts[i].X - (texts[i-1].X + texts[i-1].W)
			// Only consider positive spacings under a reasonable limit
			// (10x font size covers even very wide letter spacing)
			if spacing > 0 && spacing < texts[i].FontSize*10 {
				spacings = append(spacings, spacing)
			}
		}
	}

	if len(spacings) < 5 {
		return 0
	}

	// Return median spacing (more robust than mean)
	sort.Float64s(spacings)
	return spacings[len(spacings)/2]
}

// textsToBlocks converts pdf.Text elements to TextBlocks, merging adjacent chars into words.
func (la *LayoutAnalyzer) textsToBlocks(texts []pdf.Text) []TextBlock {
	if len(texts) == 0 {
		return nil
	}

	// Calculate adaptive word spacing threshold if enabled
	var adaptiveThreshold float64
	if la.UseAdaptiveSpacing {
		medianSpacing := la.calculateMedianCharSpacing(texts)
		if medianSpacing > 0 {
			// Use 5x median character spacing as word break threshold
			// This adapts to the actual character spacing in the document
			// Higher multiplier needed for PDFs with variable intra-word spacing
			adaptiveThreshold = medianSpacing * 5.0
		}
	}

	// Group into rows first
	rows := la.groupIntoRows(texts)

	var blocks []TextBlock
	for _, row := range rows {
		// Sort by X within row
		sort.Slice(row, func(i, j int) bool {
			return row[i].X < row[j].X
		})

		// Merge characters into words/blocks
		var currentBlock *TextBlock
		for _, t := range row {
			if currentBlock == nil {
				currentBlock = &TextBlock{
					X:        t.X,
					Y:        t.Y,
					Width:    t.W,
					Height:   t.FontSize,
					Text:     t.S,
					FontSize: t.FontSize,
					Chars:    []pdf.Text{t},
				}
				continue
			}

			// Check if this character should be part of the current block
			gap := t.X - (currentBlock.X + currentBlock.Width)

			// Use adaptive threshold if available, otherwise fall back to font-based threshold
			var threshold float64
			if adaptiveThreshold > 0 {
				threshold = adaptiveThreshold
			} else {
				threshold = la.WordSpaceMultiplier * currentBlock.FontSize
				if currentBlock.FontSize == 0 {
					threshold = 3.0 // fallback
				}
			}

			if gap <= threshold {
				// Same word/block - append
				currentBlock.Width = t.X + t.W - currentBlock.X
				currentBlock.Text += t.S
				currentBlock.Chars = append(currentBlock.Chars, t)
			} else {
				// New word - save current and start new
				blocks = append(blocks, *currentBlock)
				currentBlock = &TextBlock{
					X:        t.X,
					Y:        t.Y,
					Width:    t.W,
					Height:   t.FontSize,
					Text:     t.S,
					FontSize: t.FontSize,
					Chars:    []pdf.Text{t},
				}
			}
		}
		if currentBlock != nil {
			blocks = append(blocks, *currentBlock)
		}
	}

	return blocks
}

// detectTables identifies table structures from text blocks.
func (la *LayoutAnalyzer) detectTables(blocks []TextBlock) []Table {
	if len(blocks) < 4 { // Need at least 2x2 for a table
		return nil
	}

	// Find horizontal and vertical alignment patterns
	xPositions := make(map[int][]TextBlock) // Bucketed X -> blocks at that X
	yPositions := make(map[int][]TextBlock) // Bucketed Y -> blocks at that Y

	xBucket := 5.0 // 5pt buckets
	yBucket := 3.0 // 3pt buckets

	for _, b := range blocks {
		xKey := int(b.X / xBucket)
		yKey := int(b.Y / yBucket)
		xPositions[xKey] = append(xPositions[xKey], b)
		yPositions[yKey] = append(yPositions[yKey], b)
	}

	// Find column positions (X positions with multiple aligned blocks)
	var columnXs []float64
	for xKey, aligned := range xPositions {
		if len(aligned) >= 3 { // At least 3 vertically aligned blocks
			columnXs = append(columnXs, float64(xKey)*xBucket)
		}
	}

	// Find row positions (Y positions with multiple aligned blocks)
	var rowYs []float64
	for yKey, aligned := range yPositions {
		if len(aligned) >= 2 { // At least 2 horizontally aligned blocks
			rowYs = append(rowYs, float64(yKey)*yBucket)
		}
	}

	// Need at least 2 columns and 2 rows for a table
	if len(columnXs) < 2 || len(rowYs) < 2 {
		return nil
	}

	sort.Float64s(columnXs)
	sort.Float64s(rowYs)
	// Reverse rowYs so top row is first (higher Y = higher on page)
	for i, j := 0, len(rowYs)-1; i < j; i, j = i+1, j-1 {
		rowYs[i], rowYs[j] = rowYs[j], rowYs[i]
	}

	// Check for consistent spacing (table-like structure)
	if !la.hasConsistentSpacing(columnXs, 0.3) || !la.hasConsistentSpacing(rowYs, 0.3) {
		return nil
	}

	// Build the table
	table := Table{
		X:     columnXs[0],
		Y:     rowYs[0],
		Width: columnXs[len(columnXs)-1] - columnXs[0] + la.TableCellMinWidth,
		Rows:  len(rowYs),
		Cols:  len(columnXs),
		Cells: make([][]TableCell, len(rowYs)),
	}

	for r := range table.Cells {
		table.Cells[r] = make([]TableCell, len(columnXs))
		for c := range table.Cells[r] {
			table.Cells[r][c] = TableCell{
				Row: r,
				Col: c,
				X:   columnXs[c],
				Y:   rowYs[r],
			}
			if c < len(columnXs)-1 {
				table.Cells[r][c].Width = columnXs[c+1] - columnXs[c]
			} else {
				table.Cells[r][c].Width = la.TableCellMinWidth
			}
		}
	}

	// Assign blocks to cells
	for _, b := range blocks {
		// Find matching row
		rowIdx := -1
		for r, rowY := range rowYs {
			if math.Abs(b.Y-rowY) < yBucket*2 {
				rowIdx = r
				break
			}
		}
		if rowIdx == -1 {
			continue
		}

		// Find matching column
		colIdx := -1
		for c, colX := range columnXs {
			if math.Abs(b.X-colX) < xBucket*2 {
				colIdx = c
				break
			}
		}
		if colIdx == -1 {
			continue
		}

		// Add text to cell
		cell := &table.Cells[rowIdx][colIdx]
		if cell.Text != "" {
			cell.Text += " "
		}
		cell.Text += b.Text
	}

	return []Table{table}
}

// hasConsistentSpacing checks if positions have relatively consistent spacing.
func (la *LayoutAnalyzer) hasConsistentSpacing(positions []float64, tolerance float64) bool {
	if len(positions) < 2 {
		return false
	}

	// Calculate spacings
	spacings := make([]float64, len(positions)-1)
	for i := 0; i < len(positions)-1; i++ {
		spacings[i] = math.Abs(positions[i+1] - positions[i])
	}

	// Calculate mean
	var sum float64
	for _, s := range spacings {
		sum += s
	}
	mean := sum / float64(len(spacings))

	if mean == 0 {
		return false
	}

	// Check if all spacings are within tolerance of mean
	for _, s := range spacings {
		if math.Abs(s-mean)/mean > tolerance {
			return false
		}
	}

	return true
}

// formatBlocks converts blocks to text with proper spacing and line breaks.
func (la *LayoutAnalyzer) formatBlocks(blocks []TextBlock) string {
	if len(blocks) == 0 {
		return ""
	}

	var result strings.Builder
	var lastY float64 = -1
	var lastX float64 = -1

	for i, b := range blocks {
		if i == 0 {
			result.WriteString(b.Text)
			lastY = b.Y
			lastX = b.X + b.Width
			continue
		}

		// Check if we're on a new line (Y changed significantly)
		yDiff := math.Abs(lastY - b.Y)
		if yDiff > la.RowTolerance {
			result.WriteString("\n")
			lastX = -1
		} else if lastX >= 0 {
			// Same line - add space if there's a gap
			gap := b.X - lastX
			if gap > la.WordSpaceMultiplier*b.FontSize || gap > 3.0 {
				result.WriteString(" ")
			}
		}

		result.WriteString(b.Text)
		lastY = b.Y
		lastX = b.X + b.Width
	}

	return result.String()
}

// formatTablesAndText formats blocks, rendering detected tables specially.
func (la *LayoutAnalyzer) formatTablesAndText(blocks []TextBlock, tables []Table) string {
	if len(tables) == 0 {
		return la.formatBlocks(blocks)
	}

	// For simplicity, format the first table found
	// (A more complete implementation would interleave text and multiple tables)
	table := tables[0]

	var result strings.Builder

	// Find blocks before the table
	var beforeBlocks, afterBlocks []TextBlock
	for _, b := range blocks {
		if b.Y > table.Y+la.RowTolerance*2 {
			beforeBlocks = append(beforeBlocks, b)
		} else if b.Y < table.Y-float64(table.Rows)*20 { // Rough estimate of table height
			afterBlocks = append(afterBlocks, b)
		}
	}

	// Format blocks before table
	if len(beforeBlocks) > 0 {
		result.WriteString(la.formatBlocks(beforeBlocks))
		result.WriteString("\n\n")
	}

	// Format table
	result.WriteString(la.formatTable(table))

	// Format blocks after table
	if len(afterBlocks) > 0 {
		result.WriteString("\n\n")
		result.WriteString(la.formatBlocks(afterBlocks))
	}

	return result.String()
}

// formatTable renders a table as aligned text.
func (la *LayoutAnalyzer) formatTable(table Table) string {
	if table.Rows == 0 || table.Cols == 0 {
		return ""
	}

	// Calculate column widths
	colWidths := make([]int, table.Cols)
	for c := 0; c < table.Cols; c++ {
		for r := 0; r < table.Rows; r++ {
			cellLen := len(table.Cells[r][c].Text)
			if cellLen > colWidths[c] {
				colWidths[c] = cellLen
			}
		}
		// Minimum width
		if colWidths[c] < 3 {
			colWidths[c] = 3
		}
	}

	var result strings.Builder

	for r := 0; r < table.Rows; r++ {
		if r > 0 {
			result.WriteString("\n")
		}
		for c := 0; c < table.Cols; c++ {
			if c > 0 {
				result.WriteString(" | ")
			}
			cell := table.Cells[r][c].Text
			// Pad cell to column width
			result.WriteString(cell)
			for i := len(cell); i < colWidths[c]; i++ {
				result.WriteString(" ")
			}
		}
	}

	return result.String()
}

// FontDecoder handles font encoding issues common in PDFs.
type FontDecoder struct {
	// Common substitution maps for problematic fonts
	ligatures     map[rune]string
	substitutions map[rune]rune
}

// NewFontDecoder creates a FontDecoder with common substitutions.
func NewFontDecoder() *FontDecoder {
	return &FontDecoder{
		ligatures: map[rune]string{
			'\uFB00': "ff",
			'\uFB01': "fi",
			'\uFB02': "fl",
			'\uFB03': "ffi",
			'\uFB04': "ffl",
			'\uFB05': "st", // long s + t
			'\uFB06': "st",
			'\u0132': "IJ", // Dutch IJ
			'\u0133': "ij",
			'\u0152': "OE", // French OE
			'\u0153': "oe",
			'\u00C6': "AE", // AE ligature
			'\u00E6': "ae",
		},
		substitutions: map[rune]rune{
			// Smart quotes to ASCII
			'\u2018': '\'', // left single quote
			'\u2019': '\'', // right single quote
			'\u201C': '"',  // left double quote
			'\u201D': '"',  // right double quote
			// Dashes
			'\u2013': '-', // en dash
			'\u2014': '-', // em dash
			'\u2015': '-', // horizontal bar
			'\u2212': '-', // minus sign
			// Spaces
			'\u00A0': ' ', // non-breaking space
			'\u2000': ' ', // en quad
			'\u2001': ' ', // em quad
			'\u2002': ' ', // en space
			'\u2003': ' ', // em space
			'\u2004': ' ', // three-per-em space
			'\u2005': ' ', // four-per-em space
			'\u2006': ' ', // six-per-em space
			'\u2007': ' ', // figure space
			'\u2008': ' ', // punctuation space
			'\u2009': ' ', // thin space
			'\u200A': ' ', // hair space
			'\u202F': ' ', // narrow no-break space
			'\u205F': ' ', // medium mathematical space
			'\u3000': ' ', // ideographic space
			// Other common substitutions
			'\u2022': '*', // bullet
			'\u2023': '>',  // triangular bullet
			'\u2043': '-', // hyphen bullet
			'\u00B7': '.', // middle dot
			// Note: '\u2026' (ellipsis) is handled specially in Decode()
		},
	}
}

// Decode normalizes text by expanding ligatures and fixing encoding issues.
func (fd *FontDecoder) Decode(text string) string {
	var result strings.Builder
	result.Grow(len(text))

	for _, r := range text {
		// Check for ligatures first
		if expanded, ok := fd.ligatures[r]; ok {
			result.WriteString(expanded)
			continue
		}

		// Check for simple substitutions
		if sub, ok := fd.substitutions[r]; ok {
			result.WriteRune(sub)
			continue
		}

		// Handle ellipsis specially
		if r == '\u2026' {
			result.WriteString("...")
			continue
		}

		// Pass through normal characters
		result.WriteRune(r)
	}

	return result.String()
}

// DecodeROT3 attempts to decode ROT3-encoded text (common in some PDFs).
// ROT3 shifts each letter by 3 positions in the alphabet.
func (fd *FontDecoder) DecodeROT3(text string) string {
	var result strings.Builder
	result.Grow(len(text))

	for _, r := range text {
		if r >= 'A' && r <= 'Z' {
			// Shift uppercase back by 3
			shifted := 'A' + (r-'A'+23)%26
			result.WriteRune(shifted)
		} else if r >= 'a' && r <= 'z' {
			// Shift lowercase back by 3
			shifted := 'a' + (r-'a'+23)%26
			result.WriteRune(shifted)
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// IsLikelyROT3 checks if text appears to be ROT3 encoded.
func (fd *FontDecoder) IsLikelyROT3(text string) bool {
	// ROT3 encoded English has characteristic patterns
	// Common words like "the" become "wkh", "and" becomes "dqg"
	lower := strings.ToLower(text)
	rot3Patterns := []string{"wkh", "dqg", "iru", "zlwk", "wklv", "iurp"}

	matchCount := 0
	for _, pattern := range rot3Patterns {
		if strings.Contains(lower, pattern) {
			matchCount++
		}
	}

	// If multiple ROT3 patterns found, likely encoded
	return matchCount >= 2
}

// GlyphMapper handles Private Use Area (PUA) character mapping.
// Many PDFs map custom fonts to PUA characters (U+E000-U+F8FF).
type GlyphMapper struct {
	// Maps PUA codepoints to their likely ASCII equivalents
	puaToASCII map[rune]string
}

// NewGlyphMapper creates a GlyphMapper with common PUA mappings.
func NewGlyphMapper() *GlyphMapper {
	gm := &GlyphMapper{
		puaToASCII: make(map[rune]string),
	}

	// Common patterns for Symbol/Wingdings/custom fonts
	// These are heuristic - actual mappings vary by document
	// We detect and learn mappings dynamically

	return gm
}

// LearnFromContext tries to learn PUA mappings from surrounding context.
// This is a heuristic approach that looks for patterns.
func (gm *GlyphMapper) LearnFromContext(texts []pdf.Text) {
	// Look for sequences like "PUA + known_word" or "known_word + PUA"
	// and try to infer what the PUA character represents

	// Common patterns in legal documents
	knownPhrases := []struct {
		pattern string
		expect  rune
	}{
		{"Plaintiff", 'P'},
		{"Defendant", 'D'},
		{"Court", 'C'},
		{"Judge", 'J'},
		{"Section", 'S'},
		{"Article", 'A'},
	}

	for i := 0; i < len(texts)-1; i++ {
		currRunes := []rune(texts[i].S)
		if len(currRunes) != 1 {
			continue
		}

		r := currRunes[0]
		if !gm.isPUA(r) {
			continue
		}

		// Check if next text starts a known phrase
		nextText := texts[i+1].S
		for _, kp := range knownPhrases {
			if strings.HasPrefix(nextText, kp.pattern[1:]) {
				gm.puaToASCII[r] = string(kp.expect)
				break
			}
		}
	}
}

func (gm *GlyphMapper) isPUA(r rune) bool {
	return r >= '\uE000' && r <= '\uF8FF'
}

// Map converts PUA characters to their ASCII equivalents if known.
func (gm *GlyphMapper) Map(text string) string {
	var result strings.Builder
	result.Grow(len(text))

	for _, r := range text {
		if gm.isPUA(r) {
			if mapped, ok := gm.puaToASCII[r]; ok {
				result.WriteString(mapped)
			} else {
				result.WriteRune(' ') // Unknown PUA -> space
			}
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// ToUnicodeMapper handles PDF ToUnicode CMap parsing.
// This is a simplified version - full CMap parsing is complex.
type ToUnicodeMapper struct {
	// CID to Unicode mappings
	mappings map[uint16]rune
}

// NewToUnicodeMapper creates an empty ToUnicode mapper.
func NewToUnicodeMapper() *ToUnicodeMapper {
	return &ToUnicodeMapper{
		mappings: make(map[uint16]rune),
	}
}

// ParseCMap attempts to parse a ToUnicode CMap stream.
// This handles the most common CMap formats.
func (tm *ToUnicodeMapper) ParseCMap(data string) {
	// Look for beginbfchar...endbfchar sections
	// Format: <CID> <Unicode>
	// Example: <0001> <0041>  (CID 1 = 'A')

	lines := strings.Split(data, "\n")
	inBfChar := false
	inBfRange := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(line, "beginbfchar") {
			inBfChar = true
			continue
		}
		if strings.Contains(line, "endbfchar") {
			inBfChar = false
			continue
		}
		if strings.Contains(line, "beginbfrange") {
			inBfRange = true
			continue
		}
		if strings.Contains(line, "endbfrange") {
			inBfRange = false
			continue
		}

		if inBfChar {
			// Parse: <XXXX> <YYYY>
			tm.parseBfCharLine(line)
		}
		if inBfRange {
			// Parse: <XXXX> <YYYY> <ZZZZ>
			tm.parseBfRangeLine(line)
		}
	}
}

func (tm *ToUnicodeMapper) parseBfCharLine(line string) {
	// Format: <0001> <0041>
	parts := strings.Fields(line)
	if len(parts) != 2 {
		return
	}

	cid := tm.parseHexValue(parts[0])
	unicode := tm.parseHexValue(parts[1])

	if cid > 0 && unicode > 0 {
		tm.mappings[uint16(cid)] = rune(unicode)
	}
}

func (tm *ToUnicodeMapper) parseBfRangeLine(line string) {
	// Format: <0001> <0010> <0041>
	// Maps CIDs 0001-0010 to Unicode starting at 0041
	parts := strings.Fields(line)
	if len(parts) != 3 {
		return
	}

	startCID := tm.parseHexValue(parts[0])
	endCID := tm.parseHexValue(parts[1])
	startUnicode := tm.parseHexValue(parts[2])

	if startCID > 0 && endCID >= startCID && startUnicode > 0 {
		for cid := startCID; cid <= endCID; cid++ {
			tm.mappings[uint16(cid)] = rune(startUnicode + (cid - startCID))
		}
	}
}

func (tm *ToUnicodeMapper) parseHexValue(s string) int {
	s = strings.Trim(s, "<>")
	var val int
	for _, c := range s {
		val <<= 4
		if c >= '0' && c <= '9' {
			val |= int(c - '0')
		} else if c >= 'A' && c <= 'F' {
			val |= int(c - 'A' + 10)
		} else if c >= 'a' && c <= 'f' {
			val |= int(c - 'a' + 10)
		}
	}
	return val
}

// Map applies the ToUnicode mapping to text.
func (tm *ToUnicodeMapper) Map(text string) string {
	if len(tm.mappings) == 0 {
		return text
	}

	var result strings.Builder
	result.Grow(len(text))

	for _, r := range text {
		if mapped, ok := tm.mappings[uint16(r)]; ok {
			result.WriteRune(mapped)
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// EnhancedTextCleaner provides more aggressive text cleanup.
type EnhancedTextCleaner struct {
	fontDecoder    *FontDecoder
	glyphMapper    *GlyphMapper
	layoutAnalyzer *LayoutAnalyzer
	textRepair     *TextRepair
}

// NewEnhancedTextCleaner creates a cleaner with all components initialized.
func NewEnhancedTextCleaner() *EnhancedTextCleaner {
	return &EnhancedTextCleaner{
		fontDecoder:    NewFontDecoder(),
		glyphMapper:    NewGlyphMapper(),
		layoutAnalyzer: NewLayoutAnalyzer(),
		textRepair:     NewTextRepair(),
	}
}

// Clean applies all cleaning steps to extracted text.
func (etc *EnhancedTextCleaner) Clean(text string) string {
	// Step 1: Decode ligatures and normalize characters
	text = etc.fontDecoder.Decode(text)

	// Step 2: Handle PUA characters
	text = etc.glyphMapper.Map(text)

	// Step 3: Auto-detect and fix encoding issues (ROT-N, symbol substitution)
	// Uses frequency analysis for more accurate detection than pattern matching
	decoded, fixType := etc.textRepair.AutoDecodeText(text)
	if fixType != "" {
		text = decoded
	} else if etc.fontDecoder.IsLikelyROT3(text) {
		// Fallback to pattern-based ROT3 detection
		text = etc.fontDecoder.DecodeROT3(text)
	}

	// Step 4: Repair fragmented text (single-char word sequences)
	if etc.textRepair.DetectFragmentedText(text) {
		text = etc.textRepair.RepairFragmentedText(text)
	}

	// Step 5: Final cleanup
	text = etc.normalizeWhitespace(text)

	return text
}

func (etc *EnhancedTextCleaner) normalizeWhitespace(text string) string {
	var result strings.Builder
	result.Grow(len(text))

	prevSpace := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			if r == '\n' {
				result.WriteRune('\n')
				prevSpace = true
			} else if !prevSpace {
				result.WriteRune(' ')
				prevSpace = true
			}
		} else {
			result.WriteRune(r)
			prevSpace = false
		}
	}

	return strings.TrimSpace(result.String())
}

// ExtractWithLayout extracts text from a page using full layout analysis.
func (etc *EnhancedTextCleaner) ExtractWithLayout(page pdf.Page) string {
	content := page.Content()
	if len(content.Text) == 0 {
		return ""
	}

	// Learn PUA mappings from context
	etc.glyphMapper.LearnFromContext(content.Text)

	// Extract with layout analysis
	text := etc.layoutAnalyzer.ExtractWithLayout(page)

	// Apply font decoding and cleanup
	text = etc.Clean(text)

	return text
}
