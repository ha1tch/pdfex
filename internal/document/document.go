package document

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yourusername/pdfex/internal/metrics"
	"github.com/yourusername/pdfex/internal/utils"
)

// Pre-compile frequently used regex patterns
var ()

// PDFDocument represents a parsed PDF document
type PDFDocument struct {
	Version     string
	Objects     map[int]PDFObject
	Trailer     map[string]interface{}
	Pages       []PDFPage
	TextChunks  []string
	Fonts       map[string]PDFFont // Key is the font resource name
	XRefTable   map[int]PDFXRefEntry
	XRefOffset  int64
	RootCatalog int // Object number of the root catalog
	metrics     *metrics.PDFMetrics
}

// ParsePDF parses a PDF file and returns a PDFDocument
func ParsePDF(filename string) (*PDFDocument, error) {
	startTime := time.Now()

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Get file size for metrics
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}
	fileSize := fileInfo.Size()

	doc := &PDFDocument{
		Objects:   make(map[int]PDFObject),
		XRefTable: make(map[int]PDFXRefEntry),
		Trailer:   make(map[string]interface{}),
		Fonts:     make(map[string]PDFFont),
		metrics:   metrics.NewPDFMetrics(filename, fileSize),
	}

	// Check PDF header and find version
	err = identifyPDFVersion(doc, file)
	if err != nil {
		return nil, err
	}

	// Find xref offset from end of file
	xrefOffset, err := findLastXRefOffset(file, fileSize)
	if err != nil {
		utils.Logf(utils.LogWarning, "XRef table not found, falling back to linear parsing: %v\n", err)
		// Fallback to linear parsing if xref not found
		return fallbackLinearParse(filename)
	}

	doc.XRefOffset = xrefOffset

	// Parse xref table and trailer
	err = parseXRefAndTrailer(file, xrefOffset, doc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse xref table and trailer: %v", err)
	}

	// Get root catalog
	if rootRef, ok := doc.Trailer["Root"]; ok {
		objNum, err := utils.ExtractReference(rootRef.(string))
		if err != nil {
			utils.Logf(utils.LogWarning, "Invalid Root reference: %v\n", err)
		} else {
			doc.RootCatalog = objNum
		}
	}

	// Load objects using the xref table
	err = loadObjects(file, doc)
	if err != nil {
		return nil, fmt.Errorf("failed to load objects: %v", err)
	}

	// Extract text from content streams
	textStartTime := time.Now()

	// Process document structure - call the implementations
	processPages(doc)
	processFonts(doc)
	handleMissingFonts(doc)
	processText(doc)
	processTextChunks(doc)

	doc.metrics.TextExtractionTime = time.Since(textStartTime)
	doc.metrics.ParseTime = time.Since(startTime)

	// Update metrics
	updateMetrics(doc)

	return doc, nil
}

// identifyPDFVersion identifies the PDF version from the header
func identifyPDFVersion(doc *PDFDocument, file *os.File) error {
	// Reset file pointer to beginning
	_, err := file.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to beginning of file: %v", err)
	}

	// Check PDF header
	header := make([]byte, 8)
	_, err = file.Read(header)
	if err != nil {
		return fmt.Errorf("failed to read header: %v", err)
	}

	if !strings.HasPrefix(string(header), "%PDF-") {
		return fmt.Errorf("not a PDF file")
	}

	// Extract version
	doc.Version = string(header[5:])
	doc.metrics.Version = doc.Version

	return nil
}

// fallbackLinearParse falls back to linear parsing if xref table can't be used
func fallbackLinearParse(filename string) (*PDFDocument, error) {
	utils.Logf(utils.LogInfo, "Using linear parsing for file: %s\n", filename)

	startTime := time.Now()

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Get file size for metrics
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}
	fileSize := fileInfo.Size()

	// Read the entire file into memory for linear parsing
	fileContent := make([]byte, fileSize)
	_, err = file.Read(fileContent)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	// Create new document with metrics
	doc := &PDFDocument{
		Objects: make(map[int]PDFObject),
		Trailer: make(map[string]interface{}),
		Fonts:   make(map[string]PDFFont),
		metrics: metrics.NewPDFMetrics(filename, fileSize),
	}

	// Identify the PDF version
	err = identifyPDFVersion(doc, file)
	if err != nil {
		return nil, err
	}

	// Use linear parsing to find and parse objects
	err = parseObjectsLinearly(fileContent, doc)
	if err != nil {
		return nil, fmt.Errorf("error during linear parsing: %v", err)
	}

	// Extract document structure after parsing - call the implementations
	processStreams(doc)
	processPages(doc)
	processFonts(doc)
	handleMissingFonts(doc)
	processText(doc)
	processTextChunks(doc)

	// Update metrics
	doc.metrics.ParseTime = time.Since(startTime)
	updateMetrics(doc)

	return doc, nil
}

