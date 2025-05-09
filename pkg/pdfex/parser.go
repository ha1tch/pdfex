package pdfex

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/yourusername/pdfex/internal/document"
	"github.com/yourusername/pdfex/internal/metrics"
	"github.com/yourusername/pdfex/internal/text"
	"github.com/yourusername/pdfex/internal/utils"
)

// PDFDocument represents a parsed PDF document with a public API
type PDFDocument struct {
	doc *document.PDFDocument
}

// ParseOptions contains options for parsing PDFs
type ParseOptions struct {
	LogLevel              utils.LogLevel
	ExtractText           bool
	ExtractFonts          bool
	ExtractImages         bool
	MaxObjectsToScan      int
	SkipPageTree          bool
	FollowReferences      bool
	StrictMode            bool
	OutputChunks          bool
	ChunkOutputPath       string
	TreatWarningsAsErrors bool
}

// DefaultParseOptions returns default parsing options
func DefaultParseOptions() *ParseOptions {
	return &ParseOptions{
		LogLevel:              utils.LogWarning,
		ExtractText:           true,
		ExtractFonts:          true,
		ExtractImages:         false,
		MaxObjectsToScan:      0, // 0 means no limit
		SkipPageTree:          false,
		FollowReferences:      true,
		StrictMode:            false,
		OutputChunks:          false,
		TreatWarningsAsErrors: false,
	}
}

// ParsePDF parses a PDF file with default options
func ParsePDF(filename string) (*PDFDocument, error) {
	return ParsePDFWithOptions(filename, DefaultParseOptions())
}

// ParsePDFWithOptions parses a PDF file with the specified options
func ParsePDFWithOptions(filename string, options *ParseOptions) (*PDFDocument, error) {
	// Set up logging
	utils.SetLogLevel(options.LogLevel)

	// Parse the PDF
	doc, err := document.ParsePDF(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PDF: %v", err)
	}

	return &PDFDocument{doc}, nil
}

// ParsePDFFromBytes parses a PDF from a byte slice
func ParsePDFFromBytes(data []byte, name string) (*PDFDocument, error) {
	// Write data to a temporary file
	tmpFile, err := os.CreateTemp("", "pdfex-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %v", err)
	}

	tempName := tmpFile.Name()
	defer os.Remove(tempName) // Clean up

	_, err = tmpFile.Write(data)
	if err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write data to temporary file: %v", err)
	}

	tmpFile.Close()

	// Parse the temporary file
	return ParsePDF(tempName)
}

// Version returns the PDF version
func (p *PDFDocument) Version() string {
	return p.doc.Version
}

// PageCount returns the number of pages in the document
func (p *PDFDocument) PageCount() int {
	return len(p.doc.Pages)
}

// ObjectCount returns the number of objects in the document
func (p *PDFDocument) ObjectCount() int {
	return len(p.doc.Objects)
}

// FontCount returns the number of fonts in the document
func (p *PDFDocument) FontCount() int {
	return len(p.doc.Fonts)
}

// TextChunkCount returns the number of text chunks in the document
func (p *PDFDocument) TextChunkCount() int {
	return len(p.doc.TextChunks)
}

// GetText returns the text content of the document
func (p *PDFDocument) GetText() string {
	var allText strings.Builder
	for _, page := range p.doc.Pages {
		allText.WriteString(page.Text)
		allText.WriteString("\n\n")
	}
	return allText.String()
}

// GetPageText returns the text content of a specific page
func (p *PDFDocument) GetPageText(pageNum int) (string, error) {
	if pageNum < 1 || pageNum > len(p.doc.Pages) {
		return "", fmt.Errorf("page number out of range: %d", pageNum)
	}
	return p.doc.Pages[pageNum-1].Text, nil
}

// GetTextChunks returns the text chunks of the document
func (p *PDFDocument) GetTextChunks() []string {
	return p.doc.TextChunks
}

// SaveChunksToFile saves the text chunks to a file
func (p *PDFDocument) SaveChunksToFile(filename string) error {
	return p.doc.SaveChunksToFile(filename)
}

