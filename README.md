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

- **Text Extraction**: Extract text with layout preservation and Unicode support via ToUnicode CMap
- **Table Detection**: Automatically detect and extract tables from PDFs
- **Object Access**: Access individual characters, lines, rectangles, and curves
- **High Performance**: 17-18x faster than Python pdfplumber for text extraction
- **Visual Debugging**: Generate visual representations of extracted data (planned)
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
- [x] Text extraction with ToUnicode CMap support (101% accuracy improvement)
- [x] Table extraction (functional)
- [x] Graphics extraction (lines, rectangles)
- [x] Character, line, and rectangle object extraction
- [ ] Visual debugging (planned)
- [ ] Form fields and annotations (planned)
- [ ] Image extraction (planned)

See [TODOs.md](TODOs.md) for detailed development progress.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [pdfplumber](https://github.com/jsvine/pdfplumber) - The original Python library this project is based on
- [pdfcpu](https://github.com/pdfcpu/pdfcpu) - The underlying PDF processing library

## Performance

PDFPlumber-Go offers exceptional performance compared to Python pdfplumber:

| Operation | Go Performance | Python Performance | Speedup |
|-----------|---------------|-------------------|---------|
| Text Extraction | 8.75ms | 161ms | **18.4x faster** |
| Page Processing | 260 pages/sec | 14.84 pages/sec | **17.6x faster** |
| Total Processing | 15.33ms | 269ms | **17.5x faster** |

See [BENCHMARK_RESULTS.md](BENCHMARK_RESULTS.md) for detailed benchmarks.

## Comparison with Python pdfplumber

| Feature | Python pdfplumber | PDFPlumber-Go | Status |
|---------|------------------|---------------|---------|
| Text extraction | ✅ | ✅ | Done (with CMap support) |
| Table extraction | ✅ | ✅ | Functional |
| Visual debugging | ✅ | 📋 | Planned |
| Character-level access | ✅ | ✅ | Done |
| Line/rect extraction | ✅ | ✅ | Done |
| Image extraction | ✅ | 📋 | Planned |
| Form fields | ✅ | 📋 | Planned |
| ToUnicode CMap | ✅ | ✅ | Done |
| Performance | Baseline | **17x faster** | Optimized |