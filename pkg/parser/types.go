package parser

import (
	"fmt"
)

// PDFObject represents any PDF object
type PDFObject interface {
	Type() string
}

// ObjectRef represents an indirect object reference
type ObjectRef struct {
	Number     int
	Generation int
}

func (r ObjectRef) String() string {
	return fmt.Sprintf("%d %d R", r.Number, r.Generation)
}

func (ObjectRef) Type() string { return "ref" }

// PDFNull represents a null object
type PDFNull struct{}

func (PDFNull) Type() string { return "null" }

// PDFBool represents a boolean object
type PDFBool bool

func (PDFBool) Type() string { return "bool" }

// PDFInt represents an integer object
type PDFInt int64

func (PDFInt) Type() string { return "int" }

// PDFFloat represents a floating-point object
type PDFFloat float64

func (PDFFloat) Type() string { return "float" }

// PDFString represents a string object
type PDFString []byte

func (PDFString) Type() string { return "string" }

// PDFName represents a name object
type PDFName string

func (PDFName) Type() string { return "name" }

// PDFArray represents an array object
type PDFArray []PDFObject

func (PDFArray) Type() string { return "array" }

// PDFDict represents a dictionary object
type PDFDict map[PDFName]PDFObject

func (PDFDict) Type() string { return "dict" }

// Get retrieves a value from the dictionary
func (d PDFDict) Get(key PDFName) PDFObject {
	return d[key]
}

// GetName retrieves a name value from the dictionary
func (d PDFDict) GetName(key PDFName) (PDFName, bool) {
	if obj, ok := d[key]; ok {
		if name, ok := obj.(PDFName); ok {
			return name, true
		}
	}
	return "", false
}

// GetInt retrieves an integer value from the dictionary
func (d PDFDict) GetInt(key PDFName) (int64, bool) {
	if obj, ok := d[key]; ok {
		if i, ok := obj.(PDFInt); ok {
			return int64(i), true
		}
	}
	return 0, false
}

// GetArray retrieves an array value from the dictionary
func (d PDFDict) GetArray(key PDFName) (PDFArray, bool) {
	if obj, ok := d[key]; ok {
		if arr, ok := obj.(PDFArray); ok {
			return arr, true
		}
	}
	return nil, false
}

// GetDict retrieves a dictionary value from the dictionary
func (d PDFDict) GetDict(key PDFName) (PDFDict, bool) {
	if obj, ok := d[key]; ok {
		if dict, ok := obj.(PDFDict); ok {
			return dict, true
		}
	}
	return nil, false
}

// PDFStream represents a stream object
type PDFStream struct {
	Dict PDFDict
	Data []byte
}

func (PDFStream) Type() string { return "stream" }

// PDFIndirectObject represents an indirect object
type PDFIndirectObject struct {
	Number     int
	Generation int
	Object     PDFObject
}

// XRefEntry represents an entry in the cross-reference table
type XRefEntry struct {
	Offset     int64
	Generation int
	InUse      bool
}

// XRefTable represents the cross-reference table
type XRefTable struct {
	Entries map[ObjectRef]*XRefEntry
}

// NewXRefTable creates a new cross-reference table
func NewXRefTable() *XRefTable {
	return &XRefTable{
		Entries: make(map[ObjectRef]*XRefEntry),
	}
}

// Add adds an entry to the cross-reference table
func (x *XRefTable) Add(ref ObjectRef, entry *XRefEntry) {
	x.Entries[ref] = entry
}

// Get retrieves an entry from the cross-reference table
func (x *XRefTable) Get(ref ObjectRef) (*XRefEntry, bool) {
	entry, ok := x.Entries[ref]
	return entry, ok
}

// PDFDocument represents a parsed PDF document
type PDFDocument struct {
	Version  string
	XRef     *XRefTable
	Trailer  PDFDict
	Catalog  PDFDict
	Pages    []*PDFPage
	Objects  map[ObjectRef]PDFObject
	parser   interface{} // Keep reference to parser for lazy loading
}

// PDFPage represents a page in the PDF
type PDFPage struct {
	Number    int
	Dict      PDFDict
	Resources PDFDict
	Contents  []PDFStream
	MediaBox  []float64
	CropBox   []float64
	Document  *PDFDocument // Reference to parent document
}