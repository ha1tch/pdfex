package text

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/yourusername/pdfex/internal/document"
	"github.com/yourusername/pdfex/internal/utils"
)

// FontProcessor handles font processing and character mapping
type FontProcessor struct {
	Fonts map[string]document.PDFFont
}

// NewFontProcessor creates a new font processor
func NewFontProcessor() *FontProcessor {
	return &FontProcessor{
		Fonts: make(map[string]document.PDFFont),
	}
}

// ProcessFonts extracts fonts from a document's objects
func (fp *FontProcessor) ProcessFonts(doc *document.PDFDocument) {
	// Scan through all objects to find font dictionaries
	for objNum, obj := range doc.Objects {
		if objType, ok := obj.Dictionary["Type"]; ok && objType == "/Font" {
			font := fp.processFont(objNum, obj, doc)
			fp.Fonts[font.Name] = font
		}
	}

	// Scan pages for font resources
	for _, page := range doc.Pages {
		if fontsRef, ok := page.ResourcesDict["Font"]; ok {
			switch fonts := fontsRef.(type) {
			case string:
				if strings.HasPrefix(fonts, "<<") {
					// Inline dictionary
					fontsDict := make(map[string]interface{})
					dictBytes := []byte(fonts)[2 : len(fonts)-2]
					err := utils.ParseDictionary(dictBytes, fontsDict)
					if err != nil {
						utils.Logf(utils.LogWarning, "Error parsing fonts dictionary: %v\n", err)
						continue
					}

					// Process each font in the dictionary
					for fontName, fontRefValue := range fontsDict {
						fp.processNamedFont(fontName, fontRefValue.(string), doc)
					}
				} else {
					// Reference to font dictionary
					fontsObjNum, err := utils.ExtractReference(fonts)
					if err != nil {
						utils.Logf(utils.LogWarning, "Invalid fonts reference: %v\n", err)
						continue
					}
					if fontsObj, ok := doc.Objects[fontsObjNum]; ok {
						// Process each font in the dictionary
						for fontName, fontRefValue := range fontsObj.Dictionary {
							if strings.HasPrefix(fontName, "F") {
								fp.processNamedFont(fontName, fontRefValue.(string), doc)
							}
						}
					}
				}
			case map[string]interface{}:
				// Direct dictionary
				for fontName, fontRefValue := range fonts {
					if refStr, ok := fontRefValue.(string); ok {
						fp.processNamedFont(fontName, refStr, doc)
					}
				}
			}
		}
	}

	// Create default font if no fonts were found
	if len(fp.Fonts) == 0 {
		fp.createDefaultFont()
	}

	// Ensure a default font is always available
	if _, ok := fp.Fonts["/DefaultFont"]; !ok {
		fp.createDefaultFont()
	}
}

// processNamedFont processes a named font reference
func (fp *FontProcessor) processNamedFont(fontName string, fontRef string, doc *document.PDFDocument) {
	fontObjNum, err := utils.ExtractReference(fontRef)
	if err != nil {
		utils.Logf(utils.LogWarning, "Invalid font reference %s: %v\n", fontRef, err)
		return
	}

	fontObj, ok := doc.Objects[fontObjNum]
	if !ok {
		utils.Logf(utils.LogWarning, "Font object %d not found\n", fontObjNum)
		return
	}

	font := fp.processFont(fontObjNum, fontObj, doc)

	// Override the name with the resource name
	font.Name = fontName
	fp.Fonts["/"+fontName] = font
}

