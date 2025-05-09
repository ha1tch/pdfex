package document

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/yourusername/pdfex/internal/utils"
)

// Regular expressions for XRef table parsing
var (
	trailerDictRegex = regexp.MustCompile(`trailer\s+<<(.*?)>>`)
	xrefEntryRegex   = regexp.MustCompile(`^(\d{10}) (\d{5}) ([nf])$`)
)

// findLastXRefOffset finds the offset of the last xref table
func findLastXRefOffset(file *os.File, fileSize int64) (int64, error) {
	// Look for startxref at the end of the file
	bufSize := int64(1024)
	if fileSize < bufSize {
		bufSize = fileSize
	}

	buffer := make([]byte, bufSize)
	_, err := file.Seek(fileSize-bufSize, io.SeekStart)
	if err != nil {
		return 0, fmt.Errorf("failed to seek to end of file: %v", err)
	}

	n, err := file.Read(buffer)
	if err != nil {
		return 0, fmt.Errorf("failed to read from end of file: %v", err)
	}

	// Look for "startxref" followed by a number
	startxrefPattern := regexp.MustCompile(`startxref\s*(\d+)`)
	matches := startxrefPattern.FindSubmatch(buffer[:n])

	if len(matches) < 2 {
		return 0, fmt.Errorf("startxref not found in last %d bytes", bufSize)
	}

	offset, err := strconv.ParseInt(string(matches[1]), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid startxref offset: %v", err)
	}

	return offset, nil
}

