package document

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/yourusername/pdfex/internal/content"
	"github.com/yourusername/pdfex/internal/utils"
)

// PDFObject represents a PDF object
type PDFObject struct {
	ObjectNumber int
	Generation   int
	Content      []byte
	Dictionary   map[string]interface{}
	Stream       []byte
	IsStream     bool
}

// PDFPage represents a page in the PDF
type PDFPage struct {
	PageNumber    int
	Contents      []byte
	Text          string
	ResourcesDict map[string]interface{}
	TextPositions []TextPosition
	Width         float64
	Height        float64
}

// TextPosition represents a text element with position information
type TextPosition struct {
	X        float64 // Position on the page
	Y        float64 // Position on the page
	FontSize float64 // Current font size
	Text     string  // The text at this position
	FontName string  // Name of the font used
}

// PDFFont represents a font in the PDF
type PDFFont struct {
	Name      string
	Subtype   string
	Encoding  string
	ToUnicode []byte // The ToUnicode CMap if available

	// Character code to unicode mapping
	CodeToUnicode map[int]rune
}

// PDFXRefEntry represents an entry in the cross-reference table
type PDFXRefEntry struct {
	Offset     int64
	Generation int
	InUse      bool
}

// Pre-compile regex patterns for object handling
var (
	objHeaderPattern = regexp.MustCompile(`(\d+) (\d+) obj`)
)

