package text

import (
	//	"bytes"
	"regexp"
	"strings"

	"github.com/yourusername/pdfex/internal/document"
	"github.com/yourusername/pdfex/internal/utils"
)

// Extractor handles text extraction from PDF content
type Extractor struct {
	Pages []document.PDFPage
	Fonts map[string]document.PDFFont
}

// NewExtractor creates a new text extractor
func NewExtractor(pages []document.PDFPage, fonts map[string]document.PDFFont) *Extractor {
	return &Extractor{
		Pages: pages,
		Fonts: fonts,
	}
}

// ExtractText extracts text from all pages
func (e *Extractor) ExtractText() []string {
	var results []string

	for i := range e.Pages {
		text := e.extractTextFromPage(&e.Pages[i])
		results = append(results, text)
	}

	return results
}

// extractTextFromPage extracts text from a page using content stream operators
func (e *Extractor) extractTextFromPage(page *document.PDFPage) string {
	// Extract text positions from content stream
	e.extractTextWithPositioning(page)

	// Generate ordered text from positions
	return page.ExtractOrderedText()
}

// extractTextWithPositioning extracts text with positioning information
func (e *Extractor) extractTextWithPositioning(page *document.PDFPage) {
	var textPositions []document.TextPosition
	var textState struct {
		Tm       [6]float64 // Text matrix
		Tlm      [6]float64 // Text line matrix
		FontSize float64
		FontName string
		Leading  float64 // Text leading
	}

	// Initialize matrices to identity
	textState.Tm = [6]float64{1, 0, 0, 1, 0, 0}
	textState.Tlm = [6]float64{1, 0, 0, 1, 0, 0}

	// Find text objects
	textRegex := regexp.MustCompile(`BT(.*?)ET`)
	textMatches := textRegex.FindAll(page.Contents, -1)

	for _, textBlock := range textMatches {
		// Reset text state for each text block
		textState.Tm = [6]float64{1, 0, 0, 1, 0, 0}
		textState.Tlm = [6]float64{1, 0, 0, 1, 0, 0}

		// Find Tm (text matrix) operators: a b c d e f Tm
		tmRegex := regexp.MustCompile(`([-+]?[0-9]*\.?[0-9]+)\s+([-+]?[0-9]*\.?[0-9]+)\s+([-+]?[0-9]*\.?[0-9]+)\s+([-+]?[0-9]*\.?[0-9]+)\s+([-+]?[0-9]*\.?[0-9]+)\s+([-+]?[0-9]*\.?[0-9]+)\s+Tm`)
		tmMatches := tmRegex.FindAllSubmatch(textBlock, -1)

		for _, tmMatch := range tmMatches {
			// Update text matrix
			for i := 0; i < 6; i++ {
				val, err := utils.ParseFloat(string(tmMatch[i+1]))
				if err != nil {
					utils.Logf(utils.LogWarning, "Invalid text matrix value: %v\n", err)
					continue
				}
				textState.Tm[i] = val
				textState.Tlm[i] = val
			}
		}

		// Find Td (text displacement) operators: tx ty Td
		tdRegex := regexp.MustCompile(`([-+]?[0-9]*\.?[0-9]+)\s+([-+]?[0-9]*\.?[0-9]+)\s+Td`)
		tdMatches := tdRegex.FindAllSubmatch(textBlock, -1)

		for _, tdMatch := range tdMatches {
			tx, err := utils.ParseFloat(string(tdMatch[1]))
			if err != nil {
				utils.Logf(utils.LogWarning, "Invalid tx value: %v\n", err)
				continue
			}

			ty, err := utils.ParseFloat(string(tdMatch[2]))
			if err != nil {
				utils.Logf(utils.LogWarning, "Invalid ty value: %v\n", err)
				continue
			}

			// Update text matrix: Tlm = [1 0 0 1 tx ty] × Tlm
			textState.Tlm[4] += tx
			textState.Tlm[5] += ty

			// Set Tm = Tlm
			copy(textState.Tm[:], textState.Tlm[:])
		}

		// Find TD (text displacement with leading) operators: tx ty TD
		tdRegex = regexp.MustCompile(`([-+]?[0-9]*\.?[0-9]+)\s+([-+]?[0-9]*\.?[0-9]+)\s+TD`)
		tdMatches = tdRegex.FindAllSubmatch(textBlock, -1)

		for _, tdMatch := range tdMatches {
			tx, err := utils.ParseFloat(string(tdMatch[1]))
			if err != nil {
				utils.Logf(utils.LogWarning, "Invalid tx value: %v\n", err)
				continue
			}

			ty, err := utils.ParseFloat(string(tdMatch[2]))
			if err != nil {
				utils.Logf(utils.LogWarning, "Invalid ty value: %v\n", err)
				continue
			}

			// Set leading and update text matrix
			textState.Leading = -ty

			// Update text matrix: Tlm = [1 0 0 1 tx ty] × Tlm
			textState.Tlm[4] += tx
			textState.Tlm[5] += ty

			// Set Tm = Tlm
			copy(textState.Tm[:], textState.Tlm[:])
		}

		// Find T* (next line) operator
		tStarRegex := regexp.MustCompile(`T\*`)
		tStarMatches := tStarRegex.FindAll(textBlock, -1)

		for range tStarMatches {
			// Move to next line: Tlm = [1 0 0 1 0 -TL] × Tlm
			textState.Tlm[5] -= textState.Leading

			// Set Tm = Tlm
			copy(textState.Tm[:], textState.Tlm[:])
		}

		// Find font operators: /Font size Tf
		fontRegex := regexp.MustCompile(`/([A-Za-z0-9]+)\s+([-+]?[0-9]*\.?[0-9]+)\s+Tf`)
		fontMatches := fontRegex.FindAllSubmatch(textBlock, -1)

		for _, fontMatch := range fontMatches {
			fontName := string(fontMatch[1])
			fontSize, err := utils.ParseFloat(string(fontMatch[2]))
			if err != nil {
				utils.Logf(utils.LogWarning, "Invalid font size: %v\n", err)
				continue
			}

			textState.FontName = fontName
			textState.FontSize = fontSize
		}

		// Process text showing operators
		tjRegex := regexp.MustCompile(`\((.*?)\)\s+Tj`)
		tjMatches := tjRegex.FindAllSubmatchIndex(textBlock, -1)

		for _, match := range tjMatches {
			textBytes := textBlock[match[2]:match[3]]

			// Get current font
			var currentFont document.PDFFont
			fontKey := "/" + textState.FontName
			if font, ok := e.Fonts[fontKey]; ok {
				currentFont = font
			} else {
				// Use default font if not found
				fontKey = "/DefaultFont"
				if font, ok := e.Fonts[fontKey]; ok {
					currentFont = font
				}
			}

			// Decode text
			text := decodeText(textBytes, currentFont)

			// Create text position entry
			pos := document.TextPosition{
				X:        textState.Tm[4],
				Y:        textState.Tm[5],
				FontSize: textState.FontSize,
				Text:     text,
				FontName: textState.FontName,
			}

			textPositions = append(textPositions, pos)

			// Advance text position based on the width of the text
			// This is simplified - in reality, we'd need to calculate
			// the actual width based on font metrics
			textState.Tm[4] += float64(len(text)) * textState.FontSize * 0.6
		}

		// Handle TJ operator
		tjArrayRegex := regexp.MustCompile(`\[(.*?)\]\s+TJ`)
		tjArrayMatches := tjArrayRegex.FindAllSubmatch(textBlock, -1)

		for _, tjArrayMatch := range tjArrayMatches {
			tjArray := tjArrayMatch[1]

			// Extract string parts from the TJ array
			stringRegex := regexp.MustCompile(`\((.*?)\)`)
			stringMatches := stringRegex.FindAllSubmatchIndex(tjArray, -1)

			for _, match := range stringMatches {
				textBytes := tjArray[match[2]:match[3]]

				// Get current font
				var currentFont document.PDFFont
				fontKey := "/" + textState.FontName
				if font, ok := e.Fonts[fontKey]; ok {
					currentFont = font
				} else {
					// Use default font if not found
					fontKey = "/DefaultFont"
					if font, ok := e.Fonts[fontKey]; ok {
						currentFont = font
					}
				}

				// Decode text
				text := decodeText(textBytes, currentFont)

				// Create text position entry
				pos := document.TextPosition{
					X:        textState.Tm[4],
					Y:        textState.Tm[5],
					FontSize: textState.FontSize,
					Text:     text,
					FontName: textState.FontName,
				}

				textPositions = append(textPositions, pos)

				// Advance text position
				textState.Tm[4] += float64(len(text)) * textState.FontSize * 0.6
			}
		}
	}

	// Sort text positions by reading order
	SortTextPositions(textPositions, page.Width, page.Height)

	page.TextPositions = textPositions
}