// parseXRef parses the cross-reference table
func parseXRef(file *os.File, offset int64, doc *PDFDocument) error {
	utils.LogDebugf("Attempting to parse xref table at offset %d", offset)

	_, err := file.Seek(offset, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek to xref table: %v", err)
	}

	// Read a small buffer to check what's at this position
	checkBuf := make([]byte, 20)
	n, err := file.Read(checkBuf)
	if err != nil {
		return fmt.Errorf("failed to read at xref position: %v", err)
	}

	utils.LogDebugf("Content at xref offset: %q", string(checkBuf[:n]))

	// Check for 'xref' at the beginning of the buffer
	if !bytes.HasPrefix(checkBuf[:n], []byte("xref")) {
		// Try to handle the case where there might be whitespace or line breaks
		if n >= 5 {
			xrefPos := bytes.Index(checkBuf[:n], []byte("xref"))
			if xrefPos != -1 {
				offset += int64(xrefPos)
				_, err = file.Seek(offset, io.SeekStart)
				if err != nil {
					return fmt.Errorf("failed to seek to adjusted xref position: %v", err)
				}
				utils.LogDebugf("Found 'xref' at adjusted position %d", offset)
			} else {
				return fmt.Errorf("xref keyword not found at the specified offset")
			}
		} else {
			return fmt.Errorf("insufficient data at xref position")
		}
	}

	// Reposition at the start of the xref table
	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek back to xref table: %v", err)
	}

	scanner := bufio.NewScanner(file)

	// Verify xref
	if !scanner.Scan() {
		return fmt.Errorf("failed to read first line of xref table")
	}

	firstLine := scanner.Text()
	utils.LogDebugf("First line of supposed xref table: %q", firstLine)

	if !strings.HasPrefix(firstLine, "xref") {
		return fmt.Errorf("invalid xref table: missing 'xref' keyword")
	}

	// Read subsections
	for scanner.Scan() {
		line := scanner.Text()
		utils.LogDebugf("Processing xref line: %q", line)

		if strings.HasPrefix(line, "trailer") {
			break
		}

		// Parse subsection header
		parts := strings.Fields(line)
		if len(parts) == 2 {
			startObj, err := strconv.Atoi(parts[0])
			if err != nil {
				utils.Logf(utils.LogWarning, "Invalid xref subsection start: %v\n", err)
				continue
			}

			count, err := strconv.Atoi(parts[1])
			if err != nil {
				utils.Logf(utils.LogWarning, "Invalid xref subsection count: %v\n", err)
				continue
			}

			utils.LogDebugf("Processing xref subsection: start=%d, count=%d", startObj, count)

			// Read entries
			for i := 0; i < count && scanner.Scan(); i++ {
				entry := scanner.Text()
				matches := xrefEntryRegex.FindStringSubmatch(entry)

				if len(matches) == 4 {
					offset, err := strconv.ParseInt(matches[1], 10, 64)
					if err != nil {
						utils.Logf(utils.LogWarning, "Invalid xref entry offset: %v\n", err)
						continue
					}

					gen, err := strconv.Atoi(matches[2])
					if err != nil {
						utils.Logf(utils.LogWarning, "Invalid xref entry generation: %v\n", err)
						continue
					}

					inUse := matches[3] == "n"

					objNum := startObj + i
					doc.XRefTable[objNum] = PDFXRefEntry{
						Offset:     offset,
						Generation: gen,
						InUse:      inUse,
					}

					utils.LogDebugf("Added xref entry for object %d: offset=%d, gen=%d, inUse=%v",
						objNum, offset, gen, inUse)
				} else {
					utils.Logf(utils.LogWarning, "Invalid xref entry format: %s\n", entry)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning xref table: %v", err)
	}

	utils.LogDebugf("Successfully parsed xref table with %d entries", len(doc.XRefTable))
	return nil
}

// parseTrailer parses the trailer dictionary
func parseTrailer(file *os.File, xrefOffset int64, doc *PDFDocument) error {
	utils.LogDebugf("Parsing trailer starting from xref offset %d", xrefOffset)

	_, err := file.Seek(xrefOffset, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek to xref table for trailer: %v", err)
	}

	// Read until we find the trailer
	scanner := bufio.NewScanner(file)
	trailerData := ""
	inTrailer := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "trailer") {
			inTrailer = true
			utils.LogDebugf("Found trailer marker")
			continue
		}

		if inTrailer {
			trailerData += line + "\n"
			if strings.Contains(line, "startxref") {
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning trailer: %v", err)
	}

	if trailerData == "" {
		return fmt.Errorf("no trailer data found")
	}

	utils.LogDebugf("Collected trailer data: %s", trailerData[:min(100, len(trailerData))])

	// Extract trailer dictionary
	matches := trailerDictRegex.FindStringSubmatch(trailerData)

	if len(matches) > 1 {
		dictBytes := []byte(matches[1])
		err := utils.ParseDictionary(dictBytes, doc.Trailer)
		if err != nil {
			return fmt.Errorf("failed to parse trailer dictionary: %v", err)
		}
		utils.LogDebugf("Successfully parsed trailer dictionary with %d entries", len(doc.Trailer))
	} else {
		return fmt.Errorf("trailer dictionary not found")
	}

	return nil
}

// parseXRefAndTrailer parses both the xref table and trailer
func parseXRefAndTrailer(file *os.File, xrefOffset int64, doc *PDFDocument) error {
	utils.LogDebugf("Starting xref and trailer parsing from offset %d", xrefOffset)

	// Set log level to debug temporarily for this parse operation
	origLogLevel := utils.GetLogLevel()
	utils.SetLogLevel(utils.LogDebug)
	defer utils.SetLogLevel(origLogLevel)

	// Try standard xref table parsing
	err := parseXRef(file, xrefOffset, doc)
	if err != nil {
		utils.LogDebugf("Standard xref parsing failed: %v", err)

		// Try to recover by looking for xref in the vicinity
		newOffset, found := findNearbyXref(file, xrefOffset)
		if found {
			utils.LogDebugf("Found xref marker at nearby offset %d, retrying", newOffset)
			err = parseXRef(file, newOffset, doc)
			if err != nil {
				return fmt.Errorf("failed to parse xref table even at adjusted offset: %v", err)
			}
			xrefOffset = newOffset // Update offset for trailer parsing
		} else {
			// Try to rebuild xref table by scanning the file
			utils.LogDebugf("Attempting to rebuild xref table by scanning file")
			err = rebuildXRefTable(file, doc)
			if err != nil {
				return fmt.Errorf("failed to parse or rebuild xref table: %v", err)
			}
		}
	}

	// Parse the trailer dictionary
	err = parseTrailer(file, xrefOffset, doc)
	if err != nil {
		utils.LogDebugf("Standard trailer parsing failed: %v", err)

		// Try to find trailer by scanning from the end
		trailerOffset, found := findTrailerFromEnd(file)
		if found {
			utils.LogDebugf("Found trailer at offset %d, retrying", trailerOffset)
			err = parseTrailer(file, trailerOffset, doc)
			if err != nil {
				return fmt.Errorf("failed to parse trailer even at adjusted offset: %v", err)
			}
		} else {
			return fmt.Errorf("failed to parse trailer dictionary: %v", err)
		}
	}

	return nil
}

// findNearbyXref searches for the "xref" keyword near the given offset
func findNearbyXref(file *os.File, offset int64) (int64, bool) {
	// Try within a reasonable range (1KB) before and after the offset
	const searchRange = 1024

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		utils.LogDebugf("Failed to get file info: %v", err)
		return 0, false
	}

	fileSize := fileInfo.Size()

	// Set start offset, ensuring we don't go below 0
	startOffset := offset - searchRange
	if startOffset < 0 {
		startOffset = 0
	}

	// Set end offset, ensuring we don't go beyond file size
	endOffset := offset + searchRange
	if endOffset > fileSize {
		endOffset = fileSize
	}

	// Allocate buffer for the search range
	bufSize := endOffset - startOffset
	buffer := make([]byte, bufSize)

	// Seek to start offset
	_, err = file.Seek(startOffset, io.SeekStart)
	if err != nil {
		utils.LogDebugf("Failed to seek to search start: %v", err)
		return 0, false
	}

	// Read the search range
	_, err = io.ReadFull(file, buffer)
	if err != nil {
		utils.LogDebugf("Failed to read search range: %v", err)
		return 0, false
	}

	// Look for "xref" in the buffer
	xrefIndex := bytes.Index(buffer, []byte("xref"))
	if xrefIndex != -1 {
		// Found "xref" at this offset within the buffer
		foundOffset := startOffset + int64(xrefIndex)
		utils.LogDebugf("Found 'xref' at offset %d", foundOffset)
		return foundOffset, true
	}

	return 0, false
}

// findTrailerFromEnd scans backward from the end of the file to find the trailer
func findTrailerFromEnd(file *os.File) (int64, bool) {
	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		utils.LogDebugf("Failed to get file info: %v", err)
		return 0, false
	}

	fileSize := fileInfo.Size()

	// Use a reasonable buffer size for scanning from the end
	const bufSize = 4096
	buffer := make([]byte, bufSize)

	// Start from the end of the file and work backward
	for offset := fileSize - bufSize; offset >= 0; offset -= bufSize / 2 {
		// Adjust buffer size for the last chunk
		readSize := bufSize
		if offset+bufSize > fileSize {
			readSize = int(fileSize - offset)
		}

		// Seek to the current position
		_, err = file.Seek(offset, io.SeekStart)
		if err != nil {
			utils.LogDebugf("Failed to seek during trailer search: %v", err)
			return 0, false
		}

		// Read the chunk
		n, err := file.Read(buffer[:readSize])
		if err != nil && err != io.EOF {
			utils.LogDebugf("Failed to read during trailer search: %v", err)
			return 0, false
		}

		// Look for "trailer" in this chunk
		trailerIndex := bytes.Index(buffer[:n], []byte("trailer"))
		if trailerIndex != -1 {
			// Found "trailer" at this offset within the buffer
			foundOffset := offset + int64(trailerIndex)
			utils.LogDebugf("Found 'trailer' at offset %d", foundOffset)
			return foundOffset, true
		}
	}

	return 0, false
}

