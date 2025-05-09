package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourusername/pdfex/internal/utils"
	"github.com/yourusername/pdfex/pkg/pdfex"
)

func main() {
	// Define command-line flags
	verbose := flag.Bool("v", false, "Enable verbose output")
	debug := flag.Bool("debug", false, "Enable debug output")
	extractText := flag.Bool("text", true, "Extract text content")
	outputFile := flag.String("o", "", "Output file for extracted text (default: stdout)")
	jsonStats := flag.Bool("json", false, "Output statistics in JSON format")
	statsFile := flag.String("stats", "", "Output file for statistics (default: stdout)")
	recursive := flag.Bool("r", false, "Process directories recursively")
	csvStats := flag.Bool("csv", false, "Output statistics in CSV format")
	findPattern := flag.String("find", "", "Find text matching pattern")

	// Parse flags
	flag.Parse()

	// Configure logging
	logLevel := utils.LogWarning
	if *verbose {
		logLevel = utils.LogInfo
	}
	if *debug {
		logLevel = utils.LogDebug
	}
	utils.SetLogLevel(logLevel)

	// Check for input files/directories
	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("Usage: basic_extraction [options] <pdf_file_or_directory>...")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Process each input
	var filesToProcess []string

	for _, path := range args {
		fileInfo, err := os.Stat(path)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		if fileInfo.IsDir() {
			// Process directory
			if *recursive {
				err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() && strings.ToLower(filepath.Ext(filePath)) == ".pdf" {
						filesToProcess = append(filesToProcess, filePath)
					}
					return nil
				})
				if err != nil {
					fmt.Printf("Error walking directory %s: %v\n", path, err)
				}
			} else {
				// Just process PDF files in the top level
				files, err := os.ReadDir(path)
				if err != nil {
					fmt.Printf("Error reading directory %s: %v\n", path, err)
					continue
				}

				for _, file := range files {
					if !file.IsDir() && strings.ToLower(filepath.Ext(file.Name())) == ".pdf" {
						filesToProcess = append(filesToProcess, filepath.Join(path, file.Name()))
					}
				}
			}
		} else if strings.ToLower(filepath.Ext(path)) == ".pdf" {
			// Single PDF file
			filesToProcess = append(filesToProcess, path)
		} else {
			fmt.Printf("Skipping non-PDF file: %s\n", path)
		}
	}

	// Check if we found any PDF files
	if len(filesToProcess) == 0 {
		fmt.Println("No PDF files found.")
		os.Exit(1)
	}

	fmt.Printf("Found %d PDF files to process.\n", len(filesToProcess))

	// Prepare CSV output if needed
	var csvFile *os.File
	if *csvStats {
		var err error
		outputPath := "pdfex_stats.csv"
		if *statsFile != "" {
			outputPath = *statsFile
		}

		csvFile, err = os.Create(outputPath)
		if err != nil {
			fmt.Printf("Error creating CSV file: %v\n", err)
			os.Exit(1)
		}
		defer csvFile.Close()

		// Write CSV header
		fmt.Fprintf(csvFile, "Filename,Size,Pages,Objects,Fonts,Streams,ParseTime\n")
	}

	// Process each PDF file
	for _, pdfFile := range filesToProcess {
		fmt.Printf("Processing %s...\n", pdfFile)

		startTime := time.Now()

		// Parse the PDF
		doc, err := pdfex.ParsePDF(pdfFile)
		if err != nil {
			fmt.Printf("Error parsing %s: %v\n", pdfFile, err)
			continue
		}

		parseTime := time.Since(startTime)

		// Output basic information
		fmt.Printf("  PDF Version: %s\n", doc.Version())
		fmt.Printf("  Pages: %d\n", doc.PageCount())
		fmt.Printf("  Objects: %d\n", doc.ObjectCount())
		fmt.Printf("  Parse Time: %v\n", parseTime)

		// Extract and output text if requested
		if *extractText {
			text, err := doc.ExtractTextContent()
			if err != nil {
				fmt.Printf("Error extracting text: %v\n", err)
			} else {
				if *findPattern != "" {
					// Find text matching pattern
					matches, err := doc.GetTextByPattern(*findPattern)
					if err != nil {
						fmt.Printf("Error searching for pattern: %v\n", err)
					} else {
						fmt.Printf("  Found %d matches for pattern '%s':\n", len(matches), *findPattern)
						for i, match := range matches {
							if i < 10 { // Limit output to 10 matches
								fmt.Printf("    %d: %s\n", i+1, truncateString(match, 80))
							}
						}
						if len(matches) > 10 {
							fmt.Printf("    ... and %d more matches\n", len(matches)-10)
						}
					}
				} else if *outputFile != "" {
					// Write text to file
					err := os.WriteFile(*outputFile, []byte(text), 0644)
					if err != nil {
						fmt.Printf("Error writing text to file: %v\n", err)
					} else {
						fmt.Printf("  Text written to %s\n", *outputFile)
					}
				} else {
					// Print a preview of the text
					preview := truncateString(text, 200)
					fmt.Printf("  Text Preview: %s\n", preview)
				}
			}
		}

		// Output statistics if requested
		if *jsonStats {
			stats := doc.Metrics()
			jsonData, err := stats.JSONFormat()
			if err != nil {
				fmt.Printf("Error generating JSON statistics: %v\n", err)
			} else {
				if *statsFile != "" {
					err := os.WriteFile(*statsFile, jsonData, 0644)
					if err != nil {
						fmt.Printf("Error writing statistics to file: %v\n", err)
					} else {
						fmt.Printf("  Statistics written to %s\n", *statsFile)
					}
				} else {
					fmt.Println("PDF Statistics:")
					fmt.Println(string(jsonData))
				}
			}
		}

		// Write CSV stats if requested
		if *csvStats && csvFile != nil {
			stats := doc.Metrics()
			fmt.Fprintf(csvFile, "%s,%d,%d,%d,%d,%d,%v\n",
				escapeCsvField(filepath.Base(pdfFile)),
				stats.FileSize,
				stats.PageCount,
				stats.ObjectCount,
				stats.FontCount,
				stats.StreamObjectCount,
				stats.ParseTime)
		}

		fmt.Println() // Add a blank line between files
	}

	fmt.Printf("Processed %d PDF files.\n", len(filesToProcess))
}

// truncateString truncates a string to the specified length and adds "..." if needed
func truncateString(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")

	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen-3] + "..."
}

// escapeCsvField escapes a field for CSV output
func escapeCsvField(s string) string {
	if strings.Contains(s, ",") || strings.Contains(s, "\"") || strings.Contains(s, "\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}
