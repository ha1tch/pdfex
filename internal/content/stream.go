package content

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/yourusername/pdfex/internal/utils"
)

// StreamProcessor handles PDF stream processing
type StreamProcessor struct {
	ObjectNumber int
	Stream       []byte
	Dictionary   map[string]interface{}
	Filtered     bool
}

// NewStreamProcessor creates a new stream processor
func NewStreamProcessor(objNum int, stream []byte, dict map[string]interface{}) *StreamProcessor {
	return &StreamProcessor{
		ObjectNumber: objNum,
		Stream:       stream,
		Dictionary:   dict,
		Filtered:     false,
	}
}

// Process processes the stream with all filters
func (sp *StreamProcessor) Process() error {
	// Check if stream has a filter
	if filter, ok := sp.Dictionary["Filter"]; ok {
		// Get decode parameters if any
		var decodeParms map[string]interface{}
		if parms, ok := sp.Dictionary["DecodeParms"]; ok {
			switch p := parms.(type) {
			case string:
				if strings.HasPrefix(p, "<<") {
					decodeParms = make(map[string]interface{})
					parmBytes := []byte(p)[2 : len(p)-2]
					err := utils.ParseDictionary(parmBytes, decodeParms)
					if err != nil {
						return fmt.Errorf("error parsing DecodeParms: %v", err)
					}
				}
			case map[string]interface{}:
				decodeParms = p
			}
		}

		// Decompress the stream based on filter type
		decompressed, err := DecompressStream(sp.Stream, filter.(string), decodeParms)
		if err != nil {
			return fmt.Errorf("failed to decompress stream: %v", err)
		}

		sp.Stream = decompressed
		sp.Filtered = true
	}

	return nil
}

// GetLength returns the stream length
func (sp *StreamProcessor) GetLength() (int, error) {
	// Check for Length key in dictionary
	if lengthObj, ok := sp.Dictionary["Length"]; ok {
		switch l := lengthObj.(type) {
		case string:
			if strings.Contains(l, " R") {
				// Length is an indirect reference
				// Here we would normally look up the object, but for simplicity
				// we'll just return the actual length
				return len(sp.Stream), nil
			}
			// Length is a direct value
			length, err := strconv.Atoi(l)
			if err != nil {
				return 0, fmt.Errorf("invalid Length value: %v", err)
			}
			return length, nil
		case int:
			return l, nil
		}
	}

	// If no Length key, return actual length
	return len(sp.Stream), nil
}

// GetStreamType returns the stream type based on the dictionary
func (sp *StreamProcessor) GetStreamType() string {
	// Check for Type key
	if typeObj, ok := sp.Dictionary["Type"]; ok {
		return typeObj.(string)
	}

	// Check for Subtype key
	if subtypeObj, ok := sp.Dictionary["Subtype"]; ok {
		return subtypeObj.(string)
	}

	// Check content type based on other keys
	if _, ok := sp.Dictionary["Width"]; ok {
		if _, ok := sp.Dictionary["Height"]; ok {
			return "Image"
		}
	}

	if _, ok := sp.Dictionary["BBox"]; ok {
		return "Form"
	}

	return "Unknown"
}

// IsContentStream returns whether the stream is a page content stream
func (sp *StreamProcessor) IsContentStream() bool {
	// Content streams typically don't have a Type or Subtype
	// but may contain text or drawing operators
	if bytes.Contains(sp.Stream, []byte("BT")) && bytes.Contains(sp.Stream, []byte("ET")) {
		return true
	}

	// Check for common graphics operators
	operators := []string{"m", "l", "c", "v", "y", "h", "re", "S", "s", "f", "F", "n", "q", "Q"}
	for _, op := range operators {
		if bytes.Contains(sp.Stream, []byte(" "+op+" ")) {
			return true
		}
	}

	return false
}

// ExtractText extracts text from a content stream
func (sp *StreamProcessor) ExtractText() (string, error) {
	if !sp.IsContentStream() {
		return "", fmt.Errorf("not a content stream")
	}

	// Find all text objects
	textRegex := regexp.MustCompile(`BT(.*?)ET`)
	textMatches := textRegex.FindAll(sp.Stream, -1)

	var textBuilder strings.Builder

	for _, textBlock := range textMatches {
		// Extract text showing operators
		tjRegex := regexp.MustCompile(`\((.*?)\)\s+Tj`)
		tjMatches := tjRegex.FindAllSubmatch(textBlock, -1)

		for _, match := range tjMatches {
			textBytes := match[1]
			text := string(textBytes)
			textBuilder.WriteString(text)
			textBuilder.WriteString(" ")
		}

		// Handle TJ operator
		tjArrayRegex := regexp.MustCompile(`\[(.*?)\]\s+TJ`)
		tjArrayMatches := tjArrayRegex.FindAllSubmatch(textBlock, -1)

		for _, tjArrayMatch := range tjArrayMatches {
			tjArray := tjArrayMatch[1]

			// Extract string parts from the TJ array
			stringRegex := regexp.MustCompile(`\((.*?)\)`)
			stringMatches := stringRegex.FindAllSubmatch(tjArray, -1)

			for _, match := range stringMatches {
				textBytes := match[1]
				text := string(textBytes)
				textBuilder.WriteString(text)
				textBuilder.WriteString(" ")
			}
		}
	}

	return textBuilder.String(), nil
}

// ProcessInlineImage processes an inline image in a content stream
func ProcessInlineImage(content []byte) (map[string]interface{}, []byte, error) {
	// Inline images start with BI, followed by dictionary, then ID, then image data, then EI
	startIdx := bytes.Index(content, []byte("BI"))
	if startIdx == -1 {
		return nil, nil, fmt.Errorf("inline image start not found")
	}

	dictStart := startIdx + 2 // Skip 'BI'
	idIdx := bytes.Index(content[dictStart:], []byte("ID"))
	if idIdx == -1 {
		return nil, nil, fmt.Errorf("inline image ID marker not found")
	}
	idIdx += dictStart

	// Extract dictionary
	dictBytes := content[dictStart:idIdx]
	dict := make(map[string]interface{})

	// Parse inline image dictionary (simple key-value pairs)
	keyValueRegex := regexp.MustCompile(`/([A-Za-z0-9]+)\s+([^/\s]+)`)
	matches := keyValueRegex.FindAllSubmatch(dictBytes, -1)

	for _, match := range matches {
		key := string(match[1])
		value := string(match[2])
		dict[key] = value
	}

	// Find end of image data
	dataStart := idIdx + 2 // Skip 'ID'
	eiIdx := bytes.Index(content[dataStart:], []byte("EI"))
	if eiIdx == -1 {
		return nil, nil, fmt.Errorf("inline image EI marker not found")
	}
	eiIdx += dataStart

	// Extract image data
	imageData := content[dataStart:eiIdx]

	return dict, imageData, nil
}

// ProcessStream is a convenience function to process a stream
func ProcessStream(objNum int, stream []byte, dict map[string]interface{}) ([]byte, error) {
	processor := NewStreamProcessor(objNum, stream, dict)
	err := processor.Process()
	if err != nil {
		return nil, err
	}
	return processor.Stream, nil
}

// ReadStream reads a stream from an io.Reader
func ReadStream(r io.Reader, length int) ([]byte, error) {
	buffer := make([]byte, length)
	_, err := io.ReadFull(r, buffer)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}
