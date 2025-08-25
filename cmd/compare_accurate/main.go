package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pyhub-apps/pdfplumber-golang"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <pdf-file>")
		os.Exit(1)
	}
	
	pdfPath := os.Args[1]
	
	fmt.Println("=== Go pdfplumber-go (ledongthuc) vs Python pdfplumber ===")
	
	// Open with our implementation
	doc, err := pdfplumber.OpenWithLedongthuc(pdfPath)
	if err != nil {
		log.Fatalf("Failed to open PDF: %v", err)
	}
	defer doc.Close()
	
	fmt.Println("Go pdfplumber-go:")
	fmt.Printf("  Pages: %d\n", doc.PageCount())
	
	if doc.PageCount() > 0 {
		page, err := doc.GetPage(0)
		if err != nil {
			log.Fatal(err)
		}
		
		fmt.Printf("\nPage 1:\n")
		fmt.Printf("  Width: %.0f\n", page.GetWidth())
		fmt.Printf("  Height: %.0f\n", page.GetHeight())
		bbox := page.GetBBox()
		fmt.Printf("  Bbox: (%.0f, %.0f, %.0f, %.0f)\n", bbox.X0, bbox.Y0, bbox.X1, bbox.Y1)
		
		// Get characters
		objects := page.GetObjects()
		fmt.Printf("\n  Total characters: %d\n", len(objects.Chars))
		
		// Show first 5 characters
		if len(objects.Chars) > 0 {
			fmt.Println("  First 5 characters:")
			for i, char := range objects.Chars {
				if i >= 5 {
					break
				}
				fmt.Printf("    %d. Text: '%s'\n", i+1, char.Text)
				fmt.Printf("       Position: x0=%.2f, y0=%.2f, x1=%.2f, y1=%.2f\n", 
					char.X0, char.Y0, char.X1, char.Y1)
				fmt.Printf("       Font: %s, Size: %.2f\n", char.Font, char.FontSize)
			}
		}
		
		// Extract text
		text := page.ExtractText()
		fmt.Printf("\n  Extracted text: %q\n", text)
	}
	
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("\nPython pdfplumber (from our test):")
	fmt.Println("  Pages: 1")
	fmt.Println("\nPage 1:")
	fmt.Println("  Width: 595")
	fmt.Println("  Height: 842")
	fmt.Println("  Bbox: (0, 0, 595, 842)")
	fmt.Println("\n  Total characters: 14")
	fmt.Println("  First 5 characters:")
	fmt.Println("    1. Text: 'D'")
	fmt.Println("       Position: x0=56.80, y0=71.20, x1=68.42, y1=87.30")
	fmt.Println("       Font: BAAAAA+Arial-BoldMT, Size: 16.10")
	fmt.Println("    2. Text: 'u'")
	fmt.Println("       Position: x0=68.42, y0=71.20, x1=78.25, y1=87.30")
	fmt.Println("       Font: BAAAAA+Arial-BoldMT, Size: 16.10")
	fmt.Println("\n  Extracted text: \"Dummy PDF file\"")
	
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("\nComparison:")
	
	if doc.PageCount() > 0 {
		page, _ := doc.GetPage(0)
		objects := page.GetObjects()
		
		// Check dimensions
		if page.GetWidth() == 595 && page.GetHeight() == 842 {
			fmt.Println("✅ Page dimensions match!")
		} else {
			fmt.Printf("❌ Page dimensions differ: Go(%fx%f) vs Python(595x842)\n", 
				page.GetWidth(), page.GetHeight())
		}
		
		// Check character count
		if len(objects.Chars) == 14 {
			fmt.Println("✅ Character count matches!")
		} else {
			fmt.Printf("❌ Character count differs: Go(%d) vs Python(14)\n", len(objects.Chars))
		}
		
		// Check Y coordinate inversion
		if len(objects.Chars) > 0 {
			firstChar := objects.Chars[0]
			pythonY := 71.20
			// Check if Y is inverted properly (should be close to 71.20)
			if firstChar.Y0 >= 70 && firstChar.Y0 <= 72 {
				fmt.Println("✅ Y coordinate inversion is correct!")
			} else {
				fmt.Printf("❌ Y coordinate issue: Go(%.2f) vs Python(%.2f)\n", 
					firstChar.Y0, pythonY)
				fmt.Printf("   Hint: Need to invert Y: height(%.0f) - PDF_Y(%.2f) = %.2f\n",
					page.GetHeight(), 758.10, page.GetHeight()-758.10)
			}
		}
		
		// Check text extraction
		text := page.ExtractText()
		if text == "Dummy PDF file" {
			fmt.Println("✅ Text extraction matches!")
		} else {
			fmt.Printf("❌ Text extraction differs: Go(%q) vs Python(\"Dummy PDF file\")\n", text)
		}
	}
}