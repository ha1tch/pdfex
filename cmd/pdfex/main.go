package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/yourusername/pdfex/internal/utils"
	"github.com/yourusername/pdfex/pkg/pdfex"
)

func main() {
	// Define command line flags
	verbose := flag.Bool("v", false, "Enable verbose output (sets log level to INFO)")
	debug := flag.Bool("debug", false, "Enable debug output (sets log level to DEBUG)")
	statsOutput := flag.String("stats", "", "Output statistics in human-readable format to the specified file")
	jsonOutput := flag.String("json", "", "Output statistics in JSON format to the specified file")

	// Parse command line flags
	flag.Parse()

	// Set log level based on flags
	if *debug {
		utils.SetLogLevel(utils.LogDebug)
	} else if *verbose {
		utils.SetLogLevel(utils.LogInfo)
	}

	// Check if a PDF file was specified
	if flag.NArg() < 1 {
		fmt.Println("Usage: pdfex [options] <pdf_file>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	filename := flag.Arg(0)

	// Parse the PDF file
	doc, err := pdfex.ParsePDF(filename)
	if err != nil {
		fmt.Printf("Error parsing PDF: %v\n", err)
		os.Exit(1)
	}

	// Print basic info
	fmt.Printf("PDF Version: %s\n", doc.Version())
	fmt.Printf("Number of objects: %d\n", doc.ObjectCount())
	fmt.Printf("Number of pages: %d\n", doc.PageCount())
	fmt.Printf("Number of fonts: %d\n", doc.FontCount())
	fmt.Printf("Number of text chunks: %d\n", doc.TextChunkCount())

	// Output chunks to a file
	chunksFile := strings.TrimSuffix(filename, ".pdf") + "_chunks.txt"
	err = doc.SaveChunksToFile(chunksFile)
	if err != nil {
		fmt.Printf("Error saving chunks: %v\n", err)
	} else {
		fmt.Printf("Chunks saved to %s\n", chunksFile)
	}

	// Output statistics if requested
	if *statsOutput != "" {
		statsContent := doc.Metrics().HumanReadableFormat()
		err = os.WriteFile(*statsOutput, []byte(statsContent), 0644)
		if err != nil {
			fmt.Printf("Error writing statistics to %s: %v\n", *statsOutput, err)
		} else {
			fmt.Printf("Statistics saved to %s\n", *statsOutput)
		}
	}

	if *jsonOutput != "" {
		jsonContent, err := doc.Metrics().JSONFormat()
		if err != nil {
			fmt.Printf("Error generating JSON: %v\n", err)
		} else {
			err = os.WriteFile(*jsonOutput, jsonContent, 0644)
			if err != nil {
				fmt.Printf("Error writing JSON to %s: %v\n", *jsonOutput, err)
			} else {
				fmt.Printf("JSON statistics saved to %s\n", *jsonOutput)
			}
		}
	}

	// If no output file specified but stats flag provided, print to stdout
	if *statsOutput == "" && *jsonOutput == "" {
		// Check if any of the stats arguments were passed in the command line
		statsArgProvided := false
		jsonArgProvided := false

		for _, arg := range os.Args {
			if strings.HasPrefix(arg, "-stats") || strings.HasPrefix(arg, "--stats") {
				statsArgProvided = true
			}
			if strings.HasPrefix(arg, "-json") || strings.HasPrefix(arg, "--json") {
				jsonArgProvided = true
			}
		}

		if statsArgProvided || jsonArgProvided {
			fmt.Println("\nPDF Statistics:")
			fmt.Println(doc.Metrics().HumanReadableFormat())
		}
	}
}
