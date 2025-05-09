package text

import (
	"math"
	"sort"

	"github.com/yourusername/pdfex/internal/document"
)

// SortTextPositions sorts text positions in reading order
func SortTextPositions(positions []document.TextPosition, pageWidth, pageHeight float64) {
	// Simplified approach: sort by rows, then by columns within each row
	const lineHeightFactor = 1.5 // Multiplier for line height threshold

	// Find average font size to determine line height
	var totalFontSize float64
	for _, pos := range positions {
		totalFontSize += pos.FontSize
	}

	// Default line height if no positions
	lineHeight := 14.0
	if len(positions) > 0 {
		avgFontSize := totalFontSize / float64(len(positions))
		lineHeight = avgFontSize * lineHeightFactor
	}

	// Group by rows based on Y position
	rows := make(map[int][]document.TextPosition)
	for _, pos := range positions {
		rowKey := int(pos.Y / lineHeight)
		rows[rowKey] = append(rows[rowKey], pos)
	}

	// Sort each row by X position
	for rowKey, rowPositions := range rows {
		sort.Slice(rowPositions, func(i, j int) bool {
			return rowPositions[i].X < rowPositions[j].X
		})
		rows[rowKey] = rowPositions
	}

	// Get sorted row keys
	var rowKeys []int
	for k := range rows {
		rowKeys = append(rowKeys, k)
	}

	// Sort in descending order (assuming Y increases downward)
	sort.Sort(sort.Reverse(sort.IntSlice(rowKeys)))

	// Rebuild the positions array in reading order
	positions = positions[:0]
	for _, rowKey := range rowKeys {
		positions = append(positions, rows[rowKey]...)
	}
}

// DetectColumns attempts to identify columns in text positions
func DetectColumns(positions []document.TextPosition, pageWidth float64) [][]document.TextPosition {
	// Simple column detection based on X positions
	if len(positions) == 0 {
		return nil
	}

	// Calculate average character width
	var totalFontSize float64
	for _, pos := range positions {
		totalFontSize += pos.FontSize
	}
	avgFontSize := totalFontSize / float64(len(positions))
	charWidth := avgFontSize * 0.6

	// Group by X position
	xPositions := make(map[int]int)
	for _, pos := range positions {
		key := int(pos.X / charWidth)
		xPositions[key]++
	}

	// Find most frequent X positions (column starts)
	type xFreq struct {
		x    int
		freq int
	}
	var freqs []xFreq
	for x, freq := range xPositions {
		freqs = append(freqs, xFreq{x, freq})
	}

	sort.Slice(freqs, func(i, j int) bool {
		return freqs[i].freq > freqs[j].freq
	})

	// Take top X positions as column starts (use at most 5 columns)
	var columnStarts []int
	maxColumns := 5
	if len(freqs) < maxColumns {
		maxColumns = len(freqs)
	}

	for i := 0; i < maxColumns; i++ {
		if freqs[i].freq > len(positions)/20 { // Threshold: at least 5% of positions
			columnStarts = append(columnStarts, freqs[i].x)
		}
	}

	// Sort column starts
	sort.Ints(columnStarts)

	// Group text positions by column
	columns := make([][]document.TextPosition, len(columnStarts))
	for _, pos := range positions {
		col := findColumn(pos.X/charWidth, columnStarts)
		columns[col] = append(columns[col], pos)
	}

	// Sort each column by Y position
	for i := range columns {
		sort.Slice(columns[i], func(a, b int) bool {
			return columns[i][a].Y > columns[i][b].Y
		})
	}

	return columns
}

// findColumn finds which column a text position belongs to
func findColumn(x float64, columnStarts []int) int {
	for i, start := range columnStarts {
		if float64(start) > x {
			if i == 0 {
				return 0
			}
			return i
		}
	}
	return len(columnStarts) - 1
}

// MergeClosePositions merges text positions that are close together
func MergeClosePositions(positions []document.TextPosition) []document.TextPosition {
	if len(positions) <= 1 {
		return positions
	}

	var result []document.TextPosition
	currentPos := positions[0]

	for i := 1; i < len(positions); i++ {
		// If positions are on the same line and close together
		if math.Abs(positions[i].Y-currentPos.Y) < 2 &&
			positions[i].X-currentPos.X < currentPos.FontSize*positions[i].FontSize*0.8 {
			// Merge them
			currentPos.Text += positions[i].Text
		} else {
			// Add current to result and start new position
			result = append(result, currentPos)
			currentPos = positions[i]
		}
	}

	// Add the last position
	result = append(result, currentPos)

	return result
}

