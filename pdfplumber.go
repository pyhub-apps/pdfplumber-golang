// Package pdfplumber provides PDF parsing and extraction capabilities similar to Python's pdfplumber
package pdfplumber

import (
	"github.com/pyhub-apps/pdfplumber-golang/pkg/pdf"
)

// Re-export types from pdf package for public API
type (
	Document               = pdf.Document
	Page                  = pdf.Page
	Table                 = pdf.Table
	TableExtractionOption = pdf.TableExtractionOption
	TextExtractionOption  = pdf.TextExtractionOption
	WordExtractionOption  = pdf.WordExtractionOption
	Word                  = pdf.Word
	Objects               = pdf.Objects
	CharObject            = pdf.CharObject
	LineObject            = pdf.LineObject
	RectObject            = pdf.RectObject
	CurveObject           = pdf.CurveObject
	BoundingBox           = pdf.BoundingBox
)

// Re-export option functions
var (
	WithTableStrategy = pdf.WithTableStrategy
	WithMinTableSize  = pdf.WithMinTableSize
	WithTextTolerance = pdf.WithTextTolerance
	WithLayout        = pdf.WithLayout
	WithXTolerance    = pdf.WithXTolerance
	WithYTolerance    = pdf.WithYTolerance
)

// Open opens a PDF file and returns a Document
func Open(filepath string) (pdf.Document, error) {
	// Try ledongthuc implementation first as it has the most accurate text extraction
	doc, err := pdf.OpenWithLedongthuc(filepath)
	if err == nil {
		return doc, nil
	}
	
	// Fallback to dslipak implementation
	doc, err = pdf.OpenWithDslipak(filepath)
	if err == nil {
		return doc, nil
	}
	
	// Final fallback to pdfcpu implementation
	return pdf.Open(filepath)
}

// OpenWithPassword opens a password-protected PDF file
func OpenWithPassword(filepath string, password string) (pdf.Document, error) {
	return pdf.OpenWithPassword(filepath, password)
}

// OpenWithDslipak opens a PDF file using the dslipak/pdf library
func OpenWithDslipak(filepath string) (pdf.Document, error) {
	return pdf.OpenWithDslipak(filepath)
}

// OpenWithLedongthuc opens a PDF file using the ledongthuc/pdf library
// This provides the most accurate text extraction with proper coordinates
func OpenWithLedongthuc(filepath string) (pdf.Document, error) {
	return pdf.OpenWithLedongthuc(filepath)
}