package main

import (
	"fmt"
	"log"
	"strings"
	
	"github.com/allieus/pdfplumber-go"
)

func main() {
	// Open the PDF
	doc, err := pdfplumber.Open("testdata/sample.pdf")
	if err != nil {
		log.Fatalf("Failed to open PDF: %v", err)
	}
	defer doc.Close()
	
	fmt.Printf("Document has %d pages\n\n", doc.PageCount())
	
	// Process each page
	for i := 0; i < doc.PageCount(); i++ {
		page, err := doc.GetPage(i)
		if err != nil {
			log.Printf("Failed to get page %d: %v", i+1, err)
			continue
		}
		
		fmt.Printf("=== Page %d ===\n", i+1)
		
		// Try different extraction strategies
		strategies := []struct {
			name string
			opts []pdfplumber.TableExtractionOption
		}{
			{
				name: "Line-based (default)",
				opts: []pdfplumber.TableExtractionOption{},
			},
			{
				name: "Text-based",
				opts: []pdfplumber.TableExtractionOption{
					pdfplumber.WithTableStrategy("text", "text"),
				},
			},
		}
		
		for _, strategy := range strategies {
			fmt.Printf("\nStrategy: %s\n", strategy.name)
			tables := page.ExtractTables(strategy.opts...)
			
			if len(tables) == 0 {
				fmt.Println("  No tables found")
				continue
			}
			
			fmt.Printf("  Found %d table(s)\n", len(tables))
			
			for j, table := range tables {
				fmt.Printf("\n  Table %d:\n", j+1)
				fmt.Printf("    Dimensions: %d rows x %d columns\n", 
					len(table.Rows), getMaxColumns(table.Rows))
				fmt.Printf("    BBox: (%.2f, %.2f) to (%.2f, %.2f)\n",
					table.BBox.X0, table.BBox.Y0, table.BBox.X1, table.BBox.Y1)
				
				// Print the table content
				printTable(table)
			}
		}
		
		fmt.Println()
	}
}

// getMaxColumns returns the maximum number of columns in any row
func getMaxColumns(rows [][]string) int {
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	return maxCols
}

// printTable prints a table in a formatted way
func printTable(table pdfplumber.Table) {
	if len(table.Rows) == 0 {
		return
	}
	
	// Calculate column widths
	colWidths := make([]int, getMaxColumns(table.Rows))
	for _, row := range table.Rows {
		for j, cell := range row {
			if j < len(colWidths) {
				cellLen := len(strings.TrimSpace(cell))
				if cellLen > colWidths[j] {
					colWidths[j] = cellLen
				}
			}
		}
	}
	
	// Ensure minimum width
	for i := range colWidths {
		if colWidths[i] < 3 {
			colWidths[i] = 3
		}
		if colWidths[i] > 30 {
			colWidths[i] = 30 // Cap at 30 for readability
		}
	}
	
	// Print separator
	printSeparator(colWidths)
	
	// Print rows
	for i, row := range table.Rows {
		fmt.Print("    |")
		for j := 0; j < len(colWidths); j++ {
			cell := ""
			if j < len(row) {
				cell = strings.TrimSpace(row[j])
				if len(cell) > colWidths[j] {
					cell = cell[:colWidths[j]-3] + "..."
				}
			}
			fmt.Printf(" %-*s |", colWidths[j], cell)
		}
		fmt.Println()
		
		// Print separator after header (first row)
		if i == 0 {
			printSeparator(colWidths)
		}
	}
	
	// Print final separator
	printSeparator(colWidths)
}

// printSeparator prints a table separator line
func printSeparator(colWidths []int) {
	fmt.Print("    +")
	for _, width := range colWidths {
		fmt.Print(strings.Repeat("-", width+2) + "+")
	}
	fmt.Println()
}