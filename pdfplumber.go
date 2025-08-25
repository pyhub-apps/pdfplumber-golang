// Package pdfplumber provides PDF parsing and extraction capabilities similar to Python's pdfplumber
package pdfplumber

import (
	"github.com/allieus/pdfplumber-go/pkg/pdf"
)

// Open opens a PDF file and returns a Document
func Open(filepath string) (pdf.Document, error) {
	// Try dslipak implementation first as it has better text extraction
	doc, err := pdf.OpenWithDslipak(filepath)
	if err == nil {
		return doc, nil
	}
	
	// Fallback to pdfcpu implementation
	return pdf.Open(filepath)
}

// OpenWithPassword opens a password-protected PDF file
func OpenWithPassword(filepath string, password string) (pdf.Document, error) {
	return pdf.OpenWithPassword(filepath, password)
}

// OpenWithDslipak opens a PDF file using the dslipak/pdf library
// This provides better text extraction capabilities
func OpenWithDslipak(filepath string) (pdf.Document, error) {
	return pdf.OpenWithDslipak(filepath)
}