// decodeText decodes a byte string using font encoding
func decodeText(textBytes []byte, font document.PDFFont) string {
	var result strings.Builder

	// Handle basic PDF escape sequences
	var i int
	for i < len(textBytes) {
		if textBytes[i] == '\\' && i+1 < len(textBytes) {
			// Handle escape sequence
			switch textBytes[i+1] {
			case '\\':
				result.WriteRune('\\')
			case '(':
				result.WriteRune('(')
			case ')':
				result.WriteRune(')')
			case 'n':
				result.WriteRune('\n')
			case 'r':
				result.WriteRune('\r')
			case 't':
				result.WriteRune('\t')
			case 'b':
				result.WriteRune('\b')
			case 'f':
				result.WriteRune('\f')
			case '0', '1', '2', '3', '4', '5', '6', '7':
				// Octal code (simplified)
				if i+3 < len(textBytes) && isOctal(textBytes[i+2]) && isOctal(textBytes[i+3]) {
					octalStr := string(textBytes[i+1 : i+4])
					val, err := utils.ParseOctal(octalStr)
					if err != nil {
						utils.Logf(utils.LogWarning, "Invalid octal escape: %s\n", octalStr)
						result.WriteRune(rune(textBytes[i+1]))
						i += 2
						continue
					}

					// Map through font encoding if available
					if char, ok := font.CodeToUnicode[int(val)]; ok {
						result.WriteRune(char)
					} else {
						result.WriteRune(rune(val))
					}
					i += 3
				} else {
					// Invalid octal, just output the character
					result.WriteRune(rune(textBytes[i+1]))
				}
			default:
				// Unknown escape, just output the character
				result.WriteRune(rune(textBytes[i+1]))
			}
			i += 2
		} else {
			// Regular character - map through font encoding if available
			if char, ok := font.CodeToUnicode[int(textBytes[i])]; ok {
				result.WriteRune(char)
			} else {
				result.WriteRune(rune(textBytes[i]))
			}
			i++
		}
	}

	return result.String()
}

// isOctal checks if a byte is an octal digit
func isOctal(b byte) bool {
	return b >= '0' && b <= '7'
}

// ExtractTextContent extracts all text content from a document
func ExtractTextContent(doc *document.PDFDocument) (string, error) {
	extractor := NewExtractor(doc.Pages, doc.Fonts)
	pageTexts := extractor.ExtractText()

	var allText strings.Builder
	for i, text := range pageTexts {
		allText.WriteString(text)
		if i < len(pageTexts)-1 {
			allText.WriteString("\n\n")
		}
	}

	return allText.String(), nil
}
