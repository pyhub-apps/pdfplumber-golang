package main

import (
	"fmt"
	"log"
	"os"

	pdf "github.com/ledongthuc/pdf"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <pdf-file>")
		os.Exit(1)
	}

	pdfPath := os.Args[1]
	
	// Open PDF
	f, r, err := pdf.Open(pdfPath)
	if err != nil {
		log.Fatalf("Failed to open PDF: %v", err)
	}
	defer f.Close()
	
	fmt.Printf("PDF Info:\n")
	fmt.Printf("  Pages: %d\n", r.NumPage())
	
	// Get first page
	if r.NumPage() > 0 {
		p := r.Page(1)
		
		fmt.Printf("\nPage 1:\n")
		
		// Check if page has MediaBox
		mediaBox := p.V.Key("MediaBox")
		if mediaBox.String() != "" {
			fmt.Printf("  MediaBox: %v\n", mediaBox)
		}
		
		// Get content
		content := p.Content()
		fmt.Printf("  Text items: %d\n", len(content.Text))
		
		// Show text details
		if len(content.Text) > 0 {
			fmt.Println("  First 5 text items:")
			for i, text := range content.Text {
				if i >= 5 {
					break
				}
				fmt.Printf("    %d. Text: %q\n", i+1, text.S)
				fmt.Printf("       Position: X=%.2f, Y=%.2f\n", text.X, text.Y)
				fmt.Printf("       Width: %.2f\n", text.W)
				fmt.Printf("       Font: %s\n", text.Font)
			}
		}
		
		// Try to get all text
		allText := ""
		for _, text := range content.Text {
			allText += text.S
		}
		fmt.Printf("\n  Full text: %q\n", allText)
	}
}