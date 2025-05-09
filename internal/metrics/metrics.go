package metrics

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PDFMetrics contains statistics about the parsed PDF
type PDFMetrics struct {
	Filename           string
	FileSize           int64
	ParseTime          time.Duration
	Version            string
	ObjectCount        int
	PageCount          int
	FontCount          int
	StreamObjectCount  int
	TextExtractionTime time.Duration
	CharacterCount     int
	TextChunkCount     int
	XRefTableSize      int
	ImageCount         int
	FlatDecodeStreams  int
	ASCII85Streams     int
	LZWStreams         int
	RunLengthStreams   int
	DCTStreams         int
	JPXStreams         int
	CCITTFaxStreams    int
	JBIG2Streams       int
	ObjectTypeCounts   map[string]int
}

// NewPDFMetrics creates a new PDFMetrics instance
func NewPDFMetrics(filename string, fileSize int64) *PDFMetrics {
	return &PDFMetrics{
		Filename:         filename,
		FileSize:         fileSize,
		ObjectTypeCounts: make(map[string]int),
	}
}

// JSONFormat outputs the metrics in JSON format
func (m *PDFMetrics) JSONFormat() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// HumanReadableFormat outputs the metrics in a human-readable format
func (m *PDFMetrics) HumanReadableFormat() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("PDF Analysis for: %s\n", m.Filename))
	sb.WriteString(fmt.Sprintf("File Size: %d bytes\n", m.FileSize))
	sb.WriteString(fmt.Sprintf("Parse Time: %v\n", m.ParseTime))
	sb.WriteString(fmt.Sprintf("PDF Version: %s\n\n", m.Version))

	sb.WriteString("Document Structure:\n")
	sb.WriteString(fmt.Sprintf("- Object Count: %d\n", m.ObjectCount))
	sb.WriteString(fmt.Sprintf("- Stream Objects: %d\n", m.StreamObjectCount))
	sb.WriteString(fmt.Sprintf("- Page Count: %d\n", m.PageCount))
	sb.WriteString(fmt.Sprintf("- Font Count: %d\n", m.FontCount))
	sb.WriteString(fmt.Sprintf("- Image Count: %d\n", m.ImageCount))
	sb.WriteString(fmt.Sprintf("- XRef Table Size: %d\n\n", m.XRefTableSize))

	sb.WriteString("Text Statistics:\n")
	sb.WriteString(fmt.Sprintf("- Text Extraction Time: %v\n", m.TextExtractionTime))
	sb.WriteString(fmt.Sprintf("- Character Count: %d\n", m.CharacterCount))
	sb.WriteString(fmt.Sprintf("- Text Chunk Count: %d\n\n", m.TextChunkCount))

	sb.WriteString("Stream Filters Usage:\n")
	sb.WriteString(fmt.Sprintf("- FlatDecode: %d\n", m.FlatDecodeStreams))
	sb.WriteString(fmt.Sprintf("- ASCII85: %d\n", m.ASCII85Streams))
	sb.WriteString(fmt.Sprintf("- LZW: %d\n", m.LZWStreams))
	sb.WriteString(fmt.Sprintf("- RunLength: %d\n", m.RunLengthStreams))
	sb.WriteString(fmt.Sprintf("- DCT (JPEG): %d\n", m.DCTStreams))
	sb.WriteString(fmt.Sprintf("- JPX (JPEG2000): %d\n", m.JPXStreams))
	sb.WriteString(fmt.Sprintf("- CCITTFax: %d\n", m.CCITTFaxStreams))
	sb.WriteString(fmt.Sprintf("- JBIG2: %d\n\n", m.JBIG2Streams))

	sb.WriteString("Object Types:\n")
	for objType, count := range m.ObjectTypeCounts {
		sb.WriteString(fmt.Sprintf("- %s: %d\n", objType, count))
	}

	return sb.String()
}

// CSVHeader returns the header row for a CSV export
func (m *PDFMetrics) CSVHeader() string {
	return "Filename,FileSize,ParseTime,Version,ObjectCount,PageCount,FontCount,StreamObjectCount," +
		"CharacterCount,TextChunkCount,ImageCount,FlatDecodeStreams,ASCII85Streams,LZWStreams," +
		"RunLengthStreams,DCTStreams,JPXStreams,CCITTFaxStreams,JBIG2Streams"
}

// CSVFormat outputs the metrics in CSV format
func (m *PDFMetrics) CSVFormat() string {
	return fmt.Sprintf("%s,%d,%v,%s,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d",
		escapeCSV(m.Filename),
		m.FileSize,
		m.ParseTime,
		escapeCSV(m.Version),
		m.ObjectCount,
		m.PageCount,
		m.FontCount,
		m.StreamObjectCount,
		m.CharacterCount,
		m.TextChunkCount,
		m.ImageCount,
		m.FlatDecodeStreams,
		m.ASCII85Streams,
		m.LZWStreams,
		m.RunLengthStreams,
		m.DCTStreams,
		m.JPXStreams,
		m.CCITTFaxStreams,
		m.JBIG2Streams)
}

