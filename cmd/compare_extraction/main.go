package main

import (
	"fmt"
	"log"
	"os"

	"github.com/pyhub-apps/pdfplumber-golang"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <pdf-file>")
		os.Exit(1)
	}
	
	pdfPath := os.Args[1]
	
	// Open with our implementation
	doc, err := pdfplumber.OpenWithDslipak(pdfPath)
	if err != nil {
		log.Fatalf("Failed to open PDF: %v", err)
	}
	defer doc.Close()
	
	fmt.Println("Go pdfplumber-go extraction:")
	fmt.Printf("  Pages: %d\n", doc.PageCount())
	
	if doc.PageCount() > 0 {
		page, err := doc.GetPage(0)
		if err != nil {
			log.Fatal(err)
		}
		
		fmt.Printf("  Page 1:\n")
		fmt.Printf("    Width: %.2f\n", page.GetWidth())
		fmt.Printf("    Height: %.2f\n", page.GetHeight())
		bbox := page.GetBBox()
		fmt.Printf("    Bbox: (%.2f, %.2f, %.2f, %.2f)\n", bbox.X0, bbox.Y0, bbox.X1, bbox.Y1)
		
		// Get characters
		objects := page.GetObjects()
		fmt.Printf("    Total characters: %d\n", len(objects.Chars))
		
		// Show first 5 characters
		if len(objects.Chars) > 0 {
			fmt.Println("    First 5 characters:")
			for i, char := range objects.Chars {
				if i >= 5 {
					break
				}
				fmt.Printf("      %d. Text: '%s'\n", i+1, char.Text)
				fmt.Printf("         Position: x0=%.2f, y0=%.2f, x1=%.2f, y1=%.2f\n", 
					char.X0, char.Y0, char.X1, char.Y1)
				fmt.Printf("         Font: %s, Size: %.2f\n", char.Font, char.FontSize)
			}
		}
		
		// Extract text
		text := page.ExtractText()
		fmt.Printf("    Extracted text: %q\n", text)
	}
	
	fmt.Println("\nDifferences from Python pdfplumber:")
	fmt.Println("1. Y coordinates: We use PDF coordinates (bottom=0), pdfplumber inverts (top=0)")
	fmt.Println("2. Font info: We need to extract actual font names and sizes")
	fmt.Println("3. Word extraction: We need to implement extract_words properly")
	fmt.Println("4. Layout mode: We need to implement layout preservation")
}