// processFont processes a font object and creates a PDFFont
func (fp *FontProcessor) processFont(objNum int, obj document.PDFObject, doc *document.PDFDocument) document.PDFFont {
	font := document.PDFFont{
		Name:          strconv.Itoa(objNum), // Default name based on object number
		CodeToUnicode: make(map[int]rune),
	}

	// Extract font properties
	if subtype, ok := obj.Dictionary["Subtype"]; ok {
		font.Subtype = subtype.(string)
	}

	if encoding, ok := obj.Dictionary["Encoding"]; ok {
		// Use type assertion to convert to string
		encodingStr, ok := encoding.(string)
		if ok {
			font.Encoding = encodingStr
			// Handle standard encodings
			if encodingStr == "/WinAnsiEncoding" {
				loadWinAnsiEncoding(&font)
			} else if encodingStr == "/MacRomanEncoding" {
				loadMacRomanEncoding(&font)
			} else if strings.HasPrefix(encodingStr, "/Identity") {
				loadIdentityEncoding(&font)
			}
		} else {
			utils.Logf(utils.LogWarning, "Font encoding is not a string: %v\n", encoding)
		}
	}

	// Check for ToUnicode CMap
	if toUnicodeRef, ok := obj.Dictionary["ToUnicode"]; ok {
		toUnicodeObjNum, err := utils.ExtractReference(toUnicodeRef.(string))
		if err != nil {
			utils.Logf(utils.LogWarning, "Invalid ToUnicode reference: %v\n", err)
		} else if toUnicodeObj, ok := doc.Objects[toUnicodeObjNum]; ok && toUnicodeObj.IsStream {
			font.ToUnicode = toUnicodeObj.Stream
			// Parse the ToUnicode CMap
			parseCMap(font.ToUnicode, &font)
		}
	}

	return font
}

// createDefaultFont creates a default font mapping
func (fp *FontProcessor) createDefaultFont() {
	defaultFont := document.PDFFont{
		Name:          "DefaultFont",
		CodeToUnicode: make(map[int]rune),
	}

	// Add basic ASCII mapping
	for i := 0; i < 128; i++ {
		defaultFont.CodeToUnicode[i] = rune(i)
	}

	// Add common non-ASCII characters
	for i := 128; i < 256; i++ {
		defaultFont.CodeToUnicode[i] = rune(i)
	}

	fp.Fonts["/DefaultFont"] = defaultFont
}

// loadWinAnsiEncoding loads the WinAnsi encoding into a font
func loadWinAnsiEncoding(font *document.PDFFont) {
	// WinAnsiEncoding is similar to ISO-8859-1
	for i := 0; i < 256; i++ {
		// This is simplified - actual implementation would need full mapping table
		font.CodeToUnicode[i] = rune(i)
	}

	// Overrides for common characters that differ
	winAnsiOverrides := map[int]rune{
		128: '\u20AC', // Euro sign
		130: '\u201A', // Single low-9 quotation mark
		131: '\u0192', // Latin small letter f with hook
		132: '\u201E', // Double low-9 quotation mark
		133: '\u2026', // Horizontal ellipsis
		134: '\u2020', // Dagger
		135: '\u2021', // Double dagger
		136: '\u02C6', // Modifier letter circumflex accent
		137: '\u2030', // Per mille sign
		138: '\u0160', // Latin capital letter S with caron
		139: '\u2039', // Single left-pointing angle quotation mark
		140: '\u0152', // Latin capital ligature OE
		142: '\u017D', // Latin capital letter Z with caron
		145: '\u2018', // Left single quotation mark
		146: '\u2019', // Right single quotation mark
		147: '\u201C', // Left double quotation mark
		148: '\u201D', // Right double quotation mark
		149: '\u2022', // Bullet
		150: '\u2013', // En dash
		151: '\u2014', // Em dash
		152: '\u02DC', // Small tilde
		153: '\u2122', // Trade mark sign
		154: '\u0161', // Latin small letter s with caron
		155: '\u203A', // Single right-pointing angle quotation mark
		156: '\u0153', // Latin small ligature oe
		158: '\u017E', // Latin small letter z with caron
		159: '\u0178', // Latin capital letter Y with diaeresis
	}

	for code, char := range winAnsiOverrides {
		font.CodeToUnicode[code] = char
	}
}

