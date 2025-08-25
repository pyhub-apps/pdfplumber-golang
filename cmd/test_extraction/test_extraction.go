package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/allieus/pdfplumber-go"
	"github.com/allieus/pdfplumber-go/pkg/pdf"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test_extraction.go <pdf-file>")
		fmt.Println("\nThis program demonstrates text extraction capabilities.")
		os.Exit(1)
	}
	
	pdfPath := os.Args[1]
	
	fmt.Printf("Testing PDFPlumber-Go with: %s\n", pdfPath)
	fmt.Println(strings.Repeat("=", 50))
	
	// Open the PDF file
	doc, err := pdfplumber.OpenWithDslipak(pdfPath)
	if err != nil {
		log.Fatalf("Failed to open PDF: %v", err)
	}
	defer doc.Close()
	
	// Get document information
	fmt.Printf("Total pages: %d\n", doc.PageCount())
	fmt.Println(strings.Repeat("-", 50))
	
	// Process each page
	for i := 0; i < doc.PageCount() && i < 3; i++ { // Limit to first 3 pages for testing
		page, err := doc.GetPage(i)
		if err != nil {
			log.Printf("Failed to get page %d: %v", i+1, err)
			continue
		}
		
		fmt.Printf("\n📄 Page %d:\n", i+1)
		fmt.Printf("   Dimensions: %.2f x %.2f\n", page.GetWidth(), page.GetHeight())
		
		// Extract text
		text := page.ExtractText()
		if text != "" {
			fmt.Println("\n   📝 Extracted Text:")
			// Show first 500 characters of extracted text
			displayText := text
			if len(displayText) > 500 {
				displayText = displayText[:500] + "..."
			}
			// Indent the text for better readability
			lines := strings.Split(displayText, "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					fmt.Printf("      %s\n", line)
				}
			}
		} else {
			fmt.Println("   ⚠️  No text found on this page")
		}
		
		// Get objects count
		objects := page.GetObjects()
		fmt.Printf("\n   📊 Objects found:\n")
		fmt.Printf("      Characters: %d\n", len(objects.Chars))
		fmt.Printf("      Lines: %d\n", len(objects.Lines))
		fmt.Printf("      Rectangles: %d\n", len(objects.Rects))
		fmt.Printf("      Images: %d\n", len(objects.Images))
		
		// Show sample character objects
		if len(objects.Chars) > 0 {
			fmt.Printf("\n   🔤 Sample characters (first 10):\n")
			for j, char := range objects.Chars {
				if j >= 10 {
					break
				}
				fmt.Printf("      '%s' at (%.2f, %.2f)\n", char.Text, char.X0, char.Y0)
			}
		}
		
		fmt.Println(strings.Repeat("-", 50))
	}
	
	// Test cropping functionality
	if doc.PageCount() > 0 {
		fmt.Println("\n🔍 Testing crop functionality:")
		page, _ := doc.GetPage(0)
		
		// Crop to the top half of the page
		bbox := page.GetBBox()
		cropBox := pdf.BoundingBox{
			X0: bbox.X0,
			Y0: bbox.Y0,
			X1: bbox.X1,
			Y1: bbox.Y0 + (bbox.Y1-bbox.Y0)/2,
		}
		
		croppedPage := page.Crop(cropBox)
		croppedText := croppedPage.ExtractText()
		
		fmt.Printf("   Original text length: %d characters\n", len(page.ExtractText()))
		fmt.Printf("   Cropped text length: %d characters\n", len(croppedText))
		
		if len(croppedText) > 0 && len(croppedText) < len(page.ExtractText()) {
			fmt.Println("   ✅ Cropping works - extracted less text from cropped area")
		}
	}
	
	fmt.Println("\n✨ Test complete!")
}