// ObjectCount returns the number of objects in the document
func (doc *PDFDocument) ObjectCount() int {
	return len(doc.Objects)
}

// PageCount returns the number of pages in the document
func (doc *PDFDocument) PageCount() int {
	return len(doc.Pages)
}

// FontCount returns the number of fonts in the document
func (doc *PDFDocument) FontCount() int {
	return len(doc.Fonts)
}

// TextChunkCount returns the number of text chunks in the document
func (doc *PDFDocument) TextChunkCount() int {
	return len(doc.TextChunks)
}

// Metrics returns the metrics object
func (doc *PDFDocument) Metrics() *metrics.PDFMetrics {
	return doc.metrics
}

// SaveChunksToFile saves the text chunks to a file
func (doc *PDFDocument) SaveChunksToFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	for i, chunk := range doc.TextChunks {
		_, err = fmt.Fprintf(file, "--- Chunk %d ---\n%s\n\n", i+1, chunk)
		if err != nil {
			return err
		}
	}

	return nil
}

// updateMetrics updates the document metrics
func updateMetrics(doc *PDFDocument) {
	// Update metrics
	doc.metrics.ObjectCount = len(doc.Objects)
	doc.metrics.PageCount = len(doc.Pages)
	doc.metrics.FontCount = len(doc.Fonts)
	doc.metrics.TextChunkCount = len(doc.TextChunks)
	doc.metrics.XRefTableSize = len(doc.XRefTable)

	// Count various object types
	countObjects(doc)
}

// countObjects counts various types of objects and updates metrics
func countObjects(doc *PDFDocument) {
	var streamCount, charCount int

	for _, obj := range doc.Objects {
		if obj.IsStream {
			streamCount++

			// Count filter types
			if filter, ok := obj.Dictionary["Filter"]; ok {
				filterStr := filter.(string)
				if strings.Contains(filterStr, "/FlatDecode") {
					doc.metrics.FlatDecodeStreams++
				}
				if strings.Contains(filterStr, "/ASCII85Decode") {
					doc.metrics.ASCII85Streams++
				}
				if strings.Contains(filterStr, "/LZWDecode") {
					doc.metrics.LZWStreams++
				}
				if strings.Contains(filterStr, "/RunLengthDecode") {
					doc.metrics.RunLengthStreams++
				}
				if strings.Contains(filterStr, "/DCTDecode") {
					doc.metrics.DCTStreams++
				}
				if strings.Contains(filterStr, "/JPXDecode") {
					doc.metrics.JPXStreams++
				}
				if strings.Contains(filterStr, "/CCITTFaxDecode") {
					doc.metrics.CCITTFaxStreams++
				}
				if strings.Contains(filterStr, "/JBIG2Decode") {
					doc.metrics.JBIG2Streams++
				}
			}
		}

		// Count object types
		if objType, ok := obj.Dictionary["Type"]; ok {
			typeName := objType.(string)
			doc.metrics.ObjectTypeCounts[typeName]++

			// Count specific types
			if typeName == "/XObject" {
				if subtype, ok := obj.Dictionary["Subtype"]; ok && subtype.(string) == "/Image" {
					doc.metrics.ImageCount++
				}
			}
		}
	}

	// Count total characters across all pages
	for _, page := range doc.Pages {
		charCount += len(page.Text)
	}

	doc.metrics.StreamObjectCount = streamCount
	doc.metrics.CharacterCount = charCount
}

// GetText returns the full text content of the document
func (doc *PDFDocument) GetText() string {
	var allText strings.Builder
	for _, page := range doc.Pages {
		allText.WriteString(page.Text)
		allText.WriteString("\n")
	}
	return allText.String()
}

// parseObjectsLinearly is declared here but implemented elsewhere
func parseObjectsLinearly(fileContent []byte, doc *PDFDocument) error {
	// Implementation in another file
	return nil
}
