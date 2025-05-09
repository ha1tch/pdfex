package document

import (
	"bytes"
	"strings"

	"github.com/yourusername/pdfex/internal/utils"
)

// Implementation of the page processing functionality
// We'll use function names with "Impl" suffix to avoid conflicts

// Functions documented in document.go but implemented here:
func processPages(doc *PDFDocument) {
	// Find catalog and pages
	var catalogObj PDFObject
	if doc.RootCatalog != 0 {
		if obj, ok := doc.Objects[doc.RootCatalog]; ok {
			catalogObj = obj
		}
	} else {
		// Fallback: Look for catalog object
		for _, obj := range doc.Objects {
			if typ, ok := obj.Dictionary["Type"]; ok && typ == "/Catalog" {
				catalogObj = obj
				break
			}
		}
	}

	// Find pages
	if pagesRef, ok := catalogObj.Dictionary["Pages"]; ok {
		// Extract object number from reference
		pageTreeObjNum, err := utils.ExtractReference(pagesRef.(string))
		if err != nil {
			utils.Logf(utils.LogWarning, "Invalid Pages reference: %v\n", err)
			return
		}
		processPageTree(doc, pageTreeObjNum, 1)
	}
}

// processPageTree processes a page tree node
func processPageTree(doc *PDFDocument, objNum int, pageCounter int) int {
	obj, ok := doc.Objects[objNum]
	if !ok {
		utils.Logf(utils.LogWarning, "Page tree object %d not found\n", objNum)
		return pageCounter
	}

	if nodeType, ok := obj.Dictionary["Type"]; ok {
		if nodeType == "/Pages" {
			// This is a page tree node
			if kidsRef, ok := obj.Dictionary["Kids"]; ok {
				// Extract kid references
				kidRefs := utils.ParseArray(kidsRef.(string))

				for _, kidRef := range kidRefs {
					kidObjNum, err := utils.ExtractReference(kidRef)
					if err != nil {
						utils.Logf(utils.LogWarning, "Invalid kid reference: %v\n", err)
						continue
					}
					pageCounter = processPageTree(doc, kidObjNum, pageCounter)
				}
			}
		} else if nodeType == "/Page" {
			// This is a page
			page := PDFPage{
				PageNumber:    pageCounter,
				ResourcesDict: make(map[string]interface{}),
			}

			// Get page dimensions
			if mediaBoxRef, ok := obj.Dictionary["MediaBox"]; ok {
				mediaBox := utils.ParseArray(mediaBoxRef.(string))
				if len(mediaBox) == 4 {
					// MediaBox is [llx lly urx ury]
					llx, err := utils.ParseFloat(mediaBox[0])
					if err != nil {
						utils.Logf(utils.LogWarning, "Invalid MediaBox llx: %v\n", err)
					}
					lly, err := utils.ParseFloat(mediaBox[1])
					if err != nil {
						utils.Logf(utils.LogWarning, "Invalid MediaBox lly: %v\n", err)
					}
					urx, err := utils.ParseFloat(mediaBox[2])
					if err != nil {
						utils.Logf(utils.LogWarning, "Invalid MediaBox urx: %v\n", err)
					}
					ury, err := utils.ParseFloat(mediaBox[3])
					if err != nil {
						utils.Logf(utils.LogWarning, "Invalid MediaBox ury: %v\n", err)
					}

					page.Width = urx - llx
					page.Height = ury - lly
				}
			}

			// Get resources
			if resourcesRef, ok := obj.Dictionary["Resources"]; ok {
				switch res := resourcesRef.(type) {
				case string:
					if strings.HasPrefix(res, "<<") {
						// Resources are inline
						resourcesDict := make(map[string]interface{})
						dictBytes := []byte(res)[2 : len(res)-2]
						err := utils.ParseDictionary(dictBytes, resourcesDict)
						if err != nil {
							utils.Logf(utils.LogWarning, "Error parsing resources dictionary: %v\n", err)
						}
						page.ResourcesDict = resourcesDict
					} else {
						// Resources are a reference
						resourcesObjNum, err := utils.ExtractReference(res)
						if err != nil {
							utils.Logf(utils.LogWarning, "Invalid resources reference: %v\n", err)
						} else if resourcesObj, ok := doc.Objects[resourcesObjNum]; ok {
							page.ResourcesDict = resourcesObj.Dictionary
						}
					}
				case map[string]interface{}:
					page.ResourcesDict = res
				}
			}

			// Get content stream
			if contentsRef, ok := obj.Dictionary["Contents"]; ok {
				switch contents := contentsRef.(type) {
				case string:
					if strings.HasPrefix(contents, "[") {
						// Multiple content streams
						contentRefs := utils.ParseArray(contents)
						var allContents bytes.Buffer

						for _, contentRef := range contentRefs {
							contentObjNum, err := utils.ExtractReference(contentRef)
							if err != nil {
								utils.Logf(utils.LogWarning, "Invalid content reference: %v\n", err)
								continue
							}
							if contentObj, ok := doc.Objects[contentObjNum]; ok && contentObj.IsStream {
								allContents.Write(contentObj.Stream)
								allContents.WriteString("\n")
							}
						}

						page.Contents = allContents.Bytes()
					} else {
						// Single content stream
						contentObjNum, err := utils.ExtractReference(contents)
						if err != nil {
							utils.Logf(utils.LogWarning, "Invalid content reference: %v\n", err)
						} else if contentObj, ok := doc.Objects[contentObjNum]; ok && contentObj.IsStream {
							page.Contents = contentObj.Stream
						}
					}
				}
			}

			doc.Pages = append(doc.Pages, page)
			return pageCounter + 1
		}
	}

	return pageCounter
}

