package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/allieus/pdfplumber-go/pkg/pdf"
)

func main() {
	var (
		pdfPath = flag.String("pdf", "", "Path to PDF file")
		library = flag.String("lib", "ledongthuc", "PDF library to use (ledongthuc, dslipak)")
	)
	flag.Parse()

	if *pdfPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Open PDF with the specified library
	var doc pdf.Document
	var err error

	switch *library {
	case "ledongthuc":
		doc, err = pdf.OpenWithLedongthuc(*pdfPath)
	case "dslipak":
		doc, err = pdf.OpenWithDslipak(*pdfPath)
	default:
		log.Fatalf("Unknown library: %s", *library)
	}

	if err != nil {
		log.Fatalf("Failed to open PDF: %v", err)
	}
	defer doc.Close()

	fmt.Printf("Using library: %s\n", *library)
	fmt.Printf("PDF opened successfully\n")
	fmt.Printf("Pages: %d\n\n", doc.PageCount())

	// Process first page
	page, err := doc.GetPage(0)
	if err != nil {
		log.Fatalf("Failed to get page: %v", err)
	}

	// Get character objects
	objects := page.GetObjects()
	chars := objects.Chars

	fmt.Printf("Page 1 - Character Font Information:\n")
	fmt.Printf("Total characters: %d\n\n", len(chars))

	// Show first 10 characters with font details
	maxChars := 10
	if len(chars) < maxChars {
		maxChars = len(chars)
	}

	fmt.Println("Character details:")
	for i := 0; i < maxChars; i++ {
		char := chars[i]
		fmt.Printf("%d. '%s' - Font: %s, Size: %.2f\n",
			i+1, char.Text, char.Font, char.FontSize)
	}

	// Check if all characters have the same font size
	if len(chars) > 0 {
		firstSize := chars[0].FontSize
		allSame := true
		for _, char := range chars {
			if char.FontSize != firstSize {
				allSame = false
				break
			}
		}
		
		fmt.Printf("\nFont size consistency: ")
		if allSame {
			fmt.Printf("All characters use font size %.2f\n", firstSize)
		} else {
			fmt.Println("Characters use different font sizes")
		}
	}

	// Compare with Python pdfplumber format
	fmt.Println("\nPython-compatible format (first 5 characters):")
	for i := 0; i < 5 && i < len(chars); i++ {
		char := chars[i]
		fmt.Printf("{'text': '%s', 'fontname': '%s', 'size': %f}\n",
			char.Text, char.Font, char.FontSize)
	}
}