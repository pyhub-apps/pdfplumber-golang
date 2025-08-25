# PDFPlumber-Go

A Go port of the popular Python [pdfplumber](https://github.com/jsvine/pdfplumber) library for extracting information from PDF files.

## Overview

PDFPlumber-Go provides detailed PDF parsing capabilities, allowing you to extract:
- Text with precise positioning
- Tables with structure preservation
- Images and metadata
- Lines, rectangles, and other graphical elements
- Form fields and annotations

This library is built on top of [pdfcpu](https://github.com/pdfcpu/pdfcpu) and aims to provide a similar API to the Python pdfplumber library.

## Features

- **Text Extraction**: Extract text with layout preservation and Unicode support
- **Table Detection**: Automatically detect and extract tables from PDFs
- **Object Access**: Access individual characters, lines, rectangles, and curves
- **Visual Debugging**: Generate visual representations of extracted data
- **Filtering**: Filter and crop content by bounding boxes
- **Go-Native**: Pure Go implementation with no external dependencies

## Installation

```bash
go get github.com/pyhub-apps/pdfplumber-golang
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/pyhub-apps/pdfplumber-golang"
)

func main() {
    // Open a PDF file
    doc, err := pdfplumber.Open("example.pdf")
    if err != nil {
        log.Fatal(err)
    }
    defer doc.Close()
    
    // Get the first page
    page, err := doc.GetPage(0)
    if err != nil {
        log.Fatal(err)
    }
    
    // Extract text from the page
    text := page.ExtractText()
    fmt.Println(text)
    
    // Extract tables
    tables := page.ExtractTables()
    for _, table := range tables {
        for _, row := range table.Rows {
            fmt.Println(row)
        }
    }
}
```

## API Reference

### Document

```go
type Document interface {
    GetMetadata() Metadata
    GetPages() []Page
    GetPage(index int) (Page, error)
    PageCount() int
    Close() error
}
```

### Page

```go
type Page interface {
    GetPageNumber() int
    GetWidth() float64
    GetHeight() float64
    GetObjects() Objects
    ExtractText(opts ...TextExtractionOption) string
    ExtractTables(opts ...TableExtractionOption) []Table
    Crop(bbox BoundingBox) Page
    WithinBBox(bbox BoundingBox) Objects
    Filter(predicate func(Object) bool) Objects
}
```

## Development Status

This project is currently under active development. The following features are implemented:

- [x] Basic PDF document structure
- [x] Core interfaces and types
- [x] Page operations framework
- [ ] Text extraction (in progress)
- [ ] Table extraction (planned)
- [ ] Visual debugging (planned)
- [ ] Complete object extraction (planned)

See [TODOs.md](TODOs.md) for detailed development progress.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [pdfplumber](https://github.com/jsvine/pdfplumber) - The original Python library this project is based on
- [pdfcpu](https://github.com/pdfcpu/pdfcpu) - The underlying PDF processing library

## Comparison with Python pdfplumber

| Feature | Python pdfplumber | PDFPlumber-Go | Status |
|---------|------------------|---------------|---------|
| Text extraction | ✅ | 🚧 | In Progress |
| Table extraction | ✅ | 📋 | Planned |
| Visual debugging | ✅ | 📋 | Planned |
| Character-level access | ✅ | ✅ | Done |
| Line/rect extraction | ✅ | 🚧 | In Progress |
| Image extraction | ✅ | 📋 | Planned |
| Form fields | ✅ | 📋 | Planned |