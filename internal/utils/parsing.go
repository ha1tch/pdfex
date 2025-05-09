package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Regular expressions for parsing PDF objects
	keyRegex        = regexp.MustCompile(`/([A-Za-z0-9]+)[\s]+([\S]+|<<.*?>>|\[.*?\])`)
	nestedDictRegex = regexp.MustCompile(`/([A-Za-z0-9]+)\s+<<(.*?)>>`)
	refPattern      = regexp.MustCompile(`(\d+)\s+\d+\s+R`)
)

// ParseDictionary parses a PDF dictionary with improved handling for nested dictionaries
func ParseDictionary(data []byte, dict map[string]interface{}) error {
	// Handle nested dictionaries
	matches := nestedDictRegex.FindAllSubmatch(data, -1)

	for _, match := range matches {
		key := string(match[1])
		dictData := match[2]

		// Create nested dictionary
		nestedDict := make(map[string]interface{})
		err := ParseDictionary(dictData, nestedDict)
		if err != nil {
			return fmt.Errorf("error parsing nested dictionary for key %s: %v", key, err)
		}

		dict[key] = nestedDict
	}

	// Regular key-value extraction
	matches = keyRegex.FindAllSubmatch(data, -1)

	for _, match := range matches {
		key := string(match[1])
		value := string(match[2])

		// Skip keys that are already processed as nested dictionaries
		if _, exists := dict[key]; exists {
			if _, ok := dict[key].(map[string]interface{}); ok {
				continue
			}
		}

		dict[key] = value
	}

	return nil
}

// ParseArray parses a PDF array
func ParseArray(arrayStr string) []string {
	if !strings.HasPrefix(arrayStr, "[") || !strings.HasSuffix(arrayStr, "]") {
		return nil
	}

	// Remove brackets
	content := arrayStr[1 : len(arrayStr)-1]

	// Split by whitespace, but preserve things like << >> and nested arrays
	var items []string
	var currentItem strings.Builder
	var nestedLevel, dictLevel int

	for i := 0; i < len(content); i++ {
		c := content[i]

		if c == '[' {
			nestedLevel++
			currentItem.WriteByte(c)
		} else if c == ']' {
			nestedLevel--
			currentItem.WriteByte(c)
		} else if c == '<' && i+1 < len(content) && content[i+1] == '<' {
			dictLevel++
			currentItem.WriteString("<<")
			i++
		} else if c == '>' && i+1 < len(content) && content[i+1] == '>' {
			dictLevel--
			currentItem.WriteString(">>")
			i++
		} else if (c == ' ' || c == '\t' || c == '\n' || c == '\r') && nestedLevel == 0 && dictLevel == 0 {
			// Whitespace outside of nested structures
			if currentItem.Len() > 0 {
				items = append(items, currentItem.String())
				currentItem.Reset()
			}
		} else {
			currentItem.WriteByte(c)
		}
	}

	// Add the last item if any
	if currentItem.Len() > 0 {
		items = append(items, currentItem.String())
	}

	return items
}

// ExtractReference extracts the object number from a PDF reference (e.g., "123 0 R")
func ExtractReference(ref string) (int, error) {
	matches := refPattern.FindStringSubmatch(ref)

	if len(matches) > 1 {
		objNum, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, fmt.Errorf("invalid object number: %v", err)
		}
		return objNum, nil
	}
	return 0, fmt.Errorf("invalid reference format: %s", ref)
}

// ParseFloat parses a float from a string
func ParseFloat(str string) (float64, error) {
	return strconv.ParseFloat(str, 64)
}

// ParseInt parses an integer from a string
func ParseInt(str string) (int, error) {
	return strconv.Atoi(str)
}

// ParseOctal parses an octal number from a string
func ParseOctal(str string) (int64, error) {
	return strconv.ParseInt(str, 8, 64)
}

// ParseHex parses a hexadecimal number from a string
func ParseHex(str string) (int64, error) {
	return strconv.ParseInt(str, 16, 64)
}

// IsReference returns true if the string is a PDF reference
func IsReference(str string) bool {
	return refPattern.MatchString(str)
}

// IsDictionary returns true if the string is a PDF dictionary
func IsDictionary(str string) bool {
	return strings.HasPrefix(str, "<<") && strings.HasSuffix(str, ">>")
}

// IsArray returns true if the string is a PDF array
func IsArray(str string) bool {
	return strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]")
}

