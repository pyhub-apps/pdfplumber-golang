package main

import (
	"fmt"
	"github.com/pyhub-apps/pdfplumber-golang/pkg/pdf"
)

func main() {
	cmapData := `
		beginbfchar
		<0001> <0041>
		<0002> <0042>
		endbfchar
	`
	
	cmap := pdf.NewToUnicodeCMap()
	err := cmap.Parse([]byte(cmapData))
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)
		return
	}
	
	fmt.Printf("CMap has %d mappings\n", cmap.GetMappingCount())
	fmt.Printf("CMap details: %s\n", cmap.String())
	
	// Test mapping
	for i := uint16(0); i < 10; i++ {
		if unicode, ok := cmap.MapCIDToUnicode(i); ok {
			fmt.Printf("CID %04X -> %q\n", i, unicode)
		}
	}
}