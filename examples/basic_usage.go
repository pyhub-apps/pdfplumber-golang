package main

import (
	"fmt"

	// These imports will be used once full implementation is complete
	// "log"
	// "github.com/allieus/pdfplumber-go"
	
	"github.com/allieus/pdfplumber-go/pkg/pdf"
)

func main() {
	// Example usage (requires a PDF file to test)
	fmt.Println("PDFPlumber-Go Basic Usage Example")
	fmt.Println("==================================")
	
	// This is a demonstration of the API structure
	// Actual functionality will be implemented in subsequent phases
	
	// Example of how the API will work once fully implemented:
	/*
	// Open a PDF file
	doc, err := pdfplumber.Open("sample.pdf")
	if err != nil {
		log.Fatal(err)
	}
	defer doc.Close()
	
	// Get document metadata
	metadata := doc.GetMetadata()
	fmt.Printf("Title: %s\n", metadata.Title)
	fmt.Printf("Author: %s\n", metadata.Author)
	fmt.Printf("Pages: %d\n", doc.PageCount())
	
	// Process first page
	page, err := doc.GetPage(0)
	if err != nil {
		log.Fatal(err)
	}
	
	// Get page dimensions
	fmt.Printf("Page 1 dimensions: %.2f x %.2f\n", page.GetWidth(), page.GetHeight())
	
	// Extract text (to be implemented)
	text := page.ExtractText()
	fmt.Printf("Text: %s\n", text)
	
	// Extract tables (to be implemented)
	tables := page.ExtractTables()
	fmt.Printf("Found %d tables\n", len(tables))
	
	// Filter objects by type
	objects := page.GetObjects()
	fmt.Printf("Characters: %d\n", len(objects.Chars))
	fmt.Printf("Lines: %d\n", len(objects.Lines))
	fmt.Printf("Rectangles: %d\n", len(objects.Rects))
	
	// Crop page to specific area
	croppedPage := page.Crop(pdf.BoundingBox{
		X0: 100,
		Y0: 100,
		X1: 400,
		Y1: 500,
	})
	croppedText := croppedPage.ExtractText()
	fmt.Printf("Cropped text: %s\n", croppedText)
	*/
	
	fmt.Println("\nNote: This example shows the planned API structure.")
	fmt.Println("Full functionality is being implemented in phases.")
	fmt.Println("See TODOs.md for current implementation status.")
}

// Example of custom text extraction options (future implementation)
func extractWithOptions(page pdf.Page) {
	// Extract text with layout preservation
	text := page.ExtractText(
		pdf.WithLayout(true),
		pdf.WithXTolerance(3.0),
		pdf.WithYTolerance(3.0),
	)
	fmt.Println(text)
}

// Example of filtering objects (current implementation)
func filterRedText(page pdf.Page) {
	// Filter to get only red-colored text
	redObjects := page.Filter(func(obj pdf.Object) bool {
		if obj.GetType() != pdf.ObjectTypeChar {
			return false
		}
		
		props := obj.GetProperties()
		if color, ok := props["color"].(pdf.Color); ok {
			return color.R > 200 && color.G < 50 && color.B < 50
		}
		return false
	})
	
	fmt.Printf("Found %d red characters\n", len(redObjects.Chars))
}

// Example of working with bounding boxes (current implementation)
func demonstrateBoundingBox() {
	// Create a bounding box
	bbox := pdf.BoundingBox{
		X0: 72,  // 1 inch from left
		Y0: 72,  // 1 inch from top
		X1: 540, // 7.5 inches from left
		Y1: 720, // 10 inches from top
	}
	
	fmt.Printf("Bounding box dimensions: %.2f x %.2f\n", bbox.Width(), bbox.Height())
	
	// Check if a point is within the box
	if bbox.Contains(100, 100) {
		fmt.Println("Point (100, 100) is within the bounding box")
	}
	
	// Check intersection with another box
	otherBox := pdf.BoundingBox{X0: 500, Y0: 600, X1: 600, Y1: 700}
	if bbox.Intersects(otherBox) {
		fmt.Println("Boxes intersect")
	}
}