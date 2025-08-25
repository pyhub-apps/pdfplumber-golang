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
		pdfPath    = flag.String("pdf", "", "Path to PDF file")
		xTolerance = flag.Float64("x-tolerance", 3.0, "X tolerance for word separation")
		library    = flag.String("lib", "ledongthuc", "PDF library to use (ledongthuc, dslipak)")
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

	fmt.Printf("Page 1:\n")
	fmt.Printf("  Width: %.2f\n", page.GetWidth())
	fmt.Printf("  Height: %.2f\n", page.GetHeight())

	// Extract words with custom tolerance
	words := page.ExtractWords(pdf.WithWordXTolerance(*xTolerance))
	
	fmt.Printf("\nWords extracted: %d\n", len(words))
	fmt.Printf("X tolerance used: %.1f\n\n", *xTolerance)

	// Print first 10 words with details
	maxWords := 10
	if len(words) < maxWords {
		maxWords = len(words)
	}

	fmt.Println("First words with positions:")
	for i := 0; i < maxWords; i++ {
		word := words[i]
		fmt.Printf("%d. '%s' at (%.2f, %.2f) - (%.2f, %.2f)\n",
			i+1, word.Text, word.X0, word.Y0, word.X1, word.Y1)
	}

	// Print all words as text
	fmt.Println("\nAll words as text:")
	for i, word := range words {
		fmt.Printf("'%s'", word.Text)
		if i < len(words)-1 {
			fmt.Print(" ")
		}
	}
	fmt.Println()

	// Compare with Python format
	fmt.Println("\nPython-compatible format (first 3 words):")
	for i := 0; i < 3 && i < len(words); i++ {
		word := words[i]
		fmt.Printf("{'text': '%s', 'x0': %.2f, 'top': %.2f, 'x1': %.2f, 'bottom': %.2f}\n",
			word.Text, word.X0, word.Y0, word.X1, word.Y1)
	}
}