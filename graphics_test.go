package pdfplumber

import (
	"testing"
)

func TestGraphicsExtraction(t *testing.T) {
	// Test opening a PDF file
	// Note: For proper testing, use a PDF with actual graphics elements
	doc, err := Open("testdata/sample.pdf")
	if err != nil {
		t.Fatalf("Failed to open PDF: %v", err)
	}
	defer doc.Close()

	// Get first page
	page, err := doc.GetPage(0)
	if err != nil {
		t.Fatalf("Failed to get page: %v", err)
	}

	// Get objects
	objects := page.GetObjects()

	// Log what we found
	t.Logf("Graphics extraction results:")
	t.Logf("  Lines: %d", len(objects.Lines))
	t.Logf("  Rectangles: %d", len(objects.Rects))
	t.Logf("  Curves: %d", len(objects.Curves))
	
	// Print first few lines if any
	if len(objects.Lines) > 0 {
		t.Log("First few lines:")
		maxLines := 3
		if len(objects.Lines) < maxLines {
			maxLines = len(objects.Lines)
		}
		for i := 0; i < maxLines; i++ {
			line := objects.Lines[i]
			t.Logf("  Line %d: (%.2f, %.2f) to (%.2f, %.2f) width=%.2f",
				i+1, line.X0, line.Y0, line.X1, line.Y1, line.Width)
		}
	}
	
	// Print first few rectangles if any
	if len(objects.Rects) > 0 {
		t.Log("First few rectangles:")
		maxRects := 3
		if len(objects.Rects) < maxRects {
			maxRects = len(objects.Rects)
		}
		for i := 0; i < maxRects; i++ {
			rect := objects.Rects[i]
			t.Logf("  Rect %d: (%.2f, %.2f) to (%.2f, %.2f) width=%.2f filled=%v",
				i+1, rect.X0, rect.Y0, rect.X1, rect.Y1, rect.Width, rect.NonStroking)
		}
	}
	
	// Print first few curves if any
	if len(objects.Curves) > 0 {
		t.Log("First few curves:")
		maxCurves := 3
		if len(objects.Curves) < maxCurves {
			maxCurves = len(objects.Curves)
		}
		for i := 0; i < maxCurves; i++ {
			curve := objects.Curves[i]
			t.Logf("  Curve %d: %d points, width=%.2f",
				i+1, len(curve.Points), curve.Width)
		}
	}
}

func TestColorExtraction(t *testing.T) {
	// Note: For proper testing, use a PDF with actual graphics elements
	doc, err := Open("testdata/sample.pdf")
	if err != nil {
		t.Fatalf("Failed to open PDF: %v", err)
	}
	defer doc.Close()

	page, err := doc.GetPage(0)
	if err != nil {
		t.Fatalf("Failed to get page: %v", err)
	}

	objects := page.GetObjects()
	
	// Check if any lines have non-default colors
	for i, line := range objects.Lines {
		if line.StrokeColor.R != 0 || line.StrokeColor.G != 0 || line.StrokeColor.B != 0 {
			t.Logf("Line %d has color: RGB(%d, %d, %d)", 
				i, line.StrokeColor.R, line.StrokeColor.G, line.StrokeColor.B)
		}
	}
	
	// Check if any rectangles have fill colors
	for i, rect := range objects.Rects {
		if rect.NonStroking {
			t.Logf("Rectangle %d is filled with color: RGB(%d, %d, %d)",
				i, rect.FillColor.R, rect.FillColor.G, rect.FillColor.B)
		}
	}
}