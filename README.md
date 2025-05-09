# pdfex - PDF Extraction Library and Tool

`pdfex` is a Go library and command-line tool for parsing PDF files and extracting text and metadata. It provides a clean API for developers and a versatile command-line interface for end users.

## STATUS: EXPERIMENTAL
#### The design is mostly done and it almost works, but several crucial features are still not supported / not implemented.

## Features

- Parse and extract text from PDF files
- Extract document structure and metadata
- Analyze PDF content with detailed metrics
- Handle various PDF encodings and filters
- Process compressed stream objects
- Support for PDF versions 1.0 through 1.7
- Command-line interface for easy extraction tasks
- Go API for integration with other applications

## Installation

### Command-line Tool

```bash
go install github.com/yourusername/pdfex/cmd/pdfex@latest
```

### Library

```bash
go get github.com/yourusername/pdfex
```

## Quick Start

### Using the Command-line Tool

```bash
# Extract text from a PDF file
pdfex -text document.pdf

# Save extracted text to a file
pdfex -text -o output.txt document.pdf

# Generate statistics about a PDF file
pdfex -stats document.pdf

# Output statistics in JSON format
pdfex -json -stats stats.json document.pdf

# Process all PDFs in a directory
pdfex -r -text -csv=stats.csv /path/to/documents/
```

### Using the Library

```go
package main

import (
	"fmt"
	"log"

	"github.com/yourusername/pdfex/pkg/pdfex"
)

func main() {
	// Parse a PDF file
	doc, err := pdfex.ParsePDF("document.pdf")
	if err != nil {
		log.Fatalf("Error parsing PDF: %v", err)
	}

	// Extract text
	text, err := doc.ExtractTextContent()
	if err != nil {
		log.Fatalf("Error extracting text: %v", err)
	}

	// Print text
	fmt.Println(text)

	// Get document metrics
	metrics := doc.Metrics()
	fmt.Printf("PDF Version: %s\n", doc.Version())
	fmt.Printf("Pages: %d\n", metrics.PageCount)
	fmt.Printf("Objects: %d\n", metrics.ObjectCount)
	fmt.Printf("Characters: %d\n", metrics.CharacterCount)
}
```

## Command-line Options

```
Usage: pdfex [options] <pdf_file_or_directory>...

Options:
  -v           Enable verbose output
  -debug       Enable debug output
  -text        Extract text content (default: true)
  -o string    Output file for extracted text (default: stdout)
  -stats       Output statistics in human-readable format
  -json        Output statistics in JSON format
  -csv         Output statistics in CSV format
  -r           Process directories recursively
  -find string Find text matching pattern
```

## Library API

### Main Types

- `pdfex.PDFDocument`: Represents a parsed PDF document
- `metrics.PDFMetrics`: Contains statistics about a PDF document
- `document.PDFPage`: Represents a page in a PDF document

### Key Functions

- `pdfex.ParsePDF(filename string) (*PDFDocument, error)`: Parse a PDF file
- `pdfex.ParsePDFWithOptions(filename string, options *ParseOptions) (*PDFDocument, error)`: Parse a PDF file with options
- `pdfex.ParsePDFFromBytes(data []byte, name string) (*PDFDocument, error)`: Parse a PDF from memory
- `pdfex.GetPDFInfo(filename string) (*PDFInfo, error)`: Get basic information about a PDF file

### Document Methods

- `doc.Version() string`: Get the PDF version
- `doc.PageCount() int`: Get the number of pages
- `doc.GetText() string`: Get the text content of the document
- `doc.GetPageText(pageNum int) (string, error)`: Get the text of a specific page
- `doc.Metrics() *metrics.PDFMetrics`: Get document metrics
- `doc.GetMetadata() map[string]string`: Get document metadata

## Architecture

The `pdfex` library is organized into several packages:

- `pkg/pdfex`: Main public API
- `internal/document`: Core document structure
- `internal/content`: Stream and filter processing
- `internal/text`: Text extraction logic
- `internal/metrics`: Statistics gathering
- `internal/utils`: Common utilities

## Limitations

- Limited support for encrypted PDFs
- No support for interactive forms
- Limited support for some advanced font features
- No support for rendering PDF content as images
- Limited support for PDF/A validation

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

