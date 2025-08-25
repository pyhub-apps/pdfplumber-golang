package pdfplumber

import (
	"strings"
	"testing"
)

func TestOpenPDF(t *testing.T) {
	// Test opening a PDF file
	doc, err := Open("testdata/sample.pdf")
	if err != nil {
		t.Fatalf("Failed to open PDF: %v", err)
	}
	defer doc.Close()

	// Check page count
	if doc.PageCount() != 1 {
		t.Errorf("Expected 1 page, got %d", doc.PageCount())
	}
}

func TestExtractText(t *testing.T) {
	// Open PDF
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

	// Extract text
	text := page.ExtractText()
	
	// Check if text contains expected content
	if !strings.Contains(text, "Dummy PDF file") {
		t.Errorf("Expected text to contain 'Dummy PDF file', got: %s", text)
	}
}

func TestPageProperties(t *testing.T) {
	// Open PDF
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

	// Check page number
	if page.GetPageNumber() != 1 {
		t.Errorf("Expected page number 1, got %d", page.GetPageNumber())
	}

	// Check page dimensions (A4 size)
	width := page.GetWidth()
	height := page.GetHeight()
	
	// A4 is approximately 595 x 842 points
	if width < 590 || width > 600 {
		t.Errorf("Unexpected page width: %.2f", width)
	}
	
	if height < 840 || height > 845 {
		t.Errorf("Unexpected page height: %.2f", height)
	}
}

func TestGetObjects(t *testing.T) {
	// Open PDF
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
	
	// Check that we have characters
	if len(objects.Chars) == 0 {
		t.Error("Expected to find character objects")
	}
	
	// Check first character
	if len(objects.Chars) > 0 {
		firstChar := objects.Chars[0]
		if firstChar.Text != "D" {
			t.Errorf("Expected first character to be 'D', got '%s'", firstChar.Text)
		}
	}
}