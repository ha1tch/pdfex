package content

import (
	"fmt"
	"strconv"
)

// applyPredictor applies predictor algorithms for FlatDecode and LZWDecode
func applyPredictor(data []byte, predictor int, decodeParms map[string]interface{}) ([]byte, error) {
	// Get parameters
	columns := 1
	if columnsStr, ok := decodeParms["Columns"]; ok {
		var err error
		columns, err = strconv.Atoi(columnsStr.(string))
		if err != nil {
			return nil, fmt.Errorf("invalid Columns value: %v", err)
		}
	}

	colors := 1
	if colorsStr, ok := decodeParms["Colors"]; ok {
		var err error
		colors, err = strconv.Atoi(colorsStr.(string))
		if err != nil {
			return nil, fmt.Errorf("invalid Colors value: %v", err)
		}
	}

	bitsPerComponent := 8
	if bpcStr, ok := decodeParms["BitsPerComponent"]; ok {
		var err error
		bitsPerComponent, err = strconv.Atoi(bpcStr.(string))
		if err != nil {
			return nil, fmt.Errorf("invalid BitsPerComponent value: %v", err)
		}
	}

	// Calculate bytes per pixel and row stride
	bytesPerPixel := (colors*bitsPerComponent + 7) / 8
	rowLength := ((columns*colors*bitsPerComponent + 7) / 8)

	// PNG/TIFF predictors (2-15)
	if predictor >= 10 && predictor <= 15 {
		// PNG prediction
		return applyPNGPredictor(data, columns, bytesPerPixel, rowLength)
	} else if predictor == 2 {
		// TIFF predictor
		return applyTIFFPredictor(data, columns, colors, bitsPerComponent, rowLength)
	}

	// No predictor or unsupported predictor
	return data, nil
}

// applyPNGPredictor applies the PNG predictor algorithm
func applyPNGPredictor(data []byte, columns, bytesPerPixel, rowLength int) ([]byte, error) {
	// Each row starts with a predictor byte followed by the row data

	// Calculate total rows
	rowStride := rowLength + 1 // +1 for predictor byte
	rows := len(data) / rowStride

	if rows == 0 || len(data)%rowStride != 0 {
		return nil, fmt.Errorf("invalid data size for PNG predictor")
	}

	// Result buffer (without predictor bytes)
	result := make([]byte, rows*rowLength)

	for row := 0; row < rows; row++ {
		// Get predictor type from first byte of row
		predictor := data[row*rowStride]
		rowData := data[row*rowStride+1 : (row+1)*rowStride]
		resultRow := result[row*rowLength : (row+1)*rowLength]

		// Apply predictor based on type
		switch predictor {
		case 0: // None
			copy(resultRow, rowData)

		case 1: // Sub
			// Each byte is replaced by the difference between it and the byte to its left
			for i := 0; i < len(rowData); i++ {
				if i < bytesPerPixel {
					resultRow[i] = rowData[i]
				} else {
					resultRow[i] = rowData[i] + resultRow[i-bytesPerPixel]
				}
			}

		case 2: // Up
			// Each byte is replaced by the difference between it and the byte above it
			if row == 0 {
				copy(resultRow, rowData)
			} else {
				prevRow := result[(row-1)*rowLength : row*rowLength]
				for i := 0; i < len(rowData); i++ {
					resultRow[i] = rowData[i] + prevRow[i]
				}
			}

		case 3: // Average
			// Each byte is replaced by the difference between it and the average of the byte to its left
			// and the byte above it, truncating any fractional part
			for i := 0; i < len(rowData); i++ {
				var left, up byte

				if i >= bytesPerPixel {
					left = resultRow[i-bytesPerPixel]
				}

				if row > 0 {
					up = result[(row-1)*rowLength+i]
				}

				resultRow[i] = rowData[i] + (left+up)/2
			}

		case 4: // Paeth
			// Complex predictor using a function of the byte to the left, above, and to the above-left
			for i := 0; i < len(rowData); i++ {
				var left, up, upLeft byte

				if i >= bytesPerPixel {
					left = resultRow[i-bytesPerPixel]
				}

				if row > 0 {
					up = result[(row-1)*rowLength+i]

					if i >= bytesPerPixel {
						upLeft = result[(row-1)*rowLength+i-bytesPerPixel]
					}
				}

				resultRow[i] = rowData[i] + paethPredictor(left, up, upLeft)
			}

		default:
			// Invalid predictor, use None
			copy(resultRow, rowData)
		}
	}

	return result, nil
}

// paethPredictor implements the Paeth predictor function from the PNG specification
func paethPredictor(a, b, c byte) byte {
	p := int(a) + int(b) - int(c)
	pa := abs(p - int(a))
	pb := abs(p - int(b))
	pc := abs(p - int(c))

	if pa <= pb && pa <= pc {
		return a
	} else if pb <= pc {
		return b
	}
	return c
}

// applyTIFFPredictor applies the TIFF predictor algorithm
func applyTIFFPredictor(data []byte, columns, colors, bitsPerComponent, rowLength int) ([]byte, error) {
	// TIFF horizontal differencing
	// Only implemented for 8-bit samples for now
	if bitsPerComponent != 8 {
		return data, nil
	}

	rows := len(data) / rowLength
	result := make([]byte, rows*rowLength)

	for row := 0; row < rows; row++ {
		rowData := data[row*rowLength : (row+1)*rowLength]
		resultRow := result[row*rowLength : (row+1)*rowLength]

		// Copy first pixel as is
		for i := 0; i < colors; i++ {
			resultRow[i] = rowData[i]
		}

		// For each remaining pixel, add the difference to the previous pixel
		for i := colors; i < len(rowData); i++ {
			resultRow[i] = rowData[i] + resultRow[i-colors]
		}
	}

	return result, nil
}

// abs returns the absolute value of x
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// IsPNGPredictor returns whether a predictor value is a PNG predictor
func IsPNGPredictor(predictor int) bool {
	return predictor >= 10 && predictor <= 15
}

// IsTIFFPredictor returns whether a predictor value is a TIFF predictor
func IsTIFFPredictor(predictor int) bool {
	return predictor == 2
}

// GetPredictorName returns a human-readable name for a predictor value
func GetPredictorName(predictor int) string {
	if predictor == 1 {
		return "No Prediction"
	} else if predictor == 2 {
		return "TIFF Predictor"
	} else if predictor >= 10 && predictor <= 15 {
		return "PNG Predictor"
	}
	return "Unknown Predictor"
}

// SupportedPredictors returns a list of supported predictor values
func SupportedPredictors() []int {
	return []int{1, 2, 10, 11, 12, 13, 14, 15}
}

// GetPNGPredictorName returns a human-readable name for a PNG predictor value
func GetPNGPredictorName(predictor byte) string {
	switch predictor {
	case 0:
		return "None"
	case 1:
		return "Sub"
	case 2:
		return "Up"
	case 3:
		return "Average"
	case 4:
		return "Paeth"
	default:
		return "Unknown"
	}
}
