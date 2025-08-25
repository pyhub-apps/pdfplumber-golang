package pdfplumber

import (
	"fmt"
	"testing"
)

func TestTableExtraction(t *testing.T) {
	// Test with a PDF that contains tables
	doc, err := Open("testdata/sample.pdf")
	if err != nil {
		t.Fatalf("Failed to open PDF: %v", err)
	}
	defer doc.Close()

	// Get first page
	page, err := doc.GetPage(0)
	if err != nil {
		t.Fatalf("Failed to get page: %v", err)
	}

	// Extract tables
	tables := page.ExtractTables()

	// Log results
	t.Logf("Found %d tables", len(tables))
	
	for i, table := range tables {
		t.Logf("Table %d:", i+1)
		t.Logf("  Dimensions: %d rows x %d columns", len(table.Rows), getMaxColumns(table.Rows))
		t.Logf("  BBox: (%.2f, %.2f) to (%.2f, %.2f)", 
			table.BBox.X0, table.BBox.Y0, table.BBox.X1, table.BBox.Y1)
		
		// Print first few rows
		maxRows := 5
		if len(table.Rows) < maxRows {
			maxRows = len(table.Rows)
		}
		
		for j := 0; j < maxRows; j++ {
			t.Logf("  Row %d: %v", j+1, table.Rows[j])
		}
		
		if len(table.Rows) > maxRows {
			t.Logf("  ... and %d more rows", len(table.Rows)-maxRows)
		}
	}
}

func TestTableExtractionWithOptions(t *testing.T) {
	doc, err := Open("testdata/sample.pdf")
	if err != nil {
		t.Fatalf("Failed to open PDF: %v", err)
	}
	defer doc.Close()

	page, err := doc.GetPage(0)
	if err != nil {
		t.Fatalf("Failed to get page: %v", err)
	}

	// Test with different strategies
	testCases := []struct {
		name     string
		opts     []TableExtractionOption
		expected string
	}{
		{
			name: "Line-based detection",
			opts: []TableExtractionOption{
				WithTableStrategy("lines", "lines"),
			},
			expected: "line-based tables",
		},
		{
			name: "Text-based detection",
			opts: []TableExtractionOption{
				WithTableStrategy("text", "text"),
			},
			expected: "text-based tables",
		},
		{
			name: "Custom tolerance",
			opts: []TableExtractionOption{
				WithTextTolerance(5.0),
			},
			expected: "custom tolerance",
		},
		{
			name: "Minimum table size",
			opts: []TableExtractionOption{
				WithMinTableSize(5),
			},
			expected: "minimum 5 rows",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tables := page.ExtractTables(tc.opts...)
			t.Logf("Test %s: Found %d tables", tc.name, len(tables))
			
			for i, table := range tables {
				t.Logf("  Table %d: %d rows x %d columns",
					i+1, len(table.Rows), getMaxColumns(table.Rows))
			}
		})
	}
}

func TestTableExtractionAccuracy(t *testing.T) {
	// This test would verify specific table content if we had a known test PDF
	doc, err := Open("testdata/sample.pdf")
	if err != nil {
		t.Fatalf("Failed to open PDF: %v", err)
	}
	defer doc.Close()

	page, err := doc.GetPage(0)
	if err != nil {
		t.Fatalf("Failed to get page: %v", err)
	}

	tables := page.ExtractTables()
	
	// If we find tables, validate their structure
	for i, table := range tables {
		if len(table.Rows) == 0 {
			t.Errorf("Table %d has no rows", i+1)
			continue
		}
		
		// Check that all rows have consistent column count
		if len(table.Rows) > 1 {
			firstRowCols := len(table.Rows[0])
			for j, row := range table.Rows[1:] {
				if len(row) != firstRowCols {
					t.Logf("Warning: Table %d row %d has %d columns, expected %d",
						i+1, j+2, len(row), firstRowCols)
				}
			}
		}
		
		// Check bounding box validity
		if table.BBox.X1 <= table.BBox.X0 || table.BBox.Y1 <= table.BBox.Y0 {
			t.Errorf("Table %d has invalid bounding box: (%.2f, %.2f) to (%.2f, %.2f)",
				i+1, table.BBox.X0, table.BBox.Y0, table.BBox.X1, table.BBox.Y1)
		}
	}
}

// Helper function to get maximum columns in a table
func getMaxColumns(rows [][]string) int {
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	return maxCols
}

// Benchmark table extraction
func BenchmarkTableExtraction(b *testing.B) {
	doc, err := Open("testdata/sample.pdf")
	if err != nil {
		b.Fatalf("Failed to open PDF: %v", err)
	}
	defer doc.Close()

	page, err := doc.GetPage(0)
	if err != nil {
		b.Fatalf("Failed to get page: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = page.ExtractTables()
	}
}

// Example of how to use table extraction
func ExamplePage_ExtractTables() {
	doc, err := Open("testdata/sample.pdf")
	if err != nil {
		panic(err)
	}
	defer doc.Close()

	page, err := doc.GetPage(0)
	if err != nil {
		panic(err)
	}

	tables := page.ExtractTables()
	
	for i, table := range tables {
		fmt.Printf("Table %d has %d rows\n", i+1, len(table.Rows))
		
		// Print header row if exists
		if len(table.Rows) > 0 {
			fmt.Printf("Headers: %v\n", table.Rows[0])
		}
	}
}