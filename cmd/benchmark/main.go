package main

import (
	"fmt"
	"log"
	"os"
	"time"
	
	"github.com/pyhub-apps/pdfplumber-golang"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run benchmark.go <pdf-file>")
		os.Exit(1)
	}
	
	pdfPath := os.Args[1]
	
	// Warm-up run
	doc, err := pdfplumber.Open(pdfPath)
	if err != nil {
		log.Fatalf("Failed to open PDF: %v", err)
	}
	doc.Close()
	
	// Benchmark PDF opening
	start := time.Now()
	doc, err = pdfplumber.Open(pdfPath)
	if err != nil {
		log.Fatalf("Failed to open PDF: %v", err)
	}
	openTime := time.Since(start)
	
	fmt.Printf("=== Go PDFPlumber Benchmark ===\n")
	fmt.Printf("File: %s\n", pdfPath)
	fmt.Printf("Pages: %d\n", doc.PageCount())
	fmt.Printf("Open time: %v\n", openTime)
	
	// Benchmark text extraction
	var totalTextLen int
	start = time.Now()
	for i := 0; i < doc.PageCount(); i++ {
		page, err := doc.GetPage(i)
		if err != nil {
			continue
		}
		text := page.ExtractText()
		totalTextLen += len(text)
	}
	textTime := time.Since(start)
	
	fmt.Printf("Text extraction time: %v\n", textTime)
	fmt.Printf("Total text length: %d chars\n", totalTextLen)
	fmt.Printf("Text/sec: %.0f chars/sec\n", float64(totalTextLen)/textTime.Seconds())
	
	// Benchmark table extraction
	var totalTables int
	start = time.Now()
	for i := 0; i < doc.PageCount(); i++ {
		page, err := doc.GetPage(i)
		if err != nil {
			continue
		}
		tables := page.ExtractTables(nil)
		totalTables += len(tables)
	}
	tableTime := time.Since(start)
	
	fmt.Printf("Table extraction time: %v\n", tableTime)
	fmt.Printf("Total tables found: %d\n", totalTables)
	
	// Benchmark object extraction
	var totalObjects int
	start = time.Now()
	for i := 0; i < doc.PageCount(); i++ {
		page, err := doc.GetPage(i)
		if err != nil {
			continue
		}
		
		objects := page.GetObjects()
		totalObjects += len(objects.Chars) + len(objects.Lines) + len(objects.Rects)
	}
	objectTime := time.Since(start)
	
	fmt.Printf("Object extraction time: %v\n", objectTime)
	fmt.Printf("Total objects: %d\n", totalObjects)
	fmt.Printf("Objects/sec: %.0f obj/sec\n", float64(totalObjects)/objectTime.Seconds())
	
	// Summary
	totalTime := openTime + textTime + tableTime + objectTime
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total processing time: %v\n", totalTime)
	fmt.Printf("Pages/sec: %.2f\n", float64(doc.PageCount())/totalTime.Seconds())
	
	doc.Close()
}