// escapeCSV escapes a string for CSV output
func escapeCSV(s string) string {
	if strings.Contains(s, ",") || strings.Contains(s, "\"") || strings.Contains(s, "\n") {
		return "\"" + strings.Replace(s, "\"", "\"\"", -1) + "\""
	}
	return s
}

// GetFilterCounts returns a map of filter types and their counts
func (m *PDFMetrics) GetFilterCounts() map[string]int {
	return map[string]int{
		"FlatDecode":      m.FlatDecodeStreams,
		"ASCII85Decode":   m.ASCII85Streams,
		"LZWDecode":       m.LZWStreams,
		"RunLengthDecode": m.RunLengthStreams,
		"DCTDecode":       m.DCTStreams,
		"JPXDecode":       m.JPXStreams,
		"CCITTFaxDecode":  m.CCITTFaxStreams,
		"JBIG2Decode":     m.JBIG2Streams,
	}
}

// GetTotalFilterCount returns the total number of filters used
func (m *PDFMetrics) GetTotalFilterCount() int {
	return m.FlatDecodeStreams + m.ASCII85Streams + m.LZWStreams +
		m.RunLengthStreams + m.DCTStreams + m.JPXStreams +
		m.CCITTFaxStreams + m.JBIG2Streams
}

// ObjectDensity returns the object density (objects per page)
func (m *PDFMetrics) ObjectDensity() float64 {
	if m.PageCount == 0 {
		return 0
	}
	return float64(m.ObjectCount) / float64(m.PageCount)
}

// TextDensity returns the text density (characters per page)
func (m *PDFMetrics) TextDensity() float64 {
	if m.PageCount == 0 {
		return 0
	}
	return float64(m.CharacterCount) / float64(m.PageCount)
}

// CompactSummary returns a compact summary of the metrics
func (m *PDFMetrics) CompactSummary() string {
	return fmt.Sprintf("PDF: %s, %d pages, %d objects, %d chars, %v parse time",
		m.Filename, m.PageCount, m.ObjectCount, m.CharacterCount, m.ParseTime)
}

// MetricsCollection represents a collection of PDF metrics
type MetricsCollection struct {
	Metrics []*PDFMetrics
}

// NewMetricsCollection creates a new metrics collection
func NewMetricsCollection() *MetricsCollection {
	return &MetricsCollection{
		Metrics: make([]*PDFMetrics, 0),
	}
}

// Add adds metrics to the collection
func (mc *MetricsCollection) Add(metrics *PDFMetrics) {
	mc.Metrics = append(mc.Metrics, metrics)
}

// ExportCSV exports the collection to CSV format
func (mc *MetricsCollection) ExportCSV() string {
	if len(mc.Metrics) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(mc.Metrics[0].CSVHeader())
	sb.WriteString("\n")

	for _, metrics := range mc.Metrics {
		sb.WriteString(metrics.CSVFormat())
		sb.WriteString("\n")
	}

	return sb.String()
}

// GetAverages returns average metrics across the collection
func (mc *MetricsCollection) GetAverages() *PDFMetrics {
	if len(mc.Metrics) == 0 {
		return NewPDFMetrics("Average", 0)
	}

	avg := NewPDFMetrics("Average", 0)

	// Sum up all metrics
	for _, m := range mc.Metrics {
		avg.FileSize += m.FileSize
		avg.ParseTime += m.ParseTime
		avg.ObjectCount += m.ObjectCount
		avg.PageCount += m.PageCount
		avg.FontCount += m.FontCount
		avg.StreamObjectCount += m.StreamObjectCount
		avg.TextExtractionTime += m.TextExtractionTime
		avg.CharacterCount += m.CharacterCount
		avg.TextChunkCount += m.TextChunkCount
		avg.XRefTableSize += m.XRefTableSize
		avg.ImageCount += m.ImageCount
		avg.FlatDecodeStreams += m.FlatDecodeStreams
		avg.ASCII85Streams += m.ASCII85Streams
		avg.LZWStreams += m.LZWStreams
		avg.RunLengthStreams += m.RunLengthStreams
		avg.DCTStreams += m.DCTStreams
		avg.JPXStreams += m.JPXStreams
		avg.CCITTFaxStreams += m.CCITTFaxStreams
		avg.JBIG2Streams += m.JBIG2Streams
	}

	// Calculate averages
	count := len(mc.Metrics)
	avg.FileSize /= int64(count)
	avg.ParseTime /= time.Duration(count)
	avg.ObjectCount /= count
	avg.PageCount /= count
	avg.FontCount /= count
	avg.StreamObjectCount /= count
	avg.TextExtractionTime /= time.Duration(count)
	avg.CharacterCount /= count
	avg.TextChunkCount /= count
	avg.XRefTableSize /= count
	avg.ImageCount /= count
	avg.FlatDecodeStreams /= count
	avg.ASCII85Streams /= count
	avg.LZWStreams /= count
	avg.RunLengthStreams /= count
	avg.DCTStreams /= count
	avg.JPXStreams /= count
	avg.CCITTFaxStreams /= count
	avg.JBIG2Streams /= count

	return avg
}