// rebuildXRefTable attempts to rebuild the xref table by scanning the file for objects
func rebuildXRefTable(file *os.File, doc *PDFDocument) error {
	utils.LogDebugf("Rebuilding xref table by scanning file")

	// Seek to beginning of file
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek to beginning of file: %v", err)
	}

	// Pattern to match object definitions: "# # obj"
	objPattern := regexp.MustCompile(`(\d+)\s+(\d+)\s+obj`)

	// Create scanner with a large buffer to handle long lines
	scanner := bufio.NewScanner(file)
	// Allocate a buffer of 10MB
	const maxScanBufferSize = 10 * 1024 * 1024
	scanBuf := make([]byte, maxScanBufferSize)
	scanner.Buffer(scanBuf, maxScanBufferSize)

	fileOffset := int64(0)
	lineCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		// Look for object definitions
		matches := objPattern.FindStringSubmatch(line)
		if len(matches) == 3 {
			objNum, err := strconv.Atoi(matches[1])
			if err != nil {
				utils.LogDebugf("Invalid object number at line %d: %v", lineCount, err)
				continue
			}

			gen, err := strconv.Atoi(matches[2])
			if err != nil {
				utils.LogDebugf("Invalid generation number at line %d: %v", lineCount, err)
				continue
			}

			// Calculate object offset (account for bytes we've already read)
			objOffset := fileOffset - int64(len(line)) - 1 // -1 for the newline
			headerIdx := strings.Index(line, matches[0])
			if headerIdx >= 0 {
				objOffset += int64(headerIdx)
			}

			// Add to xref table
			doc.XRefTable[objNum] = PDFXRefEntry{
				Offset:     objOffset,
				Generation: gen,
				InUse:      true,
			}

			utils.LogDebugf("Rebuilt xref: Object %d gen %d at offset %d", objNum, gen, objOffset)
		}

		// Update file offset
		fileOffset += int64(len(scanner.Bytes()) + 1) // +1 for the newline
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning file during xref rebuilding: %v", err)
	}

	utils.LogDebugf("Rebuilt xref table with %d entries", len(doc.XRefTable))
	return nil
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// findXRefEntries finds xref entries for a specific object
func (doc *PDFDocument) findXRefEntries(objNum int) ([]PDFXRefEntry, bool) {
	entry, ok := doc.XRefTable[objNum]
	if ok {
		return []PDFXRefEntry{entry}, true
	}
	return nil, false
}

