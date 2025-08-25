package main

import (
	"fmt"
	"log"
	
	"github.com/allieus/pdfplumber-go"
)

func main() {
	// Open the PDF
	doc, err := pdfplumber.Open("testdata/sample.pdf")
	if err != nil {
		log.Fatalf("Failed to open PDF: %v", err)
	}
	defer doc.Close()
	
	fmt.Printf("Document has %d pages\n", doc.PageCount())
	
	// Get first page
	page, err := doc.GetPage(0)
	if err != nil {
		log.Fatalf("Failed to get page: %v", err)
	}
	
	// Get objects
	objects := page.GetObjects()
	
	fmt.Println("\nExtracted objects:")
	fmt.Printf("  Characters: %d\n", len(objects.Chars))
	fmt.Printf("  Lines: %d\n", len(objects.Lines))
	fmt.Printf("  Rectangles: %d\n", len(objects.Rects))
	fmt.Printf("  Curves: %d\n", len(objects.Curves))
	
	// Print lines
	if len(objects.Lines) > 0 {
		fmt.Println("\nLines:")
		for i, line := range objects.Lines {
			fmt.Printf("  Line %d: (%.2f, %.2f) to (%.2f, %.2f) width=%.2f color=RGB(%d,%d,%d)\n",
				i+1, line.X0, line.Y0, line.X1, line.Y1, line.Width,
				line.StrokeColor.R, line.StrokeColor.G, line.StrokeColor.B)
		}
	}
	
	// Print rectangles
	if len(objects.Rects) > 0 {
		fmt.Println("\nRectangles:")
		for i, rect := range objects.Rects {
			fmt.Printf("  Rect %d: (%.2f, %.2f) to (%.2f, %.2f) width=%.2f filled=%v\n",
				i+1, rect.X0, rect.Y0, rect.X1, rect.Y1, rect.Width, rect.NonStroking)
			if rect.NonStroking {
				fmt.Printf("    Fill color: RGB(%d,%d,%d)\n",
					rect.FillColor.R, rect.FillColor.G, rect.FillColor.B)
			} else {
				fmt.Printf("    Stroke color: RGB(%d,%d,%d)\n",
					rect.StrokeColor.R, rect.StrokeColor.G, rect.StrokeColor.B)
			}
		}
	}
	
	// Print curves
	if len(objects.Curves) > 0 {
		fmt.Println("\nCurves:")
		for i, curve := range objects.Curves {
			fmt.Printf("  Curve %d: %d points, width=%.2f color=RGB(%d,%d,%d)\n",
				i+1, len(curve.Points), curve.Width,
				curve.StrokeColor.R, curve.StrokeColor.G, curve.StrokeColor.B)
		}
	}
	
	// Print text
	text := page.ExtractText()
	if text != "" {
		fmt.Printf("\nExtracted text: %s\n", text)
	}
}