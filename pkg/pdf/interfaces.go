package pdf

import (
	"io"
)

// Document represents a PDF document with methods similar to pdfplumber.PDF
type Document interface {
	// GetMetadata returns the PDF metadata
	GetMetadata() Metadata
	
	// GetPages returns all pages in the document
	GetPages() []Page
	
	// GetPage returns a specific page by index (0-based)
	GetPage(index int) (Page, error)
	
	// PageCount returns the total number of pages
	PageCount() int
	
	// Close releases resources associated with the document
	Close() error
}

// Page represents a single page in a PDF document
type Page interface {
	// GetPageNumber returns the page number (1-based)
	GetPageNumber() int
	
	// GetWidth returns the page width
	GetWidth() float64
	
	// GetHeight returns the page height
	GetHeight() float64
	
	// GetRotation returns the page rotation in degrees
	GetRotation() int
	
	// GetBBox returns the page bounding box
	GetBBox() BoundingBox
	
	// GetObjects returns all objects on the page
	GetObjects() Objects
	
	// ExtractText extracts text from the page
	ExtractText(opts ...TextExtractionOption) string
	
	// ExtractTables extracts tables from the page
	ExtractTables(opts ...TableExtractionOption) []Table
	
	// Crop returns a new page cropped to the specified bounding box
	Crop(bbox BoundingBox) Page
	
	// WithinBBox filters objects within a bounding box
	WithinBBox(bbox BoundingBox) Objects
	
	// Filter filters objects based on a predicate function
	Filter(predicate func(Object) bool) Objects
	
	// ToImage renders the page to an image (for visual debugging)
	ToImage(opts ...ImageOption) (io.Reader, error)
}

// Object represents a PDF object (char, line, rect, curve, etc.)
type Object interface {
	// GetType returns the object type
	GetType() ObjectType
	
	// GetBBox returns the object's bounding box
	GetBBox() BoundingBox
	
	// GetProperties returns object-specific properties
	GetProperties() map[string]interface{}
}

// Extractor is the base interface for all extractors
type Extractor interface {
	// Extract performs the extraction
	Extract(page Page) (interface{}, error)
}

// TextExtractor extracts text from a page
type TextExtractor interface {
	Extractor
	// ExtractText extracts text with options
	ExtractText(page Page, opts ...TextExtractionOption) (string, error)
}

// TableExtractor extracts tables from a page
type TableExtractor interface {
	Extractor
	// ExtractTables extracts tables with options
	ExtractTables(page Page, opts ...TableExtractionOption) ([]Table, error)
}