// Metrics returns the document metrics
func (p *PDFDocument) Metrics() *metrics.PDFMetrics {
	return p.doc.Metrics()
}

// ExtractTextContent extracts text from the document
func (p *PDFDocument) ExtractTextContent() (string, error) {
	return text.ExtractTextContent(p.doc)
}

// GetTextByPattern searches for text matching a pattern
func (p *PDFDocument) GetTextByPattern(pattern string) ([]string, error) {
	var results []string

	// Get full text
	fullText, err := p.ExtractTextContent()
	if err != nil {
		return nil, err
	}

	// Create regex from pattern
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %v", err)
	}

	// Search for pattern
	matches := regex.FindAllString(fullText, -1)

	// Add all matches to results
	for _, matchText := range matches {
		results = append(results, matchText)
	}

	return results, nil
}

// GetPageDimensions returns the width and height of a specific page
func (p *PDFDocument) GetPageDimensions(pageNum int) (width, height float64, err error) {
	if pageNum < 1 || pageNum > len(p.doc.Pages) {
		return 0, 0, fmt.Errorf("page number out of range: %d", pageNum)
	}

	page := p.doc.Pages[pageNum-1]
	return page.Width, page.Height, nil
}

// GetMetadata returns the document metadata
func (p *PDFDocument) GetMetadata() map[string]string {
	metadata := make(map[string]string)

	// Try to find the info dictionary
	if infoRef, ok := p.doc.Trailer["Info"]; ok {
		objNum, err := utils.ExtractReference(infoRef.(string))
		if err == nil {
			if infoObj, ok := p.doc.Objects[objNum]; ok {
				// Extract common metadata fields
				commonFields := []string{"Title", "Author", "Subject", "Keywords", "Creator", "Producer", "CreationDate", "ModDate"}

				for _, field := range commonFields {
					if value, ok := infoObj.Dictionary[field]; ok {
						metadata[field] = value.(string)
					}
				}

				// Process any other fields in the info dictionary
				for key, value := range infoObj.Dictionary {
					if !contains(commonFields, key) {
						metadata[key] = value.(string)
					}
				}
			}
		}
	}

	return metadata
}

// contains checks if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Close releases any resources associated with the document
func (p *PDFDocument) Close() error {
	// Currently, there's nothing to close
	return nil
}

// PDFInfo represents basic information about a PDF file
type PDFInfo struct {
	Filename  string
	FileSize  int64
	PageCount int
	Version   string
	ParseTime time.Duration
}

// GetPDFInfo returns basic information about a PDF file without fully parsing it
func GetPDFInfo(filename string) (*PDFInfo, error) {
	startTime := time.Now()

	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}
	fileSize := fileInfo.Size()

	// Check PDF header
	header := make([]byte, 8)
	_, err = file.Read(header)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %v", err)
	}

	if !strings.HasPrefix(string(header), "%PDF-") {
		return nil, fmt.Errorf("not a PDF file")
	}

	// Extract PDF version
	version := string(header[5:])

	// Try to get page count without fully parsing
	pageCount := -1 // -1 means unknown

	// First try to get the root catalog
	doc, err := ParsePDF(filename)
	if err == nil {
		pageCount = doc.PageCount()
	}

	parseTime := time.Since(startTime)

	return &PDFInfo{
		Filename:  filename,
		FileSize:  fileSize,
		PageCount: pageCount,
		Version:   version,
		ParseTime: parseTime,
	}, nil
}

// CreatePDFMetricsCollection creates a metrics collection from multiple PDF files
func CreatePDFMetricsCollection(filenames []string) (*metrics.MetricsCollection, error) {
	collection := metrics.NewMetricsCollection()

	for _, filename := range filenames {
		doc, err := ParsePDF(filename)
		if err != nil {
			utils.LogWarningf("Failed to parse %s: %v", filename, err)
			continue
		}

		collection.Add(doc.Metrics())
	}

	return collection, nil
}

// Version returns the version of the pdfex library
func Version() string {
	return "1.0.0" // Change this to match your actual version
}