// processTextChunks chunks the extracted text
func processTextChunks(doc *PDFDocument) {
	// Combine all text from all pages
	var allText strings.Builder
	for _, page := range doc.Pages {
		allText.WriteString(page.Text)
		allText.WriteString("\n")
	}

	text := allText.String()

	// Split into chunks (by paragraph, with a max size)
	const maxChunkSize = 1000

	lines := strings.Split(text, "\n")

	var currentChunk strings.Builder

	for _, line := range lines {
		if currentChunk.Len()+len(line)+1 > maxChunkSize {
			// Save current chunk and start a new one
			if currentChunk.Len() > 0 {
				doc.TextChunks = append(doc.TextChunks, currentChunk.String())
				currentChunk.Reset()
			}
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n")
		}
		currentChunk.WriteString(line)
	}

	// Add the last chunk if it's not empty
	if currentChunk.Len() > 0 {
		doc.TextChunks = append(doc.TextChunks, currentChunk.String())
	}
}

// processText extracts text from the document (basic implementation)
func processText(doc *PDFDocument) {
	// This is a placeholder implementation - in a real project,
	// this would be implemented in text/extraction.go
	for i := range doc.Pages {
		if len(doc.Pages[i].Contents) > 0 {
			// Simply convert the content to string as a placeholder
			doc.Pages[i].Text = string(doc.Pages[i].Contents)
		}
	}
}

// processFonts extracts fonts from the document (basic implementation)
func processFonts(doc *PDFDocument) {
	// This is a placeholder implementation - in a real project,
	// this would be implemented in text/fonts.go
	// Create a default font
	doc.Fonts["/DefaultFont"] = PDFFont{
		Name:          "DefaultFont",
		CodeToUnicode: make(map[int]rune),
	}
}

// handleMissingFonts adds a default font for missing fonts
func handleMissingFonts(doc *PDFDocument) {
	// Minimal implementation for interface satisfaction
	// Would be implemented in text/fonts.go in a real project
}

// processStreams processes streams in the document
func processStreams(doc *PDFDocument) {
	// This would be implemented in content/stream.go in a real project
}

// Public API methods

// GetPageText returns the text content of a specific page
func (doc *PDFDocument) GetPageText(pageNum int) (string, error) {
	if pageNum < 1 || pageNum > len(doc.Pages) {
		return "", utils.NewError("page number out of range")
	}

	return doc.Pages[pageNum-1].Text, nil
}

// GetPageCount returns the number of pages in the document
func (doc *PDFDocument) GetPageCount() int {
	return len(doc.Pages)
}

// GetPageDimensions returns the width and height of a specific page
func (doc *PDFDocument) GetPageDimensions(pageNum int) (width, height float64, err error) {
	if pageNum < 1 || pageNum > len(doc.Pages) {
		return 0, 0, utils.NewError("page number out of range")
	}

	page := doc.Pages[pageNum-1]
	return page.Width, page.Height, nil
}