// loadMacRomanEncoding loads the MacRoman encoding into a font
func loadMacRomanEncoding(font *document.PDFFont) {
	// Simplified MacRoman encoding
	for i := 0; i < 128; i++ {
		// ASCII range is the same
		font.CodeToUnicode[i] = rune(i)
	}

	// MacRoman special characters (simplified)
	macRomanHighMap := map[int]rune{
		128: '\u00C4', // Latin capital letter A with diaeresis
		129: '\u00C5', // Latin capital letter A with ring above
		130: '\u00C7', // Latin capital letter C with cedilla
		131: '\u00C9', // Latin capital letter E with acute
		132: '\u00D1', // Latin capital letter N with tilde
		133: '\u00D6', // Latin capital letter O with diaeresis
		134: '\u00DC', // Latin capital letter U with diaeresis
		135: '\u00E1', // Latin small letter a with acute
		136: '\u00E0', // Latin small letter a with grave
		137: '\u00E2', // Latin small letter a with circumflex
		138: '\u00E4', // Latin small letter a with diaeresis
		139: '\u00E3', // Latin small letter a with tilde
		140: '\u00E5', // Latin small letter a with ring above
		141: '\u00E7', // Latin small letter c with cedilla
		142: '\u00E9', // Latin small letter e with acute
		143: '\u00E8', // Latin small letter e with grave
		144: '\u00EA', // Latin small letter e with circumflex
		145: '\u00EB', // Latin small letter e with diaeresis
		146: '\u00ED', // Latin small letter i with acute
		147: '\u00EC', // Latin small letter i with grave
		148: '\u00EE', // Latin small letter i with circumflex
		149: '\u00EF', // Latin small letter i with diaeresis
		150: '\u00F1', // Latin small letter n with tilde
		151: '\u00F3', // Latin small letter o with acute
		152: '\u00F2', // Latin small letter o with grave
		153: '\u00F4', // Latin small letter o with circumflex
		154: '\u00F6', // Latin small letter o with diaeresis
		155: '\u00F5', // Latin small letter o with tilde
		156: '\u00FA', // Latin small letter u with acute
		157: '\u00F9', // Latin small letter u with grave
		158: '\u00FB', // Latin small letter u with circumflex
		159: '\u00FC', // Latin small letter u with diaeresis
		160: '\u2020', // Dagger
		161: '\u00B0', // Degree sign
		162: '\u00A2', // Cent sign
		163: '\u00A3', // Pound sign
		164: '\u00A7', // Section sign
		165: '\u2022', // Bullet
		166: '\u00B6', // Pilcrow sign
		167: '\u00DF', // Latin small letter sharp s
		168: '\u00AE', // Registered sign
		169: '\u00A9', // Copyright sign
		170: '\u2122', // Trade mark sign
		171: '\u00B4', // Acute accent
		172: '\u00A8', // Diaeresis
		173: '\u2260', // Not equal to
		174: '\u00C6', // Latin capital letter AE
		175: '\u00D8', // Latin capital letter O with stroke
		176: '\u221E', // Infinity
		177: '\u00B1', // Plus-minus sign
		178: '\u2264', // Less-than or equal to
		179: '\u2265', // Greater-than or equal to
		180: '\u00A5', // Yen sign
		181: '\u00B5', // Micro sign
		182: '\u2202', // Partial differential
		183: '\u2211', // N-ary summation
		184: '\u220F', // N-ary product
		185: '\u03C0', // Greek small letter pi
		186: '\u222B', // Integral
		187: '\u00AA', // Feminine ordinal indicator
		188: '\u00BA', // Masculine ordinal indicator
		189: '\u03A9', // Greek capital letter Omega
		190: '\u00E6', // Latin small letter ae
		191: '\u00F8', // Latin small letter o with stroke
		192: '\u00BF', // Inverted question mark
		193: '\u00A1', // Inverted exclamation mark
		194: '\u00AC', // Not sign
		195: '\u221A', // Square root
		196: '\u0192', // Latin small letter f with hook
		197: '\u2248', // Almost equal to
		198: '\u2206', // Increment
		199: '\u00AB', // Left-pointing double angle quotation mark
		200: '\u00BB', // Right-pointing double angle quotation mark
		201: '\u2026', // Horizontal ellipsis
		202: '\u00A0', // No-break space
		203: '\u00C0', // Latin capital letter A with grave
		204: '\u00C3', // Latin capital letter A with tilde
		205: '\u00D5', // Latin capital letter O with tilde
		206: '\u0152', // Latin capital ligature OE
		207: '\u0153', // Latin small ligature oe
		208: '\u2013', // En dash
		209: '\u2014', // Em dash
		210: '\u201C', // Left double quotation mark
		211: '\u201D', // Right double quotation mark
		212: '\u2018', // Left single quotation mark
		213: '\u2019', // Right single quotation mark
		214: '\u00F7', // Division sign
		215: '\u25CA', // Lozenge
		216: '\u00FF', // Latin small letter y with diaeresis
		217: '\u0178', // Latin capital letter Y with diaeresis
		218: '\u2044', // Fraction slash
		219: '\u20AC', // Euro sign
		220: '\u2039', // Single left-pointing angle quotation mark
		221: '\u203A', // Single right-pointing angle quotation mark
		222: '\uFB01', // Latin small ligature fi
		223: '\uFB02', // Latin small ligature fl
		224: '\u2021', // Double dagger
		225: '\u00B7', // Middle dot
		226: '\u201A', // Single low-9 quotation mark
		227: '\u201E', // Double low-9 quotation mark
		228: '\u2030', // Per mille sign
		229: '\u00C2', // Latin capital letter A with circumflex
		230: '\u00CA', // Latin capital letter E with circumflex
		231: '\u00C1', // Latin capital letter A with acute
		232: '\u00CB', // Latin capital letter E with diaeresis
		233: '\u00C8', // Latin capital letter E with grave
		234: '\u00CD', // Latin capital letter I with acute
		235: '\u00CE', // Latin capital letter I with circumflex
		236: '\u00CF', // Latin capital letter I with diaeresis
		237: '\u00CC', // Latin capital letter I with grave
		238: '\u00D3', // Latin capital letter O with acute
		239: '\u00D4', // Latin capital letter O with circumflex
		240: ' ',      // Space (unused in Mac Roman)
		241: '\u00D2', // Latin capital letter O with grave
		242: '\u00DA', // Latin capital letter U with acute
		243: '\u00DB', // Latin capital letter U with circumflex
		244: '\u00D9', // Latin capital letter U with grave
		245: '\u0131', // Latin small letter dotless i
		246: '\u02C6', // Modifier letter circumflex accent
		247: '\u02DC', // Small tilde
		248: '\u00AF', // Macron
		249: '\u02D8', // Breve
		250: '\u02D9', // Dot above
		251: '\u02DA', // Ring above
		252: '\u00B8', // Cedilla
		253: '\u02DD', // Double acute accent
		254: '\u02DB', // Ogonek
		255: '\u02C7', // Caron
	}

	for code, char := range macRomanHighMap {
		font.CodeToUnicode[code] = char
	}
}