// getObjectFromXRef gets an object using the xref table
func (doc *PDFDocument) getObjectFromXRef(objNum int, file *os.File) (PDFObject, error) {
	entries, ok := doc.findXRefEntries(objNum)
	if !ok || len(entries) == 0 {
		return PDFObject{}, fmt.Errorf("object %d not found in xref table", objNum)
	}

	entry := entries[0]
	if !entry.InUse {
		return PDFObject{}, fmt.Errorf("object %d is marked as free", objNum)
	}

	// Seek to the object
	_, err := file.Seek(entry.Offset, io.SeekStart)
	if err != nil {
		return PDFObject{}, fmt.Errorf("failed to seek to object %d: %v", objNum, err)
	}

	// Read the object header
	objHeader := make([]byte, 50) // Should be enough for the header
	n, err := file.Read(objHeader)
	if err != nil {
		return PDFObject{}, fmt.Errorf("failed to read object %d header: %v", objNum, err)
	}

	// Check object header format
	objHeaderPattern := regexp.MustCompile(`(\d+) (\d+) obj`)
	headerMatches := objHeaderPattern.FindSubmatch(objHeader[:n])

	if len(headerMatches) < 3 {
		return PDFObject{}, fmt.Errorf("invalid object %d header format", objNum)
	}

	matchedObjNum, err := strconv.Atoi(string(headerMatches[1]))
	if err != nil {
		return PDFObject{}, fmt.Errorf("invalid object number: %v", err)
	}

	generation, err := strconv.Atoi(string(headerMatches[2]))
	if err != nil {
		return PDFObject{}, fmt.Errorf("invalid generation number: %v", err)
	}

	if matchedObjNum != objNum || generation != entry.Generation {
		return PDFObject{}, fmt.Errorf("object number mismatch: expected %d gen %d, got %d gen %d",
			objNum, entry.Generation, matchedObjNum, generation)
	}

	// Read the object content
	var contentBuffer bytes.Buffer

	_, err = file.Seek(entry.Offset, io.SeekStart)
	if err != nil {
		return PDFObject{}, fmt.Errorf("failed to seek to object %d: %v", objNum, err)
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
		return PDFObject{}, fmt.Errorf("error scanning object %d: %v", objNum, err)
	}

	content := contentBuffer.Bytes()

	// Trim "endobj" from the end
	if idx := bytes.Index(content, []byte("endobj")); idx != -1 {
		content = content[:idx]
	}

	obj := PDFObject{
		ObjectNumber: objNum,
		Generation:   generation,
		Content:      content,
		Dictionary:   make(map[string]interface{}),
	}

	// Parse dictionary
	if bytes.HasPrefix(content, []byte("<<")) && bytes.Contains(content, []byte(">>")) {
		dictEnd := bytes.Index(content, []byte(">>"))
		dictBytes := content[2:dictEnd]
		err := utils.ParseDictionary(dictBytes, obj.Dictionary)
		if err != nil {
			utils.Logf(utils.LogWarning, "Error parsing dictionary for object %d: %v\n", objNum, err)
		}
	}

	// Check for stream
	if bytes.Contains(content, []byte("stream")) && bytes.Contains(content, []byte("endstream")) {
		streamStart := bytes.Index(content, []byte("stream"))
		streamStart += 6 // length of "stream"
		if content[streamStart] == '\r' && content[streamStart+1] == '\n' {
			streamStart += 2
		} else if content[streamStart] == '\n' {
			streamStart += 1
		}

		streamEnd := bytes.Index(content, []byte("endstream"))
		if streamStart < streamEnd {
			obj.Stream = content[streamStart:streamEnd]
			obj.IsStream = true
		}
	}

	return obj, nil
}

// GetTrailerEntry gets an entry from the trailer dictionary
func (doc *PDFDocument) GetTrailerEntry(key string) (interface{}, bool) {
	value, ok := doc.Trailer[key]
	return value, ok
}

// GetRootObject gets the root catalog object
func (doc *PDFDocument) GetRootObject() (PDFObject, bool) {
	if doc.RootCatalog != 0 {
		if obj, ok := doc.Objects[doc.RootCatalog]; ok {
			return obj, true
		}
	}
	return PDFObject{}, false
}

// GetXRefTableSize returns the size of the cross-reference table
func (doc *PDFDocument) GetXRefTableSize() int {
	return len(doc.XRefTable)
}
