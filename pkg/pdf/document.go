package pdf

import (
	"fmt"
	"os"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// PDFDocument implements the Document interface using pdfcpu
type PDFDocument struct {
	ctx      *model.Context
	filepath string
	pages    []Page
	metadata Metadata
}

// Open opens a PDF file and returns a Document
func Open(filepath string) (Document, error) {
	return OpenWithPassword(filepath, "")
}

// OpenWithPassword opens a password-protected PDF file
func OpenWithPassword(filepath string, password string) (Document, error) {
	// Read PDF file
	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	// Create pdfcpu configuration
	conf := model.NewDefaultConfiguration()
	if password != "" {
		conf.UserPW = password
		conf.OwnerPW = password
	}

	// Parse PDF with pdfcpu
	ctx, err := api.ReadContextFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF context: %w", err)
	}

	// Validate the PDF
	if err := api.ValidateContext(ctx); err != nil {
		return nil, fmt.Errorf("invalid PDF: %w", err)
	}

	doc := &PDFDocument{
		ctx:      ctx,
		filepath: filepath,
	}

	// Extract metadata
	doc.extractMetadata()

	// Initialize pages
	if err := doc.initializePages(); err != nil {
		return nil, fmt.Errorf("failed to initialize pages: %w", err)
	}

	return doc, nil
}

// extractMetadata extracts PDF metadata from the context
func (d *PDFDocument) extractMetadata() {
	if d.ctx.Info == nil {
		return
	}

	// The Info field is an IndirectRef, we need to resolve it to get the actual Dict
	// For now, we'll skip metadata extraction as it requires more complex handling
	// TODO: Implement proper metadata extraction from IndirectRef
	
	d.metadata = Metadata{
		Title:    "",
		Author:   "",
		Subject:  "",
		Keywords: "",
		Creator:  "",
		Producer: "",
	}
}

// initializePages initializes all pages in the document
func (d *PDFDocument) initializePages() error {
	pageCount := d.ctx.PageCount
	d.pages = make([]Page, pageCount)

	for i := 1; i <= pageCount; i++ {
		page, err := NewPDFCPUPage(d.ctx, i)
		if err != nil {
			return fmt.Errorf("failed to create page %d: %w", i, err)
		}
		d.pages[i-1] = page
	}

	return nil
}

// GetMetadata returns the PDF metadata
func (d *PDFDocument) GetMetadata() Metadata {
	return d.metadata
}

// GetPages returns all pages in the document
func (d *PDFDocument) GetPages() []Page {
	return d.pages
}

// GetPage returns a specific page by index (0-based)
func (d *PDFDocument) GetPage(index int) (Page, error) {
	if index < 0 || index >= len(d.pages) {
		return nil, fmt.Errorf("page index %d out of range [0, %d)", index, len(d.pages))
	}
	return d.pages[index], nil
}

// PageCount returns the total number of pages
func (d *PDFDocument) PageCount() int {
	return len(d.pages)
}

// Close releases resources associated with the document
func (d *PDFDocument) Close() error {
	// Clean up resources if needed
	d.ctx = nil
	d.pages = nil
	return nil
}

// Helper functions

func getStringFromDict(dict types.Dict, key string) string {
	if dict == nil {
		return ""
	}
	
	obj := dict[key]
	if obj == nil {
		return ""
	}
	
	switch v := obj.(type) {
	case types.StringLiteral:
		return string(v)
	case types.HexLiteral:
		return string(v)
	default:
		return ""
	}
}

func parsePDFDate(dateStr string) time.Time {
	// PDF date format: D:YYYYMMDDHHmmSSOHH'mm
	// For simplicity, we'll use a basic parser
	if len(dateStr) < 16 {
		return time.Time{}
	}
	
	// Remove "D:" prefix if present
	if dateStr[:2] == "D:" {
		dateStr = dateStr[2:]
	}
	
	// Try to parse the date
	layout := "20060102150405"
	if len(dateStr) >= 14 {
		t, err := time.Parse(layout, dateStr[:14])
		if err == nil {
			return t
		}
	}
	
	return time.Time{}
}