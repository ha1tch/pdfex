package content

import (
	"bytes"
	"compress/zlib"
	"encoding/ascii85"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/yourusername/pdfex/internal/utils"
)

// DecompressStream decompresses a PDF stream based on its filter type
func DecompressStream(stream []byte, filterSpec string, decodeParms map[string]interface{}) ([]byte, error) {
	// Handle filter arrays like [/FlateDecode /ASCII85Decode]
	if strings.HasPrefix(filterSpec, "[") && strings.HasSuffix(filterSpec, "]") {
		filterArray := utils.ParseArray(filterSpec)
		result := stream
		var err error

		// Apply filters in reverse order (last filter first)
		for i := len(filterArray) - 1; i >= 0; i-- {
			// Get decode parameters for this filter if available
			var filterParms map[string]interface{}
			if decodeParms != nil {
				if parmsArray, ok := decodeParms["Array"]; ok {
					// If DecodeParms is an array, get the corresponding parameters
					parmsArrayStr := parmsArray.(string)
					if strings.HasPrefix(parmsArrayStr, "[") {
						parmsItems := utils.ParseArray(parmsArrayStr)
						if i < len(parmsItems) {
							filterParms = make(map[string]interface{})
							if strings.HasPrefix(parmsItems[i], "<<") {
								parmBytes := []byte(parmsItems[i])[2 : len(parmsItems[i])-2]
								err := utils.ParseDictionary(parmBytes, filterParms)
								if err != nil {
									return nil, fmt.Errorf("error parsing filter params: %v", err)
								}
							}
						}
					}
				}
			}

			result, err = applySingleFilter(result, filterArray[i], filterParms)
			if err != nil {
				return nil, fmt.Errorf("filter %s error: %v", filterArray[i], err)
			}
		}

		return result, nil
	}

	// Single filter
	return applySingleFilter(stream, filterSpec, decodeParms)
}

// applySingleFilter applies a single filter to a stream
func applySingleFilter(stream []byte, filterType string, decodeParms map[string]interface{}) ([]byte, error) {
	switch filterType {
	case "/FlatDecode":
		// Standard library zlib
		reader := bytes.NewReader(stream)
		zlibReader, err := zlib.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("zlib reader initialization failed: %v", err)
		}
		defer zlibReader.Close()

		decompressed, err := io.ReadAll(zlibReader)
		if err != nil {
			return nil, fmt.Errorf("zlib decompression failed: %v", err)
		}

		// Handle predictor if specified
		if decodeParms != nil {
			if predictorStr, ok := decodeParms["Predictor"]; ok {
				predictor, _ := strconv.Atoi(predictorStr.(string))
				if predictor > 1 {
					// Apply predictor post-processing
					return applyPredictor(decompressed, predictor, decodeParms)
				}
			}
		}

		return decompressed, nil

	case "/ASCII85Decode":
		// Standard library ascii85
		decoder := ascii85.NewDecoder(bytes.NewReader(stream))
		decoded, err := io.ReadAll(decoder)
		if err != nil {
			return nil, fmt.Errorf("ascii85 decoding failed: %v", err)
		}
		return decoded, nil

	case "/LZWDecode":
		// LZW Decode (simplified implementation)
		// In a full implementation, you would implement LZW decompression
		return stream, fmt.Errorf("LZW decompression not implemented")

	case "/RunLengthDecode":
		// Custom implementation (simple algorithm)
		return decodeRunLength(stream)

	case "/DCTDecode":
		// DCT (JPEG) - just return the stream as is since it's a JPEG image
		// In a full implementation, you might use an image library to decode
		return stream, nil

	case "/JPXDecode":
		// JPEG 2000 - just return the stream as is
		return stream, nil

	case "/CCITTFaxDecode":
		// CCITT Fax - not implemented here
		return stream, fmt.Errorf("CCITT fax decompression not implemented")

	case "/JBIG2Decode":
		// JBIG2 - not implemented here
		return stream, fmt.Errorf("JBIG2 decompression not implemented")

	default:
		// Return the stream as is if filter not supported
		return stream, fmt.Errorf("unsupported filter type: %s", filterType)
	}
}

// decodeRunLength decodes a run-length encoded stream
func decodeRunLength(input []byte) ([]byte, error) {
	var output bytes.Buffer
	i := 0

	for i < len(input) {
		length := int(input[i])

		if length == 128 {
			// End of data
			break
		} else if length < 128 {
			// Copy the next (length+1) bytes literally
			copyLength := length + 1
			if i+1+copyLength > len(input) {
				return nil, fmt.Errorf("run length decode error: not enough data for literal copy")
			}

			output.Write(input[i+1 : i+1+copyLength])
			i += 1 + copyLength
		} else {
			// Repeat the next byte (257-length) times
			repeatLength := 257 - length
			if i+1 >= len(input) {
				return nil, fmt.Errorf("run length decode error: not enough data for repeat")
			}

			for j := 0; j < repeatLength; j++ {
				output.WriteByte(input[i+1])
			}

			i += 2
		}
	}

	return output.Bytes(), nil
}

// IsSupported returns whether a filter type is supported
func IsSupported(filterType string) bool {
	switch filterType {
	case "/FlatDecode", "/ASCII85Decode", "/RunLengthDecode", "/DCTDecode", "/JPXDecode":
		return true
	default:
		return false
	}
}

// GetSupportedFilters returns a list of supported filter types
func GetSupportedFilters() []string {
	return []string{
		"/FlatDecode",
		"/ASCII85Decode",
		"/RunLengthDecode",
		"/DCTDecode",
		"/JPXDecode",
	}
}

// GetFilterCounts counts the filter types used in a document
func GetFilterCounts(filterSpecs []string) map[string]int {
	counts := make(map[string]int)

	for _, spec := range filterSpecs {
		if strings.HasPrefix(spec, "[") && strings.HasSuffix(spec, "]") {
			// Filter array
			filters := utils.ParseArray(spec)
			for _, filter := range filters {
				counts[filter]++
			}
		} else {
			// Single filter
			counts[spec]++
		}
	}

	return counts
}