// loadObjects loads objects using the xref table
func loadObjects(file *os.File, doc *PDFDocument) error {
	for objNum, xrefEntry := range doc.XRefTable {
		if !xrefEntry.InUse || xrefEntry.Offset == 0 {
			continue
		}

		_, err := file.Seek(xrefEntry.Offset, io.SeekStart)
		if err != nil {
			utils.Logf(utils.LogWarning, "Failed to seek to object %d: %v\n", objNum, err)
			continue
		}

		// Read the object header
		objHeader := make([]byte, 50) // Should be enough for the header
		n, err := file.Read(objHeader)
		if err != nil {
			utils.Logf(utils.LogWarning, "Failed to read object %d header: %v\n", objNum, err)
			continue
		}

		// Check object header format
		headerMatches := objHeaderPattern.FindSubmatch(objHeader[:n])

		if len(headerMatches) < 3 {
			utils.Logf(utils.LogWarning, "Invalid object %d header format\n", objNum)
			continue
		}

		matchedObjNum, err := strconv.Atoi(string(headerMatches[1]))
		if err != nil {
			utils.Logf(utils.LogWarning, "Invalid object number: %v\n", err)
			continue
		}

		generation, err := strconv.Atoi(string(headerMatches[2]))
		if err != nil {
			utils.Logf(utils.LogWarning, "Invalid generation number: %v\n", err)
			continue
		}

		if matchedObjNum != objNum || generation != xrefEntry.Generation {
			utils.Logf(utils.LogWarning, "Object number mismatch: expected %d gen %d, got %d gen %d\n",
				objNum, xrefEntry.Generation, matchedObjNum, generation)
			continue
		}

		// Read the object content
		var contentBuffer bytes.Buffer

		_, err = file.Seek(xrefEntry.Offset, io.SeekStart)
		if err != nil {
			utils.Logf(utils.LogWarning, "Failed to seek to object %d: %v\n", objNum, err)
			continue
		}

		inObject := false
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()

			if !inObject {
				if strings.Contains(line, " obj") {
					inObject = true
					// Add everything after "obj"
					parts := strings.SplitN(line, "obj", 2)
					if len(parts) > 1 {
						contentBuffer.WriteString(parts[1])
					}
				}
			} else {
				contentBuffer.WriteString(line)
				contentBuffer.WriteString("\n")

				if strings.Contains(line, "endobj") {
					break
				}
			}
		}

		if err := scanner.Err(); err != nil {
			utils.Logf(utils.LogWarning, "Error scanning object %d: %v\n", objNum, err)
			continue
		}

		contentBytes := contentBuffer.Bytes()

		// Trim "endobj" from the end
		if idx := bytes.Index(contentBytes, []byte("endobj")); idx != -1 {
			contentBytes = contentBytes[:idx]
		}

		obj := PDFObject{
			ObjectNumber: objNum,
			Generation:   generation,
			Content:      contentBytes,
			Dictionary:   make(map[string]interface{}),
		}

		// Parse dictionary
		if bytes.HasPrefix(contentBytes, []byte("<<")) && bytes.Contains(contentBytes, []byte(">>")) {
			dictEnd := bytes.Index(contentBytes, []byte(">>"))
			dictBytes := contentBytes[2:dictEnd]
			err := utils.ParseDictionary(dictBytes, obj.Dictionary)
			if err != nil {
				utils.Logf(utils.LogWarning, "Error parsing dictionary for object %d: %v\n", objNum, err)
			}
		}

		// Check for stream
		if bytes.Contains(contentBytes, []byte("stream")) && bytes.Contains(contentBytes, []byte("endstream")) {
			streamStart := bytes.Index(contentBytes, []byte("stream"))
			streamStart += 6 // length of "stream"
			if contentBytes[streamStart] == '\r' && contentBytes[streamStart+1] == '\n' {
				streamStart += 2
			} else if contentBytes[streamStart] == '\n' {
				streamStart += 1
			}

			streamEnd := bytes.Index(contentBytes, []byte("endstream"))
			if streamStart < streamEnd {
				obj.Stream = contentBytes[streamStart:streamEnd]
				obj.IsStream = true

				// Check if the stream has a filter
				if filter, ok := obj.Dictionary["Filter"]; ok {
					// Get decode parameters if any
					var decodeParms map[string]interface{}
					if parms, ok := obj.Dictionary["DecodeParms"]; ok {
						switch p := parms.(type) {
						case string:
							if strings.HasPrefix(p, "<<") {
								decodeParms = make(map[string]interface{})
								parmBytes := []byte(p)[2 : len(p)-2]
								err := utils.ParseDictionary(parmBytes, decodeParms)
								if err != nil {
									utils.Logf(utils.LogWarning, "Error parsing DecodeParms for object %d: %v\n", objNum, err)
								}
							}
						case map[string]interface{}:
							decodeParms = p
						}
					}

					// Decompress the stream based on filter type
					decompressed, err := content.DecompressStream(obj.Stream, filter.(string), decodeParms)
					if err == nil {
						obj.Stream = decompressed
					} else {
						utils.Logf(utils.LogWarning, "Failed to decompress stream for object %d: %v\n", objNum, err)
					}
				}
			}
		}

		doc.Objects[objNum] = obj
	}

	return nil
}

// ExtractOrderedText generates text from ordered positions
func (page *PDFPage) ExtractOrderedText() string {
	var text strings.Builder
	var lastY, lastX float64
	const lineThreshold = 5.0  // Threshold to detect line breaks
	const spaceThreshold = 2.0 // Threshold to detect spaces

	for i, pos := range page.TextPositions {
		if i > 0 {
			// Check for new line
			yDiff := pos.Y - lastY
			if yDiff < -lineThreshold || yDiff > lineThreshold {
				text.WriteString("\n")
			} else if pos.X-lastX > pos.FontSize*spaceThreshold {
				// Space between words detected
				text.WriteString(" ")
			}
		}

		text.WriteString(pos.Text)
		lastY = pos.Y
		lastX = pos.X + float64(len(pos.Text))*pos.FontSize*0.6
	}

	return text.String()
}

// GetObject returns an object by object number
func (doc *PDFDocument) GetObject(objNum int) (PDFObject, bool) {
	obj, ok := doc.Objects[objNum]
	return obj, ok
}

// GetPage returns a page by page number (1-based)
func (doc *PDFDocument) GetPage(pageNum int) (PDFPage, bool) {
	if pageNum < 1 || pageNum > len(doc.Pages) {
		return PDFPage{}, false
	}
	return doc.Pages[pageNum-1], true
}