// loadIdentityEncoding loads the Identity encoding into a font
func loadIdentityEncoding(font *document.PDFFont) {
	// Identity encoding is a direct mapping
	// This is primarily used with CID fonts
	// Without a ToUnicode map, we can't do much,
	// but we'll set up basic ASCII for simple cases
	for i := 0; i < 128; i++ {
		font.CodeToUnicode[i] = rune(i)
	}
}

// parseCMap parses a CMap to extract character mappings
func parseCMap(cmapData []byte, font *document.PDFFont) {
	// Look for beginbfchar sections which define character mappings
	bfcharRegex := regexp.MustCompile(`beginbfchar\s+(.*?)\s+endbfchar`)
	matches := bfcharRegex.FindAllSubmatch(cmapData, -1)

	for _, match := range matches {
		mappings := match[1]
		// Extract character mappings (hex code to Unicode)
		mapRegex := regexp.MustCompile(`<([0-9A-F]+)>\s+<([0-9A-F]+)>`)
		mapMatches := mapRegex.FindAllSubmatch(mappings, -1)

		for _, mapMatch := range mapMatches {
			srcHex := string(mapMatch[1])
			destHex := string(mapMatch[2])

			// Convert hex strings to integers
			src, err := strconv.ParseInt(srcHex, 16, 32)
			if err != nil {
				utils.Logf(utils.LogWarning, "Invalid source hex in CMap: %s\n", srcHex)
				continue
			}

			dest, err := strconv.ParseInt(destHex, 16, 32)
			if err != nil {
				utils.Logf(utils.LogWarning, "Invalid destination hex in CMap: %s\n", destHex)
				continue
			}

			font.CodeToUnicode[int(src)] = rune(dest)
		}
	}

	// Look for beginbfrange sections which define character ranges
	bfrangeRegex := regexp.MustCompile(`beginbfrange\s+(.*?)\s+endbfrange`)
	rangeMatches := bfrangeRegex.FindAllSubmatch(cmapData, -1)

	for _, match := range rangeMatches {
		ranges := match[1]
		// Extract range mappings
		rangeRegex := regexp.MustCompile(`<([0-9A-F]+)>\s+<([0-9A-F]+)>\s+<([0-9A-F]+)>`)
		rangeEntries := rangeRegex.FindAllSubmatch(ranges, -1)

		for _, rangeEntry := range rangeEntries {
			startHex := string(rangeEntry[1])
			endHex := string(rangeEntry[2])
			destStartHex := string(rangeEntry[3])

			// Convert hex strings to integers
			start, err := strconv.ParseInt(startHex, 16, 32)
			if err != nil {
				utils.Logf(utils.LogWarning, "Invalid start hex in CMap range: %s\n", startHex)
				continue
			}

			end, err := strconv.ParseInt(endHex, 16, 32)
			if err != nil {
				utils.Logf(utils.LogWarning, "Invalid end hex in CMap range: %s\n", endHex)
				continue
			}

			destStart, err := strconv.ParseInt(destStartHex, 16, 32)
			if err != nil {
				utils.Logf(utils.LogWarning, "Invalid destination start hex in CMap range: %s\n", destStartHex)
				continue
			}

			// Add mappings for the entire range
			for i := int(start); i <= int(end); i++ {
				// Convert int64 to int for indexing
				offset := int(i) - int(start)
				font.CodeToUnicode[i] = rune(destStart + int64(offset))
			}
		}
	}
}

// GetFonts returns all fonts processed by the processor
func (fp *FontProcessor) GetFonts() map[string]document.PDFFont {
	return fp.Fonts
}

// GetFont returns a specific font by name
func (fp *FontProcessor) GetFont(name string) (document.PDFFont, bool) {
	font, ok := fp.Fonts[name]
	return font, ok
}

// ProcessFontInDocument processes all fonts in a document
func ProcessFontInDocument(doc *document.PDFDocument) {
	fp := NewFontProcessor()
	fp.ProcessFonts(doc)

	// Update the document's fonts
	doc.Fonts = fp.GetFonts()
}

// GetDefaultFont returns a default font for fallback
func GetDefaultFont() document.PDFFont {
	defaultFont := document.PDFFont{
		Name:          "DefaultFont",
		CodeToUnicode: make(map[int]rune),
	}

	// Add basic ASCII mapping
	for i := 0; i < 256; i++ {
		defaultFont.CodeToUnicode[i] = rune(i)
	}

	return defaultFont
}
