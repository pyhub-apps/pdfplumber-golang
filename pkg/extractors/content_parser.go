package extractors

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/pyhub-apps/pdfplumber-golang/pkg/pdf"
	gopdf "github.com/dslipak/pdf"
)

// ContentParser parses PDF content streams to extract objects
type ContentParser struct {
	reader *gopdf.Reader
}

// NewContentParser creates a new content parser
func NewContentParser(filepath string) (*ContentParser, error) {
	r, err := gopdf.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %w", err)
	}
	
	return &ContentParser{
		reader: r,
	}, nil
}

// ExtractPageObjects extracts all objects from a specific page
func (cp *ContentParser) ExtractPageObjects(pageNum int) (pdf.Objects, error) {
	if pageNum < 1 || pageNum > cp.reader.NumPage() {
		return pdf.Objects{}, fmt.Errorf("invalid page number: %d", pageNum)
	}
	
	page := cp.reader.Page(pageNum)
	
	objects := pdf.Objects{
		Chars:  []pdf.CharObject{},
		Lines:  []pdf.LineObject{},
		Rects:  []pdf.RectObject{},
		Curves: []pdf.CurveObject{},
		Images: []pdf.ImageObject{},
		Annos:  []pdf.AnnotationObject{},
	}
	
	// Extract text content
	content := page.Content()
	textContent, err := cp.extractTextFromContent(content)
	if err != nil {
		return objects, fmt.Errorf("failed to extract text: %w", err)
	}
	
	// Convert extracted text to CharObjects
	objects.Chars = cp.textToCharObjects(textContent)
	
	// Extract graphics operations (lines, rects, etc.)
	// This requires parsing the content stream operators
	if err := cp.extractGraphicsFromContent(content, &objects); err != nil {
		// Log error but continue - graphics extraction is secondary
		fmt.Printf("Warning: failed to extract graphics: %v\n", err)
	}
	
	return objects, nil
}

// extractTextFromContent extracts text from page content stream
func (cp *ContentParser) extractTextFromContent(content gopdf.Content) (string, error) {
	var buf bytes.Buffer
	
	for _, item := range content.Text {
		// Each text item contains text and position information
		if item.S != "" {
			buf.WriteString(item.S)
			if !strings.HasSuffix(item.S, " ") {
				buf.WriteString(" ")
			}
		}
	}
	
	return buf.String(), nil
}

// textToCharObjects converts text string to CharObject slice
func (cp *ContentParser) textToCharObjects(text string) []pdf.CharObject {
	chars := []pdf.CharObject{}
	
	// Simple character extraction - in real implementation,
	// we would need to track position information from content stream
	x, y := 0.0, 0.0
	charWidth, charHeight := 10.0, 12.0 // Default sizes
	
	for _, ch := range text {
		if ch == '\n' {
			x = 0
			y += charHeight
			continue
		}
		
		char := pdf.CharObject{
			Text:     string(ch),
			Font:     "default",
			FontSize: 12,
			X0:       x,
			Y0:       y,
			X1:       x + charWidth,
			Y1:       y + charHeight,
			Width:    charWidth,
			Height:   charHeight,
			Color:    pdf.Color{R: 0, G: 0, B: 0, A: 255},
		}
		
		chars = append(chars, char)
		x += charWidth
		
		// Simple line wrapping
		if x > 600 {
			x = 0
			y += charHeight
		}
	}
	
	return chars
}

// extractGraphicsFromContent extracts graphics operations from content stream
func (cp *ContentParser) extractGraphicsFromContent(content gopdf.Content, objects *pdf.Objects) error {
	// This is a placeholder for graphics extraction
	// The dslipak/pdf library doesn't directly expose graphics operations,
	// so we would need to parse the raw content stream
	
	// For now, we'll just extract basic path information if available
	// Real implementation would parse PDF operators like:
	// - m (moveto), l (lineto) for lines
	// - re (rectangle) for rectangles
	// - c, v, y (curveto) for curves
	
	return nil
}

// ExtractText extracts all text from a page
func (cp *ContentParser) ExtractText(pageNum int) (string, error) {
	if pageNum < 1 || pageNum > cp.reader.NumPage() {
		return "", fmt.Errorf("invalid page number: %d", pageNum)
	}
	
	page := cp.reader.Page(pageNum)
	
	content := page.Content()
	return cp.extractTextFromContent(content)
}

// GetPageCount returns the number of pages in the PDF
func (cp *ContentParser) GetPageCount() int {
	return cp.reader.NumPage()
}

// Close releases resources
func (cp *ContentParser) Close() error {
	// The reader doesn't need explicit closing
	return nil
}

// ParseContentStream parses a raw PDF content stream
func ParseContentStream(stream io.Reader) ([]ContentOperation, error) {
	// This would parse the raw PDF content stream operators
	// For now, return empty slice
	return []ContentOperation{}, nil
}

// ContentOperation represents a single PDF content stream operation
type ContentOperation struct {
	Operator string
	Operands []interface{}
}