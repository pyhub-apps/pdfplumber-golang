package main

import (
	"fmt"
	"log"
	"os"
	
	"github.com/allieus/pdfplumber-go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: extract_text <pdf_file>")
		os.Exit(1)
	}
	
	pdfPath := os.Args[1]
	
	// Open the PDF file
	fmt.Printf("Opening PDF: %s\n", pdfPath)
	doc, err := pdfplumber.Open(pdfPath)
	if err != nil {
		log.Fatalf("Failed to open PDF: %v", err)
	}
	defer doc.Close()
	
	// Get document info
	fmt.Printf("Document has %d pages\n\n", doc.PageCount())
	
	// Extract text from each page
	for i := 0; i < doc.PageCount(); i++ {
		page, err := doc.GetPage(i)
		if err != nil {
			log.Printf("Failed to get page %d: %v", i+1, err)
			continue
		}
		
		fmt.Printf("=== Page %d ===\n", page.GetPageNumber())
		fmt.Printf("Size: %.2f x %.2f\n", page.GetWidth(), page.GetHeight())
		
		// Extract text
		text := page.ExtractText()
		if text != "" {
			fmt.Println("\nExtracted Text:")
			fmt.Println(text)
		} else {
			fmt.Println("No text found on this page")
		}
		
		// Get objects count
		objects := page.GetObjects()
		fmt.Printf("\nObjects found:\n")
		fmt.Printf("  Characters: %d\n", len(objects.Chars))
		fmt.Printf("  Lines: %d\n", len(objects.Lines))
		fmt.Printf("  Rectangles: %d\n", len(objects.Rects))
		fmt.Printf("  Curves: %d\n", len(objects.Curves))
		
		// Show first few characters with positions
		if len(objects.Chars) > 0 {
			fmt.Println("\nFirst few characters:")
			maxChars := 5
			if len(objects.Chars) < maxChars {
				maxChars = len(objects.Chars)
			}
			
			for j := 0; j < maxChars; j++ {
				char := objects.Chars[j]
				fmt.Printf("  '%s' at (%.2f, %.2f) size=%.2f\n", 
					char.Text, char.X0, char.Y0, char.FontSize)
			}
		}
		
		fmt.Println()
	}
}