// IsName returns true if the string is a PDF name
func IsName(str string) bool {
	return strings.HasPrefix(str, "/")
}

// IsStream returns true if the object dictionary contains a Length key
func IsStream(dict map[string]interface{}) bool {
	_, ok := dict["Length"]
	return ok
}

// GetBoolean interprets a PDF value as boolean
func GetBoolean(val interface{}, defaultValue bool) bool {
	switch v := val.(type) {
	case string:
		if v == "true" || v == "/true" {
			return true
		} else if v == "false" || v == "/false" {
			return false
		}
	case bool:
		return v
	}
	return defaultValue
}

// GetInteger interprets a PDF value as integer
func GetInteger(val interface{}, defaultValue int) int {
	switch v := val.(type) {
	case string:
		if strings.HasPrefix(v, "/") {
			v = v[1:]
		}
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	case int:
		return v
	case float64:
		return int(v)
	}
	return defaultValue
}

// GetFloat interprets a PDF value as float
func GetFloat(val interface{}, defaultValue float64) float64 {
	switch v := val.(type) {
	case string:
		if strings.HasPrefix(v, "/") {
			v = v[1:]
		}
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	case int:
		return float64(v)
	case float64:
		return v
	}
	return defaultValue
}

// GetString interprets a PDF value as string
func GetString(val interface{}, defaultValue string) string {
	switch v := val.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	}
	return defaultValue
}

// GetName returns a PDF name without the leading slash
func GetName(val interface{}, defaultValue string) string {
	s := GetString(val, defaultValue)
	if strings.HasPrefix(s, "/") {
		return s[1:]
	}
	return s
}

// DecodePDFString decodes a PDF string (handles hex strings and escapes)
func DecodePDFString(str string) (string, error) {
	// Check if this is a hex string
	if strings.HasPrefix(str, "<") && strings.HasSuffix(str, ">") {
		// Hex string
		hexStr := str[1 : len(str)-1]
		bytes := make([]byte, 0, len(hexStr)/2)

		for i := 0; i < len(hexStr); i += 2 {
			// Handle odd-length hex strings
			if i+1 >= len(hexStr) {
				b, err := strconv.ParseUint(hexStr[i:]+"0", 16, 8)
				if err != nil {
					return "", err
				}
				bytes = append(bytes, byte(b))
				break
			}

			b, err := strconv.ParseUint(hexStr[i:i+2], 16, 8)
			if err != nil {
				return "", err
			}
			bytes = append(bytes, byte(b))
		}

		return string(bytes), nil
	}

	// Regular string (handle escapes)
	if strings.HasPrefix(str, "(") && strings.HasSuffix(str, ")") {
		// Remove parentheses
		str = str[1 : len(str)-1]

		var result strings.Builder
		for i := 0; i < len(str); i++ {
			if str[i] == '\\' && i+1 < len(str) {
				switch str[i+1] {
				case 'n':
					result.WriteByte('\n')
				case 'r':
					result.WriteByte('\r')
				case 't':
					result.WriteByte('\t')
				case 'b':
					result.WriteByte('\b')
				case 'f':
					result.WriteByte('\f')
				case '(':
					result.WriteByte('(')
				case ')':
					result.WriteByte(')')
				case '\\':
					result.WriteByte('\\')
				case '\r', '\n':
					// Ignore line breaks after backslash
					if str[i+1] == '\r' && i+2 < len(str) && str[i+2] == '\n' {
						i++ // Skip the extra character
					}
				default:
					// Could be an octal escape
					if isOctalDigit(str[i+1]) {
						// Try to read up to 3 octal digits
						end := i + 2
						for end < i+4 && end < len(str) && isOctalDigit(str[end]) {
							end++
						}

						octal := str[i+1 : end]
						val, err := strconv.ParseUint(octal, 8, 8)
						if err != nil {
							result.WriteByte(str[i+1])
						} else {
							result.WriteByte(byte(val))
							i = end - 1 // Skip the digits
							continue
						}
					} else {
						// Just an escaped character
						result.WriteByte(str[i+1])
					}
				}
				i++
			} else {
				result.WriteByte(str[i])
			}
		}

		return result.String(), nil
	}

	// Just return the string if it's not in a recognized format
	return str, nil
}

// isOctalDigit returns true if the character is an octal digit
func isOctalDigit(c byte) bool {
	return c >= '0' && c <= '7'
}