// DetectParagraphs groups text positions into paragraphs
func DetectParagraphs(positions []document.TextPosition) [][]document.TextPosition {
	var paragraphs [][]document.TextPosition
	if len(positions) == 0 {
		return paragraphs
	}

	// Calculate average line height
	var totalFontSize float64
	for _, pos := range positions {
		totalFontSize += pos.FontSize
	}
	avgFontSize := totalFontSize / float64(len(positions))
	lineHeight := avgFontSize * 1.2
	paragraphBreak := lineHeight * 1.5

	var currentParagraph []document.TextPosition
	var lastY float64

	// First position is always start of first paragraph
	currentParagraph = append(currentParagraph, positions[0])
	lastY = positions[0].Y

	for i := 1; i < len(positions); i++ {
		// Check if this is a new paragraph
		yDiff := math.Abs(positions[i].Y - lastY)

		if yDiff > paragraphBreak || (yDiff > 0 && positions[i].X < positions[i-1].X) {
			// End of paragraph, start new one
			if len(currentParagraph) > 0 {
				paragraphs = append(paragraphs, currentParagraph)
				currentParagraph = []document.TextPosition{}
			}
		}

		currentParagraph = append(currentParagraph, positions[i])
		lastY = positions[i].Y
	}

	// Add the last paragraph
	if len(currentParagraph) > 0 {
		paragraphs = append(paragraphs, currentParagraph)
	}

	return paragraphs
}

// CalculateTextBounds calculates the bounding box of text positions
func CalculateTextBounds(positions []document.TextPosition) (minX, minY, maxX, maxY float64) {
	if len(positions) == 0 {
		return 0, 0, 0, 0
	}

	minX = positions[0].X
	minY = positions[0].Y
	maxX = positions[0].X
	maxY = positions[0].Y

	for _, pos := range positions {
		// Estimate text width based on font size
		textWidth := float64(len(pos.Text)) * pos.FontSize * 0.6

		if pos.X < minX {
			minX = pos.X
		}
		if pos.Y < minY {
			minY = pos.Y
		}
		if pos.X+textWidth > maxX {
			maxX = pos.X + textWidth
		}
		if pos.Y+pos.FontSize > maxY {
			maxY = pos.Y + pos.FontSize
		}
	}

	return minX, minY, maxX, maxY
}

// IsLikelyHeader checks if a text position is likely a header
func IsLikelyHeader(pos document.TextPosition, avgFontSize float64) bool {
	// Headers are typically larger than surrounding text
	return pos.FontSize > avgFontSize*1.2
}

// DetectTextBlocks groups text positions into blocks based on layout
func DetectTextBlocks(positions []document.TextPosition, pageWidth, pageHeight float64) [][]document.TextPosition {
	// First sort positions
	SortTextPositions(positions, pageWidth, pageHeight)

	// Find paragraphs
	paragraphs := DetectParagraphs(positions)

	// Group paragraphs into blocks
	var blocks [][]document.TextPosition

	if len(paragraphs) == 0 {
		return blocks
	}

	var currentBlock []document.TextPosition

	// Start with first paragraph
	currentBlock = append(currentBlock, paragraphs[0]...)

	for i := 1; i < len(paragraphs); i++ {
		// Calculate bounds of current block and this paragraph
		// blockMinX, blockMinY, blockMaxX, blockMaxY := CalculateTextBounds(currentBlock)
		blockMinX, _, blockMaxX, blockMaxY := CalculateTextBounds(currentBlock)
		paraMinX, paraMinY, paraMaxX, _ := CalculateTextBounds(paragraphs[i])

		// Check if paragraph is part of the same block
		// 1. Similar horizontal position
		horizontalOverlap := math.Max(0, math.Min(blockMaxX, paraMaxX)-math.Max(blockMinX, paraMinX))
		horizontalOverlapRatio := horizontalOverlap / math.Min(blockMaxX-blockMinX, paraMaxX-paraMinX)

		// 2. Vertical distance
		verticalDistance := math.Abs(blockMaxY - paraMinY)

		// Calculate average font size
		var totalFontSize float64
		for _, pos := range positions {
			totalFontSize += pos.FontSize
		}
		avgFontSize := totalFontSize / float64(len(positions))

		// If paragraph is close to block and has similar horizontal position, add to current block
		if horizontalOverlapRatio > 0.7 && verticalDistance < avgFontSize*2 {
			currentBlock = append(currentBlock, paragraphs[i]...)
		} else {
			// Start a new block
			blocks = append(blocks, currentBlock)
			currentBlock = paragraphs[i]
		}
	}

	// Add the last block
	if len(currentBlock) > 0 {
		blocks = append(blocks, currentBlock)
	}

	return blocks
}
