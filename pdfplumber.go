// Package pdfplumber provides PDF parsing and extraction capabilities similar to Python's pdfplumber
package pdfplumber

import (
	"github.com/allieus/pdfplumber-go/pkg/pdf"
)

// Open opens a PDF file and returns a Document
func Open(filepath string) (pdf.Document, error) {
	return pdf.Open(filepath)
}

// OpenWithPassword opens a password-protected PDF file
func OpenWithPassword(filepath string, password string) (pdf.Document, error) {
	return pdf.OpenWithPassword(filepath, password)
}